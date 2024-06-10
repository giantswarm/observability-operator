package sharding

import "math"

type Strategy struct {
	// Configures the number of series needed to add a new shard. Computation is number of series / ScaleUpSeriesCount
	ScaleUpSeriesCount float64
	// Percentage of needed series based on ScaleUpSeriesCount to scale down agents
	ScaleDownPercentage float64
}

func (s Strategy) Merge(newStrategy *Strategy) Strategy {
	strategy := Strategy{
		s.ScaleUpSeriesCount,
		s.ScaleDownPercentage,
	}
	if newStrategy != nil {
		if newStrategy.ScaleUpSeriesCount > 0 {
			strategy.ScaleUpSeriesCount = newStrategy.ScaleUpSeriesCount
		}
		if newStrategy.ScaleDownPercentage > 0 {
			strategy.ScaleDownPercentage = newStrategy.ScaleDownPercentage
		}
	}
	return strategy
}

// We want to start with 1 prometheus-agent for each 1M time series with a scale down 20% threshold.
func (pass Strategy) ComputeShards(currentShardCount int, timeSeries float64) int {
	shardScaleDownThreshold := pass.ScaleDownPercentage * pass.ScaleUpSeriesCount
	desiredShardCount := int(math.Ceil(timeSeries / pass.ScaleUpSeriesCount))

	// Compute Scale Down
	if currentShardCount > desiredShardCount {
		// Check if the remaining time series from ( timeSeries mod ScaleupSeriesCount ) is bigger than the scale down threshold.
		if math.Mod(timeSeries, pass.ScaleUpSeriesCount) > pass.ScaleUpSeriesCount-shardScaleDownThreshold {
			desiredShardCount = currentShardCount
		}
	}

	// We always have a minimum of 1 agent, even if there is no worker node
	if desiredShardCount <= 0 {
		return 1
	}
	return desiredShardCount
}
