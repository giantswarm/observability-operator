package events

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/sprig/v3"
	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
)

var (
	//go:embed templates/events-logger.alloy.template
	alloyEventsConfig         string
	alloyEventsConfigTemplate *template.Template

	//go:embed templates/events-logger-config.alloy.yaml.template
	alloyEventsYAMLConfig         string
	alloyEventsYAMLConfigTemplate *template.Template
)

func init() {
	alloyEventsConfigTemplate = template.Must(template.New("events-logger.alloy").Funcs(sprig.FuncMap()).Parse(alloyEventsConfig))
	alloyEventsYAMLConfigTemplate = template.Must(template.New("events-logger-config.alloy.yaml").Funcs(sprig.FuncMap()).Parse(alloyEventsYAMLConfig))
}

func ConfigMap(cluster *clusterv1.Cluster) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (a *Service) GenerateAlloyEventsConfigMapData(ctx context.Context, cluster *clusterv1.Cluster, tracingEnabled bool, observabilityBundleVersion semver.Version) (map[string]string, error) {
	// Get list of tenants
	tenants, err := a.TenantRepository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	// Get cluster metadata
	organization, err := a.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	provider, err := a.Config.Cluster.GetClusterProvider(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	isWorkloadCluster := a.Config.Cluster.IsWorkloadCluster(cluster)

	// Get Tempo URL for tracing (only for workload clusters with tracing enabled)
	tempoURL := ""
	if tracingEnabled && isWorkloadCluster {
		tempoURL = fmt.Sprintf(common.TempoIngressURLFormat, a.Config.Cluster.BaseDomain)
	}

	// Generate the Alloy configuration from template
	alloyConfig, err := a.generateAlloyEventsConfig(
		cluster.Name,
		a.Config.Cluster.GetClusterType(cluster),
		organization,
		provider,
		tempoURL,
		tenants,
		tracingEnabled,
		isWorkloadCluster,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate alloy events config: %w", err)
	}

	// Generate the values YAML that wraps the Alloy config
	valuesYAML, err := a.generateEventsYAMLConfig(alloyConfig, tracingEnabled, isWorkloadCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to generate events YAML config: %w", err)
	}

	return map[string]string{
		"values": valuesYAML,
	}, nil
}

func (a *Service) generateAlloyEventsConfig(
	clusterID string,
	clusterType string,
	organization string,
	provider string,
	tempoURL string,
	tenants []string,
	tracingEnabled bool,
	isWorkloadCluster bool,
) (string, error) {
	var buf bytes.Buffer

	// Tempo endpoint must be in host:port format for gRPC
	tracingEndpoint := ""
	if tracingEnabled && tempoURL != "" {
		tracingEndpoint = net.JoinHostPort(tempoURL, "443")
	}

	// Template data structure
	data := struct {
		ClusterID          string
		ClusterType        string
		Organization       string
		Provider           string
		InsecureSkipVerify string
		MaxBackoffPeriod   string
		RemoteTimeout      string
		IncludeNamespaces  []string
		ExcludeNamespaces  []string
		SecretName         string
		LoggingURLKey      string
		LoggingTenantIDKey string
		LoggingUsernameKey string
		LoggingPasswordKey string
		IsWorkloadCluster  bool
		TracingEnabled     bool
		TracingEndpoint    string
		TracingUsernameKey string
		TracingPasswordKey string
		Tenants            []string
	}{
		ClusterID:          clusterID,
		ClusterType:        clusterType,
		Organization:       organization,
		Provider:           provider,
		InsecureSkipVerify: fmt.Sprintf("%t", a.Config.Cluster.InsecureCA),
		MaxBackoffPeriod:   common.LokiMaxBackoffPeriod,
		RemoteTimeout:      common.LokiRemoteTimeout,
		IncludeNamespaces:  a.Config.Logging.IncludeEventsNamespaces,
		ExcludeNamespaces:  a.Config.Logging.ExcludeEventsNamespaces,
		SecretName:         apps.AlloyEventsAppName,
		LoggingURLKey:      common.LokiURLKey,
		LoggingTenantIDKey: common.LokiTenantIDKey,
		LoggingUsernameKey: common.LokiUsernameKey,
		LoggingPasswordKey: common.LokiPasswordKey,
		IsWorkloadCluster:  isWorkloadCluster,
		TracingEnabled:     tracingEnabled,
		TracingEndpoint:    tracingEndpoint,
		TracingUsernameKey: common.TempoUsernameKey,
		TracingPasswordKey: common.TempoPasswordKey,
		Tenants:            tenants,
	}

	if err := alloyEventsConfigTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute alloy events config template: %w", err)
	}

	return buf.String(), nil
}

func (a *Service) generateEventsYAMLConfig(alloyConfig string, tracingEnabled bool, isWorkloadCluster bool) (string, error) {
	var buf bytes.Buffer

	data := struct {
		AlloyConfig       string
		TracingEnabled    bool
		IsWorkloadCluster bool
	}{
		AlloyConfig:       alloyConfig,
		TracingEnabled:    tracingEnabled,
		IsWorkloadCluster: isWorkloadCluster,
	}

	if err := alloyEventsYAMLConfigTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute events YAML config template: %w", err)
	}

	// Validate that the generated YAML is valid
	var yamlCheck interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &yamlCheck); err != nil {
		return "", fmt.Errorf("generated invalid YAML: %w", err)
	}

	return buf.String(), nil
}
