package grafana

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	SharedOrgName = "Shared Org"

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

func CreateOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization Organization) (Organization, error) {
	logger := log.FromContext(ctx)

	logger.Info("creating organization")
	err := assertNameIsAvailable(ctx, grafanaAPI, organization)
	if err != nil {
		return organization, errors.WithStack(err)
	}

	createdOrg, err := grafanaAPI.Orgs.CreateOrg(&models.CreateOrgCommand{
		Name: organization.Name,
	})
	if err != nil {
		logger.Error(err, "failed to create organization")
		return organization, errors.WithStack(err)
	}
	logger.Info("organization created")

	return Organization{
		ID:   *createdOrg.Payload.OrgID,
		Name: organization.Name,
	}, nil
}

func UpdateOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization Organization) (Organization, error) {
	logger := log.FromContext(ctx)

	logger.Info("updating organization")
	found, err := findByID(ctx, grafanaAPI, organization.ID)
	if err != nil {
		if isNotFound(err) {
			logger.Info("organization id not found, creating")
			// If the CR orgID does not exist in Grafana, then we create the organization
			return CreateOrganization(ctx, grafanaAPI, organization)
		}
		logger.Error(err, fmt.Sprintf("failed to find organization with ID: %d", organization.ID))
		return organization, errors.WithStack(err)
	}

	// If both name matches, there is nothing to do.
	if found.Name == organization.Name {
		logger.Info("the organization already exists in Grafana and does not need to be updated.")
		return organization, nil
	}

	err = assertNameIsAvailable(ctx, grafanaAPI, organization)
	if err != nil {
		return organization, errors.WithStack(err)
	}

	// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
	_, err = grafanaAPI.Orgs.UpdateOrg(organization.ID, &models.UpdateOrgForm{
		Name: organization.Name,
	})
	if err != nil {
		logger.Error(err, "failed to update organization name")
		return organization, errors.WithStack(err)
	}

	logger.Info("updated organization")

	return Organization{
		ID:   organization.ID,
		Name: organization.Name,
	}, nil
}

func DeleteByID(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, id int64) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting organization")
	_, err := findByID(ctx, grafanaAPI, id)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to find organization with ID: %d", id))
	}

	_, err = grafanaAPI.Orgs.DeleteOrgByID(id)
	if err != nil {
		logger.Error(err, "failed to delete organization")
		return errors.WithStack(err)
	}
	logger.Info("deleted organization")

	return nil
}

func GenerateGrafanaConfiguration(organizations []v1alpha1.GrafanaOrganization) (string, error) {
	var orgMappings []string
	orgMappings = append(orgMappings, fmt.Sprintf(`"*:%s:%s"`, SharedOrgName, grafanaAdminRole))
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

	data := struct {
		OrgMapping string
	}{
		OrgMapping: orgMapping,
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

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Parsing error message to find out the error code
	return strings.Contains(err.Error(), "(status 404)")
}

// assertNameIsAvailable is a helper function to check if the organization name is available in Grafana
func assertNameIsAvailable(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization Organization) error {
	logger := log.FromContext(ctx)

	found, err := findByName(ctx, grafanaAPI, organization.Name)
	// We only error if we have any error other than a 404
	if err != nil && !isNotFound(err) {
		logger.Error(err, fmt.Sprintf("failed to find organization with name: %s", organization.Name))
		return errors.WithStack(err)
	}

	if found != nil {
		logger.Error(err, "a grafana organization with the same name already exists. Please choose a different display name.")
		return errors.WithStack(err)
	}
	return nil
}

// findByName is a wrapper function used to find a Grafana organization by its name
func findByName(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, name string) (*Organization, error) {
	organization, err := grafanaAPI.Orgs.GetOrgByName(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// findByID is a wrapper function used to find a Grafana organization by its id
func findByID(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, orgID int64) (*Organization, error) {
	organization, err := grafanaAPI.Orgs.GetOrgByID(orgID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}
