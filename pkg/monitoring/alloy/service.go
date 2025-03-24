package alloy

import (
	"context"
	_ "embed"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

const (
	ConfigMapName = "monitoring-config"
	SecretName    = "monitoring-secret"
)

type Service struct {
	client.Client
	organization.OrganizationRepository
	PasswordManager password.Manager
	common.ManagementCluster
	MonitoringConfig monitoring.Config
}

func (a *Service) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-service - ensuring alloy is configured")

	// Get list of tenants
	var tenants []string
	tenants, err := listTenants(ctx, a.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	configmap := ConfigMap(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, a.Client, configmap, func() error {
		data, err := a.GenerateAlloyMonitoringConfigMapData(ctx, configmap, cluster, tenants, observabilityBundleVersion)
		if err != nil {
			logger.Error(err, "alloy-service - failed to generate alloy monitoring configmap")
			return errors.WithStack(err)
		}
		configmap.Data = data

		return nil
	})
	if err != nil {
		logger.Error(err, "alloy-service - failed to create or update alloy monitoring configmap")
		return errors.WithStack(err)
	}

	secret := Secret(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, a.Client, secret, func() error {
		data, err := a.GenerateAlloyMonitoringSecretData(ctx, cluster)
		if err != nil {
			logger.Error(err, "alloy-service - failed to generate alloy monitoring secret")
			return errors.WithStack(err)
		}
		secret.Data = data

		return nil
	})
	if err != nil {
		logger.Error(err, "alloy-service - failed to create or update alloy monitoring secret")
		return errors.WithStack(err)
	}

	logger.Info("alloy-service - ensured alloy is configured")

	return nil
}

func (a *Service) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-service - ensuring alloy is removed")

	configmap := ConfigMap(cluster)
	err := a.Client.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.WithStack(err)
	}

	secret := Secret(cluster)
	err = a.Client.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.WithStack(err)
	}

	logger.Info("alloy-service - ensured alloy is removed")
	return nil
}

func listTenants(ctx context.Context, k8sClient client.Client) ([]string, error) {
	tenants := make([]string, 0)
	var grafanaOrganizations v1alpha1.GrafanaOrganizationList

	err := k8sClient.List(ctx, &grafanaOrganizations)
	if err != nil {
		return nil, err
	}

	for _, organization := range grafanaOrganizations.Items {
		if !organization.DeletionTimestamp.IsZero() {
			continue
		}

		for _, tenant := range organization.Spec.Tenants {
			if !slices.Contains(tenants, string(tenant)) {
				tenants = append(tenants, string(tenant))
			}
		}
	}

	return tenants, nil
}
