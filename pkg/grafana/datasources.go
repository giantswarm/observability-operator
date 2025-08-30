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

	// Configure Grafana client to use the correct organization
	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(organization.ID)
	defer s.grafanaClient.WithOrgID(currentOrgID)

	// Generate the desired datasources for the organization
	desiredDatasources := s.generateDatasources(ctx, organization)

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
			desiredDatasource, err = s.updateDatasource(ctx, desiredDatasource)
			if err != nil {
				return nil, err
			}
			// Set the ID of the updated datasource
			desiredDatasources[index] = desiredDatasource
			logger.Info("updated datasource", "datasource", desiredDatasource.UID)
		} else {
			// Delete the datasource as it is no longer desired
			logger.Info("deleting datasource", "datasource", currentDatasource.UID)
			err := s.deleteDatasource(ctx, currentDatasource.UID)
			if err != nil {
				return nil, err
			}
			logger.Info("deleted datasource", "datasource", currentDatasource.UID)
		}
	}

	// Create any new datasources that do not exist yet
	for index := range desiredDatasources {
		desiredDatasource := desiredDatasources[index]

		if desiredDatasource.ID == 0 {
			logger.Info("creating datasource", "datasource", desiredDatasource.UID)
			desiredDatasource, err = s.createDatasource(ctx, desiredDatasource)
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
func (s *Service) generateDatasources(ctx context.Context, organization Organization) (datasources []Datasource) {
	// Multi-tenant header value is a pipe-separated list of tenant IDs
	multiTenantIDsHeaderValue := strings.Join(organization.TenantIDs, "|")

	// Create Alertmanager datasource
	// TODO: might need to change to multiple datasources: 1 per tenant
	dsAlertmanager := datasourceAlertmanager
	dsAlertmanager.setJSONData("httpHeaderName1", common.OrgIDHeader)
	dsAlertmanager.setSecureJSONData("httpHeaderValue1", multiTenantIDsHeaderValue)
	datasources = append(datasources, dsAlertmanager)

	// Create Loki datasource
	lokiDatasource := datasourceLoki
	lokiDatasource.setJSONData("httpHeaderName1", common.OrgIDHeader)
	lokiDatasource.setSecureJSONData("httpHeaderValue1", multiTenantIDsHeaderValue)
	datasources = append(datasources, lokiDatasource)

	// Create Mimir datasource for metrics only
	mimirDatasource := datasourceMimir
	mimirDatasource.Name = "Mimir"
	mimirDatasource.UID = "gs-mimir"
	mimirDatasource.IsDefault = true
	mimirDatasource.setJSONData("allowAsRecordingRulesTarget", false)
	mimirDatasource.setJSONData("manageAlerts", false)
	mimirDatasource.setJSONData("httpHeaderName1", common.OrgIDHeader)
	mimirDatasource.setSecureJSONData("httpHeaderValue1", multiTenantIDsHeaderValue)
	datasources = append(datasources, mimirDatasource)

	// Create one Mimir datasource per tenant for rules
	for _, tenant := range organization.TenantIDs {
		mimirRuleDatasource := datasourceMimir
		mimirRuleDatasource.Name = fmt.Sprintf("Mimir - %s", tenant)
		mimirRuleDatasource.UID = fmt.Sprintf("gs-mimir-%s", tenant)
		mimirRuleDatasource.setJSONData("httpHeaderName1", common.OrgIDHeader)
		mimirRuleDatasource.setSecureJSONData("httpHeaderValue1", tenant)
		datasources = append(datasources, mimirRuleDatasource)
	}

	if organization.Name == SharedOrg.Name {
		// Add extra public datasources to the "Shared Org"
		for _, extraDatasource := range extraPublicDatasources {
			extraDatasource.setJSONData("httpHeaderName1", common.OrgIDHeader)
			extraDatasource.setSecureJSONData("httpHeaderValue1", multiTenantIDsHeaderValue)
			datasources = append(datasources, extraDatasource)
		}
	}

	return datasources
}

// createDatasource creates the given datasource in Grafana.
// It returns the created datasource with its ID set.
func (s *Service) createDatasource(ctx context.Context, datasource Datasource) (Datasource, error) {
	created, err := s.grafanaClient.Datasources().AddDataSource(
		&models.AddDataSourceCommand{
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
func (s *Service) updateDatasource(ctx context.Context, datasource Datasource) (Datasource, error) {
	resp, err := s.grafanaClient.Datasources().UpdateDataSourceByUID(
		datasource.UID,
		&models.UpdateDataSourceCommand{
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
func (s *Service) deleteDatasource(ctx context.Context, uid string) error {
	_, err := s.grafanaClient.Datasources().DeleteDataSourceByUID(uid)
	if err != nil {
		var notFound *datasources.DeleteDataSourceByUIDNotFound
		if !errors.As(err, &notFound) {
			return fmt.Errorf("failed to delete datasource %q: %w", uid, err)
		}
	}

	return nil
}
