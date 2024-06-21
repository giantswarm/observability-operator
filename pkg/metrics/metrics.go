package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReconcileError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_mimir_head_series_query_errors_total",
		Help: "Total number of reconciliations error",
	}, nil)
)

func init() {
	metrics.Registry.MustRegister(
		ReconcileError,
	)
}
