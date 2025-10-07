package grafana

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

func (s *Service) SetupOrganization(ctx context.Context, organization Organization) error {
	var errs []error

	// Configure the organization's datasources
	if _, err := s.ConfigureDatasources(ctx, organization); err != nil {
		errs = append(errs, err)
	}

	// Configure the organization's authorization settings
	if err := s.ConfigureGrafanaSSO(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("SetupOrganization: failed to setup organization: %w", errors.Join(errs...))
	}

	return nil
}

func (s *Service) DeleteOrganization(ctx context.Context, organization Organization) error {
	// Delete organization in Grafana if it exists
	if organization.ID > 0 {
		return s.deleteOrganization(ctx, organization)
	}

	return nil
}

// ConfigureOrganization creates or updates the organization in Grafana and returns the organization ID.
func (s *Service) ConfigureOrganization(ctx context.Context, organization Organization) (int64, error) {
	// Validate tenant access rules
	if err := organization.ValidateTenantAccess(); err != nil {
		return -1, fmt.Errorf("ConfigureOrganization: %w", err)
	}

	err := s.UpsertOrganization(ctx, &organization)
	if err != nil {
		return -1, fmt.Errorf("ConfigureOrganization: failed to configure organization: %w", err)
	}

	return organization.ID, nil
}

// ConfigureDatasources ensures the datasources for the given GrafanaOrganization are up to date.
// It returns the list of configured datasources.
func (s *Service) ConfigureDatasources(ctx context.Context, organization Organization) ([]Datasource, error) {
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

// ConfigureGrafana ensures the SSO settings in Grafana are up to date based on the current
// list of GrafanaOrganization CRs.
func (s *Service) ConfigureGrafanaSSO(ctx context.Context) error {
	organizationList := v1alpha1.GrafanaOrganizationList{}
	err := s.client.List(ctx, &organizationList)
	if err != nil {
		return fmt.Errorf("ConfigureGrafanaSSO: failed to list grafana organizations: %w", err)
	}

	// Configure SSO settings in Grafana
	organizations := make([]Organization, 0)
	for _, organization := range organizationList.Items {
		if !organization.GetDeletionTimestamp().IsZero() {
			// Skip organizations that are being deleted
			// see https://github.com/giantswarm/observability-operator/pull/525
			continue
		}

		organizations = append(organizations, NewOrganization(&organization))
	}

	err = s.ConfigureSSOSettings(ctx, organizations)
	if err != nil {
		return fmt.Errorf("ConfigureGrafanaSSO: failed to configure SSO settings: %w", err)
	}

	return nil
}
