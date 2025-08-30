package grafana

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

func (s *Service) SetupOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	var errs []error

	// Configure the organization's datasources
	if err := s.ConfigureDatasources(ctx, grafanaOrganization); err != nil {
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

func (s *Service) DeleteOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	// Delete organization in Grafana if it exists
	var organization = NewOrganization(grafanaOrganization)
	if grafanaOrganization.Status.OrgID > 0 {
		return s.deleteOrganization(ctx, organization)
	}

	return nil
}

// ConfigureOrganization creates or updates the organization in Grafana and returns the organization ID.
func (s *Service) ConfigureOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) (int64, error) {
	organization := NewOrganization(grafanaOrganization)

	err := s.UpsertOrganization(ctx, &organization)
	if err != nil {
		return -1, fmt.Errorf("ConfigureOrganization: failed to configure organization: %w", err)
	}

	return organization.ID, nil
}

// ConfigureDatasources ensures the datasources for the given GrafanaOrganization are up to date.
func (s *Service) ConfigureDatasources(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	logger.Info("configuring datasources")

	var organization = NewOrganization(grafanaOrganization)
	datasources, err := s.ConfigureDefaultDatasources(ctx, organization)
	if err != nil {
		return fmt.Errorf("ConfigureDatasources: failed to configure default datasources: %w", err)
	}

	var configuredDatasources = make([]v1alpha1.DataSource, len(datasources))
	for i, datasource := range datasources {
		configuredDatasources[i] = v1alpha1.DataSource{
			ID:   datasource.ID,
			Name: datasource.Name,
		}
	}

	logger.Info("updating datasources in the grafanaOrganization status")
	grafanaOrganization.Status.DataSources = configuredDatasources
	if err := s.client.Status().Update(ctx, grafanaOrganization); err != nil {
		return fmt.Errorf("ConfigureDatasources: failed to update grafanaOrganization status: %w", err)
	}
	logger.Info("updated datasources in the grafanaOrganization status")

	logger.Info("configured datasources")

	return nil
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
