package logs

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
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
	Client                 client.Client
	OrganizationRepository organization.OrganizationRepository
	Config                 config.Config
	LogsAuthManager        auth.AuthManager
}

func (s *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-logs-service - ensuring alloy logs is configured")

	// Check if network monitoring should be enabled
	networkMonitoringEnabled := observabilityBundleVersion.GE(networkMonitoringMinVersion) && s.Config.Logging.EnableNetworkMonitoring

	configmap := ConfigMap(cluster)
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, configmap, func() error {
		data, err := s.GenerateAlloyLogsConfigMapData(ctx, cluster, observabilityBundleVersion, networkMonitoringEnabled)
		if err != nil {
			return fmt.Errorf("failed to generate alloy logs configmap: %w", err)
		}
		configmap.Data = data
		configmap.Labels = labels.Common

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update alloy logs configmap: %w", err)
	}

	secret := Secret(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, s.Client, secret, func() error {
		data, err := s.GenerateAlloyLogsSecretData(ctx, cluster)
		if err != nil {
			return fmt.Errorf("failed to generate alloy logs secret: %w", err)
		}
		secret.Data = data
		secret.Labels = labels.Common

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update alloy logs secret: %w", err)
	}

	logger.Info("alloy-logs-service - ensured alloy logs is configured")

	return nil
}

func (s *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-logs-service - ensuring alloy logs is removed")

	configmap := ConfigMap(cluster)
	err := s.Client.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete alloy logs configmap: %w", err)
	}

	secret := Secret(cluster)
	err = s.Client.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete alloy logs secret: %w", err)
	}

	logger.Info("alloy-logs-service - ensured alloy logs is removed")

	return nil
}
