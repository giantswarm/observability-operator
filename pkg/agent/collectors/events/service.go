package events

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/credential"
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
}

func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version, caBundle string, creds credential.BackendCredentials) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is configured")

	loggingEnabled := s.Config.Logging.IsLoggingEnabled(cluster)
	tracingEnabled := s.Config.Tracing.IsTracingEnabled(cluster)
	monitoringEnabled := s.Config.Monitoring.IsMonitoringEnabled(cluster)

	configMapData, err := s.GenerateAlloyEventsConfigMapData(ctx, cluster, loggingEnabled, tracingEnabled, monitoringEnabled)
	if err != nil {
		return fmt.Errorf("failed to generate alloy events configmap: %w", err)
	}

	secretData, err := s.GenerateAlloyEventsSecretData(cluster, loggingEnabled, tracingEnabled, monitoringEnabled, caBundle, creds)
	if err != nil {
		return fmt.Errorf("failed to generate alloy events secret: %w", err)
	}

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
