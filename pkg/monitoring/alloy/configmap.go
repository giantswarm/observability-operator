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

	"github.com/Masterminds/sprig/v3"
	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/sharding"
)

var (
	//go:embed templates/alloy-config.alloy.template
	alloyConfig         string
	alloyConfigTemplate *template.Template

	//go:embed templates/monitoring-config.yaml.template
	alloyMonitoringConfig         string
	alloyMonitoringConfigTemplate *template.Template

	versionSupportingVPA = semver.MustParse("1.7.0")
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
	query := fmt.Sprintf(`sum(max_over_time((sum(prometheus_remote_write_wal_storage_active_series{cluster_id="%s", service="%s"})by(pod))[6h:1h]))`, cluster.Name, commonmonitoring.AlloyMonitoringAgentAppName)
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, query, a.MonitoringConfig.MetricsQueryURL)
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

	alloyConfig, err := a.generateAlloyConfig(ctx, cluster, tenants)
	if err != nil {
		return nil, err
	}

	data := struct {
		AlloyConfig       string
		PriorityClassName string
		Replicas          int
		SecretName        string

		IsSupportingVPA bool
	}{
		AlloyConfig:       alloyConfig,
		PriorityClassName: commonmonitoring.PriorityClassName,
		Replicas:          shards,
		SecretName:        commonmonitoring.AlloyMonitoringAgentAppName,

		// Observability bundle in older versions do not support VPA
		IsSupportingVPA: observabilityBundleVersion.GE(versionSupportingVPA),
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

func (a *Service) generateAlloyConfig(ctx context.Context, cluster *clusterv1.Cluster, tenants []string) (string, error) {
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
		RulerAPIURLEnvVarName                  string
		RemoteWriteURLEnvVarName               string
		RemoteWriteNameEnvVarName              string
		RemoteWriteBasicAuthUsernameEnvVarName string
		RemoteWriteBasicAuthPasswordEnvVarName string
		RemoteWriteTimeout                     string
		RemoteWriteTLSInsecureSkipVerify       bool

		ClusterID         string
		IsWorkloadCluster bool

		Tenants         []string
		DefaultTenantID string

		QueueConfigCapacity          int
		QueueConfigMaxSamplesPerSend int
		QueueConfigMaxShards         int
		QueueConfigSampleAgeLimit    string

		WALTruncateFrequency string

		ExternalLabels map[string]string
	}{
		RulerAPIURLEnvVarName:                  AlloyRulerAPIURLEnvVarName,
		RemoteWriteURLEnvVarName:               AlloyRemoteWriteURLEnvVarName,
		RemoteWriteNameEnvVarName:              AlloyRemoteWriteNameEnvVarName,
		RemoteWriteBasicAuthUsernameEnvVarName: AlloyRemoteWriteBasicAuthUsernameEnvVarName,
		RemoteWriteBasicAuthPasswordEnvVarName: AlloyRemoteWriteBasicAuthPasswordEnvVarName,
		RemoteWriteTimeout:                     commonmonitoring.RemoteWriteTimeout,
		RemoteWriteTLSInsecureSkipVerify:       a.ManagementCluster.InsecureCA,

		ClusterID:         cluster.Name,
		IsWorkloadCluster: common.IsWorkloadCluster(cluster, a.ManagementCluster),

		Tenants:         tenants,
		DefaultTenantID: commonmonitoring.DefaultWriteTenant,

		QueueConfigCapacity:          commonmonitoring.QueueConfigCapacity,
		QueueConfigMaxSamplesPerSend: commonmonitoring.QueueConfigMaxSamplesPerSend,
		QueueConfigMaxShards:         commonmonitoring.QueueConfigMaxShards,
		QueueConfigSampleAgeLimit:    commonmonitoring.QueueConfigSampleAgeLimit,

		WALTruncateFrequency: a.MonitoringConfig.WALTruncateFrequency.String(),

		ExternalLabels: map[string]string{
			"cluster_id":       cluster.Name,
			"cluster_type":     common.GetClusterType(cluster, a.ManagementCluster),
			"customer":         a.ManagementCluster.Customer,
			"installation":     a.ManagementCluster.Name,
			"organization":     organization,
			"pipeline":         a.ManagementCluster.Pipeline,
			"provider":         provider,
			"region":           a.ManagementCluster.Region,
			"service_priority": commonmonitoring.GetServicePriority(cluster),
		},
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
			Labels:    labels.Common,
		},
	}

	return configmap
}
