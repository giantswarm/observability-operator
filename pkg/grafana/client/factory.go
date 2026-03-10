package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"

	grafana "github.com/grafana/grafana-openapi-client-go/client"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clientConfigNumRetries = 3

	// Secret names and keys for Grafana configuration
	grafanaNamespace = "monitoring"

	grafanaAdminSecretName        = "grafana"
	grafanaAdminSecretUserKey     = "admin-user"
	grafanaAdminSecretPasswordKey = "admin-password"

	// Ingress TLS secret (legacy - replaced by gatewayTLSSecretName) - TODO: remove once ingress is gone
	grafanaTLSSecretName = "grafana-tls"

	// Gateway API TLS secret: lives in the envoy-gateway-system namespace
	gatewayTLSSecretNamespace = "envoy-gateway-system"
	gatewayTLSSecretName      = "gateway-giantswarm-default-https-tls"

	grafanaTLSSecretCertKey = "tls.crt"
	grafanaTLSSecretKeyKey  = "tls.key"
)

// GrafanaClientGenerator defines the interface for generating Grafana clients
type GrafanaClientGenerator interface {
	GenerateGrafanaClient(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (GrafanaClient, error)
}

// DefaultGrafanaClientGenerator is the default implementation
type DefaultGrafanaClientGenerator struct{}

// GenerateGrafanaClient creates a new Grafana client by fetching credentials
// and TLS configuration from Kubernetes secrets.
func (g *DefaultGrafanaClientGenerator) GenerateGrafanaClient(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (GrafanaClient, error) {
	// Get Grafana admin credentials from secret
	adminSecret := &corev1.Secret{}
	adminSecretKey := client.ObjectKey{Namespace: grafanaNamespace, Name: grafanaAdminSecretName}
	if err := k8sClient.Get(ctx, adminSecretKey, adminSecret); err != nil {
		return nil, fmt.Errorf("failed to get Grafana admin secret %q in namespace %q: %w", grafanaAdminSecretName, grafanaNamespace, err)
	}

	adminUsernameBytes, ok := adminSecret.Data[grafanaAdminSecretUserKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in Grafana admin secret %q", grafanaAdminSecretUserKey, grafanaAdminSecretName)
	}
	if len(adminUsernameBytes) == 0 {
		return nil, fmt.Errorf("GrafanaAdminUsername is empty in secret %q", grafanaAdminSecretName)
	}

	adminPasswordBytes, ok := adminSecret.Data[grafanaAdminSecretPasswordKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in Grafana admin secret %q", grafanaAdminSecretPasswordKey, grafanaAdminSecretName)
	}
	if len(adminPasswordBytes) == 0 {
		return nil, fmt.Errorf("GrafanaAdminPassword is empty in secret %q", grafanaAdminSecretName)
	}

	// Get Grafana TLS configuration — try the Gateway API secret first,
	// fall back to the legacy ingress secret if not found.
	var clientTLSConfig *tls.Config
	tlsSecret := &corev1.Secret{}
	tlsSecretNamespace := gatewayTLSSecretNamespace
	tlsSecretName := gatewayTLSSecretName
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: gatewayTLSSecretNamespace, Name: gatewayTLSSecretName}, tlsSecret)
	if apierrors.IsNotFound(err) {
		tlsSecretNamespace = grafanaNamespace
		tlsSecretName = grafanaTLSSecretName
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: grafanaNamespace, Name: grafanaTLSSecretName}, tlsSecret)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Grafana TLS secret %q in namespace %q: %w", tlsSecretName, tlsSecretNamespace, err)
	}

	// TLS Secret found, try to load cert and key
	tlsCertBytes, certOk := tlsSecret.Data[grafanaTLSSecretCertKey]
	tlsKeyBytes, keyOk := tlsSecret.Data[grafanaTLSSecretKeyKey]

	if !certOk || len(tlsCertBytes) == 0 {
		return nil, fmt.Errorf("key %q not found or empty in Grafana TLS secret %q", grafanaTLSSecretCertKey, tlsSecretName)
	}
	if !keyOk || len(tlsKeyBytes) == 0 {
		return nil, fmt.Errorf("key %q not found or empty in Grafana TLS secret %q", grafanaTLSSecretKeyKey, tlsSecretName)
	}

	// tlsConfigInput is our local struct type from ./tls.go
	tlsConfigInput := TLSConfig{
		Cert: string(tlsCertBytes),
		Key:  string(tlsKeyBytes),
	}
	// clientTLSConfig is the *tls.Config for the Grafana client
	clientTLSConfig, err = tlsConfigInput.toTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config from secret %q: %w", tlsSecretName, err)
	}

	cfg := &grafana.TransportConfig{
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
		NumRetries: clientConfigNumRetries,
		TLSConfig:  clientTLSConfig,
	}

	return NewGrafanaClient(grafana.NewHTTPClientWithConfig(nil, cfg)), nil
}
