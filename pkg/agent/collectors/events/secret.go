package events

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

func (s *Service) GenerateAlloyEventsSecretData(ctx context.Context, cluster *clusterv1.Cluster, loggingEnabled bool, tracingEnabled bool, monitoringEnabled bool, caBundle string) (map[string]string, error) {
	secrets := map[string]string{}

	if loggingEnabled {
		username, password, err := s.CredentialReader.ReadPassword(ctx, cluster.Namespace, credential.ClusterCredentialName(cluster.Name, observabilityv1alpha1.CredentialBackendLogs))
		if err != nil {
			return nil, fmt.Errorf("failed to get loki auth credentials for cluster %s: %w", cluster.Name, err)
		}
		secrets[common.LokiURLKey] = fmt.Sprintf(common.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.LokiTenantIDKey] = s.Config.DefaultTenant
		secrets[common.LokiRulerAPIURLKey] = fmt.Sprintf(common.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.LokiOTLPURLKey] = fmt.Sprintf(common.LokiOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.LokiUsernameKey] = username
		secrets[common.LokiPasswordKey] = password
	}

	if tracingEnabled {
		username, password, err := s.CredentialReader.ReadPassword(ctx, cluster.Namespace, credential.ClusterCredentialName(cluster.Name, observabilityv1alpha1.CredentialBackendTraces))
		if err != nil {
			return nil, fmt.Errorf("failed to get tempo auth credentials for cluster %s: %w", cluster.Name, err)
		}
		secrets[common.TempoUsernameKey] = username
		secrets[common.TempoPasswordKey] = password
		secrets[common.TempoOTLPURLKey] = fmt.Sprintf("%s:443", fmt.Sprintf(common.TempoBaseURLFormat, s.Config.Cluster.BaseDomain))
	}

	if monitoringEnabled {
		username, password, err := s.CredentialReader.ReadPassword(ctx, cluster.Namespace, credential.ClusterCredentialName(cluster.Name, observabilityv1alpha1.CredentialBackendMetrics))
		if err != nil {
			return nil, fmt.Errorf("failed to get mimir otlp auth credentials for cluster %s: %w", cluster.Name, err)
		}
		secrets[common.MimirOTLPURLKey] = fmt.Sprintf(common.MimirOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.MimirUsernameKey] = username
		secrets[common.MimirPasswordKey] = password
	}

	if caBundle != "" {
		secrets[common.CABundleKey] = caBundle
	}

	return secrets, nil
}
