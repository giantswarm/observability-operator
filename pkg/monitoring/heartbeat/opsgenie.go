package heartbeat

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"

	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/opsgenie/opsgenie-go-sdk-v2/heartbeat"
	"github.com/opsgenie/opsgenie-go-sdk-v2/og"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common"
)

// OpsgenieHeartbeatRepository is a repository for managing heartbeats in Opsgenie.
type OpsgenieHeartbeatRepository struct {
	*heartbeat.Client
	common.ManagementCluster
}

// NewOpsgenieHeartbeatRepository creates a new OpsgenieHeartbeatRepository.
func NewOpsgenieHeartbeatRepository(apiKey string, mc common.ManagementCluster) (HeartbeatRepository, error) {
	c := &client.Config{
		ApiKey:         apiKey,
		OpsGenieAPIURL: client.API_URL,
		RetryCount:     1,
		LogLevel:       logrus.FatalLevel,
	}

	client, err := heartbeat.NewClient(c)
	return &OpsgenieHeartbeatRepository{client, mc}, err
}

// makeHeartbeat creates a new heartbeat for the management cluster.
func (r OpsgenieHeartbeatRepository) makeHeartbeat() *heartbeat.Heartbeat {
	tags := []string{
		"team: atlas",
		fmt.Sprintf("installation: %s", r.ManagementCluster.Name),
		"managed-by: observability-operator",
		fmt.Sprintf("pipeline: %s", r.ManagementCluster.Pipeline),
	}
	// Tags need to be sorted alphabetically to avoid unnecessary heartbeat updates
	sort.Strings(tags)

	return &heartbeat.Heartbeat{
		Name:         r.ManagementCluster.Name,
		Description:  "📗 Runbook: https://intranet.giantswarm.io/docs/support-and-ops/ops-recipes/heartbeat-expired/",
		Interval:     60,
		IntervalUnit: string(heartbeat.Minutes),
		Enabled:      true,
		Expired:      false,
		OwnerTeam: og.OwnerTeam{
			Name: "alerts_router_team",
		},
		AlertTags:     tags,
		AlertPriority: "P3",
		AlertMessage:  fmt.Sprintf("Heartbeat [%s] is expired.", r.ManagementCluster.Name),
	}
}

func (r *OpsgenieHeartbeatRepository) CreateOrUpdate(ctx context.Context) error {
	logger := log.FromContext(ctx)

	hb := r.makeHeartbeat()

	// By default, we consider the heartbeat exists
	var heartbeatExists = true
	logger.Info("checking if heartbeat is already configured")
	getResult, err := r.Client.Get(ctx, hb.Name)
	if err != nil {
		apiErr, ok := err.(*client.ApiError)
		// If the error is not a 404, we return it
		if !ok || apiErr.StatusCode != http.StatusNotFound {
			return errors.WithStack(err)
		}
		// If the heartbeat does not exist, we set the heartbeatExists to false
		heartbeatExists = false
	}

	if heartbeatExists {
		// If the heartbeat does not need to be updated, we leave early
		if !hasChanged(getResult.Heartbeat, *hb) {
			logger.Info("heartbeat is up to date")
			return nil
		}

		// We need to delete and recreate it because the update is a PATCH (so existing alert tags are kept)
		// This caused issue when installation pipeline was switched from testing to stable.
		logger.Info("heartbeat has changed and needs to be reconfigured")

		logger.Info("deleting heartbeat")
		_, err := r.Client.Delete(ctx, hb.Name)
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("deleted heartbeat")
	}

	logger.Info("creating heartbeat")
	err = r.createHeartbeat(ctx, hb)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("created heartbeat")

	return nil
}

func (r *OpsgenieHeartbeatRepository) Delete(ctx context.Context) error {
	logger := log.FromContext(ctx)

	logger.Info("checking if heartbeat exists")
	_, err := r.Client.Get(ctx, r.ManagementCluster.Name)
	if err != nil {
		apiErr, ok := err.(*client.ApiError)
		if ok && apiErr.StatusCode == http.StatusNotFound {
			logger.Info("heartbeat does not exist, skipping")
			return nil
		} else {
			return errors.WithStack(err)
		}
	}

	// The final ping to the heartbeat cleans up any opened heartbeat alerts for the cluster being deleted.
	logger.Info("triggering final heartbeat ping")
	_, err = r.Client.Ping(ctx, r.ManagementCluster.Name)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("triggered final heartbeat ping")

	logger.Info("deleting heartbeat")
	_, err = r.Client.Delete(ctx, r.ManagementCluster.Name)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Info("deleted heartbeat")
	return nil
}

// createHeartbeat creates a new heartbeat in Opsgenie.
func (r *OpsgenieHeartbeatRepository) createHeartbeat(ctx context.Context, h *heartbeat.Heartbeat) error {
	req := &heartbeat.AddRequest{
		Name:          h.Name,
		Description:   h.Description,
		Interval:      h.Interval,
		IntervalUnit:  heartbeat.Unit(h.IntervalUnit),
		Enabled:       &h.Enabled,
		OwnerTeam:     h.OwnerTeam,
		AlertMessage:  h.AlertMessage,
		AlertTag:      h.AlertTags,
		AlertPriority: h.AlertPriority,
	}
	_, err := r.Client.Add(ctx, req)
	if err != nil {
		return errors.WithStack(err)
	}

	// We ping the heartbeat to active it and make sure it pages.
	_, err = r.Client.Ping(ctx, h.Name)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// hasChanged returns true if the current heartbeat is different from the desired heartbeat.
func hasChanged(current, desired heartbeat.Heartbeat) bool {
	// Ignore those fields for comparison by setting them to the same value.
	current.Enabled = true
	desired.Enabled = true
	current.Expired = true
	desired.Expired = true
	// We get the ID back from opsgenie so we update it in the heartbeat
	desired.OwnerTeam.Id = current.OwnerTeam.Id

	return !reflect.DeepEqual(current, desired)
}
