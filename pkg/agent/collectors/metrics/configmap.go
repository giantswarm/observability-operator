package metrics

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/Masterminds/sprig/v3"
	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

var (
	//go:embed templates/metrics.alloy.template
	alloyConfig         string
	alloyConfigTemplate *template.Template

	//go:embed templates/monitoring-config.yaml.template
	alloyMonitoringConfig         string
	alloyMonitoringConfigTemplate *template.Template

	versionSupportingScrapeConfigs = semver.MustParse("2.2.0")
)

func init() {
	alloyConfigTemplate = template.Must(template.New("alloy-config.alloy").Funcs(sprig.FuncMap()).Parse(alloyConfig))
	alloyMonitoringConfigTemplate = template.Must(template.New("monitoring-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyMonitoringConfig))
}

// GenerateAlloyMonitoringConfigMapData renders the Alloy monitoring ConfigMap
// payload. It is a pure transformation of the inputs: no I/O, no Mimir query,
// no state reads. Shard resolution lives in Service.ReconcileCreate so tests
// can exercise the template without a Mimir stub.
func (s *Service) GenerateAlloyMonitoringConfigMapData(ctx context.Context, cluster *clusterv1.Cluster, tenants []string, observabilityBundleVersion semver.Version, shards int) (map[string]string, error) {
	// Defensive validation: This method should only be called when monitoring is enabled.
	// The controller ensures this, but we validate here to catch potential bugs.
	if !s.Config.Monitoring.IsMonitoringEnabled(cluster) {
		return nil, fmt.Errorf("cannot generate alloy monitoring config: monitoring is not enabled for cluster %s", cluster.Name)
	}

	alloyConfig, err := s.generateAlloyConfig(ctx, cluster, tenants, observabilityBundleVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate alloy config: %w", err)
	}

	data := struct {
		AlloyConfig       string
		HasCABundle       bool
		PriorityClassName string
		Replicas          int
	}{
		AlloyConfig:       alloyConfig,
		HasCABundle:       s.Config.Cluster.CASecretName != "",
		PriorityClassName: common.PriorityClassName,
		Replicas:          shards,
	}

	var values bytes.Buffer
	err = alloyMonitoringConfigTemplate.Execute(&values, data)
	if err != nil {
		return nil, fmt.Errorf("failed to template alloy monitoring config: %w", err)
	}

	configMapData := make(map[string]string)
	configMapData["values"] = values.String()

	return configMapData, nil
}

func (s *Service) generateAlloyConfig(ctx context.Context, cluster *clusterv1.Cluster, tenants []string, observabilityBundleVersion semver.Version) (string, error) {
	var values bytes.Buffer

	org, err := s.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("failed to read organization: %w", err)
	}

	provider, err := s.Config.Cluster.GetClusterProvider(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster provider: %w", err)
	}

	data := struct {
		AlloySecretName      string
		AlloySecretNamespace string

		MimirRulerAPIURLKey        string
		MimirUsernameKey           string
		MimirPasswordKey           string
		MimirRemoteWriteAPIURLKey  string
		MimirRemoteWriteAPINameKey string
		MimirRemoteWriteTimeout    string
		HasCABundle                bool

		ClusterID         string
		IsWorkloadCluster bool

		Tenants         []string
		DefaultTenantID string

		QueueConfigBatchSendDeadline *string
		QueueConfigCapacity          *int
		QueueConfigMaxBackoff        *string
		QueueConfigMaxSamplesPerSend *int
		QueueConfigMaxShards         *int
		QueueConfigMinBackoff        *string
		QueueConfigMinShards         *int
		QueueConfigRetryOnHttp429    *bool
		QueueConfigSampleAgeLimit    *string

		WALTruncateFrequency string

		ExternalLabels map[string]string

		IsSupportingScrapeConfigs bool
		ExemplarsEnabled          bool
	}{
		AlloySecretName:      apps.AlloyMetricsAppName,
		AlloySecretNamespace: apps.AlloyNamespace,

		MimirRulerAPIURLKey:        common.MimirRulerAPIURLKey,
		MimirUsernameKey:           common.MimirUsernameKey,
		MimirPasswordKey:           common.MimirPasswordKey,
		MimirRemoteWriteAPIURLKey:  common.MimirRemoteWriteAPIURLKey,
		MimirRemoteWriteAPINameKey: common.MimirRemoteWriteAPINameKey,
		MimirRemoteWriteTimeout:    s.Config.Monitoring.MimirRemoteWriteTimeout,
		HasCABundle:                s.Config.Cluster.CASecretName != "",

		ClusterID: cluster.Name,

		Tenants:         tenants,
		DefaultTenantID: s.Config.DefaultTenant,

		QueueConfigBatchSendDeadline: s.Config.Monitoring.QueueConfig.BatchSendDeadline,
		QueueConfigCapacity:          s.Config.Monitoring.QueueConfig.Capacity,
		QueueConfigMaxBackoff:        s.Config.Monitoring.QueueConfig.MaxBackoff,
		QueueConfigMaxSamplesPerSend: s.Config.Monitoring.QueueConfig.MaxSamplesPerSend,
		QueueConfigMaxShards:         s.Config.Monitoring.QueueConfig.MaxShards,
		QueueConfigMinBackoff:        s.Config.Monitoring.QueueConfig.MinBackoff,
		QueueConfigMinShards:         s.Config.Monitoring.QueueConfig.MinShards,
		QueueConfigRetryOnHttp429:    s.Config.Monitoring.QueueConfig.RetryOnHttp429,
		QueueConfigSampleAgeLimit:    s.Config.Monitoring.QueueConfig.SampleAgeLimit,

		WALTruncateFrequency: s.Config.Monitoring.WALTruncateFrequency.String(),

		ExternalLabels: map[string]string{
			"cluster_id":       cluster.Name,
			"cluster_type":     s.Config.Cluster.GetClusterType(cluster),
			"customer":         s.Config.Cluster.Customer,
			"installation":     s.Config.Cluster.Name,
			"organization":     org,
			"pipeline":         s.Config.Cluster.Pipeline,
			"provider":         provider,
			"region":           s.Config.Cluster.Region,
			"service_priority": monitoring.GetServicePriority(cluster),
		},

		IsWorkloadCluster:         s.Config.Cluster.IsWorkloadCluster(cluster),
		IsSupportingScrapeConfigs: observabilityBundleVersion.GE(versionSupportingScrapeConfigs),
		ExemplarsEnabled:          s.Config.Monitoring.ExemplarsEnabled,
	}

	err = alloyConfigTemplate.Execute(&values, data)
	if err != nil {
		return "", fmt.Errorf("failed to template alloy config: %w", err)
	}

	return values.String(), nil
}

func ConfigMap(cluster *clusterv1.Cluster) *v1.ConfigMap {
	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}

	return configmap
}
