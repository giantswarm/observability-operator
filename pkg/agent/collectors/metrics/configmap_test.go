package metrics

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/common/organization/mocks"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

const (
	awsClusterKind     = "AWSCluster"
	azureClusterKind   = "AzureCluster"
	vsphereClusterKind = "VSphereCluster"
	defaultNamespace   = "default"
	tenant1            = "tenant1"
	tenant2            = "tenant2"
	testCluster        = "test-cluster"
)

var managementClusterName = "dummy-cluster"

// TestMonitoringConfigReplicasRoundTrip guards against any future code changes
// between the read and write methods for the Alloy configuration, which are
// performed in 2 completely different manners:
// - read uses the minimalist monitoringConfig struct which only decodes the
// alloy.controller.replicas value
// - write uses the monitoring-config.yaml.template template file to generate
// the new Alloy configuration)
// If either side changes where the alloy.controller.replicas value is
// read/written it would silently break and reintroduce the bug. This test
// prevent this by ensuring that the value is always read and written at the
// same place in the Alloy configuration file.
func TestMonitoringConfigReplicasRoundTrip(t *testing.T) {
	const want = 7 // sentinel, distinct from the default of 1

	var rendered bytes.Buffer
	err := alloyMonitoringConfigTemplate.Execute(&rendered, struct {
		AlloyAppName      string
		AlloyConfig       string
		AlloySecretName   string
		HasCABundle       bool
		MimirUsernameKey  string
		MimirPasswordKey  string
		PriorityClassName string
		Replicas          int

		MimirRulesLivenessProbeEnabled bool
		MimirRulesLivenessProbe        config.MimirRulesLivenessProbeConfig
	}{Replicas: want, MimirRulesLivenessProbeEnabled: true})
	if err != nil {
		t.Fatalf("failed to render monitoring config template: %v", err)
	}

	var parsed monitoringConfig
	if err := yaml.Unmarshal(rendered.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal rendered config with monitoringConfig struct: %v", err)
	}

	if parsed.Alloy.Controller.Replicas != want {
		t.Errorf("read/write format drift: template wrote replicas=%d under alloy.controller.replicas, "+
			"but monitoringConfig struct read %d — the read path no longer matches the template",
			want, parsed.Alloy.Controller.Replicas)
	}
}

