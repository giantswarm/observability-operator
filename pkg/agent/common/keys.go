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
	// Batch sizes set to 1024 to balance throughput with the gRPC server's default 4 MB decompressed
	// message limit (4,194,304 bytes). At observed average payload size of 1.6 KB/item:
	// 1024 items × 1.6 KB = 1.6 MB — 2.5× safety margin from 4 MB limit.
	// Maximum payload risk at 8 KB/item: 1024 × 8 KB = 8 MB would exceed limit, but mitigated by
	// timeout: items rarely reach 8 KB in practice, and timeout forces flush before saturation.
	// Increased timeout to 500ms to give exporters (Mimir, Loki, Tempo) adequate time to process
	// batches, reducing "sending queue is full" backpressure when export destinations are slow.
	// send_batch_max_size must be ≥ send_batch_size (otelcol validates this at startup).
	OTLPBatchSendBatchSize = 1024    // Flush when this many items queued (must be ≤ OTLPBatchMaxSize)
	OTLPBatchMaxSize       = 1024    // Hard cap: prevents batches from exceeding 4 MB gRPC limit with safety margin
	OTLPBatchTimeout       = "500ms" // Maximum wait before flushing an incomplete batch

	// --- Mimir Configuration (Metrics) ---
	// Used by metrics collector for metrics storage

	// Mimir default values and URL templates
	MimirRemoteWriteName              = "mimir"                             // Default remote write name
	MimirBaseURLFormat                = "https://mimir.%s"                  // Base URL template for Mimir
	MimirRemoteWriteEndpointURLFormat = MimirBaseURLFormat + "/api/v1/push" // Full remote write endpoint URL
	MimirQueryEndpointURLFormat       = MimirBaseURLFormat + "/prometheus"  // Prometheus-compatible query endpoint for KEDA
	MimirRemoteWriteTimeout           = "60s"                               // Timeout for remote write operations

	// Mimir secret keys for remote write configuration and authentication.
	MimirQueryAPIURLKey            = "metrics-query-url"         // URL for Mimir query endpoint
	MimirRulerAPIURLKey            = "metrics-ruler-url"         // URL for Mimir ruler API
	MimirRemoteWriteAPIUsernameKey = "metrics-username"          // Username for remote write / OTLP auth
	MimirRemoteWriteAPIPasswordKey = "metrics-password"          // Password for remote write / OTLP auth
	MimirRemoteWriteAPIURLKey      = "metrics-remote-write-url"  // URL for remote write endpoint
	MimirRemoteWriteAPINameKey     = "metrics-remote-write-name" // Name identifier for remote write

	MimirOTLPBaseURLFormat = MimirBaseURLFormat + "/otlp" // Base URL for Mimir OTLP — exporter appends /v1/metrics
	MimirOTLPURLKey        = "metrics-otlp-url"           // Secret key: base URL for Mimir OTLP write (WC only)
)
