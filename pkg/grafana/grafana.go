package grafana

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	datasourceProxyAccessMode = "proxy"
	mimirOldDatasourceUID     = "gs-mimir-old"
)

var ErrOrganizationNotFound = errors.New("organization not found")

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
		JSONData: map[string]any{
			"handleGrafanaManagedAlerts": false,
			"implementation":             "mimir",
		},
	},
	{
		Name:      "Mimir",
		UID:       "gs-mimir",
		Type:      "prometheus",
		IsDefault: true,
		URL:       "http://mimir-gateway.mimir.svc/prometheus",
		Access:    datasourceProxyAccessMode,
		JSONData: map[string]any{
			// Cache matching queries on metadata endpoints within 10 minutes to improve performance
			// and reduce load on the Mimir API.
			"cacheLevel": "Medium",
			"httpMethod": "POST",
			// Enables incremental querying, which allows Grafana to fetch only new data when dashboards are refreshed,
			// rather than re-fetching all data. This is particularly useful for large datasets and improves performance.
			"incrementalQuerying": true,
			"prometheusType":      "Mimir",
			// This is the expected value for the Mimir datasource in Grafana
			"prometheusVersion": "2.9.1",
			"timeInterval":      "60s",
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

// Extra public datasources (added to "Shared Org")
var extraPublicDatasources = []Datasource{
	{
		Name:      "Mimir Cardinality",
		UID:       "gs-mimir-cardinality",
		Type:      "marcusolsson-json-datasource",
		URL:       "http://mimir-gateway.mimir.svc:8080/prometheus/api/v1/cardinality/",
		IsDefault: false,
		Access:    datasourceProxyAccessMode,
	},
}

func (s *Service) UpsertOrganization(ctx context.Context, organization *Organization) error {
	logger := log.FromContext(ctx)
	logger.Info("upserting organization")

	// Get the current organization stored in Grafana
	currentOrganization, err := s.findOrgByID(organization.ID)
	if err != nil {
		if errors.Is(err, ErrOrganizationNotFound) {
			logger.Info("organization id not found, creating")

			// If organization does not exist in Grafana, create it
			createdOrg, err := s.grafanaClient.Orgs().CreateOrg(&models.CreateOrgCommand{
				Name: organization.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to create organization: %w", err)
			}
			logger.Info("created organization")

			organization.ID = *createdOrg.Payload.OrgID
			return nil
		}

		return fmt.Errorf("failed to find organization with ID %d: %w", organization.ID, err)
	}

	// If both name matches, there is nothing to do.
	if currentOrganization.Name == organization.Name {
		logger.Info("the organization already exists in Grafana and does not need to be updated.")
		return nil
	}

	// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
	_, err = s.grafanaClient.Orgs().UpdateOrg(organization.ID, &models.UpdateOrgForm{
		Name: organization.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to update organization name: %w", err)
	}

	logger.Info("updated organization")

	return nil
}

func (s *Service) deleteOrganization(ctx context.Context, organization Organization) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting organization")
	_, err := s.findOrgByID(organization.ID)
	if err != nil {
		if errors.Is(err, ErrOrganizationNotFound) {
			logger.Info("organization id was not found, skipping deletion")
			// If the CR orgID does not exist in Grafana, then we create the organization
			return nil
		}
		return fmt.Errorf("failed to find organization: %w", err)
	}

	_, err = s.grafanaClient.Orgs().DeleteOrgByID(organization.ID)
	if err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}
	logger.Info("deleted organization")

	return nil
}

func (s *Service) ConfigureDefaultDatasources(ctx context.Context, organization Organization) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	// TODO using a serviceaccount later would be better as they are scoped to an organization

	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID)
	defer s.grafanaClient.WithOrgID(currentOrgID)

	configuredDatasourcesInGrafana, err := s.listDatasourcesForOrganization()
	if err != nil {
		return nil, fmt.Errorf("failed to list datasources: %w", err)
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

	if organization.Name == SharedOrg.Name {
		logger.Info("organization is shared org, adding extra public datasources")
		for _, extraDatasource := range extraPublicDatasources {
			found := false
			for _, configuredDatasource := range configuredDatasourcesInGrafana {
				if configuredDatasource.Name == extraDatasource.Name {
					found = true
					// We need to extract the ID from the configured datasource
					datasourcesToUpdate = append(datasourcesToUpdate, extraDatasource.withID(configuredDatasource.ID))
					break
				}
			}
			if !found {
				datasourcesToCreate = append(datasourcesToCreate, extraDatasource)
			}
		}
	}

	for index, datasource := range datasourcesToCreate {
		logger.Info("creating datasource", "datasource", datasource.Name)
		created, err := s.grafanaClient.Datasources().AddDataSource(
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
			return nil, fmt.Errorf("failed to create datasource: %w", err)
		}
		datasourcesToCreate[index].ID = *created.Payload.ID
		logger.Info("datasource created", "datasource", datasource.Name)
	}

	for _, datasource := range datasourcesToUpdate {
		logger.Info("updating datasource", "datasource", datasource.Name)
		_, err := s.grafanaClient.Datasources().UpdateDataSourceByUID(
			datasource.UID,
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
			return nil, fmt.Errorf("failed to update datasource: %w", err)
		}
		logger.Info("datasource updated", "datasource", datasource.Name)
	}

	updatedDatasources := append(datasourcesToCreate, datasourcesToUpdate...)
	// If the old mimir datasource exists, we need to delete it
	logger.Info("deleting datasource", "datasource", mimirOldDatasourceUID)
	_, err = s.grafanaClient.Datasources().DeleteDataSourceByUID(mimirOldDatasourceUID)
	if err != nil {
		var notFound *datasources.DeleteDataSourceByUIDNotFound
		if errors.As(err, &notFound) {
			logger.Info("skipping, datasource not found", "datasource", mimirOldDatasourceUID)
			return updatedDatasources, nil
		}

		return nil, fmt.Errorf("failed to delete datasource: %w", err)
	}

	logger.Info("deleted datasource", "datasource", mimirOldDatasourceUID)

	// We return the datasources and the error if it exists. This allows us to return the defer function error it it exists.
	return updatedDatasources, nil
}

func (s *Service) listDatasourcesForOrganization() ([]Datasource, error) {
	resp, err := s.grafanaClient.Datasources().GetDataSources()
	if err != nil {
		return nil, fmt.Errorf("failed to get configured datasources: %w", err)
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
func (s *Service) FindOrgByName(name string) (*Organization, error) {
	organization, err := s.grafanaClient.Orgs().GetOrgByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by name: %w", err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// findOrgByID is a wrapper function used to find a Grafana organization by its id
func (s *Service) findOrgByID(orgID int64) (*Organization, error) {
	if orgID == 0 {
		return nil, ErrOrganizationNotFound
	}

	organization, err := s.grafanaClient.Orgs().GetOrgByID(orgID)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", ErrOrganizationNotFound, err)
		}

		return nil, fmt.Errorf("failed to get organization by id: %w", err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// PublishDashboard creates or updates a dashboard in Grafana
func (s *Service) PublishDashboard(dashboard map[string]any) error {
	_, err := s.grafanaClient.Dashboards().PostDashboard(&models.SaveDashboardCommand{
		Dashboard: any(dashboard),
		Message:   "Added by observability-operator",
		Overwrite: true, // allows dashboard to be updated by the same UID

	})
	if err != nil {
		return fmt.Errorf("failed to publish dashboard: %w", err)
	}
	return nil
}
