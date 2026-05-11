package grafana

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	"github.com/giantswarm/observability-operator/pkg/metrics"
)

func (s *Service) DeleteOrganization(ctx context.Context, organization *organization.Organization) error {
	// Delete organization in Grafana if it exists
	if organization.ID() > 0 {
		if err := s.deleteOrganization(ctx, organization); err != nil {
			metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpDeleteOrg).Inc()
			return err
		}
	}

	return nil
}

// ConfigureOrganization creates or updates the organization in Grafana and returns the organization ID.
// previousName is the last display name the caller persisted for this organization (typically
// GrafanaOrganization.Status.DisplayName); see UpsertOrganization for how it is used.
func (s *Service) ConfigureOrganization(ctx context.Context, organization *organization.Organization, previousName string) (int64, error) {
	err := s.UpsertOrganization(ctx, organization, previousName)
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureOrg).Inc()
		return -1, fmt.Errorf("ConfigureOrganization: failed to configure organization: %w", err)
	}

	return organization.ID(), nil
}

// ConfigureDatasources ensures the datasources for the given organization are up to date.
// Returns the list of configured datasources.
func (s *Service) ConfigureDatasources(ctx context.Context, organization *organization.Organization) ([]Datasource, error) {
	logger := log.FromContext(ctx)

	logger.Info("configuring datasources")

	// Configure the datasources for the organization
	datasources, err := s.ConfigureDatasource(ctx, organization)
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureDatasources).Inc()
		return nil, fmt.Errorf("ConfigureDatasources: failed to configure default datasources: %w", err)
	}

	logger.Info("configured datasources")

	return datasources, nil
}
