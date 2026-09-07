package logexporter

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	// Environment variable names for static S3 credentials. otelcol.exporter.awss3 has no
	// credential arguments at all -- it reads the AWS SDK's default chain -- so these are
	// the names the SDK defined, not a choice of ours.
	//
	// They double as the keys a credentialsRef Secret has to carry, which is documented on
	// LogExport.spec.destination.s3.credentialsRef and published in the CRD schema.
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
	var resolved Credentials

	for _, e := range sorted(exports) {
		if e.Spec.Destination.S3 == nil || e.Spec.Destination.S3.CredentialsRef == nil {
			continue
		}
		ref := client.ObjectKey{Namespace: e.Namespace, Name: e.Name}

		c, ok := creds[ref]
		if !ok {
			return nil, fmt.Errorf("no resolved credentials for %s", ref)
		}

		// Several exports may carry credentials as long as they are the same ones, which
		// is the normal shape for two destinations in one account. Only a genuine
		// disagreement is unrenderable.
		if credentialed != "" && c != resolved {
			return nil, fmt.Errorf("LogExports %s and %s set spec.destination.s3.credentialsRef to different credentials, but static credentials reach the exporter as environment variables and cannot be set per destination: use the same credentials, or roleARN on all but one", credentialed, ref)
		}
		credentialed, resolved = ref.String(), c

		env[AccessKeyIDEnv] = c.AccessKeyID
		env[SecretAccessKeyEnv] = c.SecretAccessKey
	}

	return env, nil
}
