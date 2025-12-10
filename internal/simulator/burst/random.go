package burst

import (
	"sync"
	"time"

	"github.com/willfong/load-generator/internal/utils"
)

// RandomBurst implements random spike generation for stress testing.
// This simulates unpredictable events like:
// - Flash sales or promotional events
// - Breaking news causing rush to check accounts
// - System recovery causing backlog flush
// - Viral social media mentions
type RandomBurst struct {
	config BurstConfig
	mu     sync.RWMutex
	rng    *utils.Random

	// Cooldown tracking - don't spam random bursts
	lastBurstTime  time.Time
	minCooldown    time.Duration
	checkInterval  int // Number of checks between potential bursts
	checkCounter   int

	// Configurable burst characteristics
	minDuration time.Duration
	maxDuration time.Duration
	minMultiplier float64
	maxMultiplier float64
}

// NewRandomBurst creates a new random spike generator
func NewRandomBurst(cfg BurstConfig, seed int64) *RandomBurst {
	rng := utils.NewRandom(seed)

	return &RandomBurst{
		config:        cfg,
		rng:           rng,
		minCooldown:   15 * time.Minute, // At least 15 min between random bursts
		checkInterval: 10,               // Only check every 10th call
		minDuration:   5 * time.Minute,
		maxDuration:   30 * time.Minute,
		minMultiplier: 1.5,
		maxMultiplier: 4.0,
	}
}

// Type implements BurstProvider
func (rb *RandomBurst) Type() BurstType {
	return BurstTypeRandom
}

// Configure implements BurstProvider
func (rb *RandomBurst) Configure(cfg BurstConfig) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.config = cfg
}

// SetCooldown configures the minimum time between random bursts
func (rb *RandomBurst) SetCooldown(d time.Duration) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.minCooldown = d
}

// SetDurationRange configures the min/max duration for random bursts
func (rb *RandomBurst) SetDurationRange(min, max time.Duration) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.minDuration = min
	rb.maxDuration = max
}

// SetMultiplierRange configures the min/max multiplier for random bursts
func (rb *RandomBurst) SetMultiplierRange(min, max float64) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.minMultiplier = min
	rb.maxMultiplier = max
}

// CheckBurst implements BurstProvider
// Uses probability-based random triggering with cooldown protection
func (rb *RandomBurst) CheckBurst(timezone string) *BurstEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.config.Enabled {
		return nil
	}

	// Only check every N calls to reduce overhead
	rb.checkCounter++
	if rb.checkCounter < rb.checkInterval {
		return nil
	}
	rb.checkCounter = 0

	// Check cooldown
	if time.Since(rb.lastBurstTime) < rb.minCooldown {
		return nil
	}

	// Probability check
	probability := rb.config.Probability
	if probability <= 0 {
		probability = 0.01 // Default 1% chance per check (after interval)
	}

	if rb.rng.Float64() >= probability {
		return nil // No burst this time
	}

	// We're going to burst! Calculate random parameters
	rb.lastBurstTime = time.Now()

	// Random duration between min and max
	durationRange := rb.maxDuration - rb.minDuration
	duration := rb.minDuration + time.Duration(rb.rng.Float64()*float64(durationRange))

	// Random multiplier between min and max
	multiplierRange := rb.maxMultiplier - rb.minMultiplier
	multiplier := rb.minMultiplier + (rb.rng.Float64() * multiplierRange)

	// Override with config values if set
	if rb.config.Duration > 0 {
		duration = rb.config.Duration
	}
	if rb.config.Multiplier > 0 {
		multiplier = rb.config.Multiplier
	}

	// Calculate extra sessions (randomized)
	baseSessions := 5 + rb.rng.IntN(20) // 5-25 base
	extraSessions := int(float64(baseSessions) * (multiplier - 1))

	now := time.Now()
	return &BurstEvent{
		Type:            BurstTypeRandom,
		StartTime:       now,
		EndTime:         now.Add(duration),
		Multiplier:      multiplier,
		Timezone:        timezone,
		SessionIncrease: extraSessions,
	}
}

// ForceTrigger bypasses probability and cooldown to trigger a random burst
// Useful for testing or manual stress injection
func (rb *RandomBurst) ForceTrigger() *BurstEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.config.Enabled {
		return nil
	}

	rb.lastBurstTime = time.Now()

	// Use mid-range values for forced triggers
	duration := rb.minDuration + (rb.maxDuration-rb.minDuration)/2
	multiplier := rb.minMultiplier + (rb.maxMultiplier-rb.minMultiplier)/2

	// Override with config values if set
	if rb.config.Duration > 0 {
		duration = rb.config.Duration
	}
	if rb.config.Multiplier > 0 {
		multiplier = rb.config.Multiplier
	}

	extraSessions := int(float64(15) * (multiplier - 1))

	now := time.Now()
	return &BurstEvent{
		Type:            BurstTypeRandom,
		StartTime:       now,
		EndTime:         now.Add(duration),
		Multiplier:      multiplier,
		SessionIncrease: extraSessions,
	}
}

// GetStats returns current state of the random burst generator
type RandomBurstStats struct {
	LastBurstTime   time.Time
	CheckCounter    int
	CooldownRemaining time.Duration
	IsOnCooldown    bool
}

// Stats returns current state information
func (rb *RandomBurst) Stats() RandomBurstStats {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	remaining := rb.minCooldown - time.Since(rb.lastBurstTime)
	if remaining < 0 {
		remaining = 0
	}

	return RandomBurstStats{
		LastBurstTime:     rb.lastBurstTime,
		CheckCounter:      rb.checkCounter,
		CooldownRemaining: remaining,
		IsOnCooldown:      remaining > 0,
	}
}
