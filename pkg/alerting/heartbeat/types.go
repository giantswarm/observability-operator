package heartbeat

import (
	"context"
)

// HeartbeatRepository is the interface for the heartbeat repository.
// It provides methods to create or update and delete a heartbeat.
// The heartbeat is used by the monitoring system to detect if the management cluster is alive.
// The current implementation relies on OpsGenie but other implementations can be added in the future.
type HeartbeatRepository interface {
	// CreateOrUpdate creates or updates the heartbeat for the management cluster.
	CreateOrUpdate(ctx context.Context) error

	// Delete deletes the heartbeat for the management cluster.
	Delete(ctx context.Context) error
}
