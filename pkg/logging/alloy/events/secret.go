package events

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/Masterminds/sprig/v3"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

const (
	lokiURLKey              = "logging-url"
	lokiTenantIDKey         = "logging-tenant-id"
	lokiUsernameKey         = "logging-username"
	lokiPasswordKey         = "logging-password" // #nosec G101
	lokiRulerAPIURLKey      = "ruler-api-url"
	tempoTracingUsernameKey = "tracing-username"
	tempoTracingPasswordKey = "tracing-password" // #nosec G101

	lokiMaxBackoffPeriod = "10m"
	lokiRemoteTimeout    = "60s"
)

var (
	//go:embed templates/events-logger-secret.yaml.template
	alloyEventsSecret         string
	alloyEventsSecretTemplate *template.Template
)

func init() {
	alloyEventsSecretTemplate = template.Must(template.New("events-logger-secret.yaml").Funcs(sprig.FuncMap()).Parse(alloyEventsSecret))
}

func Secret(cluster *clusterv1.Cluster) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, SecretName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (a *Service) GenerateAlloyEventsSecretData(ctx context.Context, cluster *clusterv1.Cluster, tracingEnabled bool) (map[string][]byte, error) {
	lokiURL := fmt.Sprintf(commonmonitoring.LokiPushURLFormat, a.Config.Cluster.BaseDomain)
	lokiRulerURL := fmt.Sprintf(commonmonitoring.LokiBaseURLFormat, a.Config.Cluster.BaseDomain)

	// Get Loki auth credentials
	logsPassword, err := a.LogsAuthManager.GetClusterPassword(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get loki auth password for cluster %s: %w", cluster.Name, err)
	}

	// Build secret environment variables map
	secretEnv := map[string]string{
		lokiURLKey:         lokiURL,
		lokiTenantIDKey:    organization.GiantSwarmDefaultTenant,
		lokiUsernameKey:    cluster.Name,
		lokiPasswordKey:    logsPassword,
		lokiRulerAPIURLKey: lokiRulerURL,
	}

	// Add tracing credentials if tracing is enabled
	if tracingEnabled {
		tracesPassword, err := a.TracesAuthManager.GetClusterPassword(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get tempo auth password for cluster %s: %w", cluster.Name, err)
		}

		secretEnv[tempoTracingUsernameKey] = cluster.Name
		secretEnv[tempoTracingPasswordKey] = tracesPassword
	}

	// Render template with secret data
	var buf bytes.Buffer
	templateData := struct {
		ExtraSecretEnv map[string]string
	}{
		ExtraSecretEnv: secretEnv,
	}

	if err := alloyEventsSecretTemplate.Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("failed to execute events secret template: %w", err)
	}

	return map[string][]byte{
		"values": buf.Bytes(),
	}, nil
}
