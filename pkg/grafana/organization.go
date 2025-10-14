package grafana

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

func (s *Service) DeleteOrganization(ctx context.Context, organization *organization.Organization) error {
	// Delete organization in Grafana if it exists
	if organization.ID() > 0 {
		return s.deleteOrganization(ctx, organization)
	}

	return nil
}

// ConfigureOrganization creates or updates the organization in Grafana and returns the organization ID.
func (s *Service) ConfigureOrganization(ctx context.Context, organization *organization.Organization) (int64, error) {
	err := s.UpsertOrganization(ctx, organization)
	if err != nil {
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
		return nil, fmt.Errorf("ConfigureDatasources: failed to configure default datasources: %w", err)
	}

	logger.Info("configured datasources")

	return datasources, nil
}
