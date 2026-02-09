package metrics

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
)

var (
	//go:embed templates/keda-objects.yaml.template
	kedaObjectsTemplate     string
	kedaObjectsTemplateParsed *template.Template
)

func init() {
	kedaObjectsTemplateParsed = template.Must(template.New("keda-objects.yaml").Funcs(sprig.FuncMap()).Parse(kedaObjectsTemplate))
}

type kedaTemplateData struct {
	SecretName  string
	Namespace   string
	UsernameKey string
	PasswordKey string
	Username    string
	Password    string
}

// generateKEDAExtraObjects generates the YAML content for KEDA extraObjects.
// It always creates a ClusterTriggerAuthentication for Mimir authentication.
// When kedaNamespace is set, it also creates a copy of the credentials Secret
// in that namespace so KEDA can resolve it.
func generateKEDAExtraObjects(kedaNamespace string, secretData map[string]string) (string, error) {
	data := kedaTemplateData{
		SecretName:  apps.AlloyMetricsAppName,
		Namespace:   kedaNamespace,
		UsernameKey: common.MimirRemoteWriteAPIUsernameKey,
		PasswordKey: common.MimirRemoteWriteAPIPasswordKey,
		Username:    secretData[common.MimirRemoteWriteAPIUsernameKey],
		Password:    secretData[common.MimirRemoteWriteAPIPasswordKey],
	}

	var buf bytes.Buffer
	if err := kedaObjectsTemplateParsed.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render KEDA extra objects template: %w", err)
	}

	return buf.String(), nil
}
