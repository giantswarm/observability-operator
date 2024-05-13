package sharding

import "math"

type ShardingStrategy struct {
	// Configures the number of series needed to add a new shard. Computation is number of series / ScaleUpSeriesCount
	ScaleUpSeriesCount float64
	// Percentage of needed series based on ScaleUpSeriesCount to scale down agents
	ScaleDownPercentage float64
}

func (pass1 ShardingStrategy) Merge(pass2 *ShardingStrategy) ShardingStrategy {
	strategy := ShardingStrategy{
		pass1.ScaleUpSeriesCount,
		pass1.ScaleDownPercentage,
	}
	if pass2 != nil {
		if pass2.ScaleUpSeriesCount > 0 {
			strategy.ScaleUpSeriesCount = pass2.ScaleUpSeriesCount
		}
		if pass2.ScaleDownPercentage > 0 {
			strategy.ScaleDownPercentage = pass2.ScaleDownPercentage
		}
	}
	return strategy
}

// We want to start with 1 prometheus-agent for each 1M time series with a scale down 20% threshold.
func (pass ShardingStrategy) ComputeShards(currentShardCount int, timeSeries float64) int {
	shardScaleDownThreshold := pass.ScaleDownPercentage * pass.ScaleUpSeriesCount
	desiredShardCount := int(math.Ceil(timeSeries / pass.ScaleUpSeriesCount))

	// Compute Scale Down
	if currentShardCount > desiredShardCount {
		// We get the rest of a division of timeSeries by shardStep and we compare it with the scale down threshold
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
