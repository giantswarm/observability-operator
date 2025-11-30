package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClusterAuthData represents the authentication data for a single cluster
type ClusterAuthData struct {
	Password string `json:"password"`
	Htpasswd string `json:"htpasswd"`
}

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

// generateHtpasswdContent creates htpasswd content from entries in auth secret
func (am *authManager) generateHtpasswdContent(ctx context.Context) (string, error) {
	authSecret := am.getAuthSecret()
	err := am.client.Get(ctx, client.ObjectKeyFromObject(authSecret), authSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get auth secret: %w", err)
	}

	// Collect cluster names
	var clusterNames []string
	for clusterName := range authSecret.Data {
		// TODO: Remove once the credentials key has been removed from all installations in a subsequent release
		// This was the old format before per-cluster passwords were introduced
		if clusterName == "credentials" {
			continue
		}
		clusterNames = append(clusterNames, clusterName)
	}

	// Sort cluster names for deterministic output
	sort.Strings(clusterNames)

	// Build htpasswd content from entries
	var htpasswdLines []string
	for _, clusterName := range clusterNames {
		if clusterAuthBytes, exists := authSecret.Data[clusterName]; exists {
			var clusterAuthData ClusterAuthData
			if err := json.Unmarshal(clusterAuthBytes, &clusterAuthData); err != nil {
				return "", fmt.Errorf("failed to unmarshal cluster data for %s: %w", clusterName, err)
			}

			// Use cached htpasswd from JSON structure
			if clusterAuthData.Htpasswd == "" {
				return "", fmt.Errorf("missing htpasswd entry for cluster %s", clusterName)
			}
			htpasswdLines = append(htpasswdLines, clusterAuthData.Htpasswd)
		}
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

		// Generate htpasswd entry
		htpasswdEntry, err := am.passwordGenerator.GenerateHtpasswd(clusterName, password)
		if err != nil {
			return fmt.Errorf("failed to generate htpasswd for cluster %s: %w", clusterName, err)
		}

		// Store both password and htpasswd in JSON format
		clusterData := ClusterAuthData{
			Password: password,
			Htpasswd: htpasswdEntry,
		}
		clusterDataBytes, err := json.Marshal(clusterData)
		if err != nil {
			return fmt.Errorf("failed to marshal cluster data for %s: %w", clusterName, err)
		}
		authSecret.Data[clusterName] = clusterDataBytes

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update auth secret: %w", err)
	}

	logger.Info("Auth secret processed", "result", result, "cluster", clusterName)

	// Always ensure htpasswd secrets are up to date
	// This handles cases where auth secret was updated but htpasswd generation failed previously
	err = am.regenerateHtpasswdSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure htpasswd secrets are consistent: %w", err)
	}

	return nil
}

// RemoveClusterPassword removes a cluster's password from the auth secret
func (am *authManager) RemoveClusterPassword(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)
	authSecret := am.getAuthSecret()

	result, err := ctrl.CreateOrUpdate(ctx, am.client, authSecret, func() error {
		// If Data doesn't exist, nothing to do
		if authSecret.Data == nil {
			return nil
		}
		// Check if cluster password exists using comma-ok idiom
		if _, exists := authSecret.Data[clusterName]; !exists {
			return nil
		}

		// Remove cluster entry
		delete(authSecret.Data, clusterName)
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

	// Always ensure htpasswd secrets are consistent
	err = am.regenerateHtpasswdSecrets(ctx)
	if err != nil {
		// Also ignore not found errors when regenerating - if mimir namespace is gone, that's expected
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Mimir namespace not found during htpasswd regeneration - this is expected during deletion", "cluster", clusterName)
			return nil
		}
		return fmt.Errorf("failed to ensure htpasswd secrets are consistent: %w", err)
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

	// Check if cluster data exists
	if clusterAuthBytes, exists := authSecret.Data[clusterName]; exists {
		var clusterAuthData ClusterAuthData
		if err := json.Unmarshal(clusterAuthBytes, &clusterAuthData); err != nil {
			return "", fmt.Errorf("failed to unmarshal cluster data for %s: %w", clusterName, err)
		}
		return clusterAuthData.Password, nil
	}

	return "", fmt.Errorf("password not found for cluster %s", clusterName)
}

// regenerateHtpasswdSecrets ensures both ingress and httproute secrets are up to date
// This method always checks and updates the secrets regardless of whether changes were made
// to fix potential consistency issues from previous failed operations
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
