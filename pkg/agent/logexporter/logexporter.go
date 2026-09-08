package logexporter

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Tuning that is neither on the CRD nor in the operator's configuration. Every value
// here is set to a specific measured failure, and getting one wrong loses data silently
// rather than merely performing differently, so change them only with the same kind of
// evidence.
const (
	// Port is the push endpoint mirrored requests arrive on. It must match the chart's
	// extraPorts entry and the mirror backendRef.
	Port = 3100

	// WALDirectory holds the persistent sending queue. It sits under WALMountPath, which
	// is a per-replica PVC: the WAL is what carries buffered records across a pod restart.
	WALMountPath = "/var/lib/alloy"
	WALDirectory = WALMountPath + "/wal"

	// RunAsUser is the chart's Alloy user, needed as fsGroup so a fresh PVC is writable.
	RunAsUser = 473

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

	// Retries within the export timeout. Once it elapses the exporter gives up, so
	// raising these past the configured timeout buys nothing.
	RetryMaxAttempts = 10
	RetryMaxBackoff  = "5m"
)

// DefaultResources is the exporter container's sizing when the operator is configured
// with none. ephemeral-storage is required by the require-emptydir-requests-and-limits
// Kyverno policy, because the container mounts the alloy-tmp emptyDir.
var DefaultResources = corev1.ResourceRequirements{
	Requests: corev1.ResourceList{
		corev1.ResourceCPU:              resource.MustParse("100m"),
		corev1.ResourceMemory:           resource.MustParse("400Mi"),
		corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
	},
	Limits: corev1.ResourceList{
		corev1.ResourceCPU:              resource.MustParse("1"),
		corev1.ResourceMemory:           resource.MustParse("1Gi"),
		corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
	},
}
