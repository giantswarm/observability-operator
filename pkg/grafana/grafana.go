package grafana

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	datasourceProxyAccessMode = "proxy"
	mimirOldDatasourceUID     = "gs-mimir-old"
)

var orgNotFoundError = errors.New("organization not found")

var SharedOrg = Organization{
	ID:   1,
	Name: "Shared Org",
}

// We need to use a custom name for now until we can replace the existing datasources.
var defaultDatasources = []Datasource{
	{
		Name:      "Mimir Alertmanager",
		UID:       "gs-mimir-alertmanager",
		Type:      "alertmanager",
		IsDefault: true,
		URL:       "http://mimir-alertmanager.mimir.svc:8080",
		Access:    datasourceProxyAccessMode,
		JSONData: map[string]interface{}{
			"handleGrafanaManagedAlerts": false,
			"implementation":             "mimir",
		},
	},
	{
		Name:      "Mimir (old tenant data)",
		UID:       mimirOldDatasourceUID,
		Type:      "prometheus",
		IsDefault: true,
		URL:       "http://mimir-gateway.mimir.svc/prometheus",
		Access:    datasourceProxyAccessMode,
		JSONData: map[string]interface{}{
			"cacheLevel":     "None",
			"httpMethod":     "POST",
			"mimirVersion":   "2.14.0",
			"prometheusType": "Mimir",
			"timeInterval":   "60s",
		},
	},
	{
		Name:      "Mimir",
		UID:       "gs-mimir",
		Type:      "prometheus",
		IsDefault: true,
		URL:       "http://mimir-gateway.mimir.svc/prometheus",
		Access:    datasourceProxyAccessMode,
		JSONData: map[string]interface{}{
			"cacheLevel":     "None",
			"httpMethod":     "POST",
			"mimirVersion":   "2.14.0",
			"prometheusType": "Mimir",
			"timeInterval":   "60s",
		},
	},
	{
		Name:   "Loki",
		UID:    "gs-loki",
		Type:   "loki",
		URL:    "http://loki-gateway.loki.svc",
		Access: datasourceProxyAccessMode,
	},
}

func UpsertOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization *Organization) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("upserting organization")

	// Get the current organization stored in Grafana
	currentOrganization, err := findOrgByID(grafanaAPI, organization.ID)
	if err != nil {
		if errors.Is(err, orgNotFoundError) {
			logger.Info("organization id not found, creating")

			// If organization does not exist in Grafana, create it
			createdOrg, err := grafanaAPI.Orgs.CreateOrg(&models.CreateOrgCommand{
				Name: organization.Name,
			})
			if err != nil {
				logger.Error(err, "failed to create organization")
				return errors.WithStack(err)
			}
			logger.Info("created organization")

			organization.ID = *createdOrg.Payload.OrgID
			return nil
		}

		logger.Error(err, fmt.Sprintf("failed to find organization with ID: %d", organization.ID))
		return errors.WithStack(err)
	}

	// If both name matches, there is nothing to do.
	if currentOrganization.Name == organization.Name {
		logger.Info("the organization already exists in Grafana and does not need to be updated.")
		return nil
	}

	// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
	_, err = grafanaAPI.Orgs.UpdateOrg(organization.ID, &models.UpdateOrgForm{
		Name: organization.Name,
	})
	if err != nil {
		logger.Error(err, "failed to update organization name")
		return errors.WithStack(err)
	}

	logger.Info("updated organization")

	return nil
}

func DeleteOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization Organization) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting organization")
	_, err := findOrgByID(grafanaAPI, organization.ID)
	if err != nil {
		if isNotFound(err) {
			logger.Info("organization id was not found, skipping deletion")
			// If the CR orgID does not exist in Grafana, then we create the organization
			return nil
		}
		logger.Error(err, fmt.Sprintf("failed to find organization with ID: %d", organization.ID))
		return errors.WithStack(err)
	}

	_, err = grafanaAPI.Orgs.DeleteOrgByID(organization.ID)
	if err != nil {
		logger.Error(err, "failed to delete organization")
		return errors.WithStack(err)
	}
	logger.Info("deleted organization")

	return nil
}

