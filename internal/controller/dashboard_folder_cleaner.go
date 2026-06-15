package controller

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/internal/labels"
	"github.com/giantswarm/observability-operator/internal/mapper"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/folder"
	"github.com/giantswarm/observability-operator/pkg/grafana"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// folderCleanupDebounceInterval is how long the cleaner waits for a burst of
// reconciliations to settle before running orphaned-folder cleanup. Every
// Trigger resets this window, so cleanup runs once after the last reconcile.
const folderCleanupDebounceInterval = 10 * time.Second

// folderCleaner debounces orphaned Grafana folder cleanup.
//
// Dashboard reconciliations can arrive in bursts (e.g. a Grafana pod restart
// re-enqueues every dashboard ConfigMap at once). Running the cleanup — which
// lists all folders and dashboards for an organization — after each reconcile
// would be wasteful, so reconciliations only call Trigger to request one. The
// actual cleanup runs once the burst settles: each Trigger resets a debounce
// timer, and cleanup runs only after no Trigger has arrived for the debounce
// interval. The cleaner stays idle until triggered — it never runs on its own.
type folderCleaner struct {
	client           client.Client
	grafanaClientGen grafanaclient.GrafanaClientGenerator
	grafanaURL       *url.URL
	cfg              config.Config
	dashboardMapper  *mapper.DashboardMapper

	interval time.Duration
	requests chan string
}

func newFolderCleaner(c client.Client, grafanaClientGen grafanaclient.GrafanaClientGenerator, grafanaURL *url.URL, cfg config.Config) *folderCleaner {
	return &folderCleaner{
		client:           c,
		grafanaClientGen: grafanaClientGen,
		grafanaURL:       grafanaURL,
		cfg:              cfg,
		dashboardMapper:  mapper.New(),
		interval:         folderCleanupDebounceInterval,
		requests:         make(chan string, 1024),
	}
}

// Trigger requests an orphaned-folder cleanup for the given organization. It is
// safe to call from any reconcile goroutine and never blocks: it only schedules
// work — the cleanup itself runs asynchronously in Start.
func (c *folderCleaner) Trigger(orgName string) {
	if c == nil || orgName == "" {
		return
	}
	select {
	case c.requests <- orgName:
	default:
		// The request buffer is full, meaning a cleanup is already queued for
		// the current burst. Dropping is safe: the queued cleanup recomputes
		// the required folders from live cluster state anyway.
	}
}

// Start runs the debounce loop until ctx is cancelled. It implements
// manager.Runnable so the controller manager owns its lifecycle.
func (c *folderCleaner) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("dashboard-folder-cleaner")

	// The timer is created stopped so the cleaner never runs until the first
	// Trigger arms it.
	timer := time.NewTimer(c.interval)
	if !timer.Stop() {
		<-timer.C
	}

	pending := make(map[string]struct{})

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case org := <-c.requests:
			pending[org] = struct{}{}
			// Reset the debounce window: cleanup only fires once reconciles stop.
			timer.Reset(c.interval)
		case <-timer.C:
			orgs := pending
			pending = make(map[string]struct{})
			c.runCleanup(ctx, logger, orgs)
		}
	}
}

func (c *folderCleaner) runCleanup(ctx context.Context, logger logr.Logger, orgs map[string]struct{}) {
	if len(orgs) == 0 {
		return
	}

	grafanaAPI, err := c.grafanaClientGen.GenerateGrafanaClient(ctx, c.client, c.grafanaURL)
	if err != nil {
		logger.Error(err, "failed to generate Grafana client for folder cleanup")
		return
	}
	grafanaService := grafana.NewService(grafanaAPI, c.cfg)

	for orgName := range orgs {
		if err := c.cleanupOrphanedFolders(ctx, grafanaService, orgName); err != nil {
			logger.Error(err, "failed to cleanup orphaned folders", "organization", orgName)
		}
	}
}

// cleanupOrphanedFolders resolves the organization, computes which folder UIDs are still
// needed by dashboard ConfigMaps, and delegates deletion of orphans to the Grafana service.
func (c *folderCleaner) cleanupOrphanedFolders(ctx context.Context, grafanaService *grafana.Service, orgName string) error {
	org, err := grafanaService.FindOrgByName(orgName)
	if err != nil {
		return fmt.Errorf("failed to find organization %q: %w", orgName, err)
	}

	requiredUIDs, err := c.collectRequiredFolderUIDs(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to collect required folder UIDs: %w", err)
	}

	return grafanaService.CleanupOrphanedFoldersForOrg(ctx, org, requiredUIDs)
}

// collectRequiredFolderUIDs lists all dashboard ConfigMaps for the given organization and computes the set of folder UIDs they reference.
func (c *folderCleaner) collectRequiredFolderUIDs(ctx context.Context, orgName string) (map[string]struct{}, error) {
	var configMaps v1.ConfigMapList
	err := c.client.List(ctx, &configMaps, client.MatchingLabels{
		labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list dashboard configmaps: %w", err)
	}

	requiredUIDs := make(map[string]struct{})
	for i := range configMaps.Items {
		dashboards := c.dashboardMapper.FromConfigMap(&configMaps.Items[i])
		for _, dash := range dashboards {
			if dash.Organization() != orgName {
				continue
			}
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
