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

	// Environment and runtime settings
	Environment EnvironmentConfig
}

// EnvironmentConfig represents environment-specific configuration.
type EnvironmentConfig struct {
	OpsgenieApiKey                 string `env:"OPSGENIE_API_KEY"`
	CronitorHeartbeatManagementKey string `env:"CRONITOR_HEARTBEAT_MANAGEMENT_KEY"`
	CronitorHeartbeatPingKey       string `env:"CRONITOR_HEARTBEAT_PING_KEY"`
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
