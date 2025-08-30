package grafana

const (
	datasourceProxyAccessMode = "proxy"
)

// Alertmanager datasource
func Alertmanager() Datasource {
	return Datasource{
		Type:   "alertmanager",
		URL:    "http://mimir-alertmanager.mimir.svc:8080",
		Access: datasourceProxyAccessMode,
		JSONData: map[string]any{
			"handleGrafanaManagedAlerts": false,
			"implementation":             "mimir",
		},
	}
}

// Loki datasource
func Loki() Datasource {
	return Datasource{
		Name:   "Loki",
		UID:    "gs-loki",
		Type:   "loki",
		URL:    "http://loki-gateway.loki.svc",
		Access: datasourceProxyAccessMode,
	}
}

// Mimir datasource
func Mimir() Datasource {
	return Datasource{
		Type:   "prometheus",
		URL:    "http://mimir-gateway.mimir.svc/prometheus",
		Access: datasourceProxyAccessMode,
		JSONData: map[string]any{
			// Cache matching queries on metadata endpoints within 10 minutes to improve performance
			// and reduce load on the Mimir API.
			"cacheLevel": "Medium",
			"httpMethod": "POST",
			// Enables incremental querying, which allows Grafana to fetch only new data when dashboards are refreshed,
			// rather than re-fetching all data. This is particularly useful for large datasets and improves performance.
			"incrementalQuerying": true,
			"prometheusType":      "Mimir",
			// This is the expected value for the Mimir datasource in Grafana
			"prometheusVersion": "2.9.1",
			"timeInterval":      "60s",
		},
	}
}

// MimirCardinality datasource for querying cardinality data from Mimir
func MimirCardinality() Datasource {
	return Datasource{
		Name:      "Mimir Cardinality",
		UID:       "gs-mimir-cardinality",
		Type:      "marcusolsson-json-datasource",
		URL:       "http://mimir-gateway.mimir.svc:8080/prometheus/api/v1/cardinality/",
		IsDefault: false,
		Access:    datasourceProxyAccessMode,
	}
}
