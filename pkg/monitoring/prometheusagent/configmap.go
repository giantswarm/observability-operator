package prometheusagent

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"

	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
)

func (pas PrometheusAgentService) buildRemoteWriteConfig(ctx context.Context,
	cluster *clusterv1.Cluster, logger logr.Logger, currentShards int) (*corev1.ConfigMap, error) {

	organization, err := pas.Read(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster organization: %w", err)
	}

	provider, err := config.GetClusterProvider(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster provider: %w", err)
	}

	externalLabels := map[string]string{
		"cluster_id":       cluster.Name,
		"cluster_type":     pas.Cluster.GetClusterType(cluster),
		"customer":         pas.Cluster.Customer,
		"installation":     pas.Cluster.Name,
		"organization":     organization,
		"pipeline":         pas.Cluster.Pipeline,
		"provider":         provider,
		"region":           pas.Cluster.Region,
		"service_priority": commonmonitoring.GetServicePriority(cluster),
	}

	// Compute the number of shards based on the number of series.
	query := fmt.Sprintf(`sum(max_over_time((sum(prometheus_agent_active_series{cluster_id="%s"})by(pod))[6h:1h]))`, cluster.Name)
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, query, pas.Monitoring.MetricsQueryURL)
	if err != nil {
		metrics.MimirQueryErrors.WithLabelValues().Inc()
		return nil, fmt.Errorf("failed to query head series: %w", err)
	}

	clusterShardingStrategy, err := commonmonitoring.GetClusterShardingStrategy(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster sharding strategy: %w", err)
	}

	shardingStrategy := pas.Monitoring.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	shards := shardingStrategy.ComputeShards(currentShards, headSeries)

	config, err := yaml.Marshal(RemoteWriteConfig{
		PrometheusAgentConfig: &PrometheusAgentConfig{
			ExternalLabels: externalLabels,
			Image: &PrometheusAgentImage{
				Tag: pas.Monitoring.PrometheusVersion,
			},
			Shards:  shards,
			Version: pas.Monitoring.PrometheusVersion,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal remote write config: %w", err)
	}

	if currentShards < shards {
		logger.Info("scaling up shards", "old", currentShards, "new", shards)
	} else if currentShards > shards {
		logger.Info("scaling down shards", "old", currentShards, "new", shards)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
			Namespace: cluster.Namespace,
		},
		Data: map[string]string{
			"values": string(config),
		},
	}, nil
}

func getPrometheusAgentRemoteWriteConfigName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-remote-write-config", cluster.Name)
}

func readCurrentShardsFromConfig(configMap corev1.ConfigMap) (int, error) {
	remoteWriteConfig := RemoteWriteConfig{}
	err := yaml.Unmarshal([]byte(configMap.Data["values"]), &remoteWriteConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal remote write config: %w", err)
	}

	return remoteWriteConfig.PrometheusAgentConfig.Shards, nil
}
