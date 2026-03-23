package events

import (
	"context"
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

func (s *Service) GenerateAlloyEventsSecretData(ctx context.Context, cluster *clusterv1.Cluster, loggingEnabled bool, tracingEnabled bool, otlpMetricsEnabled bool, otlpLogsEnabled bool) (map[string]string, error) {
	secrets := map[string]string{}

	if loggingEnabled || otlpLogsEnabled {
		// Add Loki direct-write keys for the loki.write block
		if loggingEnabled {
			secrets[common.LokiURLKey] = fmt.Sprintf(common.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
			secrets[common.LokiTenantIDKey] = organization.GiantSwarmDefaultTenant
			secrets[common.LokiRulerAPIURLKey] = fmt.Sprintf(common.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)
		}
		// Add Loki OTLP URL when OTLP logs ingestion is enabled
		if otlpLogsEnabled {
			secrets[common.LokiOTLPURLKey] = fmt.Sprintf(common.LokiOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		}
		// Loki credentials (username/password) are shared by loki.write (direct events logging)
		// and otelcol.auth.basic loki_credentials (OTLP logs exporter).
		logsPassword, err := s.LogsAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get loki auth password for cluster %s: %w", cluster.Name, err)
		}
		secrets[common.LokiUsernameKey] = cluster.Name
		secrets[common.LokiPasswordKey] = logsPassword
	}

	// Add Tempo OTLP credentials if trace ingestion is enabled
	if tracingEnabled {
		tracesPassword, err := s.TracesAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get tempo auth password for cluster %s: %w", cluster.Name, err)
		}

		secrets[common.TempoUsernameKey] = cluster.Name
		secrets[common.TempoPasswordKey] = tracesPassword
		secrets[common.TempoOTLPURLKey] = fmt.Sprintf("%s:443", fmt.Sprintf(common.TempoBaseURLFormat, s.Config.Cluster.BaseDomain))
	}

	// Add Mimir OTLP credentials when OTLP metrics ingestion is enabled
	if otlpMetricsEnabled {
		mimirOTLPURL := fmt.Sprintf(common.MimirOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		metricsPassword, err := s.MetricsAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get mimir otlp password for cluster %s: %w", cluster.Name, err)
		}
		secrets[common.MimirOTLPURLKey] = mimirOTLPURL
		secrets[common.MimirUsernameKey] = cluster.Name
		secrets[common.MimirPasswordKey] = metricsPassword
	}

	return secrets, nil
}
