package common

// This file defines constants used across different Alloy agent collectors (metrics, logs, events).
// It includes secret keys, URL formats, and configuration values for observability backends
// such as Loki (logging), Tempo (tracing), and Mimir (metrics).

const (
	// PriorityClassName is the pod priority class for critical Alloy agent workloads
	PriorityClassName = "giantswarm-critical"

	// --- Loki Configuration (Logging) ---
	// Used by logs and events collectors for log aggregation

	// LokiBaseURLFormat is the base URL template for Loki write endpoints
	LokiBaseURLFormat = "https://write.loki.%s"
	// LokiPushURLFormat is the full URL template for Loki's push API
	LokiPushURLFormat = LokiBaseURLFormat + "/loki/api/v1/push"

	// Loki secret keys for authentication and endpoint configuration
	LokiURLKey         = "logging-url"           // URL for Loki push endpoint
	LokiTenantIDKey    = "logging-tenant-id"     // Tenant ID for multi-tenancy
	LokiUsernameKey    = "logging-username"      // Username for basic auth
	LokiPasswordKey    = "logging-password"      // Password for basic auth
	LokiRulerAPIURLKey = "logging-ruler-api-url" // URL for Loki ruler API
	// Loki OTLP configuration
	LokiOTLPBaseURLFormat = LokiBaseURLFormat + "/otlp" // Base URL for Loki OTLP — exporter appends /v1/logs
	LokiOTLPURLKey        = "logging-otlp-url"          // Secret key: base URL for Loki OTLP write (WC only)

	// Loki performance tuning parameters
	LokiMaxBackoffPeriod = "10m" // Maximum backoff period for retries
	LokiRemoteTimeout    = "60s" // Timeout for remote write operations

	// --- Tempo Configuration (Tracing) ---
	// Used by events collector for distributed tracing

	// Tempo secret keys for authentication and endpoint configuration
	TempoUsernameKey = "tracing-username" // Username for Tempo authentication
	TempoPasswordKey = "tracing-password" // Password for Tempo authentication
	TempoOTLPURLKey  = "tracing-otlp-url" // gRPC endpoint in host:port format

	// TempoBaseURLFormat is the URL template for Tempo ingress
	TempoBaseURLFormat = "tempo.%s"

	// --- Mimir Configuration (Metrics) ---
	// Used by metrics collector for metrics storage

	// Mimir default values and URL templates
	MimirRemoteWriteName              = "mimir"                             // Default remote write name
	MimirBaseURLFormat                = "https://mimir.%s"                  // Base URL template for Mimir
	MimirRemoteWriteEndpointURLFormat = MimirBaseURLFormat + "/api/v1/push" // Full remote write endpoint URL
	MimirQueryEndpointURLFormat       = MimirBaseURLFormat + "/prometheus"  // Prometheus-compatible query endpoint for KEDA
	MimirRemoteWriteTimeout           = "60s"                               // Timeout for remote write operations

	// Mimir secret keys for remote write configuration and authentication.
	MimirQueryAPIURLKey        = "metrics-query-url"         // URL for Mimir query endpoint
	MimirRulerAPIURLKey        = "metrics-ruler-url"         // URL for Mimir ruler API
	MimirUsernameKey           = "metrics-username"          // Username for Mimir remote write and OTLP auth
	MimirPasswordKey           = "metrics-password"          // Password for Mimir remote write and OTLP auth
	MimirRemoteWriteAPIURLKey  = "metrics-remote-write-url"  // URL for remote write endpoint
	MimirRemoteWriteAPINameKey = "metrics-remote-write-name" // Name identifier for remote write

	MimirOTLPBaseURLFormat = MimirBaseURLFormat + "/otlp" // Base URL for Mimir OTLP — exporter appends /v1/metrics
	MimirOTLPURLKey        = "metrics-otlp-url"           // Secret key: base URL for Mimir OTLP write (WC only)
)
