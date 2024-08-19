package alloy

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

const (
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
	url := fmt.Sprintf(commonmonitoring.RemoteWriteEndpointTemplateURL, a.ManagementCluster.BaseDomain)
	password, err := commonmonitoring.GetMimirIngressPassword(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	data := []struct {
		Name  string
		Value string
	}{
		{Name: AlloyRemoteWriteURLEnvVarName, Value: base64Encode(url)},
		{Name: AlloyRemoteWriteNameEnvVarName, Value: base64Encode(commonmonitoring.RemoteWriteName)},
		{Name: AlloyRemoteWriteBasicAuthUsernameEnvVarName, Value: base64Encode(a.ManagementCluster.Name)},
		{Name: AlloyRemoteWriteBasicAuthPasswordEnvVarName, Value: base64Encode(password)},
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
			Labels:    labels.Common(),
		},
	}

	return secret
}

func base64Encode(value string) string {
	v := []byte(value)
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(v)))
	base64.StdEncoding.Encode(dst, v)
	return string(dst)
}
