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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/grafana"
)

const sharedOrgName = "Shared Org."

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

	originalGrafanaOrganization := grafanaOrganization.DeepCopy()
	// If the grafanaOrganization doesn't have our finalizer, add it.
	if controllerutil.AddFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		logger.Info("Add finalizer to Grafana Organization")
		// Register the finalizer immediately to avoid orphaning AWS resources on delete
		if err := r.Client.Patch(ctx, grafanaOrganization, client.MergeFrom(originalGrafanaOrganization)); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	// Ensure the first organization is renamed.
	_, err := r.GrafanaAPI.Orgs.UpdateOrg(1, &grafanaAPIModels.UpdateOrgForm{
		Name: sharedOrgName,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("Could not rename Main Org. to %s", sharedOrgName))
		return ctrl.Result{}, errors.WithStack(err)
	}

	var organization *grafana.Organization
	if grafanaOrganization.Status.OrgID == 0 {
		// if the CR doesn't have an orgID, create the organization in Grafana and update the status
		organization, err = grafana.CreateOrganization(ctx, r.GrafanaAPI, grafanaOrganization)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		organization, err = grafana.UpdateOrganization(ctx, r.GrafanaAPI, grafanaOrganization)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if organization != nil && organization.ID != grafanaOrganization.Status.OrgID {
		grafanaOrganization.Status.OrgID = organization.ID

		if err = r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "Failed to update the status")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return ctrl.Result{}, nil
}

// reconcileDelete deletes the grafana organization.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	//TODO Implement the logic to delete the organization from Grafana.

	logger.Info("Remove finalizer from grafana organization")
	// Remove the finalizer.
	originalGrafanaOrganization := grafanaOrganization.DeepCopy()
	if controllerutil.RemoveFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		err := r.Client.Patch(ctx, grafanaOrganization, client.MergeFrom(originalGrafanaOrganization))
		if err != nil {
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GrafanaOrganization{}).
		Complete(r)
}
