package grafana

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"

	common "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

// ConfigureDatasources ensures the datasources for the given organization are up to date.
// It creates, updates, or deletes datasources as necessary to match the desired state.
func (s *Service) ConfigureDatasource(ctx context.Context, organization Organization) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	// Generate the desired datasources for the organization
	desiredDatasources := s.generateDatasources(organization)

	// Configure Grafana client to use the correct organization
	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID)
	defer s.grafanaClient.WithOrgID(currentOrgID)

	// Fetch the currently configured datasources in Grafana
	resp, err := s.grafanaClient.Datasources().GetDataSources()
	if err != nil {
		return nil, fmt.Errorf("failed to get configured datasources: %w", err)
	}

	// Update or delete existing datasources
	for _, currentDatasource := range resp.GetPayload() {
		// Check if the current datasource exists in the desired datasources
		index := slices.IndexFunc(desiredDatasources, func(d Datasource) bool {
			return d.UID == currentDatasource.UID
		})

		if index >= 0 {
			// Update the existing datasource
			desiredDatasource := desiredDatasources[index]

			logger.Info("updating datasource", "datasource", desiredDatasource.UID)
			desiredDatasource, err = s.updateDatasource(desiredDatasource)
			if err != nil {
				return nil, err
			}
			// Set the ID of the updated datasource
			desiredDatasources[index] = desiredDatasource
			logger.Info("updated datasource", "datasource", desiredDatasource.UID)
		} else {
			if strings.HasPrefix(currentDatasource.UID, datasourceUIDPrefix) {
				// Delete the datasource as it is no longer desired
				logger.Info("deleting datasource", "datasource", currentDatasource.UID)
				err := s.deleteDatasource(currentDatasource.UID)
				if err != nil {
					return nil, err
				}
				logger.Info("deleted datasource", "datasource", currentDatasource.UID)
			}
		}
	}

	// Create any new datasources that do not exist yet
	for index := range desiredDatasources {
		desiredDatasource := desiredDatasources[index]

		// If the datasource ID is 0, it means it does not exist yet and needs to be created
		// We already took care of updating existing datasources ID in the previous loop
		if desiredDatasource.ID == 0 {
			logger.Info("creating datasource", "datasource", desiredDatasource.UID)
			desiredDatasource, err = s.createDatasource(desiredDatasource)
			if err != nil {
				return nil, err
			}
			logger.Info("datasource created", "datasource", desiredDatasource.UID)
			// Set the ID of the created datasource
			desiredDatasources[index] = desiredDatasource
		}
	}

	return desiredDatasources, nil
}

// generateDatasources generates the list of datasources for a given organization.
// It configures the datasources to use the appropriate multi-tenant headers based on the organization's tenant IDs.
// It returns the list of desired datasources.
func (s *Service) generateDatasources(organization Organization) (datasources []Datasource) {
	// Multi-tenant header value is a pipe-separated list of tenant IDs
	multiTenantIDsHeaderValue := strings.Join(organization.TenantIDs, "|")

	// Add Loki datasource
	lokiDatasource := DatasourceLoki().Merge(Datasource{
		Name: "Loki",
		UID:  LokiDatasourceUID,
		JSONData: map[string]any{
			"httpHeaderName1": common.OrgIDHeader,
		},
		SecureJSONData: map[string]string{
			"httpHeaderValue1": multiTenantIDsHeaderValue,
		},
	})

	// Add tracing integration if tracing is enabled
	if s.cfg.Tracing.Enabled {
		// Add derived fields for Loki to Tempo integration
		lokiDatasource.JSONData["derivedFields"] = []map[string]any{
			{
				"name":          "traceID",
				"matcherRegex":  "[tT]race_?[Ii][dD]\"?[:=](\\w+)",
				"datasourceUid": TempoDatasourceUID,
				// Open a new tab when clicking the link
				"targetBlank":     true,
				"url":             "${__value.raw}",
				"urlDisplayLabel": "Trace ID",
			},
		}

	}

	datasources = append(datasources, lokiDatasource)

	// Add Mimir datasource
	datasources = append(datasources, DatasourceMimir().Merge(Datasource{
		Name:      "Mimir",
		UID:       MimirDatasourceUID,
		IsDefault: true,
		JSONData: map[string]any{
			"httpHeaderName1": common.OrgIDHeader,
		},
		SecureJSONData: map[string]string{
			"httpHeaderValue1": multiTenantIDsHeaderValue,
		},
	}))

	// Add Alertmanager datasource
	datasources = append(datasources, DatasourceMimirAlertmanager().Merge(Datasource{
		Name: "Mimir Alertmanager",
		UID:  MimirAlertmanagerDatasourceUID,
		JSONData: map[string]any{
			"httpHeaderName1": common.OrgIDHeader,
		},
		SecureJSONData: map[string]string{
			"httpHeaderValue1": multiTenantIDsHeaderValue,
		},
	}))

	// Add Tempo datasource - only if tracing is enabled
	if s.cfg.Tracing.Enabled {
		datasources = append(datasources, DatasourceTempo().Merge(Datasource{
			Name: "Tempo",
			UID:  TempoDatasourceUID,
			JSONData: map[string]any{
				"httpHeaderName1": common.OrgIDHeader,
			},
			SecureJSONData: map[string]string{
				"httpHeaderValue1": multiTenantIDsHeaderValue,
			},
		}))
	}

	if organization.Name == SharedOrg.Name {
		// Add Mimir Cardinality datasources to the "Shared Org"
		datasources = append(datasources, DatasourceMimirCardinality().Merge(Datasource{
			JSONData: map[string]any{
				"httpHeaderName1": common.OrgIDHeader,
			},
			SecureJSONData: map[string]string{
				"httpHeaderValue1": strings.Join(organization.TenantIDs, "|"),
			},
		}))
	}

	return datasources
}

