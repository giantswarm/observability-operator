package alloy

import (
	"context"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/config"
)

// EventsService manages Alloy events logger configuration for workload clusters.
// It creates and manages ConfigMaps and Secrets for Kubernetes event collection.
type EventsService struct {
	client             client.Client
	config             config.Config
	logsAuthManager    auth.AuthManager
	tracesAuthManager  auth.AuthManager
}

// NewEventsService creates a new EventsService instance.
// Note: Events logging requires both logs and traces auth managers since events
// can be sent to both Loki (logs) and Tempo (traces).
func NewEventsService(
	client client.Client,
	cfg config.Config,
	logsAuthManager auth.AuthManager,
	tracesAuthManager auth.AuthManager,
) *EventsService {
	return &EventsService{
		client:            client,
		config:            cfg,
		logsAuthManager:   logsAuthManager,
		tracesAuthManager: tracesAuthManager,
	}
}

// ReconcileCreate creates or updates the Alloy events configuration for a cluster.
// This includes:
// - Creating the events secret with Loki/Tempo write credentials
// - Creating the events ConfigMap with Alloy configuration
func (s *EventsService) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	// TODO: Implement events configuration creation
	// 1. Get auth credentials from logsAuthManager and tracesAuthManager
	// 2. Generate Alloy events configuration from template
	// 3. Create/update Secret with credentials
	// 4. Create/update ConfigMap with Alloy config
	return nil
}

// ReconcileDelete removes the Alloy events configuration for a cluster.
// This includes:
// - Deleting the events ConfigMap
// - Deleting the events Secret
func (s *EventsService) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	// TODO: Implement events configuration deletion
	// 1. Delete events ConfigMap
	// 2. Delete events Secret
	return nil
}
