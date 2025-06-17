package mimir

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/common/secret"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
)

const (
	ingressAuthSecretName = "mimir-gateway-ingress-auth" // #nosec G101
	mimirApiKey           = "mimir-basic-auth"           // #nosec G101
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
	logger := log.FromContext(ctx)
	logger.Info("configuring mimir ingress")

	err := ms.CreateApiKey(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to create mimir auth secret: %w", err)
	}

	err = ms.CreateIngressAuthenticationSecret(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to create mimir ingress secret: %w", err)
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
	err := ms.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// First all secrets using the password from the mimirApiKey secret are deleted
		// to ensure that they won't use an outdated password.
		logger.Info("Deleting old secrets")

		err := secret.DeleteSecret(ingressAuthSecretName, mimirNamespace, ctx, ms.Client)
		if err != nil {
			return fmt.Errorf("failed to delete old ingress auth secret: %w", err)
		}

		clusterList := &clusterv1.ClusterList{}
		err = ms.List(ctx, clusterList)
		if err != nil {
			return fmt.Errorf("failed to list clusters for secret cleanup: %w", err)
		}

		for _, cluster := range clusterList.Items {
			secretName := prometheusagent.GetPrometheusAgentRemoteWriteSecretName(&cluster) // #nosec G601
			err = secret.DeleteSecret(secretName, cluster.Namespace, ctx, ms.Client)
			if err != nil {
				return fmt.Errorf("failed to delete prometheus agent secret for cluster %s: %w", cluster.Name, err)
			}
		}

		// Once all secrets are deleted,the mimirApiKey one may be created.
		logger.Info("Building auth secret")

		password, err := ms.PasswordManager.GeneratePassword(32)
		if err != nil {
			return fmt.Errorf("failed to generate password for mimir auth: %w", err)
		}

		secret := secret.GenerateGenericSecret(
			mimirApiKey, mimirNamespace, "credentials", password)

		err = ms.Create(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to create mimir auth secret: %w", err)
		}

		logger.Info("Auth secret successfully created")

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get existing mimir auth secret: %w", err)
	}

	return nil
}

func (ms *MimirService) CreateIngressAuthenticationSecret(ctx context.Context, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      ingressAuthSecretName,
		Namespace: mimirNamespace,
	}

	current := &corev1.Secret{}
	err := ms.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		logger.Info("building ingress secret")

		password, err := commonmonitoring.GetMimirIngressPassword(ctx, ms.Client)
		if err != nil {
			return fmt.Errorf("failed to get mimir ingress password: %w", err)
		}

		htpasswd, err := ms.PasswordManager.GenerateHtpasswd(ms.Name, password)
		if err != nil {
			return fmt.Errorf("failed to generate htpasswd for mimir ingress: %w", err)
		}

		secret := secret.GenerateGenericSecret(ingressAuthSecretName, mimirNamespace, "auth", htpasswd)

		err = ms.Create(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to create mimir ingress auth secret: %w", err)
		}

		logger.Info("ingress secret successfully created")

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get existing mimir ingress auth secret: %w", err)
	}

	return nil
}

func (ms *MimirService) DeleteMimirSecrets(ctx context.Context) error {
	err := secret.DeleteSecret(ingressAuthSecretName, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return fmt.Errorf("failed to delete mimir ingress auth secret: %w", err)
	}

	err = secret.DeleteSecret(mimirApiKey, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return fmt.Errorf("failed to delete mimir API key secret: %w", err)
	}

	return nil
}
