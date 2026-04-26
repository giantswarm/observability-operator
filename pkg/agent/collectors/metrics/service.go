package metrics

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/credential"
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
	CredentialReader        credential.Reader
}

func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version, caBundle string) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-metrics-service - ensuring alloy metrics is configured")

	// Get list of tenants
	tenants, err := s.TenantRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	// Generate ConfigMap data
	configMapData, err := s.GenerateAlloyMonitoringConfigMapData(ctx, nil, cluster, tenants, observabilityBundleVersion)
	if err != nil {
		return fmt.Errorf("failed to generate alloy monitoring configmap: %w", err)
	}

	// Generate Secret data
	secretData, err := s.GenerateAlloyMonitoringSecretData(ctx, cluster, caBundle)
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
