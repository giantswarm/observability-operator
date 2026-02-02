package logs

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"slices"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/blang/semver/v4"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

var (
	//go:embed templates/logging.alloy.template
	alloyLogging         string
	alloyLoggingTemplate *template.Template

	//go:embed templates/logging-config.yaml.template
	alloyLoggingConfig         string
	alloyLoggingConfigTemplate *template.Template

	// Version constraints
	alloyNodeFilterFixedBundleVersion = semver.MustParse(AlloyNodeFilterFixedObservabilityBundleAppVersion)
)

func init() {
	alloyLoggingTemplate = template.Must(template.New("logging.alloy").Funcs(sprig.FuncMap()).Parse(alloyLogging))
	alloyLoggingConfigTemplate = template.Must(template.New("logging-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyLoggingConfig))
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

func (s *Service) GenerateAlloyLogsConfigMapData(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version, loggingEnabled bool, networkMonitoringEnabled bool) (map[string]string, error) {
	// Get tenant IDs
	tenants, err := s.TenantRepository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant IDs: %w", err)
	}

	// Get organization
	org, err := s.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to read organization: %w", err)
	}

	// Get cluster provider
	provider, err := s.Config.Cluster.GetClusterProvider(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster provider: %w", err)
	}

	// Network monitoring requires node filtering (clustering is incompatible with host network)
	enableNodeFiltering := s.Config.Logging.EnableNodeFiltering
	if networkMonitoringEnabled {
		enableNodeFiltering = true
	}

	// Generate Alloy logging configuration
	alloyConfig, err := generateAlloyLoggingConfig(
		cluster,
		observabilityBundleVersion,
		s.Config.Logging.DefaultNamespaces,
		tenants,
		org,
		provider,
		s.Config.Cluster.InsecureCA,
		enableNodeFiltering,
		loggingEnabled,
		networkMonitoringEnabled,
		s.Config.Cluster,
	)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"values": alloyConfig,
	}, nil
}

// generateAlloyLoggingConfig generates the Alloy logging configuration (Helm values)
func generateAlloyLoggingConfig(
	cluster *clusterv1.Cluster,
	observabilityBundleVersion semver.Version,
	defaultWorkloadClusterNamespaces []string,
	tenants []string,
	org string,
	provider string,
	insecureCA bool,
	enableNodeFiltering bool,
	enableLogging bool,
	enableNetworkMonitoring bool,
	clusterConfig config.ClusterConfig,
) (string, error) {
	// Generate River configuration
	alloyConfig, err := generateAlloyConfig(tenants, cluster, org, provider, insecureCA, enableNodeFiltering, enableLogging, enableNetworkMonitoring, clusterConfig)
	if err != nil {
		return "", err
	}

	isWorkloadCluster := clusterConfig.IsWorkloadCluster(cluster)

	// Prepare template data
	data := struct {
		AlloyConfig                      string
		AlloyImageTag                    *string
		DefaultWorkloadClusterNamespaces []string
		DefaultWriteTenant               string
		LoggingEnabled                   bool
		NetworkMonitoringEnabled         bool
		NodeFilteringEnabled             bool
		IsWorkloadCluster                bool
		PriorityClassName                string
	}{
		AlloyConfig:                      alloyConfig,
		DefaultWorkloadClusterNamespaces: defaultWorkloadClusterNamespaces,
		DefaultWriteTenant:               organization.GiantSwarmDefaultTenant,
		LoggingEnabled:                   enableLogging,
		NetworkMonitoringEnabled:         enableNetworkMonitoring,
		NodeFilteringEnabled:             enableNodeFiltering,
		IsWorkloadCluster:                isWorkloadCluster,
		PriorityClassName:                common.PriorityClassName,
	}

	// If node filtering is enabled but bundle version is before the fix, use a fixed Alloy version
	if enableNodeFiltering && observabilityBundleVersion.LT(alloyNodeFilterFixedBundleVersion) {
		imageTag := fmt.Sprintf("v%s", AlloyNodeFilterImageVersion)
		data.AlloyImageTag = &imageTag
	}

	// Execute template
	var buffer bytes.Buffer
	err = alloyLoggingConfigTemplate.Execute(&buffer, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute alloy logging config template: %w", err)
	}

	return buffer.String(), nil
}

// generateAlloyConfig generates the River configuration for Alloy logging
func generateAlloyConfig(
	tenants []string,
	cluster *clusterv1.Cluster,
	org string,
	provider string,
	insecureCA bool,
	enableNodeFiltering bool,
	enableLogging bool,
	enableNetworkMonitoring bool,
	clusterConfig config.ClusterConfig,
) (string, error) {
	// Ensure default tenant is included
	if !slices.Contains(tenants, organization.GiantSwarmDefaultTenant) {
		tenants = append(tenants, organization.GiantSwarmDefaultTenant)
	}

	isWorkloadCluster := clusterConfig.IsWorkloadCluster(cluster)

	// Prepare template data for River configuration
	data := struct {
		ClusterID                string
		ClusterType              string
		Organization             string
		Installation             string
		Provider                 string
		MaxBackoffPeriod         string
		RemoteTimeout            string
		IsWorkloadCluster        bool
		NodeFilteringEnabled     bool
		LoggingEnabled           bool
		NetworkMonitoringEnabled bool
		InsecureSkipVerify       bool
		SecretName               string
		LoggingURLKey            string
		LoggingTenantIDKey       string
		LoggingUsernameKey       string
		LoggingPasswordKey       string
		LokiRulerAPIURLKey       string
		Tenants                  []string
	}{
		ClusterID:                cluster.Name,
		ClusterType:              clusterConfig.GetClusterType(cluster),
		Organization:             org,
		Installation:             clusterConfig.Name,
		Provider:                 provider,
		MaxBackoffPeriod:         common.LokiMaxBackoffPeriod,
		RemoteTimeout:            common.LokiRemoteTimeout,
		IsWorkloadCluster:        isWorkloadCluster,
		NodeFilteringEnabled:     enableNodeFiltering,
		LoggingEnabled:           enableLogging,
		NetworkMonitoringEnabled: enableNetworkMonitoring,
		InsecureSkipVerify:       insecureCA,
		SecretName:               apps.AlloyLogsAppName,
		LoggingURLKey:            common.LokiURLKey,
		LoggingTenantIDKey:       common.LokiTenantIDKey,
		LoggingUsernameKey:       common.LokiUsernameKey,
		LoggingPasswordKey:       common.LokiPasswordKey,
		LokiRulerAPIURLKey:       common.LokiRulerAPIURLKey,
		Tenants:                  tenants,
	}

	// Execute template to generate River configuration
	var buffer bytes.Buffer
	err := alloyLoggingTemplate.Execute(&buffer, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute alloy config template: %w", err)
	}

	return buffer.String(), nil
}
