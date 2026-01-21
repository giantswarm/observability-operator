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
// It takes a map of environment variable key-value pairs and returns the rendered YAML content.
func GenerateSecretData(secrets map[string]string) ([]byte, error) {
	data := struct {
		ExtraSecretEnv map[string]string
	}{
		ExtraSecretEnv: secrets,
	}

	var buf bytes.Buffer
	if err := secretTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute secret template: %w", err)
	}

	return buf.Bytes(), nil
}