func ConfigureDefaultDatasources(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization Organization) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	// TODO using a serviceaccount later would be better as they are scoped to an organization

	currentOrgID := grafanaAPI.OrgID()
	grafanaAPI.WithOrgID(organization.ID)
	defer grafanaAPI.WithOrgID(currentOrgID)

	configuredDatasourcesInGrafana, err := listDatasourcesForOrganization(ctx, grafanaAPI)
	if err != nil {
		logger.Error(err, "failed to list datasources")
		return nil, errors.WithStack(err)
	}

	datasourcesToCreate := make([]Datasource, 0)
	datasourcesToUpdate := make([]Datasource, 0)

	// Check if the default datasources are already configured
	for _, defaultDatasource := range defaultDatasources {
		found := false
		for _, configuredDatasource := range configuredDatasourcesInGrafana {
			if configuredDatasource.Name == defaultDatasource.Name {
				found = true

				// We need to extract the ID from the configured datasource
				datasourcesToUpdate = append(datasourcesToUpdate, defaultDatasource.withID(configuredDatasource.ID))
				break
			}
		}
		if !found {
			datasourcesToCreate = append(datasourcesToCreate, defaultDatasource)
		}
	}

	for index, datasource := range datasourcesToCreate {
		logger.Info("creating datasource", "datasource", datasource.Name)
		created, err := grafanaAPI.Datasources.AddDataSource(
			&models.AddDataSourceCommand{
				UID:            datasource.UID,
				Name:           datasource.Name,
				Type:           datasource.Type,
				URL:            datasource.URL,
				IsDefault:      datasource.IsDefault,
				JSONData:       models.JSON(datasource.buildJSONData()),
				SecureJSONData: datasource.buildSecureJSONData(organization),
				Access:         models.DsAccess(datasource.Access),
			})
		if err != nil {
			logger.Error(err, "failed to create datasources", "datasource", datasourcesToCreate[index].Name)
			return nil, errors.WithStack(err)
		}
		datasourcesToCreate[index].ID = *created.Payload.ID
		logger.Info("datasource created", "datasource", datasource.Name)
	}

	for _, datasource := range datasourcesToUpdate {
		logger.Info("updating datasource", "datasource", datasource.Name)
		_, err := grafanaAPI.Datasources.UpdateDataSourceByID(
			strconv.FormatInt(datasource.ID, 10),
			&models.UpdateDataSourceCommand{
				UID:            datasource.UID,
				Name:           datasource.Name,
				Type:           datasource.Type,
				URL:            datasource.URL,
				IsDefault:      datasource.IsDefault,
				JSONData:       models.JSON(datasource.buildJSONData()),
				SecureJSONData: datasource.buildSecureJSONData(organization),
				Access:         models.DsAccess(datasource.Access),
			})
		if err != nil {
			logger.Error(err, "failed to update datasources", "datasource", datasource.Name)
			return nil, errors.WithStack(err)
		}
		logger.Info("datasource updated", "datasource", datasource.Name)
	}

	// We return the datasources and the error if it exists. This allows us to return the defer function error it it exists.
	return append(datasourcesToCreate, datasourcesToUpdate...), errors.WithStack(err)
}

func listDatasourcesForOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	resp, err := grafanaAPI.Datasources.GetDataSources()
	if err != nil {
		logger.Error(err, "failed to get configured datasources")
		return nil, errors.WithStack(err)
	}

	datasources := make([]Datasource, len(resp.Payload))
	for i, datasource := range resp.Payload {
		datasources[i] = Datasource{
			ID:        datasource.ID,
			Name:      datasource.Name,
			IsDefault: datasource.IsDefault,
			Type:      datasource.Type,
			URL:       datasource.URL,
			Access:    string(datasource.Access),
		}
	}

	return datasources, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *runtime.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsCode(http.StatusNotFound)
	}

	return false
}

// FindOrgByName is a wrapper function used to find a Grafana organization by its name
func FindOrgByName(grafanaAPI *client.GrafanaHTTPAPI, name string) (*Organization, error) {
	organization, err := grafanaAPI.Orgs.GetOrgByName(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// findOrgByID is a wrapper function used to find a Grafana organization by its id
func findOrgByID(grafanaAPI *client.GrafanaHTTPAPI, orgID int64) (*Organization, error) {
	if orgID == 0 {
		return nil, orgNotFoundError
	}

	organization, err := grafanaAPI.Orgs.GetOrgByID(orgID)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", orgNotFoundError, err)
		}

		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// PublishDashboard creates or updates a dashboard in Grafana
func PublishDashboard(grafanaAPI *client.GrafanaHTTPAPI, dashboard map[string]any) error {
	_, err := grafanaAPI.Dashboards.PostDashboard(&models.SaveDashboardCommand{
		Dashboard: any(dashboard),
		Message:   "Added by observability-operator",
		Overwrite: true, // allows dashboard to be updated by the same UID

	})
	return err
}
