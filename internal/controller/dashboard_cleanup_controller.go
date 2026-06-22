package controller

import (
	"context"
	"fmt"
	"net/url"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/internal/labels"
	"github.com/giantswarm/observability-operator/internal/mapper"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/folder"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// dashboardCleanupDelay is how long the controller waits after the first
// dashboard event before cleaning up orphaned folders for an organization.
// If dashboard configmap events arrive in bursts, so we debounce them: the
// delaying queue keeps the earliest scheduled time per organization key,
// meaning cleanup runs once, this long after the first event of a burst.
const dashboardCleanupDelay = time.Minute

// DashboardCleanupReconciler removes orphaned Grafana folders for an organization.
// It is keyed by organization name (carried in the reconcile request Name) rather
// than by a single ConfigMap, so it runs once per organization.
type DashboardCleanupReconciler struct {
	client.Client

	grafanaURL       *url.URL
	dashboardMapper  *mapper.DashboardMapper
	grafanaClientGen grafanaclient.GrafanaClientGenerator
	cfg              config.Config
	cleanupDelay     time.Duration
}

func SetupDashboardCleanupReconciler(mgr manager.Manager, cfg config.Config, grafanaClientGen grafanaclient.GrafanaClientGenerator) error {
	r := &DashboardCleanupReconciler{
		Client: mgr.GetClient(),

		grafanaURL:       cfg.Grafana.URL,
		dashboardMapper:  mapper.New(),
		grafanaClientGen: grafanaClientGen,
		cfg:              cfg,
		cleanupDelay:     dashboardCleanupDelay,
	}

	return r.SetupWithManager(mgr)
}

// Reconcile removes operator-managed folders that are no longer referenced by any
// dashboard ConfigMap of the organization identified by req.Name.
func (r *DashboardCleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	orgName := req.Name

	grafanaAPI, err := r.grafanaClientGen.GenerateGrafanaClient(ctx, r.Client, r.grafanaURL)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate Grafana client: %w", err)
	}

	grafanaService := grafana.NewService(grafanaAPI, r.cfg)

	return ctrl.Result{}, r.cleanupOrphanedFolders(ctx, grafanaService, orgName)
}

// SetupWithManager sets up the controller with the Manager. It watches dashboard
// ConfigMaps but enqueues per-organization cleanup requests with a delay so that
// a burst of dashboard events triggers a single cleanup.
func (r *DashboardCleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelectorPredicate, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
			},
		})
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		Named("dashboard-cleanup").
		Watches(
			&v1.ConfigMap{},
			r.enqueueOrganizationCleanup(),
			builder.WithPredicates(labelSelectorPredicate),
		).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

// enqueueOrganizationCleanup returns an event handler that maps a dashboard
// ConfigMap to its organization and schedules a delayed cleanup request. The
// delaying queue keeps the earliest scheduled time per organization key, so
// repeated events within a burst do not push the cleanup further out.
func (r *DashboardCleanupReconciler) enqueueOrganizationCleanup() handler.EventHandler {
	enqueue := func(obj client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		req, ok := r.organizationRequest(obj)
		if !ok {
			return
		}
		q.AddAfter(req, r.cleanupDelay)
	}

	return handler.Funcs{
		CreateFunc: func(_ context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueue(e.Object, q)
		},
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueue(e.ObjectNew, q)
		},
		DeleteFunc: func(_ context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueue(e.Object, q)
		},
	}
}

// organizationRequest extracts the organization from a dashboard ConfigMap and
// builds the reconcile request used to key per-organization cleanup. It returns
// false when the object is not a ConfigMap or has no organization set.
func (r *DashboardCleanupReconciler) organizationRequest(obj client.Object) (reconcile.Request, bool) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		return reconcile.Request{}, false
	}

	for _, dash := range r.dashboardMapper.FromConfigMap(cm) {
		if org := dash.Organization(); org != "" {
			return reconcile.Request{NamespacedName: types.NamespacedName{Name: org}}, true
		}
	}

	return reconcile.Request{}, false
}

// cleanupOrphanedFolders resolves the organization, computes which folder UIDs are still
// needed by dashboard ConfigMaps, and delegates deletion of orphans to the Grafana service.
func (r *DashboardCleanupReconciler) cleanupOrphanedFolders(ctx context.Context, grafanaService *grafana.Service, orgName string) error {
	org, err := grafanaService.FindOrgByName(orgName)
	if err != nil {
		return fmt.Errorf("failed to find organization %q: %w", orgName, err)
	}

	requiredUIDs, err := r.collectRequiredFolderUIDs(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to collect required folder UIDs: %w", err)
	}

	return grafanaService.CleanupOrphanedFoldersForOrg(ctx, org, requiredUIDs)
}

// collectRequiredFolderUIDs lists all dashboard ConfigMaps for the given organization and computes the set of folder UIDs they reference.
func (r *DashboardCleanupReconciler) collectRequiredFolderUIDs(ctx context.Context, orgName string) (map[string]struct{}, error) {
	// List dashboards ConfigMaps
	var configMaps v1.ConfigMapList
	err := r.List(ctx, &configMaps, client.MatchingLabels{
		labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list dashboard configmaps: %w", err)
	}

	// Filter dashboards by organization and collect folder UIDs
	requiredUIDs := make(map[string]struct{})
	for i := range configMaps.Items {
		dashboards := r.dashboardMapper.FromConfigMap(&configMaps.Items[i])
		for _, dash := range dashboards {
			// Skip dashboards that do not belong to the target organization
			if dash.Organization() != orgName {
				continue
			}

			// Skip dashboards that do not specify a folder path
			if dash.FolderPath() == "" {
				continue
			}

			segments := folder.ParsePath(dash.FolderPath())
			for _, seg := range segments {
				requiredUIDs[seg.UID()] = struct{}{}
			}
		}
	}

	return requiredUIDs, nil
}
