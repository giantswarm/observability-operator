package monitoring

import "github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/sharding"

// Config represents the configuration used by the monitoring package.
type Config struct {
	Enabled                 bool
	DefaultShardingStrategy sharding.Strategy
	// TODO(atlas): validate prometheus version using SemVer
	PrometheusVersion string
}
