package controller

import (
	"cmp"
	"context"
	stderrors "errors"
	"fmt"
	"slices"

	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
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
	"github.com/giantswarm/observability-operator/pkg/rules"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	GrafanaAPI *grafanaAPI.GrafanaHTTPAPI
	// AlloyRulesService is the service used to configure the alloy-rules instance.
	AlloyRulesService rules.Service
}

func SetupGrafanaOrganizationReconciler(mgr manager.Manager, conf config.Config) error {
	grafanaAPI, err := grafanaclient.GenerateGrafanaClient(conf.GrafanaURL, conf)
	if err != nil {
		return fmt.Errorf("unable to create grafana client: %w", err)
	}

	alloyRulesService := rules.Service{
		Client:          mgr.GetClient(),
		AlloyAppVersion: conf.AlloyAppVersion,
	}

	r := &GrafanaOrganizationReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		GrafanaAPI:        grafanaAPI,
		AlloyRulesService: alloyRulesService,
	}

	err = r.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	return nil
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
	err := r.Client.Get(ctx, req.NamespacedName, grafanaOrganization)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	// Handle deleted grafana organizations
	if !grafanaOrganization.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, grafanaOrganization)
	}

	// Handle non-deleted grafana organizations
	return r.reconcileCreate(ctx, grafanaOrganization)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
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
}

// reconcileCreate creates the grafanaOrganization.
// reconcileCreate ensures the Grafana organization described in grafanaOrganization CR is created in Grafana.
// This function is also responsible for:
// - Adding the finalizer to the CR
// - Updating the CR status field
// - Renaming the Grafana Main Org.
func (r GrafanaOrganizationReconciler) reconcileCreate(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) (ctrl.Result, error) { // nolint:unparam
	logger := log.FromContext(ctx)

	// Add finalizer first if not set to avoid the race condition between init and delete.
	if !controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		// We use a patch rather than an update to avoid conflicts when multiple controllers are adding their finalizer to the grafana organization
		// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
		logger.Info("adding finalizer", "finalizer", v1alpha1.GrafanaOrganizationFinalizer)
		patchHelper, err := patch.NewHelper(grafanaOrganization, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		controllerutil.AddFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer)
		if err := patchHelper.Patch(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "failed to add finalizer", "finalizer", v1alpha1.GrafanaOrganizationFinalizer)
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("added finalizer", "finalizer", v1alpha1.GrafanaOrganizationFinalizer)
		return ctrl.Result{}, nil
	}

	// Configure the organization in Grafana
	if err := r.configureOrganization(ctx, grafanaOrganization); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	var errs []error

	// Update the datasources in the CR's status
	if err := r.configureDatasources(ctx, grafanaOrganization); err != nil {
		errs = append(errs, err)
	}

	// Configure Grafana RBAC
	if err := r.configureGrafanaSSO(ctx); err != nil {
		errs = append(errs, err)
	}

	// Configure up the alloy-rules app
	if err := r.AlloyRulesService.Configure(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errors.WithStack(stderrors.Join(errs...))
	}

	return ctrl.Result{}, nil
}

func newOrganization(grafanaOrganization *v1alpha1.GrafanaOrganization) grafana.Organization {
	tenantIDs := make([]string, len(grafanaOrganization.Spec.Tenants))
	for i, tenant := range grafanaOrganization.Spec.Tenants {
		tenantIDs[i] = string(tenant)
	}

	orgID := grafanaOrganization.Status.OrgID
	// Shared Org is the only exception to the rule as we know it's ID will always be 1
	if grafanaOrganization.Spec.DisplayName == grafana.SharedOrg.Name {
		orgID = grafana.SharedOrg.ID
	}

	return grafana.Organization{
		ID:        orgID,
		Name:      grafanaOrganization.Spec.DisplayName,
		TenantIDs: tenantIDs,
		Admins:    grafanaOrganization.Spec.RBAC.Admins,
		Editors:   grafanaOrganization.Spec.RBAC.Editors,
		Viewers:   grafanaOrganization.Spec.RBAC.Viewers,
	}
}

