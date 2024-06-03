package mimir

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/common/secret"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
)

const (
	ingressAuthSecretName = "mimir-gateway-ingress-auth"
	mimirApiKey           = "mimir-basic-auth"
	mimirNamespace        = "mimir"
)

type MimirService struct {
	client.Client
	PasswordManager password.Manager
	common.ManagementCluster
}

// ConfigureMimir configures the ingress and its authentication (basic auth)
// to allow prometheus agents to send their data to Mimir
func (ms *MimirService) ConfigureMimir(ctx context.Context) error {
	logger := log.FromContext(ctx).WithValues("cluster", ms.ManagementCluster.Name)
	logger.Info("configuring mimir ingress")

	err := ms.CreateApiKey(ctx, logger)
	if err != nil {
		logger.Error(err, "failed to create mimir auth secret")
		return errors.WithStack(err)
	}

	err = ms.CreateIngressAuthenticationSecret(ctx, logger)
	if err != nil {
		logger.Error(err, "failed to create mimir ingress secret")
		return errors.WithStack(err)
	}

	logger.Info("configured mimir ingress")

	return nil
}

func (ms *MimirService) CreateApiKey(ctx context.Context, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      mimirApiKey,
		Namespace: mimirNamespace,
	}

	current := &corev1.Secret{}
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// First all secrets using the password from the mimirApiKey secret are deleted
		// to ensure that they won't use an outdated password.
		logger.Info("Deleting old secrets")

		err := secret.DeleteSecret(ingressAuthSecretName, mimirNamespace, ctx, ms.Client)
		if err != nil {
			return errors.WithStack(err)
		}

		clusterList := &clusterv1.ClusterList{}
		err = ms.Client.List(ctx, clusterList)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, cluster := range clusterList.Items {
			secretName := prometheusagent.GetPrometheusAgentRemoteWriteSecretName(&cluster)
			err = secret.DeleteSecret(secretName, cluster.Namespace, ctx, ms.Client)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		// Once all secrets are deleted,the mimirApiKey one may be created.
		logger.Info("Building auth secret")

		password, err := ms.PasswordManager.GeneratePassword(32)
		if err != nil {
			return errors.WithStack(err)
		}

		secret := secret.GenerateGenericSecret(
			mimirApiKey, mimirNamespace, "credentials", password)

		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("Auth secret successfully created")

		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (ms *MimirService) CreateIngressAuthenticationSecret(ctx context.Context, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      ingressAuthSecretName,
		Namespace: mimirNamespace,
	}

	current := &corev1.Secret{}
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		logger.Info("building ingress secret")

		password, err := prometheusagent.GetMimirIngressPassword(ctx)
		if err != nil {
			return errors.WithStack(err)
		}

		htpasswd, err := ms.PasswordManager.GenerateHtpasswd(ms.ManagementCluster.Name, password)
		if err != nil {
			return errors.WithStack(err)
		}

		secret := secret.GenerateGenericSecret(ingressAuthSecretName, mimirNamespace, "auth", htpasswd)

		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("ingress secret successfully created")

		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (ms *MimirService) DeleteMimirSecrets(ctx context.Context) error {
	err := secret.DeleteSecret(ingressAuthSecretName, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	err = secret.DeleteSecret(mimirApiKey, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
