package logs

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	ConfigMapName = "logging-config"
	SecretName    = "logging-secret"
)

// Version constraints that control feature availability based on observability bundle version
const (
	// NetworkMonitoringMinVersion is the minimum observability bundle version required for network monitoring (Beyla)
	NetworkMonitoringMinVersion = "2.3.0"
	// AlloyNodeFilterFixedObservabilityBundleAppVersion is the bundle version with the node filtering fix
	AlloyNodeFilterFixedObservabilityBundleAppVersion = "2.4.0"
	// AlloyNodeFilterImageVersion is the Alloy version to use when node filtering is enabled but bundle version is below the fix
	AlloyNodeFilterImageVersion = "1.12.0"
)

var (
	// networkMonitoringMinVersion is the minimum observability bundle version required for network monitoring
	networkMonitoringMinVersion = semver.MustParse(NetworkMonitoringMinVersion)
)

type Service struct {
	Config                  config.Config
	ConfigurationRepository agent.ConfigurationRepository
	OrganizationRepository  organization.OrganizationRepository
	TenantRepository        tenancy.TenantRepository
	LogsAuthManager         auth.AuthManager
}

func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	// No-op if Alloy logs reconciliation is disabled at installation level
	if !s.Config.Logging.EnableAlloyLogsReconciliation {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("alloy-logs-service - ensuring alloy logs is configured")

	// Check feature enablement
	loggingEnabled := s.Config.Logging.IsLoggingEnabled(cluster)
	// Network monitoring requires observability-bundle >= 2.3.0 and must be explicitly enabled
	networkMonitoringEnabled := observabilityBundleVersion.GE(networkMonitoringMinVersion) && s.Config.Monitoring.IsNetworkMonitoringEnabled(cluster)

	// Generate ConfigMap data
	configMapData, err := s.GenerateAlloyLogsConfigMapData(ctx, cluster, observabilityBundleVersion, loggingEnabled, networkMonitoringEnabled)
	if err != nil {
		return fmt.Errorf("failed to generate alloy logs configmap: %w", err)
	}

	// Generate Secret data
	secretData, err := s.GenerateAlloyLogsSecretData(ctx, cluster, loggingEnabled)
	if err != nil {
		return fmt.Errorf("failed to generate alloy logs secret: %w", err)
	}

	// Save configuration via repository
	err = s.ConfigurationRepository.Save(ctx, &agent.AgentConfiguration{
		ClusterName:      cluster.Name,
		ClusterNamespace: cluster.Namespace,
		ConfigMapName:    fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
		SecretName:       fmt.Sprintf("%s-%s", cluster.Name, SecretName),
		ConfigMapData:    configMapData,
		SecretData:       secretData,
		Labels:           labels.Common,
	})
	if err != nil {
		return fmt.Errorf("failed to save alloy logs configuration: %w", err)
	}

	logger.Info("alloy-logs-service - ensured alloy logs is configured")

	return nil
}

func (s *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	// No-op if Alloy logs reconciliation is disabled at installation level
	if !s.Config.Logging.EnableAlloyLogsReconciliation {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("alloy-logs-service - ensuring alloy logs is removed")

	err := s.ConfigurationRepository.Delete(
		ctx,
		cluster.Name,
		cluster.Namespace,
		fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
		fmt.Sprintf("%s-%s", cluster.Name, SecretName),
	)
	if err != nil {
		return fmt.Errorf("failed to delete alloy logs configuration: %w", err)
	}
	return nil
}
