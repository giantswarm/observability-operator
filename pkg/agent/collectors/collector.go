package collectors

import (
	"context"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// CollectorService is the common interface implemented by all Alloy collector services
// (metrics, logs, events). The controller uses this interface to reconcile all collectors
// uniformly without knowing which signal each one handles.
type CollectorService interface {
	ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version, caBundle string) error
	ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error
}
