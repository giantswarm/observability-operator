package auth

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AuthManager manages authentication secrets for observability services
type AuthManager interface {
	// Cluster password lifecycle
	AddClusterPassword(ctx context.Context, clusterName string) error
	RemoveClusterPassword(ctx context.Context, clusterName string) error
	GetClusterPassword(ctx context.Context, clusterName string) (string, error)

	// Cleanup
	DeleteAllSecrets(ctx context.Context) error
}

type authManager struct {
	client            client.Client
	passwordGenerator PasswordGenerator
	config            Config
}

// NewAuthManager creates a new auth manager
func NewAuthManager(client client.Client, config Config) AuthManager {
	return &authManager{
		client:            client,
		passwordGenerator: NewPasswordGenerator(),
		config:            config,
	}
}

// getAuthSecret returns a reusable auth secret object with the correct metadata
func (am *authManager) getAuthSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.config.AuthSecretName,
			Namespace: am.config.AuthSecretNamespace,
		},
	}
}

// getAllClusterPasswords gets all cluster passwords from the auth secret
func (am *authManager) getAllClusterPasswords(ctx context.Context) (map[string]string, error) {
	authSecret := am.getAuthSecret()
	err := am.client.Get(ctx, client.ObjectKeyFromObject(authSecret), authSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth secret: %w", err)
	}

	clusterPasswords := make(map[string]string)
	for clusterName, passwordBytes := range authSecret.Data {
		// Skip credentials entry if it exists (legacy compatibility)
		if clusterName == "credentials" {
			continue
		}
		clusterPasswords[clusterName] = string(passwordBytes)
	}

	return clusterPasswords, nil
}

// generateHtpasswdContent creates htpasswd content from all cluster passwords
func (am *authManager) generateHtpasswdContent(ctx context.Context) (string, error) {
	clusterPasswords, err := am.getAllClusterPasswords(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster passwords: %w", err)
	}

	var htpasswdLines []string
	for clusterName, password := range clusterPasswords {
		htpasswdEntry, err := am.passwordGenerator.GenerateHtpasswd(clusterName, password)
		if err != nil {
			return "", fmt.Errorf("failed to generate htpasswd for cluster %s: %w", clusterName, err)
		}
		htpasswdLines = append(htpasswdLines, htpasswdEntry)
	}

	return strings.Join(htpasswdLines, "\n"), nil
}

// createOrUpdateHtpasswdSecret creates or updates an htpasswd secret with the given name and data key
func (am *authManager) createOrUpdateHtpasswdSecret(ctx context.Context, secretName, dataKey string) error {
	logger := log.FromContext(ctx)
	htpasswdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: am.config.SecretsNamespace,
		},
	}

	result, err := ctrl.CreateOrUpdate(ctx, am.client, htpasswdSecret, func() error {
		// Generate htpasswd content
		htpasswdContent, err := am.generateHtpasswdContent(ctx)
		if err != nil {
			return err
		}

		// Initialize or update the secret data
		if htpasswdSecret.Data == nil {
			htpasswdSecret.Data = make(map[string][]byte)
		}
		htpasswdSecret.Data[dataKey] = []byte(htpasswdContent)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update %s secret: %w", secretName, err)
	}

	logger.Info("Htpasswd secret processed", "secret", secretName, "result", result)
	return nil
}

// AddClusterPassword adds a password for a specific cluster to the auth secret
func (am *authManager) AddClusterPassword(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)
	authSecret := am.getAuthSecret()
	passwordGenerated := false

	result, err := ctrl.CreateOrUpdate(ctx, am.client, authSecret, func() error {
		// Initialize Data map if it doesn't exist
		if authSecret.Data == nil {
			authSecret.Data = make(map[string][]byte)
		}

		// Check if cluster already has a password
		if _, exists := authSecret.Data[clusterName]; exists {
			return nil // Already exists, no changes needed
		}

		// Generate password for new cluster
		password, err := am.passwordGenerator.GeneratePassword(32)
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
		err = am.regenerateHtpasswdSecrets(ctx)
		if err != nil {
			return fmt.Errorf("failed to regenerate htpasswd secrets: %w", err)
		}
	}

	return nil
}

