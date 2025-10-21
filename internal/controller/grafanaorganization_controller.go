package controller

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/metrics"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	grafanaURL       *url.URL
	finalizerHelper  FinalizerHelper
	grafanaClientGen grafanaclient.GrafanaClientGenerator
	cfg              config.Config
}

func SetupGrafanaOrganizationReconciler(mgr manager.Manager, cfg config.Config, grafanaClientGen grafanaclient.GrafanaClientGenerator) error {
	r := &GrafanaOrganizationReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		grafanaURL:       cfg.Grafana.URL,
		finalizerHelper:  NewFinalizerHelper(mgr.GetClient(), v1alpha1.GrafanaOrganizationFinalizer),
		grafanaClientGen: grafanaClientGen,
		cfg:              cfg,
	}

	return r.SetupWithManager(mgr)
}

//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations/finalizers,verbs=update

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the GrafanaOrganization closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *GrafanaOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling Grafana Organization")
	defer logger.Info("Finished reconciling Grafana Organization")

	grafanaOrganization := &v1alpha1.GrafanaOrganization{}
	err := r.Get(ctx, req.NamespacedName, grafanaOrganization)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get GrafanaOrganization: %w", err)
		}

		return ctrl.Result{}, nil
	}

	grafanaAPI, err := r.grafanaClientGen.GenerateGrafanaClient(ctx, r.Client, r.grafanaURL)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate Grafana client: %w", err)
	}

	grafanaService := grafana.NewService(r.Client, grafanaAPI, r.cfg)

	// Handle deleted grafana organizations
	if !grafanaOrganization.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, grafanaService, grafanaOrganization)
	}

	// Handle non-deleted grafana organizations
	return r.reconcileCreate(ctx, grafanaService, grafanaOrganization)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("grafanaorganization").
		For(&v1alpha1.GrafanaOrganization{}).
		// Watch for grafana pod's status changes
		Watches(
			&v1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var logger = log.FromContext(ctx)
				var organizations v1alpha1.GrafanaOrganizationList

				err := mgr.GetClient().List(ctx, &organizations)
				if err != nil {
					logger.Error(err, "failed to list grafana organization CRs")
					return []reconcile.Request{}
				}

				// Sort organizations by orgID to ensure the order is deterministic.
				// This is important to prevent incorrect ordering of organizations on grafana restarts.
				slices.SortStableFunc(organizations.Items, func(i, j v1alpha1.GrafanaOrganization) int {
					// if both orgs have a nil orgID, they are equal
					// if one org has a nil orgID, it is higher than the other as it was not created in Grafana yet
					if i.Status.OrgID == 0 && j.Status.OrgID == 0 {
						return 0
					} else if i.Status.OrgID == 0 {
						return 1
					} else if j.Status.OrgID == 0 {
						return -1
					}
					return cmp.Compare(i.Status.OrgID, j.Status.OrgID)
				})

				// Reconcile all grafana organizations when the grafana pod is recreated
				requests := make([]reconcile.Request, 0, len(organizations.Items))
				for _, organization := range organizations.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: organization.Name,
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

// reconcileCreate ensures the Grafana organization described in GrafanaOrganization CR is created in Grafana.
// This function is also responsible for:
// - Adding the finalizer to the CR
// - Updating the CR status field
// - Renaming the Grafana Main Org.
func (r GrafanaOrganizationReconciler) reconcileCreate(ctx context.Context, grafanaService *grafana.Service, grafanaOrganization *v1alpha1.GrafanaOrganization) (ctrl.Result, error) { // nolint:unparam
	// Add finalizer first if not set to avoid the race condition between init and delete.
	finalizerAdded, err := r.finalizerHelper.EnsureAdded(ctx, grafanaOrganization)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure finalizer is added: %w", err)
	}
	if finalizerAdded {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	// Determine initial status based on current state
	orgStatus := metrics.OrgStatusPending
	if grafanaOrganization.Status.OrgID > 0 {
		orgStatus = metrics.OrgStatusActive
	}

	// Create or update the grafana organization
	updatedID, err := grafanaService.ConfigureOrganization(ctx, grafanaOrganization)
	if err != nil {
		// Set error status and update metric before returning
		orgStatus = metrics.OrgStatusError
		updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)
		return ctrl.Result{}, fmt.Errorf("failed to upsert grafanaOrganization: %w", err)
	}

	// Update CR status if anything was changed
	if grafanaOrganization.Status.OrgID != updatedID {
		logger.Info("updating orgID in the grafanaOrganization status")
		grafanaOrganization.Status.OrgID = updatedID

		err = r.Client.Status().Update(ctx, grafanaOrganization)
		if err != nil {
			orgStatus = metrics.OrgStatusError
			updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)
			return ctrl.Result{}, fmt.Errorf("failed to update grafanaOrganization status: %w", err)
		}
		orgStatus = metrics.OrgStatusActive
		logger.Info("updated orgID in the grafanaOrganization status")
	}

	// Configure the organization's datasources and authorization settings
	err = grafanaService.SetupOrganization(ctx, grafanaOrganization)
	if err != nil {
		orgStatus = metrics.OrgStatusError
		updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)
		return ctrl.Result{}, fmt.Errorf("failed to setup grafanaOrganization: %w", err)
	}

	// Set info metrics
	for _, tenant := range grafanaOrganization.Spec.Tenants {
		// for each tenant in the organization, set a metric with tenant name and org id
		metrics.GrafanaOrganizationTenantInfo.WithLabelValues(
			string(tenant),
			fmt.Sprintf("%d", grafanaOrganization.Status.OrgID),
		).Set(1)
	}

	updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)

	return ctrl.Result{}, nil
}

// reconcileDelete deletes the grafana organization.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaService *grafana.Service, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	// We do not need to delete anything if there is no finalizer on the grafana organization
	if !controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		return nil
	}

	// Store orgID before deletion for metric cleanup
	orgID := fmt.Sprintf("%d", grafanaOrganization.Status.OrgID)

	err := grafanaService.DeleteOrganization(ctx, grafanaOrganization)
	if err != nil {
		return fmt.Errorf("failed to delete grafana organization: %w", err)
	}

	grafanaOrganization.Status.OrgID = 0
	err = r.Client.Status().Update(ctx, grafanaOrganization)
	if err != nil {
		return fmt.Errorf("failed to update grafanaOrganization status: %w", err)
	}

	// Configure Grafana RBAC
	err = grafanaService.ConfigureGrafanaSSO(ctx)
	if err != nil {
		return fmt.Errorf("failed to configure Grafana SSO: %w", err)
	}

	// Clean up metrics - delete metric series for this organization
	for _, tenant := range grafanaOrganization.Spec.Tenants {
		metrics.GrafanaOrganizationTenantInfo.DeleteLabelValues(string(tenant), orgID)
	}
	metrics.GrafanaOrganizationInfo.DeleteLabelValues(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, orgID, metrics.OrgStatusActive)
	metrics.GrafanaOrganizationInfo.DeleteLabelValues(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, orgID, metrics.OrgStatusPending)
	metrics.GrafanaOrganizationInfo.DeleteLabelValues(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, orgID, metrics.OrgStatusError)

	// Finalizer handling needs to come last.
	err = r.finalizerHelper.EnsureRemoved(ctx, grafanaOrganization)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

func updateGrafanaOrganizationInfoMetric(organizationName string, displayName string, orgID int64, status string) {
	metrics.GrafanaOrganizationInfo.WithLabelValues(
		organizationName,
		displayName,
		fmt.Sprintf("%d", orgID),
		status,
	).Set(1)
}
