package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReconcileError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_reconcile_error_total",
		Help: "Total number of reconciliations error",
	}, []string{"controller", "result"})
)

func init() {
	metrics.Registry.MustRegister(
		ReconcileError,
	)
}
