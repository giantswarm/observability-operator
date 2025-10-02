package config

// OperatorConfig represents the operator-level configuration.
type OperatorConfig struct {
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	SecureMetrics        bool
	EnableHTTP2          bool
	WebhookCertPath      string
	OperatorNamespace    string
}

// Validate validates the operator configuration
func (c OperatorConfig) Validate() error {
	// Add validation logic here if needed
	// For now, operator config is always valid
	return nil
}
