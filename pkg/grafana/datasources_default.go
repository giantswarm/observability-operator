package grafana

const (
	datasourceProxyAccessMode = "proxy"
)

// datasourceGenerator is a function that returns a Datasource.
// It is used to create new instances of predefined datasources.
// This ensures that each call returns a new instance, preventing unintended modifications
// to shared instances.
type datasourceGenerator func() Datasource

// buildDatasourceGenerator is a helper function that takes a Datasource and returns a datasourceGenerator.
func buildDatasourceGenerator(d Datasource) datasourceGenerator {
	return func() Datasource {
		return Datasource{}.Merge(d)
	}
}

var (
	// Datasource for Mimir Alertmanager
	DatasourceMimirAlertmanager = buildDatasourceGenerator(Datasource{
		Type:   "alertmanager",
		URL:    "http://mimir-alertmanager.mimir.svc:8080",
		Access: datasourceProxyAccessMode,
		JSONData: map[string]any{
			"handleGrafanaManagedAlerts": false,
			"implementation":             "mimir",
		},
	})

	// Datasource for Loki
	DatasourceLoki = buildDatasourceGenerator(Datasource{
		Type:   "loki",
		URL:    "http://loki-gateway.loki.svc",
		Access: datasourceProxyAccessMode,
	})

	// Datasource for Mimir
	DatasourceMimir = buildDatasourceGenerator(Datasource{
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
	})

	// Datasource for Mimir to query cardinality data
	DatasourceMimirCardinality = buildDatasourceGenerator(Datasource{
		Type:   "marcusolsson-json-datasource",
		Name:   "Mimir Cardinality",
		UID:    "gs-mimir-cardinality",
		URL:    "http://mimir-gateway.mimir.svc:8080/prometheus/api/v1/cardinality/",
		Access: datasourceProxyAccessMode,
	})
)
