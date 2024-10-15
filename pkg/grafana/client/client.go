package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	grafanaURL                        = "http://grafana.%s.svc.cluster.local:3000"
	grafanaNamespace                  = "monitoring"
	grafanaAdminCredentialsSecretName = "grafana"
	grafanaTLSSecretName              = "grafana-tls" // nolint:gosec
)

func GenerateGrafanaClient(ctx context.Context, client client.Client, logger logr.Logger) (*grafana.GrafanaHTTPAPI, error) {
	// Get grafana admin-password and admin-user
	adminCredentials, err := getAdminCredentials(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch grafana admin secret: %w", err)
	}

	tlsConfig, err := buildTLSConfiguration(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to build tls config: %w", err)
	}

	grafanaUrl, err := url.Parse(fmt.Sprintf(grafanaURL, namespace))
	if err != nil {
		return nil, fmt.Errorf("parsing url for client: %w", err)
	}

	cfg := &grafana.TransportConfig{
		Schemes:  []string{grafanaUrl.Scheme},
		BasePath: "/api",
		Host:     grafanaUrl.Host,
		// We use basic auth to authenticate on grafana.
		BasicAuth: url.UserPassword(adminCredentials.Username, adminCredentials.Password),
		// NumRetries contains the optional number of attempted retries.
		NumRetries: 0,
		TLSConfig:  tlsConfig,
	}

	return grafana.NewHTTPClientWithConfig(nil, cfg), nil
}
