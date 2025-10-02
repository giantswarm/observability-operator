package alloy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/sprig/v3"
	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/config"
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

	versionSupportingVPA                = semver.MustParse("1.7.0")
	versionSupportingExtraQueryMatchers = semver.MustParse("1.9.0")

	alloyMetricsRuleLoadingFixedAppVersion            = semver.MustParse("0.10.0")
	alloyMetricsRuleLoadingFixedContainerImageVersion = semver.MustParse("1.8.3")
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
	headSeries, err := querier.QueryTSDBHeadSeries(ctx, query, a.Monitoring.MetricsQueryURL)
	if err != nil {
		logger.Error(err, "alloy-service - failed to query head series")
		metrics.MimirQueryErrors.WithLabelValues().Inc()
	}

	clusterShardingStrategy, err := commonmonitoring.GetClusterShardingStrategy(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster sharding strategy: %w", err)
	}

	shardingStrategy := a.Monitoring.DefaultShardingStrategy.Merge(clusterShardingStrategy)
	shards := shardingStrategy.ComputeShards(currentShards, headSeries)

	alloyConfig, err := a.generateAlloyConfig(ctx, cluster, tenants, observabilityBundleVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate alloy config: %w", err)
	}

	data := struct {
		AlloyConfig       string
		PriorityClassName string
		Replicas          int
		// AlloyImageTag is the tag of the Alloy controller image.
		AlloyImageTag *string
		// IsSupportingVPA indicates whether the Alloy controller supports VPA.
		IsSupportingVPA bool
	}{
		AlloyConfig:       alloyConfig,
		PriorityClassName: commonmonitoring.PriorityClassName,
		Replicas:          shards,
		// Observability bundle in older versions do not support VPA
		IsSupportingVPA: observabilityBundleVersion.GE(versionSupportingVPA),
	}

	alloyMetricsAppVersion, err := a.getAlloyMetricsAppVersion(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get alloy metrics app version: %w", err)
	}

	if alloyMetricsAppVersion.LT(alloyMetricsRuleLoadingFixedAppVersion) {
		version := fmt.Sprintf("v%s", alloyMetricsRuleLoadingFixedContainerImageVersion.String())
		data.AlloyImageTag = &version
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

	organization, err := a.Read(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("failed to read organization: %w", err)
	}

	provider, err := config.GetClusterProvider(cluster)
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

		IsSupportingExtraQueryMatchers bool
	}{
		AlloySecretName:      commonmonitoring.AlloyMonitoringAgentAppName,
		AlloySecretNamespace: commonmonitoring.AlloyMonitoringAgentAppNamespace,

		MimirRulerAPIURLKey:                   mimirRulerAPIURLKey,
		MimirRemoteWriteAPIUsernameKey:        mimirRemoteWriteAPIUsernameKey,
		MimirRemoteWriteAPIPasswordKey:        mimirRemoteWriteAPIPasswordKey,
		MimirRemoteWriteAPIURLKey:             mimirRemoteWriteAPIURLKey,
		MimirRemoteWriteAPINameKey:            mimirRemoteWriteAPINameKey,
		MimirRemoteWriteTimeout:               commonmonitoring.RemoteWriteTimeout,
		MimirRemoteWriteTLSInsecureSkipVerify: a.Cluster.InsecureCA,

		ClusterID: cluster.Name,

		Tenants:         tenants,
		DefaultTenantID: commonmonitoring.DefaultWriteTenant,

		QueueConfigBatchSendDeadline: a.Monitoring.QueueConfig.BatchSendDeadline,
		QueueConfigCapacity:          a.Monitoring.QueueConfig.Capacity,
		QueueConfigMaxBackoff:        a.Monitoring.QueueConfig.MaxBackoff,
		QueueConfigMaxSamplesPerSend: a.Monitoring.QueueConfig.MaxSamplesPerSend,
		QueueConfigMaxShards:         a.Monitoring.QueueConfig.MaxShards,
		QueueConfigMinBackoff:        a.Monitoring.QueueConfig.MinBackoff,
		QueueConfigMinShards:         a.Monitoring.QueueConfig.MinShards,
		QueueConfigRetryOnHttp429:    a.Monitoring.QueueConfig.RetryOnHttp429,
		QueueConfigSampleAgeLimit:    a.Monitoring.QueueConfig.SampleAgeLimit,

		WALTruncateFrequency: a.Monitoring.WALTruncateFrequency.String(),

		ExternalLabels: map[string]string{
			"cluster_id":       cluster.Name,
			"cluster_type":     a.Cluster.GetClusterType(cluster),
			"customer":         a.Cluster.Customer,
			"installation":     a.Cluster.Name,
			"organization":     organization,
			"pipeline":         a.Cluster.Pipeline,
			"provider":         provider,
			"region":           a.Cluster.Region,
			"service_priority": commonmonitoring.GetServicePriority(cluster),
		},

		IsWorkloadCluster:              a.Cluster.IsWorkloadCluster(cluster),
		IsSupportingExtraQueryMatchers: observabilityBundleVersion.GE(versionSupportingExtraQueryMatchers),
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

func (a *Service) getAlloyMetricsAppVersion(ctx context.Context, cluster *clusterv1.Cluster) (version semver.Version, err error) {
	// Get observability bundle app metadata.
	appMeta := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", cluster.GetName(), commonmonitoring.AlloyMonitoringAgentAppName),
		Namespace: cluster.GetNamespace(),
	}
	// Retrieve the app.
	var currentApp appv1.App
	err = a.Client.Get(ctx, appMeta, &currentApp)
	if err != nil {
		return version, fmt.Errorf("failed to get alloy metrics app: %w", err)
	}
	v, err := semver.Parse(currentApp.Spec.Version)
	if err != nil {
		return version, fmt.Errorf("failed to parse alloy metrics app version: %w", err)
	}
	return v, nil
}
