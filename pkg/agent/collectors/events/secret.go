package events

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

func (s *Service) GenerateAlloyEventsSecretData(ctx context.Context, cluster *clusterv1.Cluster, loggingEnabled bool, tracingEnabled bool, otlpMetricsEnabled bool, otlpLogsEnabled bool) (map[string]string, error) {
	secrets := map[string]string{}

	// Add Loki credentials if logging is enabled
	if loggingEnabled {
		lokiURL := fmt.Sprintf(common.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
		lokiRulerURL := fmt.Sprintf(common.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)

		// Get Loki auth credentials
		logsPassword, err := s.LogsAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get loki auth password for cluster %s: %w", cluster.Name, err)
		}

		secrets[common.LokiURLKey] = lokiURL
		secrets[common.LokiTenantIDKey] = organization.GiantSwarmDefaultTenant
		secrets[common.LokiUsernameKey] = cluster.Name
		secrets[common.LokiPasswordKey] = logsPassword
		secrets[common.LokiRulerAPIURLKey] = lokiRulerURL
	}

	// Add tracing credentials if tracing is enabled
	if tracingEnabled {
		tracesPassword, err := s.TracesAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get tempo auth password for cluster %s: %w", cluster.Name, err)
		}

		secrets[common.TempoUsernameKey] = cluster.Name
		secrets[common.TempoPasswordKey] = tracesPassword
	}

	// Add Loki OTLP URL for workload clusters when OTLP logs ingestion is enabled
	if otlpLogsEnabled && s.Config.Cluster.IsWorkloadCluster(cluster) {
		secrets[common.LokiOTLPURLKey] = fmt.Sprintf(common.LokiOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
	}

	// Add Mimir credentials for workload clusters when OTLP metrics ingestion is enabled
	if otlpMetricsEnabled && s.Config.Cluster.IsWorkloadCluster(cluster) {
		mimirOTLPURL := fmt.Sprintf(common.MimirOTLPBaseURLFormat, s.Config.Cluster.BaseDomain)
		metricsPassword, err := s.MetricsAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get mimir password for cluster %s: %w", cluster.Name, err)
		}
		secrets[common.MimirOTLPURLKey] = mimirOTLPURL
		secrets[common.MimirOTLPUsernameKey] = cluster.Name
		secrets[common.MimirOTLPPasswordKey] = metricsPassword
	}

	return secrets, nil
}