// RemoveClusterPassword removes a cluster's password from the auth secret
func (am *authManager) RemoveClusterPassword(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)
	authSecret := am.getAuthSecret()
	passwordRemoved := false

	result, err := ctrl.CreateOrUpdate(ctx, am.client, authSecret, func() error {
		// If Data doesn't exist or cluster password doesn't exist, nothing to do
		if authSecret.Data == nil || authSecret.Data[clusterName] == nil {
			return nil
		}

		// Remove cluster password
		delete(authSecret.Data, clusterName)
		passwordRemoved = true
		return nil
	})

	if err != nil {
		// If the namespace or secret doesn't exist, that's fine - cluster is being deleted anyway
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Auth secret or namespace not found during cluster deletion - this is expected", "cluster", clusterName)
			return nil
		}
		return fmt.Errorf("failed to update auth secret: %w", err)
	}

	logger.Info("Auth secret processed for removal", "result", result, "cluster", clusterName)

	// Only regenerate htpasswd secrets if we actually removed a password
	if passwordRemoved {
		err = am.regenerateHtpasswdSecrets(ctx)
		if err != nil {
			// Also ignore not found errors when regenerating - if mimir namespace is gone, that's expected
			if client.IgnoreNotFound(err) == nil {
				logger.Info("Mimir namespace not found during htpasswd regeneration - this is expected during deletion", "cluster", clusterName)
				return nil
			}
			return fmt.Errorf("failed to regenerate htpasswd secrets: %w", err)
		}
	}

	return nil
}

// GetClusterPassword retrieves the password for a specific cluster
func (am *authManager) GetClusterPassword(ctx context.Context, clusterName string) (string, error) {
	authSecret := am.getAuthSecret()

	err := am.client.Get(ctx, types.NamespacedName{
		Name:      am.config.AuthSecretName,
		Namespace: am.config.AuthSecretNamespace,
	}, authSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get auth secret: %w", err)
	}

	// Check if cluster password exists
	if passwordBytes, exists := authSecret.Data[clusterName]; exists {
		return string(passwordBytes), nil
	}

	return "", fmt.Errorf("password not found for cluster %s", clusterName)
}

// regenerateHtpasswdSecrets updates both ingress and httproute secrets
func (am *authManager) regenerateHtpasswdSecrets(ctx context.Context) error {
	err := am.createOrUpdateHtpasswdSecret(ctx, am.config.IngressSecretName, IngressDataKey)
	if err != nil {
		return fmt.Errorf("failed to update ingress secret: %w", err)
	}
	err = am.createOrUpdateHtpasswdSecret(ctx, am.config.HTTPRouteSecretName, HTTPRouteDataKey)
	if err != nil {
		return fmt.Errorf("failed to update httproute secret: %w", err)
	}

	return nil
}

// DeleteAllSecrets deletes all managed secrets
func (am *authManager) DeleteAllSecrets(ctx context.Context) error {
	// Delete ingress secret
	ingressSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.config.IngressSecretName,
			Namespace: am.config.SecretsNamespace,
		},
	}
	if err := client.IgnoreNotFound(am.client.Delete(ctx, ingressSecret)); err != nil {
		return fmt.Errorf("failed to delete ingress secret %s/%s: %w", am.config.SecretsNamespace, am.config.IngressSecretName, err)
	}

	// Delete httproute secret
	httprouteSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.config.HTTPRouteSecretName,
			Namespace: am.config.SecretsNamespace,
		},
	}
	if err := client.IgnoreNotFound(am.client.Delete(ctx, httprouteSecret)); err != nil {
		return fmt.Errorf("failed to delete httproute secret %s/%s: %w", am.config.SecretsNamespace, am.config.HTTPRouteSecretName, err)
	}

	// Delete auth secret
	authSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.config.AuthSecretName,
			Namespace: am.config.AuthSecretNamespace,
		},
	}
	if err := client.IgnoreNotFound(am.client.Delete(ctx, authSecret)); err != nil {
		return fmt.Errorf("failed to delete auth secret %s/%s: %w", am.config.AuthSecretNamespace, am.config.AuthSecretName, err)
	}

	return nil
}
