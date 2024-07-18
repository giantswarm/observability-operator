package prometheusagent

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/sharding"
)

func (pas PrometheusAgentService) buildRemoteWriteConfig(ctx context.Context,
	cluster *clusterv1.Cluster, logger logr.Logger, currentShards int) (*corev1.ConfigMap, error) {

	organization, err := pas.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to get cluster organization")
		return nil, errors.WithStack(err)
	}

	provider, err := common.GetClusterProvider(cluster)
	if err != nil {
		logger.Error(err, "failed to get cluster provider")
		return nil, errors.WithStack(err)
	}

	externalLabels := map[string]string{
		"cluster_id":       cluster.Name,
		"cluster_type":     common.GetClusterType(cluster, pas.ManagementCluster),
		"customer":         pas.ManagementCluster.Customer,
		"installation":     pas.ManagementCluster.Name,
		"organization":     organization,
		"pipeline":         pas.ManagementCluster.Pipeline,
		"provider":         provider,
		"region":           pas.ManagementCluster.Region,
		"service_priority": getServicePriority(cluster),
	}

	// Compute the number of shards based on the number of series.
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, cluster.Name)
	if err != nil {
		logger.Info(fmt.Sprintf("Warning, failed to query head series - %s", err.Error()))
		metrics.MimirQueryErrors.WithLabelValues().Inc()
	}

	clusterShardingStrategy, err := getClusterShardingStrategy(cluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	shardingStrategy := pas.MonitoringConfig.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	shards := shardingStrategy.ComputeShards(currentShards, headSeries)

	config, err := yaml.Marshal(RemoteWriteConfig{
		PrometheusAgentConfig: &PrometheusAgentConfig{
			ExternalLabels: externalLabels,
			Image: &PrometheusAgentImage{
				Tag: pas.MonitoringConfig.PrometheusVersion,
			},
			Shards:  shards,
			Version: pas.MonitoringConfig.PrometheusVersion,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
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

func getServicePriority(cluster *clusterv1.Cluster) string {
	if servicePriority, ok := cluster.GetLabels()[servicePriorityLabel]; ok && servicePriority != "" {
		return servicePriority
	}
	return defaultServicePriority
}

func getClusterShardingStrategy(cluster metav1.Object) (*sharding.Strategy, error) {
	var err error
	var scaleUpSeriesCount, scaleDownPercentage float64
	if value, ok := cluster.GetAnnotations()["monitoring.giantswarm.io/prometheus-agent-scale-up-series-count"]; ok {
		if scaleUpSeriesCount, err = strconv.ParseFloat(value, 64); err != nil {
			return nil, err
		}
	}
	if value, ok := cluster.GetAnnotations()["monitoring.giantswarm.io/prometheus-agent-scale-down-percentage"]; ok {
		if scaleDownPercentage, err = strconv.ParseFloat(value, 64); err != nil {
			return nil, err
		}
	}
	return &sharding.Strategy{
		ScaleUpSeriesCount:  scaleUpSeriesCount,
		ScaleDownPercentage: scaleDownPercentage,
	}, nil
}

func readCurrentShardsFromConfig(configMap corev1.ConfigMap) (int, error) {
	remoteWriteConfig := RemoteWriteConfig{}
	err := yaml.Unmarshal([]byte(configMap.Data["values"]), &remoteWriteConfig)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return remoteWriteConfig.PrometheusAgentConfig.Shards, nil
}
