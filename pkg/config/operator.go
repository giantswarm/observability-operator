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
	AgentCredential     AgentCredentialControllerConfig
	Alertmanager        ControllerConfig
	Cluster             ControllerConfig
	Dashboard           ControllerConfig
	GrafanaOrganization ControllerConfig
	LogExport           ControllerConfig
}

type ControllerConfig struct {
	Enabled bool
}

// AgentCredentialControllerConfig configures the AgentCredential controller.
type AgentCredentialControllerConfig struct {
	Enabled bool
	// IngressSecretEnabled turns on writing the nginx Ingress htpasswd Secret
	// (key "auth") for each backend. The HTTPRoute Secret is always written.
	IngressSecretEnabled bool
}
