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

	//TODO Check if orgID is present in the status
	// if not check the name availability (--> getOrg : if it returns nothing name is available)
	// if name is available create the organization

	logger.Info("Create organization in Grafana")
	_, err := grafanaAPI.Orgs.CreateOrg(&grafanaAPIModels.CreateOrgCommand{
		Name: grafanaOrganization.Name,
	})
	if err != nil {
		logger.Error(err, "Organization failed")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Add users to the organization")

	//TODO fetch orgID from above's response andd patch CR status with it

	// If orgID is present, check if it matches an existing org
	// if it does, check if the name is the same as the CR
	// if it is not, update the name of the grafana organization if it's available based on the display name

	// if orgId is present in the CR but no grafana org present, create the grafana org (after checking name availability)

	return ctrl.Result{}, nil
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
