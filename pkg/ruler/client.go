package ruler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	// DeleteAllRulesForTenant deletes all rule namespaces owned by tenantID.
	// Implementations must be idempotent: a tenant with no rules is not an error.
	DeleteAllRulesForTenant(ctx context.Context, tenantID string) error
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

func (c *client) DeleteAllRulesForTenant(ctx context.Context, tenantID string) error {
	logger := log.FromContext(ctx).WithValues("tenant", tenantID)

	namespaces, err := c.listNamespaces(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to list ruler namespaces: %w", err)
	}

	var errs []error
	for _, ns := range namespaces {
		if err := c.deleteNamespace(ctx, tenantID, ns); err != nil {
			errs = append(errs, err)
			continue
		}
		logger.Info("deleted ruler namespace", "namespace", ns)
	}

	if len(errs) == 0 && len(namespaces) > 0 {
		logger.Info("deleted all ruler rules", "namespaces_deleted", len(namespaces))
	}

	return errors.Join(errs...)
}

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

func (m *multiClient) DeleteAllRulesForTenant(ctx context.Context, tenantID string) error {
	var errs []error
	for _, c := range m.clients {
		errs = append(errs, c.DeleteAllRulesForTenant(ctx, tenantID))
	}
	return errors.Join(errs...)
}

// noopClient is a no-op implementation of Client (ruler not configured).
type noopClient struct{}

func (noopClient) DeleteAllRulesForTenant(_ context.Context, _ string) error { return nil }
