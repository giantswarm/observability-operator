package prometheusagent

import (
	"context"
	"fmt"
	"net"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/remotewrite"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/shards"
)

const (
	// defaultServicePriority is the default service priority if not set.
	defaultServicePriority string = "highest"
	// defaultShards is the default number of shards to use.
	defaultShards = 1

	// servicePriorityLabel is the label used to determine the priority of a service.
	servicePriorityLabel string = "giantswarm.io/service-priority"
)

type PrometheusAgentService struct {
	client.Client
	organization.OrganizationRepository
	common.ManagementCluster
	PrometheusVersion string
}

// ensurePrometheusAgentRemoteWriteConfig ensures that the prometheus remote write config is present in the cluster.
func (pas *PrometheusAgentService) ReconcilePrometheusAgentRemoteWriteConfig(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)
	logger.Info("ensuring prometheus remote write config")

	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
		Namespace: cluster.GetNamespace(),
	}

	current := &corev1.ConfigMap{}
	// Get the current configmap if it exists.
	err := pas.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		configMap, err := pas.buildRemoteWriteConfig(ctx, cluster, logger, defaultShards)
		if err != nil {
			return errors.WithStack(err)
		}

		err = pas.Client.Create(ctx, configMap)
		return errors.WithStack(err)
	} else if err != nil {
		return errors.WithStack(err)
	}

	currentShards, err := readCurrentShardsFromConfig(*current)
	if err != nil {
		return errors.WithStack(err)
	}

	desired, err := pas.buildRemoteWriteConfig(ctx, cluster, logger, currentShards)
	if err != nil {
		return errors.WithStack(err)
	}

	if !reflect.DeepEqual(current.Data, desired.Data) {
		err = pas.Client.Patch(ctx, current, client.MergeFrom(desired))
		if err != nil {
			return errors.WithStack(err)
		}
	}

	logger.Info("ensured prometheus remote write config")

	return nil
}

func (pas *PrometheusAgentService) DeletePrometheusAgentRemoteWriteConfig(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)
	logger.Info("deleting prometheus remote write config")

	objectKey := client.ObjectKey{
		Name:      getPrometheusAgentRemoteWriteConfigName(cluster),
		Namespace: cluster.GetNamespace(),
	}

	current := &corev1.ConfigMap{}
	// Get the current configmap if it exists.
	err := pas.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// We ignore cases where the configmap is not found (it it was manually deleted for instance)
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	desired := current.DeepCopy()
	// Delete the finalizer
	controllerutil.RemoveFinalizer(desired, monitoring.MonitoringFinalizer)
	err = pas.Client.Patch(ctx, current, client.MergeFrom(desired))
	if err != nil {
		return errors.WithStack(err)
	}

	err = pas.Client.Delete(ctx, desired)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("deleted prometheus remote write config")

	return nil
}

func (pas PrometheusAgentService) buildRemoteWriteConfig(ctx context.Context, cluster *clusterv1.Cluster, logger logr.Logger, currentShards int) (*corev1.ConfigMap, error) {
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

	clusterType := "workload_cluster"
	if val, ok := cluster.Labels["cluster.x-k8s.io/cluster-name"]; ok && val == pas.ManagementCluster.Name {
		clusterType = "management_cluster"
	}

	externalLabels := map[string]string{
		"cluster_id":       cluster.Name,
		"cluster_type":     clusterType,
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

	config, err := yaml.Marshal(remotewrite.RemoteWriteConfig{
		PrometheusAgentConfig: remotewrite.PrometheusAgentConfig{
			ExternalLabels: externalLabels,
			Image: remotewrite.PrometheusAgentImage{
				Tag: pas.PrometheusVersion,
			},
			Shards:  shards,
			Version: pas.PrometheusVersion,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
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

func readCurrentShardsFromConfig(configMap corev1.ConfigMap) (int, error) {
	remoteWriteConfig := remotewrite.RemoteWriteConfig{}
	err := yaml.Unmarshal([]byte(configMap.Data["values"]), &remoteWriteConfig)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return remoteWriteConfig.PrometheusAgentConfig.Shards, nil
}

// We want to compute the number of shards based on the number of nodes.
func getShardsCountForCluster(ctx context.Context, cluster *clusterv1.Cluster, currentShardCount int) (int, error) {
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, cluster.Name)
	if err != nil {
		// If prometheus is not accessible (for instance, not running because this is a new cluster, we check if prometheus is accessible)
		var dnsError *net.DNSError
		if errors.As(err, &dnsError) {
			return shards.ComputeShards(currentShardCount, defaultShardCount), nil
		}
		return 0, errors.WithStack(err)
	}
	return shards.ComputeShards(currentShardCount, headSeries), nil
}

func getServicePriority(cluster *clusterv1.Cluster) string {
	if servicePriority, ok := cluster.GetLabels()[servicePriorityLabel]; ok && servicePriority != "" {
		return servicePriority
	}
	return defaultServicePriority
}
