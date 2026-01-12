package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	cronitorAPIBaseURL = "https://cronitor.io/api/monitors"
	cronitorPingURL    = "https://cronitor.link/p"
)

var (
	// ErrMonitorNotFound is returned when a monitor does not exist in Cronitor.
	ErrMonitorNotFound = errors.New("monitor not found")
)

// HTTPClient is an interface for making HTTP requests, allowing for easier testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// CronitorHeartbeatRepository is a repository for managing heartbeats in Cronitor.
type CronitorHeartbeatRepository struct {
	config.Config
	httpClient HTTPClient
}

// CronitorMonitor represents a Cronitor heartbeat monitor configuration.
type CronitorMonitor struct {
	Type             string   `json:"type"`
	Key              string   `json:"key"`
	Name             string   `json:"name"`
	GraceSeconds     int      `json:"grace_seconds"`
	Schedule         string   `json:"schedule"`
	Notify           []string `json:"notify"`
	Tags             []string `json:"tags"`
	Note             string   `json:"note,omitempty"`
	FailureTolerance *int     `json:"failure_tolerance,omitempty"`
	RealertInterval  string   `json:"realert_interval,omitempty"`
}

// NewCronitorHeartbeatRepository creates a new CronitorHeartbeatRepository.
func NewCronitorHeartbeatRepository(cfg config.Config, httpClient HTTPClient) (HeartbeatRepository, error) {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
	}
	return &CronitorHeartbeatRepository{
		Config:     cfg,
		httpClient: httpClient,
	}, nil
}

// monitorKey returns the unique key for this cluster's monitor.
func (r *CronitorHeartbeatRepository) monitorKey() string {
	return fmt.Sprintf("mimir-%s", r.Config.Cluster.Name)
}

// makeMonitor creates a Cronitor monitor configuration for the management cluster.
func (r *CronitorHeartbeatRepository) makeMonitor() *CronitorMonitor {
	tags := []string{
		"team:atlas",
		"managed-by:observability-operator",
		fmt.Sprintf("installation:%s", r.Config.Cluster.Name),
		fmt.Sprintf("pipeline:%s", r.Config.Cluster.Pipeline),
	}
	// Tags need to be sorted alphabetically to avoid unnecessary heartbeat updates
	sort.Strings(tags)

	key := r.monitorKey()
	return &CronitorMonitor{
		Type:            "heartbeat",
		Key:             key,
		Name:            key,
		GraceSeconds:    1800, // 30 minutes
		Schedule:        "every 30 minutes",
		Notify:          []string{r.Config.Cluster.Pipeline},
		Tags:            tags,
		Note:            "📗 Runbook: https://intranet.giantswarm.io/docs/support-and-ops/ops-recipes/heartbeat-expired/",
		RealertInterval: "every 24 hours", // Re-alert every 24 hours if the issue persists
	}
}

func (r *CronitorHeartbeatRepository) CreateOrUpdate(ctx context.Context) error {
	logger := log.FromContext(ctx)

	monitor := r.makeMonitor()

	// Check if the monitor already exists (check without environment to find it in any env)
	logger.Info("checking if heartbeat monitor exists")
	existingMonitor, err := r.getMonitor(ctx, monitor.Key)
	if err != nil && !errors.Is(err, ErrMonitorNotFound) {
		return fmt.Errorf("failed to check if monitor exists: %w", err)
	}

	isNewMonitor := errors.Is(err, ErrMonitorNotFound)
	var needsPing bool

	if isNewMonitor {
		logger.Info("heartbeat monitor does not exist, creating new monitor")
		if err := r.createMonitor(ctx, monitor); err != nil {
			return err
		}
		needsPing = true
	} else {
		// Monitor exists, check if it needs updating
		if !r.hasChanged(existingMonitor, monitor) {
			logger.Info("heartbeat monitor is up to date")
			return nil
		}
		logger.Info("heartbeat monitor has changed, updating")
		if err := r.updateMonitor(ctx, monitor); err != nil {
			return err
		}
		// Ping if pipeline changed to associate monitor with new environment
		needsPing = r.pipelineChanged(existingMonitor)
	}

	// Ping to associate monitor with environment
	if needsPing {
		logger.Info("sending ping to associate monitor with environment",
			"is_new", isNewMonitor,
			"pipeline", r.Config.Cluster.Pipeline)
		if err := r.pingMonitor(ctx, monitor.Key); err != nil {
			logger.Error(err, "failed to ping monitor, monitor created but not associated with environment")
			// Don't fail the whole operation if ping fails
		}
	}

	return nil
}

func (r *CronitorHeartbeatRepository) Delete(ctx context.Context) error {
	logger := log.FromContext(ctx)

	monitorKey := r.monitorKey()
	logger.Info("deleting heartbeat monitor")

	// Delete from any environment - monitor will be removed across all environments
	url := fmt.Sprintf("%s/%s", cronitorAPIBaseURL, monitorKey)
	resp, err := r.doAuthenticatedRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to delete monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Info("heartbeat monitor does not exist, nothing to delete")
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return r.handleErrorResponse(ctx, resp, "DELETE", url, nil, "failed to delete monitor")
	}

	logger.Info("deleted heartbeat monitor successfully")
	return nil
}

