package prometheusagent

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

type PrometheusAgentService struct {
	client.Client
	organization.OrganizationRepository
	PasswordManager password.Manager
	common.ManagementCluster
	PrometheusVersion string
}

// ReconcileRemoteWriteConfig ensures that the prometheus remote write config is present in the cluster.
func (pas *PrometheusAgentService) ReconcileRemoteWriteConfig(
	ctx context.Context, cluster *clusterv1.Cluster) error {

	logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)
	logger.Info("ensuring prometheus agent remote write configuration")

	err := pas.createOrUpdateConfig(ctx, cluster, logger)
	if err != nil {
		logger.Error(err, "failed to create or update prometheus agent remote write config")
		return errors.WithStack(err)
	}

	err = pas.createOrUpdateSecret(ctx, cluster, logger)
	if err != nil {
		logger.Error(err, "failed to create or update prometheus agent remote write secret")
		return errors.WithStack(err)
	}

	logger.Info("ensured prometheus agent remote write configuration")

	return nil
}

func (pas PrometheusAgentService) createOrUpdateConfig(ctx context.Context,
	cluster *clusterv1.Cluster, logger logr.Logger) error {

	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
		Namespace: cluster.GetNamespace(),
	}

	current := &corev1.ConfigMap{}
	// Get the current configmap if it exists.
	err := pas.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		configMap, err := pas.buildRemoteWriteConfig(ctx, cluster, logger, defaultShards)
		if err != nil {
			return errors.WithStack(err)
		}

		err = pas.Client.Create(ctx, configMap)
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	currentShards, err := readCurrentShardsFromConfig(*current)
	if err != nil {
		return errors.WithStack(err)
	}

	desired, err := pas.buildRemoteWriteConfig(ctx, cluster, logger, currentShards)
	if err != nil {
		return errors.WithStack(err)
	}

	if !reflect.DeepEqual(current.Data, desired.Data) || !reflect.DeepEqual(current.Finalizers, desired.Finalizers) {
		err = pas.Client.Update(ctx, desired)
		if err != nil {
			logger.Info("could not update prometheus agent remote write configuration")
			return errors.WithStack(err)
		}
	}
	return nil
}

func (pas PrometheusAgentService) createOrUpdateSecret(ctx context.Context,
	cluster *clusterv1.Cluster, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteSecretName(cluster),
		Namespace: cluster.GetNamespace(),
	}

	current := &corev1.Secret{}
	// Get the current secret if it exists.
	err := pas.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		logger.Info("generating password for the prometheus agent")
		password, err := pas.PasswordManager.GeneratePassword(32)
		if err != nil {
			logger.Error(err, "failed to generate the prometheus agent password")
			return errors.WithStack(err)
		}
		logger.Info("generated password for the prometheus agent")

		secret, err := pas.buildRemoteWriteSecret(cluster, password)
		if err != nil {
			return errors.WithStack(err)
		}
		err = pas.Client.Create(ctx, secret)
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	// As it takes a long time to apply the new password to the agent due to a built-in delay in the app-platform,
	// we keep the already generated remote write password.
	password, err := readRemoteWritePasswordFromSecret(*current)
	if err != nil {
		return errors.WithStack(err)
	}

	desired, err := pas.buildRemoteWriteSecret(cluster, password)
	if err != nil {
		return errors.WithStack(err)
	}
	if !reflect.DeepEqual(current.Data, desired.Data) || !reflect.DeepEqual(current.Finalizers, desired.Finalizers) {
		err = pas.Client.Update(ctx, desired)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (pas *PrometheusAgentService) DeleteRemoteWriteConfig(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)
	logger.Info("deleting prometheus agent remote write configuration")

	err := pas.deleteConfigMap(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to delete prometheus agent remote write config")
		return errors.WithStack(err)
	}

	err = pas.deleteSecret(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to delete prometheus agent remote write secret")
		return errors.WithStack(err)
	}

	logger.Info("deleted prometheus agent remote write configuration")

	return nil
}

func (pas PrometheusAgentService) deleteConfigMap(ctx context.Context, cluster *clusterv1.Cluster) error {
	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
		Namespace: cluster.GetNamespace(),
	}
	current := &corev1.ConfigMap{}
	// Get the current configmap if it exists.
	err := pas.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// Ignore cases where the configmap is not found (if it was manually deleted, for instance).
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	// Delete the finalizer
	desired := current.DeepCopy()
	controllerutil.RemoveFinalizer(desired, monitoring.MonitoringFinalizer)
	err = pas.Client.Patch(ctx, desired, client.MergeFrom(current))
	if err != nil {
		return errors.WithStack(err)
	}

	err = pas.Client.Delete(ctx, desired)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (pas PrometheusAgentService) deleteSecret(ctx context.Context, cluster *clusterv1.Cluster) error {
	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteSecretName(cluster),
		Namespace: cluster.GetNamespace(),
	}
	current := &corev1.Secret{}
	// Get the current secret if it exists.
	err := pas.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// Ignore cases where the secret is not found (if it was manually deleted, for instance).
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	// Delete the finalizer
	desired := current.DeepCopy()
	controllerutil.RemoveFinalizer(desired, monitoring.MonitoringFinalizer)
	err = pas.Client.Patch(ctx, current, client.MergeFrom(desired))
	if err != nil {
		return errors.WithStack(err)
	}

	err = pas.Client.Delete(ctx, desired)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
