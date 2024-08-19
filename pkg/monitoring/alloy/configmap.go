package alloy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
)

var (
	//go:embed templates/alloy-config.alloy.template
	alloyConfig         string
	alloyConfigTemplate *template.Template

	//go:embed templates/monitoring-config.yaml.template
	alloyMonitoringConfig         string
	alloyMonitoringConfigTemplate *template.Template
)

func init() {
	alloyConfigTemplate = template.Must(template.New("alloy-config.alloy").Funcs(sprig.FuncMap()).Parse(alloyConfig))
	alloyMonitoringConfigTemplate = template.Must(template.New("monitoring-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyMonitoringConfig))
}

func (a *Service) GenerateAlloyMonitoringConfigMapData(ctx context.Context, currentState *v1.ConfigMap, cluster *clusterv1.Cluster) (map[string]string, error) {
	logger := log.FromContext(ctx)

	// Get current number of shards from Alloy's config.
	// Shards here is equivalent to replicas in the Alloy controller deployment.
	var currentShards int
	if currentState != nil && currentState.Data != nil && currentState.Data["values"] != "" {
		var monitoringConfig MonitoringConfig
		err := yaml.Unmarshal([]byte(currentState.Data["values"]), &monitoringConfig)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		currentShards = monitoringConfig.Alloy.Controller.Replicas
		logger.Info("alloy-service - current number of shards", "shards", currentShards)
	} else {
		currentShards = commonmonitoring.DefaultShards
	}

	// Compute the number of shards based on the number of series.
	query := fmt.Sprintf(`sum(max_over_time(prometheus_remote_write_wal_storage_active_series{cluster_id="%s", component_id="prometheus.remote_write.default", service="%s"}[6h]))`, cluster.Name, commonmonitoring.AlloyMonitoringAgentAppName)
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, query)
	if err != nil {
		logger.Error(err, "alloy-service - failed to query head series")
		metrics.MimirQueryErrors.WithLabelValues().Inc()
	}

	clusterShardingStrategy, err := commonmonitoring.GetClusterShardingStrategy(cluster)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	shardingStrategy := a.MonitoringConfig.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	shards := shardingStrategy.ComputeShards(currentShards, headSeries)

	alloyConfig, err := a.generateAlloyConfig(ctx, cluster)
	if err != nil {
		return nil, err
	}

	data := struct {
		AlloyConfig       string
		PriorityClassName string
		Replicas          int
		RequestsCPU       string
		RequestsMemory    string
	}{
		AlloyConfig:       alloyConfig,
		PriorityClassName: commonmonitoring.PriorityClassName,
		Replicas:          shards,
		RequestsCPU:       commonmonitoring.AlloyRequestsCPU,
		RequestsMemory:    commonmonitoring.AlloyRequestsMemory,
	}

	var values bytes.Buffer
	err = alloyMonitoringConfigTemplate.Execute(&values, data)
	if err != nil {
		return nil, err
	}

	configMapData := make(map[string]string)
	configMapData["values"] = values.String()

	return configMapData, nil
}

func (a *Service) generateAlloyConfig(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	var values bytes.Buffer

	organization, err := a.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return "", errors.WithStack(err)
	}

	provider, err := common.GetClusterProvider(cluster)
	if err != nil {
		return "", errors.WithStack(err)
	}

	data := struct {
		RemoteWriteURLEnvVarName               string
		RemoteWriteNameEnvVarName              string
		RemoteWriteBasicAuthUsernameEnvVarName string
		RemoteWriteBasicAuthPasswordEnvVarName string
		RemoteWriteTimeout                     string
		RemoteWriteTLSInsecureSkipVerify       bool

		QueueConfigCapacity          int
		QueueConfigMaxSamplesPerSend int
		QueueConfigMaxShards         int

		ExternalLabelsClusterID       string
		ExternalLabelsClusterType     string
		ExternalLabelsCustomer        string
		ExternalLabelsInstallation    string
		ExternalLabelsOrganization    string
		ExternalLabelsPipeline        string
		ExternalLabelsProvider        string
		ExternalLabelsRegion          string
		ExternalLabelsServicePriority string
	}{
		RemoteWriteURLEnvVarName:               AlloyRemoteWriteURLEnvVarName,
		RemoteWriteNameEnvVarName:              AlloyRemoteWriteNameEnvVarName,
		RemoteWriteBasicAuthUsernameEnvVarName: AlloyRemoteWriteBasicAuthUsernameEnvVarName,
		RemoteWriteBasicAuthPasswordEnvVarName: AlloyRemoteWriteBasicAuthPasswordEnvVarName,
		RemoteWriteTimeout:                     commonmonitoring.RemoteWriteTimeout,
		RemoteWriteTLSInsecureSkipVerify:       a.ManagementCluster.InsecureCA,

		QueueConfigCapacity:          commonmonitoring.QueueConfigCapacity,
		QueueConfigMaxSamplesPerSend: commonmonitoring.QueueConfigMaxSamplesPerSend,
		QueueConfigMaxShards:         commonmonitoring.QueueConfigMaxShards,

		ExternalLabelsClusterID:       cluster.Name,
		ExternalLabelsClusterType:     common.GetClusterType(cluster, a.ManagementCluster),
		ExternalLabelsCustomer:        a.ManagementCluster.Customer,
		ExternalLabelsInstallation:    a.ManagementCluster.Name,
		ExternalLabelsOrganization:    organization,
		ExternalLabelsPipeline:        a.ManagementCluster.Pipeline,
		ExternalLabelsProvider:        provider,
		ExternalLabelsRegion:          a.ManagementCluster.Region,
		ExternalLabelsServicePriority: commonmonitoring.GetServicePriority(cluster),
	}

	err = alloyConfigTemplate.Execute(&values, data)
	if err != nil {
		return "", err
	}

	return values.String(), nil
}

func ConfigMap(cluster *clusterv1.Cluster) *v1.ConfigMap {
	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common(),
		},
	}

	return configmap
}
