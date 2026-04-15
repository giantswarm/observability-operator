package credential

import "fmt"

// Mode controls how the operator renders AgentCredential Secrets.
type Mode string

const (
	// ModeBasicAuth mints basic-auth credentials (username, password, htpasswd)
	// and aggregates them into the per-backend gateway htpasswd Secrets.
	ModeBasicAuth Mode = "basicAuth"

	// ModeNone produces empty Secrets and writes no htpasswd entries. Use this
	// when authentication is handled at the gateway layer (e.g. workload identity).
	ModeNone Mode = "none"
)

// ValidateMode ensures the given string is a supported Mode.
func ValidateMode(m string) (Mode, error) {
	switch Mode(m) {
	case ModeBasicAuth, ModeNone:
		return Mode(m), nil
	default:
		return "", fmt.Errorf("invalid auth mode %q, must be one of %q, %q", m, ModeBasicAuth, ModeNone)
	}
}
