package config

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// LogExportConfig is installation-wide configuration for alloy-logexporter: where its
// rendered values go, how much it can buffer, how long an outage it survives, and what it
// costs.
//
// Only the target, capacity and durability are here. The queue, batch and retry settings
// stay constants in pkg/agent/logexporter, because a wrong value there loses data silently
// rather than merely performing differently.
type LogExportConfig struct {
	// Namespace is where the rendered ConfigMap and Secret are written: wherever
	// alloy-logexporter's Helm release reads its values from, which is not necessarily the
	// namespace the app itself runs in.
	Namespace string

	// ConfigMapName and SecretName name those two objects. They have to match what the
	// Helm release is configured to read, or the values never reach the app.
	ConfigMapName string
	SecretName    string

	// Replicas is the StatefulSet size. Pushes are load-balanced to one replica, so
	// replicas add capacity without duplicating records.
	Replicas int

	// WALSize is the per-replica volume holding the sending queue (e.g. "10Gi"). It only
	// has to hold what accumulates while the destination is unavailable, so size it as the
	// rate at which selected logs arrive times ExportTimeout, not as peak throughput.
	WALSize string

	// ExportTimeout is the durability window (e.g. "5m"). The awss3 exporter does not
	// expose retry_on_failure, so this is a hard ceiling: an outage longer than this loses
	// data permanently.
	ExportTimeout string

	// Resources are the exporter container's requests and limits. Nil uses the defaults in
	// pkg/agent/logexporter.
	Resources *corev1.ResourceRequirements
}

// Validate validates the log export configuration.
func (l LogExportConfig) Validate() error {
	if l.Namespace == "" {
		return fmt.Errorf("namespace must be set")
	}
	if l.ConfigMapName == "" {
		return fmt.Errorf("configmap name must be set")
	}
	if l.SecretName == "" {
		return fmt.Errorf("secret name must be set")
	}
	if l.Replicas < 1 {
		return fmt.Errorf("replicas must be at least 1, got %d", l.Replicas)
	}
	if l.WALSize == "" {
		return fmt.Errorf("WAL size must be set")
	}
	if l.ExportTimeout == "" {
		return fmt.Errorf("export timeout must be set")
	}
	return nil
}
