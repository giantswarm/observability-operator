package controller

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sync"

	v1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/internal/mapper"
	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/metrics"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	grafanaURL         *url.URL
	finalizerHelper    FinalizerHelper
	grafanaClientGen   grafanaclient.GrafanaClientGenerator
	cfg                config.Config
	organizationMapper *mapper.OrganizationMapper

	// ssoMu serializes SSO configuration updates to prevent concurrent reconciles
	// from clobbering each other's SSO org_mapping writes.
	ssoMu sync.Mutex
}

func SetupGrafanaOrganizationReconciler(mgr manager.Manager, cfg config.Config, grafanaClientGen grafanaclient.GrafanaClientGenerator) error {
	r := &GrafanaOrganizationReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		grafanaURL:         cfg.Grafana.URL,
		finalizerHelper:    NewFinalizerHelper(mgr.GetClient(), v1alpha2.GrafanaOrganizationFinalizer),
		grafanaClientGen:   grafanaClientGen,
		cfg:                cfg,
		organizationMapper: mapper.NewOrganizationMapper(),
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

	grafanaOrganization := &v1alpha2.GrafanaOrganization{}
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

	grafanaService := grafana.NewService(grafanaAPI, r.cfg)

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
		For(&v1alpha2.GrafanaOrganization{}).
		// Watch for grafana pod's status changes
		Watches(
			&v1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				logger := log.FromContext(ctx)
				var organizations v1alpha2.GrafanaOrganizationList

				err := mgr.GetClient().List(ctx, &organizations)
				if err != nil {
					logger.Error(err, "failed to list grafana organization CRs")
					return []reconcile.Request{}
				}

				// Sort organizations by orgID to ensure the order is deterministic.
				// This is important to prevent incorrect ordering of organizations on grafana restarts.
				slices.SortStableFunc(organizations.Items, func(i, j v1alpha2.GrafanaOrganization) int {
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
func (r *GrafanaOrganizationReconciler) reconcileCreate(ctx context.Context, grafanaService *grafana.Service, grafanaOrganization *v1alpha2.GrafanaOrganization) (ctrl.Result, error) { // nolint:unparam
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

	// Convert to domain object
	organization := r.organizationMapper.FromGrafanaOrganization(grafanaOrganization)

	// Create or update the grafana organization
	updatedID, err := grafanaService.ConfigureOrganization(ctx, organization)
	if err != nil {
		// Set error status and update metric before returning
		orgStatus = metrics.OrgStatusError
		updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)

		apimeta.SetStatusCondition(&grafanaOrganization.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: grafanaOrganization.Generation,
			Reason:             "ConfigureOrganizationFailed",
			Message:            err.Error(),
		})
		if statusErr := r.Client.Status().Update(ctx, grafanaOrganization); statusErr != nil {
			logger.Error(statusErr, "failed to update status conditions after ConfigureOrganization error")
		}

		return ctrl.Result{}, fmt.Errorf("failed to configure grafanaOrganization: %w", err)
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

	// Collect all errors to ensure all independent tasks have a chance to run
	var errs []error

	// Configure the organization's datasources and handle status updates
	datasources, err := grafanaService.ConfigureDatasources(ctx, organization)
	if err != nil {
		logger.Error(err, "failed to configure datasources")
		errs = append(errs, fmt.Errorf("configure datasources: %w", err))
	} else {
		// Build the list of configured datasources for the status
		configuredDatasources := make([]v1alpha2.DataSource, len(datasources))
		for i, datasource := range datasources {
			configuredDatasources[i] = v1alpha2.DataSource{
				ID:   datasource.ID,
				Name: datasource.Name,
			}
		}

		// Sort the datasources by ID to ensure consistent ordering
		slices.SortStableFunc(configuredDatasources, func(a, b v1alpha2.DataSource) int {
			return cmp.Compare(a.ID, b.ID)
		})

		// Update the status if the datasources have changed
		if !slices.Equal(grafanaOrganization.Status.DataSources, configuredDatasources) {
			logger.Info("updating datasources in the GrafanaOrganization status")
			grafanaOrganization.Status.DataSources = configuredDatasources
			if err := r.Client.Status().Update(ctx, grafanaOrganization); err != nil {
				logger.Error(err, "failed to update GrafanaOrganization datasources status")
				errs = append(errs, fmt.Errorf("update datasources status: %w", err))
			} else {
				logger.Info("updated datasources in the GrafanaOrganization status")
			}
		}
	}

	err = r.configureGrafanaSSOSettings(ctx, grafanaService)
	if err != nil {
		logger.Error(err, "failed to configure SSO settings")
		errs = append(errs, fmt.Errorf("configure SSO settings: %w", err))
	}

	// If any errors occurred, combine them and return
	if len(errs) > 0 {
		orgStatus = metrics.OrgStatusError
		updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)

		combinedErr := errors.Join(errs...)
		apimeta.SetStatusCondition(&grafanaOrganization.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: grafanaOrganization.Generation,
			Reason:             "ReconciliationFailed",
			Message:            combinedErr.Error(),
		})
		if statusErr := r.Client.Status().Update(ctx, grafanaOrganization); statusErr != nil {
			logger.Error(statusErr, "failed to update status conditions after error")
		}

		return ctrl.Result{}, combinedErr
	}

	// Set Ready condition
	apimeta.SetStatusCondition(&grafanaOrganization.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: grafanaOrganization.Generation,
		Reason:             "ReconciliationSucceeded",
		Message:            "Grafana organization is fully configured",
	})
	if err := r.Client.Status().Update(ctx, grafanaOrganization); err != nil {
		logger.Error(err, "failed to update Ready status condition")
	}

	// Set info metrics
	for _, tenant := range grafanaOrganization.Spec.Tenants {
		// for each tenant in the organization, set a metric with tenant name and org id
		metrics.GrafanaOrganizationTenantInfo.WithLabelValues(
			string(tenant.Name),
			fmt.Sprintf("%d", grafanaOrganization.Status.OrgID),
		).Set(1)
	}

	updateGrafanaOrganizationInfoMetric(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, grafanaOrganization.Status.OrgID, orgStatus)

	return ctrl.Result{}, nil
}

