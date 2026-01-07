package monitoring

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

const (
	// DefaultServicePriority is the default service priority if not set.
	defaultServicePriority = "highest"
	// ServicePriorityLabel is the label used to determine the priority of a service.
	servicePriorityLabel = "giantswarm.io/service-priority"

	PriorityClassName = "giantswarm-critical"

	QueueConfigCapacity          = 30000
	QueueConfigMaxSamplesPerSend = 150000
	QueueConfigMaxShards         = 10
	QueueConfigSampleAgeLimit    = "30m"

	RemoteWriteName              = "mimir"
	MimirBaseURLFormat           = "https://mimir.%s"
	RemoteWriteEndpointURLFormat = MimirBaseURLFormat + "/api/v1/push"
	RemoteWriteTimeout           = "60s"

	// Loki configuration (TODO move elsewhere) and remove the write prefix eventually
	LokiBaseURLFormat = "https://write.loki.%s"
	LokiPushURLFormat = LokiBaseURLFormat + "/loki/api/v1/push"

	// TODO: move these Loki secret keys to a more appropriate location
	// Secret keys used for Loki authentication in Alloy configurations
	LokiURLKey         = "logging-url"
	LokiTenantIDKey    = "logging-tenant-id"
	LokiUsernameKey    = "logging-username"
	LokiPasswordKey    = "logging-password"
	LokiRulerAPIURLKey = "ruler-api-url"

	// Loki performance tuning constants
	LokiMaxBackoffPeriod = "10m"
	LokiRemoteTimeout    = "60s"

	// Tempo configuration (TODO move elsewhere)
	TempoIngressURLFormat = "tempo-gateway.%s"

	// TODO move elsewhere
	OrgIDHeader = "X-Scope-OrgID"
)

func GetServicePriority(cluster *clusterv1.Cluster) string {
	if servicePriority, ok := cluster.GetLabels()[servicePriorityLabel]; ok && servicePriority != "" {
		return servicePriority
	}
	return defaultServicePriority
}

func GetClusterShardingStrategy(cluster metav1.Object) (s *sharding.Strategy, err error) {
	var scaleUpSeriesCount, scaleDownPercentage float64
	if value, ok := cluster.GetAnnotations()["observability.giantswarm.io/monitoring-agent-scale-up-series-count"]; ok {
		if scaleUpSeriesCount, err = strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("failed to parse scale-up series count: %w", err)
		}
	}
	if value, ok := cluster.GetAnnotations()["observability.giantswarm.io/monitoring-agent-scale-down-percentage"]; ok {
		if scaleDownPercentage, err = strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("failed to parse scale-down percentage: %w", err)
		}
	}

	s = &sharding.Strategy{
		ScaleUpSeriesCount:  scaleUpSeriesCount,
		ScaleDownPercentage: scaleDownPercentage,
	}

	return s, nil
}
