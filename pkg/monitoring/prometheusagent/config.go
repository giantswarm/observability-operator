package prometheusagent

import (
	"context"
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/shards"
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

	shards, err := getShardsCountForCluster(ctx, cluster, currentShards)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	config, err := yaml.Marshal(RemoteWriteConfig{
		PrometheusAgentConfig: &PrometheusAgentConfig{
			ExternalLabels: externalLabels,
			Image: &PrometheusAgentImage{
				Tag: pas.PrometheusVersion,
			},
			Shards:  shards,
			Version: pas.PrometheusVersion,
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
			Finalizers: []string{
				monitoring.MonitoringFinalizer,
			},
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

// We want to compute the number of shards based on the number of series.
func getShardsCountForCluster(ctx context.Context, cluster *clusterv1.Cluster, currentShardCount int) (int, error) {
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, cluster.Name)
	if err != nil {
		// If Prometheus is not accessible (DNSError), or if we don't have any data yet (ErrNoTimeSeries)
		// Then, return the default number of shards.
		var dnsError *net.DNSError
		if errors.As(err, &dnsError) || errors.Is(err, querier.ErrorNoTimeSeries) {
			return shards.ComputeShards(currentShardCount, defaultShards), nil
		}
		return 0, errors.WithStack(err)
	}
	return shards.ComputeShards(currentShardCount, headSeries), nil
}

func readCurrentShardsFromConfig(configMap corev1.ConfigMap) (int, error) {
	remoteWriteConfig := RemoteWriteConfig{}
	err := yaml.Unmarshal([]byte(configMap.Data["values"]), &remoteWriteConfig)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return remoteWriteConfig.PrometheusAgentConfig.Shards, nil
}
