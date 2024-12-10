//go:build integration
// +build integration

package alertmanager

import (
	"context"
	"testing"

	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

func TestAlertmanagerConfigLoad(t *testing.T) {
	const (
		// Mimir Alertmanager URL
		alertmanagerURL = "http://localhost:8080/"

		tenantID = "anonymous"
	)

	c := pkgconfig.Config{
		Monitoring: monitoring.Config{
			AlertmanagerURL: alertmanagerURL,
		},
	}
	job := New(c)

	//	BaseDomain:     "http://base",
	//	GrafanaAddress: "http://grafana",
	//	Installation:   "test-installation",
	//	MimirEnabled:   true,
	//	OpsgenieKey:    "opsgenie-key",
	//	Pipeline:       "test",
	//	Proxy:          httpproxy.FromEnvironment().ProxyFunc(),
	//	SlackApiToken:  "slack-token",
	//	SlackApiURL:    "http://slack",

	// Read alertmanager config
	//alertmanagerContent, err := os.ReadFile(alertmanagerConfigPath)
	//if err != nil {
	//	t.Fatalf("Error reading config: %v", err)
	//}
	alertmanagerContent := ""

	// Read alertmanager template
	//alertmanagerTemplate, err := os.ReadFile(templatePath)
	//if err != nil {
	//	t.Fatalf("Error reading template: %v", err)
	//}
	alertmanagerTemplate := ""

	templates := map[string]string{
		"notification-template.tmpl": alertmanagerTemplate,
	}

	job.configure(context.TODO(), []byte(alertmanagerContent), templates, tenantID, c)
	t.Logf("config: %d, template: %d\n", len(alertmanagerContent), len(alertmanagerTemplate))

	// Debug response
	//respData, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	t.Fatalf("Error reading response: %v", err)
	//}
	//t.Logf("response: %d %s\n", resp.StatusCode, respData)
}
