package grafana

type Organization struct {
	ID       int64
	Name     string
	TenantID string
}

type Datasource struct {
	ID        int64
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
	d.JSONData["httpHeaderName1"] = "X-Scope-OrgID"

	return d.JSONData
}

func (d Datasource) buildSecureJSONData(organization Organization) map[string]string {
	tenant := organization.TenantID
	if d.Name != "Loki" {
		// We do not support multi-tenancy for Mimir yet
		tenant = "anonymous"
	}
	return map[string]string{
		"httpHeaderValue1": tenant,
	}
}
