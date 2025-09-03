package grafana

import (
	"maps"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

type Organization struct {
	ID        int64
	Name      string
	TenantIDs []string
	Admins    []string
	Editors   []string
	Viewers   []string
}

// NewOrganization creates a new Organization instance from a GrafanaOrganization custom resource.
func NewOrganization(grafanaOrganization *v1alpha1.GrafanaOrganization) Organization {
	tenantIDs := make([]string, len(grafanaOrganization.Spec.Tenants))
	for i, tenant := range grafanaOrganization.Spec.Tenants {
		tenantIDs[i] = string(tenant)
	}

	orgID := grafanaOrganization.Status.OrgID
	// Shared Org is the only exception to the rule as we know it's ID will always be 1
	if grafanaOrganization.Spec.DisplayName == SharedOrg.Name {
		orgID = SharedOrg.ID
	}

	return Organization{
		ID:        orgID,
		Name:      grafanaOrganization.Spec.DisplayName,
		TenantIDs: tenantIDs,
		Admins:    grafanaOrganization.Spec.RBAC.Admins,
		Editors:   grafanaOrganization.Spec.RBAC.Editors,
		Viewers:   grafanaOrganization.Spec.RBAC.Viewers,
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
