package alertmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/config"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
)

const (
	// Those values are used to retrieve the Alertmanager configuration from the secret named after conf.Monitoring.AlertmanagerSecretName
	// alertmanagerConfigKey is the key to the alertmanager configuration in the secret
	alertmanagerConfigKey = "alertmanager.yaml"
	// templatesSuffix is the suffix used to identify the templates in the secret
	templatesSuffix = ".tmpl"

	orgIDHeader         = "X-Scope-OrgID"
	alertmanagerAPIPath = "/api/v1/alerts"
)

type Job struct {
	client             client.Client
	alertmanagerURL    string
	alertmanagerSecret client.ObjectKey
}

type configRequest struct {
	TemplateFiles      map[string]string `yaml:"template_files"`
	AlertmanagerConfig string            `yaml:"alertmanager_config"`
}

func New(conf pkgconfig.Config, c client.Client) Job {
	job := Job{
		client:          c,
		alertmanagerURL: strings.TrimSuffix(conf.Monitoring.AlertmanagerURL, "/"),
		alertmanagerSecret: client.ObjectKey{
			Name:      conf.Monitoring.AlertmanagerSecretName,
			Namespace: conf.Namespace,
		},
	}

	return job
}

func (j Job) Configure(ctx context.Context) error {
	logger := log.FromContext(ctx)

	logger.Info("Alertmanager: configuring")

	//TODO: get this from somewhere
	tenantID := "anonymous"

	// Read secret used as source for Alertmanager configuration
	alertmanagerSecret := v1.Secret{}
	err := j.client.Get(ctx, j.alertmanagerSecret, &alertmanagerSecret)

	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to get secret: %w", err))
	}

	// Retrieve Alertmanager configuration from secret
	alertmanagerConfigContent, ok := alertmanagerSecret.Data[alertmanagerConfigKey]
	if !ok {
		return errors.WithStack(fmt.Errorf("alertmanager: config not found"))
	}

	// Retrieve all alertmanager templates from secret
	templates := make(map[string]string)
	for key, value := range alertmanagerSecret.Data {
		if strings.HasSuffix(key, templatesSuffix) {
			templates[key] = string(value)
		}
	}

	err = j.configure(ctx, alertmanagerConfigContent, templates, tenantID)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to configure: %w", err))
	}

	logger.Info("Alertmanager: configured")
	return nil
}

func (j Job) configure(ctx context.Context, alertmanagerConfigContent []byte, templates map[string]string, tenantID string) error {
	logger := log.FromContext(ctx)

	// Load alertmanager configuration
	alertmanagerConfig, err := config.Load(string(alertmanagerConfigContent))
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to load configuration: %w", err))
	}

	// Set notification template name
	// This must match the key set for the template in configCompat.TemplateFiles. This value should not be a path otherwise the request will fail with:
	// > error validating Alertmanager config: invalid template name "/etc/dummy.tmpl": the template name cannot contain any path
	alertmanagerConfig.Templates = slices.Collect(maps.Keys(templates))
	alertmanagerConfigString := alertmanagerConfig.String()

	// Prepare request for Alertmanager API
	requestData := configRequest{
		AlertmanagerConfig: alertmanagerConfigString,
		TemplateFiles:      templates,
	}
	data, err := yaml.Marshal(requestData)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to marshal yaml: %w", err))
	}

	url := j.alertmanagerURL + alertmanagerAPIPath
	logger.WithValues("url", url, "data_size", len(data), "config_size", len(alertmanagerConfigString), "templates_count", len(templates)).Info("Alertmanager: sending configuration")

	// Send request to Alertmanager's API
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to create request: %w", err))
	}
	req.Header.Set(orgIDHeader, tenantID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to send request: %w", err))
	}
	defer resp.Body.Close() // nolint: errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to read response: %w", err))
	}

	logger.WithValues("status_code", resp.StatusCode, "response", string(respBody)).Info("Alertmanager: configuration sent")

	//TODO: handle response errors if any

	return nil
}
