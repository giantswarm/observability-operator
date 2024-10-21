package client

import (
	"fmt"
	"net/url"

	grafana "github.com/grafana/grafana-openapi-client-go/client"
)

var grafanaURL *url.URL

func init() {
	var err error
	grafanaURL, err = url.Parse("http://grafana.monitoring.svc.cluster.local")
	if err != nil {
		panic(fmt.Sprintf("failed to parse grafana url: %v", err))
	}
}

const (
	clientConfigNumRetries = 3
)

func GenerateGrafanaClient(adminUserCredentials AdminCredentials, tlsConfig TLSConfig) (*grafana.GrafanaHTTPAPI, error) {
	grafanaTLSConfig, err := tlsConfig.toTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build tls config: %w", err)
	}

	cfg := &grafana.TransportConfig{
		Schemes:  []string{grafanaURL.Scheme},
		BasePath: "/api",
		Host:     grafanaURL.Host,
		// We use basic auth to authenticate on grafana.
		BasicAuth: url.UserPassword(adminUserCredentials.Username, adminUserCredentials.Password),
		// NumRetries contains the optional number of attempted retries.
		NumRetries: clientConfigNumRetries,
		TLSConfig:  grafanaTLSConfig,
	}

	return grafana.NewHTTPClientWithConfig(nil, cfg), nil
}
