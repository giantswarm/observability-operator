package grafana

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/metrics"
)

// ConfigureDashboard configures a dashboard, ensuring folder hierarchy exists and injecting managed tag
func (s *Service) ConfigureDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	org, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureDashboard).Inc()
		return fmt.Errorf("failed to find organization: %w", err)
	}

	err = s.withinOrganization(ctx, org, func(ctx context.Context, client grafanaclient.GrafanaClient) error {
		logger := log.FromContext(ctx)

		// Ensure folder hierarchy exists and get the leaf folder UID
		folderUID, err := s.ensureFolderHierarchy(ctx, client, dashboard.FolderPath())
		if err != nil {
			return fmt.Errorf("failed to ensure folder hierarchy: %w", err)
		}

		dashboardContent := dashboard.Content()
		// removes the "id" field from the content which can cause conflicts during dashboard creation/update
		delete(dashboardContent, "id")

		// Inject the managed tag so operator dashboards are distinguishable
		injectManagedTag(dashboardContent)

		if err := publishDashboard(client, dashboardContent, folderUID); err != nil {
			return fmt.Errorf("failed to update dashboard: %w", err)
		}

		logger.Info("updated dashboard", "folderPath", dashboard.FolderPath(), "folderUID", folderUID)
		return nil
	})
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureDashboard).Inc()
	}
	return err
}

func (s *Service) DeleteDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	org, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpDeleteDashboard).Inc()
		return fmt.Errorf("failed to find organization: %w", err)
	}

	err = s.withinOrganization(ctx, org, func(ctx context.Context, client grafanaclient.GrafanaClient) error {
		logger := log.FromContext(ctx)

		_, err := client.Dashboards().GetDashboardByUID(dashboard.UID())
		if err != nil {
			return fmt.Errorf("failed to get dashboard: %w", err)
		}

		_, err = client.Dashboards().DeleteDashboardByUID(dashboard.UID())
		if err != nil {
			return fmt.Errorf("failed to delete dashboard: %w", err)
		}

		logger.Info("deleted dashboard")
		return nil
	})
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpDeleteDashboard).Inc()
	}
	return err
}

const managedDashboardTag = "managed-by: observability-operator"

// injectManagedTag ensures the managed tag is present in the dashboard content's tags array.
// This is idempotent - if the tag already exists, it does nothing.
func injectManagedTag(content map[string]any) {
	tags, ok := content["tags"].([]any)
	if !ok {
		tags = []any{}
	}

	for _, tag := range tags {
		if tag == managedDashboardTag {
			return
		}
	}

	content["tags"] = append(tags, managedDashboardTag)
}

// publishDashboard creates or updates a dashboard in Grafana.
// folderUID specifies the target folder; empty string means the General folder.
func publishDashboard(client grafanaclient.GrafanaClient, dashboard map[string]any, folderUID string) error {
	_, err := client.Dashboards().PostDashboard(&models.SaveDashboardCommand{
		Dashboard: any(dashboard),
		FolderUID: folderUID,
		Message:   "Added by observability-operator",
		Overwrite: true, // allows dashboard to be updated by the same UID
	})
	if err != nil {
		return fmt.Errorf("failed to publish dashboard: %w", err)
	}
	return nil
}

// withinOrganization runs fn against a Grafana client scoped to org. The cloned
// client is local to this call, so concurrent invocations for different orgs do
// not interfere with each other.
func (s *Service) withinOrganization(
	ctx context.Context,
	org *organization.Organization,
	fn func(ctx context.Context, client grafanaclient.GrafanaClient) error,
) error {
	logger := log.FromContext(ctx).WithValues("organization", org.Name())
	ctx = log.IntoContext(ctx, logger)
	return fn(ctx, s.grafanaClient.WithOrgID(org.ID()))
}
