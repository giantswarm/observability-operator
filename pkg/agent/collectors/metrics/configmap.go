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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/sprig/v3"
	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
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

func (s *Service) GenerateAlloyMonitoringConfigMapData(ctx context.Context, currentState *v1.ConfigMap, cluster *clusterv1.Cluster, tenants []string, observabilityBundleVersion semver.Version) (map[string]string, error) {
	logger := log.FromContext(ctx)

	// Defensive validation: This method should only be called when monitoring is enabled.
	// The controller ensures this, but we validate here to catch potential bugs.
	if !s.Config.Monitoring.IsMonitoringEnabled(cluster) {
		return nil, fmt.Errorf("cannot generate alloy monitoring config: monitoring is not enabled for cluster %s", cluster.Name)
	}

	// Get current number of shards from Alloy's config.
	// Shards here is equivalent to replicas in the Alloy controller deployment.
	var currentShards = sharding.DefaultShards
	if currentState != nil && currentState.Data != nil && currentState.Data["values"] != "" {
		var monitoringConfig monitoringConfig
		err := yaml.Unmarshal([]byte(currentState.Data["values"]), &monitoringConfig)
		if err != nil {
			logger.Info("alloy-service - failed to unmarshal current monitoring config", "error", err)
		} else {
			currentShards = monitoringConfig.Alloy.Controller.Replicas
			logger.Info("alloy-service - current number of shards", "shards", currentShards)
		}
	}

	// Compute the number of shards based on the number of series.
	query := fmt.Sprintf(`sum(max_over_time((sum(prometheus_remote_write_wal_storage_active_series{cluster_id="%s", service="%s"})by(pod))[6h:1h]))`, cluster.Name, apps.AlloyMetricsAppName)
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, query, s.Config.Monitoring.MetricsQueryURL, s.Config.DefaultTenant, s.Config.HTTP.MimirQueryTimeout)
	if err != nil {
		logger.Error(err, "alloy-service - failed to query head series")
		metrics.MimirQueryErrors.WithLabelValues().Inc()
	}

	clusterShardingStrategy, err := monitoring.GetClusterShardingStrategy(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster sharding strategy: %w", err)
	}

	shardingStrategy := s.Config.Monitoring.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	shards := shardingStrategy.ComputeShards(currentShards, headSeries)

	alloyConfig, err := s.generateAlloyConfig(ctx, cluster, tenants, observabilityBundleVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate alloy config: %w", err)
	}

	data := struct {
		AlloyConfig       string
		PriorityClassName string
		Replicas          int
	}{
		AlloyConfig:       alloyConfig,
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
