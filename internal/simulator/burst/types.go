package burst

import (
	"context"
	"sync"
	"time"
)

// BurstType identifies different types of burst scenarios
type BurstType string

const (
	BurstTypeLunch   BurstType = "lunch_atm"
	BurstTypePayroll BurstType = "payroll"
	BurstTypeRandom  BurstType = "random"
	BurstTypeManual  BurstType = "manual"
)

// BurstConfig holds configuration for a burst scenario
type BurstConfig struct {
	// Enabled controls whether this burst type is active
	Enabled bool

	// Multiplier is how much to increase load (e.g., 2.0 = double)
	Multiplier float64

	// Duration is how long the burst lasts (0 = use default based on type)
	Duration time.Duration

	// Probability is the chance of triggering random bursts (0.0-1.0)
	// Only applies to BurstTypeRandom
	Probability float64
}

// BurstEvent represents an active or upcoming burst
type BurstEvent struct {
	Type       BurstType
	StartTime  time.Time
	EndTime    time.Time
	Multiplier float64
	Timezone   string // Optional: for timezone-specific bursts
	SessionIncrease int // Extra sessions to spawn during burst
}

// IsActive returns true if the burst is currently active
func (e *BurstEvent) IsActive() bool {
	now := time.Now()
	return now.After(e.StartTime) && now.Before(e.EndTime)
}

// RemainingDuration returns how long until the burst ends
func (e *BurstEvent) RemainingDuration() time.Duration {
	return time.Until(e.EndTime)
}

// BurstProvider is the interface for burst generators
type BurstProvider interface {
	// Type returns the burst type this provider handles
	Type() BurstType

	// CheckBurst determines if a burst should be triggered
	// Returns a BurstEvent if a burst should start, nil otherwise
	CheckBurst(timezone string) *BurstEvent

	// Configure updates the provider's configuration
	Configure(cfg BurstConfig)
}

// Manager coordinates all burst scenarios and provides a unified
// interface for the scheduler to query burst state
type Manager struct {
	providers map[BurstType]BurstProvider
	active    []*BurstEvent
	mu        sync.RWMutex

	// Statistics
	stats ManagerStats
}

// ManagerStats tracks burst statistics
type ManagerStats struct {
	TotalBurstsTriggered int64
	BurstsByType         map[BurstType]int64
	TotalExtraSessions   int64
	LastBurstTime        time.Time
}

// NewManager creates a new burst manager
func NewManager() *Manager {
	return &Manager{
		providers: make(map[BurstType]BurstProvider),
		active:    make([]*BurstEvent, 0),
		stats: ManagerStats{
			BurstsByType: make(map[BurstType]int64),
		},
	}
}

// RegisterProvider adds a burst provider to the manager
func (m *Manager) RegisterProvider(provider BurstProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.Type()] = provider
}

// CheckBursts evaluates all providers and returns any new burst events
func (m *Manager) CheckBursts(timezone string) []*BurstEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up expired bursts
	m.cleanupExpired()

	// Check each provider for new bursts
	var newEvents []*BurstEvent
	for _, provider := range m.providers {
		if event := provider.CheckBurst(timezone); event != nil {
			// Avoid duplicate bursts of the same type
			if !m.hasActiveType(event.Type) {
				m.active = append(m.active, event)
				m.stats.TotalBurstsTriggered++
				m.stats.BurstsByType[event.Type]++
				m.stats.TotalExtraSessions += int64(event.SessionIncrease)
				m.stats.LastBurstTime = event.StartTime
				newEvents = append(newEvents, event)
			}
		}
	}

	return newEvents
}

// GetActiveMultiplier returns the combined multiplier from all active bursts
func (m *Manager) GetActiveMultiplier() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	multiplier := 1.0
	for _, event := range m.active {
		if event.IsActive() {
			multiplier *= event.Multiplier
		}
	}
	return multiplier
}

// GetActiveBursts returns all currently active burst events
func (m *Manager) GetActiveBursts() []*BurstEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*BurstEvent
	for _, event := range m.active {
		if event.IsActive() {
			active = append(active, event)
		}
	}
	return active
}

// GetExtraSessionCount returns how many additional sessions should run
func (m *Manager) GetExtraSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, event := range m.active {
		if event.IsActive() {
			count += event.SessionIncrease
		}
	}
	return count
}

// TriggerManualBurst creates an on-demand burst for testing
func (m *Manager) TriggerManualBurst(multiplier float64, duration time.Duration, extraSessions int) *BurstEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	event := &BurstEvent{
		Type:            BurstTypeManual,
		StartTime:       now,
		EndTime:         now.Add(duration),
		Multiplier:      multiplier,
		SessionIncrease: extraSessions,
	}

	m.active = append(m.active, event)
	m.stats.TotalBurstsTriggered++
	m.stats.BurstsByType[BurstTypeManual]++
	m.stats.TotalExtraSessions += int64(extraSessions)
	m.stats.LastBurstTime = now

	return event
}

// hasActiveType checks if a burst of the given type is already active
func (m *Manager) hasActiveType(t BurstType) bool {
	for _, event := range m.active {
		if event.Type == t && event.IsActive() {
			return true
		}
	}
	return false
}

// cleanupExpired removes expired burst events
func (m *Manager) cleanupExpired() {
	var stillActive []*BurstEvent
	for _, event := range m.active {
		if event.EndTime.After(time.Now()) {
			stillActive = append(stillActive, event)
		}
	}
	m.active = stillActive
}

// GetStats returns burst statistics
func (m *Manager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy stats
	statsCopy := ManagerStats{
		TotalBurstsTriggered: m.stats.TotalBurstsTriggered,
		BurstsByType:         make(map[BurstType]int64),
		TotalExtraSessions:   m.stats.TotalExtraSessions,
		LastBurstTime:        m.stats.LastBurstTime,
	}
	for k, v := range m.stats.BurstsByType {
		statsCopy.BurstsByType[k] = v
	}
	return statsCopy
}

// Run starts the burst manager's background monitoring
func (m *Manager) Run(ctx context.Context, checkInterval time.Duration, onBurst func(*BurstEvent)) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check all major timezones for bursts
			timezones := []string{
				"America/New_York",
				"America/Los_Angeles",
				"Europe/London",
				"Europe/Paris",
				"Asia/Tokyo",
				"Asia/Singapore",
				"Australia/Sydney",
			}

			for _, tz := range timezones {
				events := m.CheckBursts(tz)
				for _, event := range events {
					if onBurst != nil {
						onBurst(event)
					}
				}
			}

		case <-ctx.Done():
			return
		}
	}
}
