package alertmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/template"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	common "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/metrics"
)

const (
	// AlertmanagerConfigKey is the key to the alertmanager configuration in the secret.
	AlertmanagerConfigKey = "alertmanager.yaml"
	// TemplatesSuffix is the suffix used to identify the templates in the secret.
	TemplatesSuffix = ".tmpl"

	// AlertmanagerConfigFinalizer is the finalizer added to Alertmanager config secrets.
	// It ensures the config is deleted from Mimir before the secret is garbage collected.
	AlertmanagerConfigFinalizer = "observability.giantswarm.io/alertmanager-config"

	alertmanagerAPIPath = "/api/v1/alerts"
)

// Service is the interface for configuring Alertmanager.
type Service interface {
	// ConfigureFromSecret pushes the Alertmanager configuration stored in secret to Mimir for the given tenant.
	ConfigureFromSecret(ctx context.Context, secret *v1.Secret, tenantID string) error
	// DeleteForTenant removes the Alertmanager configuration for the given tenant from Mimir.
	// It is idempotent: if no configuration exists, it returns nil.
	DeleteForTenant(ctx context.Context, tenantID string) error
}

// service is the concrete implementation of Service.
type service struct {
	alertmanagerURL string
	httpClient      *http.Client
}

// configRequest is the structure used to send the configuration to Alertmanager's API.
// json tags also apply as yaml field names.
type configRequest struct {
	TemplateFiles      map[string]string `json:"template_files"`
	AlertmanagerConfig string            `json:"alertmanager_config"`
}

