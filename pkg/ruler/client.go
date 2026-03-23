package ruler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

const httpTimeout = 30 * time.Second

const (
	mimirRulesAPIPath = "/api/prom/rules"
	lokiRulesAPIPath  = "/loki/api/v1/rules"
)

// Client deletes recording and alerting rules from a ruler backend.
type Client interface {
	// DeleteClusterRulesForTenant deletes all rule namespaces owned by tenantID
	// whose name starts with clusterID (the mimir_namespace_prefix / loki_namespace_prefix
	// set by Alloy, which equals the cluster name).
	// Implementations must be idempotent: a tenant with no matching rules is not an error.
	DeleteClusterRulesForTenant(ctx context.Context, tenantID, clusterID string) error
}

// NewMimir returns a Client that targets the Mimir ruler at baseURL.
func NewMimir(baseURL string) Client {
	return &client{baseURL: baseURL, rulesAPIPath: mimirRulesAPIPath, httpClient: &http.Client{Timeout: httpTimeout}}
}

// NewLoki returns a Client that targets the Loki ruler at baseURL.
func NewLoki(baseURL string) Client {
	return &client{baseURL: baseURL, rulesAPIPath: lokiRulesAPIPath, httpClient: &http.Client{Timeout: httpTimeout}}
}

// NewNoop returns a Client that does nothing.
// Use this when the ruler URL is not configured.
func NewNoop() Client { return noopClient{} }

// client is the live implementation of Client.
type client struct {
	baseURL      string
	rulesAPIPath string
	httpClient   *http.Client
}

func (c *client) DeleteClusterRulesForTenant(ctx context.Context, tenantID, clusterID string) error {
	logger := log.FromContext(ctx).WithValues("tenant", tenantID, "cluster", clusterID)

	namespaces, err := c.listNamespaces(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to list ruler namespaces: %w", err)
	}

	var errs []error
	deleted := 0
	for _, ns := range namespaces {
		if !strings.HasPrefix(ns, clusterID) {
			continue
		}
		if err := c.deleteNamespace(ctx, tenantID, ns); err != nil {
			errs = append(errs, err)
			continue
		}
		logger.Info("deleted ruler namespace", "namespace", ns)
		deleted++
	}

	if len(errs) == 0 && deleted > 0 {
		logger.Info("deleted cluster ruler rules", "namespaces_deleted", deleted)
	}

	return errors.Join(errs...)
}

// listNamespaces returns the names of all rule namespaces that exist for tenantID.
// A 404 response is treated as "no namespaces" and returns nil, nil.
func (c *client) listNamespaces(ctx context.Context, tenantID string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+c.rulesAPIPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request: %w", err)
	}
	req.Header.Set(monitoring.OrgIDHeader, tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send list request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing rules: %s", resp.StatusCode, string(body))
	}

	var result map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	namespaces := make([]string, 0, len(result))
	for ns := range result {
		namespaces = append(namespaces, ns)
	}
	return namespaces, nil
}

// deleteNamespace deletes a single rule namespace for tenantID.
// 404 is treated as success (already gone).
func (c *client) deleteNamespace(ctx context.Context, tenantID, namespace string) error {
	u := c.baseURL + c.rulesAPIPath + "/" + url.PathEscape(namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request for namespace %s: %w", namespace, err)
	}
	req.Header.Set(monitoring.OrgIDHeader, tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request for namespace %s: %w", namespace, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete ruler namespace %s: status %d: %s", namespace, resp.StatusCode, string(body))
	}
}

// NewMulti returns a Client that fans out to all provided backends.
// All backends are always attempted; errors are joined so a failure in one does not skip the others.
func NewMulti(clients ...Client) Client {
	return &multiClient{clients: clients}
}

// multiClient fans out ruler operations to multiple backends.
type multiClient struct {
	clients []Client
}

func (m *multiClient) DeleteClusterRulesForTenant(ctx context.Context, tenantID, clusterID string) error {
	var errs []error
	for _, c := range m.clients {
		errs = append(errs, c.DeleteClusterRulesForTenant(ctx, tenantID, clusterID))
	}
	return errors.Join(errs...)
}

// noopClient is a no-op implementation of Client (ruler not configured).
type noopClient struct{}

func (noopClient) DeleteClusterRulesForTenant(_ context.Context, _, _ string) error { return nil }
