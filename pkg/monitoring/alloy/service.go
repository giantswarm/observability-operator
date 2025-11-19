package alloy

import (
	"context"
	_ "embed"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	ConfigMapName = "monitoring-config"
	SecretName    = "monitoring-secret"
)

type Service struct {
	client.Client
	organization.OrganizationRepository
	config.Config
	auth.AuthManager
}

func (a *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-service - ensuring alloy is configured")

	// Get list of tenants
	var tenants []string
	tenants, err := tenancy.ListTenants(ctx, a.Client)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	configmap := ConfigMap(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, a.Client, configmap, func() error {
		data, err := a.GenerateAlloyMonitoringConfigMapData(ctx, configmap, cluster, tenants, observabilityBundleVersion)
		if err != nil {
			return fmt.Errorf("failed to generate alloy monitoring configmap: %w", err)
		}
		configmap.Data = data

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update alloy monitoring configmap: %w", err)
	}

	secret := Secret(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, a.Client, secret, func() error {
		data, err := a.GenerateAlloyMonitoringSecretData(ctx, cluster)
		if err != nil {
			return fmt.Errorf("failed to generate alloy monitoring secret: %w", err)
		}
		secret.Data = data

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update alloy monitoring secret: %w", err)
	}

	logger.Info("alloy-service - ensured alloy is configured")

	return nil
}

func (a *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-service - ensuring alloy is removed")

	configmap := ConfigMap(cluster)
	err := a.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete alloy monitoring configmap: %w", err)
	}

	secret := Secret(cluster)
	err = a.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete alloy monitoring secret: %w", err)
	}

	logger.Info("alloy-service - ensured alloy is removed")

	return nil
}
