package controller

import (
	"context"
	"fmt"
	"slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

// AlertmanagerReconciler reconciles the Alertmanager secret created by the observability-operator Helm chart
// and configures the Alertmanager instance with the configuration stored in the secret.
// This controller do not make use of finalizers as the configuration is not removed from Alertmanager when the secret is deleted.
type AlertmanagerReconciler struct {
	client client.Client

	alertmanagerService alertmanager.Service
}

// SetupAlertmanagerReconciler adds a controller into mgr that reconciles the Alertmanager secret.
func SetupAlertmanagerReconciler(mgr ctrl.Manager, cfg config.Config) error {
	r := &AlertmanagerReconciler{
		client:              mgr.GetClient(),
		alertmanagerService: alertmanager.New(cfg),
	}

	alertmanagerConfigSecretsPredicate, err := predicates.NewAlertmanagerConfigSecretsPredicate()
	if err != nil {
		return fmt.Errorf("failed to create Alertmanager config secrets predicate: %w", err)
	}
	podPredicate := predicates.NewAlertmanagerPodPredicate()

	// Requeue the Alertmanager secret when the Mimir Alertmanager pod changes
	p := podEventHandler(cfg)

	// Setup the controller
	err = ctrl.NewControllerManagedBy(mgr).
		Named("alertmanager").
		// Reconcile only the Alertmanager secret
		For(&v1.Secret{}, builder.WithPredicates(alertmanagerConfigSecretsPredicate)).
		// Watch only the Mimir Alertmanager pod
		Watches(&v1.Pod{}, p, builder.WithPredicates(podPredicate)).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

// podEventHandler returns an event handler that enqueues requests for the Alertmanager secret only.
// For now there is only one Alertmanager secret to be reconciled.
func podEventHandler(cfg config.Config) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      cfg.Monitoring.AlertmanagerSecretName,
					Namespace: cfg.Operator.OperatorNamespace,
				},
			},
		}
	})
}

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// Reconcile main logic
func (r AlertmanagerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling")

	// Retrieve the secret being reconciled
	secret := &v1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Alertmanager secret: %w", err)
	}

	if !secret.DeletionTimestamp.IsZero() {
		// Nothing to do if the secret is being deleted
		// Configuration is not removed from Alertmanager when the secret is deleted.
		return ctrl.Result{}, nil
	}

	tenant, tenantLabelExists := secret.Labels[tenancy.TenantSelectorLabel]
	if !tenantLabelExists {
		// Tenant label is missing, skipping reconciliation
		logger.Info("Tenant label is missing, skipping reconciliation")
		return ctrl.Result{}, nil
	}
	// Get list of tenants
	var tenants []string
	tenants, err := tenancy.ListTenants(ctx, r.client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list tenants: %w", err)
	}

	if !slices.Contains(tenants, tenant) {
		// Nothing to do if the tenant is not in the list of tenants
		logger.Info("Tenant not found in the list of tenants, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// TODO: Do we want to support deletion of alerting configs?
	err = r.alertmanagerService.Configure(ctx, secret, tenant)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to configure alertmanager: %w", err)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}
