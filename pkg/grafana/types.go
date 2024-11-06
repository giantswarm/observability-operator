package grafana

import "github.com/grafana/grafana-openapi-client-go/models"

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

func (d Datasource) buildJSONData() models.JSON {
	copy := make(map[string]interface{})
	// Copy from the original map to the target map
	for key, value := range d.JSONData {
		copy[key] = value
	}
	// Add tenant header name
	copy["httpHeaderName1"] = "X-Scope-OrgID"
	return models.JSON(copy)
}

func (d Datasource) buildSecureJSONData(organization Organization) map[string]string {
	return map[string]string{
		"httpHeaderValue1": organization.TenantID,
	}
}
