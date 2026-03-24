package config

import (
	"fmt"
)

// Config represents the main configuration for the observability operator.
type Config struct {
	// Operator-level configuration
	Operator OperatorConfig

	// Subsystem configurations
	Logging    LoggingConfig
	Grafana    GrafanaConfig
	Monitoring MonitoringConfig
	Tracing    TracingConfig

	// Management cluster configuration
	Cluster ClusterConfig

	// Environment and runtime settings (secrets from environment variables)
	Environment EnvironmentConfig

	// Cronitor heartbeat monitor operational settings
	Cronitor CronitorConfig
}

// CronitorConfig holds operational settings for the Cronitor heartbeat monitor.
// The Cronitor API keys are in EnvironmentConfig (sourced from env vars).
type CronitorConfig struct {
	// GraceSeconds is the number of seconds after a missed heartbeat before an alert is triggered.
	GraceSeconds int
	// Schedule is the expected heartbeat frequency (e.g. "every 30 minutes").
	Schedule string
	// RealertInterval controls how often Cronitor re-alerts if the issue persists (e.g. "every 24 hours").
	RealertInterval string
}

// EnvironmentConfig represents environment-specific configuration.
type EnvironmentConfig struct {
	CronitorHeartbeatManagementKey string `env:"CRONITOR_HEARTBEAT_MANAGEMENT_KEY"`
	CronitorHeartbeatPingKey       string `env:"CRONITOR_HEARTBEAT_PING_KEY"`
}

// GatewayConfig holds the namespace and secret names for gateway authentication secrets.
// These secrets are read by Alloy agents on workload clusters to authenticate with the
// observability gateways (Mimir, Loki, Tempo).
type GatewayConfig struct {
	// Namespace is the Kubernetes namespace where the gateway secrets reside.
	Namespace string
	// IngressSecretName is the name of the secret used for Ingress-based auth.
	IngressSecretName string
	// HTTPRouteSecretName is the name of the secret used for HTTPRoute-based auth.
	HTTPRouteSecretName string
}

// Validate validates the entire configuration.
func (c Config) Validate() error {
	if err := c.Operator.Validate(); err != nil {
		return fmt.Errorf("operator config validation failed: %w", err)
	}
	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config validation failed: %w", err)
	}
	if err := c.Grafana.Validate(); err != nil {
		return fmt.Errorf("grafana config validation failed: %w", err)
	}
	if err := c.Tracing.Validate(); err != nil {
		return fmt.Errorf("tracing config validation failed: %w", err)
	}
	if err := c.Monitoring.Validate(); err != nil {
		return fmt.Errorf("monitoring config validation failed: %w", err)
	}
	if err := c.Cluster.Validate(); err != nil {
		return fmt.Errorf("cluster config validation failed: %w", err)
	}
	return nil
}
