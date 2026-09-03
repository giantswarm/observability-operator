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
	testAccessKeyID = "AKIAEXAMPLE"
)

func TestSecretEnv(t *testing.T) {
	withCreds := s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:         auditBucketName,
			Region:         s3Region,
			CredentialsRef: &corev1.LocalObjectReference{Name: "archive-credentials"},
		})
	withRole := s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:  "teleport-archive",
			Region:  s3Region,
			RoleARN: "arn:aws:iam::123456789012:role/log-archive-writer",
		})
	creds := map[client.ObjectKey]Credentials{
		{Namespace: "giantswarm", Name: "audit"}: {AccessKeyID: testAccessKeyID, SecretAccessKey: "s3cr3t"},
	}

	t.Run("static credentials become environment", func(t *testing.T) {
		env, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds}, creds)
		if err != nil {
			t.Fatalf("SecretEnv() failed: %v", err)
		}
		want := map[string]string{AccessKeyIDEnv: testAccessKeyID, SecretAccessKeyEnv: "s3cr3t"}
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

	t.Run("two credentialsRef exports are refused", func(t *testing.T) {
		second := withRole
		second.Spec.Destination.S3.CredentialsRef = &corev1.LocalObjectReference{Name: "other-credentials"}
		_, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds, second}, creds)
		if err == nil {
			t.Fatal("SecretEnv() accepted two credentialsRef exports")
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
