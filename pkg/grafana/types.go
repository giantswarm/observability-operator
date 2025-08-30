package grafana

import (
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

type Datasource struct {
	ID             int64
	UID            string
	Name           string
	Type           string
	URL            string
	IsDefault      bool
	JSONData       map[string]any
	SecureJSONData map[string]string
	Access         string
}

func (d *Datasource) setJSONData(key string, value any) {
	if d.JSONData == nil {
		d.JSONData = make(map[string]any)
	}

	d.JSONData[key] = value
}

func (d *Datasource) setSecureJSONData(key, value string) {
	if d.SecureJSONData == nil {
		d.SecureJSONData = make(map[string]string)
	}

	d.SecureJSONData[key] = value
}
