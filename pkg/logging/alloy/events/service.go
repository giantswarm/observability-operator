package events

import (
	"context"
	_ "embed"
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
	Client                 client.Client
	OrganizationRepository organization.OrganizationRepository
	TenantRepository       tenancy.TenantRepository
	Config                 config.Config
	LogsAuthManager        auth.AuthManager
	TracesAuthManager      auth.AuthManager
}

func (a *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	// No-op if events collection is disabled at installation level
	// TODO remove once the logging operator is gone
	if !a.Config.Logging.EnableAlloyEventsReconciliation {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is configured")

	// Determine if tracing is enabled based on config and observability bundle version
	tracingEnabled := a.Config.Tracing.Enabled && observabilityBundleVersion.GE(minimumTracingSupportVersion)

	configmap := ConfigMap(cluster)
	_, err := controllerutil.CreateOrUpdate(ctx, a.Client, configmap, func() error {
		data, err := a.GenerateAlloyEventsConfigMapData(ctx, cluster, tracingEnabled, observabilityBundleVersion)
		if err != nil {
			return fmt.Errorf("failed to generate alloy events configmap: %w", err)
		}
		configmap.Data = data
		configmap.Labels = labels.Common

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update alloy events configmap: %w", err)
	}

	secret := Secret(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, a.Client, secret, func() error {
		data, err := a.GenerateAlloyEventsSecretData(ctx, cluster, tracingEnabled)
		if err != nil {
			return fmt.Errorf("failed to generate alloy events secret: %w", err)
		}
		secret.Data = data
		secret.Labels = labels.Common

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update alloy events secret: %w", err)
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

	configmap := ConfigMap(cluster)
	err := a.Client.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete alloy events configmap: %w", err)
	}

	secret := Secret(cluster)
	err = a.Client.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete alloy events secret: %w", err)
	}

	logger.Info("alloy-events-service - ensured alloy events is removed")

	return nil
}
