package monitoring

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const (
	// DefaultServicePriority is the default service priority if not set.
	defaultServicePriority = "highest"
	mimirApiKey            = "mimir-basic-auth" // #nosec G101
	mimirNamespace         = "mimir"
	// ServicePriorityLabel is the label used to determine the priority of a service.
	servicePriorityLabel = "giantswarm.io/service-priority"

	AlloyMonitoringAgentAppName = "alloy-metrics"

	// DefaultShards is the default number of shards to use.
	DefaultShards = 1

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

	RemoteWriteName                = "mimir"
	RemoteWriteEndpointTemplateURL = "https://mimir.%s/api/v1/push"
	RemoteWriteTimeout             = "60s"
)

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
