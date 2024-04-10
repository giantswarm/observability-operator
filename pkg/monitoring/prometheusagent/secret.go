package prometheusagent

import (
	"fmt"

	"github.com/pkg/errors"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

func getPrometheusAgentRemoteWriteSecretName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-remote-write-secret", cluster.Name)
}

// buildRemoteWriteSecret builds the secret that contains the remote write configuration for the Prometheus agent.
func (pas PrometheusAgentService) buildRemoteWriteSecret(
	cluster *clusterv1.Cluster, password string) (*corev1.Secret, error) {

	url := fmt.Sprintf(remoteWriteEndpointTemplateURL, pas.ManagementCluster.BaseDomain, cluster.Name)
	config := RemoteWriteConfig{
		PrometheusAgentConfig: &PrometheusAgentConfig{
			RemoteWrite: []*RemoteWrite{
				{
					RemoteWriteSpec: promv1.RemoteWriteSpec{
						URL:           url,
						Name:          remoteWriteName,
						RemoteTimeout: "60s",
						QueueConfig: &promv1.QueueConfig{
							Capacity:          30000,
							MaxSamplesPerSend: 150000,
							MaxShards:         10,
						},
						TLSConfig: &promv1.TLSConfig{
							SafeTLSConfig: promv1.SafeTLSConfig{
								InsecureSkipVerify: pas.ManagementCluster.InsecureCA,
							},
						},
					},
					Username: cluster.Name,
					Password: password,
				},
			},
		},
	}

	marshalledValues, err := yaml.Marshal(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPrometheusAgentRemoteWriteSecretName(cluster),
			Namespace: cluster.Namespace,
			Finalizers: []string{
				monitoring.MonitoringFinalizer,
			},
		},
		Data: map[string][]byte{
			"values": marshalledValues,
		},
		Type: "Opaque",
	}, nil
}

func readRemoteWritePasswordFromSecret(secret corev1.Secret) (string, error) {
	remoteWriteConfig := RemoteWriteConfig{}
	err := yaml.Unmarshal(secret.Data["values"], &remoteWriteConfig)
	if err != nil {
		return "", errors.WithStack(err)
	}

	for _, rw := range remoteWriteConfig.PrometheusAgentConfig.RemoteWrite {
		// We read the secret from the remote write configuration named `prometheus-meta-operator` only
		// as this secret is generated per cluster.
		// This will eventually be taken care of by the multi-tenancy contoller
		if rw.Name == remoteWriteName {
			return rw.Password, nil
		}
	}

	return "", errors.New("remote write password not found in secret")
}
