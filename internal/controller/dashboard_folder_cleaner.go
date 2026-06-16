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
// Running the cleanup is expensive at is needs to lists all folders and
// dashboards for an organization. Since dashboard reconciliations can arrive
// in bursts (e.g. a Grafana pod restart re-enqueues every dashboard ConfigMap
// at once), this cleaner runs the actual cleanup once the burst settles: each
// Trigger resets a debounce timer, and cleanup runs only after no Trigger has
// arrived for the debounce interval. The cleaner stays idle until triggered —
// it never runs on its own.
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
	// Do not enqueue a cleanup if the cleaner is not initialized or the organization name is empty.
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
		// If we can't stop the timer, it means it has already fired. Drain the channel to ensure we don't receive a spurious event later.
		<-timer.C
	}

	pending := make(map[string]struct{})

	for {
		select {
		case <-ctx.Done():
			// Context cancellation was triggered, stop the timer and exit the loop.
			timer.Stop()
			return nil
		case org := <-c.requests:
			// A cleanup request was received for an organization.
			// Add it to the pending set and reset the debounce timer, so cleanup will run after the interval if no new requests arrive.
			pending[org] = struct{}{}
			timer.Reset(c.interval)
		case <-timer.C:
			// Timer fired, meaning no new requests have arrived for the debounce interval. Run the cleanup for all pending organizations.
			orgs := pending
			pending = make(map[string]struct{})
			// Carry forward any orgs whose cleanup failed so a transient error
			// (e.g. Grafana briefly unavailable) is retried after the next
			// debounce window instead of being silently dropped.
			pending = c.runCleanup(ctx, logger, orgs)
			if len(pending) > 0 {
				timer.Reset(c.interval)
			}
		}
	}
}

// runCleanup performs the actual cleanup of orphaned folders for the given organizations.
// It generates a Grafana client and delegates the cleanup to the Grafana service.
// It returns the set of organizations whose cleanup failed so the caller can retry
// them on a later debounce window instead of dropping them.
func (c *folderCleaner) runCleanup(ctx context.Context, logger logr.Logger, orgs map[string]struct{}) map[string]struct{} {
	if len(orgs) == 0 {
		return nil
	}

	// Create a Grafana API client for the cleanup operation.
	grafanaAPI, err := c.grafanaClientGen.GenerateGrafanaClient(ctx, c.client, c.grafanaURL)
	if err != nil {
		// No org could be processed without a client, so retry all of them.
		logger.Error(err, "failed to generate Grafana client for folder cleanup")
		return orgs
	}

	grafanaService := grafana.NewService(grafanaAPI, c.cfg)

	// Perform cleanup for each given organization.
	failed := make(map[string]struct{})
	for orgName := range orgs {
		if err := c.cleanupOrphanedFolders(ctx, grafanaService, orgName); err != nil {
			logger.Error(err, "failed to cleanup orphaned folders", "organization", orgName)
			failed[orgName] = struct{}{}
		}
	}
	return failed
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
