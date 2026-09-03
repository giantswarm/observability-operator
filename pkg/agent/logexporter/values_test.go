package logexporter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/config"
)

// Fixtures asserted by more than one case. Named only because they repeat; the one-off
// values below stay inline so each case still reads on its own.
const (
	auditBucketName    = "audit-export"
	teleportBucketName = "teleport-archive"
	s3Region           = "eu-west-2"
	auditExportName    = "audit"
	platformNamespace  = "giantswarm"
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
	Bucket: auditBucketName,
	Region: s3Region,
}

// testLogExportConfig is the operator configuration the golden files were rendered with.
// It carries the chart defaults, so a change to those shows up as a golden diff rather
// than passing unnoticed.
func testLogExportConfig() config.LogExportConfig {
	return config.LogExportConfig{
		Replicas:      2,
		WALSize:       "10Gi",
		ExportTimeout: "5m",
	}
}

func TestRenderValues(t *testing.T) {
	tests := []struct {
		name       string
		exports    []observabilityv1alpha1.LogExport
		goldenPath string
	}{
		{
			name:       "minimal selector",
			exports:    []observabilityv1alpha1.LogExport{s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`, auditBucket)},
			goldenPath: "alloy-logexporter-config.minimal.yaml",
		},
		{
			name: "parse and filter",
			exports: []observabilityv1alpha1.LogExport{s3Export(platformNamespace, auditExportName,
				`{scrape_job="audit-logs"} | json | verb="delete" | user=~"admin.*"`, auditBucket)},
			goldenPath: "alloy-logexporter-config.parse-and-filter.yaml",
		},
		{
			name: "s3 options",
			exports: []observabilityv1alpha1.LogExport{s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`,
				observabilityv1alpha1.S3Destination{
					Bucket:         auditBucketName,
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
					observabilityv1alpha1.S3Destination{Bucket: teleportBucketName, Region: s3Region}),
				s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`, auditBucket),
			},
			goldenPath: "alloy-logexporter-config.two-exports.yaml",
		},
		{
			name: "credentialsRef does not change the values document",
			exports: []observabilityv1alpha1.LogExport{s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`,
				observabilityv1alpha1.S3Destination{
					Bucket:         auditBucketName,
					Region:         s3Region,
					CredentialsRef: &corev1.LocalObjectReference{Name: "archive-credentials"},
				})},
			goldenPath: "alloy-logexporter-config.minimal.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderValues(tt.exports, testLogExportConfig())
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

			//nolint:gosec // G304: the path is built from the test table, not from input.
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

// TestRenderValuesAssertsSilentFailures pins the settings that fail silently when
// dropped: without them the export either misses data or costs a fortune, and nothing in
// the pipeline reports it.
func TestRenderValuesAssertsSilentFailures(t *testing.T) {
	result, err := RenderValues([]observabilityv1alpha1.LogExport{
		s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`, auditBucket),
	}, testLogExportConfig())
	if err != nil {
		t.Fatalf("RenderValues() failed: %v", err)
	}

	required := map[string]string{
		`action              = "drop"`:                         "selection has to be a drop of the negated selector; a keep exports every line in the installation",
		`selector            = "{scrape_job!=\"audit-logs\"}"`: "the drop selector has to be the negation of the customer's",
		"batch {":                     "without a batch block every log line becomes its own S3 object",
		`sizer             = "items"`: "the queue counts requests by default, which is under a second of buffer",
		"block_on_overflow = true":    "overflow has to be backpressure, not a silent drop",
		fmt.Sprintf("timeout = %q", testLogExportConfig().ExportTimeout): "the 5s default makes any outage over 5s permanent silent loss",
		`s3_partition_timezone = "UTC"`:                                  "partitions otherwise shift with the pod's timezone",
		"gs_cluster_id":                                                  "audit events carry no cluster identifier and body drops the Loki labels",
		"stabilityLevel: experimental":                                   "Alloy refuses to load an awss3 exporter without it",
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

// TestRenderValuesUsesConfig checks that the configurable values actually reach the
// document. Nothing else would catch a field wired to the wrong template key: the golden
// files only ever render the defaults.
func TestRenderValuesUsesConfig(t *testing.T) {
	values, err := RenderValues(
		[]observabilityv1alpha1.LogExport{s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"}`, auditBucket)},
		config.LogExportConfig{
			Replicas:      5,
			WALSize:       "50Gi",
			ExportTimeout: "30m",
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
		},
	)
	if err != nil {
		t.Fatalf("RenderValues() failed: %v", err)
	}

	for _, want := range []string{
		"replicas: 5",
		`storage: "50Gi"`,
		`timeout = "30m"`,
		"memory: 2Gi",
	} {
		if !strings.Contains(values, want) {
			t.Errorf("rendered values do not contain %q", want)
		}
	}
	// The defaults must be replaced, not merged with the configured resources. Asserting a
	// default *value*, since the surrounding Kyverno comment mentions the key names.
	if strings.Contains(values, "cpu: 100m") {
		t.Error("rendered values still carry the default resources")
	}
}

// TestRenderValuesIsValidYAML checks the two-stage nesting: the river config is rendered
// first and then embedded as a string in the values document, so an indentation slip
// produces a values file Flux cannot parse.
func TestRenderValuesIsValidYAML(t *testing.T) {
	values, err := RenderValues([]observabilityv1alpha1.LogExport{
		s3Export(platformNamespace, auditExportName, `{scrape_job="audit-logs"} | json | verb="delete"`, auditBucket),
		s3Export("org-fleetio", "teleport", `{scrape_job="teleport.giantswarm.io"}`, auditBucket),
	}, testLogExportConfig())
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
				ObjectMeta: metav1.ObjectMeta{Name: "to-loki", Namespace: platformNamespace},
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
			wantErr: "both render to the Alloy component name",
		},
		{
			// One case is enough: the selector subset is validation's to police, and
			// selector_test.go covers the translation.
			name: "selector rejected by validation",
			exports: []observabilityv1alpha1.LogExport{s3Export(platformNamespace, auditExportName,
				`sum by (verb) (rate({scrape_job="audit-logs"}[5m]))`, auditBucket)},
			wantErr: "aggregations are not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RenderValues(tt.exports, testLogExportConfig())
			if err == nil {
				t.Fatalf("RenderValues() succeeded, expected an error about %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("RenderValues() error does not mention %q: %v", tt.wantErr, err)
			}
		})
	}
}
