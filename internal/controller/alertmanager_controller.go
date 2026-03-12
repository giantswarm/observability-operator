package controller

import (
	"context"
	"fmt"
	"slices"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

// AlertmanagerReconciler reconciles Alertmanager config secrets labelled
// observability.giantswarm.io/kind: alertmanager-config.
// It pushes the configuration to Mimir Alertmanager and adds a finalizer so
// the config is also deleted from Mimir when the secret is removed.
type AlertmanagerReconciler struct {
	client              client.Client
	alertmanagerService alertmanager.Service
	tenantRepository    tenancy.TenantRepository
	finalizerHelper     FinalizerHelper
}

// SetupAlertmanagerReconciler adds a controller into mgr that reconciles Alertmanager config secrets.
func SetupAlertmanagerReconciler(mgr ctrl.Manager, cfg config.Config) error {
	r := &AlertmanagerReconciler{
		client:              mgr.GetClient(),
		alertmanagerService: alertmanager.New(cfg),
		tenantRepository:    tenancy.NewTenantRepository(mgr.GetClient()),
		finalizerHelper:     NewFinalizerHelper(mgr.GetClient(), alertmanager.AlertmanagerConfigFinalizer),
	}

	alertmanagerConfigSecretsPredicate, err := predicates.NewAlertmanagerConfigSecretsPredicate()
	if err != nil {
		return fmt.Errorf("failed to create Alertmanager config secrets predicate: %w", err)
	}
	podPredicate := predicates.NewAlertmanagerPodPredicate()

	err = ctrl.NewControllerManagedBy(mgr).
		Named("alertmanager").
		For(&v1.Secret{}, builder.WithPredicates(alertmanagerConfigSecretsPredicate)).
		Watches(&v1.Pod{}, podEventHandler(cfg), builder.WithPredicates(podPredicate)).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

// podEventHandler returns an event handler that enqueues requests for the Alertmanager secret only.
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

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=list;watch

// Reconcile is the main reconciliation loop for Alertmanager config secrets.
func (r AlertmanagerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("secret", req.NamespacedName)
	ctx = log.IntoContext(ctx, logger)
	logger.Info("started reconciling")
	defer logger.Info("finished reconciling")

	secret := &v1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			// Secret was fully deleted (no finalizer was set). Nothing to do.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Alertmanager secret: %w", err)
	}

	if !secret.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, secret)
	}

	return r.reconcileCreate(ctx, secret)
}

// reconcileCreate ensures the Alertmanager configuration in the secret is pushed to Mimir
// and that the secret has a finalizer so it can be cleaned up on deletion.
func (r AlertmanagerReconciler) reconcileCreate(ctx context.Context, secret *v1.Secret) (ctrl.Result, error) {
	// Add finalizer first to ensure the deletion path can clean up the Mimir config.
	finalizerAdded, err := r.finalizerHelper.EnsureAdded(ctx, secret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure finalizer is added: %w", err)
	}
	if finalizerAdded {
		// Requeue so we continue with the rest of reconciliation after the patch settles.
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	tenant, tenantLabelExists := secret.Labels[tenancy.TenantSelectorLabel]
	if !tenantLabelExists {
		logger.Info("tenant label missing, skipping")
		return ctrl.Result{}, nil
	}

	tenants, err := r.tenantRepository.List(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list tenants: %w", err)
	}

	if !slices.Contains(tenants, tenant) {
		logger.Info("tenant not in list, skipping", "tenant", tenant)
		return ctrl.Result{}, nil
	}

	if err := r.alertmanagerService.ConfigureFromSecret(ctx, secret, tenant); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to configure alertmanager: %w", err)
	}

	return ctrl.Result{}, nil
}

// reconcileDelete removes the Alertmanager configuration from Mimir for the tenant
// associated with this secret, then removes the finalizer so the secret can be deleted.
func (r AlertmanagerReconciler) reconcileDelete(ctx context.Context, secret *v1.Secret) error {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(secret, alertmanager.AlertmanagerConfigFinalizer) {
		// No finalizer — nothing to clean up.
		return nil
	}

	tenant, tenantLabelExists := secret.Labels[tenancy.TenantSelectorLabel]
	if !tenantLabelExists {
		// Cannot identify the tenant to clean up. Remove the finalizer anyway to unblock deletion.
		logger.Info("tenant label missing during deletion, removing finalizer without Mimir cleanup")
	} else {
		if err := r.alertmanagerService.DeleteForTenant(ctx, tenant); err != nil {
			return fmt.Errorf("failed to delete alertmanager config for tenant %q: %w", tenant, err)
		}
		logger.Info("deleted alertmanager configuration from Mimir", "tenant", tenant)
	}

	// Finalizer must be removed last, after all cleanup has succeeded.
	if err := r.finalizerHelper.EnsureRemoved(ctx, secret); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}
