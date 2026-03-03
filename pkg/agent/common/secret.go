package common

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed templates/secret.yaml.template
var secretTemplate string
var secretTmpl *template.Template

func init() {
	secretTmpl = template.Must(template.New("secret.yaml").Funcs(sprig.FuncMap()).Parse(secretTemplate))
}

// GenerateSecretData generates the secret data for an Alloy agent using the shared template.
// It takes a map of environment variable key-value pairs and an optional extraObjects YAML string
// to include as Helm extraObjects (e.g. KEDA resources). The extraObjects content will be indented
// under the alloy.extraObjects key in the output.
func GenerateSecretData(secrets map[string]string, extraObjects string) ([]byte, error) {
	data := struct {
		ExtraSecretEnv map[string]string
		ExtraObjects   string
	}{
		ExtraSecretEnv: secrets,
		ExtraObjects:   extraObjects,
	}

	var buf bytes.Buffer
	if err := secretTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute secret template: %w", err)
	}

	return buf.Bytes(), nil
}
