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
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	grafanaAPIModels "github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

var (
	//go:embed templates/grafana-user-values.yaml.template
	grafanaUserConfig         string
	grafanaUserConfigTemplate *template.Template
)

const (
	sharedOrgName     = "Shared Org."
	grafanaAdminRole  = "Admin"
	grafanaEditorRole = "Editor"
	grafanaViewerRole = "Viewer"
)

func init() {
	grafanaUserConfigTemplate = template.Must(template.New("grafana-user-values.yaml").Parse(grafanaUserConfig))
}

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

	_, err := grafanaAPI.Orgs.UpdateOrg(1, &grafanaAPIModels.UpdateOrgForm{
		Name: sharedOrgName,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("Could not rename Main Org. to %s", sharedOrgName))
		return ctrl.Result{}, errors.WithStack(err)
	}

	//TODO Implement the logic to create the Grafana organization

	err = r.configureOrgMapping(ctx)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

// reconcileDelete deletes the bucket.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the cluster
	if controllerutil.ContainsFinalizer(grafanaOrganization, v1alpha1.GrafanaOrganizationFinalizer) {

		//TODO Implement the logic to delete the organization from Grafana.

		err := r.configureOrgMapping(ctx)
		if err != nil {
			return errors.WithStack(err)
		}

		// We get the latest state of the object to avoid race conditions.
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
		Complete(r)
}

func (r GrafanaOrganizationReconciler) configureOrgMapping(ctx context.Context) error {
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
		Data: map[string]string{},
	}

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, &grafanaConfig, func() error {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\"*:%s:%s\"", sharedOrgName, grafanaAdminRole))
		for _, organization := range organizations.Items {
			rbac := organization.Spec.RBAC
			organizationName := organization.Spec.DisplayName
			for _, adminOrgAttribute := range rbac.Admins {
				buildOrgMapping(&sb, organizationName, adminOrgAttribute, grafanaAdminRole)
			}
			for _, editorOrgAttribute := range rbac.Editors {
				buildOrgMapping(&sb, organizationName, editorOrgAttribute, grafanaEditorRole)
			}
			for _, viewerOrgAttribute := range rbac.Viewers {
				buildOrgMapping(&sb, organizationName, viewerOrgAttribute, grafanaViewerRole)
			}
			// Set owner reference to the config map to be able to clean it up when all organizations are deleted
			err = controllerutil.SetOwnerReference(&organization, &grafanaConfig, r.Scheme)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		orgMapping := sb.String()
		logger.Info("configuring org mapping", "orgMapping", orgMapping)

		data := struct {
			OrgMapping string
		}{
			OrgMapping: orgMapping,
		}

		var values bytes.Buffer
		err = grafanaUserConfigTemplate.Execute(&values, data)
		if err != nil {
			return errors.WithStack(err)
		}

		grafanaConfig.Data = make(map[string]string)
		grafanaConfig.Data["values"] = values.String()

		return nil
	})
	if err != nil {
		logger.Error(err, "failed to generate grafana user configmap values.")
		return errors.WithStack(err)
	}

	return nil
}

func buildOrgMapping(sb *strings.Builder, organizationName, userOrgAttribute, role string) {
	// We preprend with a space and we add double quotes to the org mapping to support spaces in display names
	sb.WriteString(" \"")
	// We need to escape the colon in the userOrgAttribute
	sb.WriteString(strings.ReplaceAll(userOrgAttribute, ":", "\\:"))
	sb.WriteString(":")
	sb.WriteString(organizationName)
	sb.WriteString(":")
	sb.WriteString(role)
	sb.WriteString("\"")
}
