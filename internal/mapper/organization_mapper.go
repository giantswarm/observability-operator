package mapper

import (
	"github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

// OrganizationMapper handles conversion from Kubernetes resources to domain objects
type OrganizationMapper struct{}

// NewOrganizationMapper creates a new organization mapper
func NewOrganizationMapper() *OrganizationMapper {
	return &OrganizationMapper{}
}

// FromGrafanaOrganization converts a v1alpha2.GrafanaOrganization to a domain organization
func (m *OrganizationMapper) FromGrafanaOrganization(grafanaOrganization *v1alpha2.GrafanaOrganization) *organization.Organization {
	tenants := make([]organization.TenantConfig, len(grafanaOrganization.Spec.Tenants))
	for i, tenant := range grafanaOrganization.Spec.Tenants {
		types := make([]string, len(tenant.Types))
		for j, t := range tenant.Types {
			types[j] = string(t)
		}
		// Default to data-only if no types specified
		if len(types) == 0 {
			types = []string{"data"}
		}
		tenants[i] = organization.TenantConfig{
			Name:  string(tenant.Name),
			Types: types,
		}
	}

	return organization.New(
		grafanaOrganization.Status.OrgID,
		grafanaOrganization.Spec.DisplayName,
		tenants,
		grafanaOrganization.Spec.RBAC.Admins,
		grafanaOrganization.Spec.RBAC.Editors,
		grafanaOrganization.Spec.RBAC.Viewers,
	)
}
