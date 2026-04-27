package events

import (
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

func (s *Service) GenerateAlloyEventsSecretData(cluster *clusterv1.Cluster, loggingEnabled bool, tracingEnabled bool, monitoringEnabled bool, caBundle string, creds credential.BackendCredentials) (map[string]string, error) {
	secrets := map[string]string{}

	if loggingEnabled {
		auth, ok := creds.Get(observabilityv1alpha1.CredentialBackendLogs)
		if !ok {
			return nil, fmt.Errorf("logs credentials missing for cluster %s", cluster.Name)
		}
		secrets[common.LokiURLKey] = fmt.Sprintf(common.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.LokiTenantIDKey] = s.Config.DefaultTenant
		secrets[common.LokiRulerAPIURLKey] = fmt.Sprintf(common.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.LokiOTLPURLKey] = fmt.Sprintf(common.LokiOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.LokiUsernameKey] = auth.Username
		secrets[common.LokiPasswordKey] = auth.Password
	}

	if tracingEnabled {
		auth, ok := creds.Get(observabilityv1alpha1.CredentialBackendTraces)
		if !ok {
			return nil, fmt.Errorf("traces credentials missing for cluster %s", cluster.Name)
		}
		secrets[common.TempoUsernameKey] = auth.Username
		secrets[common.TempoPasswordKey] = auth.Password
		secrets[common.TempoOTLPURLKey] = fmt.Sprintf("%s:443", fmt.Sprintf(common.TempoBaseURLFormat, s.Config.Cluster.BaseDomain))
	}

	if monitoringEnabled {
		auth, ok := creds.Get(observabilityv1alpha1.CredentialBackendMetrics)
		if !ok {
			return nil, fmt.Errorf("metrics credentials missing for cluster %s", cluster.Name)
		}
		secrets[common.MimirOTLPURLKey] = fmt.Sprintf(common.MimirOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		secrets[common.MimirUsernameKey] = auth.Username
		secrets[common.MimirPasswordKey] = auth.Password
	}

	if caBundle != "" {
		secrets[common.CABundleKey] = caBundle
	}

	return secrets, nil
}
