package logexporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// s3Export builds a LogExport with an S3 destination, for overriding per test.
func s3Export(namespace, name, selector string, s3 observabilityv1alpha1.S3Destination) observabilityv1alpha1.LogExport {
	return observabilityv1alpha1.LogExport{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: observabilityv1alpha1.LogExportSpec{
			Selector: selector,
			Destination: observabilityv1alpha1.LogExportDestination{
				Type: observabilityv1alpha1.LogExportDestinationS3,
				S3:   &s3,
			},
		},
	}
}

var auditBucket = observabilityv1alpha1.S3Destination{
	Bucket: "audit-export",
	Region: "eu-west-2",
}

func TestRenderValues(t *testing.T) {
	tests := []struct {
		name       string
		exports    []observabilityv1alpha1.LogExport
		goldenPath string
	}{
		{
			name:       "minimal selector",
			exports:    []observabilityv1alpha1.LogExport{s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`, auditBucket)},
			goldenPath: "alloy-logexporter-config.minimal.yaml",
		},
		{
			name: "parse and filter",
			exports: []observabilityv1alpha1.LogExport{s3Export("giantswarm", "audit",
				`{scrape_job="audit-logs"} | json | verb="delete" | user=~"admin.*"`, auditBucket)},
			goldenPath: "alloy-logexporter-config.parse-and-filter.yaml",
		},
		{
			name: "s3 options",
			exports: []observabilityv1alpha1.LogExport{s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`,
				observabilityv1alpha1.S3Destination{
					Bucket:         "audit-export",
					Region:         "us-east-1",
					Prefix:         "giantswarm/audit",
					Endpoint:       "http://minio.spike03.svc.cluster.local:9000",
					ForcePathStyle: true,
					RoleARN:        "arn:aws:iam::123456789012:role/log-archive-writer",
				})},
			goldenPath: "alloy-logexporter-config.s3-options.yaml",
		},
		{
			name: "two exports, two destinations",
			exports: []observabilityv1alpha1.LogExport{
				// Deliberately out of order: rendering sorts, so the output is stable.
				s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`,
					observabilityv1alpha1.S3Destination{Bucket: "teleport-archive", Region: "eu-west-2"}),
				s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`, auditBucket),
			},
			goldenPath: "alloy-logexporter-config.two-exports.yaml",
		},
		{
			name: "credentialsRef does not change the values document",
			exports: []observabilityv1alpha1.LogExport{s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`,
				observabilityv1alpha1.S3Destination{
					Bucket:         "audit-export",
					Region:         "eu-west-2",
					CredentialsRef: &corev1.LocalObjectReference{Name: "archive-credentials"},
				})},
			goldenPath: "alloy-logexporter-config.minimal.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderValues(tt.exports)
			if err != nil {
				t.Fatalf("RenderValues() failed: %v", err)
			}

			golden := filepath.Join("testdata", tt.goldenPath)
			if os.Getenv("UPDATE_GOLDEN_FILES") == "true" {
				if err := os.MkdirAll(filepath.Dir(golden), 0750); err != nil {
					t.Fatalf("failed to create golden directory: %v", err)
				}
				//nolint:gosec
				if err := os.WriteFile(golden, []byte(result), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v", golden, err)
			}
			if diff := cmp.Diff(string(expected), result); diff != "" {
				t.Errorf("RenderValues() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRenderValuesAssertsSilentFailures pins the settings from 01-findings.md that fail
// silently when dropped: without them the export either misses data or costs a fortune,
// and nothing in the pipeline reports it.
func TestRenderValuesAssertsSilentFailures(t *testing.T) {
	result, err := RenderValues([]observabilityv1alpha1.LogExport{
		s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`, auditBucket),
	})
	if err != nil {
		t.Fatalf("RenderValues() failed: %v", err)
	}

	required := map[string]string{
		`action              = "drop"`:                         "selection has to be a drop of the negated selector; a keep exports every line in the installation",
		`selector            = "{scrape_job!=\"audit-logs\"}"`: "the drop selector has to be the negation of the customer's",
		"batch {":                       "without a batch block every log line becomes its own S3 object",
		`sizer             = "items"`:   "the queue counts requests by default, which is under a second of buffer",
		"block_on_overflow = true":      "overflow has to be backpressure, not a silent drop",
		`timeout = "5m"`:                "the 5s default makes any outage over 5s permanent silent loss",
		`s3_partition_timezone = "UTC"`: "partitions otherwise shift with the pod's timezone",
		"gs_cluster_id":                 "audit events carry no cluster identifier and body drops the Loki labels",
		"stabilityLevel: experimental":  "Alloy refuses to load an awss3 exporter without it",
	}
	for snippet, why := range required {
		if !strings.Contains(result, snippet) {
			t.Errorf("rendered values are missing %q: %s", snippet, why)
		}
	}

	// resource_attrs_to_s3 files records under the wrong cluster when batching is on, and
	// batching is mandatory.
	if strings.Contains(result, "resource_attrs_to_s3") {
		t.Error("rendered values use resource_attrs_to_s3, which misattributes records once batched")
	}
}

