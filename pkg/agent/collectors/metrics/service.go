package metrics

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
  "github.com/giantswarm/observability-operator/pkg/credential"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

const (
	ConfigMapName = "monitoring-config"
	SecretName    = "monitoring-secret"
)

type Service struct {
	Config                  config.Config
	ConfigurationRepository agent.ConfigurationRepository
	OrganizationRepository  organization.OrganizationRepository
	TenantRepository        tenancy.TenantRepository
	MetricsQuerier          MetricsQuerier
}

func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version, caBundle string, creds credential.BackendCredentials) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-metrics-service - ensuring alloy metrics is configured")

	// Get list of tenants
	tenants, err := s.TenantRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	// Resolve shard count before rendering so the renderer stays side-effect free.
	// Query failures are non-fatal: sharding falls back to the current head-series
	// of 0, which ComputeShards clamps to the default minimum.
	shards, err := s.resolveShards(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to resolve shards: %w", err)
	}

	// Generate ConfigMap data
	configMapData, err := s.GenerateAlloyMonitoringConfigMapData(ctx, cluster, tenants, observabilityBundleVersion, shards)
	if err != nil {
		return fmt.Errorf("failed to generate alloy monitoring configmap: %w", err)
	}

	// Generate Secret data
	secretData, err := s.GenerateAlloyMonitoringSecretData(cluster, caBundle, creds)
	if err != nil {
		return fmt.Errorf("failed to generate alloy monitoring secret: %w", err)
	}

	// Generate KEDA extra objects if KEDA authentication is enabled
	var extraSecretObjects string
	if s.Config.Monitoring.IsKEDAAuthenticationEnabled(cluster) {
		kedaNamespace := config.GetKEDANamespace(cluster)
		extraSecretObjects, err = generateKEDAExtraObjects(kedaNamespace, secretData)
		if err != nil {
			return fmt.Errorf("failed to generate KEDA extra objects: %w", err)
		}
	}

	// Save configuration via repository
	err = s.ConfigurationRepository.Save(ctx, &agent.AgentConfiguration{
		ClusterName:        cluster.Name,
		ClusterNamespace:   cluster.Namespace,
		ConfigMapName:      fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
		SecretName:         fmt.Sprintf("%s-%s", cluster.Name, SecretName),
		ConfigMapData:      configMapData,
		SecretData:         secretData,
		ExtraSecretObjects: extraSecretObjects,
		Labels:             labels.Common,
	})
	if err != nil {
		return fmt.Errorf("failed to save alloy monitoring configuration: %w", err)
	}

	logger.Info("alloy-metrics-service - ensured alloy metrics is configured")

	return nil
}

// resolveShards computes the Alloy replica count from Mimir head-series, so that
// GenerateAlloyMonitoringConfigMapData can stay a pure transformation. The
// Mimir query is best-effort: transient failures are logged and treated as a
// zero head-series count — the sharding strategy then clamps to the default
// minimum rather than failing the whole reconcile.
func (s *Service) resolveShards(ctx context.Context, cluster *clusterv1.Cluster) (int, error) {
	logger := log.FromContext(ctx)

	var headSeries float64
	if s.MetricsQuerier != nil {
		var err error
		headSeries, err = s.MetricsQuerier.QueryHeadSeries(ctx, cluster)
		if err != nil {
			logger.Error(err, "alloy-metrics-service - failed to query head series")
			metrics.MimirQueryErrors.Inc()
			headSeries = 0
		}
	}

	clusterShardingStrategy, err := monitoring.GetClusterShardingStrategy(cluster)
	if err != nil {
		return 0, fmt.Errorf("failed to get cluster sharding strategy: %w", err)
	}

	shardingStrategy := s.Config.Monitoring.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	return shardingStrategy.ComputeShards(sharding.DefaultShards, headSeries), nil
}

func (s *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-metrics-service - ensuring alloy metrics is removed")

	err := s.ConfigurationRepository.Delete(
		ctx,
		cluster.Name,
		cluster.Namespace,
		fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
		fmt.Sprintf("%s-%s", cluster.Name, SecretName),
	)
	if err != nil {
		return fmt.Errorf("failed to delete alloy monitoring configuration: %w", err)
	}

	logger.Info("alloy-metrics-service - ensured alloy metrics is removed")

	return nil
}
