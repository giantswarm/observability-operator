package logs

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

var (
	//go:embed templates/alloy-secret.yaml.template
	alloySecret         string
	alloySecretTemplate *template.Template
)

func init() {
	alloySecretTemplate = template.Must(template.New("alloy-secret.yaml").Funcs(sprig.FuncMap()).Parse(alloySecret))
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

func (s *Service) GenerateAlloyLogsSecretData(ctx context.Context, cluster *clusterv1.Cluster) (map[string][]byte, error) {
	lokiURL := fmt.Sprintf(commonmonitoring.LokiPushURLFormat, s.Config.Cluster.BaseDomain)
	lokiRulerAPIURL := fmt.Sprintf(commonmonitoring.LokiBaseURLFormat, s.Config.Cluster.BaseDomain)

	// Get Loki password
	logsPassword, err := s.LogsAuthManager.GetClusterPassword(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get loki auth password for cluster %s: %w", cluster.Name, err)
	}

	// Build secret environment variables map
	secretEnv := map[string]string{
		commonmonitoring.LokiURLKey:         lokiURL,
		commonmonitoring.LokiTenantIDKey:    organization.GiantSwarmDefaultTenant,
		commonmonitoring.LokiUsernameKey:    cluster.Name,
		commonmonitoring.LokiPasswordKey:    logsPassword,
		commonmonitoring.LokiRulerAPIURLKey: lokiRulerAPIURL,
	}

	// Prepare template data
	templateData := struct {
		ExtraSecretEnv map[string]string
	}{
		ExtraSecretEnv: secretEnv,
	}

	// Execute template
	var values bytes.Buffer
	err = alloySecretTemplate.Execute(&values, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute alloy secret template: %w", err)
	}

	return map[string][]byte{
		"values": values.Bytes(),
	}, nil
}