// TestRenderValuesIsValidYAML checks the two-stage nesting: the river config is rendered
// first and then embedded as a string in the values document, so an indentation slip
// produces a values file Flux cannot parse.
func TestRenderValuesIsValidYAML(t *testing.T) {
	values, err := RenderValues([]observabilityv1alpha1.LogExport{
		s3Export("giantswarm", "audit", `{scrape_job="audit-logs"} | json | verb="delete"`, auditBucket),
		s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`, auditBucket),
	})
	if err != nil {
		t.Fatalf("RenderValues() failed: %v", err)
	}

	var doc struct {
		Alloy struct {
			Enabled bool `yaml:"enabled"`
			Alloy   struct {
				ConfigMap struct {
					Content string `yaml:"content"`
				} `yaml:"configMap"`
			} `yaml:"alloy"`
		} `yaml:"alloy"`
	}
	if err := yaml.Unmarshal([]byte(values), &doc); err != nil {
		t.Fatalf("rendered values are not valid YAML: %v", err)
	}

	// Without this the app stays inert: the collections defaults ship enabled: false.
	if !doc.Alloy.Enabled {
		t.Error("rendered values do not set alloy.enabled: true")
	}
	// The river has to survive the round trip through the YAML string.
	config := doc.Alloy.Alloy.ConfigMap.Content
	for _, want := range []string{
		`loki.source.api "mirror"`,
		`loki.process "select_giantswarm_audit"`,
		`loki.process "select_org_fleetio_teleport"`,
		`otelcol.exporter.awss3 "giantswarm_audit"`,
		`otelcol.exporter.awss3 "org_fleetio_teleport"`,
		`otelcol.storage.file "wal"`,
	} {
		if !strings.Contains(config, want) {
			t.Errorf("embedded Alloy config is missing %q", want)
		}
	}
}

func TestRenderValuesErrors(t *testing.T) {
	tests := []struct {
		name    string
		exports []observabilityv1alpha1.LogExport
		wantErr string
	}{
		{
			name:    "no exports",
			exports: nil,
			wantErr: "no LogExport to render",
		},
		{
			name: "loki destination",
			exports: []observabilityv1alpha1.LogExport{{
				ObjectMeta: metav1.ObjectMeta{Name: "to-loki", Namespace: "giantswarm"},
				Spec: observabilityv1alpha1.LogExportSpec{
					Selector: `{scrape_job="audit-logs"}`,
					Destination: observabilityv1alpha1.LogExportDestination{
						Type: observabilityv1alpha1.LogExportDestinationLoki,
						Loki: &observabilityv1alpha1.LokiDestination{URL: "https://logs.example.com/loki/api/v1/push"},
					},
				},
			}},
			wantErr: `destination type "loki" is not implemented yet`,
		},
		{
			name: "component name collision",
			exports: []observabilityv1alpha1.LogExport{
				s3Export("a-b", "c", `{scrape_job="audit-logs"}`, auditBucket),
				s3Export("a", "b-c", `{scrape_job="audit-logs"}`, auditBucket),
			},
			wantErr: "both map to the Alloy component name",
		},
		{
			// One case is enough: the selector subset is validation's to police, and
			// selector_test.go covers the translation.
			name: "selector rejected by validation",
			exports: []observabilityv1alpha1.LogExport{s3Export("giantswarm", "audit",
				`sum by (verb) (rate({scrape_job="audit-logs"}[5m]))`, auditBucket)},
			wantErr: "aggregations are not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RenderValues(tt.exports)
			if err == nil {
				t.Fatalf("RenderValues() succeeded, expected an error about %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("RenderValues() error does not mention %q: %v", tt.wantErr, err)
			}
		})
	}
}

func TestSecretEnv(t *testing.T) {
	withCreds := s3Export("giantswarm", "audit", `{scrape_job="audit-logs"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:         "audit-export",
			Region:         "eu-west-2",
			CredentialsRef: &corev1.LocalObjectReference{Name: "archive-credentials"},
		})
	withRole := s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`,
		observabilityv1alpha1.S3Destination{
			Bucket:  "teleport-archive",
			Region:  "eu-west-2",
			RoleARN: "arn:aws:iam::123456789012:role/log-archive-writer",
		})
	creds := map[client.ObjectKey]Credentials{
		{Namespace: "giantswarm", Name: "audit"}: {AccessKeyID: "AKIAEXAMPLE", SecretAccessKey: "s3cr3t"},
	}

	t.Run("static credentials become environment", func(t *testing.T) {
		env, err := SecretEnv([]observabilityv1alpha1.LogExport{withCreds}, creds)
		if err != nil {
			t.Fatalf("SecretEnv() failed: %v", err)
		}
		want := map[string]string{AccessKeyIDKey: "AKIAEXAMPLE", SecretAccessKeyKey: "s3cr3t"}
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
		if env[AccessKeyIDKey] != "AKIAEXAMPLE" {
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
