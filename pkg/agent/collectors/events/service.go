package events

import (
	"context"
	_ "embed"
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
	ConfigMapName = "events-logger-config"
	SecretName    = "events-logger-secret"
)

type Service struct {
	Config                  config.Config
	ConfigurationRepository agent.ConfigurationRepository
	OrganizationRepository  organization.OrganizationRepository
	TenantRepository        tenancy.TenantRepository
	LogsAuthManager         auth.AuthManager
	TracesAuthManager       auth.AuthManager
	MetricsAuthManager      auth.AuthManager
}

func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is configured")

	// Determine if logging, tracing, and OTLP signals are enabled for this cluster
	loggingEnabled := s.Config.Logging.IsLoggingEnabled(cluster)
	tracingEnabled := s.Config.Tracing.IsTracingEnabled(cluster)
	metricsEnabled := s.Config.Monitoring.IsMonitoringEnabled(cluster) && s.Config.Monitoring.OTLPEnabled

	// Generate ConfigMap data
	configMapData, err := s.GenerateAlloyEventsConfigMapData(ctx, cluster, loggingEnabled, tracingEnabled, metricsEnabled)
	if err != nil {
		return fmt.Errorf("failed to generate alloy events configmap: %w", err)
	}

	// Generate Secret data
	secretData, err := s.GenerateAlloyEventsSecretData(ctx, cluster, loggingEnabled, tracingEnabled, metricsEnabled)
	if err != nil {
		return fmt.Errorf("failed to generate alloy events secret: %w", err)
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
		return fmt.Errorf("failed to save alloy events configuration: %w", err)
	}

	logger.Info("alloy-events-service - ensured alloy events is configured")

	return nil
}

func (s *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is removed")

	err := s.ConfigurationRepository.Delete(
		ctx,
		cluster.Name,
		cluster.Namespace,
		fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
		fmt.Sprintf("%s-%s", cluster.Name, SecretName),
	)
	if err != nil {
		return fmt.Errorf("failed to delete alloy events configuration: %w", err)
	}
	return nil
}
