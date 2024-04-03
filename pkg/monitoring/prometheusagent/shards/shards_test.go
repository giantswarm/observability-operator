package shards

import (
	"flag"
	"testing"
)

var _ = flag.Bool("update", false, "update the output file")

func TestShardComputationScaleUp(t *testing.T) {
	expected := 1
	result := ComputeShards(0, float64(1_000_000))
	if result != expected {
		t.Errorf(`expected ComputeShards(0, 1_000_000) to be %d, got %d`, expected, result)
	}

	expected = 2
	result = ComputeShards(0, float64(1_000_001))
	if result != expected {
		t.Errorf(`expected ComputeShards(0, 1_000_001) to be %d, got %d`, expected, result)
	}

	expected = 3
	result = ComputeShards(0, float64(2_000_001))
	if result != expected {
		t.Errorf(`expected ComputeShards(0, 2_000_001) to be %d, got %d`, expected, result)
	}
}

func TestShardComputationReturnsAtLeast1Shart(t *testing.T) {
	expected := 1
	result := ComputeShards(0, 0)
	if result != expected {
		t.Errorf(`expected ComputeShards(0, 0) to be %d, got %d`, expected, result)
	}

	expected = 1
	result = ComputeShards(0, -5)
	if result != expected {
		t.Errorf(`expected ComputeShards(0, -5) to be %d, got %d`, expected, result)
	}
}

func TestShardComputationScaleDown(t *testing.T) {
	expected := 2
	result := ComputeShards(1, 1_000_001)
	if result != expected {
		t.Errorf(`expected ComputeShards(1, 1_000_001) to be %d, got %d`, expected, result)
	}

	expected = 2
	result = ComputeShards(2, 999_999)
	if result != expected {
		t.Errorf(`expected ComputeShards(2, 999_999) to be %d, got %d`, expected, result)
	}

	expected = 2
	result = ComputeShards(2, 800_001)
	if result != expected {
		t.Errorf(`expected ComputeShards(2, 800_001) to be %d, got %d`, expected, result)
	}

	// threshold hit
	expected = 1
	result = ComputeShards(2, 800_000)
	if result != expected {
		t.Errorf(`expected ComputeShards(2, 800_000) to be %d, got %d`, expected, result)
	}
}
