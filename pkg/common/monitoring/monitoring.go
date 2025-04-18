package monitoring

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/sharding"
)

const (
	// DefaultServicePriority is the default service priority if not set.
	defaultServicePriority = "highest"
	mimirApiKey            = "mimir-basic-auth" // #nosec G101
	mimirNamespace         = "mimir"
	// ServicePriorityLabel is the label used to determine the priority of a service.
	servicePriorityLabel = "giantswarm.io/service-priority"

	AlloyMonitoringAgentAppName = "alloy-metrics"

	// Values accepted by the monitoring-agent flag
	MonitoringAgentPrometheus = "prometheus-agent"
	MonitoringAgentAlloy      = "alloy"
	// Applications name in the observability-bundle
	MonitoringPrometheusAgentAppName = "prometheusAgent"
	MonitoringAlloyAppName           = "alloyMetrics"

	PriorityClassName = "giantswarm-critical"

	QueueConfigCapacity          = 30000
	QueueConfigMaxSamplesPerSend = 150000
	QueueConfigMaxShards         = 10
	QueueConfigSampleAgeLimit    = "30m"

	RemoteWriteName              = "mimir"
	MimirBaseURLFormat           = "https://mimir.%s"
	RemoteWriteEndpointURLFormat = MimirBaseURLFormat + "/api/v1/push"
	RemoteWriteTimeout           = "60s"

	OrgIDHeader        = "X-Scope-OrgID"
	DefaultWriteTenant = "giantswarm"
)

var DefaultReadTenants = []string{"anonymous", "giantswarm"}

func GetServicePriority(cluster *clusterv1.Cluster) string {
	if servicePriority, ok := cluster.GetLabels()[servicePriorityLabel]; ok && servicePriority != "" {
		return servicePriority
	}
	return defaultServicePriority
}

func GetMimirIngressPassword(ctx context.Context) (string, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return "", err
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return "", err
	}

	secret := &corev1.Secret{}

	err = c.Get(ctx, client.ObjectKey{
		Name:      mimirApiKey,
		Namespace: mimirNamespace,
	}, secret)
	if err != nil {
		return "", err
	}

	mimirPassword, err := readMimirAuthPasswordFromSecret(*secret)

	return mimirPassword, err
}

func readMimirAuthPasswordFromSecret(secret corev1.Secret) (string, error) {
	if credentials, ok := secret.Data["credentials"]; !ok {
		return "", errors.New("credentials key not found in secret")
	} else {
		var secretData string

		err := yaml.Unmarshal(credentials, &secretData)
		if err != nil {
			return "", errors.WithStack(err)
		}
		return secretData, nil
	}
}

func GetClusterShardingStrategy(cluster metav1.Object) (*sharding.Strategy, error) {
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
