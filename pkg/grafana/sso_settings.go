package grafana

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	grafanaAdminRole  = "Admin"
	grafanaEditorRole = "Editor"
	grafanaViewerRole = "Viewer"
)

var ssoProviders = []string{
	"generic_oauth",
	"jwt",
}

func ConfigureSSOSettings(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organizations []Organization) error {
	logger := log.FromContext(ctx)

	orgsMapping := generateGrafanaOrgsMapping(organizations)

	for _, provider := range ssoProviders {
		resp, err := grafanaAPI.SsoSettings.GetProviderSettings(provider, nil)
		if err != nil {
			logger.Error(err, "failed to get sso provider settings.")
			return errors.WithStack(err)
		}

		settings := resp.Payload.Settings.(map[string]interface{})
		settings["org_mapping"] = orgsMapping

		logger.Info("configuring grafana sso settings", "provider", provider, "settings", settings)
		// Update the provider settings
		_, err = grafanaAPI.SsoSettings.UpdateProviderSettings(provider,
			&models.UpdateProviderSettingsParamsBody{
				ID:       resp.Payload.ID,
				Provider: resp.Payload.Provider,
				Settings: settings,
			})

		if err != nil {
			logger.Error(err, "failed to configure grafana sso.")
			return errors.WithStack(err)
		}
	}

	return nil
}

func generateGrafanaOrgsMapping(organizations []Organization) string {
	var orgMappings []string
	for _, organization := range organizations {
		for _, adminOrgAttribute := range organization.Admins {
			orgMappings = append(orgMappings, buildOrgMapping(organization.Name, adminOrgAttribute, grafanaAdminRole))
		}
		for _, editorOrgAttribute := range organization.Editors {
			orgMappings = append(orgMappings, buildOrgMapping(organization.Name, editorOrgAttribute, grafanaEditorRole))
		}
		for _, viewerOrgAttribute := range organization.Viewers {
			orgMappings = append(orgMappings, buildOrgMapping(organization.Name, viewerOrgAttribute, grafanaViewerRole))
		}
	}

	return strings.Join(orgMappings, " ")
}

func buildOrgMapping(organizationName, userOrgAttribute, role string) string {
	// We need to escape the colon in the userOrgAttribute
	u := strings.ReplaceAll(userOrgAttribute, ":", "\\:")
	// We add double quotes to the org mapping to support spaces in display names
	return fmt.Sprintf(`"%s:%s:%s"`, u, organizationName, role)
}
