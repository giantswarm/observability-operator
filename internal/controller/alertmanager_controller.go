package controller

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/internal/controller/predicates"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/config"
)

// AlertmanagerReconciler reconciles the Alertmanager secret created by the observability-operator Helm chart
// and configures the Alertmanager instance with the configuration stored in the secret.
// This controller do not make use of finalizers as the configuration is not removed from Alertmanager when the secret is deleted.
type AlertmanagerReconciler struct {
	client client.Client

	alertmanagerJob alertmanager.Job
}

// SetupAlertmanagerReconciler adds a controller into mgr that reconciles the Alertmanager secret.
func SetupAlertmanagerReconciler(mgr ctrl.Manager, conf config.Config) error {
	r := &AlertmanagerReconciler{
		client:          mgr.GetClient(),
		alertmanagerJob: alertmanager.New(conf),
	}

	// Filter only the Alertmanager secret created by the observability-operator Helm chart
	secretPredicate := predicates.NewAlertmanagerSecretPredicate(conf.Monitoring.AlertmanagerSecretName, conf.Namespace)

	// Filter only the Mimir Alertmanager pod
	podPredicate := predicates.NewAlertmanagerPodPredicate()

	// Requeue the Alertmanager secret when the Mimir Alertmanager pod changes
	p := podEventHandler(conf.Monitoring.AlertmanagerSecretName, conf.Namespace)

	// Setup the controller
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(secretPredicate)).
		Watches(&v1.Pod{}, p, builder.WithPredicates(podPredicate)).
		Complete(r)
}

// podEventHandler returns an event handler that enqueues requests for the Alertmanager secret only.
// For now there is only one Alertmanager secret to be reconciled.
func podEventHandler(secretName, namespace string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      secretName,
					Namespace: namespace,
				},
			},
		}
	})
}

// Reconcile main logic
func (r AlertmanagerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("controller", "alertmanager")
	log.IntoContext(ctx, logger)

	logger.Info("Started reconciling")
	defer logger.Info("Finished reconciling")

	// Retrieve the secret being reconciled
	secret := &v1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if !secret.DeletionTimestamp.IsZero() {
		// Nothing to do if the secret is being deleted
		// Configuration is not removed from Alertmanager when the secret is deleted.
		return ctrl.Result{}, nil
	}

	return r.reconcileCreate(ctx, secret)
}

// Handle create and update events
func (r AlertmanagerReconciler) reconcileCreate(ctx context.Context, secret *v1.Secret) (ctrl.Result, error) { // nolint: unparam
	// Ensure the configuration is set and up to date in Alertmanager
	err := r.alertmanagerJob.Configure(ctx, secret)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}
