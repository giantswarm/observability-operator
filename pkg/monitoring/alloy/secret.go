package alloy

import (
	"context"
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

const (
	AlloyRemoteWriteURLEnvVarName               = "REMOTE_WRITE_URL"
	AlloyRemoteWriteNameEnvVarName              = "REMOTE_WRITE_NAME"
	AlloyRemoteWriteBasicAuthUsernameEnvVarName = "BASIC_AUTH_USERNAME"
	AlloyRemoteWriteBasicAuthPasswordEnvVarName = "BASIC_AUTH_PASSWORD" // #nosec G101
)

func (a *Service) GenerateAlloyMonitoringSecretData(ctx context.Context, cluster *clusterv1.Cluster) (map[string][]byte, error) {
	url := fmt.Sprintf(commonmonitoring.RemoteWriteEndpointTemplateURL, a.ManagementCluster.BaseDomain)
	password, err := commonmonitoring.GetMimirIngressPassword(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	data := make(map[string][]byte)
	data[AlloyRemoteWriteURLEnvVarName] = []byte(url)
	data[AlloyRemoteWriteNameEnvVarName] = []byte(commonmonitoring.RemoteWriteName)
	data[AlloyRemoteWriteBasicAuthUsernameEnvVarName] = []byte(a.ManagementCluster.Name)
	data[AlloyRemoteWriteBasicAuthPasswordEnvVarName] = []byte(password)

	return data, nil
}

func Secret(cluster *clusterv1.Cluster) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, SecretName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common(),
		},
	}

	return secret
}
