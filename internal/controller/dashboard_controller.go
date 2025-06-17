package controller

import (
	"context"
	"fmt"
	"net/url"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"

	"github.com/giantswarm/observability-operator/internal/predicates"
)

// DashboardReconciler reconciles a Dashboard object
type DashboardReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	grafanaURL      *url.URL
	finalizerHelper FinalizerHelper
}

const (
	DashboardFinalizer = "observability.giantswarm.io/grafanadashboard"
	// TODO migrate to observability.giantswarm.io/kind
	DashboardSelectorLabelName  = "app.giantswarm.io/kind"
	DashboardSelectorLabelValue = "dashboard"
)

func SetupDashboardReconciler(mgr manager.Manager, conf config.Config) error {
	r := &DashboardReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		grafanaURL:      conf.GrafanaURL,
		finalizerHelper: NewFinalizerHelper(mgr.GetClient(), DashboardFinalizer),
	}

	err := r.SetupWithManager(mgr)
	if err != nil {
		return fmt.Errorf("failed to setup dashboard controller with manager: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the Dashboard closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *DashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling Grafana Dashboard Configmaps")
	defer logger.Info("Finished reconciling Grafana Dashboard Configmaps")

	dashboard := &v1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, dashboard)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	grafanaAPI, err := grafanaclient.GenerateGrafanaClient(ctx, r.Client, r.grafanaURL)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate grafana client: %w", err)
	}

	grafanaService := grafana.NewService(r.Client, grafanaAPI)

	// Handle deleted grafana dashboards
	if !dashboard.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, grafanaService, dashboard)
	}

	// Handle non-deleted grafana dashboards
	return ctrl.Result{}, r.reconcileCreate(ctx, grafanaService, dashboard)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelectorPredicate, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				DashboardSelectorLabelName: DashboardSelectorLabelValue,
			},
		})
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("dashboard").
		For(&v1.ConfigMap{}, builder.WithPredicates(labelSelectorPredicate)).
		// Watch for grafana pod's status changes
		Watches(
			&v1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var logger = log.FromContext(ctx)
				var dashboards v1.ConfigMapList

				err := mgr.GetClient().List(ctx, &dashboards, client.MatchingLabels{DashboardSelectorLabelName: DashboardSelectorLabelValue})
				if err != nil {
					logger.Error(err, "failed to list grafana dashboard configmaps")
					return []reconcile.Request{}
				}

				// Reconcile all grafana dashboards when the grafana pod is recreated
				requests := make([]reconcile.Request, 0, len(dashboards.Items))
				for _, dashboard := range dashboards.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      dashboard.Name,
							Namespace: dashboard.Namespace,
						},
					})
				}
				return requests
			}),
			builder.WithPredicates(predicates.GrafanaPodRecreatedPredicate{}),
		).
		Complete(r)
}

// reconcileCreate creates the dashboard.
// reconcileCreate ensures the Grafana dashboard described in configmap is created in Grafana.
// This function is also responsible for:
// - Adding the finalizer to the configmap
func (r DashboardReconciler) reconcileCreate(ctx context.Context, grafanaService *grafana.Service, dashboard *v1.ConfigMap) error { // nolint:unparam
	// Add finalizer first if not set to avoid the race condition between init and delete.
	finalizerAdded, err := r.finalizerHelper.EnsureAdded(ctx, dashboard)
	if err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}
	if finalizerAdded {
		return nil
	}

	// Configure the dashboard in Grafana
	err = grafanaService.ConfigureDashboard(ctx, dashboard)
	if err != nil {
		return fmt.Errorf("failed to configure dashboard in grafana: %w", err)
	}

	return nil
}

// reconcileDelete deletes the grafana dashboard.
func (r DashboardReconciler) reconcileDelete(ctx context.Context, grafanaService *grafana.Service, dashboard *v1.ConfigMap) error {
	// We do not need to delete anything if there is no finalizer on the grafana dashboard
	if !controllerutil.ContainsFinalizer(dashboard, DashboardFinalizer) {
		return nil
	}

	// Unconfigure the dashboard in Grafana
	err := grafanaService.DeleteDashboard(ctx, dashboard)
	if err != nil {
		return fmt.Errorf("failed to delete dashboard from grafana: %w", err)
	}

	// Finalizer handling needs to come last.
	err = r.finalizerHelper.EnsureRemoved(ctx, dashboard)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}
