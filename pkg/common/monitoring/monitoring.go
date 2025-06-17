package monitoring

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

const (
	// DefaultServicePriority is the default service priority if not set.
	defaultServicePriority = "highest"
	mimirApiKey            = "mimir-basic-auth" // #nosec G101
	mimirNamespace         = "mimir"
	// ServicePriorityLabel is the label used to determine the priority of a service.
	servicePriorityLabel = "giantswarm.io/service-priority"

	// secret is created in via https://github.com/giantswarm/alloy-app/blob/main/helm/alloy/templates/secret.yaml.
	// this means the secret is created in the same namespace and with the same name as the alloy app.
	AlloyMonitoringAgentAppName      = "alloy-metrics"
	AlloyMonitoringAgentAppNamespace = "kube-system"

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

var DefaultReadTenant = "giantswarm"

func GetServicePriority(cluster *clusterv1.Cluster) string {
	if servicePriority, ok := cluster.GetLabels()[servicePriorityLabel]; ok && servicePriority != "" {
		return servicePriority
	}
	return defaultServicePriority
}

func GetMimirIngressPassword(ctx context.Context, k8sClient client.Client) (string, error) {
	secret := &corev1.Secret{}

	err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      mimirApiKey,
		Namespace: mimirNamespace,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get mimir auth secret: %w", err)
	}

	mimirPassword, err := readMimirAuthPasswordFromSecret(*secret)
	if err != nil {
		return "", fmt.Errorf("failed to read mimir auth password from secret: %w", err)
	}

	return mimirPassword, nil
}

func readMimirAuthPasswordFromSecret(secret corev1.Secret) (string, error) {
	if credentials, ok := secret.Data["credentials"]; !ok {
		return "", errors.New("credentials key not found in secret")
	} else {
		var secretData string

		err := yaml.Unmarshal(credentials, &secretData)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal mimir auth credentials: %w", err)
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
