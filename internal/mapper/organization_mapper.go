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
	tenantIDs := make([]string, len(grafanaOrganization.Spec.Tenants))
	for i, tenant := range grafanaOrganization.Spec.Tenants {
		tenantIDs[i] = string(tenant.Name)
	}

	return organization.New(
		grafanaOrganization.Status.OrgID,
		grafanaOrganization.Spec.DisplayName,
		tenantIDs,
		grafanaOrganization.Spec.RBAC.Admins,
		grafanaOrganization.Spec.RBAC.Editors,
		grafanaOrganization.Spec.RBAC.Viewers,
	)
}
