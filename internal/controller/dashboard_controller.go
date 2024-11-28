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
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-openapi/strfmt"
	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/models"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"

	"github.com/giantswarm/observability-operator/internal/controller/predicates"
)

// DashboardReconciler reconciles a Dashboard object
type DashboardReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	GrafanaAPI *grafanaAPI.GrafanaHTTPAPI
}

const (
	DashboardFinalizer      = "observability.giantswarm.io/grafanadashboard"
	DashboardTeamAnnotation = "giantswarm.io/organization"
)

func SetupDashboardReconciler(mgr manager.Manager, conf config.Config) error {
	// Generate Grafana client
	// Get grafana admin-password and admin-user
	grafanaAdminCredentials := grafanaclient.AdminCredentials{
		Username: conf.Environment.GrafanaAdminUsername,
		Password: conf.Environment.GrafanaAdminPassword,
	}
	if grafanaAdminCredentials.Username == "" {
		return fmt.Errorf("GrafanaAdminUsername not set: %q", conf.Environment.GrafanaAdminUsername)
	}
	if grafanaAdminCredentials.Password == "" {
		return fmt.Errorf("GrafanaAdminPassword not set: %q", conf.Environment.GrafanaAdminPassword)
	}

	grafanaTLSConfig := grafanaclient.TLSConfig{
		Cert: conf.Environment.GrafanaTLSCertFile,
		Key:  conf.Environment.GrafanaTLSKeyFile,
	}
	grafanaAPI, err := grafanaclient.GenerateGrafanaClient(conf.GrafanaURL, grafanaAdminCredentials, grafanaTLSConfig)
	if err != nil {
		return fmt.Errorf("unable to create grafana client: %w", err)
	}

	r := &DashboardReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		GrafanaAPI: grafanaAPI,
	}

	err = r.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps/status,verbs=get;update;patch
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
	err := r.Client.Get(ctx, req.NamespacedName, dashboard)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	// Handle deleted grafana dashboards
	if !dashboard.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, dashboard)
	}

	// Handle non-deleted grafana dashboards
	return r.reconcileCreate(ctx, dashboard)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {

	labelSelectorPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{"app.giantswarm.io/kind": "dashboard"}})
	if err != nil {
		return errors.WithStack(err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(labelSelectorPredicate)).
		// Watch for grafana pod's status changes
		Watches(
			&v1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var logger = log.FromContext(ctx)
				var dashboards v1.ConfigMapList

				err := mgr.GetClient().List(ctx, &dashboards, client.MatchingLabels{"app.giantswarm.io/kind": "dashboard"})
				if err != nil {
					logger.Error(err, "failed to list grafana dashboard configmaps")
					return []reconcile.Request{}
				}

				// Reconcile all grafana dashboards when the grafana pod is recreated
				requests := make([]reconcile.Request, 0, len(dashboards.Items))
				for _, dashboard := range dashboards.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: dashboard.Name,
						},
					})
				}
				return requests
			}),
			builder.WithPredicates(predicates.GrafanaPodRecreatedPredicate{}),
		).
		Complete(r)
}

