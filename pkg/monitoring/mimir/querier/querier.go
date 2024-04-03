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
)

// headerAdder is an http.RoundTripper that adds additional headers to the request
type headerAdder struct {
	headers map[string][]string

	rt http.RoundTripper
}

func (h *headerAdder) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, vv := range h.headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	return h.rt.RoundTrip(req)
}

// QueryTSDBHeadSeries performs an instant query against Mimir.
func QueryTSDBHeadSeries(ctx context.Context, clusterName string) (float64, error) {
	headerAdder := &headerAdder{
		headers: map[string][]string{
			"X-Org-Id": {"anonynous"},
		},
		rt: http.DefaultTransport,
	}
	config := api.Config{
		Address:      "http://mimir-gateway.mimir.svc/prometheus",
		RoundTripper: headerAdder,
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		return 0, err
	}

	// Run query against client.
	api := v1.NewAPI(c)

	queryContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	val, _, err := api.Query(queryContext, fmt.Sprintf("max_over_time(count({cluster_id=\"%s\"})[6h])", clusterName), time.Now())
	cancel()
	if err != nil {
		return 0, err
	}

	switch val.Type() {
	case model.ValVector:
		vector := val.(model.Vector)
		return float64(vector[0].Value), nil
	default:
		return 0, errors.New("failed to get current number of time series")
	}
}
