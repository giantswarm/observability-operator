package controller

import (
	"context"
	"slices"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/internal/controller/predicates"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	AlertmanagerConfigSelectorLabelName  = "observability.giantswarm.io/kind"
	AlertmanagerConfigSelectorLabelValue = "alertmanager-config"
)

// AlertmanagerReconciler reconciles the Alertmanager secret created by the observability-operator Helm chart
// and configures the Alertmanager instance with the configuration stored in the secret.
// This controller do not make use of finalizers as the configuration is not removed from Alertmanager when the secret is deleted.
type AlertmanagerReconciler struct {
	client client.Client

	alertmanagerService alertmanager.Service
}

// SetupAlertmanagerReconciler adds a controller into mgr that reconciles the Alertmanager secret.
func SetupAlertmanagerReconciler(mgr ctrl.Manager, conf config.Config) error {
	r := &AlertmanagerReconciler{
		client:              mgr.GetClient(),
		alertmanagerService: alertmanager.New(conf),
	}

	// Filter only the Alertmanager configuration secrets
	alertmanagerConfigSecretsPredicate, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				AlertmanagerConfigSelectorLabelName: AlertmanagerConfigSelectorLabelValue,
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      tenancy.TenantSelectorLabel,
					Operator: metav1.LabelSelectorOpExists,
				},
			},
		})
	if err != nil {
		return errors.WithStack(err)
	}
	// Filter only the Mimir Alertmanager pod
	podPredicate := predicates.NewAlertmanagerPodPredicate()

	// Requeue the Alertmanager secret when the Mimir Alertmanager pod changes
	p := podEventHandler(conf)

	// Setup the controller
	return ctrl.NewControllerManagedBy(mgr).
		Named("alertmanager").
		For(&v1.Secret{}, builder.WithPredicates(alertmanagerConfigSecretsPredicate)).
		Watches(&v1.Pod{}, p, builder.WithPredicates(podPredicate)).
		Complete(r)
}

// podEventHandler returns an event handler that enqueues requests for the Alertmanager secret only.
// For now there is only one Alertmanager secret to be reconciled.
func podEventHandler(conf config.Config) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      conf.Monitoring.AlertmanagerSecretName,
					Namespace: conf.OperatorNamespace,
				},
			},
		}
	})
}

// Reconcile main logic
func (r AlertmanagerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling")

	// Retrieve the secret being reconciled
	secret := &v1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
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
		return ctrl.Result{}, errors.WithStack(err)
	}

	if !slices.Contains(tenants, tenant) {
		// Nothing to do if the tenant is not in the list of tenants
		logger.Info("Tenant not found in the list of tenants, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// TODO: Do we want to support deletion of alerting configs?
	err = r.alertmanagerService.Configure(ctx, secret, tenant)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}