// listActiveGrafanaOrganizations retrieves all GrafanaOrganization CRs and converts them to domain objects
func (r *GrafanaOrganizationReconciler) listActiveGrafanaOrganizations(ctx context.Context) ([]*organization.Organization, error) {
	organizationList := &v1alpha2.GrafanaOrganizationList{}
	err := r.Client.List(ctx, organizationList)
	if err != nil {
		return nil, fmt.Errorf("failed to list grafana organizations: %w", err)
	}

	organizations := make([]*organization.Organization, 0, len(organizationList.Items))
	for _, org := range organizationList.Items {
		if !org.GetDeletionTimestamp().IsZero() {
			// Skip organizations that are being deleted
			// see https://github.com/giantswarm/observability-operator/pull/525
			continue
		}
		organizations = append(organizations, r.organizationMapper.FromGrafanaOrganization(&org))
	}

	return organizations, nil
}

// configureGrafanaSSOSettings configures Grafana SSO settings based on all active GrafanaOrganizations.
// It uses a mutex to serialize concurrent SSO updates and prevent last-write-wins races
// when multiple GrafanaOrganization reconciles happen in parallel.
func (r *GrafanaOrganizationReconciler) configureGrafanaSSOSettings(ctx context.Context, grafanaService *grafana.Service) error {
	r.ssoMu.Lock()
	defer r.ssoMu.Unlock()

	allOrganizations, err := r.listActiveGrafanaOrganizations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all organizations for SSO configuration: %w", err)
	}

	err = grafanaService.ConfigureSSOSettings(ctx, allOrganizations)
	if err != nil {
		return fmt.Errorf("failed to configure SSO: %w", err)
	}
	return nil
}

// reconcileDelete deletes the grafana organization.
func (r *GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaService *grafana.Service, grafanaOrganization *v1alpha2.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the grafana organization
	if !controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha2.GrafanaOrganizationFinalizer) {
		return nil
	}

	// Store orgID before deletion for metric cleanup
	orgID := fmt.Sprintf("%d", grafanaOrganization.Status.OrgID)

	// Convert to domain object
	organization := r.organizationMapper.FromGrafanaOrganization(grafanaOrganization)

	// Collect all errors to ensure all cleanup tasks have a chance to run
	var errs []error

	err := grafanaService.DeleteOrganization(ctx, organization)
	if err != nil {
		logger.Error(err, "failed to delete grafana organization")
		errs = append(errs, fmt.Errorf("delete organization: %w", err))
	}

	grafanaOrganization.Status.OrgID = 0
	err = r.Client.Status().Update(ctx, grafanaOrganization)
	if err != nil {
		logger.Error(err, "failed to update grafanaOrganization status")
		errs = append(errs, fmt.Errorf("update status: %w", err))
	}

	err = r.configureGrafanaSSOSettings(ctx, grafanaService)
	if err != nil {
		logger.Error(err, "failed to configure SSO settings after deletion")
		errs = append(errs, fmt.Errorf("configure SSO settings: %w", err))
	}

	// Clean up metrics - delete metric series for this organization
	for _, tenant := range grafanaOrganization.Spec.Tenants {
		metrics.GrafanaOrganizationTenantInfo.DeleteLabelValues(string(tenant.Name), orgID)
	}
	metrics.GrafanaOrganizationInfo.DeleteLabelValues(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, orgID, metrics.OrgStatusActive)
	metrics.GrafanaOrganizationInfo.DeleteLabelValues(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, orgID, metrics.OrgStatusPending)
	metrics.GrafanaOrganizationInfo.DeleteLabelValues(grafanaOrganization.Name, grafanaOrganization.Spec.DisplayName, orgID, metrics.OrgStatusError)

	// If any errors occurred during deletion, combine them and return
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

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
