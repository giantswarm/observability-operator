package metrics

import (
	"context"
	"fmt"
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir/querier"
)

// MetricsQuerier resolves the Alloy head-series count for a cluster so that
// config rendering can stay a pure transformation. The controller runs this
// against Mimir; tests supply a deterministic fake.
type MetricsQuerier interface {
	QueryHeadSeries(ctx context.Context, cluster *clusterv1.Cluster) (float64, error)
}

// MimirQuerier is the production MetricsQuerier backed by Mimir.
type MimirQuerier struct {
	MetricsQueryURL string
	DefaultTenant   string
	QueryTimeout    time.Duration
}

// QueryHeadSeries returns the 6h max of the Alloy remote_write active series
// for the given cluster, which the sharding strategy uses to size replicas.
func (q MimirQuerier) QueryHeadSeries(ctx context.Context, cluster *clusterv1.Cluster) (float64, error) {
	query := fmt.Sprintf(
		`sum(max_over_time((sum(prometheus_remote_write_wal_storage_active_series{cluster_id="%s", service="%s"})by(pod))[6h:1h]))`,
		cluster.Name, apps.AlloyMetricsAppName,
	)
	return querier.QueryTSDBHeadSeries(ctx, query, q.MetricsQueryURL, q.DefaultTenant, q.QueryTimeout)
}
