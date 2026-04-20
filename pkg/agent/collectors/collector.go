package collectors

import (
	"context"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/credential"
)

// CollectorService is the common interface implemented by all Alloy collector services
// (metrics, logs, events). The controller uses this interface to reconcile all collectors
// uniformly without knowing which signal each one handles.
//
// Credentials are resolved once by the controller and passed in, so the render
// path has no credential-store I/O and no notion of readiness.
type CollectorService interface {
	ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version, caBundle string, creds credential.BackendCredentials) error
	ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error
}
