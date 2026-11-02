package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	// Ingress TLS secret (legacy - replaced by GatewayTLSSecretName) - TODO: remove once ingress is gone
	grafanaLegacyTLSSecretName = "grafana-tls" // nolint:gosec // G101: Secret resource name, not a credential

	grafanaAdminSecretUserKey     = "admin-user"
	grafanaAdminSecretPasswordKey = "admin-password"

	grafanaTLSSecretCertKey = "tls.crt"
	grafanaTLSSecretKeyKey  = "tls.key"
)

// GrafanaClientGenerator defines the interface for generating Grafana clients
type GrafanaClientGenerator interface {
	GenerateGrafanaClient(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (GrafanaClient, error)
}

// DefaultGrafanaClientGenerator is the default implementation
type DefaultGrafanaClientGenerator struct {
	cfg config.GrafanaConfig
}

// NewDefaultGrafanaClientGenerator creates a DefaultGrafanaClientGenerator with the given config.
func NewDefaultGrafanaClientGenerator(cfg config.GrafanaConfig) *DefaultGrafanaClientGenerator {
	return &DefaultGrafanaClientGenerator{cfg: cfg}
}

// GenerateGrafanaClient creates a new Grafana client by fetching credentials
// and, for https URLs, an optional client TLS certificate from Kubernetes secrets.
func (g *DefaultGrafanaClientGenerator) GenerateGrafanaClient(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (GrafanaClient, error) {
	adminSecretNamespace := g.cfg.AdminSecretNamespace
	adminSecretName := g.cfg.AdminSecretName

	// Get Grafana admin credentials from secret
	adminSecret := &corev1.Secret{}
	adminSecretKey := client.ObjectKey{Namespace: adminSecretNamespace, Name: adminSecretName}
	if err := k8sClient.Get(ctx, adminSecretKey, adminSecret); err != nil {
		return nil, fmt.Errorf("failed to get Grafana admin secret %q in namespace %q: %w", adminSecretName, adminSecretNamespace, err)
	}

	adminUsernameBytes, ok := adminSecret.Data[grafanaAdminSecretUserKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in Grafana admin secret %q", grafanaAdminSecretUserKey, adminSecretName)
	}
	if len(adminUsernameBytes) == 0 {
		return nil, fmt.Errorf("GrafanaAdminUsername is empty in secret %q", adminSecretName)
	}

	adminPasswordBytes, ok := adminSecret.Data[grafanaAdminSecretPasswordKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in Grafana admin secret %q", grafanaAdminSecretPasswordKey, adminSecretName)
	}
	if len(adminPasswordBytes) == 0 {
		return nil, fmt.Errorf("GrafanaAdminPassword is empty in secret %q", adminSecretName)
	}

	clientTLSConfig, err := g.clientTLSConfig(ctx, k8sClient, grafanaURL)
	if err != nil {
		return nil, err
	}

	transportCfg := &grafana.TransportConfig{
		Schemes:  []string{grafanaURL.Scheme},
		BasePath: "/api",
		// Initialize the client with the first organization.
		// This enforces requests to be made in the context of the
		// first org, if not specified otherwise via the WithOrgID() method.
		// This overrides the server side defined org context,
		// see https://grafana.com/docs/grafana/latest/developers/http_api/user/#switch-user-context-for-signed-in-user
		// This ensures operations like deleting organizations other than the first org. work as expected.
		OrgID: 1,
		Host:  grafanaURL.Host,
		// TODO using a serviceaccount later would be better as they are scoped to an organization
		BasicAuth:  url.UserPassword(string(adminUsernameBytes), string(adminPasswordBytes)),
		NumRetries: g.cfg.ClientRetries,
		TLSConfig:  clientTLSConfig,
	}

	return NewGrafanaClient(grafana.NewHTTPClientWithConfig(nil, transportCfg)), nil
}

// clientTLSConfig returns the TLS client configuration for the Grafana client.
// The client certificate is optional: it returns nil (no client certificate,
// default TLS settings) when the Grafana URL uses plain http or when no TLS
// Secret is found. The Gateway TLS Secret is looked up first when its name is
// configured, then the legacy ingress Secret in the admin Secret's namespace.
// A Secret that is found but malformed is an error.
func (g *DefaultGrafanaClientGenerator) clientTLSConfig(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (*tls.Config, error) {
	logger := log.FromContext(ctx).V(1)

	if grafanaURL.Scheme == "http" {
		logger.Info("skipping Grafana client certificate", "reason", "Grafana URL uses http")
		return nil, nil
	}

	tlsSecret, err := g.findTLSSecret(ctx, k8sClient, logger)
	if err != nil {
		return nil, err
	}
	if tlsSecret == nil {
		logger.Info("skipping Grafana client certificate", "reason", "no TLS secret found",
			"gatewaySecretNamespace", g.cfg.GatewayTLSSecretNamespace, "gatewaySecretName", g.cfg.GatewayTLSSecretName,
			"legacySecretNamespace", g.cfg.AdminSecretNamespace, "legacySecretName", grafanaLegacyTLSSecretName)
		return nil, nil
	}

	tlsCertBytes, certOk := tlsSecret.Data[grafanaTLSSecretCertKey]
	tlsKeyBytes, keyOk := tlsSecret.Data[grafanaTLSSecretKeyKey]
	if !certOk || len(tlsCertBytes) == 0 {
		return nil, fmt.Errorf("key %q not found or empty in Grafana TLS secret %q", grafanaTLSSecretCertKey, tlsSecret.Name)
	}
	if !keyOk || len(tlsKeyBytes) == 0 {
		return nil, fmt.Errorf("key %q not found or empty in Grafana TLS secret %q", grafanaTLSSecretKeyKey, tlsSecret.Name)
	}

	tlsConfigInput := TLSConfig{
		Cert: string(tlsCertBytes),
		Key:  string(tlsKeyBytes),
	}
	clientTLSConfig, err := tlsConfigInput.toTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config from secret %q: %w", tlsSecret.Name, err)
	}

	logger.Info("loaded Grafana client certificate", "secretNamespace", tlsSecret.Namespace, "secretName", tlsSecret.Name)
	return clientTLSConfig, nil
}

// findTLSSecret looks up the Grafana client TLS Secret: the Gateway TLS Secret
// when its name is configured, then the legacy ingress Secret. It returns nil
// without error when neither exists. Errors other than NotFound are returned.
func (g *DefaultGrafanaClientGenerator) findTLSSecret(ctx context.Context, k8sClient client.Client, logger logr.Logger) (*corev1.Secret, error) {
	candidates := make([]client.ObjectKey, 0, 2)
	if g.cfg.GatewayTLSSecretName != "" {
		candidates = append(candidates, client.ObjectKey{Namespace: g.cfg.GatewayTLSSecretNamespace, Name: g.cfg.GatewayTLSSecretName})
	} else {
		logger.Info("no Grafana gateway TLS secret configured, trying the legacy TLS secret")
	}
	candidates = append(candidates, client.ObjectKey{Namespace: g.cfg.AdminSecretNamespace, Name: grafanaLegacyTLSSecretName})

	for _, key := range candidates {
		secret := &corev1.Secret{}
		err := k8sClient.Get(ctx, key, secret)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get Grafana TLS secret %q in namespace %q: %w", key.Name, key.Namespace, err)
		}
		return secret, nil
	}
	return nil, nil
}
