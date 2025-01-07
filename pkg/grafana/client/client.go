package client

import (
	"fmt"
	"net/url"

	grafana "github.com/grafana/grafana-openapi-client-go/client"

	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	clientConfigNumRetries = 3
)

func GenerateGrafanaClient(grafanaURL *url.URL, conf config.Config) (*grafana.GrafanaHTTPAPI, error) {
	var err error

	// Generate Grafana client
	// Get grafana admin-password and admin-user
	adminUserCredentials := AdminCredentials{
		Username: conf.Environment.GrafanaAdminUsername,
		Password: conf.Environment.GrafanaAdminPassword,
	}
	if adminUserCredentials.Username == "" {
		return nil, fmt.Errorf("GrafanaAdminUsername not set: %q", conf.Environment.GrafanaAdminUsername)

	}
	if adminUserCredentials.Password == "" {
		return nil, fmt.Errorf("GrafanaAdminPassword not set: %q", conf.Environment.GrafanaAdminPassword)

	}

	grafanaTLSConfig, err := TLSConfig{
		Cert: conf.Environment.GrafanaTLSCertFile,
		Key:  conf.Environment.GrafanaTLSKeyFile,
	}.toTLSConfig()
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
