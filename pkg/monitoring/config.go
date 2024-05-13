package monitoring

// Config represents the configuration used by the monitoring package.
type Config struct {
	Enabled                     bool
	ShardingScaleUpSeriesCount  float64
	ShardingScaleDownPercentage float64
	// TODO(atlas): validate prometheus version
	PrometheusVersion string
}
