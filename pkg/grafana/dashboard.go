package grafana

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/grafana/grafana-openapi-client-go/models"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

// ConfigureDashboard configures a dashboard, ensuring folder hierarchy exists and injecting managed tag
func (s *Service) ConfigureDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	return s.withinOrganization(ctx, dashboard, func(ctx context.Context) error {
		logger := log.FromContext(ctx)

		// Ensure folder hierarchy exists and get the leaf folder UID
		folderUID, err := s.EnsureFolderHierarchy(ctx, dashboard.FolderPath())
		if err != nil {
			return fmt.Errorf("failed to ensure folder hierarchy: %w", err)
		}

		dashboardContent := dashboard.Content()
		// removes the "id" field from the content which can cause conflicts during dashboard creation/update
		delete(dashboardContent, "id")

		// Inject the managed tag so operator dashboards are distinguishable
		InjectManagedTag(dashboardContent)

		// Create or update dashboard in the target folder
		err = s.PublishDashboard(dashboardContent, folderUID)
		if err != nil {
			return fmt.Errorf("failed to update dashboard: %w", err)
		}

		logger.Info("updated dashboard", "folderPath", dashboard.FolderPath(), "folderUID", folderUID)
		return nil
	})
}

func (s *Service) DeleteDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	return s.withinOrganization(ctx, dashboard, func(ctx context.Context) error {
		logger := log.FromContext(ctx)

		_, err := s.grafanaClient.Dashboards().GetDashboardByUID(dashboard.UID())
		if err != nil {
			return fmt.Errorf("failed to get dashboard: %w", err)
		}

		_, err = s.grafanaClient.Dashboards().DeleteDashboardByUID(dashboard.UID())
		if err != nil {
			return fmt.Errorf("failed to delete dashboard: %w", err)
		}

		logger.Info("deleted dashboard")
		return nil
	})
}

// PublishDashboard creates or updates a dashboard in Grafana.
// folderUID specifies the target folder; empty string means the General folder.
func (s *Service) PublishDashboard(dashboard map[string]any, folderUID string) error {
	_, err := s.grafanaClient.Dashboards().PostDashboard(&models.SaveDashboardCommand{
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

// withinOrganization executes the given function within the context of the dashboard's organization
func (s *Service) withinOrganization(ctx context.Context, dashboard *dashboard.Dashboard, fn func(ctx context.Context) error) error {
	logger := log.FromContext(ctx)

	// Switch context to the dashboard-defined org
	organization, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		return fmt.Errorf("failed to find organization: %w", err)
	}
	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID())
	defer s.grafanaClient.WithOrgID(currentOrgID)
	ctx = log.IntoContext(ctx, logger.WithValues("organization", organization.Name(), "dashboard", dashboard.UID()))

	// Execute the provided function within the organization context
	return fn(ctx)
}
