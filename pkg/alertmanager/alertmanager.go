package alertmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/prometheus/alertmanager/config"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	common "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
)

const (
	// Those values are used to retrieve the Alertmanager configuration from the secret named after conf.Monitoring.AlertmanagerSecretName
	// AlertmanagerConfigKey is the key to the alertmanager configuration in the secret
	AlertmanagerConfigKey = "alertmanager.yaml"
	// TemplatesSuffix is the suffix used to identify the templates in the secret
	TemplatesSuffix = ".tmpl"

	alertmanagerAPIPath = "/api/v1/alerts"
)

type Service struct {
	alertmanagerURL string
}

// configRequest is the structure used to send the configuration to Alertmanager's API
// json tags also applies yaml field names
type configRequest struct {
	TemplateFiles      map[string]string `json:"template_files"`
	AlertmanagerConfig string            `json:"alertmanager_config"`
}

func New(conf pkgconfig.Config) Service {
	service := Service{
		alertmanagerURL: strings.TrimSuffix(conf.Monitoring.AlertmanagerURL, "/"),
	}

	return service
}

func ExtractAlertmanagerConfig(ctx context.Context, secret *v1.Secret) ([]byte, error) {
	// Check that the secret contains an Alertmanager configuration file.
	alertmanagerConfig, found := secret.Data[AlertmanagerConfigKey]
	if !found {
		return nil, fmt.Errorf("missing %s in the secret", AlertmanagerConfigKey)
	}
	// Validate Alertmanager configuration
	// The returned config is not used, as transforming it via String() would produce an invalid configuration with all secrets replaced with <redacted>.
	_, err := config.Load(string(alertmanagerConfig))
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return alertmanagerConfig, nil
}

func (s Service) Configure(ctx context.Context, secret *v1.Secret, tenantID string) error {
	logger := log.FromContext(ctx)

	logger.Info("configuring alertmanager")
	if secret == nil {
		return fmt.Errorf("failed to get secret: secret is nil")
	}

	// Retrieve and Validate alertmanager configuration from secret
	alertmanagerConfig, err := ExtractAlertmanagerConfig(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed to extract alertmanager config: %w", err)
	}

	// Retrieve all alertmanager templates from secret
	templates := make(map[string]string)
	// TODO Validate templates (and add it in the validating webhook)
	for key, value := range secret.Data {
		if strings.HasSuffix(key, TemplatesSuffix) {
			// Template key/name should not be a path otherwise the request will fail with:
			// > error validating Alertmanager config: invalid template name "/etc/dummy.tmpl": the template name cannot contain any path
			baseKey := path.Base(key)
			templates[baseKey] = string(value)
		}
	}

	err = s.configure(ctx, alertmanagerConfig, templates, tenantID)
	if err != nil {
		return fmt.Errorf("failed to configure alertmanager: %w", err)
	}

	logger.Info("configured alertmanager")
	return nil
}

// configure sends the configuration and templates to Mimir Alertmanager's API
// It is the caller responsibility to make sure templates names are valid (do not contain any path), and that templates are referenced in the configuration.
// https://grafana.com/docs/mimir/latest/references/http-api/#set-alertmanager-configuration
func (s Service) configure(ctx context.Context, alertmanagerConfigContent []byte, templates map[string]string, tenantID string) error {
	logger := log.FromContext(ctx)

	// Prepare request for Alertmanager API
	requestData := configRequest{
		AlertmanagerConfig: string(alertmanagerConfigContent),
		TemplateFiles:      templates,
	}
	data, err := yaml.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("alertmanager: failed to marshal yaml: %w", err)
	}
	dataLen := len(data)

	url := s.alertmanagerURL + alertmanagerAPIPath
	logger.WithValues("url", url, "data_size", dataLen, "config_size", len(alertmanagerConfigContent), "templates_count", len(templates)).Info("Alertmanager: sending configuration")

	// Send request to Alertmanager's API
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("alertmanager: failed to create request: %w", err)
	}
	req.Header.Set(common.OrgIDHeader, tenantID)
	req.ContentLength = int64(dataLen)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("alertmanager: failed to send request: %w", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	logger.WithValues("status_code", resp.StatusCode).Info("Alertmanager: configuration sent")

	if resp.StatusCode != http.StatusCreated {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("alertmanager: failed to read response: %w", err)
		}

		e := APIError{
			Code:    resp.StatusCode,
			Message: string(respBody),
		}

		return fmt.Errorf("alertmanager: failed to send configuration: %w", e)
	}

	return nil
}
