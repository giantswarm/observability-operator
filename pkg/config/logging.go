package config

// LoggingConfig represents the configuration used by the logging package.
type LoggingConfig struct {
	Enabled bool
}

// Validate validates the logging configuration
func (c LoggingConfig) Validate() error {
	// Logging config is always valid since it's just a boolean flag
	return nil
}
