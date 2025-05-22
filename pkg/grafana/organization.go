package grafana

import (
	"context"
	stderrors "errors"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

func (s *Service) SetupOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	var errs []error

	// Update the datasources in the CR's status
	if err := s.ConfigureDatasources(ctx, grafanaOrganization); err != nil {
		errs = append(errs, err)
	}

	// Configure Grafana RBAC
	if err := s.ConfigureGrafanaSSO(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.WithStack(stderrors.Join(errs...))
	}

	return nil
}

func (s *Service) DeleteOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	// Delete organization in Grafana if it exists
	var organization = NewOrganization(grafanaOrganization)
	if grafanaOrganization.Status.OrgID > 0 {
		err := s.deleteOrganization(ctx, organization)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (s *Service) ConfigureOrganization(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) (int64, error) {
	// Create or update organization in Grafana
	organization := NewOrganization(grafanaOrganization)

	err := s.UpsertOrganization(ctx, &organization)
	if err != nil {
		return -1, errors.WithStack(err)
	}

	return organization.ID, nil
}

func (s *Service) ConfigureDatasources(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	logger.Info("configuring data sources")

	// Create or update organization in Grafana
	var organization = NewOrganization(grafanaOrganization)
	datasources, err := s.ConfigureDefaultDatasources(ctx, organization)
	if err != nil {
		logger.Error(err, "failed to configure the grafanaOrganization with default datasources")
		return errors.WithStack(err)
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
		logger.Error(err, "failed to update the the grafanaOrganization status with datasources information")
		return errors.WithStack(err)
	}
	logger.Info("updated datasources in the grafanaOrganization status")
	logger.Info("configured data sources")

	return nil
}

// ConfigureGrafana ensures the RBAC configuration is set in Grafana.
func (s *Service) ConfigureGrafanaSSO(ctx context.Context) error {
	logger := log.FromContext(ctx)

	organizationList := v1alpha1.GrafanaOrganizationList{}
	err := s.client.List(ctx, &organizationList)
	if err != nil {
		logger.Error(err, "failed to list grafana organizations")
		return errors.WithStack(err)
	}

	// Configure SSO settings in Grafana
	organizations := make([]Organization, len(organizationList.Items))
	for i, organization := range organizationList.Items {
		organizations[i] = NewOrganization(&organization)
	}
	err = s.ConfigureSSOSettings(ctx, organizations)
	if err != nil {
		logger.Error(err, "failed to configure grafanaOrganization with SSO settings")
		return errors.WithStack(err)
	}

	return nil
}
