package grafana

import (
	"strings"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	common "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

type Organization struct {
	ID        int64
	Name      string
	TenantIDs []string
	Admins    []string
	Editors   []string
	Viewers   []string
}

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
	ID        int64
	UID       string
	Name      string
	IsDefault bool
	Type      string
	URL       string
	Access    string
	JSONData  map[string]interface{}
}

func (d Datasource) withID(id int64) Datasource {
	d.ID = id
	return d
}

func (d Datasource) buildJSONData() map[string]interface{} {
	if d.JSONData == nil {
		d.JSONData = make(map[string]interface{})
	}

	// Add tenant header name
	d.JSONData["httpHeaderName1"] = common.OrgIDHeader

	return d.JSONData
}

func (d Datasource) buildSecureJSONData(organization Organization) map[string]string {
	tenantIDs := organization.TenantIDs
	if d.UID == mimirOldDatasourceUID {
		// For the old Mimir datasource, we need to use the anonymous tenant
		tenantIDs = []string{"anonymous"}
	}
	return map[string]string{
		"httpHeaderValue1": strings.Join(tenantIDs, "|"),
	}
}
