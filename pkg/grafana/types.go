package grafana

import (
	"strings"

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
