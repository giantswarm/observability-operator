package logexporter

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	testAccessKeyID     = "AKIAEXAMPLE"
	testSecretAccessKey = "s3cr3t" //nolint:gosec // G101: a test fixture, not a credential.
)

func TestSecretEnv(t *testing.T) {
	withCreds := s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:         auditBucketName,
			Region:         s3Region,
			CredentialsRef: &corev1.LocalObjectReference{Name: "archive-credentials"},
		})
	withRole := s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:  teleportBucketName,
			Region:  s3Region,
			RoleARN: "arn:aws:iam::123456789012:role/log-archive-writer",
		})
	creds := map[client.ObjectKey]Credentials{
		{Namespace: platformNamespace, Name: auditExportName}: {AccessKeyID: testAccessKeyID, SecretAccessKey: testSecretAccessKey},
	}

	t.Run("static credentials become environment", func(t *testing.T) {
		env, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds}, creds)
		if err != nil {
			t.Fatalf("SecretEnv() failed: %v", err)
		}
		want := map[string]string{AccessKeyIDEnv: testAccessKeyID, SecretAccessKeyEnv: testSecretAccessKey}
		if diff := cmp.Diff(want, env); diff != "" {
			t.Errorf("SecretEnv() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("IRSA needs no environment", func(t *testing.T) {
		env, err := SecretEnv([]observabilityv1alpha1.LogExport{withRole}, nil)
		if err != nil {
			t.Fatalf("SecretEnv() failed: %v", err)
		}
		if len(env) != 0 {
			t.Errorf("SecretEnv() returned %v, expected nothing", env)
		}
	})

	t.Run("static credentials alongside a roleARN export", func(t *testing.T) {
		env, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds, withRole}, creds)
		if err != nil {
			t.Fatalf("SecretEnv() failed: %v", err)
		}
		if env[AccessKeyIDEnv] != testAccessKeyID {
			t.Errorf("SecretEnv() lost the credentials: %v", env)
		}
	})

	// A fresh export rather than a copy of withRole: the two share the S3 pointer, so
	// mutating one would edit the other.
	alsoCredentialed := s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:         teleportBucketName,
			Region:         s3Region,
			CredentialsRef: &corev1.LocalObjectReference{Name: "other-credentials"},
		})
	secondRef := client.ObjectKey{Namespace: "org-fleetio", Name: "teleport"}

	t.Run("two exports may share one set of credentials", func(t *testing.T) {
		same := map[client.ObjectKey]Credentials{
			{Namespace: platformNamespace, Name: auditExportName}: {AccessKeyID: testAccessKeyID, SecretAccessKey: testSecretAccessKey},
			secondRef: {AccessKeyID: testAccessKeyID, SecretAccessKey: testSecretAccessKey},
		}
		env, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds, alsoCredentialed}, same)
		if err != nil {
			t.Fatalf("SecretEnv() rejected two exports with identical credentials: %v", err)
		}
		want := map[string]string{AccessKeyIDEnv: testAccessKeyID, SecretAccessKeyEnv: testSecretAccessKey}
		if diff := cmp.Diff(want, env); diff != "" {
			t.Errorf("SecretEnv() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("two exports with different credentials are refused", func(t *testing.T) {
		differing := map[client.ObjectKey]Credentials{
			{Namespace: platformNamespace, Name: auditExportName}: {AccessKeyID: testAccessKeyID, SecretAccessKey: testSecretAccessKey},
			secondRef: {AccessKeyID: "AKIAOTHER", SecretAccessKey: "other"},
		}
		_, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds, alsoCredentialed}, differing)
		if err == nil {
			t.Fatal("SecretEnv() accepted two exports with different credentials")
		}
		if !strings.Contains(err.Error(), "cannot be set per destination") {
			t.Errorf("SecretEnv() error does not explain the limitation: %v", err)
		}
	})

	t.Run("unresolved credentials are an error", func(t *testing.T) {
		_, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds}, nil)
		if err == nil || !strings.Contains(err.Error(), "no resolved credentials") {
			t.Errorf("SecretEnv() should report unresolved credentials, got: %v", err)
		}
	})
}
