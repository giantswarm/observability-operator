package prometheusagent

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"

	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

func GetPrometheusAgentRemoteWriteSecretName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-remote-write-secret", cluster.Name)
}

// buildRemoteWriteSecret builds the secret that contains the remote write configuration for the Prometheus agent.
func (pas PrometheusAgentService) buildRemoteWriteSecret(ctx context.Context,
	cluster *clusterv1.Cluster) (*corev1.Secret, error) {
	url := fmt.Sprintf(commonmonitoring.RemoteWriteEndpointURLFormat, pas.BaseDomain)

	password, err := commonmonitoring.GetMimirIngressPassword(ctx, pas.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir ingress password: %w", err)
	}

	config := RemoteWriteConfig{
		PrometheusAgentConfig: &PrometheusAgentConfig{
			RemoteWrite: []*RemoteWrite{
				{
					RemoteWriteSpec: promv1.RemoteWriteSpec{
						URL:           url,
						Name:          ptr.To(commonmonitoring.RemoteWriteName),
						RemoteTimeout: ptr.To(promv1.Duration(commonmonitoring.RemoteWriteTimeout)),
						QueueConfig: &promv1.QueueConfig{
							Capacity:          commonmonitoring.QueueConfigCapacity,
							MaxSamplesPerSend: commonmonitoring.QueueConfigMaxSamplesPerSend,
							MaxShards:         commonmonitoring.QueueConfigMaxShards,
						},
						TLSConfig: &promv1.TLSConfig{
							SafeTLSConfig: promv1.SafeTLSConfig{
								InsecureSkipVerify: &pas.InsecureCA,
							},
						},
						Headers: map[string]string{
							commonmonitoring.OrgIDHeader: commonmonitoring.DefaultWriteTenant,
						},
					},
					Username: pas.Name,
					Password: password,
				},
			},
		},
	}

	marshalledValues, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal remote write config: %w", err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPrometheusAgentRemoteWriteSecretName(cluster),
			Namespace: cluster.Namespace,
		},
		Data: map[string][]byte{
			"values": marshalledValues,
		},
		Type: "Opaque",
	}, nil
}
