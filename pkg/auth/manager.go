package auth

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AuthManager manages authentication secrets for observability services
type AuthManager interface {
	// Cluster authentication lifecycle
	EnsureClusterAuth(ctx context.Context, cluster *clusterv1.Cluster) error
	DeleteClusterAuth(ctx context.Context, cluster *clusterv1.Cluster) error
	GetClusterPassword(ctx context.Context, cluster *clusterv1.Cluster) (string, error)

	// Cleanup
	DeleteGatewaySecrets(ctx context.Context) error
}

type authManager struct {
	client            client.Client
	passwordGenerator PasswordGenerator
	config            Config
}

// NewAuthManager creates a new auth manager with config for managing htpasswd secrets
func NewAuthManager(client client.Client, config Config) AuthManager {
	return &authManager{
		client:            client,
		passwordGenerator: NewPasswordGenerator(),
		config:            config,
	}
}

// getClusterSecretName returns the secret name for a cluster's auth
func (am *authManager) getClusterSecretName(clusterName string) string {
	return fmt.Sprintf("%s-observability-%s-auth", clusterName, am.config.AuthType)
}

// getClusterSecret returns a cluster auth secret object with the correct metadata
func (am *authManager) getClusterSecret(cluster *clusterv1.Cluster) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.getClusterSecretName(cluster.Name),
			Namespace: cluster.Namespace,
		},
	}
}

// generateHtpasswdContent creates htpasswd content from per-cluster auth secrets
func (am *authManager) generateHtpasswdContent(ctx context.Context) (string, error) {
	// List all cluster auth secrets using labels across all namespaces
	secretList := &corev1.SecretList{}
	err := am.client.List(ctx, secretList,
		client.MatchingLabels{
			"app.kubernetes.io/component":           fmt.Sprintf("%s-auth", am.config.AuthType),
			"observability.giantswarm.io/auth-type": string(am.config.AuthType),
		})
	if err != nil {
		return "", fmt.Errorf("failed to list cluster auth secrets: %w", err)
	}

	// Collect htpasswd entries from cluster secrets
	var htpasswdEntries []string
	for _, secret := range secretList.Items {
		htpasswdData, exists := secret.Data["htpasswd"]
		if !exists {
			continue // Skip secrets without htpasswd data
		}
		htpasswdEntries = append(htpasswdEntries, string(htpasswdData))
	}

	// Sort for deterministic output
	sort.Strings(htpasswdEntries)

	return strings.Join(htpasswdEntries, "\n"), nil
}

// createOrUpdateGatewaySecret creates or updates a gateway secret with htpasswd content
func (am *authManager) createOrUpdateGatewaySecret(ctx context.Context, secretName, dataKey string) error {
	logger := log.FromContext(ctx)
	gatewaySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: am.config.GatewaySecrets.Namespace,
		},
	}

	result, err := ctrl.CreateOrUpdate(ctx, am.client, gatewaySecret, func() error {
		// Generate htpasswd content from per-cluster secrets
		htpasswdContent, err := am.generateHtpasswdContent(ctx)
		if err != nil {
			return err
		}

		// Initialize or update the secret data
		if gatewaySecret.Data == nil {
			gatewaySecret.Data = make(map[string][]byte)
		}
		gatewaySecret.Data[dataKey] = []byte(htpasswdContent)

		return nil
	})

	if err != nil {
		// If namespace doesn't exist, ignore the error - this happens during deletion
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Gateway secret namespace not found - this is expected during deletion", "secret", secretName, "namespace", am.config.GatewaySecrets.Namespace)
			return nil
		}
		return fmt.Errorf("failed to create or update gateway secret %s: %w", secretName, err)
	}

	logger.Info("Gateway secret processed", "secret", secretName, "result", result)
	return nil
}

// GetClusterPassword retrieves the password for a specific cluster
func (am *authManager) GetClusterPassword(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	clusterSecret := am.getClusterSecret(cluster)

	err := am.client.Get(ctx, client.ObjectKeyFromObject(clusterSecret), clusterSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster auth secret: %w", err)
	}

	password, exists := clusterSecret.Data["password"]
	if !exists {
		return "", fmt.Errorf("password not found in cluster auth secret")
	}

	return string(password), nil
}

// regenerateGatewaySecrets ensures both ingress and httproute gateway secrets are up to date
// This method aggregates htpasswd entries from all cluster secrets
func (am *authManager) regenerateGatewaySecrets(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// Update ingress gateway secret
	err := am.createOrUpdateGatewaySecret(ctx,
		am.config.GatewaySecrets.IngressSecretName,
		am.config.GatewaySecrets.IngressDataKey)
	if err != nil {
		// Ignore namespace not found errors during deletion - this is expected
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Gateway secrets namespace not found during regeneration - this is expected during deletion")
			return nil
		}
		return fmt.Errorf("failed to update ingress gateway secret: %w", err)
	}

	// Update HTTPRoute gateway secret
	err = am.createOrUpdateGatewaySecret(ctx,
		am.config.GatewaySecrets.HTTPRouteSecretName,
		am.config.GatewaySecrets.HTTPRouteDataKey)
	if err != nil {
		// Ignore namespace not found errors during deletion - this is expected
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Gateway secrets namespace not found during regeneration - this is expected during deletion")
			return nil
		}
		return fmt.Errorf("failed to update HTTPRoute gateway secret: %w", err)
	}

	return nil
}

