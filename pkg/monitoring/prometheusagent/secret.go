package prometheusagent

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const (
	mimirApiKey    = "mimir-basic-auth"
	mimirNamespace = "mimir"
)

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

func getPrometheusAgentRemoteWriteSecretName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-remote-write-secret", cluster.Name)
}

// buildRemoteWriteSecret builds the secret that contains the remote write configuration for the Prometheus agent.
func (pas PrometheusAgentService) buildRemoteWriteSecret(ctx context.Context,
	cluster *clusterv1.Cluster) (*corev1.Secret, error) {
	url := fmt.Sprintf(remoteWriteEndpointTemplateURL, pas.ManagementCluster.BaseDomain)
	password, err := GetMimirIngressPassword(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

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
					Username: pas.ManagementCluster.Name,
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
		},
		Data: map[string][]byte{
			"values": marshalledValues,
		},
		Type: "Opaque",
	}, nil
}

func readMimirAuthPasswordFromSecret(secret corev1.Secret) (string, error) {
	var secretData string

	err := yaml.Unmarshal(secret.Data["credentials"], &secretData)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return secretData, nil
}
