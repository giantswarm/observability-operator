package controller

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/credential"
	operatormetrics "github.com/giantswarm/observability-operator/pkg/metrics"
)

// AgentCredentialReconciler reconciles AgentCredential objects. It renders a
// per-credential basic-auth Secret and aggregates the htpasswd entries into
// the per-backend gateway Secrets.
type AgentCredentialReconciler struct {
	client.Client
	Renderer        *credential.Renderer
	Aggregator      *credential.Aggregator
	finalizerHelper FinalizerHelper
}

// SetupAgentCredentialReconciler wires the reconciler into the manager. The
// caller is expected to skip calling this when auth mode is `none`.
func SetupAgentCredentialReconciler(mgr manager.Manager, cfg config.Config) error {
	gatewayConfigs := credential.GatewayConfigs{
		observabilityv1alpha1.CredentialBackendMetrics: credential.NewGatewayConfig(
			cfg.Monitoring.Gateway.Namespace,
			cfg.Monitoring.Gateway.IngressSecretName,
			cfg.Monitoring.Gateway.HTTPRouteSecretName,
		),
		observabilityv1alpha1.CredentialBackendLogs: credential.NewGatewayConfig(
			cfg.Logging.Gateway.Namespace,
			cfg.Logging.Gateway.IngressSecretName,
			cfg.Logging.Gateway.HTTPRouteSecretName,
		),
		observabilityv1alpha1.CredentialBackendTraces: credential.NewGatewayConfig(
			cfg.Tracing.Gateway.Namespace,
			cfg.Tracing.Gateway.IngressSecretName,
			cfg.Tracing.Gateway.HTTPRouteSecretName,
		),
	}

	r := &AgentCredentialReconciler{
		Client:          mgr.GetClient(),
		Renderer:        credential.NewRenderer(mgr.GetClient()),
		Aggregator:      credential.NewAggregator(mgr.GetClient(), gatewayConfigs),
		finalizerHelper: NewFinalizerHelper(mgr.GetClient(), observabilityv1alpha1.AgentCredentialFinalizer),
	}

	return r.SetupWithManager(mgr)
}

// SetupWithManager registers the controller with the manager.
func (r *AgentCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("agentcredential").
		For(&observabilityv1alpha1.AgentCredential{}).
		Owns(&corev1.Secret{}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build agentcredential controller: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=agentcredentials,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=agentcredentials/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=agentcredentials/finalizers,verbs=update

// Reconcile moves the AgentCredential's Secret and the per-backend gateway
// htpasswd Secret towards the desired state.
func (r *AgentCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("agentcredential", req.NamespacedName)
	ctx = log.IntoContext(ctx, logger)

	cred := &observabilityv1alpha1.AgentCredential{}
	if err := r.Get(ctx, req.NamespacedName, cred); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get agent credential: %w", err)
	}

	if !cred.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cred)
	}

	return r.reconcileCreate(ctx, cred)
}

func (r *AgentCredentialReconciler) reconcileCreate(ctx context.Context, cred *observabilityv1alpha1.AgentCredential) (ctrl.Result, error) {
	added, err := r.finalizerHelper.EnsureAdded(ctx, cred)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
	}
	if added {
		return ctrl.Result{}, nil
	}

	secret, err := r.Renderer.Render(ctx, cred)
	if err != nil {
		operatormetrics.AgentCredentialReconcileErrors.WithLabelValues(string(cred.Spec.Backend), "render").Inc()
		r.setCondition(cred, observabilityv1alpha1.AgentCredentialConditionReady, metav1.ConditionFalse, "RenderFailed", err.Error())
		return ctrl.Result{}, errors.Join(err, r.Status().Update(ctx, cred))
	}
	r.setCondition(cred, observabilityv1alpha1.AgentCredentialConditionReady, metav1.ConditionTrue, "Rendered", "Secret rendered")

	if err := r.Aggregator.Aggregate(ctx, cred.Spec.Backend); err != nil {
		operatormetrics.AgentCredentialReconcileErrors.WithLabelValues(string(cred.Spec.Backend), "aggregate").Inc()
		r.setCondition(cred, observabilityv1alpha1.AgentCredentialConditionGatewaySynced, metav1.ConditionFalse, "AggregateFailed", err.Error())
		return ctrl.Result{}, errors.Join(err, r.Status().Update(ctx, cred))
	}
	r.setCondition(cred, observabilityv1alpha1.AgentCredentialConditionGatewaySynced, metav1.ConditionTrue, "Aggregated", "Gateway htpasswd aggregated")

	cred.Status.SecretRef = &corev1.LocalObjectReference{Name: secret.Name}
	if err := r.Status().Update(ctx, cred); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update agent credential status: %w", err)
	}

	operatormetrics.AgentCredentialInfo.WithLabelValues(cred.Namespace, cred.Name, string(cred.Spec.Backend), cred.Spec.AgentName).Set(1)
	return ctrl.Result{}, nil
}

func (r *AgentCredentialReconciler) reconcileDelete(ctx context.Context, cred *observabilityv1alpha1.AgentCredential) (ctrl.Result, error) {
	// Re-aggregate to drop this credential's entry from the gateway secret.
	// The deletion-timestamp check inside the aggregator ensures the entry is
	// omitted from the regenerated htpasswd.
	if err := r.Aggregator.Aggregate(ctx, cred.Spec.Backend); err != nil {
		operatormetrics.AgentCredentialReconcileErrors.WithLabelValues(string(cred.Spec.Backend), "aggregate").Inc()
		return ctrl.Result{}, fmt.Errorf("failed to aggregate gateway secret on delete: %w", err)
	}

	operatormetrics.AgentCredentialInfo.DeleteLabelValues(cred.Namespace, cred.Name, string(cred.Spec.Backend), cred.Spec.AgentName)

	if err := r.finalizerHelper.EnsureRemoved(ctx, cred); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *AgentCredentialReconciler) setCondition(cred *observabilityv1alpha1.AgentCredential, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