// mimirRulesLivenessProbeValues holds the rendered alloy-metrics values of the liveness probe.
type mimirRulesLivenessProbeValues struct {
	Alloy struct {
		Alloy struct {
			LivenessProbe struct {
				Exec struct {
					Command []string `json:"command"`
				} `json:"exec"`
			} `json:"livenessProbe"`
			ExtraEnv []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"extraEnv"`
		} `json:"alloy"`
		Controller struct {
			Volumes struct {
				Extra []struct {
					Name      string `json:"name"`
					ConfigMap *struct {
						Name     string `json:"name"`
						Optional *bool  `json:"optional"`
					} `json:"configMap"`
				} `json:"extra"`
			} `json:"volumes"`
		} `json:"controller"`
	} `json:"alloy"`
}

// renderMimirRulesLivenessProbeValues renders the alloy-metrics values with the given probe config.
func renderMimirRulesLivenessProbeValues(t *testing.T, probe config.MimirRulesLivenessProbeConfig) mimirRulesLivenessProbeValues {
	t.Helper()
	service := newTestService(true, true, false)
	service.Config.Monitoring.MimirRulesLivenessProbe = probe
	result, err := service.GenerateAlloyMonitoringConfigMapData(context.Background(), nil, &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: testCluster, Namespace: defaultNamespace},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{Kind: awsClusterKind},
		},
	}, []string{organization.GiantSwarmDefaultTenant}, versionSupportingMimirRulesLivenessProbe)
	if err != nil {
		t.Fatalf("GenerateAlloyMonitoringConfigMapData() failed: %v", err)
	}

	var values mimirRulesLivenessProbeValues
	if err := yaml.Unmarshal([]byte(result["values"]), &values); err != nil {
		t.Fatalf("failed to unmarshal rendered values: %v", err)
	}
	return values
}

// TestMimirRulesLivenessProbeVolume asserts the script ConfigMap volume is optional. alloy-app
// renders the ConfigMap only with mimirRulesLivenessProbe.enabled. A required volume keeps
// alloy-metrics in ContainerCreating without it.
func TestMimirRulesLivenessProbeVolume(t *testing.T) {
	values := renderMimirRulesLivenessProbeValues(t, config.DefaultMimirRulesLivenessProbeConfig())
	for _, v := range values.Alloy.Controller.Volumes.Extra {
		if v.Name != "mimir-rules-liveness-probe" {
			continue
		}
		if v.ConfigMap == nil || v.ConfigMap.Name != "alloy-metrics-mimir-rules-liveness-probe" {
			t.Fatalf("unexpected liveness probe volume %+v", v)
		}
		if v.ConfigMap.Optional == nil || !*v.ConfigMap.Optional {
			t.Errorf("liveness probe ConfigMap volume is not optional")
		}
		return
	}
	t.Fatalf("liveness probe volume not rendered")
}

// TestMimirRulesLivenessProbeEnv decodes the rendered script settings and compares them with
// the config, so that a quoting change cannot alter the pattern.
func TestMimirRulesLivenessProbeEnv(t *testing.T) {
	custom := config.DefaultMimirRulesLivenessProbeConfig()
	custom.UnhealthyMessagePattern = `^(a|b) "c" $d \. 'e' {{ f }}`
	custom.MimirReadyPath = `/a #b: "c" $d`

	for name, probe := range map[string]config.MimirRulesLivenessProbeConfig{
		"defaults": config.DefaultMimirRulesLivenessProbeConfig(),
		"custom":   custom,
	} {
		t.Run(name, func(t *testing.T) {
			values := renderMimirRulesLivenessProbeValues(t, probe)
			got := map[string]string{}
			for _, e := range values.Alloy.Alloy.ExtraEnv {
				got[e.Name] = e.Value
			}
			want := map[string]string{
				"HTTP_TIMEOUT":              strconv.Itoa(int(probe.HTTPTimeout.Seconds())),
				"PROBE_TIMEOUT":             strconv.Itoa(int(probe.ProbeTimeout.Seconds())),
				"HEALTH_AGE_MARGIN":         strconv.Itoa(int(probe.HealthAgeMargin.Seconds())),
				"UNHEALTHY_MESSAGE_PATTERN": probe.UnhealthyMessagePattern,
				"MIMIR_READY_PATH":          probe.MimirReadyPath,
				// Read from secretKeyRef.
				"MIMIR_USERNAME": "",
				"MIMIR_PASSWORD": "",
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("rendered extraEnv mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestMimirRulesLivenessProbeCommand runs the rendered liveness probe command. A missing
// ConfigMap, or a values layer replacing mounts.extra, removes the script: the probe must
// then succeed, or the kubelet restarts Alloy forever.
func TestMimirRulesLivenessProbeCommand(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	values := renderMimirRulesLivenessProbeValues(t, config.DefaultMimirRulesLivenessProbeConfig())
	command := values.Alloy.Alloy.LivenessProbe.Exec.Command
	const scriptPath = "/etc/mimir-rules-liveness-probe/mimir-rules-liveness-probe.sh"
	if len(command) == 0 || command[0] != "bash" || command[len(command)-1] != scriptPath {
		t.Fatalf("unexpected liveness probe command %q", command)
	}

	tests := []struct {
		name string
		// script is the probe script content; empty means the script is missing.
		script   string
		wantCode int
		wantOut  string
	}{
		{name: "missing script", wantCode: 0},
		{name: "healthy", script: "exit 0", wantCode: 0},
		{name: "unhealthy", script: "exit 1", wantCode: 1},
		// The script re-executes itself through $0.
		{name: "script path in $0", script: `echo "$0"`, wantOut: "/mimir-rules-liveness-probe.sh\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mimir-rules-liveness-probe.sh")
			if tt.script != "" {
				if err := os.WriteFile(path, []byte(tt.script), 0o600); err != nil {
					t.Fatalf("failed to write script: %v", err)
				}
			}
			args := append(slices.Clone(command[1:len(command)-1]), path)

			out, err := exec.Command(bash, args...).Output() //nolint:gosec
			code := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("failed to run liveness probe command: %v", err)
			}
			if code != tt.wantCode {
				t.Errorf("liveness probe command exited %d, want %d", code, tt.wantCode)
			}
			if tt.wantOut != "" && string(out) != filepath.Dir(path)+tt.wantOut {
				t.Errorf("liveness probe command printed %q, want %q", out, filepath.Dir(path)+tt.wantOut)
			}
		})
	}
}

func newTestService(monitoringEnabled, exemplarsEnabled, vcenterEnabled bool) *Service {
	return &Service{
		OrganizationRepository: mocks.NewMockOrganizationRepository("dummy-org"),
		Config: config.Config{
			Cluster: config.ClusterConfig{
				BaseDomain: "test.gigantic.io",
				Customer:   "dummy-customer",
				Name:       managementClusterName,
				Pipeline:   "dummy-pipeline",
				Region:     "dummy-region",
			},
			Monitoring: config.MonitoringConfig{
				Enabled:                 monitoringEnabled,
				ExemplarsEnabled:        exemplarsEnabled,
				VCenterEnabled:          vcenterEnabled,
				WALTruncateFrequency:    time.Minute,
				MimirRemoteWriteTimeout: "60s",
				QueueConfig: config.QueueConfig{
					Capacity:          &[]int{10000}[0],
					MaxShards:         &[]int{50}[0],
					MaxSamplesPerSend: &[]int{2000}[0],
					SampleAgeLimit:    &[]string{"30m"}[0],
				},
				MimirRulesLivenessProbe: config.DefaultMimirRulesLivenessProbeConfig(),
			},
			HTTP: config.HTTPConfig{
				MimirQueryTimeout: 2 * time.Minute,
			},
			DefaultTenant: "giantswarm",
		},
	}
}

func TestGenerateMonitoringConfig(t *testing.T) {
	tests := []struct {
		name                       string
		cluster                    *clusterv1.Cluster
		tenants                    []string
		goldenPath                 string
		observabilityBundleVersion semver.Version
		monitoringEnabled          bool
		exemplarsEnabled           bool
		vcenterEnabled             bool
		// mimirRulesLivenessProbe overrides config.DefaultMimirRulesLivenessProbeConfig() when set.
		mimirRulesLivenessProbe *config.MimirRulesLivenessProbeConfig
	}{
		// Version 2.0.0+ tests (with extra query matchers, without scrape configs)
		{
			name: "WorkloadCluster_TwoTenants_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testCluster,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1, tenant2},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.200.WC.multi-tenants.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_TwoTenants_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1, tenant2},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.200.MC.multi-tenants.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "WorkloadCluster_SingleTenant_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-tenant-cluster",
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.200.WC.single-tenant.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_SingleTenant_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.200.MC.single-tenant.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "WorkloadCluster_DefaultTenant_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-tenant-cluster",
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.200.WC.default-tenant.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_DefaultTenant_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.200.MC.default-tenant.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},

		// Version 2.2.0+ tests (with extra query matchers and scrape configs)
		{
			name: "WorkloadCluster_TwoTenants_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testCluster,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1, tenant2},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.WC.multi-tenants.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_TwoTenants_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1, tenant2},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.MC.multi-tenants.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "WorkloadCluster_SingleTenant_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-tenant-cluster",
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.WC.single-tenant.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_SingleTenant_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.MC.single-tenant.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "WorkloadCluster_DefaultTenant_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-tenant-cluster",
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.WC.default-tenant.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_DefaultTenant_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: azureClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.MC.default-tenant.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_VSphere_DefaultTenant_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: vsphereClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.MC.vsphere.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
			vcenterEnabled:             true,
		},
		{
			name: "ManagementCluster_VSphere_VCenterDisabled_v220",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: vsphereClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.220.MC.vsphere.vcenter-disabled.yaml"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		// Mimir rules liveness probe tests
		{
			name: "WorkloadCluster_DefaultTenant_MimirRulesLivenessProbe",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testCluster,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.mimir-rules-liveness-probe.WC.default-tenant.yaml"),
			observabilityBundleVersion: versionSupportingMimirRulesLivenessProbe,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "ManagementCluster_DefaultTenant_MimirRulesLivenessProbe",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.mimir-rules-liveness-probe.MC.default-tenant.yaml"),
			observabilityBundleVersion: versionSupportingMimirRulesLivenessProbe,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
		},
		{
			name: "WorkloadCluster_DefaultTenant_MimirRulesLivenessProbeDisabled",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testCluster,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.mimir-rules-liveness-probe-disabled.WC.default-tenant.yaml"),
			observabilityBundleVersion: versionSupportingMimirRulesLivenessProbe,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
			mimirRulesLivenessProbe:    &config.MimirRulesLivenessProbeConfig{Enabled: false},
		},
		{
			name: "WorkloadCluster_DefaultTenant_MimirRulesLivenessProbeCustom",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testCluster,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{organization.GiantSwarmDefaultTenant},
			goldenPath:                 filepath.Join("testdata", "monitoring-config.mimir-rules-liveness-probe-custom.WC.default-tenant.yaml"),
			observabilityBundleVersion: versionSupportingMimirRulesLivenessProbe,
			monitoringEnabled:          true,
			exemplarsEnabled:           true,
			mimirRulesLivenessProbe: &config.MimirRulesLivenessProbeConfig{
				Enabled:                 true,
				HTTPTimeout:             10 * time.Second,
				ProbeTimeout:            80 * time.Second,
				HealthAgeMargin:         30 * time.Second,
				UnhealthyMessagePattern: "connection refused|i/o timeout",
				MimirReadyPath:          "/ruler/ready",
				InitialDelaySeconds:     120,
				PeriodSeconds:           30,
				TimeoutSeconds:          90,
				FailureThreshold:        5,
			},
		},
		{
			name: "ExemplarsDisabledInMC_v200",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants:                    []string{tenant1},
			goldenPath:                 filepath.Join("testdata", "monitoring-config_singletenant.200.mc.exemplars-disabled.yaml"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          true,
			exemplarsEnabled:           false,
		},
		{
			name: "MonitoringDisabled",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testCluster,
					Namespace: defaultNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: awsClusterKind,
					},
				},
			},
			tenants: []string{tenant1},
			// goldenPath omitted - this should return an error
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			monitoringEnabled:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			service := newTestService(tt.monitoringEnabled, tt.exemplarsEnabled, tt.vcenterEnabled)
			if tt.mimirRulesLivenessProbe != nil {
				service.Config.Monitoring.MimirRulesLivenessProbe = *tt.mimirRulesLivenessProbe
			}

			resultMap, err := service.GenerateAlloyMonitoringConfigMapData(
				ctx,
				nil, // currentState
				tt.cluster,
				tt.tenants,
				tt.observabilityBundleVersion,
			)

			// Check if this is a "monitoring disabled" test case (no golden path)
			if tt.goldenPath == "" {
				// Should return an error when monitoring is disabled
				if err == nil {
					t.Errorf("GenerateAlloyMonitoringConfigMapData() expected error when monitoring disabled, got nil")
				}
				return
			}

			// For valid test cases, no error should occur
			if err != nil {
				t.Fatalf("GenerateAlloyMonitoringConfigMapData() failed: %v", err)
			}

			result, ok := resultMap["values"]
			if !ok {
				t.Fatalf("GenerateAlloyMonitoringConfigMapData() did not return 'values' key")
			}

			if os.Getenv("UPDATE_GOLDEN_FILES") == "true" {
				t.Logf("Environment variable UPDATE_GOLDEN_FILES=true detected, updating golden files")
				if err := os.MkdirAll(filepath.Dir(tt.goldenPath), 0o750); err != nil {
					t.Fatalf("failed to create golden directory: %v", err)
				}
				//nolint:gosec
				if err := os.WriteFile(tt.goldenPath, []byte(result), 0o644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			expected, err := os.ReadFile(tt.goldenPath)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v", tt.goldenPath, err)
			}

			if diff := cmp.Diff(string(expected), result); diff != "" {
				t.Errorf("GenerateAlloyMonitoringConfigMapData() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
