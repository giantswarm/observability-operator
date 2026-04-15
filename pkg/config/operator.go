package config

// OperatorConfig represents the operator-level configuration.
type OperatorConfig struct {
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	SecureMetrics        bool
	EnableHTTP2          bool
	WebhookCertPath      string
	MetricsCertPath      string
	OperatorNamespace    string
	Controllers          Controllers
}

// Validate validates the operator configuration
func (c OperatorConfig) Validate() error {
	// Add validation logic here if needed
	// For now, operator config is always valid
	return nil
}

type Controllers struct {
	Alertmanager        ControllerConfig
	Cluster             ControllerConfig
	Dashboard           ControllerConfig
	GrafanaOrganization ControllerConfig
}

type ControllerConfig struct {
	Enabled bool
}
