package mimir

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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

// getAuthSecret returns a reusable auth secret object with the correct metadata
func (ms *MimirService) getAuthSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mimirApiKey,
			Namespace: mimirNamespace,
		},
	}
}

// generateHtpasswdContent creates htpasswd content from all cluster passwords
func (ms *MimirService) generateHtpasswdContent(ctx context.Context) (string, error) {
	// Get all cluster passwords from the centralized secret
	clusterPasswords, err := commonmonitoring.GetAllClusterPasswords(ctx, ms.Client)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster passwords: %w", err)
	}

	// Generate htpasswd entries for all clusters
	var htpasswdLines []string
	for clusterName, password := range clusterPasswords {
		htpasswdEntry, err := ms.PasswordManager.GenerateHtpasswd(clusterName, password)
		if err != nil {
			return "", fmt.Errorf("failed to generate htpasswd for cluster %s: %w", clusterName, err)
		}
		htpasswdLines = append(htpasswdLines, htpasswdEntry)
	}

	return strings.Join(htpasswdLines, "\n"), nil
}

// createOrUpdateHtpasswdSecret creates or updates an htpasswd secret with the given name and data key
func (ms *MimirService) createOrUpdateHtpasswdSecret(ctx context.Context, secretName, dataKey string, logger logr.Logger) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: mimirNamespace,
		},
	}

	result, err := ctrl.CreateOrUpdate(ctx, ms.Client, secret, func() error {
		// Generate htpasswd content
		htpasswdContent, err := ms.generateHtpasswdContent(ctx)
		if err != nil {
			return err
		}

		// Initialize or update the secret data
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[dataKey] = []byte(htpasswdContent)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update %s secret: %w", secretName, err)
	}

	logger.Info("Htpasswd secret processed", "secret", secretName, "result", result)
	return nil
}

func (ms *MimirService) configureIngressAuthenticationSecret(ctx context.Context, logger logr.Logger) error {
	return ms.createOrUpdateHtpasswdSecret(ctx, ingressAuthSecretName, "auth", logger)
}

func (ms *MimirService) configureHTTPRouteAuthenticationSecret(ctx context.Context, logger logr.Logger) error {
	return ms.createOrUpdateHtpasswdSecret(ctx, httprouteAuthSecretName, ".htpasswd", logger)
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

// AddClusterPassword adds a password for a specific cluster to the auth secret,
// creates the secret if it doesn't exist, and regenerates htpasswd secrets
func (ms *MimirService) AddClusterPassword(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)
	authSecret := ms.getAuthSecret()
	passwordGenerated := false

	result, err := ctrl.CreateOrUpdate(ctx, ms.Client, authSecret, func() error {
		// Initialize Data map if it doesn't exist
		if authSecret.Data == nil {
			authSecret.Data = make(map[string][]byte)
		}

		// Check if cluster already has a password
		if _, exists := authSecret.Data[clusterName]; exists {
			return nil // Already exists, no changes needed
		}

		// Generate password for new cluster
		password, err := ms.PasswordManager.GeneratePassword(32)
		if err != nil {
			return fmt.Errorf("failed to generate password for cluster %s: %w", clusterName, err)
		}

		// Add to secret
		authSecret.Data[clusterName] = []byte(password)
		passwordGenerated = true
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update auth secret: %w", err)
	}

	logger.Info("Auth secret processed", "result", result, "cluster", clusterName)

	// Only regenerate htpasswd secrets if we actually added a new password
	if passwordGenerated {
		err = ms.regenerateAuthSecrets(ctx, logger)
		if err != nil {
			return fmt.Errorf("failed to regenerate htpasswd secrets: %w", err)
		}
	}

	return nil
}

// RemoveClusterPassword removes a cluster's password from the auth secret
// and regenerates htpasswd secrets
func (ms *MimirService) RemoveClusterPassword(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)
	authSecret := ms.getAuthSecret()
	passwordRemoved := false

	result, err := ctrl.CreateOrUpdate(ctx, ms.Client, authSecret, func() error {
		// Remove cluster password
		delete(authSecret.Data, clusterName)
		return nil
	})

	if err != nil {
		// If the namespace or secret doesn't exist, that's fine - cluster is being deleted anyway
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("failed to update auth secret: %w", err)
	}

	logger.Info("Auth secret processed for removal", "result", result, "cluster", clusterName)

	err = ms.regenerateAuthSecrets(ctx, logger)
	if err != nil {
		// Also ignore not found errors when regenerating - if mimir namespace is gone, that's expected
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("failed to regenerate htpasswd secrets: %w", err)
	}

	return nil
}

// regenerateAuthSecrets updates the ingress and httproute auth secrets with current cluster passwords
func (ms *MimirService) regenerateAuthSecrets(ctx context.Context, logger logr.Logger) error {
	// Update secrets with current cluster passwords
	err := ms.configureIngressAuthenticationSecret(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to update ingress auth secret: %w", err)
	}

	err = ms.configureHTTPRouteAuthenticationSecret(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to update httproute auth secret: %w", err)
	}

	return nil
}
