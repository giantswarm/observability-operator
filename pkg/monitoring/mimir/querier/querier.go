package querier

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var (
	ErrorNoTimeSeries                 = errors.New("no time series found")
	ErrorFailedToConvertValueToVector = errors.New("failed to convert value to vector")
	ErrorMoreThanOneTimeSeriesFound   = errors.New("more than one time series found")
	ErrorFailedToGetTimeSeries        = errors.New("failed to get time series")
)

// QueryTSDBHeadSeries performs an instant query against Mimir.
func QueryTSDBHeadSeries(ctx context.Context, query string, metricsQueryURL string) (float64, error) {
	config := api.Config{
		Address: metricsQueryURL,
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		return 0, err
	}

	// Run query against client.
	api := v1.NewAPI(c)

	queryContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	val, _, err := api.Query(queryContext, query, time.Now())
	cancel()
	if err != nil {
		return 0, err
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
