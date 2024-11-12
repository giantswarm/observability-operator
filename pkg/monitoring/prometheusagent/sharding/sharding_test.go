package sharding

import (
	"testing"
)

type testCase struct {
	currentShardCount int
	timeSeries        float64
	expected          int
}

var defaultShardingStrategy = Strategy{ScaleUpSeriesCount: float64(1_000_000), ScaleDownPercentage: float64(0.20)}

var tests = []struct {
	name     string
	strategy Strategy
	cases    []testCase
}{
	{
		// Testing scale up strategy
		// Scale up triggers when the number of time series is greater than the scale up series count
		name:     "scale up",
		strategy: defaultShardingStrategy,
		cases: []testCase{
			// Test cases when reaching the threshold
			{
				currentShardCount: 0,
				timeSeries:        float64(1_000_000),
				expected:          1,
			},
			{
				currentShardCount: 0,
				timeSeries:        float64(2_000_000),
				expected:          2,
			},
			{
				currentShardCount: 0,
				timeSeries:        float64(3_000_000),
				expected:          3,
			},
			// Test cases when crossing above threshold
			{
				currentShardCount: 0,
				timeSeries:        float64(1_000_001),
				expected:          2,
			},
			{
				currentShardCount: 0,
				timeSeries:        float64(2_000_001),
				expected:          3,
			},
			{
				currentShardCount: 0,
				timeSeries:        float64(3_000_001),
				expected:          4,
			},
		},
	},
	{
		name:     "scale down",
		strategy: defaultShardingStrategy,
		cases: []testCase{
			// Test cases when scale down threshold is hit
			{
				currentShardCount: 2,
				timeSeries:        float64(800_000),
				expected:          1,
			},
			{
				currentShardCount: 3,
				timeSeries:        float64(1_800_000),
				expected:          2,
			},
			{
				currentShardCount: 4,
				timeSeries:        float64(2_800_000),
				expected:          3,
			},
			// Test cases when above scale down threshold
			{
				currentShardCount: 2,
				timeSeries:        float64(800_001),
				expected:          2,
			},
			{
				currentShardCount: 3,
				timeSeries:        float64(1_800_001),
				expected:          3,
			},
			{
				currentShardCount: 4,
				timeSeries:        float64(2_800_001),
				expected:          4,
			},
		},
	},
	{
		name:     "keep current shards when no time series",
		strategy: defaultShardingStrategy,
		cases: []testCase{
			{
				currentShardCount: 0,
				timeSeries:        float64(-5),
				expected:          1,
			},
			{
				currentShardCount: 0,
				timeSeries:        float64(0),
				expected:          1,
			},
			{
				currentShardCount: 1,
				timeSeries:        float64(0),
				expected:          1,
			},
			{
				currentShardCount: 2,
				timeSeries:        float64(0),
				expected:          2,
			},
			{
				currentShardCount: 3,
				timeSeries:        float64(0),
				expected:          3,
			},
		},
	},
}

func TestShardComputationLogic(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.cases {
				result := tt.strategy.ComputeShards(c.currentShardCount, c.timeSeries)
				if result != c.expected {
					t.Errorf(`expected computeShards(%d, %f) to be %d, got %d`, c.currentShardCount, c.timeSeries, c.expected, result)
				}
				t.Logf(`computeShards(%d, %f) = %d`, c.currentShardCount, c.timeSeries, result)
			}
		})
	}
}
