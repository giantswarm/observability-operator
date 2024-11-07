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
	datasource.ID = id
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
	if d.Name != "Loki" {
		return map[string]string{
			// We do not support multi-tenancy for Mimir yet
			"httpHeaderValue1": "anonymous",
		}
	}
	return map[string]string{
		"httpHeaderValue1": organization.TenantID,
	}
}