// reconcileCreate creates the dashboard.
// reconcileCreate ensures the Grafana dashboard described in configmap is created in Grafana.
// This function is also responsible for:
// - Adding the finalizer to the configmap
func (r DashboardReconciler) reconcileCreate(ctx context.Context, dashboard *v1.ConfigMap) (ctrl.Result, error) { // nolint:unparam
	logger := log.FromContext(ctx)

	// Add finalizer first if not set to avoid the race condition between init and delete.
	if !controllerutil.ContainsFinalizer(dashboard, DashboardFinalizer) {
		// We use a patch rather than an update to avoid conflicts when multiple controllers are adding their finalizer to the grafana dashboard
		// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
		logger.Info("adding finalizer", "finalizer", DashboardFinalizer)
		patchHelper, err := patch.NewHelper(dashboard, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		controllerutil.AddFinalizer(dashboard, DashboardFinalizer)
		if err := patchHelper.Patch(ctx, dashboard); err != nil {
			logger.Error(err, "failed to add finalizer", "finalizer", DashboardFinalizer)
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("added finalizer", "finalizer", DashboardFinalizer)
		return ctrl.Result{}, nil
	}

	// Configure the dashboard in Grafana
	if err := r.configureDashboard(ctx, dashboard); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func getDashboardUID(jsonDashboard string) (UID string, err error) {

	var Dashboard map[string]interface{}
	err = json.Unmarshal([]byte(jsonDashboard), &Dashboard)
	if err != nil {
		return "", err
	}

	UID, ok := Dashboard["uid"].(string)
	if !ok {
		return "", errors.New("dashboard UID not found in configmap")
	}
	return UID, nil
}

func getDashboardCMOrg(dashboard *v1.ConfigMap) (Org string, err error) {

	// Try to look for an annotation first
	orgName := dashboard.GetAnnotations()[DashboardTeamAnnotation]
	if orgName != "" {
		return orgName, nil
	}

	// Then look for a label
	orgName = dashboard.GetLabels()[DashboardTeamAnnotation]

	if orgName != "" {
		return orgName, nil
	}

	// Return an error if no label was found
	return "", errors.New("No organization label found in configmap")
}

func (r DashboardReconciler) configureDashboard(ctx context.Context, dashboardCM *v1.ConfigMap) (err error) {
	logger := log.FromContext(ctx)

	dashboardOrg, err := getDashboardCMOrg(dashboardCM)
	if err != nil {
		logger.Info("Skipping dashboard, no organization found")
		return nil
	}

	// We always switch back to the shared org
	defer func() {
		if _, err = r.GrafanaAPI.SignedInUser.UserSetUsingOrg(grafana.SharedOrg.ID); err != nil {
			logger.Error(err, "failed to change current org for signed in user")
		}
	}()

	for _, dashboard := range dashboardCM.Data {
		dashboardUID, err := getDashboardUID(dashboard)
		if err != nil {
			logger.Info("Skipping dashboard, no UID found")
			continue
		}
		// Switch context to the current org
		organization, err := grafana.FindOrgByName(r.GrafanaAPI, dashboardOrg)
		if err != nil {
			logger.Error(err, "failed to find organization", "organization", dashboardOrg)
			return errors.WithStack(err)
		}
		logger.Info("Found dashboard org", "Organization", organization)

		if _, err = r.GrafanaAPI.SignedInUser.UserSetUsingOrg(organization.ID); err != nil {
			logger.Error(err, "failed to change current org for signed in user")
			return errors.WithStack(err)
		}

		var dashboardjson interface{}
		err = json.Unmarshal([]byte(dashboard), &dashboardjson)
		if err != nil {
			logger.Info("Failed converting dashboard to json")
			return errors.WithStack(err)
		}

		// Create or update dashboard
		_, err = r.GrafanaAPI.Dashboards.PostDashboard(&models.SaveDashboardCommand{
			UpdatedAt: strfmt.DateTime(time.Now()),
			Dashboard: dashboardjson,
			FolderID:  0,
			FolderUID: "",
			IsFolder:  false,
			Message:   "Added by observability-operator",
			Overwrite: true,
			UserID:    0,
		})
		if err != nil {
			logger.Info("Failed updating dashboard")
			return errors.WithStack(err)
		}

		logger.Info("updated dashboard", "Dashboard UID", dashboardUID, "Dashboard Org", dashboardOrg)
	}

	// return nil
	return errors.New("All good, but not implemented yet")
}

// reconcileDelete deletes the grafana dashboard.
func (r DashboardReconciler) reconcileDelete(ctx context.Context, dashboardCM *v1.ConfigMap) error {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the grafana dashboard
	if !controllerutil.ContainsFinalizer(dashboardCM, DashboardFinalizer) {
		return nil
	}

	dashboardOrg, err := getDashboardCMOrg(dashboardCM)
	if err != nil {
		logger.Info("Skipping dashboard, no organization found")
		return nil
	}

	for _, dashboard := range dashboardCM.Data {
		dashboardUID, err := getDashboardUID(dashboard)
		if err != nil {
			logger.Info("Skipping dashboard, no UID found")
			continue
		}

		// TODO: search for dashboard by ID
		// TODO: delete dashboard if it exits
		logger.Info("deleted dashboard", "Dashboard UID", dashboardUID, "Dashboard Org", dashboardOrg)
	}

	// Finalizer handling needs to come last.
	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("removing finalizer", "finalizer", DashboardFinalizer)
	patchHelper, err := patch.NewHelper(dashboardCM, r.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	controllerutil.RemoveFinalizer(dashboardCM, DashboardFinalizer)
	if err := patchHelper.Patch(ctx, dashboardCM); err != nil {
		logger.Error(err, "failed to remove finalizer, requeuing", "finalizer", DashboardFinalizer)
		return errors.WithStack(err)
	}
	logger.Info("removed finalizer", "finalizer", DashboardFinalizer)

	return nil
}
