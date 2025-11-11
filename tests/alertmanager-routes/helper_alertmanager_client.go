package alertmanagerroutes

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
	"github.com/go-logr/logr"
	clientruntime "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/goccy/go-yaml"
	"github.com/prometheus/alertmanager/api/v2/client"
	"github.com/prometheus/alertmanager/api/v2/client/alert"
	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/alertmanager/config"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// hostAddress is the ip address used to reach the host from a container
	hostAddress = "172.17.0.1"

	// configDir is directory path where the Alertmanager configuration files are located
	configDir = ""
	// configWaitTimeout is the time to wait for Alertmanager configuration
	// to be propagated
	configWaitTimeout = 30 * time.Second
	// configNoReceiver is the name of the default receiver present
	// in the Alertmanager configuration when no configuration has been applied
	configNoReceiver = "no-receiver"

	// TODO: pass those values by flag or env variable
	alertmanagerURL = &url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:8080",
	}
)

func init() {
	flag.StringVar(&alertmanagerURL.Host, "alertmanager-url", alertmanagerURL.Host, "Alertmanager URL")
	flag.StringVar(&configDir, "alertmanager-config-dir", configDir, "Alertmanager configuration director path")
	flag.DurationVar(&configWaitTimeout, "alertmanager-config-wait-timeout", configWaitTimeout, "Time to wait for Alertmanager configuration propagation")
	flag.StringVar(&hostAddress, "docker-host-address", hostAddress, "IP address used to reach the host from a container")

	// do not redact secret values when marshalling Alertmanager configuration
	commonconfig.MarshalSecretValue = true

	// logger must be set since it's used in alertmanager pkg
	log.SetLogger(logr.New(log.NullLogSink{}))
}

// alertmanagerClient is a client for Mimir Alertmanager
// This client is capable of both managing alerts and configuring Alertmanager
type alertmanagerClient struct {
	t *testing.T

	alertmanagerURL     *url.URL
	TenantID            string
	httpReceiverAddress string

	alertmanagerAPIClient *client.AlertmanagerAPI
	alertmanagerService   *alertmanager.Service
	httpClient            *http.Client
}

// NewAlertmanagerClient creates a new Mimir Alertmanager client
func NewAlertmanagerClient(t *testing.T, alertmanagerURL *url.URL, tenantID, httpReceiverPort string) *alertmanagerClient {
	amClient := &alertmanagerClient{
		t:                   t,
		alertmanagerURL:     alertmanagerURL,
		TenantID:            tenantID,
		httpReceiverAddress: net.JoinHostPort(hostAddress, httpReceiverPort),
	}

	// Create Mimir Alertmanager client used for alert management
	{
		amClient.httpClient = &http.Client{}
		amClient.httpClient.Transport = newMimirTenantHTTPTransport(tenantID)
		cr := clientruntime.NewWithClient(alertmanagerURL.Host, path.Join("/alertmanager", "/api/v2"), []string{alertmanagerURL.Scheme}, amClient.httpClient)

		amClient.alertmanagerAPIClient = client.New(cr, strfmt.Default)
	}

	// Create Mimir Alertmanager client used for configuration
	{
		amConfigurationClient := alertmanager.New(pkgconfig.Config{
			Monitoring: pkgconfig.MonitoringConfig{
				AlertmanagerURL: alertmanagerURL.String(),
			},
		})

		amClient.alertmanagerService = &amConfigurationClient
	}

	return amClient
}

// Configure uploads the given alertmanagerConfig into Alertmanager.
// It does override the global.http_config.proxy_url with the given httpAddress,
// and enforces insecure_skip_verify to true.
func (a alertmanagerClient) Configure() error {
	// Read Alertmanager configuration file
	alertmanagerConfigFilePath := filepath.Join(configDir, alertmanager.AlertmanagerConfigKey)
	amConfigContent, err := os.ReadFile(alertmanagerConfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", alertmanagerConfigFilePath, err)
	}

	amConfig := &config.Config{}
	err = yaml.Unmarshal(amConfigContent, amConfig)
	if err != nil {
		return fmt.Errorf("failed to load alertmanager configuration: %w", err)
	}

	proxyURL, err := url.Parse("http://" + a.httpReceiverAddress)
	if err != nil {
		return fmt.Errorf("failed to parse proxy URL: %v", err)
	}
	commonProxyURL := commonconfig.URL{URL: proxyURL}

	// Set proxy_url in alertmanager config
	if amConfig.Global == nil {
		amConfig.Global = &config.GlobalConfig{}
	}
	if amConfig.Global.HTTPConfig == nil {
		amConfig.Global.HTTPConfig = &commonconfig.HTTPClientConfig{}
	}
	setHTTPClientProxy(amConfig.Global.HTTPConfig, commonProxyURL)
	for _, receiver := range amConfig.Receivers {
		err = setReceiverHTTPClientProxy(receiver, commonProxyURL)
		if err != nil {
			return err
		}
	}

	overrideWaitTimes(amConfig.Route)

	// Marshal the modifier alertmanager config back to yaml
	// Remove the custom Marshaler of config.AlertmanagerConfig, which prodcues an invalid configuration with all secrets replaced with <redacted>.
	amConfigContent, err = yaml.MarshalWithOptions(amConfig, yaml.CustomMarshaler(noSecretHidding), yaml.CustomMarshaler(noSecretURLHidding))
	if err != nil {
		return fmt.Errorf("failed to marshal alertmanager configuration: %v", err)
	}

	templateFiles := make(map[string]string)
	paths, err := filepath.Glob(filepath.Join(configDir, "*"+alertmanager.TemplatesSuffix))
	if err != nil {
		return fmt.Errorf("failed to list template files: %v", err)
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %v", path, err)
		}
		templateFiles[filepath.Base(path)] = string(content)
	}

	// Configure Alertmanager
	err = a.alertmanagerService.Configure(a.t.Context(), amConfigContent, templateFiles, a.TenantID)
	if err != nil {
		return fmt.Errorf("failed to upload configuration: %v", err)
	}

	err = a.waitForConfigPropagationWithTimeout()
	if err != nil {
		return fmt.Errorf("failed to wait for configuration propagation: %v", err)
	}

	return nil
}