// createDatasource creates the given datasource in Grafana.
// It returns the created datasource with its ID set.
func (s *Service) createDatasource(datasource Datasource) (Datasource, error) {
	created, err := s.grafanaClient.Datasources().AddDataSource(&models.AddDataSourceCommand{
		UID:            datasource.UID,
		Name:           datasource.Name,
		Type:           datasource.Type,
		URL:            datasource.URL,
		IsDefault:      datasource.IsDefault,
		JSONData:       models.JSON(datasource.JSONData),
		SecureJSONData: datasource.SecureJSONData,
		Access:         models.DsAccess(datasource.Access),
	})
	if err != nil {
		return Datasource{}, fmt.Errorf("failed to create datasource %q: %w", datasource.UID, err)
	}

	if created.Payload == nil || created.Payload.ID == nil {
		return Datasource{}, fmt.Errorf("failed to create datasource %q: response payload or ID is nil", datasource.UID)
	}

	datasource.ID = *created.Payload.ID

	return datasource, nil
}

// updateDatasource updates the given datasource in Grafana.
// The datasource is identified by its UID.
// It returns the updated datasource with its ID set.
func (s *Service) updateDatasource(datasource Datasource) (Datasource, error) {
	resp, err := s.grafanaClient.Datasources().UpdateDataSourceByUID(datasource.UID, &models.UpdateDataSourceCommand{
		UID:            datasource.UID,
		Name:           datasource.Name,
		Type:           datasource.Type,
		URL:            datasource.URL,
		IsDefault:      datasource.IsDefault,
		JSONData:       models.JSON(datasource.JSONData),
		SecureJSONData: datasource.SecureJSONData,
		Access:         models.DsAccess(datasource.Access),
	})
	if err != nil {
		return Datasource{}, fmt.Errorf("failed to update datasource %q: %w", datasource.UID, err)
	}

	if resp.Payload == nil || resp.Payload.ID == nil {
		return Datasource{}, fmt.Errorf("failed to update datasource %q: response payload or ID is nil", datasource.UID)
	}

	datasource.ID = *resp.Payload.ID

	return datasource, nil
}

// deleteDatasource deletes the datasource with the given UID.
// If the datasource does not exist, no error is returned.
func (s *Service) deleteDatasource(uid string) error {
	_, err := s.grafanaClient.Datasources().DeleteDataSourceByUID(uid)
	if err != nil {
		var notFound *datasources.DeleteDataSourceByUIDNotFound
		if !errors.As(err, &notFound) {
			return fmt.Errorf("failed to delete datasource %q: %w", uid, err)
		}
	}

	return nil
}
