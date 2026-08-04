package logexport

import (
	"fmt"
	"os"
)

// Environment variable names holding the object-store credentials for the export
// destination. They are read from the operator's own environment rather than from
// the observability credential store, because these are the customer's bucket
// credentials, not an observability backend's.
const (
	//nolint:gosec // G101: environment variable names, not credentials
	envAccessKeyID = "LOGEXPORT_AWS_ACCESS_KEY_ID"
	//nolint:gosec // G101: environment variable names, not credentials
	envSecretAccessKey = "LOGEXPORT_AWS_SECRET_ACCESS_KEY"
)

// Keys in the generated Secret. Alloy's chart maps these into the container as
// environment variables of the same name via extraSecretEnv, and the AWS SDK picks
// them up from there.
const (
	//nolint:gosec // G101: Secret key names, not credentials
	accessKeyIDKey = "AWS_ACCESS_KEY_ID"
	//nolint:gosec // G101: Secret key names, not credentials
	secretAccessKeyKey = "AWS_SECRET_ACCESS_KEY"
)

// GenerateAlloyLogExportSecretData returns the object-store credentials for the
// export destination.
//
// Both values are required: the exporter cannot write anywhere without them, and a
// missing bucket or a rejected credential fails in a way that is only visible in
// otelcol_exporter_send_failed_log_records_total, so it is better to refuse to
// configure the app than to deploy something that silently drops everything.
func (s *Service) GenerateAlloyLogExportSecretData() (map[string]string, error) {
	accessKeyID := os.Getenv(envAccessKeyID)
	secretAccessKey := os.Getenv(envSecretAccessKey)

	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf(
			"log export is enabled but object store credentials are missing: both %s and %s must be set",
			envAccessKeyID, envSecretAccessKey)
	}

	return map[string]string{
		accessKeyIDKey:     accessKeyID,
		secretAccessKeyKey: secretAccessKey,
	}, nil
}
