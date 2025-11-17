package mimir

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/common/secret"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	ingressAuthSecretName   = "mimir-gateway-ingress-auth"   // #nosec G101
	httprouteAuthSecretName = "mimir-gateway-httproute-auth" // #nosec G101
	mimirApiKey             = "mimir-basic-auth"             // #nosec G101
	mimirNamespace          = "mimir"
)

type MimirService struct {
	Client          client.Client
	PasswordManager password.Manager
	config.Config
}

// ConfigureMimir configures the ingress and its authentication (basic auth)
// to allow alloys to send their data to Mimir
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

	err = ms.CreateHTTPRouteAuthenticationSecret(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to create mimir httproute secret: %w", err)
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
			return fmt.Errorf("failed to delete secret %s/%s: %w", mimirNamespace, ingressAuthSecretName, err)
		}

		// Once all secrets are deleted,the mimirApiKey one may be created.
		logger.Info("Building auth secret")

		password, err := ms.PasswordManager.GeneratePassword(32)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}

		secret := secret.GenerateGenericSecret(
			mimirApiKey, mimirNamespace, "credentials", password)

		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to create secret %s/%s: %w", mimirNamespace, mimirApiKey, err)
		}

		logger.Info("Auth secret successfully created")

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", mimirNamespace, mimirApiKey, err)
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

		password, err := commonmonitoring.GetMimirAuthPassword(ctx, ms.Client)
		if err != nil {
			return fmt.Errorf("failed to get mimir auth password: %w", err)
		}

		htpasswd, err := ms.PasswordManager.GenerateHtpasswd(ms.Cluster.Name, password)
		if err != nil {
			return fmt.Errorf("failed to generate htpasswd: %w", err)
		}

		secret := secret.GenerateGenericSecret(ingressAuthSecretName, mimirNamespace, "auth", htpasswd)

		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to create secret %s/%s: %w", mimirNamespace, ingressAuthSecretName, err)
		}

		logger.Info("ingress secret successfully created")

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", mimirNamespace, ingressAuthSecretName, err)
	}

	return nil
}

func (ms *MimirService) CreateHTTPRouteAuthenticationSecret(ctx context.Context, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      httprouteAuthSecretName,
		Namespace: mimirNamespace,
	}

	current := &corev1.Secret{}
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		logger.Info("building httproute secret")

		password, err := commonmonitoring.GetMimirAuthPassword(ctx, ms.Client)
		if err != nil {
			return fmt.Errorf("failed to get mimir auth password: %w", err)
		}

		htpasswd, err := ms.PasswordManager.GenerateHtpasswd(ms.Cluster.Name, password)
		if err != nil {
			return fmt.Errorf("failed to generate htpasswd: %w", err)
		}

		secret := secret.GenerateGenericSecret(httprouteAuthSecretName, mimirNamespace, ".htpasswd", htpasswd)

		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to create secret %s/%s: %w", mimirNamespace, httprouteAuthSecretName, err)
		}

		logger.Info("HTTPRoute secret successfully created")

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", mimirNamespace, httprouteAuthSecretName, err)
	}

	return nil
}

func (ms *MimirService) DeleteMimirSecrets(ctx context.Context) error {
	err := secret.DeleteSecret(ingressAuthSecretName, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s/%s: %w", mimirNamespace, ingressAuthSecretName, err)
	}

	err = secret.DeleteSecret(httprouteAuthSecretName, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s/%s: %w", mimirNamespace, httprouteAuthSecretName, err)
	}

	err = secret.DeleteSecret(mimirApiKey, mimirNamespace, ctx, ms.Client)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s/%s: %w", mimirNamespace, mimirApiKey, err)
	}

	return nil
}
