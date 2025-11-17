package v1alpha1

import (
	"github.com/giantswarm/observability-operator/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)


// ConvertTo converts this GrafanaOrganization to the Hub version (v1alpha2)
func (src *GrafanaOrganization) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.GrafanaOrganization)

	// Convert metadata
	dst.ObjectMeta = src.ObjectMeta

	// Convert spec
	dst.Spec.DisplayName = src.Spec.DisplayName
	dst.Spec.RBAC = (*v1alpha2.RBAC)(src.Spec.RBAC)

	// Convert tenants from []TenantID to []TenantConfig
	dst.Spec.Tenants = make([]v1alpha2.TenantConfig, len(src.Spec.Tenants))
	for i, tenantID := range src.Spec.Tenants {
		dst.Spec.Tenants[i] = v1alpha2.TenantConfig{
			Name: v1alpha2.TenantID(tenantID),
			// Default to both types for backward compatibility
			Types: []v1alpha2.TenantType{v1alpha2.TenantTypeData, v1alpha2.TenantTypeAlerting},
		}
	}

	// Convert status
	dst.Status.OrgID = src.Status.OrgID
	dst.Status.DataSources = make([]v1alpha2.DataSource, len(src.Status.DataSources))
	for i, ds := range src.Status.DataSources {
		dst.Status.DataSources[i] = v1alpha2.DataSource{
			ID:   ds.ID,
			Name: ds.Name,
		}
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version
func (dst *GrafanaOrganization) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.GrafanaOrganization)

	// Convert metadata
	dst.ObjectMeta = src.ObjectMeta

	// Convert spec
	dst.Spec.DisplayName = src.Spec.DisplayName
	dst.Spec.RBAC = (*RBAC)(src.Spec.RBAC)

	// Convert tenants from []TenantConfig to []TenantID
	// Note: This loses type information, but maintains compatibility
	dst.Spec.Tenants = make([]TenantID, len(src.Spec.Tenants))
	for i, tenant := range src.Spec.Tenants {
		dst.Spec.Tenants[i] = TenantID(tenant.Name)
	}

	// Convert status
	dst.Status.OrgID = src.Status.OrgID
	dst.Status.DataSources = make([]DataSource, len(src.Status.DataSources))
	for i, ds := range src.Status.DataSources {
		dst.Status.DataSources[i] = DataSource{
			ID:   ds.ID,
			Name: ds.Name,
		}
	}

	return nil
}
