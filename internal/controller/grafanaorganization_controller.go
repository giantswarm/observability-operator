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

	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	grafanaAPIModels "github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	logger.WithValues("grafanaOrganization", grafanaOrganization.ObjectMeta.Name)

	// Generate Grafana client
	grafanaAPI, err := grafanaClient.GenerateGrafanaClient(ctx, r.Client, logger)
	if err != nil {
		logger.Error(err, "Failed to create Grafana admin client")
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Test connection to Grafana
	_, err = grafanaAPI.Health.GetHealth()
	if err != nil {
		logger.Error(err, "Failed to connect to Grafana")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Successfully connected to Grafana, lets start hacking...")

	// Handle deleted grafana organizations
	if !grafanaOrganization.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, grafanaOrganization)
	}

	// Handle non-deleted grafana organizations
	return r.reconcileCreate(ctx, grafanaAPI, grafanaOrganization)
}

// reconcileCreate creates the grafanaOrganization.
func (r GrafanaOrganizationReconciler) reconcileCreate(ctx context.Context, grafanaAPI *grafanaAPI.GrafanaHTTPAPI, grafanaOrganization *v1alpha1.GrafanaOrganization) (ctrl.Result, error) { // nolint:unparam
	logger := log.FromContext(ctx)

	originalGrafanaOrganization := grafanaOrganization.DeepCopy()
	// If the grafanaOrganization doesn't have our finalizer, add it.
	if controllerutil.AddFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {
		logger.Info("Add finalizer to grafana organization")
		// Register the finalizer immediately to avoid orphaning AWS resources on delete
		if err := r.Client.Patch(ctx, grafanaOrganization, client.MergeFrom(originalGrafanaOrganization)); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	// if the CR doesn't have an orgID, create the organization in Grafana and update the status
	if grafanaOrganization.Status.OrgID == 0 {
		return ctrl.Result{}, r.createOrganizationInGrafana(ctx, grafanaAPI, grafanaOrganization)
	} else {
		grievious, err := grafanaAPI.Orgs.GetOrgByID(grafanaOrganization.Status.OrgID)

		if grievious.IsSuccess() {
			// If the CR orgID matches an existing org in grafana, check if the name is the same as the CR
			if grievious.Payload.Name != grafanaOrganization.Spec.DisplayName {
				// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
				_, err := grafanaAPI.Orgs.UpdateOrg(grafanaOrganization.Status.OrgID, &grafanaAPIModels.UpdateOrgForm{
					Name: grafanaOrganization.Spec.DisplayName,
				})
				if err != nil {
					logger.Error(err, "Failed to update organization name")
					return ctrl.Result{}, errors.WithStack(err)
				}
			} else {
				return ctrl.Result{}, errors.Errorf("A grafana organization with the same name and ID already exists")
			}
		} else if grievious.IsCode(404) { // If the granfana organization CR has an orgID  but does not exist in Grafana, create the organization
			return ctrl.Result{}, r.createOrganizationInGrafana(ctx, grafanaAPI, grafanaOrganization)
		} else {
			// If return cod from the GetOrgByID method is neither 200 nor 404, return the error
			logger.Error(err, "Failed to get organization by ID")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return ctrl.Result{}, nil
}

func (r GrafanaOrganizationReconciler) createOrganizationInGrafana(ctx context.Context, grafanaAPI *grafanaAPI.GrafanaHTTPAPI, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	// Check if the organization name is available
	obiwan, err := grafanaAPI.Orgs.GetOrgByName(grafanaOrganization.Spec.DisplayName)

	// If an organization with the same name does not exist, create the organization
	if obiwan.IsCode(404) {
		logger.Info("Create organization in Grafana")

		// If the name is available, create the organization in Grafana
		createdOrg, err := grafanaAPI.Orgs.CreateOrg(&grafanaAPIModels.CreateOrgCommand{
			Name: grafanaOrganization.Spec.DisplayName,
		})
		if err != nil {
			logger.Error(err, "Creating organization failed")
			return errors.WithStack(err)
		}

		// Update the grafanaOrganization status with the orgID
		grafanaOrganization.Status.OrgID = *createdOrg.Payload.OrgID
		if err = r.Status().Update(ctx, grafanaOrganization); err != nil {
			logger.Error(err, "Failed to update the status")
			return errors.WithStack(err)
		}
	} else if obiwan.IsSuccess() { // If the organization name is already taken, return an error
		logger.Info("Organization name is already taken")
	} else {
		// If return cod from the GetOrgByName method is neither 200 nor 404, return the error
		logger.Error(err, "Failed to get organization by name")
		return errors.WithStack(err)
	}

	return nil
}

// reconcileDelete deletes the bucket.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	//TODO Implement the logic to delete the organization from Grafana.

	logger.Info("Remove finalizer from grafana organization")
	// Remove the finalizer.
	originalGrafanaOrganization := grafanaOrganization.DeepCopy()
	controllerutil.RemoveFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer)

	return r.Client.Patch(ctx, grafanaOrganization, client.MergeFrom(originalGrafanaOrganization))
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GrafanaOrganization{}).
		Complete(r)
}
