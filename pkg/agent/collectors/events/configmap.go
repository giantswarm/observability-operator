package events

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net"
	"slices"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/sprig/v3"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

var (
	//go:embed templates/events-logger.alloy.template
	alloyEventsConfig         string
	alloyEventsConfigTemplate *template.Template

	//go:embed templates/events-logger-config.yaml.template
	alloyEventsYAMLConfig         string
	alloyEventsYAMLConfigTemplate *template.Template
)

func init() {
	alloyEventsConfigTemplate = template.Must(template.New("events-logger.alloy").Funcs(sprig.FuncMap()).Parse(alloyEventsConfig))
	alloyEventsYAMLConfigTemplate = template.Must(template.New("events-logger-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyEventsYAMLConfig))
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

func (s *Service) GenerateAlloyEventsConfigMapData(ctx context.Context, cluster *clusterv1.Cluster, loggingEnabled bool, tracingEnabled bool, otlpMetricsEnabled bool, otlpLogsEnabled bool) (map[string]string, error) {
	// Defensive validation: This method should only be called when at least one feature is enabled.
	// The controller ensures this, but we validate here to catch potential bugs.
	if !loggingEnabled && !tracingEnabled && !otlpMetricsEnabled && !otlpLogsEnabled {
		return nil, fmt.Errorf("cannot generate alloy events config: neither logging nor tracing nor OTLP metrics is enabled")
	}

	// Get list of tenants
	tenants, err := s.TenantRepository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	// Ensure the default GiantSwarm tenant is always present, matching the logs collector behaviour.
	if !slices.Contains(tenants, organization.GiantSwarmDefaultTenant) {
		tenants = append(tenants, organization.GiantSwarmDefaultTenant)
	}

	// Get cluster metadata
	org, err := s.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	provider, err := s.Config.Cluster.GetClusterProvider(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	isWorkloadCluster := s.Config.Cluster.IsWorkloadCluster(cluster)

	// Get Tempo URL for tracing (only for workload clusters with tracing enabled)
	tempoURL := ""
	if tracingEnabled && isWorkloadCluster {
		tempoURL = fmt.Sprintf(common.TempoIngressURLFormat, s.Config.Cluster.BaseDomain)
	}

	// Generate the Alloy configuration from template
	alloyConfig, err := s.generateAlloyEventsConfig(
		cluster.Name,
		s.Config.Cluster.GetClusterType(cluster),
		org,
		provider,
		tempoURL,
		tenants,
		loggingEnabled,
		tracingEnabled,
		otlpMetricsEnabled,
		otlpLogsEnabled,
		isWorkloadCluster,
	)
	if err != nil {
	}
	valuesYAML, err := s.generateEventsYAMLConfig(alloyConfig, loggingEnabled, tracingEnabled, otlpMetricsEnabled, otlpLogsEnabled, isWorkloadCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to generate events YAML config: %w", err)
	}

	return map[string]string{
		"values": valuesYAML,
	}, nil
}

func (s *Service) generateAlloyEventsConfig(
	clusterID string,
	clusterType string,
	organization string,
	provider string,
	tempoURL string,
	tenants []string,
	loggingEnabled bool,
	tracingEnabled bool,
	otlpMetricsEnabled bool,
	otlpLogsEnabled bool,
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
		ClusterID              string
		ClusterType            string
		Organization           string
		Provider               string
		InsecureSkipVerify     string
		MaxBackoffPeriod       string
		RemoteTimeout          string
		IncludeNamespaces      []string
		ExcludeNamespaces      []string
		SecretName             string
		LoggingURLKey          string
		LoggingTenantIDKey     string
		LoggingUsernameKey     string
		LoggingPasswordKey     string
		IsWorkloadCluster      bool
		LoggingEnabled         bool
		TracingEnabled         bool
		TracingEndpoint        string
		TempoUsernameKey       string
		TempoPasswordKey       string
		OTLPBatchSendBatchSize int
		OTLPBatchTimeout       string
		OTLPBatchMaxSize       int
		Tenants                []string
		OTLPMetricsEnabled     bool
		MimirOTLPURLKey        string
		MimirOTLPUsernameKey   string
		MimirOTLPPasswordKey   string
		OTLPLogsEnabled        bool
		LokiOTLPURLKey         string
		LokiOTLPUsernameKey    string
		LokiOTLPPasswordKey    string
	}{
		ClusterID:              clusterID,
		ClusterType:            clusterType,
		Organization:           organization,
		Provider:               provider,
		InsecureSkipVerify:     fmt.Sprintf("%t", s.Config.Cluster.InsecureCA),
		MaxBackoffPeriod:       common.LokiMaxBackoffPeriod,
		RemoteTimeout:          common.LokiRemoteTimeout,
		IncludeNamespaces:      s.Config.Logging.IncludeEventsNamespaces,
		ExcludeNamespaces:      s.Config.Logging.ExcludeEventsNamespaces,
		SecretName:             apps.AlloyEventsAppName,
		LoggingURLKey:          common.LokiURLKey,
		LoggingTenantIDKey:     common.LokiTenantIDKey,
		LoggingUsernameKey:     common.LokiUsernameKey,
		LoggingPasswordKey:     common.LokiPasswordKey,
		IsWorkloadCluster:      isWorkloadCluster,
		LoggingEnabled:         loggingEnabled,
		TracingEnabled:         tracingEnabled,
		TracingEndpoint:        tracingEndpoint,
		TempoUsernameKey:       common.TempoUsernameKey,
		TempoPasswordKey:       common.TempoPasswordKey,
		OTLPBatchSendBatchSize: common.OTLPBatchSendBatchSize,
		OTLPBatchTimeout:       common.OTLPBatchTimeout,
		OTLPBatchMaxSize:       common.OTLPBatchMaxSize,
		Tenants:                tenants,
		OTLPMetricsEnabled:     otlpMetricsEnabled,
		MimirOTLPURLKey:        common.MimirOTLPURLKey,
		MimirOTLPUsernameKey:   common.MimirRemoteWriteAPIUsernameKey,
		MimirOTLPPasswordKey:   common.MimirRemoteWriteAPIPasswordKey,
		OTLPLogsEnabled:        otlpLogsEnabled,
		LokiOTLPURLKey:         common.LokiOTLPURLKey,
		LokiOTLPUsernameKey:    common.LokiUsernameKey,
		LokiOTLPPasswordKey:    common.LokiPasswordKey,
	}

	if err := alloyEventsConfigTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute alloy events config template: %w", err)
	}

	return buf.String(), nil
}

func (s *Service) generateEventsYAMLConfig(alloyConfig string, loggingEnabled bool, tracingEnabled bool, otlpMetricsEnabled bool, otlpLogsEnabled bool, isWorkloadCluster bool) (string, error) {
	var buf bytes.Buffer

	data := struct {
		AlloyConfig        string
		LoggingEnabled     bool
		TracingEnabled     bool
		OTLPMetricsEnabled bool
		OTLPLogsEnabled    bool
		IsWorkloadCluster  bool
	}{
		AlloyConfig:        alloyConfig,
		LoggingEnabled:     loggingEnabled,
		TracingEnabled:     tracingEnabled,
		OTLPMetricsEnabled: otlpMetricsEnabled,
		OTLPLogsEnabled:    otlpLogsEnabled,
		IsWorkloadCluster:  isWorkloadCluster,
	}

	if err := alloyEventsYAMLConfigTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute events YAML config template: %w", err)
	}

	// Validate that the generated YAML is valid
	var yamlCheck any
	if err := yaml.Unmarshal(buf.Bytes(), &yamlCheck); err != nil {
		return "", fmt.Errorf("generated invalid YAML: %w", err)
	}

	return buf.String(), nil
}
