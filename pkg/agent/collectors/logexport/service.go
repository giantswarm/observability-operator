// Package logexport configures alloy-logexporter, the installation-level Alloy
// instance that archives audit and Teleport logs to a customer-owned object store.
//
// It differs from the metrics, logs and events collectors in three ways:
//
//   - The app is not part of observability-bundle. management-cluster-bases
//     collections ships it to every management cluster in an inert state, and this
//     package's ConfigMap is what switches it on.
//   - It is installation-level, so it reconciles only for the management cluster.
//     The Envoy mirror that feeds it is configured once at the gateway, so a
//     workload cluster instance would receive nothing.
//   - Its ConfigMap has a fixed name in a fixed namespace rather than being
//     prefixed with the cluster name, because the HelmRelease shipped by
//     collections references it by name and a HelmRelease can only resolve
//     valuesFrom references in its own namespace.
package logexport

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/credential"
)

const (
	// ConfigMapName and SecretName are fixed, not prefixed with the cluster name:
	// the alloy-logexporter HelmRelease in management-cluster-bases references
	// these names literally.
	ConfigMapName = "alloy-logexporter-config"
	//nolint:gosec // G101: a Kubernetes object name, not a credential
	SecretName = "alloy-logexporter-secret"

	// Replicas of the exporter. Each mirrored push is load-balanced to one
	// replica; replicas do not fan out, so this is not a duplication source.
	Replicas = 2

	// WALSize bounds the otelcol.storage.file write-ahead log. Buffering a
	// four-minute destination outage on a testing installation used ~136MB.
	WALSize = "10Gi"

	// Timeout is the exporter's per-request timeout and, because the awss3
	// exporter's retry_on_failure is not exposed by Alloy and is disabled, it is
	// also the durability window. An outage longer than this loses data.
	Timeout = "5m"

	// QueueSize is measured in items, not requests, so it is a record count.
	QueueSize = 200000

	// Batching bounds. Without batching every log line becomes its own object,
	// because otelcol.receiver.loki calls ConsumeLogs once per entry.
	BatchMinSize      = 1000
	BatchMaxSize      = 10000
	BatchFlushTimeout = "60s"
)

type Service struct {
	Config                  config.Config
	ConfigurationRepository agent.ConfigurationRepository
}

// ReconcileCreate writes the values ConfigMap and credentials Secret that enable
// alloy-logexporter.
//
// observabilityBundleVersion, caBundle and creds are part of the shared
// CollectorService interface and unused here: this app is not a bundle app, it does
// not talk to Loki or Mimir, and its credentials are the object store's rather than
// an observability backend's.
func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, _ semver.Version, _ string, _ credential.BackendCredentials) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-logexporter-service - ensuring alloy logexporter is configured")

	configMapData, err := s.GenerateAlloyLogExportConfigMapData()
	if err != nil {
		return fmt.Errorf("failed to generate alloy logexporter configmap: %w", err)
	}

	secretData, err := s.GenerateAlloyLogExportSecretData()
	if err != nil {
		return fmt.Errorf("failed to generate alloy logexporter secret: %w", err)
	}

	err = s.ConfigurationRepository.Save(ctx, &agent.AgentConfiguration{
		ClusterName: cluster.Name,
		// The exporter's HelmRelease lives here, not in the cluster's namespace.
		ClusterNamespace: s.Config.LogExport.Namespace,
		ConfigMapName:    ConfigMapName,
		SecretName:       SecretName,
		ConfigMapData:    configMapData,
		SecretData:       secretData,
		Labels:           labels.Common,
	})
	if err != nil {
		return fmt.Errorf("failed to save alloy logexporter configuration: %w", err)
	}

	logger.Info("alloy-logexporter-service - ensured alloy logexporter is configured")

	return nil
}

// ReconcileDelete removes the ConfigMap and Secret, which returns the app to the
// inert state that collections ships.
func (s *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-logexporter-service - ensuring alloy logexporter is removed")

	err := s.ConfigurationRepository.Delete(
		ctx,
		cluster.Name,
		s.Config.LogExport.Namespace,
		ConfigMapName,
		SecretName,
	)
	if err != nil {
		return fmt.Errorf("failed to delete alloy logexporter configuration: %w", err)
	}

	logger.Info("alloy-logexporter-service - ensured alloy logexporter is removed")

	return nil
}
