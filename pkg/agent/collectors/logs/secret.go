package logs

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/credential"
)

func Secret(cluster *clusterv1.Cluster) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, SecretName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (s *Service) GenerateAlloyLogsSecretData(ctx context.Context, cluster *clusterv1.Cluster, loggingEnabled bool, caBundle string) (map[string]string, error) {
	secrets := map[string]string{}

	if loggingEnabled {
		lokiURL := fmt.Sprintf(common.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
		lokiRulerAPIURL := fmt.Sprintf(common.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)

		username, password, err := s.CredentialReader.ReadPassword(ctx, cluster.Namespace, credential.ClusterCredentialName(cluster.Name, observabilityv1alpha1.CredentialBackendLogs))
		if err != nil {
			return nil, fmt.Errorf("failed to get loki auth credentials for cluster %s: %w", cluster.Name, err)
		}

		secrets[common.LokiURLKey] = lokiURL
		secrets[common.LokiTenantIDKey] = s.Config.DefaultTenant
		secrets[common.LokiUsernameKey] = username
		secrets[common.LokiPasswordKey] = password
		secrets[common.LokiRulerAPIURLKey] = lokiRulerAPIURL
	}

	if caBundle != "" {
		secrets[common.CABundleKey] = caBundle
	}

	return secrets, nil
}
