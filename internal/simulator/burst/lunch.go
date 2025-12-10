package burst

import (
	"sync"
	"time"
)

// LunchBurst implements lunch-time ATM burst injection.
// During typical lunch hours (12:00-14:00 local time), there's a spike
// in ATM usage as people withdraw cash for lunch expenses.
type LunchBurst struct {
	config BurstConfig
	mu     sync.RWMutex

	// Track which timezones have already triggered today
	triggeredToday map[string]time.Time

	// Time parsing cache for efficiency
	locationCache map[string]*time.Location
	cacheMu       sync.RWMutex
}

// NewLunchBurst creates a new lunch-time ATM burst provider
func NewLunchBurst(cfg BurstConfig) *LunchBurst {
	return &LunchBurst{
		config:         cfg,
		triggeredToday: make(map[string]time.Time),
		locationCache:  make(map[string]*time.Location),
	}
}

// Type implements BurstProvider
func (lb *LunchBurst) Type() BurstType {
	return BurstTypeLunch
}

// Configure implements BurstProvider
func (lb *LunchBurst) Configure(cfg BurstConfig) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.config = cfg
}

// CheckBurst implements BurstProvider
// Returns a burst event if it's lunch time in the given timezone and
// we haven't already triggered a burst today for this timezone
func (lb *LunchBurst) CheckBurst(timezone string) *BurstEvent {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if !lb.config.Enabled {
		return nil
	}

	// Get local time in the timezone
	localTime := lb.getLocalTime(timezone)
	if localTime.IsZero() {
		return nil
	}

	hour := localTime.Hour()
	minute := localTime.Minute()

	// Lunch burst window: 12:00-12:15 (trigger at start of lunch)
	// We only trigger at the start to avoid repeated triggers
	if hour != 12 || minute > 15 {
		return nil
	}

	// Check if we already triggered today for this timezone
	today := localTime.Truncate(24 * time.Hour)
	if lastTriggered, ok := lb.triggeredToday[timezone]; ok {
		if lastTriggered.Truncate(24 * time.Hour).Equal(today) {
			return nil // Already triggered today
		}
	}

	// Mark as triggered for today
	lb.triggeredToday[timezone] = localTime

	// Clean up old entries (keep last 7 days)
	lb.cleanupOldTriggers()

	// Calculate burst duration (default: 2 hours for full lunch period)
	duration := lb.config.Duration
	if duration == 0 {
		duration = 2 * time.Hour
	}

	// Calculate extra sessions based on multiplier
	// At lunch, we expect ~50% more ATM activity
	extraSessions := int(float64(10) * (lb.config.Multiplier - 1))
	if extraSessions < 0 {
		extraSessions = 0
	}

	now := time.Now()
	return &BurstEvent{
		Type:            BurstTypeLunch,
		StartTime:       now,
		EndTime:         now.Add(duration),
		Multiplier:      lb.config.Multiplier,
		Timezone:        timezone,
		SessionIncrease: extraSessions,
	}
}

// getLocalTime returns the current time in the specified timezone
func (lb *LunchBurst) getLocalTime(timezone string) time.Time {
	lb.cacheMu.RLock()
	loc, ok := lb.locationCache[timezone]
	lb.cacheMu.RUnlock()

	if !ok {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}
		}

		lb.cacheMu.Lock()
		lb.locationCache[timezone] = loc
		lb.cacheMu.Unlock()
	}

	return time.Now().In(loc)
}

// cleanupOldTriggers removes trigger records older than 7 days
func (lb *LunchBurst) cleanupOldTriggers() {
	cutoff := time.Now().AddDate(0, 0, -7)
	for tz, triggered := range lb.triggeredToday {
		if triggered.Before(cutoff) {
			delete(lb.triggeredToday, tz)
		}
	}
}

// IsLunchHour returns true if the current time in the timezone is during lunch
func (lb *LunchBurst) IsLunchHour(timezone string) bool {
	localTime := lb.getLocalTime(timezone)
	if localTime.IsZero() {
		return false
	}

	hour := localTime.Hour()
	return hour >= 12 && hour < 14
}
