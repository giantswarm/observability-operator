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
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

var (
	//go:embed templates/alloy-config.alloy.template
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

func (a *Service) GenerateAlloyMonitoringConfigMapData(ctx context.Context, currentState *v1.ConfigMap, cluster *clusterv1.Cluster, tenants []string, observabilityBundleVersion semver.Version) (map[string]string, error) {
	logger := log.FromContext(ctx)

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

	headSeries, err := querier.QueryTSDBHeadSeries(ctx, query, a.Config.Monitoring.MetricsQueryURL)
	if err != nil {
		logger.Error(err, "alloy-service - failed to query head series")
		metrics.MimirQueryErrors.WithLabelValues().Inc()
	}

	clusterShardingStrategy, err := monitoring.GetClusterShardingStrategy(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster sharding strategy: %w", err)
	}

	shardingStrategy := a.Config.Monitoring.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	shards := shardingStrategy.ComputeShards(currentShards, headSeries)

	alloyConfig, err := a.generateAlloyConfig(ctx, cluster, tenants, observabilityBundleVersion)
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

func (a *Service) generateAlloyConfig(ctx context.Context, cluster *clusterv1.Cluster, tenants []string, observabilityBundleVersion semver.Version) (string, error) {
	var values bytes.Buffer

	org, err := a.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("failed to read organization: %w", err)
	}

	provider, err := a.Config.Cluster.GetClusterProvider(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster provider: %w", err)
	}

	data := struct {
		AlloySecretName      string
		AlloySecretNamespace string

		MimirRulerAPIURLKey                   string
		MimirRemoteWriteAPIUsernameKey        string
		MimirRemoteWriteAPIPasswordKey        string
		MimirRemoteWriteAPIURLKey             string
		MimirRemoteWriteAPINameKey            string
		MimirRemoteWriteTimeout               string
		MimirRemoteWriteTLSInsecureSkipVerify bool

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
	}{
		AlloySecretName:      apps.AlloyMetricsAppName,
		AlloySecretNamespace: apps.AlloyNamespace,

		MimirRulerAPIURLKey:                   common.MimirRulerAPIURLKey,
		MimirRemoteWriteAPIUsernameKey:        common.MimirRemoteWriteAPIUsernameKey,
		MimirRemoteWriteAPIPasswordKey:        common.MimirRemoteWriteAPIPasswordKey,
		MimirRemoteWriteAPIURLKey:             common.MimirRemoteWriteAPIURLKey,
		MimirRemoteWriteAPINameKey:            common.MimirRemoteWriteAPINameKey,
		MimirRemoteWriteTimeout:               common.MimirRemoteWriteTimeout,
		MimirRemoteWriteTLSInsecureSkipVerify: a.Config.Cluster.InsecureCA,

		ClusterID: cluster.Name,

		Tenants:         tenants,
		DefaultTenantID: organization.GiantSwarmDefaultTenant,

		QueueConfigBatchSendDeadline: a.Config.Monitoring.QueueConfig.BatchSendDeadline,
		QueueConfigCapacity:          a.Config.Monitoring.QueueConfig.Capacity,
		QueueConfigMaxBackoff:        a.Config.Monitoring.QueueConfig.MaxBackoff,
		QueueConfigMaxSamplesPerSend: a.Config.Monitoring.QueueConfig.MaxSamplesPerSend,
		QueueConfigMaxShards:         a.Config.Monitoring.QueueConfig.MaxShards,
		QueueConfigMinBackoff:        a.Config.Monitoring.QueueConfig.MinBackoff,
		QueueConfigMinShards:         a.Config.Monitoring.QueueConfig.MinShards,
		QueueConfigRetryOnHttp429:    a.Config.Monitoring.QueueConfig.RetryOnHttp429,
		QueueConfigSampleAgeLimit:    a.Config.Monitoring.QueueConfig.SampleAgeLimit,

		WALTruncateFrequency: a.Config.Monitoring.WALTruncateFrequency.String(),

		ExternalLabels: map[string]string{
			"cluster_id":       cluster.Name,
			"cluster_type":     a.Config.Cluster.GetClusterType(cluster),
			"customer":         a.Config.Cluster.Customer,
			"installation":     a.Config.Cluster.Name,
			"organization":     org,
			"pipeline":         a.Config.Cluster.Pipeline,
			"provider":         provider,
			"region":           a.Config.Cluster.Region,
			"service_priority": monitoring.GetServicePriority(cluster),
		},

		IsWorkloadCluster:         a.Config.Cluster.IsWorkloadCluster(cluster),
		IsSupportingScrapeConfigs: observabilityBundleVersion.GE(versionSupportingScrapeConfigs),
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
