/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"slices"
	"strings"

	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/internal/controller/predicates"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	"github.com/giantswarm/observability-operator/pkg/grafana/templating"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	GrafanaAPI *grafanaAPI.GrafanaHTTPAPI
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

	// Configure the shared organization in Grafana
	if err := r.configureSharedOrg(ctx); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Configure the organization in Grafana
	if err := r.configureOrganization(ctx, grafanaOrganization); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Update the datasources in the CR's status
	if err := r.configureDatasources(ctx, grafanaOrganization); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Configure Grafana RBAC
	if err := r.configureGrafana(ctx); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func (r GrafanaOrganizationReconciler) configureSharedOrg(ctx context.Context) error {
	logger := log.FromContext(ctx)

	sharedOrg := grafana.SharedOrg

	logger.Info("configuring shared organization")
	if _, err := grafana.UpdateOrganization(ctx, r.GrafanaAPI, sharedOrg); err != nil {
		logger.Error(err, "failed to rename shared org")
		return errors.WithStack(err)
	}

	if _, err := grafana.ConfigureDefaultDatasources(ctx, r.GrafanaAPI, grafana.SharedOrg); err != nil {
		logger.Info("failed to configure datasources for shared org")
		return errors.WithStack(err)
	}

	logger.Info("configured shared org")
	return nil
}

func (r GrafanaOrganizationReconciler) configureOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) (err error) {
	logger := log.FromContext(ctx)
	// Create or update organization in Grafana
	var organization = grafana.Organization{
		ID:       grafanaOrganization.Status.OrgID,
		Name:     grafanaOrganization.Spec.DisplayName,
		TenantID: grafanaOrganization.Name,
	}

	if organization.ID == 0 {
		// if the CR doesn't have an orgID, create the organization in Grafana
		organization, err = grafana.CreateOrganization(ctx, r.GrafanaAPI, organization)
	} else {
		organization, err = grafana.UpdateOrganization(ctx, r.GrafanaAPI, organization)
	}

	if err != nil {
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
	var organization = grafana.Organization{
		ID:       grafanaOrganization.Status.OrgID,
		Name:     grafanaOrganization.Spec.DisplayName,
		TenantID: grafanaOrganization.Name,
	}

	datasources, err := grafana.ConfigureDefaultDatasources(ctx, r.GrafanaAPI, organization)
	if err != nil {
		return errors.WithStack(err)
	}

	var desiredDatasources = make([]v1alpha1.DataSource, len(datasources))
	for i, datasource := range datasources {
		desiredDatasources[i] = v1alpha1.DataSource{
			ID:   datasource.ID,
			Name: datasource.Name,
		}
	}

	logger.Info("updating datasources in the grafanaOrganization status")
	grafanaOrganization.Status.DataSources = desiredDatasources
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

	// Delete organization in Grafana if it exists
	if grafanaOrganization.Status.OrgID > 0 {
		err := grafana.DeleteByID(ctx, r.GrafanaAPI, grafanaOrganization.Status.OrgID)
		if err != nil {
			return errors.WithStack(err)
		}

		grafanaOrganization.Status.OrgID = 0
		if err = r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "failed to update grafanaOrganization status")
			return errors.WithStack(err)
		}
	}

	err := r.configureGrafana(ctx)
	if err != nil {
		return errors.WithStack(err)
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
func (r *GrafanaOrganizationReconciler) configureGrafana(ctx context.Context) error {
	logger := log.FromContext(ctx)

	organizationList := v1alpha1.GrafanaOrganizationList{}
	err := r.Client.List(ctx, &organizationList)
	if err != nil {
		logger.Error(err, "failed to list grafana organizations.")
		return errors.WithStack(err)
	}

	grafanaConfig := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana-user-values",
			Namespace: "giantswarm",
		},
	}

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, grafanaConfig, func() error {
		organizations := organizationList.Items
		// We always sort the organizations to ensure the order is deterministic and the configmap is stable
		slices.SortFunc(organizations, func(i, j v1alpha1.GrafanaOrganization) int {
			return strings.Compare(i.Name, j.Name)
		})

		config, err := templating.GenerateGrafanaConfiguration(organizations)
		if err != nil {
			logger.Error(err, "failed to generate grafana user configmap values.")
			return errors.WithStack(err)
		}

		for _, organization := range organizations {
			// Set owner reference to the config map to be able to clean it up when all organizations are deleted
			err = controllerutil.SetOwnerReference(&organization, grafanaConfig, r.Scheme)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		logger.Info("updating grafana-user-values", "config", config)

		grafanaConfig.Data = make(map[string]string)
		grafanaConfig.Data["values"] = config

		return nil
	})

	if err != nil {
		logger.Error(err, "failed to configure grafana.")
		return errors.WithStack(err)
	}

	return nil
}
