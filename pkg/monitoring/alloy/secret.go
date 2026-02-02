package alloy

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
)

const (
	mimirRulerAPIURLKey            = "mimirRulerAPIURL"
	mimirRemoteWriteAPINameKey     = "mimirRemoteWriteAPIName"
	mimirRemoteWriteAPIURLKey      = "mimirRemoteWriteAPIURL"
	mimirRemoteWriteAPIUsernameKey = "mimirRemoteWriteAPIUsername"
	mimirRemoteWriteAPIPasswordKey = "mimirRemoteWriteAPIPassword" // #nosec G101
)

var (
	//go:embed templates/monitoring-secret.yaml.template
	alloyMonitoringSecret         string
	alloyMonitoringSecretTemplate *template.Template
)

func init() {
	alloyMonitoringSecretTemplate = template.Must(template.New("monitoring-secret.yaml").Funcs(sprig.FuncMap()).Parse(alloyMonitoringSecret))
}

func (a *Service) GenerateAlloyMonitoringSecretData(ctx context.Context, cluster *clusterv1.Cluster) (map[string][]byte, error) {
	remoteWriteUrl := fmt.Sprintf(commonmonitoring.RemoteWriteEndpointURLFormat, a.Config.Cluster.BaseDomain)
	password, err := a.AuthManager.GetClusterPassword(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir auth password for cluster %s: %w", cluster.Name, err)
	}

	mimirRulerUrl := fmt.Sprintf(commonmonitoring.MimirBaseURLFormat, a.Config.Cluster.BaseDomain)

	// Build secret environment variables map
	secretEnv := map[string]string{
		mimirRulerAPIURLKey:            mimirRulerUrl,
		mimirRemoteWriteAPIURLKey:      remoteWriteUrl,
		mimirRemoteWriteAPINameKey:     commonmonitoring.RemoteWriteName,
		mimirRemoteWriteAPIUsernameKey: cluster.Name,
		mimirRemoteWriteAPIPasswordKey: password,
	}

	// Prepare template data
	templateData := struct {
		ExtraSecretEnv map[string]string
	}{
		ExtraSecretEnv: secretEnv,
	}

	// Execute template
	var values bytes.Buffer
	err = alloyMonitoringSecretTemplate.Execute(&values, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to template alloy monitoring secret: %w", err)
	}

	return map[string][]byte{
		"values": values.Bytes(),
	}, nil
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
