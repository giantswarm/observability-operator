package credential

import (
	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	// HTTPRouteDataKey is the data key of the gateway secret consumed by the
	// Gateway API HTTPRoute.
	HTTPRouteDataKey = ".htpasswd"

	// SecretKeyUsername, SecretKeyPassword and SecretKeyHtpasswd are the keys
	// written into the rendered per-credential Secret. Username/password match
	// the required keys of the kubernetes.io/basic-auth Secret type.
	SecretKeyUsername = "username"
	SecretKeyPassword = "password"
	SecretKeyHtpasswd = "htpasswd"
)

// GatewayConfig holds the namespace and secret name for a single backend's gateway
// authentication secret.
type GatewayConfig struct {
	// Namespace is the Kubernetes namespace where the gateway secret resides.
	Namespace string

	// HTTPRouteSecretName is the name of the auth secret consumed by the
	// Gateway API HTTPRoute.
	HTTPRouteSecretName string
	// HTTPRouteDataKey is the data key within the HTTPRoute secret.
	HTTPRouteDataKey string
}

// NewGatewayConfig builds a GatewayConfig with the standard data keys.
func NewGatewayConfig(namespace, httprouteSecretName string) GatewayConfig {
	return GatewayConfig{
		Namespace:           namespace,
		HTTPRouteSecretName: httprouteSecretName,
		HTTPRouteDataKey:    HTTPRouteDataKey,
	}
}

// GatewayConfigs maps each backend to its gateway secret configuration.
type GatewayConfigs map[observabilityv1alpha1.CredentialBackend]GatewayConfig
