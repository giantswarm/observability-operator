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
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// ConfigureDatasource ensures the datasources for the given organization are up to date.
// It creates, updates, or deletes datasources as necessary to match the desired state.
func (s *Service) ConfigureDatasource(ctx context.Context, organization *organization.Organization) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	desiredDatasources := s.generateDatasources(organization)
	client := s.grafanaClient.WithOrgID(organization.ID())

	resp, err := client.Datasources().GetDataSources()
	if err != nil {
		return nil, fmt.Errorf("failed to get configured datasources: %w", err)
	}

	// Update or delete existing datasources
	for _, currentDatasource := range resp.GetPayload() {
		index := slices.IndexFunc(desiredDatasources, func(d Datasource) bool {
			return d.UID == currentDatasource.UID
		})

		if index >= 0 {
			desiredDatasource := desiredDatasources[index]

			logger.Info("updating datasource", "datasource", desiredDatasource.UID)
			desiredDatasource, err = updateDatasource(client, desiredDatasource)
			if err != nil {
				return nil, err
			}
			desiredDatasources[index] = desiredDatasource
			logger.Info("updated datasource", "datasource", desiredDatasource.UID)
		} else if strings.HasPrefix(currentDatasource.UID, datasourceUIDPrefix) {
			logger.Info("deleting datasource", "datasource", currentDatasource.UID)
			if err := deleteDatasource(client, currentDatasource.UID); err != nil {
				return nil, err
			}
			logger.Info("deleted datasource", "datasource", currentDatasource.UID)
		}
	}

	// Create any new datasources that do not exist yet
	for index := range desiredDatasources {
		desiredDatasource := desiredDatasources[index]

		// ID == 0 means it doesn't exist yet — the update loop above already
		// stamped IDs on the ones that did.
		if desiredDatasource.ID == 0 {
			logger.Info("creating datasource", "datasource", desiredDatasource.UID)
			desiredDatasource, err = createDatasource(client, desiredDatasource)
			if err != nil {
				return nil, err
			}
			logger.Info("datasource created", "datasource", desiredDatasource.UID)
			desiredDatasources[index] = desiredDatasource
		}
	}

	return desiredDatasources, nil
}

