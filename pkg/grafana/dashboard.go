package grafana

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

// ConfigureDashboard configures a dashboard
func (s *Service) ConfigureDashboard(ctx context.Context, dash *dashboard.Dashboard) error {
	logger := log.FromContext(ctx).WithValues("Dashboard UID", dash.UID(), "Dashboard Org", dash.Organization())

	return s.withinOrganization(ctx, dash, func() error {
		// Prepare dashboard content for Grafana API using local function
		dashboardContent := prepareForGrafanaAPI(dash)

		// Create or update dashboard
		err := s.PublishDashboard(dashboardContent)
		if err != nil {
			return fmt.Errorf("failed to update dashboard: %w", err)
		}

		logger.Info("updated dashboard")
		return nil
	})
}

func (s *Service) DeleteDashboard(ctx context.Context, dash *dashboard.Dashboard) error {
	logger := log.FromContext(ctx).WithValues("Dashboard UID", dash.UID(), "Dashboard Org", dash.Organization())

	return s.withinOrganization(ctx, dash, func() error {
		_, err := s.grafanaAPI.Dashboards.GetDashboardByUID(dash.UID())
		if err != nil {
			return fmt.Errorf("failed to get dashboard: %w", err)
		}

		_, err = s.grafanaAPI.Dashboards.DeleteDashboardByUID(dash.UID())
		if err != nil {
			return fmt.Errorf("failed to delete dashboard: %w", err)
		}

		logger.Info("deleted dashboard")
		return nil
	})
}

// withinOrganization executes the given function within the context of the dashboard's organization
func (s *Service) withinOrganization(ctx context.Context, dash *dashboard.Dashboard, fn func() error) error {
	logger := log.FromContext(ctx)

	// Switch context to the dashboard-defined org
	organization, err := s.FindOrgByName(dash.Organization())
	if err != nil {
		return fmt.Errorf("failed to find organization: %w", err)
	}
	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID)
	defer s.grafanaClient.WithOrgID(currentOrgID)

	// Execute the provided function within the organization context
	return fn()
}

// prepareForGrafanaAPI removes the "id" field which can cause conflicts during dashboard creation/update
func prepareForGrafanaAPI(dash *dashboard.Dashboard) map[string]any {
	content := dash.Content()

	if content["id"] != nil {
		delete(content, "id")
	}

	return content
}
