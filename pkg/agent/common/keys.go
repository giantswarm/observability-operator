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
	LokiURLKey         = "logging-url"       // URL for Loki push endpoint
	LokiTenantIDKey    = "logging-tenant-id" // Tenant ID for multi-tenancy
	LokiUsernameKey    = "logging-username"  // Username for basic auth
	LokiPasswordKey    = "logging-password"  // Password for basic auth
	LokiRulerAPIURLKey = "ruler-api-url"     // URL for Loki ruler API

	// Loki performance tuning parameters
	LokiMaxBackoffPeriod = "10m" // Maximum backoff period for retries
	LokiRemoteTimeout    = "60s" // Timeout for remote write operations

	// --- Tempo Configuration (Tracing) ---
	// Used by events collector for distributed tracing

	// Tempo secret keys for authentication
	TempoUsernameKey = "tracing-username" // Username for Tempo authentication
	TempoPasswordKey = "tracing-password" // Password for Tempo authentication

	// TempoIngressURLFormat is the URL template for Tempo gateway ingress
	TempoIngressURLFormat = "tempo-gateway.%s"

	// --- Mimir Configuration (Metrics) ---
	// Used by metrics collector for metrics storage

	// Mimir secret keys for remote write configuration and authentication
	MimirRulerAPIURLKey            = "mimirRulerAPIURL"            // URL for Mimir ruler API
	MimirRemoteWriteAPIUsernameKey = "mimirRemoteWriteAPIUsername" // Username for remote write auth
	MimirRemoteWriteAPIPasswordKey = "mimirRemoteWriteAPIPassword" // Password for remote write auth
	MimirRemoteWriteAPIURLKey      = "mimirRemoteWriteAPIURL"      // URL for remote write endpoint
	MimirRemoteWriteAPINameKey     = "mimirRemoteWriteAPIName"     // Name identifier for remote write

	// Mimir default values and URL templates
	MimirRemoteWriteName              = "mimir"                             // Default remote write name
	MimirBaseURLFormat                = "https://mimir.%s"                  // Base URL template for Mimir
	MimirRemoteWriteEndpointURLFormat = MimirBaseURLFormat + "/api/v1/push" // Full remote write endpoint URL
	MimirQueryEndpointURLFormat       = MimirBaseURLFormat + "/prometheus"  // Prometheus-compatible query endpoint for KEDA
	MimirRemoteWriteTimeout           = "60s"                               // Timeout for remote write operations

	// MimirQueryAPIURLKey is the secret key for the Mimir query URL (used by KEDA prometheus scaler)
	MimirQueryAPIURLKey = "mimirQueryAPIURL"
)
