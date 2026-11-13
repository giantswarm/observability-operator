package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

const (
	dashboardAPIUserAgent = "observability-operator"
	orgIDHeader           = "X-Grafana-Org-Id"
)

// ErrDashboardNotFound is returned by DashboardAPIClient.Get when the dashboard does not exist.
var ErrDashboardNotFound = errors.New("dashboard not found")

// DashboardAPIClient exposes the Grafana app-platform dashboard API
// /apis/dashboard.grafana.app/<version>/namespaces/<namespace>/dashboards.
// Request and response bodies are Kubernetes-style Dashboard resources.
type DashboardAPIClient interface {
	// Get returns the dashboard resource. It returns ErrDashboardNotFound when the dashboard does not exist.
	Get(ctx context.Context, version, namespace, uid string) (json.RawMessage, error)
	// Create creates the dashboard resource with a POST request.
	Create(ctx context.Context, version, namespace string, dashboard json.RawMessage) error
	// Update replaces the dashboard resource with a PUT request.
	Update(ctx context.Context, version, namespace, uid string, dashboard json.RawMessage) error
}

// OrgNamespace returns the app-platform namespace of a Grafana organization:
// "default" for org 1, "org-<id>" for every other org.
func OrgNamespace(orgID int64) string {
	if orgID == 1 {
		return "default"
	}
	return "org-" + strconv.FormatInt(orgID, 10)
}

// dashboardAPIClient is the HTTP implementation of DashboardAPIClient.
//
// It is hand-written because grafana-openapi-client-go is generated from the
// classic /api swagger and does not cover the app-platform /apis groups.
type dashboardAPIClient struct {
	httpClient *http.Client
	baseURL    *url.URL
	basicAuth  *url.Userinfo
	orgID      int64
}

func newDashboardAPIClient(httpClient *http.Client, baseURL *url.URL, basicAuth *url.Userinfo, orgID int64) *dashboardAPIClient {
	return &dashboardAPIClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		basicAuth:  basicAuth,
		orgID:      orgID,
	}
}

// withOrgID returns a copy of the client scoped to orgID. The receiver is not modified.
func (c *dashboardAPIClient) withOrgID(orgID int64) *dashboardAPIClient {
	clone := *c
	clone.orgID = orgID
	return &clone
}

func (c *dashboardAPIClient) Get(ctx context.Context, version, namespace, uid string) (json.RawMessage, error) {
	path := c.dashboardsPath(version, namespace, uid)
	body, status, err := c.do(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("failed to get dashboard: %s %s: %w", http.MethodGet, path, ErrDashboardNotFound)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("failed to get dashboard: %s %s: status %d: %s", http.MethodGet, path, status, body)
	}
	return body, nil
}

func (c *dashboardAPIClient) Create(ctx context.Context, version, namespace string, dashboard json.RawMessage) error {
	path := c.dashboardsPath(version, namespace, "")
	body, status, err := c.do(ctx, http.MethodPost, path, writeQuery(version), dashboard)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("failed to create dashboard: %s %s: status %d: %s", http.MethodPost, path, status, body)
	}
	return nil
}

func (c *dashboardAPIClient) Update(ctx context.Context, version, namespace, uid string, dashboard json.RawMessage) error {
	path := c.dashboardsPath(version, namespace, uid)
	body, status, err := c.do(ctx, http.MethodPut, path, writeQuery(version), dashboard)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("failed to update dashboard: %s %s: status %d: %s", http.MethodPut, path, status, body)
	}
	return nil
}

func (c *dashboardAPIClient) dashboardsPath(version, namespace, uid string) string {
	path := "/apis/dashboard.grafana.app/" + url.PathEscape(version) + "/namespaces/" + url.PathEscape(namespace) + "/dashboards"
	if uid != "" {
		path += "/" + url.PathEscape(uid)
	}
	return path
}

// writeQuery returns the query string of a create or update request.
//
// Grafana validates v1 dashboards strictly against the latest classic schema and
// rejects older dashboard JSON, such as string datasource references, with HTTP 422.
// fieldValidation=Ignore skips that check. Grafana still migrates the dashboard to
// the latest schema version. The v2 versions keep the strict validation.
func writeQuery(version string) string {
	if version == "v1" {
		return "fieldValidation=Ignore"
	}
	return ""
}

// do sends the request and returns the response body and status code.
func (c *dashboardAPIClient) do(ctx context.Context, method, path, rawQuery string, reqBody []byte) ([]byte, int, error) {
	u := c.baseURL.JoinPath(path)
	u.RawQuery = rawQuery

	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build %s request to %s: %w", method, path, err)
	}
	req.Header.Set("User-Agent", dashboardAPIUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set(orgIDHeader, strconv.FormatInt(c.orgID, 10))
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.basicAuth != nil {
		password, _ := c.basicAuth.Password()
		req.SetBasicAuth(c.basicAuth.Username(), password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send %s request to %s: %w", method, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response of %s request to %s: %w", method, path, err)
	}
	return respBody, resp.StatusCode, nil
}
