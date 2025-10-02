package config

import (
	"fmt"
	"net/url"
)

// GrafanaConfig represents the Grafana-specific configuration.
type GrafanaConfig struct {
	URL *url.URL
}

// Validate validates the Grafana configuration
func (c GrafanaConfig) Validate() error {
	if c.URL == nil {
		return fmt.Errorf("grafana URL is required")
	}
	return nil
}
