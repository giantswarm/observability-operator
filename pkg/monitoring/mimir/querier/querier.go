package querier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// QueryTSDBHeadSeries performs an instant query against Mimir.
func QueryTSDBHeadSeries(ctx context.Context, clusterName string) (float64, error) {
	config := api.Config{
		Address: "http://mimir-gateway.mimir.svc/prometheus",
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		return 0, err
	}

	// Run query against client.
	api := v1.NewAPI(c)

	queryContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	query := fmt.Sprintf("sum(max_over_time(prometheus_agent_active_series{cluster_id=\"%s\"}[6h]))", clusterName)
	val, _, err := api.Query(queryContext, query, time.Now())
	cancel()
	if err != nil {
		return 0, err
	}

	switch val.Type() {
	case model.ValVector:
		vector, ok := val.(model.Vector)
		if !ok {
			return 0, errors.New("failed to convert value to vector")
		}
		if len(vector) == 0 {
			return 0, errors.New("no time series found")
		}
		if len(vector) > 1 {
			return 0, errors.New("more than one time series found")
		}
		return float64(vector[0].Value), nil
	default:
		return 0, errors.New("failed to get current number of time series")
	}
}
