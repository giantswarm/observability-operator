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

	"github.com/giantswarm/observability-operator/internal/mapper"
	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// DashboardReconciler reconciles a Dashboard object
type DashboardReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	grafanaURL       *url.URL
	finalizerHelper  FinalizerHelper
	dashboardMapper  *mapper.DashboardMapper
	grafanaClientGen grafanaclient.GrafanaClientGenerator
}

const (
	DashboardFinalizer = "observability.giantswarm.io/grafanadashboard"
	// TODO migrate to observability.giantswarm.io/kind
	DashboardSelectorLabelName  = "app.giantswarm.io/kind"
	DashboardSelectorLabelValue = "dashboard"
)

func SetupDashboardReconciler(mgr manager.Manager, conf config.Config, grafanaClientGen grafanaclient.GrafanaClientGenerator) error {
	r := &DashboardReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		grafanaURL:       conf.GrafanaURL,
		finalizerHelper:  NewFinalizerHelper(mgr.GetClient(), DashboardFinalizer),
		dashboardMapper:  mapper.New(),
		grafanaClientGen: grafanaClientGen,
	}

	return r.SetupWithManager(mgr)
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
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get dashboard configmap: %w", err)
		}

		return ctrl.Result{}, nil
	}

	grafanaAPI, err := r.grafanaClientGen.GenerateGrafanaClient(ctx, r.Client, r.grafanaURL)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate Grafana client: %w", err)
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

	err = ctrl.NewControllerManagedBy(mgr).
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
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

// reconcileCreate ensures the Grafana dashboard described in configmap is created in Grafana.
// This function is also responsible for:
// - Adding the finalizer to the configmap
func (r DashboardReconciler) reconcileCreate(ctx context.Context, grafanaService *grafana.Service, dashboard *v1.ConfigMap) error { // nolint:unparam
	logger := log.FromContext(ctx)

	// Add finalizer first if not set to avoid the race condition between init and delete.
	finalizerAdded, err := r.finalizerHelper.EnsureAdded(ctx, dashboard)
	if err != nil {
		return fmt.Errorf("failed to ensure finalizer is added: %w", err)
	}
	if finalizerAdded {
		return nil
	}

	// Convert ConfigMap to domain objects using mapper
	dashboards := r.dashboardMapper.FromConfigMap(dashboard)

	// Defensive validation: Ensure dashboards are valid even if webhook was bypassed
	for _, dash := range dashboards {
		if validationErrors := dash.Validate(); len(validationErrors) > 0 {
			logger.Error(nil, "Dashboard validation failed during reconciliation - webhook may have been bypassed",
				"dashboard", dash.UID(), "organization", dash.Organization(), "errors", validationErrors,
				"configmap", dashboard.Name, "namespace", dashboard.Namespace)
			return fmt.Errorf("dashboard validation failed for uid %s: %v", dash.UID(), validationErrors)
		}
	}

	// Process each dashboard
	for _, dashboard := range dashboards {
		logger.Info("Configuring dashboard", "uid", dashboard.UID(), "organization", dashboard.Organization())
		err = grafanaService.ConfigureDashboard(ctx, dashboard)
		if err != nil {
			return fmt.Errorf("failed to configure dashboard: %w", err)
		}
		logger.Info("Configured dashboard in Grafana", "uid", dashboard.UID(), "organization", dashboard.Organization())
	}

	return nil
}

// reconcileDelete deletes the grafana dashboard.
func (r DashboardReconciler) reconcileDelete(ctx context.Context, grafanaService *grafana.Service, dashboard *v1.ConfigMap) error {
	// We do not need to delete anything if there is no finalizer on the grafana dashboard
	if !controllerutil.ContainsFinalizer(dashboard, DashboardFinalizer) {
		return nil
	}

	// Convert ConfigMap to domain objects using mapper
	dashboards := r.dashboardMapper.FromConfigMap(dashboard)
	for _, dashboard := range dashboards {
		err := grafanaService.DeleteDashboard(ctx, dashboard)
		if err != nil {
			return fmt.Errorf("failed to delete dashboard: %w", err)
		}
	}

	// Finalizer handling needs to come last.
	err := r.finalizerHelper.EnsureRemoved(ctx, dashboard)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}
