package grafana

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	SharedOrgName       = "Shared Org"
	MimirDatasourceName = "Mimir"
	MimirDatasourceType = "prometheus"
	MimirDatasourceUrl  = "http://mimir-gateway.mimir.svc/prometheus"
	lokiDatasourceName  = "Loki"
	LokiDatasourceType  = "loki"
	LokiDatasourceUrl   = "http://grafana-multi-tenant-proxy.monitoring.svc"
)

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
	found, err := findByID(grafanaAPI, organization.ID)
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
	_, err := findByID(grafanaAPI, id)
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

func CreateDatasource(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	createdDatasources := make([]Datasource, 0)
	mimirDatasource := Datasource{
		Name: MimirDatasourceName,
		Type: MimirDatasourceType,
		Url:  MimirDatasourceUrl,
	}
	lokiDatasource := Datasource{
		Name: lokiDatasourceName,
		Type: LokiDatasourceType,
		Url:  LokiDatasourceUrl,
	}

	logger.Info("creating datasources")
	for _, toCreateDatasource := range []Datasource{mimirDatasource, lokiDatasource} {
		logger.Info(fmt.Sprintf("TRYING TO ADD THE FOLLOWING DATASOURCE : %v", toCreateDatasource))
		createdDataSource, err := grafanaAPI.Datasources.AddDataSource(&models.AddDataSourceCommand{
			Name: toCreateDatasource.Name,
			Type: toCreateDatasource.Type,
			URL:  toCreateDatasource.Url,
		})
		logger.Info(fmt.Sprintf("RESULT OF ADD DATASOURCE METHOD : %v", createdDataSource))
		if err != nil {
			logger.Error(err, "failed to create datasource")
			return []Datasource{}, errors.WithStack(err)
		}

		createdDatasources = append(createdDatasources, Datasource{
			Name: *createdDataSource.Payload.Name,
			ID:   *createdDataSource.Payload.ID,
		})
	}
	logger.Info("datasources created")

	return createdDatasources, nil
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

	found, err := findByName(grafanaAPI, organization.Name)
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
func findByName(grafanaAPI *client.GrafanaHTTPAPI, name string) (*Organization, error) {
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
func findByID(grafanaAPI *client.GrafanaHTTPAPI, orgID int64) (*Organization, error) {
	organization, err := grafanaAPI.Orgs.GetOrgByID(orgID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}