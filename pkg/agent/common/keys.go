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
	// Loki OTLP configuration
	LokiOTLPBaseURLFormat = LokiBaseURLFormat + "/otlp" // Base URL for Loki OTLP — exporter appends /v1/logs
	LokiOTLPURLKey        = "logging-otlp-url"          // Secret key: base URL for Loki OTLP write (WC only)

	// Loki performance tuning parameters
	LokiMaxBackoffPeriod = "10m" // Maximum backoff period for retries
	LokiRemoteTimeout    = "60s" // Timeout for remote write operations

	// --- Tempo Configuration (Tracing) ---
	// Used by events collector for distributed tracing

	// Tempo secret keys for authentication
	TempoUsernameKey = "tracing-username" // Username for Tempo authentication
	TempoPasswordKey = "tracing-password" // Password for Tempo authentication

	// TempoIngressURLFormat is the URL template for Tempo ingress
	TempoIngressURLFormat = "tempo.%s"

	// --- OTLP Batch Processor Configuration ---
	// Controls the otelcol.processor.batch block shared by all OTLP pipelines (traces, metrics, logs).
	// Tune here if an installation shows export latency or oversized payloads; do not expose via Helm
	// values since these are internal Alloy pipeline knobs, not user-facing behaviour toggles.
	//
	// Both send_batch_size and send_batch_max_size are set to 512 to prevent batches from exceeding
	// the gRPC server's default 4 MB decompressed message limit (4,194,304 bytes).
	// Observed burst: 2652 items = 4 MB → ~1.6 KB/item average.
	// OTLP items (k8s events, spans, metrics) can reach ~8 KB for detailed payloads;
	// at 512 items × 8 KB = 4 MB — the theoretical ceiling.
	// At observed average sizes: 512 × 1.6 KB = 0.8 MB — 5× safety margin.
	// send_batch_max_size must be ≥ send_batch_size (otelcol validates this at startup).
	OTLPBatchSendBatchSize = 512     // Flush when this many items queued (must be ≤ OTLPBatchMaxSize)
	OTLPBatchMaxSize       = 512     // Hard cap: prevents batches that exceed the 4 MB gRPC server limit
	OTLPBatchTimeout       = "200ms" // Maximum wait before flushing an incomplete batch

	// --- Mimir Configuration (Metrics) ---
	// Used by metrics collector for metrics storage

	// Mimir default values and URL templates
	MimirRemoteWriteName              = "mimir"                             // Default remote write name
	MimirBaseURLFormat                = "https://mimir.%s"                  // Base URL template for Mimir
	MimirRemoteWriteEndpointURLFormat = MimirBaseURLFormat + "/api/v1/push" // Full remote write endpoint URL
	MimirQueryEndpointURLFormat       = MimirBaseURLFormat + "/prometheus"  // Prometheus-compatible query endpoint for KEDA
	MimirRemoteWriteTimeout           = "60s"                               // Timeout for remote write operations

	// Mimir secret keys for remote write configuration and authentication.
	// All keys follow the kebab-case signal-purpose convention used by Loki (logging-*)
	// and Tempo (tracing-*) keys.
	MimirQueryAPIURLKey            = "metrics-query-url"         // URL for Mimir query endpoint
	MimirRulerAPIURLKey            = "metrics-ruler-url"         // URL for Mimir ruler API
	MimirRemoteWriteAPIUsernameKey = "metrics-username"          // Username for remote write / OTLP auth
	MimirRemoteWriteAPIPasswordKey = "metrics-password"          // Password for remote write / OTLP auth
	MimirRemoteWriteAPIURLKey      = "metrics-remote-write-url"  // URL for remote write endpoint
	MimirRemoteWriteAPINameKey     = "metrics-remote-write-name" // Name identifier for remote write

	MimirOTLPBaseURLFormat  = MimirBaseURLFormat + "/otlp" // Base URL for Mimir OTLP — exporter appends /v1/metrics
	MimirOTLPWriteAPIURLKey = "metrics-otlp-url"           // Secret key: base URL for Mimir OTLP write (WC only)
)
