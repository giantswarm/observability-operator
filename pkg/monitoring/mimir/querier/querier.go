package querier

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

var (
	ErrorNoTimeSeries                 = errors.New("no time series found")
	ErrorFailedToConvertValueToVector = errors.New("failed to convert value to vector")
	ErrorMoreThanOneTimeSeriesFound   = errors.New("more than one time series found")
	ErrorFailedToGetTimeSeries        = errors.New("failed to get time series")
)

// authRoundTripper is a custom HTTP transport that adds authentication
// to outgoing requests. It wraps an existing http.RoundTripper and injects
// basic auth credentials and the organization ID header required by Mimir.
type authRoundTripper struct {
	rt       http.RoundTripper // The underlying RoundTripper to perform the actual HTTP request
	username string
	password string
	tenant   string // Tenant ID(s) for multi-tenancy (pipe-separated for multiple tenants)
}

// RoundTrip implements the http.RoundTripper interface.
// It creates a copy of the original request, preserves all existing headers,
// adds basic auth and the tenant organization ID header before forwarding
// the request to the underlying transport.
func (t authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a new request to avoid modifying the original
	reqCopy := req.Clone(req.Context())

	// Ensure headers are initialized
	if reqCopy.Header == nil {
		reqCopy.Header = make(http.Header)
	}

	// Set basic auth credentials
	reqCopy.SetBasicAuth(t.username, t.password)

	// Set the tenant organization ID header
	reqCopy.Header.Set(monitoring.OrgIDHeader, t.tenant)

	// Forward the request to the underlying transport
	return t.rt.RoundTrip(reqCopy)
}

// QueryTSDBHeadSeries performs an instant query against Mimir.
func QueryTSDBHeadSeries(ctx context.Context, query, metricsQueryURL, username, password string) (float64, error) {
	config := api.Config{
		Address: metricsQueryURL,
		RoundTripper: authRoundTripper{
			rt:       api.DefaultRoundTripper,
			username: username,
			password: password,
			tenant:   organization.GiantSwarmDefaultTenant,
		},
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		return 0, fmt.Errorf("failed to create client: %w", err)
	}

	// Run query against client.
	api := v1.NewAPI(c)

	queryContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	val, _, err := api.Query(queryContext, query, time.Now())
	cancel()
	if err != nil {
		return 0, fmt.Errorf("failed to query prometheus: %w", err)
	}

	switch val.Type() {
	case model.ValVector:
		vector, ok := val.(model.Vector)
		if !ok {
			return 0, ErrorFailedToConvertValueToVector
		}
		if len(vector) == 0 {
			return 0, ErrorNoTimeSeries
		}
		if len(vector) > 1 {
			return 0, ErrorMoreThanOneTimeSeriesFound
		}
		return float64(vector[0].Value), nil
	default:
		return 0, ErrorFailedToGetTimeSeries
	}
}
