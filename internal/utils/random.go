package utils

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"sync"
	"time"
)

// Random provides a deterministic pseudo-random number generator with
// convenient methods for common generation tasks. It's designed to be
// reproducible given the same seed.
type Random struct {
	rng  *rand.Rand
	seed uint64
	mu   sync.Mutex
}

// NewRandom creates a new Random instance with the given seed.
// If seed is 0, a cryptographically random seed is generated.
func NewRandom(seed int64) *Random {
	var actualSeed uint64
	if seed == 0 {
		actualSeed = generateRandomSeed()
	} else {
		actualSeed = uint64(seed)
	}

	return &Random{
		rng:  rand.New(rand.NewPCG(actualSeed, actualSeed^0xDEADBEEF)),
		seed: actualSeed,
	}
}

// generateRandomSeed creates a cryptographically random seed
func generateRandomSeed() uint64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		return uint64(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint64(b[:])
}

// Seed returns the seed used to initialize this RNG
func (r *Random) Seed() uint64 {
	return r.seed
}

// Fork creates a new Random instance with a derived seed.
// Useful for creating independent RNG streams for parallel processing
// while maintaining reproducibility.
func (r *Random) Fork() *Random {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate a new seed from the current RNG
	newSeed := r.rng.Uint64()
	return &Random{
		rng:  rand.New(rand.NewPCG(newSeed, newSeed^0xCAFEBABE)),
		seed: newSeed,
	}
}

// ForkN creates N independent Random instances with derived seeds.
// Useful for spawning worker goroutines that each need their own RNG.
func (r *Random) ForkN(n int) []*Random {
	results := make([]*Random, n)
	for i := 0; i < n; i++ {
		results[i] = r.Fork()
	}
	return results
}

// Int returns a non-negative pseudo-random int
func (r *Random) Int() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int(r.rng.Uint64() >> 1) // Shift to ensure non-negative
}

// IntN returns a pseudo-random int in [0, n)
func (r *Random) IntN(n int) int {
	if n <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.IntN(n)
}

// IntRange returns a pseudo-random int in [min, max]
func (r *Random) IntRange(min, max int) int {
	if min >= max {
		return min
	}
	return min + r.IntN(max-min+1)
}

// Int64N returns a pseudo-random int64 in [0, n)
func (r *Random) Int64N(n int64) int64 {
	if n <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Int64N(n)
}

// Int64Range returns a pseudo-random int64 in [min, max]
func (r *Random) Int64Range(min, max int64) int64 {
	if min >= max {
		return min
	}
	return min + r.Int64N(max-min+1)
}

// Float64 returns a pseudo-random float64 in [0.0, 1.0)
func (r *Random) Float64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Float64()
}

// Float64Range returns a pseudo-random float64 in [min, max)
func (r *Random) Float64Range(min, max float64) float64 {
	if min >= max {
		return min
	}
	return min + r.Float64()*(max-min)
}

// Bool returns a pseudo-random boolean
func (r *Random) Bool() bool {
	return r.IntN(2) == 1
}

// Probability returns true with the given probability (0.0 to 1.0)
func (r *Random) Probability(p float64) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	return r.Float64() < p
}

// PickString returns a random string from the slice
func (r *Random) PickString(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	return slice[r.IntN(len(slice))]
}

// PickInt returns a random int from the slice
func (r *Random) PickInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	return slice[r.IntN(len(slice))]
}

// ShuffleStrings randomly reorders elements in the string slice in-place
func (r *Random) ShuffleStrings(slice []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(slice) - 1; i > 0; i-- {
		j := r.rng.IntN(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// ShuffleInts randomly reorders elements in the int slice in-place
func (r *Random) ShuffleInts(slice []int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(slice) - 1; i > 0; i-- {
		j := r.rng.IntN(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// WeightedPick selects an index based on weights
// weights[i] is the relative weight for index i
func (r *Random) WeightedPick(weights []int) int {
	if len(weights) == 0 {
		return -1
	}

	total := 0
	for _, w := range weights {
		total += w
	}

	if total <= 0 {
		return r.IntN(len(weights))
	}

	target := r.IntN(total) + 1
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if target <= cumulative {
			return i
		}
	}

	return len(weights) - 1
}

// NormalFloat64 returns a normally distributed float64 with mean 0 and stddev 1
// Uses the Box-Muller transform
func (r *Random) NormalFloat64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.NormFloat64()
}

// NormalFloat64Range returns a normally distributed float64 with given mean and stddev
func (r *Random) NormalFloat64Range(mean, stddev float64) float64 {
	return mean + r.NormalFloat64()*stddev
}

// ExpFloat64 returns an exponentially distributed float64 with rate 1
func (r *Random) ExpFloat64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.ExpFloat64()
}

// Duration returns a random duration in [min, max]
func (r *Random) Duration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	return min + time.Duration(r.Int64N(int64(max-min+1)))
}

// Date returns a random date between start and end (inclusive)
func (r *Random) Date(start, end time.Time) time.Time {
	if !start.Before(end) {
		return start
	}
	delta := end.Sub(start)
	return start.Add(r.Duration(0, delta))
}

// DateInRange returns a random date within the given number of days from now
// daysBack: how many days in the past (positive number)
func (r *Random) DateInPast(daysBack int) time.Time {
	if daysBack <= 0 {
		return time.Now()
	}
	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	return r.Date(start, end)
}

// String generates a random alphanumeric string of the given length
func (r *Random) String(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[r.IntN(len(charset))]
	}
	return string(result)
}

// NumericString generates a random numeric string of the given length
func (r *Random) NumericString(length int) string {
	const charset = "0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[r.IntN(len(charset))]
	}
	return string(result)
}

// Digit returns a random digit character ('0'-'9')
func (r *Random) Digit() byte {
	return '0' + byte(r.IntN(10))
}

// Letter returns a random uppercase letter ('A'-'Z')
func (r *Random) Letter() byte {
	return 'A' + byte(r.IntN(26))
}
