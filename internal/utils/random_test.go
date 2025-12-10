package utils

import (
	"testing"
	"time"
)

func TestRandomReproducibility(t *testing.T) {
	seed := int64(42)

	// Create two RNGs with the same seed
	rng1 := NewRandom(seed)
	rng2 := NewRandom(seed)

	// Verify they produce identical sequences
	t.Run("IntN", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			v1 := rng1.IntN(1000)
			v2 := rng2.IntN(1000)
			if v1 != v2 {
				t.Errorf("Mismatch at iteration %d: %d != %d", i, v1, v2)
				return
			}
		}
	})

	// Reset with new RNGs
	rng1 = NewRandom(seed)
	rng2 = NewRandom(seed)

	t.Run("Float64", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			v1 := rng1.Float64()
			v2 := rng2.Float64()
			if v1 != v2 {
				t.Errorf("Mismatch at iteration %d: %f != %f", i, v1, v2)
				return
			}
		}
	})

	// Reset with new RNGs
	rng1 = NewRandom(seed)
	rng2 = NewRandom(seed)

	t.Run("Mixed operations", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			if rng1.IntN(100) != rng2.IntN(100) {
				t.Error("IntN mismatch")
				return
			}
			if rng1.Float64() != rng2.Float64() {
				t.Error("Float64 mismatch")
				return
			}
			if rng1.Bool() != rng2.Bool() {
				t.Error("Bool mismatch")
				return
			}
			if rng1.IntRange(10, 20) != rng2.IntRange(10, 20) {
				t.Error("IntRange mismatch")
				return
			}
		}
	})
}

func TestRandomSeedStorage(t *testing.T) {
	// Test explicit seed
	rng := NewRandom(12345)
	if rng.Seed() != 12345 {
		t.Errorf("Expected seed 12345, got %d", rng.Seed())
	}

	// Test auto-generated seed (seed 0)
	rng = NewRandom(0)
	if rng.Seed() == 0 {
		t.Error("Expected non-zero auto-generated seed")
	}
}

func TestRandomFork(t *testing.T) {
	seed := int64(42)
	rng1 := NewRandom(seed)
	rng2 := NewRandom(seed)

	// Fork both in the same order
	fork1a := rng1.Fork()
	fork1b := rng1.Fork()
	fork2a := rng2.Fork()
	fork2b := rng2.Fork()

	// Forked RNGs from the same parent should produce the same sequences
	for i := 0; i < 100; i++ {
		if fork1a.IntN(1000) != fork2a.IntN(1000) {
			t.Error("Fork A sequences don't match")
			return
		}
		if fork1b.IntN(1000) != fork2b.IntN(1000) {
			t.Error("Fork B sequences don't match")
			return
		}
	}
}

func TestRandomForkN(t *testing.T) {
	seed := int64(42)
	rng1 := NewRandom(seed)
	rng2 := NewRandom(seed)

	forks1 := rng1.ForkN(5)
	forks2 := rng2.ForkN(5)

	// Each corresponding fork should produce the same sequence
	for i := range forks1 {
		for j := 0; j < 100; j++ {
			if forks1[i].IntN(1000) != forks2[i].IntN(1000) {
				t.Errorf("Fork %d sequences don't match at iteration %d", i, j)
				return
			}
		}
	}
}

func TestRandomRanges(t *testing.T) {
	rng := NewRandom(42)

	t.Run("IntRange", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			v := rng.IntRange(10, 20)
			if v < 10 || v > 20 {
				t.Errorf("IntRange(10, 20) returned %d", v)
			}
		}
	})

	t.Run("Int64Range", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			v := rng.Int64Range(100, 200)
			if v < 100 || v > 200 {
				t.Errorf("Int64Range(100, 200) returned %d", v)
			}
		}
	})

	t.Run("Float64Range", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			v := rng.Float64Range(1.0, 2.0)
			if v < 1.0 || v >= 2.0 {
				t.Errorf("Float64Range(1.0, 2.0) returned %f", v)
			}
		}
	})

	t.Run("Duration", func(t *testing.T) {
		min := 100 * time.Millisecond
		max := 500 * time.Millisecond
		for i := 0; i < 1000; i++ {
			v := rng.Duration(min, max)
			if v < min || v > max {
				t.Errorf("Duration(%v, %v) returned %v", min, max, v)
			}
		}
	})
}

func TestRandomProbability(t *testing.T) {
	rng := NewRandom(42)

	// Probability(0) should always return false
	for i := 0; i < 100; i++ {
		if rng.Probability(0) {
			t.Error("Probability(0) returned true")
		}
	}

	// Probability(1) should always return true
	for i := 0; i < 100; i++ {
		if !rng.Probability(1) {
			t.Error("Probability(1) returned false")
		}
	}

	// Probability(0.5) should return roughly 50% true
	trueCount := 0
	iterations := 10000
	for i := 0; i < iterations; i++ {
		if rng.Probability(0.5) {
			trueCount++
		}
	}
	ratio := float64(trueCount) / float64(iterations)
	if ratio < 0.45 || ratio > 0.55 {
		t.Errorf("Probability(0.5) returned %.2f%% true, expected ~50%%", ratio*100)
	}
}

func TestRandomPick(t *testing.T) {
	rng := NewRandom(42)

	t.Run("PickString", func(t *testing.T) {
		slice := []string{"a", "b", "c", "d", "e"}
		counts := make(map[string]int)
		for i := 0; i < 1000; i++ {
			v := rng.PickString(slice)
			counts[v]++
		}
		// Each element should be picked at least once
		for _, s := range slice {
			if counts[s] == 0 {
				t.Errorf("Element '%s' was never picked", s)
			}
		}
	})

	t.Run("PickString empty", func(t *testing.T) {
		v := rng.PickString([]string{})
		if v != "" {
			t.Errorf("PickString on empty slice returned '%s', expected ''", v)
		}
	})
}

func TestRandomWeightedPick(t *testing.T) {
	rng := NewRandom(42)

	// Test with heavily skewed weights
	weights := []int{1, 1, 1, 1000}
	counts := make([]int, len(weights))

	iterations := 10000
	for i := 0; i < iterations; i++ {
		idx := rng.WeightedPick(weights)
		counts[idx]++
	}

	// Index 3 (weight 1000) should be picked much more often
	if counts[3] < 9000 {
		t.Errorf("Weighted pick: expected index 3 to be picked >9000 times, got %d", counts[3])
	}
}

func TestRandomNumericString(t *testing.T) {
	rng := NewRandom(42)

	str := rng.NumericString(10)
	if len(str) != 10 {
		t.Errorf("NumericString(10) returned length %d", len(str))
	}

	for _, c := range str {
		if c < '0' || c > '9' {
			t.Errorf("NumericString contained non-digit: %c", c)
		}
	}
}

func TestRandomString(t *testing.T) {
	rng := NewRandom(42)

	str := rng.String(20)
	if len(str) != 20 {
		t.Errorf("String(20) returned length %d", len(str))
	}

	for _, c := range str {
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlphaNum {
			t.Errorf("String contained non-alphanumeric: %c", c)
		}
	}
}
