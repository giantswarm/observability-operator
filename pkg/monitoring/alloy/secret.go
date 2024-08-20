package alloy

import (
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

const (
	AlloyRemoteWriteURLEnvVarName               = "REMOTE_WRITE_URL"
	AlloyRemoteWriteNameEnvVarName              = "REMOTE_WRITE_NAME"
	AlloyRemoteWriteBasicAuthUsernameEnvVarName = "BASIC_AUTH_USERNAME"
	AlloyRemoteWriteBasicAuthPasswordEnvVarName = "BASIC_AUTH_PASSWORD" // #nosec G101
)

var (
	//go:embed templates/monitoring-secret.yaml.template
	alloyMonitoringSecret string
)

func init() {
	template.Must(template.New("monitoring-secret.yaml").Funcs(sprig.FuncMap()).Parse(alloyMonitoringSecret))
}
