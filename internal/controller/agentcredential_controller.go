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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
// caller gates this on --controllers-agent-credential-enabled.
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
		// Re-evaluate the other credentials on the same backend whenever one
		// may take or give up an agent name, so a duplicate is promoted as soon
		// as the credential that owned the name is deleted or renamed.
		Watches(
			&observabilityv1alpha1.AgentCredential{},
			handler.EnqueueRequestsFromMapFunc(r.mapToBackendPeers),
			builder.WithPredicates(agentNameClaimChanged()),
		).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build agentcredential controller: %w", err)
	}
	return nil
}

// mapToBackendPeers enqueues every other AgentCredential on the backend of
// the given one. On delete it receives the deleted object, whose backend still
// identifies the peers to enqueue.
func (r *AgentCredentialReconciler) mapToBackendPeers(ctx context.Context, obj client.Object) []reconcile.Request {
	changed, ok := obj.(*observabilityv1alpha1.AgentCredential)
	if !ok {
		return nil
	}

	credentials := &observabilityv1alpha1.AgentCredentialList{}
	if err := r.List(ctx, credentials); err != nil {
		log.FromContext(ctx).Error(err, "failed to list agent credentials to enqueue backend peers", "backend", changed.Spec.Backend)
		return nil
	}

	var requests []reconcile.Request
	for i := range credentials.Items {
		peer := &credentials.Items[i]
		if peer.Spec.Backend != changed.Spec.Backend {
			continue
		}
		if peer.Namespace == changed.Namespace && peer.Name == changed.Name {
			continue
		}
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(peer)})
	}
	return requests
}

// agentNameClaimChanged passes the events that can change which credential
// owns an agent name: creation, deletion, the start of deletion, and changes
// to the backend or agent name. Status-only updates are dropped so peers do
// not reconcile each other in a loop.
func agentNameClaimChanged() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCred, okOld := e.ObjectOld.(*observabilityv1alpha1.AgentCredential)
			newCred, okNew := e.ObjectNew.(*observabilityv1alpha1.AgentCredential)
			if !okOld || !okNew {
				return false
			}
			return oldCred.Spec.Backend != newCred.Spec.Backend ||
				oldCred.Spec.AgentName != newCred.Spec.AgentName ||
				oldCred.DeletionTimestamp.IsZero() != newCred.DeletionTimestamp.IsZero()
		},
	}
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

	// Ready and GatewaySynced are set on the in-memory object as each step
	// completes and written together by a single status update at the end (or
	// on the first failure), so neither write clobbers the other.
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

	owner, err := r.agentNameOwner(ctx, cred)
	if err != nil {
		return ctrl.Result{}, err
	}
	if owner != nil && client.ObjectKeyFromObject(owner) != client.ObjectKeyFromObject(cred) {
		// A loser is not requeued: the peer watch reconciles it again when the
		// owner is deleted or renamed.
		r.setCondition(cred, observabilityv1alpha1.AgentCredentialConditionGatewaySynced, metav1.ConditionFalse, "DuplicateAgentName",
			fmt.Sprintf("Agent name %q on backend %q is already used by AgentCredential %s/%s, which is older; this credential's entry is not served until that one is deleted or renamed",
				cred.Spec.AgentName, cred.Spec.Backend, owner.Namespace, owner.Name))
	} else {
		r.setCondition(cred, observabilityv1alpha1.AgentCredentialConditionGatewaySynced, metav1.ConditionTrue, "Aggregated", "Gateway htpasswd aggregated")
	}

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

// agentNameOwner returns the AgentCredential whose htpasswd entry the gateway
// serves for cred's backend and agent name, following the aggregator's rule.
func (r *AgentCredentialReconciler) agentNameOwner(ctx context.Context, cred *observabilityv1alpha1.AgentCredential) (*observabilityv1alpha1.AgentCredential, error) {
	credentials := &observabilityv1alpha1.AgentCredentialList{}
	if err := r.List(ctx, credentials); err != nil {
		return nil, fmt.Errorf("failed to list agent credentials: %w", err)
	}
	return credential.AgentNameOwners(credentials.Items, cred.Spec.Backend)[cred.Spec.AgentName], nil
}

func (r *AgentCredentialReconciler) setCondition(cred *observabilityv1alpha1.AgentCredential, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
