package heartbeat

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/opsgenie/opsgenie-go-sdk-v2/heartbeat"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/config"
)

// OpsgenieHeartbeatRepository is a repository for managing heartbeats in Opsgenie.
type OpsgenieHeartbeatRepository struct {
	*heartbeat.Client
	config.Config
}

// NewOpsgenieHeartbeatRepository creates a new OpsgenieHeartbeatRepository.
func NewOpsgenieHeartbeatRepository(apiKey string, cfg config.Config) (HeartbeatRepository, error) {
	c := &client.Config{
		ApiKey:         apiKey,
		OpsGenieAPIURL: client.API_URL,
		RetryCount:     1,
		LogLevel:       logrus.FatalLevel,
	}

	client, err := heartbeat.NewClient(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create heartbeat client: %w", err)
	}
	return &OpsgenieHeartbeatRepository{client, cfg}, nil
}

func (r *OpsgenieHeartbeatRepository) CreateOrUpdate(ctx context.Context) error {
	return r.Delete(ctx)
}

func (r *OpsgenieHeartbeatRepository) Delete(ctx context.Context) error {
	logger := log.FromContext(ctx)

	logger.Info("checking if heartbeat exists")
	_, err := r.Get(ctx, r.Config.Cluster.Name)
	if err != nil {
		apiErr, ok := err.(*client.ApiError)
		if ok && apiErr.StatusCode == http.StatusNotFound {
			logger.Info("heartbeat does not exist, skipping")
			return nil
		}
		return fmt.Errorf("failed to get heartbeat: %w", err)
	}

	// The final ping to the heartbeat cleans up any opened heartbeat alerts for the cluster being deleted.
	logger.Info("triggering final heartbeat ping")
	_, err = r.Ping(ctx, r.Config.Cluster.Name)
	if err != nil {
		return fmt.Errorf("failed to ping heartbeat: %w", err)
	}
	logger.Info("triggered final heartbeat ping")

	logger.Info("deleting heartbeat")
	_, err = r.Client.Delete(ctx, r.Config.Cluster.Name)
	if err != nil {
		return fmt.Errorf("failed to delete heartbeat: %w", err)
	}
	logger.Info("deleted heartbeat")
	return nil
}
