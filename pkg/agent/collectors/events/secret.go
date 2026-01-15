package events

import (
	"context"
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

const (
	tempoTracingUsernameKey = "tracing-username"
	tempoTracingPasswordKey = "tracing-password" // #nosec G101
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

func (a *Service) GenerateAlloyEventsSecretData(ctx context.Context, cluster *clusterv1.Cluster, tracingEnabled bool) (map[string]string, error) {
	lokiURL := fmt.Sprintf(commonmonitoring.LokiPushURLFormat, a.Config.Cluster.BaseDomain)
	lokiRulerURL := fmt.Sprintf(commonmonitoring.LokiBaseURLFormat, a.Config.Cluster.BaseDomain)

	// Get Loki auth credentials
	logsPassword, err := a.LogsAuthManager.GetClusterPassword(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get loki auth password for cluster %s: %w", cluster.Name, err)
	}

	// Build secret environment variables map
	secrets := map[string]string{
		commonmonitoring.LokiURLKey:         lokiURL,
		commonmonitoring.LokiTenantIDKey:    organization.GiantSwarmDefaultTenant,
		commonmonitoring.LokiUsernameKey:    cluster.Name,
		commonmonitoring.LokiPasswordKey:    logsPassword,
		commonmonitoring.LokiRulerAPIURLKey: lokiRulerURL,
	}

	// Add tracing credentials if tracing is enabled
	if tracingEnabled {
		tracesPassword, err := a.TracesAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get tempo auth password for cluster %s: %w", cluster.Name, err)
		}

		secrets[tempoTracingUsernameKey] = cluster.Name
		secrets[tempoTracingPasswordKey] = tracesPassword
	}

	return secrets, nil
}
