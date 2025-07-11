package grafana

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	grafanaAdminRole  = "Admin"
	grafanaEditorRole = "Editor"
	grafanaViewerRole = "Viewer"

	// ssoProvider is the OAuth provider type used for SSO configuration
	ssoProvider = "generic_oauth"
)

// ConfigureSSOSettings configures Grafana SSO settings with organization mappings.
// It retrieves the current SSO provider settings, updates the org_mapping field
// with the provided organizations, and applies the changes to Grafana.
func (s *Service) ConfigureSSOSettings(ctx context.Context, organizations []Organization) error {
	logger := log.FromContext(ctx).WithValues("provider", ssoProvider, "organizations_count", len(organizations))

	if len(organizations) == 0 {
		logger.Info("skipping SSO configuration, no organizations provided")
		return nil
	}

	resp, err := s.grafanaClient.SsoSettings().GetProviderSettings(ssoProvider)
	if err != nil {
		return fmt.Errorf("failed to get SSO provider settings for %s: %w", ssoProvider, err)
	}

	if resp.Payload == nil {
		return fmt.Errorf("received nil payload from SSO provider settings for %s", ssoProvider)
	}

	// Safe type assertion with error handling
	settings, ok := resp.Payload.Settings.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected settings type for %s: expected map[string]any, got %T", ssoProvider, resp.Payload.Settings)
	}

	orgsMapping, err := generateGrafanaOrgsMapping(organizations)
	if err != nil {
		return fmt.Errorf("failed to generate organization mappings for %s: %w", ssoProvider, err)
	}

	settings["org_mapping"] = orgsMapping

	logger.Info("configuring SSO settings")

	// Update the provider settings
	_, err = s.grafanaClient.SsoSettings().UpdateProviderSettings(ssoProvider,
		&models.UpdateProviderSettingsParamsBody{
			ID:       resp.Payload.ID,
			Provider: resp.Payload.Provider,
			Settings: settings,
		})

	if err != nil {
		return fmt.Errorf("failed to update SSO provider settings for %s: %w", ssoProvider, err)
	}

	logger.Info("configured SSO settings successfully")
	return nil
}

// generateGrafanaOrgsMapping generates Grafana organization mappings from the provided organizations.
// Each organization's users are mapped to Grafana roles (Admin, Editor, Viewer) based on their attributes.
func generateGrafanaOrgsMapping(organizations []Organization) (string, error) {
	var orgMappings []string
	for _, organization := range organizations {
		if organization.Name == "" {
			return "", fmt.Errorf("organization name cannot be empty")
		}

		// Process admins
		for _, adminOrgAttribute := range organization.Admins {
			if adminOrgAttribute == "" {
				return "", fmt.Errorf("admin attribute cannot be empty for organization %s", organization.Name)
			}
			orgMappings = append(orgMappings, buildOrgMapping(organization.Name, adminOrgAttribute, grafanaAdminRole))
		}

		// Process editors
		for _, editorOrgAttribute := range organization.Editors {
			orgMappings = append(orgMappings, buildOrgMapping(organization.Name, editorOrgAttribute, grafanaEditorRole))
		}

		// Process viewers
		for _, viewerOrgAttribute := range organization.Viewers {
			orgMappings = append(orgMappings, buildOrgMapping(organization.Name, viewerOrgAttribute, grafanaViewerRole))
		}
	}

	return strings.Join(orgMappings, " "), nil
}

func buildOrgMapping(organizationName, userOrgAttribute, role string) string {
	// We need to escape the colon in the userOrgAttribute
	u := strings.ReplaceAll(userOrgAttribute, ":", "\\:")
	// We add double quotes to the org mapping to support spaces in display names
	return fmt.Sprintf(`"%s:%s:%s"`, u, organizationName, role)
}
