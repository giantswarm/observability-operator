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
