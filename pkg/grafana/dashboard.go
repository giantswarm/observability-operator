package grafana

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	grafanaOrganizationLabel = "observability.giantswarm.io/organization"
)

func (s *Service) ConfigureDashboard(ctx context.Context, dashboardCM *v1.ConfigMap) error {
	return s.processDashboards(ctx, dashboardCM, func(ctx context.Context, dashboard map[string]any, dashboardUID string) error {
		logger := log.FromContext(ctx)

		// Create or update dashboard
		err := s.PublishDashboard(dashboard)
		if err != nil {
			return fmt.Errorf("failed to update dashboard: %w", err)
		}

		logger.Info("updated dashboard")
		return nil
	})
}

func (s *Service) DeleteDashboard(ctx context.Context, dashboardCM *v1.ConfigMap) error {

	return s.processDashboards(ctx, dashboardCM, func(ctx context.Context, dashboard map[string]any, dashboardUID string) error {
		logger := log.FromContext(ctx)

		_, err := s.grafanaClient.GetDashboardByUID(dashboardUID)
		if err != nil {
			logger.Error(err, "Failed getting dashboard")
			return err
		}

		_, err = s.grafanaClient.DeleteDashboardByUID(dashboardUID)
		if err != nil {
			logger.Error(err, "Failed deleting dashboard")
			return err
		}

		logger.Info("deleted dashboard")
		return nil
	})
}

func (s *Service) processDashboards(ctx context.Context, dashboardCM *v1.ConfigMap, f func(ctx context.Context, dashboard map[string]any, dashboardUID string) error) error {
	logger := log.FromContext(ctx)

	dashboardOrg, err := getOrgFromDashboardConfigmap(dashboardCM)
	if err != nil {
		logger.Error(err, "Skipping dashboard, no organization found")
		return nil
	}

	logger = logger.WithValues("Dashboard Org", dashboardOrg)

	// TODO Tenant Governance: Filter the dashboards with the list of authorized tenants

	// Switch context to the dashboards-defined org
	organization, err := s.FindOrgByName(dashboardOrg)
	if err != nil {
		return fmt.Errorf("failed to find organization: %w", err)
	}
	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID)
	defer s.grafanaClient.WithOrgID(currentOrgID)

	for _, dashboardString := range dashboardCM.Data {
		var dashboard map[string]any
		err = json.Unmarshal([]byte(dashboardString), &dashboard)
		if err != nil {
			logger.Error(err, "Failed converting dashboard to json")
			continue
		}

		dashboardUID, err := getDashboardUID(dashboard)
		if err != nil {
			logger.Error(err, "Skipping dashboard, no UID found")
			continue
		}

		// Clean the dashboard ID to avoid conflicts
		cleanDashboardID(dashboard)

		// Create a new logger with the dashboard UID, this is to avoid overwriting the logger
		dashboardLogger := logger.WithValues("Dashboard UID", dashboardUID)
		ctx = log.IntoContext(ctx, dashboardLogger)

		err = f(ctx, dashboard, dashboardUID)
		if err != nil {
			return fmt.Errorf("failed to process dashboard %q: %w", dashboardUID, err)
		}
	}

	return nil
}

func getOrgFromDashboardConfigmap(dashboard *v1.ConfigMap) (string, error) {
	// Try to look for an annotation first
	annotations := dashboard.GetAnnotations()
	if annotations != nil && annotations[grafanaOrganizationLabel] != "" {
		return annotations[grafanaOrganizationLabel], nil
	}

	// Then look for a label
	labels := dashboard.GetLabels()
	if labels != nil && labels[grafanaOrganizationLabel] != "" {
		return labels[grafanaOrganizationLabel], nil
	}

	// Return an error if no label was found
	return "", fmt.Errorf("No organization label found in configmap")
}

func getDashboardUID(dashboard map[string]interface{}) (string, error) {
	UID, ok := dashboard["uid"].(string)
	if !ok {
		return "", fmt.Errorf("dashboard UID not found in configmap")
	}
	return UID, nil
}

func cleanDashboardID(dashboard map[string]interface{}) {
	if dashboard["id"] != nil {
		delete(dashboard, "id")
	}
}
