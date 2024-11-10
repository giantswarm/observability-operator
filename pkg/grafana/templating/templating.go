package templating

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/grafana"
)

const (
	grafanaAdminRole  = "Admin"
	grafanaEditorRole = "Editor"
	grafanaViewerRole = "Viewer"
)

var (
	//go:embed templates/grafana-user-values.yaml.template
	grafanaUserConfig         string
	grafanaUserConfigTemplate *template.Template
)

func init() {
	grafanaUserConfigTemplate = template.Must(template.New("grafana-user-values.yaml").Parse(grafanaUserConfig))
}

func GenerateGrafanaConfiguration(organizations []v1alpha1.GrafanaOrganization) (string, error) {
	var orgMappings []string
	// TODO: We need to be admins to be able to see the private dashboards, make this editor once https://github.com/giantswarm/roadmap/issues/3696 is done.
	orgMappings = append(orgMappings, fmt.Sprintf(`"*:%s:%s"`, grafana.SharedOrg.Name, grafanaAdminRole))
	for _, organization := range organizations {
		rbac := organization.Spec.RBAC
		organizationName := organization.Spec.DisplayName
		for _, adminOrgAttribute := range rbac.Admins {
			orgMappings = append(orgMappings, buildOrgMapping(organizationName, adminOrgAttribute, grafanaAdminRole))
		}
		for _, editorOrgAttribute := range rbac.Editors {
			orgMappings = append(orgMappings, buildOrgMapping(organizationName, editorOrgAttribute, grafanaEditorRole))
		}
		for _, viewerOrgAttribute := range rbac.Viewers {
			orgMappings = append(orgMappings, buildOrgMapping(organizationName, viewerOrgAttribute, grafanaViewerRole))
		}
	}

	orgMapping := strings.Join(orgMappings, " ")

	provisionedOrganizations := make([]grafana.Organization, len(organizations)+1)
	provisionedOrganizations[0] = grafana.SharedOrg
	for i, org := range organizations {
		provisionedOrganizations[i+1] = grafana.Organization{
			ID:       org.Status.OrgID,
			Name:     org.Spec.DisplayName,
			TenantID: org.Name,
		}
	}

	data := struct {
		OrgMapping    string
		Organizations []grafana.Organization
	}{
		OrgMapping:    orgMapping,
		Organizations: provisionedOrganizations,
	}

	var values bytes.Buffer
	err := grafanaUserConfigTemplate.Execute(&values, data)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return values.String(), nil
}

func buildOrgMapping(organizationName, userOrgAttribute, role string) string {
	// We need to escape the colon in the userOrgAttribute
	u := strings.ReplaceAll(userOrgAttribute, ":", "\\:")
	// We add double quotes to the org mapping to support spaces in display names
	return fmt.Sprintf(`"%s:%s:%s"`, u, organizationName, role)
}
