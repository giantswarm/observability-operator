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
		name:     "scale up",
		strategy: defaultShardingStrategy,
		cases: []testCase{
			{
				currentShardCount: 0,
				timeSeries:        float64(1_000_000),
				expected:          1,
			},
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
		},
	},
	{
		name:     "scale down",
		strategy: defaultShardingStrategy,
		cases: []testCase{
			{
				currentShardCount: 1,
				timeSeries:        float64(1_000_001),
				expected:          2,
			},
			{
				currentShardCount: 2,
				timeSeries:        float64(999_999),
				expected:          2,
			},
			{
				currentShardCount: 2,
				timeSeries:        float64(800_001),
				expected:          2,
			},
			{
				currentShardCount: 2,
				// 20% default threshold hit
				timeSeries: float64(800_000),
				expected:   1,
			},
		},
	},
	{
		name:     "always defaults to 1",
		strategy: defaultShardingStrategy,
		cases: []testCase{
			{
				currentShardCount: 0,
				timeSeries:        float64(0),
				expected:          1,
			},
			{
				currentShardCount: 0,
				timeSeries:        float64(-5),
				expected:          1,
			},
		},
	},
}

func TestShardComputationLogic(t *testing.T) {
	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, c := range tt.cases {
				c := c
				result := tt.strategy.ComputeShards(c.currentShardCount, c.timeSeries)
				if result != c.expected {
					t.Errorf(`expected computeShards(%d, %f) to be %d, got %d`, c.currentShardCount, c.timeSeries, c.expected, result)
				}
				t.Logf(`computeShards(%d, %f) = %d`, c.currentShardCount, c.timeSeries, result)
			}
		})
	}
}
