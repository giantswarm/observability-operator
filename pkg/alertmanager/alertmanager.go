package alertmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/config"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	common "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
)

const (
	// Those values are used to retrieve the Alertmanager configuration from the secret named after conf.Monitoring.AlertmanagerSecretName
	// alertmanagerConfigKey is the key to the alertmanager configuration in the secret
	alertmanagerConfigKey = "alertmanager.yaml"
	// templatesSuffix is the suffix used to identify the templates in the secret
	templatesSuffix = ".tmpl"

	alertmanagerAPIPath = "/api/v1/alerts"

	rulerTenantDeletionUrl = "http://mimir-gateway.mimir.svc/ruler/delete_tenant_config"
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

func (s Service) Configure(ctx context.Context, secret *v1.Secret) error {
	logger := log.FromContext(ctx)

	logger.Info("Alertmanager: configuring")

	if secret == nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to get secret"))
	}

	// Retrieve Alertmanager configuration from secret
	alertmanagerConfigContent, ok := secret.Data[alertmanagerConfigKey]
	if !ok {
		return errors.WithStack(fmt.Errorf("alertmanager: config not found"))
	}

	// Retrieve all alertmanager templates from secret
	templates := make(map[string]string)
	for key, value := range secret.Data {
		if strings.HasSuffix(key, templatesSuffix) {
			// Template key/name should not be a path otherwise the request will fail with:
			// > error validating Alertmanager config: invalid template name "/etc/dummy.tmpl": the template name cannot contain any path
			baseKey := path.Base(key)
			templates[baseKey] = string(value)
		}
	}

	err := s.configure(ctx, alertmanagerConfigContent, templates, monitoring.DefaultWriteTenant)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to configure: %w", err))
	}

	logger.Info("Alertmanager: configured")

	// TODO Clean up the anonymous tenant alertmanager deletion code in the next release
	logger.Info("Alertmanager: cleaning up anonymous tenant")

	err = s.deleteTenantConfiguration(ctx, "anonymous")
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to delete the alertmanager anonymous tenant configuration: %w", err))
	}

	err = s.deleteRules(ctx, "anonymous")
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to delete rules for the anonymous tenant: %w", err))
	}

	logger.Info("Alertmanager: anonymous tenant cleaned up")
	return nil
}

// configure sends the configuration and templates to Mimir Alertmanager's API
// It is the caller responsibility to make sure templates names are valid (do not contain any path), and that templates are referenced in the configuration.
// https://grafana.com/docs/mimir/latest/references/http-api/#set-alertmanager-configuration
func (s Service) configure(ctx context.Context, alertmanagerConfigContent []byte, templates map[string]string, tenantID string) error {
	logger := log.FromContext(ctx)

	// Validate Alertmanager configuration
	// The returned config is not used, as transforming it via String() would produce an invalid configuration with all secrets replaced with <redacted>.
	_, err := config.Load(string(alertmanagerConfigContent))
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to load configuration: %w", err))
	}

	// Prepare request for Alertmanager API
	requestData := configRequest{
		AlertmanagerConfig: string(alertmanagerConfigContent),
		TemplateFiles:      templates,
	}
	data, err := yaml.Marshal(requestData)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to marshal yaml: %w", err))
	}
	dataLen := len(data)

	url := s.alertmanagerURL + alertmanagerAPIPath
	logger.WithValues("url", url, "data_size", dataLen, "config_size", len(alertmanagerConfigContent), "templates_count", len(templates)).Info("Alertmanager: sending configuration")

	// Send request to Alertmanager's API
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to create request: %w", err))
	}
	req.Header.Set(common.OrgIDHeader, tenantID)
	req.ContentLength = int64(dataLen)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to send request: %w", err))
	}
	defer resp.Body.Close() // nolint: errcheck

	logger.WithValues("status_code", resp.StatusCode).Info("Alertmanager: configuration sent")

	if resp.StatusCode != http.StatusCreated {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.WithStack(fmt.Errorf("alertmanager: failed to read response: %w", err))
		}

		e := APIError{
			Code:    resp.StatusCode,
			Message: string(respBody),
		}

		return errors.WithStack(fmt.Errorf("alertmanager: failed to send configuration: %w", e))
	}

	return nil
}

// deleteTenantConfiguration deletes a tenant using the Mimir Alertmanager's API
func (s Service) deleteTenantConfiguration(ctx context.Context, tenantID string) error {
	logger := log.FromContext(ctx)

	url := s.alertmanagerURL + alertmanagerAPIPath
	logger.WithValues("url", url, "tenant", tenantID).Info("Alertmanager: deleting tenant")

	// Send request to Alertmanager's API
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to create request: %w", err))
	}

	req.Header.Set(common.OrgIDHeader, tenantID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to send request: %w", err))
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.WithStack(fmt.Errorf("alertmanager: failed to read response: %w", err))
		}

		e := APIError{
			Code:    resp.StatusCode,
			Message: string(respBody),
		}

		return errors.WithStack(fmt.Errorf("alertmanager: failed to delete tenant configuration: %w", e))
	}

	logger.WithValues("status_code", resp.StatusCode).Info("Alertmanager: tenant configuration deleted")

	return nil
}

// deleteRules deletes all rules for a tenant using the Mimir Alertmanager's API
func (s Service) deleteRules(ctx context.Context, tenantID string) error {
	logger := log.FromContext(ctx)

	logger.WithValues("url", rulerTenantDeletionUrl, "tenant", tenantID).Info("Alertmanager: delete rules for tenant")

	// Send request to Alertmanager's API
	req, err := http.NewRequest(http.MethodPost, rulerTenantDeletionUrl, nil)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to create request: %w", err))
	}

	req.Header.Set(common.OrgIDHeader, tenantID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to send request: %w", err))
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.WithStack(fmt.Errorf("alertmanager: failed to read response: %w", err))
		}

		e := APIError{
			Code:    resp.StatusCode,
			Message: string(respBody),
		}

		return errors.WithStack(fmt.Errorf("alertmanager: failed to delete rules for tenant: %w", e))
	}

	logger.WithValues("status_code", resp.StatusCode).Info("Alertmanager: deleted rules for tenant")

	return nil
}
