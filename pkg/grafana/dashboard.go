package grafana

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

// ConfigureDashboard configures a dashboard, ensuring folder hierarchy exists and injecting managed tag
func (s *Service) ConfigureDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	org, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		return fmt.Errorf("failed to find organization: %w", err)
	}

	return s.withinOrganization(ctx, org, func(ctx context.Context) error {
		logger := log.FromContext(ctx)

		// Ensure folder hierarchy exists and get the leaf folder UID
		folderUID, err := s.ensureFolderHierarchy(ctx, dashboard.FolderPath())
		if err != nil {
			return fmt.Errorf("failed to ensure folder hierarchy: %w", err)
		}

		dashboardContent := dashboard.Content()
		// removes the "id" field from the content which can cause conflicts during dashboard creation/update
		delete(dashboardContent, "id")

		// Inject the managed tag so operator dashboards are distinguishable
		injectManagedTag(dashboardContent)

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
	org, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		return fmt.Errorf("failed to find organization: %w", err)
	}

	return s.withinOrganization(ctx, org, func(ctx context.Context) error {
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

const managedDashboardTag = "managed-by: gitops"

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

// withinOrganization executes the given function within the context of the given organization.
// NOTE: The WithOrgID pattern mutates shared state on the Grafana client. If two reconciliations
// run concurrently for different orgs, they will clobber each other's org context.
func (s *Service) withinOrganization(ctx context.Context, org *organization.Organization, fn func(ctx context.Context) error) error {
	logger := log.FromContext(ctx)

	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(org.ID())
	defer s.grafanaClient.WithOrgID(currentOrgID)
	ctx = log.IntoContext(ctx, logger.WithValues("organization", org.Name()))

	return fn(ctx)
}
