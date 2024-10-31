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
	"fmt"

	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	grafanaAPIModels "github.com/grafana/grafana-openapi-client-go/models"
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

// reconcileCreate creates the grafanaOrganization.
func (r GrafanaOrganizationReconciler) reconcileCreate(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) (ctrl.Result, error) { // nolint:unparam
	logger := log.FromContext(ctx)

	// Add finalizer first if not set to avoid the race condition between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp is not set.
	if !controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		// We use a patch rather than an update to avoid conflicts when multiple controllers are adding their finalizer to the ClusterCR
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

	// Ensure the first organization is renamed.
	_, err := r.GrafanaAPI.Orgs.UpdateOrg(1, &grafanaAPIModels.UpdateOrgForm{
		Name: grafana.SharedOrgName,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to rename Main Org. to %s", grafana.SharedOrgName))
		return ctrl.Result{}, errors.WithStack(err)
	}

	// TODO add datasources for shared org.

	// Create or update organization in Grafana
	var organization grafana.Organization = grafana.Organization{
		ID:   grafanaOrganization.Status.OrgID,
		Name: grafanaOrganization.Spec.DisplayName,
	}

	if organization.ID == 0 {
		// if the CR doesn't have an orgID, create the organization in Grafana
		organization, err = grafana.CreateOrganization(ctx, r.GrafanaAPI, organization)
	} else {
		organization, err = grafana.UpdateOrganization(ctx, r.GrafanaAPI, organization)
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	// Update CR status if anything was changed
	// Update orgID in the CR's satus
	if grafanaOrganization.Status.OrgID != organization.ID {
		logger.Info("updating orgID in the org's status")
		grafanaOrganization.Status.OrgID = organization.ID

		if err := r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "failed to update the status")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	// Update the datasources in the CR's status
	if err = r.updateDatasourceInStatus(ctx, grafanaOrganization); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.configureGrafana(ctx)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func (r GrafanaOrganizationReconciler) updateDatasourceInStatus(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	if grafanaOrganization.Status.DataSources == nil {
		log.Log.Info("updating dataSources in the org's status")
		var datasources []v1alpha1.DataSources

		// Switch context to the current org
		orgGrafanaAPI := r.GrafanaAPI.WithOrgID(grafanaOrganization.Status.OrgID)

		createdDatasources, err := grafana.CreateDefaultDatasources(ctx, orgGrafanaAPI)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, createdDatasource := range createdDatasources {
			datasources = append(datasources, v1alpha1.DataSources{
				Name: createdDatasource.Name,
				ID:   createdDatasource.ID,
			})
		}

		grafanaOrganization.Status.DataSources = datasources
		if err := r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "failed to update the status")
			return errors.WithStack(err)
		}

		// Switch context back to default org
		r.GrafanaAPI = r.GrafanaAPI.WithOrgID(1)
	}

	return nil
}

// reconcileDelete deletes the grafana organization.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the cluster
	if controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {

		// Delete organization in Grafana if it exists
		if grafanaOrganization.Status.OrgID > 0 {
			err := grafana.DeleteByID(ctx, r.GrafanaAPI, grafanaOrganization.Status.OrgID)
			if err != nil {
				return errors.WithStack(err)
			}

			grafanaOrganization.Status.OrgID = 0
			if err = r.Status().Update(ctx, grafanaOrganization); err != nil {
				logger.Error(err, "failed to update the status")
				return errors.WithStack(err)
			}
		}

		err := r.configureGrafana(ctx)
		if err != nil {
			return errors.WithStack(err)
		}

		//TODO delete org's datasources

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
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GrafanaOrganization{}).
		// Watch for grafana pod's status changes
		Watches(
			&v1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				k8sClient := mgr.GetClient()
				var organizations v1alpha1.GrafanaOrganizationList

				err := k8sClient.List(ctx, &organizations)
				if err != nil {
					log.FromContext(ctx).Error(err, "failed to list grafana organization CRs")
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

func (r *GrafanaOrganizationReconciler) configureGrafana(ctx context.Context) error {
	logger := log.FromContext(ctx)

	organizations := v1alpha1.GrafanaOrganizationList{}
	err := r.Client.List(ctx, &organizations)
	if err != nil {
		logger.Error(err, "failed to list grafana organizations.")
		return errors.WithStack(err)
	}

	grafanaConfig := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana-user-values",
			Namespace: "giantswarm",
		},
	}

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, &grafanaConfig, func() error {
		config, err := templating.GenerateGrafanaConfiguration(organizations.Items)
		if err != nil {
			logger.Error(err, "failed to generate grafana user configmap values.")
			return errors.WithStack(err)
		}

		for _, organization := range organizations.Items {
			// Set owner reference to the config map to be able to clean it up when all organizations are deleted
			err = controllerutil.SetOwnerReference(&organization, &grafanaConfig, r.Scheme)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		logger.Info("configuring grafana", "config", config)

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