// getMonitor retrieves an existing monitor from Cronitor.
func (r *CronitorHeartbeatRepository) getMonitor(ctx context.Context, key string) (*CronitorMonitor, error) {
	url := fmt.Sprintf("%s/%s", cronitorAPIBaseURL, key)
	resp, err := r.doAuthenticatedRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMonitorNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, r.handleErrorResponse(ctx, resp, "GET", url, nil, "failed to get monitor")
	}

	var monitor CronitorMonitor
	if err := json.NewDecoder(resp.Body).Decode(&monitor); err != nil {
		return nil, fmt.Errorf("failed to decode monitor response: %w", err)
	}

	return &monitor, nil
}

// createMonitor creates a new monitor in Cronitor using POST.
// Note: Environments are never sent via API - they are automatically associated
// when telemetry (pings) are sent to that environment.
func (r *CronitorHeartbeatRepository) createMonitor(ctx context.Context, monitor *CronitorMonitor) error {
	logger := log.FromContext(ctx)

	body, err := json.Marshal(monitor)
	if err != nil {
		return fmt.Errorf("failed to marshal monitor: %w", err)
	}

	resp, err := r.doAuthenticatedRequest(ctx, http.MethodPost, cronitorAPIBaseURL, body)
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return r.handleErrorResponse(ctx, resp, "POST", cronitorAPIBaseURL, body, "failed to create monitor")
	}

	logger.Info("created heartbeat monitor successfully")
	return nil
}

// updateMonitor updates an existing monitor in Cronitor using PUT.
// Note: Environments are never sent via API - they are automatically associated
// when telemetry (pings) are sent to that environment.
func (r *CronitorHeartbeatRepository) updateMonitor(ctx context.Context, monitor *CronitorMonitor) error {
	logger := log.FromContext(ctx)

	body, err := json.Marshal(monitor)
	if err != nil {
		return fmt.Errorf("failed to marshal monitor: %w", err)
	}

	url := fmt.Sprintf("%s/%s", cronitorAPIBaseURL, monitor.Key)
	resp, err := r.doAuthenticatedRequest(ctx, http.MethodPut, url, body)
	if err != nil {
		return fmt.Errorf("failed to update monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return r.handleErrorResponse(ctx, resp, "PUT", url, body, "failed to update monitor")
	}

	logger.Info("updated heartbeat monitor successfully")
	return nil
}

// hasChanged compares the existing monitor with the desired configuration.
func (r *CronitorHeartbeatRepository) hasChanged(existing, desired *CronitorMonitor) bool {
	return existing.GraceSeconds != desired.GraceSeconds ||
		existing.Schedule != desired.Schedule ||
		existing.Note != desired.Note ||
		existing.RealertInterval != desired.RealertInterval ||
		!slices.Equal(existing.Tags, desired.Tags)
}

// pipelineChanged checks if the pipeline (environment) changed by comparing pipeline tags.
func (r *CronitorHeartbeatRepository) pipelineChanged(existing *CronitorMonitor) bool {
	desiredPipelineTag := fmt.Sprintf("pipeline:%s", r.Config.Cluster.Pipeline)
	var existingPipelineTag string
	for _, tag := range existing.Tags {
		if strings.HasPrefix(tag, "pipeline:") {
			existingPipelineTag = tag
			break
		}
	}
	// If no pipeline tag found in existing, treat as changed
	if existingPipelineTag == "" {
		return true
	}
	return existingPipelineTag != desiredPipelineTag
}

// pingMonitor sends a ping to the Cronitor telemetry API to associate the monitor with an environment.
func (r *CronitorHeartbeatRepository) pingMonitor(ctx context.Context, monitorKey string) error {
	url := fmt.Sprintf("%s/%s/%s?env=%s", cronitorPingURL, r.Config.Environment.CronitorHeartbeatPingKey, monitorKey, r.Config.Cluster.Pipeline)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to ping monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return r.handleErrorResponse(ctx, resp, "GET", url, nil, "failed to ping monitor")
	}

	return nil
}

// doAuthenticatedRequest creates and executes an HTTP request with authentication.
func (r *CronitorHeartbeatRepository) doAuthenticatedRequest(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	logger := log.FromContext(ctx)

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for all requests
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(r.Config.Environment.CronitorHeartbeatManagementKey, "")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		logger.Error(err, "cronitor API request failed",
			"method", req.Method,
			"url", req.URL.String(),
		)
		return nil, err
	}

	return resp, nil
}

// handleErrorResponse reads the response body and returns a formatted error with request details.
func (r *CronitorHeartbeatRepository) handleErrorResponse(ctx context.Context, resp *http.Response, method, url string, requestBody []byte, message string) error {
	logger := log.FromContext(ctx)
	responseBody, _ := io.ReadAll(resp.Body)

	// Log the full request details for debugging
	logger.Error(nil, "cronitor API request failed",
		"message", message,
		"method", method,
		"url", url,
		"status_code", resp.StatusCode,
		"request_body", string(requestBody),
		"response_body", string(responseBody),
	)

	return fmt.Errorf("%s: method=%s url=%s status_code=%d response_body=%s",
		message, method, url, resp.StatusCode, string(responseBody))
}
