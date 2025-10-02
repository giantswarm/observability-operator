package config

// TracingConfig represents the configuration for tracing support in Grafana.
type TracingConfig struct {
	Enabled bool
}

// Validate validates the tracing configuration
func (c TracingConfig) Validate() error {
	// Tracing config is always valid since it's just a boolean flag
	return nil
}
