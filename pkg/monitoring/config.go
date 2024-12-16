package monitoring

import (
	"strconv"
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/sharding"
)

const MonitoringLabel = "giantswarm.io/monitoring"

// Config represents the configuration used by the monitoring package.
type Config struct {
	Enabled bool

	AlertmanagerURL string

	MonitoringAgent         string
	DefaultShardingStrategy sharding.Strategy
	// WALTruncateFrequency is the frequency at which the WAL segments should be truncated.
	WALTruncateFrequency time.Duration
	// TODO(atlas): validate prometheus version using SemVer
	PrometheusVersion string
}

// Monitoring should be enabled when all conditions are met:
//   - global monitoring flag is enabled
//   - monitoring label is not set or is set to true on the cluster object
func (c Config) IsMonitored(cluster *clusterv1.Cluster) bool {
	if !c.Enabled {
		return false
	}

	// Check if label is set on the cluster object
	labels := cluster.GetLabels()
	monitoringLabelValue, ok := labels[MonitoringLabel]
	if !ok {
		// If it's not set, monitoring is enabled by default
		return true
	}

	monitoringEnabled, err := strconv.ParseBool(monitoringLabelValue)
	if err != nil {
		return true
	}
	return monitoringEnabled
}
