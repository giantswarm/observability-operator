package metrics

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
)

func (s *Service) GenerateAlloyMonitoringSecretData(ctx context.Context, cluster *clusterv1.Cluster, caBundle string) (map[string]string, error) {
	remoteWriteUrl := fmt.Sprintf(common.MimirRemoteWriteEndpointURLFormat, s.Config.Cluster.BaseDomain)
	password, err := s.AuthManager.GetClusterPassword(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir auth password for cluster %s: %w", cluster.Name, err)
	}

	mimirRulerUrl := fmt.Sprintf(common.MimirBaseURLFormat, s.Config.Cluster.BaseDomain)
	mimirQueryUrl := fmt.Sprintf(common.MimirQueryEndpointURLFormat, s.Config.Cluster.BaseDomain)

	secrets := map[string]string{
		common.MimirQueryAPIURLKey:        mimirQueryUrl,
		common.MimirRulerAPIURLKey:        mimirRulerUrl,
		common.MimirRemoteWriteAPIURLKey:  remoteWriteUrl,
		common.MimirRemoteWriteAPINameKey: common.MimirRemoteWriteName,
		common.MimirUsernameKey:           cluster.Name,
		common.MimirPasswordKey:           password,
	}

	if caBundle != "" {
		secrets[common.CABundleKey] = caBundle
	}

	return secrets, nil
}

func Secret(cluster *clusterv1.Cluster) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, SecretName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}

	return secret
}
