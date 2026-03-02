package grafana

import "fmt"

const (
	// datasourceUIDPrefix is the prefix for all datasources managed by the operator
	datasourceUIDPrefix       = "gs-"
	datasourceProxyAccessMode = "proxy"

	// traceIDRegex is a regular expression pattern that matches trace IDs in raw log lines.
	// It handles all common field name conventions across languages and frameworks:
	//   - snake_case:  trace_id  (Python, Ruby, OpenTelemetry default)
	//   - camelCase:   traceId   (Java, JavaScript)
	//   - PascalCase:  TraceId   (C#)
	//   - Go acronym:  TraceID   (Go)
	// Supports both colon (:) and equals (=) separators, with an optional quote
	// before the separator to handle JSON key formatting ("trace_id":value).
	// Used by Loki derivedFields (Loki → Tempo direction) and as the basis for
	// traceToLogsQuery (Tempo → Loki direction).
	traceIDRegex = "[tT]race_?[Ii][dD]\"?[:=](\\w+)"

	// traceToLogsQuery is the LogQL query used for Tempo → Loki trace correlation.
	// It uses | regexp with the same pattern as traceIDRegex (adapted with a named capture group)
	// to extract the trace ID from raw log lines regardless of format (JSON, logfmt, plain text)
	// and field name casing (trace_id, traceId, TraceId, TraceID, etc.).
	//
	// Using | regexp instead of | json + label filters is intentional:
	//   - | json only works for JSON-formatted logs and only after JSON is parsed
	//   - | regexp operates directly on raw log lines, making it format-agnostic
	//   - It handles all field name variants covered by traceIDRegex in one pass
	//   - Lines that do not contain a matching trace ID field are excluded naturally
	//     (regexp extraction yields an empty label, which never equals the span trace ID)
	traceToLogsQuery = `{${__tags}} | regexp ` + "`" + `[tT]race_?[Ii][dD]"?[:=](?P<extracted_trace_id>\w+)` + "`" + ` | extracted_trace_id="${__span.traceId}"`
)

var (
	LokiDatasourceUID              = fmt.Sprintf("%sloki", datasourceUIDPrefix)
	MimirDatasourceUID             = fmt.Sprintf("%smimir", datasourceUIDPrefix)
	MimirAlertmanagerDatasourceUID = fmt.Sprintf("%smimir-alertmanager", datasourceUIDPrefix)
	TempoDatasourceUID             = fmt.Sprintf("%stempo", datasourceUIDPrefix)
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
			Type: "tempo",
			// We connect to the Tempo Query Frontend service to support streaming
			URL:    "http://tempo-query-frontend.tempo.svc:3200",
			Access: datasourceProxyAccessMode,
			JSONData: map[string]any{
				// Service Map configuration - generates visual service dependency maps
				// from trace data using metrics from the connected Prometheus datasource
				// Links to our specific Mimir instance for service map generation
				"serviceMap": map[string]any{
					"datasourceUid": MimirDatasourceUID,
				},

				// Node Graph configuration - enables node graph visualization
				// for trace service dependencies
				"nodeGraph": map[string]any{
					"enabled": true,
				},

				// Streaming configuration for better performance with metrics
				"streamingEnabled": map[string]any{
					"metrics": true,
					"search":  true,
				},

				// Traces to Logs correlation V2 - allows jumping from trace spans
				// to related log entries in Loki for better debugging context.
				// Links to our specific Loki instance.
				// customQuery uses | regexp to match trace IDs across all log formats
				// and field name conventions. See traceToLogsQuery for details.
				"tracesToLogsV2": map[string]any{
					"datasourceUid":      LokiDatasourceUID,
					"spanStartTimeShift": "-10m",
					"spanEndTimeShift":   "10m",
					"filterByTraceID":    false,
					"customQuery":        traceToLogsQuery,
				},

				// Traces to Metrics correlation - allows jumping from trace spans
				// to related metrics in Prometheus/Mimir for performance analysis
				// Links to our specific Mimir instance
				"tracesToMetrics": map[string]any{
					"datasourceUid": MimirDatasourceUID,
				},
			},
		}
	}
)
