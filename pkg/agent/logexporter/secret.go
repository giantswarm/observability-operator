package logexporter

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	// Environment variable names for static S3 credentials. otelcol.exporter.awss3 has no
	// credential arguments at all -- it reads the AWS SDK's default chain -- so these are
	// the SDK's names rather than a choice of ours.
	//
	// The keys a customer has to use inside their credentialsRef Secret are the same two
	// names, but that contract belongs to whoever reads the Secret: it is documented on
	// LogExport.spec.destination.s3.credentialsRef and published in the CRD schema, and
	// spelled out separately as logExportAccessKeyIDKey in the controller. They coincide
	// today only because the SDK picked the obvious names.
	AccessKeyIDEnv     = "AWS_ACCESS_KEY_ID"
	SecretAccessKeyEnv = "AWS_SECRET_ACCESS_KEY" //nolint:gosec // G101: an environment variable name, not a credential.
)

// Credentials are the resolved contents of a destination's credentialsRef Secret.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// SecretEnv returns the environment the exporter needs for static credentials, keyed by
// variable name, ready for common.GenerateSecretData.
//
// The awss3 exporter has no credential fields: it uses the AWS SDK's default chain, which
// reads the process environment. Environment variables are per-container, not per
// exporter, so only one export can carry static credentials. Additional destinations have
// to authenticate by workload identity with roleARN, which is per exporter.
func SecretEnv(exports []observabilityv1alpha1.LogExport, creds map[client.ObjectKey]Credentials) (map[string]string, error) {
	env := map[string]string{}
	var credentialed string

	for _, e := range sorted(exports) {
		if e.Spec.Destination.S3 == nil || e.Spec.Destination.S3.CredentialsRef == nil {
			continue
		}
		ref := client.ObjectKey{Namespace: e.Namespace, Name: e.Name}
		if credentialed != "" {
			return nil, fmt.Errorf("%s and %s both set spec.destination.s3.credentialsRef, but static credentials reach the exporter as environment variables and cannot be set per destination: use roleARN on all but one", credentialed, ref)
		}
		credentialed = ref.String()

		c, ok := creds[ref]
		if !ok {
			return nil, fmt.Errorf("no resolved credentials for %s", ref)
		}
		env[AccessKeyIDEnv] = c.AccessKeyID
		env[SecretAccessKeyEnv] = c.SecretAccessKey
	}

	return env, nil
}
