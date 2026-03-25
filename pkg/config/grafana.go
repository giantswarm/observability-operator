package config

import (
	"fmt"
	"net/url"
)

// GrafanaConfig represents the Grafana-specific configuration.
type GrafanaConfig struct {
	URL         *url.URL
	Datasources DatasourcesConfig

	// ClientRetries is the number of retries for Grafana HTTP API calls.
	ClientRetries int
	// AdminSecretNamespace is the Kubernetes namespace of the Grafana admin credentials secret.
	AdminSecretNamespace string
	// AdminSecretName is the name of the Kubernetes secret holding Grafana admin credentials.
	AdminSecretName string
	// GatewayTLSSecretNamespace is the namespace of the gateway TLS secret used for Grafana client mTLS.
	GatewayTLSSecretNamespace string
	// GatewayTLSSecretName is the name of the gateway TLS secret used for Grafana client mTLS.
	GatewayTLSSecretName string
}

// DatasourcesConfig holds the service URLs for the Grafana datasources provisioned by the operator.
// These default to the standard in-cluster svc DNS names used by a GiantSwarm stack.
type DatasourcesConfig struct {
	// LokiURL is the URL of the Loki gateway service.
	LokiURL string
	// MimirURL is the URL of the Mimir query-frontend / gateway (Prometheus-compatible endpoint).
	MimirURL string
	// MimirAlertmanagerURL is the URL of the Mimir Alertmanager service.
	MimirAlertmanagerURL string
	// MimirCardinalityURL is the URL of the Mimir cardinality API (used for the JSON datasource).
	MimirCardinalityURL string
	// TempoURL is the URL of the Tempo query-frontend service.
	TempoURL string
}

// Validate validates the Grafana configuration
func (c GrafanaConfig) Validate() error {
	if c.URL == nil {
		return fmt.Errorf("grafana URL is required")
	}
	return nil
}
