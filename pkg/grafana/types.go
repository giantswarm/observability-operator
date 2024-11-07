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
	copy := make(map[string]interface{})
	// Copy from the original map to the target map
	for key, value := range d.JSONData {
		copy[key] = value
	}
	// Add tenant header name
	copy["httpHeaderName1"] = "X-Scope-OrgID"
	return copy
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
