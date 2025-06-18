package controller

import (
	"context"
	"errors"
	"net/url"

	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"sigs.k8s.io/controller-runtime/pkg/client"

	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// MockGrafanaClientGenerator is a simple mock for the GrafanaClientGenerator interface
// that can be reused across different controller tests
type MockGrafanaClientGenerator struct {
	shouldReturnError bool

	// Call tracking
	CallCount int
	LastURL   *url.URL
}

func (m *MockGrafanaClientGenerator) GenerateGrafanaClient(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (*grafana.GrafanaHTTPAPI, error) {
	m.CallCount++
	m.LastURL = grafanaURL

	if m.shouldReturnError {
		return nil, errors.New("grafana service unavailable")
	}

	// For the working scenario, we'll return an error that indicates
	// the configuration should be skipped (simulating a scenario
	// where Grafana client generation succeeds but operations are not performed)
	// This is a limitation of the current test approach - we can't easily mock the complex Grafana API
	return nil, errors.New("configuration skipped for testing")
}

// SetShouldReturnError configures whether the mock should return an error
func (m *MockGrafanaClientGenerator) SetShouldReturnError(shouldError bool) {
	m.shouldReturnError = shouldError
}

// Reset clears the call tracking data
func (m *MockGrafanaClientGenerator) Reset() {
	m.CallCount = 0
	m.LastURL = nil
}

// Ensure MockGrafanaClientGenerator implements the interface
var _ grafanaclient.GrafanaClientGenerator = (*MockGrafanaClientGenerator)(nil)