// generateDatasources generates the list of datasources for a given organization.
// It configures the datasources to use the appropriate multi-tenant headers based on the organization's tenant IDs.
// It returns the list of desired datasources.
func (s *Service) generateDatasources(org *organization.Organization) (datasources []Datasource) {
	// Multi-tenant header value is a pipe-separated list of tenant IDs for data reading
	multiTenantIDsHeaderValue := strings.Join(org.TenantIDs(), "|")
	alertingTenants := org.GetAlertingTenants()

	// 1. Create multi-tenant data reading datasources

	// Add Loki datasource for multi-tenant data reading
	lokiDatasource := DatasourceLoki().Merge(Datasource{
		Name: LokiDatasourceName,
		UID:  LokiDatasourceUID,
		URL:  s.cfg.Grafana.Datasources.LokiURL,
		JSONData: map[string]any{
			"httpHeaderName1": common.OrgIDHeader,
			"manageAlerts":    len(alertingTenants) == 1,
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
				"matcherRegex":  traceIDRegex,
				"datasourceUid": TempoDatasourceUID,
				// Open a new tab when clicking the link
				"targetBlank":     true,
				"url":             "${__value.raw}",
				"urlDisplayLabel": "Trace ID",
			},
		}
	}

	datasources = append(datasources, lokiDatasource)

	// Add Mimir datasource for multi-tenant data reading
	mimirDatasource := DatasourceMimir().Merge(Datasource{
		Name:      MimirDatasourceName,
		UID:       MimirDatasourceUID,
		URL:       s.cfg.Grafana.Datasources.MimirURL,
		IsDefault: true,
		JSONData: map[string]any{
			"httpHeaderName1": common.OrgIDHeader,
			"manageAlerts":    len(alertingTenants) == 1,
		},
		SecureJSONData: map[string]string{
			"httpHeaderValue1": multiTenantIDsHeaderValue,
		},
	})

	// Add tracing integration if tracing is enabled
	if s.cfg.Tracing.Enabled {
		// Add exemplar destinations for Mimir to Tempo integration
		mimirDatasource.JSONData["exemplarTraceIdDestinations"] = []map[string]any{
			{
				"name":            "traceID",
				"datasourceUid":   TempoDatasourceUID,
				"urlDisplayLabel": "View in Tempo",
			},
		}
	}

	datasources = append(datasources, mimirDatasource)

	// Add Tempo datasource - only if tracing is enabled
	if s.cfg.Tracing.Enabled {
		tempoDatasource := DatasourceTempo().Merge(Datasource{
			Name: TempoDatasourceName,
			UID:  TempoDatasourceUID,
			URL:  s.cfg.Grafana.Datasources.TempoURL,
			JSONData: map[string]any{
				"httpHeaderName1": common.OrgIDHeader,
			},
			SecureJSONData: map[string]string{
				"httpHeaderValue1": multiTenantIDsHeaderValue,
			},
		})
		datasources = append(datasources, tempoDatasource)
	}

	// 2. Create per-tenant datasources ONLY for alerting-enabled tenants
	// Skip per-tenant datasources for mono-tenant organizations as they're redundant
	if len(alertingTenants) > 1 {
		for _, tenant := range alertingTenants {
			// Per-tenant Loki datasource for log viewing and alerting
			lokiPerTenantDatasource := DatasourceLoki().Merge(Datasource{
				Name: fmt.Sprintf("%s (%s)", LokiDatasourceName, tenant.Name),
				UID:  fmt.Sprintf("%s-%s", LokiDatasourceUID, tenant.Name),
				URL:  s.cfg.Grafana.Datasources.LokiURL,
				JSONData: map[string]any{
					"httpHeaderName1": common.OrgIDHeader,
					"manageAlerts":    true,
				},
				SecureJSONData: map[string]string{
					"httpHeaderValue1": tenant.Name,
				},
			})

			// Add traceToLogs configuration if tracing is enabled
			if s.cfg.Tracing.Enabled {
				lokiPerTenantDatasource.JSONData["derivedFields"] = []map[string]any{
					{
						"name":            "traceID",
						"matcherRegex":    traceIDRegex,
						"datasourceUid":   TempoDatasourceUID,
						"targetBlank":     true,
						"url":             "${__value.raw}",
						"urlDisplayLabel": "Trace ID",
					},
				}
			}

			datasources = append(datasources, lokiPerTenantDatasource)

			// Per-tenant Mimir datasource for rules management
			mimirPerTenantDatasource := DatasourceMimir().Merge(Datasource{
				Name: fmt.Sprintf("%s (%s)", MimirDatasourceName, tenant.Name),
				UID:  fmt.Sprintf("%s-%s", MimirDatasourceUID, tenant.Name),
				URL:  s.cfg.Grafana.Datasources.MimirURL,
				JSONData: map[string]any{
					"httpHeaderName1": common.OrgIDHeader,
					"manageAlerts":    true,
				},
				SecureJSONData: map[string]string{
					"httpHeaderValue1": tenant.Name,
				},
			})

			if s.cfg.Tracing.Enabled {
				mimirPerTenantDatasource.JSONData["exemplarTraceIdDestinations"] = []map[string]any{
					{
						"name":            "traceID",
						"datasourceUid":   TempoDatasourceUID,
						"urlDisplayLabel": "View in Tempo",
					},
				}
			}

			datasources = append(datasources, mimirPerTenantDatasource)

			// Per-tenant Alertmanager datasource for alerts management
			datasources = append(datasources, DatasourceMimirAlertmanager().Merge(Datasource{
				Name: fmt.Sprintf("%s (%s)", MimirAlertmanagerDatasourceName, tenant.Name),
				UID:  fmt.Sprintf("%s-%s", MimirAlertmanagerDatasourceUID, tenant.Name),
				URL:  s.cfg.Grafana.Datasources.MimirAlertmanagerURL,
				JSONData: map[string]any{
					"httpHeaderName1": common.OrgIDHeader,
				},
				SecureJSONData: map[string]string{
					"httpHeaderValue1": tenant.Name,
				},
			}))
		}
	} else if len(alertingTenants) == 1 {
		// For single-alerting-tenant organizations, add alerting datasources without tenant suffix
		tenant := alertingTenants[0]
		// Alertmanager datasource for alerts management
		datasources = append(datasources, DatasourceMimirAlertmanager().Merge(Datasource{
			Name: MimirAlertmanagerDatasourceName,
			UID:  MimirAlertmanagerDatasourceUID,
			URL:  s.cfg.Grafana.Datasources.MimirAlertmanagerURL,
			JSONData: map[string]any{
				"httpHeaderName1": common.OrgIDHeader,
			},
			SecureJSONData: map[string]string{
				"httpHeaderValue1": tenant.Name,
			},
		}))
	}

	// 3. Add special datasources for Shared Org
	if org.Name() == organization.SharedOrg.Name() {
		// Add Mimir Cardinality datasources to the "Shared Org"
		datasources = append(datasources, DatasourceMimirCardinality().Merge(Datasource{
			URL: s.cfg.Grafana.Datasources.MimirCardinalityURL,
			JSONData: map[string]any{
				"httpHeaderName1": common.OrgIDHeader,
			},
			SecureJSONData: map[string]string{
				"httpHeaderValue1": strings.Join(org.TenantIDs(), "|"),
			},
		}))
	}

	return datasources
}

// createDatasource creates the given datasource in Grafana.
// It returns the created datasource with its ID set.
func createDatasource(client grafanaclient.GrafanaClient, datasource Datasource) (Datasource, error) {
	created, err := client.Datasources().AddDataSource(&models.AddDataSourceCommand{
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
func updateDatasource(client grafanaclient.GrafanaClient, datasource Datasource) (Datasource, error) {
	resp, err := client.Datasources().UpdateDataSourceByUID(datasource.UID, &models.UpdateDataSourceCommand{
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
func deleteDatasource(client grafanaclient.GrafanaClient, uid string) error {
	_, err := client.Datasources().DeleteDataSourceByUID(uid)
	if err != nil {
		var notFound *datasources.DeleteDataSourceByUIDNotFound
		if !errors.As(err, &notFound) {
			return fmt.Errorf("failed to delete datasource %q: %w", uid, err)
		}
	}

	return nil
}
