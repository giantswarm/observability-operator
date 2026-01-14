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

// minimumTracingSupportVersion is the minimum observability bundle version that supports tracing
var minimumTracingSupportVersion = semver.MustParse("1.11.0")

type Service struct {
	Config                  config.Config
	ConfigurationRepository agent.ConfigurationRepository
	OrganizationRepository  organization.OrganizationRepository
	TenantRepository        tenancy.TenantRepository
	LogsAuthManager         auth.AuthManager
	TracesAuthManager       auth.AuthManager
}

func (a *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	// No-op if events collection is disabled at installation level
	// TODO remove once the logging operator is gone
	if !a.Config.Logging.EnableAlloyEventsReconciliation {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is configured")

	// Determine if logging and tracing are enabled for this cluster
	loggingEnabled := a.Config.Logging.IsLoggingEnabled(cluster)
	tracingEnabled := a.Config.Tracing.IsTracingEnabled(cluster) && observabilityBundleVersion.GE(minimumTracingSupportVersion)

	// Generate ConfigMap data
	configMapData, err := a.GenerateAlloyEventsConfigMapData(ctx, cluster, loggingEnabled, tracingEnabled, observabilityBundleVersion)
	if err != nil {
		return fmt.Errorf("failed to generate alloy events configmap: %w", err)
	}

	// Generate Secret data
	secretData, err := a.GenerateAlloyEventsSecretData(ctx, cluster, loggingEnabled, tracingEnabled)
	if err != nil {
		return fmt.Errorf("failed to generate alloy events secret: %w", err)
	}

	// Save configuration via repository
	err = a.ConfigurationRepository.Save(ctx, &agent.AgentConfiguration{
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

func (a *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	// No-op if events collection is disabled at installation level
	// TODO remove once the logging operator is gone
	if !a.Config.Logging.EnableAlloyEventsReconciliation {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is removed")

	err := a.ConfigurationRepository.Delete(
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
