// Package logexporter renders the Helm values that configure alloy-logexporter, the
// installation-wide app that archives selected logs to destinations outside the
// observability platform.
//
// Rendering is pure: LogExport resources and already-resolved credentials in, values
// documents out. The controller owns reading credentials and persisting the results, the
// same split the collectors use.
package logexporter

// Tuning that is not exposed on the CRD. Every value here is set to a specific measured
// failure, so change them only with the same kind of evidence.
const (
	// Port is the push endpoint mirrored requests arrive on. It must match the chart's
	// extraPorts entry and the mirror backendRef.
	Port = 3100

	// WALDirectory holds the persistent sending queue. It sits under WALMountPath, which
	// is a per-replica PVC: the WAL is what carries buffered records across a pod restart.
	WALMountPath = "/var/lib/alloy"
	WALDirectory = WALMountPath + "/wal"
	// WALSize is sized for rate x ExportTimeout, not for throughput.
	WALSize = "10Gi"

	// RunAsUser is the chart's Alloy user, needed as fsGroup so a fresh PVC is writable.
	RunAsUser = 473

	// Replicas: pushes are load-balanced to one replica, so replicas add capacity without
	// duplicating records.
	Replicas = 2

	// ExportTimeout is the durability window. The exporter does not expose
	// retry_on_failure and it is disabled, so this is a hard ceiling on the AWS SDK's own
	// retries: a destination outage longer than this loses data permanently.
	ExportTimeout = "5m"

	// QueueSize is in items because the queue holds one entry per Loki entry. The default
	// of 1000 requests is under a second of buffer at installation scale and overflows
	// silently.
	QueueSize      = 200000
	QueueConsumers = 4

	// Batching is mandatory: otelcol.receiver.loki calls ConsumeLogs once per entry, so
	// without it every log line becomes its own object. BatchMinSize sets object size and
	// BatchFlushTimeout sets the worst-case latency to the archive.
	BatchMinSize      = 1000
	BatchMaxSize      = 10000
	BatchFlushTimeout = "60s"

	// Retries inside ExportTimeout.
	RetryMaxAttempts = 10
	RetryMaxBackoff  = "5m"

	// ClusterIDField is the field injected into every exported line. A Kubernetes audit
	// event carries no cluster identifier, and the body marshaler drops the Loki labels
	// that would otherwise carry it.
	ClusterIDField = "gs_cluster_id"

	// Credential keys read from an S3 destination's credentialsRef Secret, and the
	// environment variable names the AWS SDK's default chain looks for. They are the same
	// names, which is why they appear once.
	AccessKeyIDKey     = "AWS_ACCESS_KEY_ID"
	SecretAccessKeyKey = "AWS_SECRET_ACCESS_KEY"
)
