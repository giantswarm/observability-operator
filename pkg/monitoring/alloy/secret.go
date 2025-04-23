package alloy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

const (
	AlloyRulerAPIURLEnvVarName                  = "RULER_API_URL"
	AlloyRemoteWriteURLEnvVarName               = "REMOTE_WRITE_URL"
	AlloyRemoteWriteNameEnvVarName              = "REMOTE_WRITE_NAME"
	AlloyRemoteWriteBasicAuthUsernameEnvVarName = "BASIC_AUTH_USERNAME"
	AlloyRemoteWriteBasicAuthPasswordEnvVarName = "BASIC_AUTH_PASSWORD" // #nosec G101
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
	remoteWriteUrl := fmt.Sprintf(commonmonitoring.RemoteWriteEndpointURLFormat, a.BaseDomain)
	password, err := commonmonitoring.GetMimirIngressPassword(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mimirRulerUrl := fmt.Sprintf(commonmonitoring.MimirBaseURLFormat, a.BaseDomain)

	data := []struct {
		Name  string
		Value string
	}{
		{Name: AlloyRulerAPIURLEnvVarName, Value: mimirRulerUrl},
		{Name: AlloyRemoteWriteURLEnvVarName, Value: remoteWriteUrl},
		{Name: AlloyRemoteWriteNameEnvVarName, Value: commonmonitoring.RemoteWriteName},
		{Name: AlloyRemoteWriteBasicAuthUsernameEnvVarName, Value: a.Name},
		{Name: AlloyRemoteWriteBasicAuthPasswordEnvVarName, Value: password},
	}

	var values bytes.Buffer
	err = alloyMonitoringSecretTemplate.Execute(&values, data)
	if err != nil {
		return nil, err
	}

	secretData := make(map[string][]byte)
	secretData["values"] = values.Bytes()

	return secretData, nil
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
