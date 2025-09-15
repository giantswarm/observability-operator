package grafana

import "fmt"

const (
	// datasourceUIDPrefix is the prefix for all datasources managed by the operator
	datasourceUIDPrefix       = "gs-"
	datasourceProxyAccessMode = "proxy"
)

// Predefined datasources
// These are functions to ensure a new instance is created each time
// so that modifications to the returned struct do not affect others.
// This is important when using them as templates.
// They can be used as-is or merged with custom settings using the Merge method.
var (
	// Datasource for Mimir Alertmanager
	DatasourceMimirAlertmanager = func() Datasource {
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

	// Datasource for Loki
	DatasourceLoki = func() Datasource {
		return Datasource{
			Type:   "loki",
			URL:    "http://loki-gateway.loki.svc",
			Access: datasourceProxyAccessMode,
		}
	}

	// Datasource for Mimir
	DatasourceMimir = func() Datasource {
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

	// Datasource for Mimir to query cardinality data
	DatasourceMimirCardinality = func() Datasource {
		return Datasource{
			Type:   "marcusolsson-json-datasource",
			Name:   "Mimir Cardinality",
			UID:    fmt.Sprintf("%smimir-cardinality", datasourceUIDPrefix),
			URL:    "http://mimir-gateway.mimir.svc:8080/prometheus/api/v1/cardinality/",
			Access: datasourceProxyAccessMode,
		}
	}

	// Datasource for Tempo distributed tracing backend
	DatasourceTempo = func() Datasource {
		return Datasource{
			Type:   "tempo",
			URL:    "http://tempo-gateway.tempo.svc",
			Access: datasourceProxyAccessMode,
			JSONData: map[string]any{
				// Service Map configuration - generates visual service dependency maps
				// from trace data using metrics from the connected Prometheus datasource
				// Non-default: Links to our Mimir instance for service map generation
				"serviceMap": map[string]any{
					"datasourceUid": "gs-mimir",
				},

				// Traces to Logs correlation - allows jumping from trace spans
				// to related log entries in Loki for better debugging context
				// Non-default: Configured for our specific Loki instance and tag mapping
				"tracesToLogs": map[string]any{
					"datasourceUid": "gs-loki",

					// Map common Kubernetes/service mesh tags to log labels
					"tags": []string{"service_name", "pod"},
					"mappedTags": []map[string]string{
						{
							"key":   "service_name",
							"value": "service", // Maps to 'service' label in logs
						},
					},
				},
			},
		}
	}
)
