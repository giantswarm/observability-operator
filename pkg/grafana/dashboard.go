package grafana

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

// ConfigureDashboard configures a dashboard
func (s *Service) ConfigureDashboard(ctx context.Context, dash *dashboard.Dashboard) error {
	return s.withinOrganization(ctx, dash, func(ctx context.Context) error {
		logger := log.FromContext(ctx)
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
	return s.withinOrganization(ctx, dash, func(ctx context.Context) error {
		logger := log.FromContext(ctx)
		_, err := s.grafanaClient.GetDashboardByUID(dash.UID())
		if err != nil {
			return fmt.Errorf("failed to get dashboard: %w", err)
		}

		_, err = s.grafanaClient.DeleteDashboardByUID(dash.UID())
		if err != nil {
			return fmt.Errorf("failed to delete dashboard: %w", err)
		}

		logger.Info("deleted dashboard")
		return nil
	})
}

// withinOrganization executes the given function within the context of the dashboard's organization
func (s *Service) withinOrganization(ctx context.Context, dash *dashboard.Dashboard, fn func(ctx context.Context) error) error {
	logger := log.FromContext(ctx)

	// Validate the dashboard first
	if validationErrors := dash.Validate(); len(validationErrors) > 0 {
		logger.Info("Skipping dashboard due to validation errors", "errors", validationErrors)
		// Return nil to indicate successful handling (graceful skip)
		return nil
	}

	// Switch context to the dashboard-defined org
	organization, err := s.FindOrgByName(dash.Organization())
	if err != nil {
		return fmt.Errorf("failed to find organization: %w", err)
	}
	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID)
	defer s.grafanaClient.WithOrgID(currentOrgID)
	ctx = log.IntoContext(ctx, logger.WithValues("organization", organization.Name, "dashboard", dash.UID()))

	// Execute the provided function within the organization context
	return fn(ctx)
}

// prepareForGrafanaAPI removes the "id" field which can cause conflicts during dashboard creation/update
func prepareForGrafanaAPI(dash *dashboard.Dashboard) map[string]any {
	content := dash.Content()

	if content["id"] != nil {
		delete(content, "id")
	}

	return content
}
