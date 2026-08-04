package apps

// Alloy agent application names and their keys in the observability-bundle

const (
	// Alloy namespace - all alloy apps are deployed in kube-system
	AlloyNamespace = "kube-system"

	// Monitoring
	AlloyMetricsAppName      = "alloy-metrics"
	AlloyMetricsHelmValueKey = "alloyMetrics"

	// Logging
	AlloyLogsAppName      = "alloy-logs"
	AlloyLogsHelmValueKey = "alloyLogs"

	// Events (used for both logging and tracing)
	AlloyEventsAppName      = "alloy-events"
	AlloyEventsHelmValueKey = "alloyEvents"

	// Log export. Unlike the three above, this one is not an observability-bundle
	// app: it is shipped to management clusters by management-cluster-bases
	// collections and is inert until this operator writes its values ConfigMap, so
	// it has no bundle Helm value key.
	AlloyLogExporterAppName = "alloy-logexporter"
)
