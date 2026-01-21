package agent

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
)

// AgentConfiguration represents the configuration for an Alloy agent deployment
type AgentConfiguration struct {
	// Cluster information
	ClusterName      string
	ClusterNamespace string

	// Resource names
	ConfigMapName string
	SecretName    string

	// Data to persist
	ConfigMapData map[string]string
	SecretData    map[string]string // Environment variables for the secret

	// Labels to apply to resources
	Labels map[string]string
}

// ConfigurationRepository manages persistence of agent configurations
type ConfigurationRepository interface {
	// Save creates or updates both ConfigMap and Secret for an agent
	Save(ctx context.Context, config *AgentConfiguration) error

	// Delete removes both ConfigMap and Secret for an agent
	Delete(ctx context.Context, clusterName, clusterNamespace, configMapName, secretName string) error
}

// k8sConfigurationRepository implements ConfigurationRepository using Kubernetes client
type k8sConfigurationRepository struct {
	client client.Client
}

// NewConfigurationRepository creates a new Kubernetes-based configuration repository
func NewConfigurationRepository(client client.Client) ConfigurationRepository {
	return &k8sConfigurationRepository{
		client: client,
	}
}

func (r *k8sConfigurationRepository) Save(ctx context.Context, config *AgentConfiguration) error {
	logger := log.FromContext(ctx)

	// Create or update ConfigMap
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.ClusterNamespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, configMap, func() error {
		configMap.Data = config.ConfigMapData
		configMap.Labels = config.Labels
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update configmap %s: %w", config.ConfigMapName, err)
	}

	logger.V(1).Info("saved configmap", "name", config.ConfigMapName, "namespace", config.ClusterNamespace)

	// Create or update Secret
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SecretName,
			Namespace: config.ClusterNamespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.client, secret, func() error {
		// Generate secret data using the shared template
		secretData, err := common.GenerateSecretData(config.SecretData)
		if err != nil {
			return fmt.Errorf("failed to generate secret data: %w", err)
		}
		secret.Data = map[string][]byte{
			"values": secretData,
		}
		secret.Labels = config.Labels
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update secret %s: %w", config.SecretName, err)
	}

	logger.V(1).Info("saved secret", "name", config.SecretName, "namespace", config.ClusterNamespace)

	return nil
}

func (r *k8sConfigurationRepository) Delete(ctx context.Context, clusterName, clusterNamespace, configMapName, secretName string) error {
	logger := log.FromContext(ctx)

	// Delete ConfigMap
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: clusterNamespace,
		},
	}

	err := r.client.Delete(ctx, configMap)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete configmap %s: %w", configMapName, err)
	}

	if err == nil {
		logger.V(1).Info("deleted configmap", "name", configMapName, "namespace", clusterNamespace)
	}

	// Delete Secret
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: clusterNamespace,
		},
	}

	err = r.client.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secret %s: %w", secretName, err)
	}

	if err == nil {
		logger.V(1).Info("deleted secret", "name", secretName, "namespace", clusterNamespace)
	}

	return nil
}
