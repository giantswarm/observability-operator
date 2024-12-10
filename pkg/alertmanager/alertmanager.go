package alertmanager

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/config"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	return j.configure(alertmanagerConfigContent, templates, tenantID)
}

func (j Job) configure(alertmanagerConfigContent []byte, templates map[string]string, tenantID string) error {
	// Load alertmanager configuration
	alertmanagerConfig, err := config.Load(string(alertmanagerConfigContent))
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to load configuration: %w", err))
	}

	// Set notification template name
	// This must match the key set for the template in configCompat.TemplateFiles. This value should not be a path otherwise the request will fail with:
	// > error validating Alertmanager config: invalid template name "/etc/dummy.tmpl": the template name cannot contain any path
	alertmanagerConfig.Templates = slices.Collect(maps.Keys(templates))

	// Prepare request for Alertmanager API
	requestData := configRequest{
		AlertmanagerConfig: alertmanagerConfig.String(),
		TemplateFiles:      templates,
	}
	data, err := yaml.Marshal(requestData)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to marshal yaml: %w", err))
	}

	// Send request to Alertmanager's API
	req, err := http.NewRequest(http.MethodPost, j.alertmanagerURL+alertmanagerAPIPath, bytes.NewBuffer(data))
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to create request: %w", err))
	}
	req.Header.Set(orgIDHeader, tenantID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.WithStack(fmt.Errorf("alertmanager: failed to send request: %w", err))
	}
	defer resp.Body.Close() // nolint: errcheck

	//TODO: handle response errors if any

	return nil
}
