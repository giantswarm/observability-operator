package grafana

import (
	"fmt"
	"maps"
	"slices"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/api/v1alpha2"
)

// TenantConfig represents a tenant configuration with its access types
type TenantConfig struct {
	Name  string
	Types []string
}

type Organization struct {
	ID      int64
	Name    string
	Tenants []TenantConfig
	Admins  []string
	Editors []string
	Viewers []string
}

// GetTenantIDs returns a slice of all tenant IDs in the organization
func (o *Organization) GetTenantIDs() []string {
	tenantIDs := make([]string, len(o.Tenants))
	for i, tenant := range o.Tenants {
		tenantIDs[i] = tenant.Name
	}
	return tenantIDs
}

// ValidateTenantAccess validates that the organization follows business rules for tenant access.
// Returns an error if giantswarm tenant has alerting access in a non-Giant Swarm organization.
func (o *Organization) ValidateTenantAccess() error {
	// Only Giant Swarm organization can have giantswarm tenant with alerting access
	if o.Name != "Giant Swarm" {
		for _, tenant := range o.Tenants {
			if tenant.Name == "giantswarm" && slices.Contains(tenant.Types, string(v1alpha2.TenantTypeAlerting)) {
				return fmt.Errorf("giantswarm tenant cannot have alerting access in organization %q", o.Name)
			}
		}
	}
	return nil
}

// GetAlertingTenants returns a list of tenant configs that have alerting access
func (o *Organization) GetAlertingTenants() []TenantConfig {
	var alertingTenants []TenantConfig
	for _, tenant := range o.Tenants {
		if slices.Contains(tenant.Types, string(v1alpha2.TenantTypeAlerting)) {
			alertingTenants = append(alertingTenants, tenant)
		}
	}
	return alertingTenants
}

// NewOrganization creates a new Organization instance from a v1alpha1 GrafanaOrganization custom resource.
func NewOrganization(grafanaOrganization *v1alpha1.GrafanaOrganization) Organization {
	tenants := make([]TenantConfig, len(grafanaOrganization.Spec.Tenants))

	for i, tenant := range grafanaOrganization.Spec.Tenants {
		tenantID := string(tenant)
		// For v1alpha1, all tenants have both data and alerting access for backward compatibility
		tenants[i] = TenantConfig{
			Name:  tenantID,
			Types: []string{string(v1alpha2.TenantTypeData), string(v1alpha2.TenantTypeAlerting)},
		}
	}

	orgID := grafanaOrganization.Status.OrgID
	// Shared Org is the only exception to the rule as we know it's ID will always be 1
	if grafanaOrganization.Spec.DisplayName == SharedOrg.Name {
		orgID = SharedOrg.ID
	}

	return Organization{
		ID:      orgID,
		Name:    grafanaOrganization.Spec.DisplayName,
		Tenants: tenants,
		Admins:  grafanaOrganization.Spec.RBAC.Admins,
		Editors: grafanaOrganization.Spec.RBAC.Editors,
		Viewers: grafanaOrganization.Spec.RBAC.Viewers,
	}
}

// NewOrganizationFromV2 creates a new Organization instance from a v1alpha2 GrafanaOrganization custom resource.
func NewOrganizationFromV2(grafanaOrganization *v1alpha2.GrafanaOrganization) Organization {
	tenants := make([]TenantConfig, len(grafanaOrganization.Spec.Tenants))

	for i, tenant := range grafanaOrganization.Spec.Tenants {
		tenantID := string(tenant.Name)

		types := make([]string, len(tenant.Types))
		for j, tType := range tenant.Types {
			types[j] = string(tType)
		}
		// Default to data-only if no types specified
		if len(types) == 0 {
			types = []string{string(v1alpha2.TenantTypeData)}
		}

		tenants[i] = TenantConfig{
			Name:  tenantID,
			Types: types,
		}
	}

	orgID := grafanaOrganization.Status.OrgID
	// Shared Org is the only exception to the rule as we know it's ID will always be 1
	if grafanaOrganization.Spec.DisplayName == SharedOrg.Name {
		orgID = SharedOrg.ID
	}

	return Organization{
		ID:      orgID,
		Name:    grafanaOrganization.Spec.DisplayName,
		Tenants: tenants,
		Admins:  grafanaOrganization.Spec.RBAC.Admins,
		Editors: grafanaOrganization.Spec.RBAC.Editors,
		Viewers: grafanaOrganization.Spec.RBAC.Viewers,
	}
}

// Datasource represents a Grafana datasource.
type Datasource struct {
	ID             int64
	UID            string
	Name           string
	Type           string
	URL            string
	IsDefault      bool
	Access         string
	JSONData       map[string]any
	SecureJSONData map[string]string
}

// Merge merges the non-zero fields from src into d and returns the result.
// For JSONData and SecureJSONData maps, it merges the key-value pairs, with src's values taking precedence in case of key conflicts.
func (d Datasource) Merge(src Datasource) Datasource {
	if !IsZeroVar(src.ID) {
		d.ID = src.ID
	}
	if !IsZeroVar(src.UID) {
		d.UID = src.UID
	}
	if !IsZeroVar(src.Name) {
		d.Name = src.Name
	}
	if !IsZeroVar(src.Type) {
		d.Type = src.Type
	}
	if !IsZeroVar(src.URL) {
		d.URL = src.URL
	}
	if d.IsDefault != src.IsDefault {
		d.IsDefault = src.IsDefault
	}
	if !IsZeroVar(src.Access) {
		d.Access = src.Access
	}
	if src.JSONData != nil {
		if d.JSONData == nil {
			d.JSONData = make(map[string]any)
		}
		maps.Copy(d.JSONData, src.JSONData)
	}
	if src.SecureJSONData != nil {
		if d.SecureJSONData == nil {
			d.SecureJSONData = make(map[string]string)
		}
		maps.Copy(d.SecureJSONData, src.SecureJSONData)
	}

	return d
}

// IsZeroVar reports whether v is the zero value for its type.
func IsZeroVar[T comparable](v T) bool {
	var zero T
	return v == zero
}