func (r GrafanaOrganizationReconciler) configureOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)
	// Create or update organization in Grafana
	var organization = newOrganization(grafanaOrganization)
	err := grafana.UpsertOrganization(ctx, r.GrafanaAPI, &organization)
	if err != nil {
		logger.Error(err, "failed to upsert grafanaOrganization")
		return errors.WithStack(err)
	}

	// Update CR status if anything was changed
	if grafanaOrganization.Status.OrgID != organization.ID {
		logger.Info("updating orgID in the grafanaOrganization status")
		grafanaOrganization.Status.OrgID = organization.ID

		if err = r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "failed to update grafanaOrganization status")
			return errors.WithStack(err)
		}
		logger.Info("updated orgID in the grafanaOrganization status")
	}

	return nil
}

func (r GrafanaOrganizationReconciler) configureDatasources(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	logger.Info("configuring data sources")

	// Create or update organization in Grafana
	var organization = newOrganization(grafanaOrganization)
	datasources, err := grafana.ConfigureDefaultDatasources(ctx, r.GrafanaAPI, organization)
	if err != nil {
		logger.Error(err, "failed to configure the grafanaOrganization with default datasources")
		return errors.WithStack(err)
	}

	var configuredDatasources = make([]v1alpha1.DataSource, len(datasources))
	for i, datasource := range datasources {
		configuredDatasources[i] = v1alpha1.DataSource{
			ID:   datasource.ID,
			Name: datasource.Name,
		}
	}

	logger.Info("updating datasources in the grafanaOrganization status")
	grafanaOrganization.Status.DataSources = configuredDatasources
	if err := r.Status().Update(ctx, grafanaOrganization); err != nil {
		logger.Error(err, "failed to update the the grafanaOrganization status with datasources information")
		return errors.WithStack(err)
	}
	logger.Info("updated datasources in the grafanaOrganization status")
	logger.Info("configured data sources")

	return nil
}

// reconcileDelete deletes the grafana organization.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the grafana organization
	if !controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		return nil
	}

	// Delete organization in Grafana
	var organization = newOrganization(grafanaOrganization)

	// Delete organization in Grafana if it exists
	if grafanaOrganization.Status.OrgID > 0 {
		err := grafana.DeleteOrganization(ctx, r.GrafanaAPI, organization)
		if err != nil {
			return errors.WithStack(err)
		}

		grafanaOrganization.Status.OrgID = 0
		if err = r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "failed to update grafanaOrganization status")
			return errors.WithStack(err)
		}
	}

	var errs []error

	// Configure Grafana RBAC
	if err := r.configureGrafanaSSO(ctx); err != nil {
		errs = append(errs, err)
	}

	// Configure up the alloy-rules app
	if err := r.AlloyRulesService.Configure(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.WithStack(stderrors.Join(errs...))
	}

	// Finalizer handling needs to come last.
	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("removing finalizer", "finalizer", v1alpha1.GrafanaOrganizationFinalizer)
	patchHelper, err := patch.NewHelper(grafanaOrganization, r.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	controllerutil.RemoveFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer)
	if err := patchHelper.Patch(ctx, grafanaOrganization); err != nil {
		logger.Error(err, "failed to remove finalizer, requeuing", "finalizer", v1alpha1.GrafanaOrganizationFinalizer)
		return errors.WithStack(err)
	}
	logger.Info("removed finalizer", "finalizer", v1alpha1.GrafanaOrganizationFinalizer)

	return nil
}

// configureGrafana ensures the RBAC configuration is set in Grafana.
func (r *GrafanaOrganizationReconciler) configureGrafanaSSO(ctx context.Context) error {
	logger := log.FromContext(ctx)

	organizationList := v1alpha1.GrafanaOrganizationList{}
	err := r.Client.List(ctx, &organizationList)
	if err != nil {
		logger.Error(err, "failed to list grafana organizations")
		return errors.WithStack(err)
	}

	// Configure SSO settings in Grafana
	organizations := make([]grafana.Organization, len(organizationList.Items))
	for i, organization := range organizationList.Items {
		organizations[i] = newOrganization(&organization)
	}
	err = grafana.ConfigureSSOSettings(ctx, r.GrafanaAPI, organizations)
	if err != nil {
		logger.Error(err, "failed to configure grafanaOrganization with SSO settings")
		return errors.WithStack(err)
	}

	return nil
}
