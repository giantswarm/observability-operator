//go:build integration
// +build integration

package alertmanager

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/giantswarm/prometheus-meta-operator/v2/service/controller/resource/alerting/alertmanagerconfig"
	"github.com/prometheus/alertmanager/config"
	"golang.org/x/net/http/httpproxy"
	"gopkg.in/yaml.v2"
)

const alertmanagerAPIPath = "/api/v1/alerts"

type configCompat struct {
	TemplateFiles      map[string]string `yaml:"template_files"`
	AlertmanagerConfig string            `yaml:"alertmanager_config"`
}

func TestAlertmanagerConfigLoad(t *testing.T) {
	const (
		//alertmanagerConfigPath = "alertmanager.yaml"
		templatePath = "notification-template.tmpl"

		// Mimir Alertmanager URL and path
		alertmanagerURL = "http://localhost:8080/api/v1/alerts"

		tenandID = "anonymous"
	)

	// Use alertmanagerconfig.Resource from prometheus-meta-operator
	// Re-using the same template files and rendering logic to ensure same config are generated.
	c := alertmanagerconfig.Config{
		BaseDomain:     "http://base",
		GrafanaAddress: "http://grafana",
		Installation:   "test-installation",
		MimirEnabled:   true,
		OpsgenieKey:    "opsgenie-key",
		Pipeline:       "test",
		Proxy:          httpproxy.FromEnvironment().ProxyFunc(),
		SlackApiToken:  "slack-token",
		SlackApiURL:    "http://slack",
	}
	r, err := alertmanagerconfig.New(c)
	if err != nil {
		t.Fatalf("Error instantiating resource: %v", err)
	}

	// Render alertmanager config
	alertmanagerContent, err := r.RenderAlertmanagerConfig()
	//alertmanagerContent, err := os.ReadFile(alertmanagerConfigPath)
	if err != nil {
		t.Fatalf("Error rendering config: %v", err)
	}

	// Ensure template name in alertmanager config is a name and not a path, this is to avoid following error:
	// > error validating Alertmanager config: invalid template name "/etc/dummy.tmpl": the template name cannot contain any path
	templateBase := filepath.Base(templatePath)
	alertmanagerConfig, err := config.Load(string(alertmanagerContent))
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}
	alertmanagerConfig.Templates = []string{templateBase}
	alertmanagerConfigString := alertmanagerConfig.String()

	// Render alertmanager template
	alertmanagerTemplate, err := r.RenderNotificationTemplate()
	//alertmanagerTemplate, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("Error rendering template: %v", err)
	}

	t.Logf("config: %d, template: %d\n", len(alertmanagerConfigString), len(alertmanagerTemplate))

	// Prepare request for Alertmanager API
	compact := configCompat{
		AlertmanagerConfig: string(alertmanagerConfigString),
		TemplateFiles: map[string]string{
			templateBase: string(alertmanagerTemplate),
		},
	}
	data, err := yaml.Marshal(compact)
	if err != nil {
		t.Fatalf("Error marshalling yaml: %v", err)
	}

	// Send request to Alertmanager API
	req, err := http.NewRequest("POST", alertmanagerURL, bytes.NewBuffer(data))
	//req, err := http.NewRequest("DELETE", "http://localhost:8080/api/v1/alerts", nil)
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("X-Scope-OrgID", tenandID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Debug response
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response: %v", err)
	}
	t.Logf("response: %d %s\n", resp.StatusCode, respData)
}