// EnsureClusterAuth ensures cluster authentication is configured (implements controller interface)
func (am *authManager) EnsureClusterAuth(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	// Create per-cluster secret with owner reference
	clusterSecret := am.getClusterSecret(cluster)

	result, err := ctrl.CreateOrUpdate(ctx, am.client, clusterSecret, func() error {
		// Set owner reference for automatic cleanup
		if err := controllerutil.SetControllerReference(cluster, clusterSecret, am.client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		// Set labels for easy identification and filtering
		if clusterSecret.Labels == nil {
			clusterSecret.Labels = make(map[string]string)
		}
		clusterSecret.Labels["app.kubernetes.io/component"] = fmt.Sprintf("%s-auth", am.config.AuthType)
		clusterSecret.Labels["app.kubernetes.io/part-of"] = "observability-operator"
		clusterSecret.Labels["observability.giantswarm.io/cluster"] = cluster.Name
		clusterSecret.Labels["observability.giantswarm.io/auth-type"] = string(am.config.AuthType)

		// Initialize Data map if it doesn't exist
		if clusterSecret.Data == nil {
			clusterSecret.Data = make(map[string][]byte)
		}

		// Check if password already exists
		if _, hasPassword := clusterSecret.Data["password"]; hasPassword {
			return nil // Already configured
		}

		// Generate new credentials
		password, err := am.passwordGenerator.GeneratePassword(32)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}

		htpasswdEntry, err := am.passwordGenerator.GenerateHtpasswd(cluster.Name, password)
		if err != nil {
			return fmt.Errorf("failed to generate htpasswd: %w", err)
		}

		// Store credentials
		clusterSecret.Data["password"] = []byte(password)
		clusterSecret.Data["htpasswd"] = []byte(htpasswdEntry)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update cluster auth secret: %w", err)
	}

	logger.Info("Cluster auth secret processed", "secret", clusterSecret.Name, "result", result)

	// Regenerate gateway secrets to include this cluster
	return am.regenerateGatewaySecrets(ctx)
}

// DeleteClusterAuth removes cluster authentication (implements controller interface)
func (am *authManager) DeleteClusterAuth(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	clusterSecret := am.getClusterSecret(cluster)

	// Delete cluster secret (owner reference should handle this automatically, but be explicit)
	err := client.IgnoreNotFound(am.client.Delete(ctx, clusterSecret))
	if err != nil {
		return fmt.Errorf("failed to delete cluster auth secret: %w", err)
	}

	logger.Info("Cluster auth secret deleted", "secret", clusterSecret.Name)

	// Regenerate gateway secrets to exclude this cluster
	return am.regenerateGatewaySecrets(ctx)
}

// DeleteGatewaySecrets deletes all managed gateway secrets (for cleanup)
func (am *authManager) DeleteGatewaySecrets(ctx context.Context) error {
	// Delete gateway secrets
	ingressSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.config.GatewaySecrets.IngressSecretName,
			Namespace: am.config.GatewaySecrets.Namespace,
		},
	}
	if err := client.IgnoreNotFound(am.client.Delete(ctx, ingressSecret)); err != nil {
		return fmt.Errorf("failed to delete ingress gateway secret: %w", err)
	}

	httprouteSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      am.config.GatewaySecrets.HTTPRouteSecretName,
			Namespace: am.config.GatewaySecrets.Namespace,
		},
	}
	if err := client.IgnoreNotFound(am.client.Delete(ctx, httprouteSecret)); err != nil {
		return fmt.Errorf("failed to delete HTTPRoute gateway secret: %w", err)
	}

	// Delete all cluster auth secrets (they should be cleaned up by owner references, but be explicit)
	secretList := &corev1.SecretList{}
	err := am.client.List(ctx, secretList,
		client.MatchingLabels{
			"app.kubernetes.io/component":           fmt.Sprintf("%s-auth", am.config.AuthType),
			"observability.giantswarm.io/auth-type": string(am.config.AuthType),
		})
	if err != nil {
		return fmt.Errorf("failed to list cluster auth secrets for deletion: %w", err)
	}

	for _, secret := range secretList.Items {
		if err := client.IgnoreNotFound(am.client.Delete(ctx, &secret)); err != nil {
			return fmt.Errorf("failed to delete cluster auth secret %s: %w", secret.Name, err)
		}
	}

	return nil
}
