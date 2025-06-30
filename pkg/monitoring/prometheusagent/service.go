package prometheusagent

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

type PrometheusAgentService struct {
	client.Client
	organization.OrganizationRepository
	PasswordManager password.Manager
	common.ManagementCluster
	MonitoringConfig monitoring.Config
}

// ReconcileRemoteWriteConfiguration ensures that the prometheus remote write config is present in the cluster.
func (pas *PrometheusAgentService) ReconcileRemoteWriteConfiguration(
	ctx context.Context, cluster *clusterv1.Cluster) error {

	logger := log.FromContext(ctx)
	logger.Info("ensuring prometheus agent remote write configmap and secret")

	err := pas.createOrUpdateConfigMap(ctx, cluster, logger)
	if err != nil {
		return fmt.Errorf("failed to create or update prometheus agent remote write configmap: %w", err)
	}

	err = pas.createOrUpdateSecret(ctx, cluster, logger)
	if err != nil {
		return fmt.Errorf("failed to create or update prometheus agent remote write secret: %w", err)
	}

	logger.Info("ensured prometheus agent remote write configmap and secret")

	return nil
}

func (pas PrometheusAgentService) createOrUpdateConfigMap(ctx context.Context,
	cluster *clusterv1.Cluster, logger logr.Logger) error {

	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
		Namespace: cluster.GetNamespace(),
	}

	current := &corev1.ConfigMap{}
	// Get the current configmap if it exists.
	err := pas.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		configMap, err := pas.buildRemoteWriteConfig(ctx, cluster, logger, sharding.DefaultShards)
		if err != nil {
			return fmt.Errorf("failed to build remote write config: %w", err)
		}

		err = pas.Create(ctx, configMap)
		if err != nil {
			return fmt.Errorf("failed to create remote write configmap: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get remote write configmap: %w", err)
	}

	currentShards, err := readCurrentShardsFromConfig(*current)
	if err != nil {
		return fmt.Errorf("failed to read current shards from config: %w", err)
	}

	desired, err := pas.buildRemoteWriteConfig(ctx, cluster, logger, currentShards)
	if err != nil {
		return fmt.Errorf("failed to build remote write config: %w", err)
	}

	if !reflect.DeepEqual(current.Data, desired.Data) || !reflect.DeepEqual(current.Finalizers, desired.Finalizers) {
		err = pas.Update(ctx, desired)
		if err != nil {
			return fmt.Errorf("failed to update prometheus agent remote write configmap: %w", err)
		}
	}
	return nil
}

func (pas PrometheusAgentService) createOrUpdateSecret(ctx context.Context,
	cluster *clusterv1.Cluster, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      GetPrometheusAgentRemoteWriteSecretName(cluster),
		Namespace: cluster.GetNamespace(),
	}

	current := &corev1.Secret{}
	// Get the current secret if it exists.
	err := pas.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		logger.Info("generating remote write secret for the prometheus agent")
		secret, err := pas.buildRemoteWriteSecret(ctx, cluster)
		if err != nil {
			return fmt.Errorf("failed to generate the remote write secret for the prometheus agent: %w", err)
		}
		logger.Info("generated the remote write secret for the prometheus agent")

		err = pas.Create(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to create remote write secret: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get remote write secret: %w", err)
	}

	desired, err := pas.buildRemoteWriteSecret(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to build remote write secret: %w", err)
	}
	if !reflect.DeepEqual(current.Data, desired.Data) || !reflect.DeepEqual(current.Finalizers, desired.Finalizers) {
		err = pas.Update(ctx, desired)
		if err != nil {
			return fmt.Errorf("failed to update remote write secret: %w", err)
		}
	}

	return nil
}

func (pas *PrometheusAgentService) DeleteRemoteWriteConfiguration(
	ctx context.Context, cluster *clusterv1.Cluster) error {

	logger := log.FromContext(ctx)
	logger.Info("deleting prometheus agent remote write configmap and secret")

	err := pas.deleteConfigMap(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to delete prometheus agent remote write configmap: %w", err)
	}

	err = pas.deleteSecret(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to delete prometheus agent remote write secret: %w", err)
	}

	logger.Info("deleted prometheus agent remote write configmap and secret")

	return nil
}

func (pas PrometheusAgentService) deleteConfigMap(ctx context.Context, cluster *clusterv1.Cluster) error {
	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
		Namespace: cluster.GetNamespace(),
	}
	configMap := &corev1.ConfigMap{}
	// Get the current configmap if it exists.
	err := pas.Get(ctx, objectKey, configMap)
	if apierrors.IsNotFound(err) {
		// Ignore cases where the configmap is not found (if it was manually deleted, for instance).
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get remote write configmap: %w", err)
	}

	err = pas.Delete(ctx, configMap)
	if err != nil {
		return fmt.Errorf("failed to delete remote write configmap: %w", err)
	}
	return nil
}

func (pas PrometheusAgentService) deleteSecret(ctx context.Context, cluster *clusterv1.Cluster) error {
	objectKey := client.ObjectKey{
		Name:      GetPrometheusAgentRemoteWriteSecretName(cluster),
		Namespace: cluster.GetNamespace(),
	}
	secret := &corev1.Secret{}
	// Get the current secret if it exists.
	err := pas.Get(ctx, objectKey, secret)
	if apierrors.IsNotFound(err) {
		// Ignore cases where the secret is not found (if it was manually deleted, for instance).
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get remote write secret: %w", err)
	}

	err = pas.Delete(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed to delete remote write secret: %w", err)
	}
	return nil
}
