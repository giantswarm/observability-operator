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

// ConfigureOrganization creates or updates the organization in Grafana and returns
// the organization with its resolved ID. The input is not mutated; callers must use
// the returned organization for any follow-up calls that depend on the ID.
func (s *Service) ConfigureOrganization(ctx context.Context, org *organization.Organization) (*organization.Organization, error) {
	resolved, err := s.UpsertOrganization(ctx, org)
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureOrg).Inc()
		return nil, fmt.Errorf("ConfigureOrganization: failed to configure organization: %w", err)
	}

	return resolved, nil
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
