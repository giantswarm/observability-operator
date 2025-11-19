package auth

const (
	IngressDataKey   = "auth"
	HTTPRouteDataKey = ".htpasswd"
)

type Config struct {
	// Auth secret (cluster passwords) - can be in different namespace
	AuthSecretName      string
	AuthSecretNamespace string

	// Ingress/HTTPRoute secrets - same namespace
	SecretsNamespace    string
	IngressSecretName   string
	HTTPRouteSecretName string
}

// NewConfig creates a new auth configuration
func NewConfig(authSecretName, authSecretNamespace, secretsNamespace, ingressSecretName, httprouteSecretName string) Config {
	return Config{
		AuthSecretName:      authSecretName,
		AuthSecretNamespace: authSecretNamespace,
		SecretsNamespace:    secretsNamespace,
		IngressSecretName:   ingressSecretName,
		HTTPRouteSecretName: httprouteSecretName,
	}
}
