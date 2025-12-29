package auth

// AuthType represents the type of observability authentication
type AuthType string

const (
	// Auth types
	AuthTypeMetrics         AuthType = "metrics"
	AuthTypeLogs            AuthType = "logs"
	AuthTypeTraces          AuthType = "traces"
	AuthTypeVictoriaMetrics AuthType = "victoriametrics"

	// Gateway secret data keys (supporting migration from Ingress to Gateway API)
	IngressDataKey   = "auth"
	HTTPRouteDataKey = ".htpasswd"
)

type Config struct {
	// Auth type (metrics, logs, traces)
	AuthType AuthType

	// Gateway secret configuration
	GatewaySecrets GatewaySecretsConfig
}

// GatewaySecretsConfig contains configuration for gateway authentication secrets
type GatewaySecretsConfig struct {
	// Namespace where gateway secrets are created
	Namespace string

	// Ingress secret name and data key (legacy)
	IngressSecretName string
	IngressDataKey    string

	// HTTPRoute secret name and data key (Gateway API)
	HTTPRouteSecretName string
	HTTPRouteDataKey    string
}

// NewConfig creates a new auth configuration
func NewConfig(authType AuthType, gatewaySecretsNamespace, ingressSecretName, httprouteSecretName string) Config {
	return Config{
		AuthType: authType,
		GatewaySecrets: GatewaySecretsConfig{
			Namespace: gatewaySecretsNamespace,

			// Ingress configuration (legacy)
			IngressSecretName: ingressSecretName,
			IngressDataKey:    IngressDataKey,

			// HTTPRoute configuration (Gateway API)
			HTTPRouteSecretName: httprouteSecretName,
			HTTPRouteDataKey:    HTTPRouteDataKey,
		},
	}
}
