package alloy

import (
	"context"
	_ "embed"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
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
	AuthManager             auth.AuthManager
}

func (a *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-service - ensuring alloy is configured")

	// Get list of tenants
	tenants, err := a.TenantRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	// Generate ConfigMap data
	configMapData, err := a.GenerateAlloyMonitoringConfigMapData(ctx, nil, cluster, tenants, observabilityBundleVersion)
	if err != nil {
		return fmt.Errorf("failed to generate alloy monitoring configmap: %w", err)
	}

	// Generate Secret data
	secretData, err := a.GenerateAlloyMonitoringSecretData(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to generate alloy monitoring secret: %w", err)
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
		return fmt.Errorf("failed to save alloy monitoring configuration: %w", err)
	}

	logger.Info("alloy-service - ensured alloy is configured")

	return nil
}

func (a *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-service - ensuring alloy is removed")

	err := a.ConfigurationRepository.Delete(
		ctx,
		cluster.Name,
		cluster.Namespace,
		fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
		fmt.Sprintf("%s-%s", cluster.Name, SecretName),
	)
	if err != nil {
		return fmt.Errorf("failed to delete alloy monitoring configuration: %w", err)
	}

	logger.Info("alloy-service - ensured alloy is removed")

	return nil
}