func New(cfg pkgconfig.Config) Service {
	return &service{
		alertmanagerURL: strings.TrimSuffix(cfg.Monitoring.AlertmanagerURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Validate checks that the secret contains a valid Alertmanager configuration and
// that all template files parse correctly. It is the single entry point used by
// the admission webhook — callers that need the extracted data should use
// ConfigureFromSecret instead.
func Validate(secret *v1.Secret) error {
	raw, err := extractAlertmanagerConfig(secret)
	if err != nil {
		return err
	}
	if _, err := parseAlertmanagerConfig(raw); err != nil {
		return err
	}
	_, err = extractTemplates(secret)
	return err
}

func extractAlertmanagerConfig(secret *v1.Secret) ([]byte, error) {
	alertmanagerConfig, found := secret.Data[AlertmanagerConfigKey]
	if !found {
		return nil, fmt.Errorf("missing %s in alertmanager secret", AlertmanagerConfigKey)
	}
	return alertmanagerConfig, nil
}

func (s *service) ConfigureFromSecret(ctx context.Context, secret *v1.Secret, tenantID string) error {
	logger := log.FromContext(ctx)

	logger.Info("configuring alertmanager")
	if secret == nil {
		return fmt.Errorf("alertmanager secret is nil")
	}

	alertmanagerConfig, err := extractAlertmanagerConfig(secret)
	if err != nil {
		return fmt.Errorf("failed to extract alertmanager config: %w", err)
	}

	// Parse and validate the configuration.
	// The returned config is used only for metrics and not sent to alertmanager
	// as transforming it via String() would produce an invalid configuration
	// with all secrets replaced with <redacted>.
	amConfig, err := parseAlertmanagerConfig(alertmanagerConfig)
	if err != nil {
		return fmt.Errorf("failed to load alertmanager configuration: %w", err)
	}

	routeCount := countRoutes(amConfig.Route)
	metrics.AlertmanagerRoutes.WithLabelValues(tenantID).Set(float64(routeCount))
	logger.WithValues("tenant", tenantID, "routes", routeCount).Info("Updated Alertmanager routes metric")

	templates, err := extractTemplates(secret)
	if err != nil {
		return fmt.Errorf("failed to extract alertmanager templates: %w", err)
	}

	err = s.configure(ctx, alertmanagerConfig, templates, tenantID)
	if err != nil {
		metrics.MimirAlertmanagerAPIErrors.WithLabelValues("push_config").Inc()
		return fmt.Errorf("failed to configure alertmanager: %w", err)
	}

	logger.Info("configured alertmanager")
	return nil
}

// DeleteForTenant removes the Alertmanager configuration for the given tenant from Mimir.
// https://grafana.com/docs/mimir/latest/references/http-api/#delete-alertmanager-configuration
func (s *service) DeleteForTenant(ctx context.Context, tenantID string) error {
	logger := log.FromContext(ctx)

	url := s.alertmanagerURL + alertmanagerAPIPath
	logger.WithValues("url", url, "tenant", tenantID).Info("Alertmanager: deleting configuration")

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set(common.OrgIDHeader, tenantID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		metrics.MimirAlertmanagerAPIErrors.WithLabelValues("delete_config").Inc()
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	logger.WithValues("status_code", resp.StatusCode).Info("Alertmanager: delete response received")

	switch resp.StatusCode {
	case http.StatusOK:
		// Successfully deleted.
		return nil
	case http.StatusNotFound:
		// No configuration existed for this tenant — treat as success (idempotent).
		logger.Info("Alertmanager: no configuration found for tenant, treating as already deleted", "tenant", tenantID)
		return nil
	default:
		metrics.MimirAlertmanagerAPIErrors.WithLabelValues("delete_config").Inc()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unexpected status %d and failed to read response body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("failed to delete configuration: %w", APIError{Code: resp.StatusCode, Message: string(respBody)})
	}
}

func parseAlertmanagerConfig(alertmanagerConfig []byte) (*config.Config, error) {
	amConfig, err := config.Load(string(alertmanagerConfig))
	if err != nil {
		return nil, err
	}
	return amConfig, nil
}

func extractTemplates(secret *v1.Secret) (map[string]string, error) {
	// Create the engine once — mirrors Alertmanager's runtime behaviour where all
	// templates share a single namespace, so cross-template references are validated too.
	t, err := template.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create template engine: %w", err)
	}

	templates := make(map[string]string)
	for key, value := range secret.Data {
		if !strings.HasSuffix(key, TemplatesSuffix) {
			continue
		}
		// Template key/name should not be a path otherwise the request will fail with:
		// > error validating Alertmanager config: invalid template name "/etc/dummy.tmpl": the template name cannot contain any path
		baseKey := path.Base(key)
		content := string(value)

		if err := t.Parse(strings.NewReader(content)); err != nil {
			return nil, fmt.Errorf("invalid template %q: %w", baseKey, err)
		}

		templates[baseKey] = content
	}
	return templates, nil
}

// configure sends the configuration and templates to Mimir Alertmanager's API.
// It is the caller's responsibility to ensure template names are valid (no path separators)
// and that templates are referenced in the configuration.
// https://grafana.com/docs/mimir/latest/references/http-api/#set-alertmanager-configuration
func (s *service) configure(ctx context.Context, alertmanagerConfigContent []byte, templates map[string]string, tenantID string) error {
	logger := log.FromContext(ctx)

	requestData := configRequest{
		AlertmanagerConfig: string(alertmanagerConfigContent),
		TemplateFiles:      templates,
	}
	data, err := yaml.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	dataLen := len(data)

	url := s.alertmanagerURL + alertmanagerAPIPath
	logger.WithValues("url", url, "data_size", dataLen, "config_size", len(alertmanagerConfigContent), "templates_count", len(templates)).Info("Alertmanager: sending configuration")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set(common.OrgIDHeader, tenantID)
	req.Header.Set("Content-Type", "application/yaml")
	req.ContentLength = int64(dataLen)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	logger.WithValues("status_code", resp.StatusCode).Info("Alertmanager: configuration sent")

	if resp.StatusCode != http.StatusCreated {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		return fmt.Errorf("failed to send configuration: %w", APIError{Code: resp.StatusCode, Message: string(respBody)})
	}

	return nil
}

// countRoutes recursively counts the number of routes in an Alertmanager configuration.
func countRoutes(route *config.Route) int {
	if route == nil {
		return 0
	}

	count := 1
	for _, subRoute := range route.Routes {
		count += countRoutes(subRoute)
	}
	return count
}