func setHTTPClientProxy(c *commonconfig.HTTPClientConfig, proxyURL commonconfig.URL) {
	if c == nil {
		return
	}

	c.ProxyURL = proxyURL
	c.ProxyFromEnvironment = false
	c.TLSConfig.InsecureSkipVerify = true
}

func setReceiverHTTPClientProxy(r config.Receiver, proxyURL commonconfig.URL) error {
	if len(r.EmailConfigs) > 0 {
		return fmt.Errorf("test not implemented for email configs")
	}

	for _, c := range r.DiscordConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.PagerdutyConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.SlackConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.WebhookConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.OpsGenieConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.WechatConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.PushoverConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.VictorOpsConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.SNSConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.TelegramConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.WebexConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.MSTeamsConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	for _, c := range r.JiraConfigs {
		setHTTPClientProxy(c.HTTPConfig, proxyURL)
	}
	return nil
}

// Reset any wait times in the route tree to avoid delays in tests
func overrideWaitTimes(route *config.Route) {
	route.GroupByStr = []string{"..."}
	route.GroupInterval = nil
	route.GroupWait = nil
	route.RepeatInterval = nil

	for _, childRoute := range route.Routes {
		overrideWaitTimes(childRoute)
	}
}

func amDuration(d time.Duration) *model.Duration {
	m := model.Duration(d)

	return &m
}

type alertmanagerStatus struct {
	Config map[string]string `json:"config"`
}

func (a alertmanagerClient) waitForConfigPropagationWithTimeout() error {
	waitCtx, cancel := context.WithTimeout(a.t.Context(), configWaitTimeout)
	defer cancel()

	a.t.Log("Waiting for alertmanager configuration propagation...")
	for {
		err := a.waitForConfigPropagation()
		if err == nil {
			return nil
		}

		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		default:
			time.Sleep(5 * time.Second)
		}
	}
}

func (a alertmanagerClient) waitForConfigPropagation() error {
	req, err := http.NewRequest(http.MethodGet, a.alertmanagerURL.JoinPath("/alertmanager/api/v2/status").String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create waitForConfigPropagation request: %v", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform waitForConfigPropagation request: %v", err)
	}
	defer resp.Body.Close()

	amStatus := alertmanagerStatus{}
	err = json.NewDecoder(resp.Body).Decode(&amStatus)
	if err != nil {
		return fmt.Errorf("failed to decode alertmanager status response: %w", err)
	}

	configOriginal, ok := amStatus.Config["original"]
	if !ok {
		return fmt.Errorf("alertmanager configuration not yet applied")
	}

	amConfig := config.Config{}
	err = yaml.Unmarshal([]byte(configOriginal), &amConfig)
	if err != nil {
		return fmt.Errorf("failed to read alertmanager configuration: %w", err)
	}

	if amConfig.Route != nil && amConfig.Route.Receiver != configNoReceiver {
		return nil
	}

	return fmt.Errorf("alertmanager configuration not yet applied")
}

func (a alertmanagerClient) UnConfigure() error {
	req, err := http.NewRequest(http.MethodPost, a.alertmanagerURL.JoinPath("/multitenant_alertmanager/delete_tenant_config").String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create unconfigure request: %v", err)
	}

	_, err = a.httpClient.Do(req)
	return err
}

func noSecretHidding(c config.Secret) ([]byte, error) {
	s := fmt.Sprintf("%v", c)
	return []byte(s), nil
}

func noSecretURLHidding(c *config.SecretURL) ([]byte, error) {
	return []byte(c.URL.String()), nil
}

// PostAlerts sends an alert with the given labels to Alertmanager
func (a alertmanagerClient) PostAlerts(alertData Alert) error {
	labels := alertData.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["alertname"] = alertData.Name

	alertPost := &alert.PostAlertsParams{
		Alerts: models.PostableAlerts{
			{
				Annotations: nil,
				Alert: models.Alert{
					Labels: models.LabelSet(labels),
				},
				StartsAt: strfmt.DateTime(time.Now().Add(-1 * time.Minute)),
			},
		},
	}

	_, err := a.alertmanagerAPIClient.Alert.PostAlerts(alertPost)
	return err
}

// mimirTenantHTTPTransport is an HTTP transport that adds the tenant ID header to each request
type mimirTenantHTTPTransport struct {
	http.RoundTripper
	TenantID string
}

func newMimirTenantHTTPTransport(tenantID string) *mimirTenantHTTPTransport {
	return &mimirTenantHTTPTransport{
		RoundTripper: http.DefaultTransport,
		TenantID:     tenantID,
	}
}

func (m *mimirTenantHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Set tenant id header
	req.Header.Set(monitoring.OrgIDHeader, m.TenantID)

	return m.RoundTripper.RoundTrip(req)
}
