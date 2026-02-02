package logs

import (
	"context"
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
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

func (s *Service) GenerateAlloyLogsSecretData(ctx context.Context, cluster *clusterv1.Cluster, loggingEnabled bool) (map[string]string, error) {
	secrets := map[string]string{}
	// Add Loki credentials if logging is enabled
	if loggingEnabled {
		lokiURL := fmt.Sprintf(common.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
		lokiRulerAPIURL := fmt.Sprintf(common.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)

		// Get Loki password
		logsPassword, err := s.LogsAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get loki auth password for cluster %s: %w", cluster.Name, err)
		}

		// Build secret environment variables map
		secrets[common.LokiURLKey] = lokiURL
		secrets[common.LokiTenantIDKey] = organization.GiantSwarmDefaultTenant
		secrets[common.LokiUsernameKey] = cluster.Name
		secrets[common.LokiPasswordKey] = logsPassword
		secrets[common.LokiRulerAPIURLKey] = lokiRulerAPIURL
  }

	return secrets, nil
}
