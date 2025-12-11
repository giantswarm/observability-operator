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
)
