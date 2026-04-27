package metrics

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

func (s *Service) GenerateAlloyMonitoringSecretData(cluster *clusterv1.Cluster, caBundle string, creds credential.BackendCredentials) (map[string]string, error) {
	auth, ok := creds.Get(observabilityv1alpha1.CredentialBackendMetrics)
	if !ok {
		return nil, fmt.Errorf("metrics credentials missing for cluster %s", cluster.Name)
	}

	remoteWriteUrl := fmt.Sprintf(common.MimirRemoteWriteEndpointURLFormat, s.Config.Cluster.BaseDomain)
	mimirRulerUrl := fmt.Sprintf(common.MimirBaseURLFormat, s.Config.Cluster.BaseDomain)
	mimirQueryUrl := fmt.Sprintf(common.MimirQueryEndpointURLFormat, s.Config.Cluster.BaseDomain)

	secrets := map[string]string{
		common.MimirQueryAPIURLKey:        mimirQueryUrl,
		common.MimirRulerAPIURLKey:        mimirRulerUrl,
		common.MimirRemoteWriteAPIURLKey:  remoteWriteUrl,
		common.MimirRemoteWriteAPINameKey: common.MimirRemoteWriteName,
		common.MimirUsernameKey:           auth.Username,
		common.MimirPasswordKey:           auth.Password,
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
