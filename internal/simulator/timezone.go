package simulator

import (
	"sync"
	"time"
)

// TimezoneManager handles timezone-aware scheduling for global customers.
// It caches timezone locations and provides activity probability calculations.
type TimezoneManager struct {
	// Cached timezone locations (IANA name -> *time.Location)
	cache sync.Map

	// Active window configuration
	activeStart int // Hour of day (0-23) when activity begins
	activeEnd   int // Hour of day (0-23) when activity ends

	// Intraday pattern weights (24 hours)
	// Higher weight = more likely to be active during that hour
	intradayWeights [24]float64
}

// NewTimezoneManager creates a new timezone manager with the given active window
func NewTimezoneManager(activeStart, activeEnd int) *TimezoneManager {
	tm := &TimezoneManager{
		activeStart: activeStart,
		activeEnd:   activeEnd,
	}
	tm.initIntradayWeights()
	return tm
}

// initIntradayWeights sets up realistic intraday activity patterns.
// This models the typical banking activity curve throughout a business day:
// - Morning rush (8-10 AM): People check accounts before/during commute
// - Midday dip (11-12 PM): Slightly lower activity
// - Lunch peak (12-1 PM): ATM withdrawals, quick checks
// - Afternoon steady (1-4 PM): Consistent business activity
// - Evening tail (4-6 PM): Last transactions of the day
// - Night (6 PM - 8 AM): Minimal activity
func (tm *TimezoneManager) initIntradayWeights() {
	// Default weights: 0.0 = no activity, 1.0 = peak activity
	weights := [24]float64{
		0.02, // 00:00 - Minimal overnight activity
		0.01, // 01:00
		0.01, // 02:00
		0.01, // 03:00
		0.02, // 04:00 - Very early risers
		0.05, // 05:00 - Early commuters
		0.10, // 06:00 - Morning starts
		0.20, // 07:00 - Pre-work checks
		0.70, // 08:00 - Morning rush begins
		0.90, // 09:00 - Peak morning
		0.85, // 10:00 - Still busy
		0.75, // 11:00 - Slight dip
		0.80, // 12:00 - Lunch ATM peak
		0.70, // 13:00 - After lunch
		0.65, // 14:00 - Afternoon steady
		0.60, // 15:00 - Afternoon continues
		0.55, // 16:00 - End of business day starts
		0.40, // 17:00 - After work checks
		0.25, // 18:00 - Evening activity
		0.15, // 19:00 - Dinner time, lower activity
		0.10, // 20:00 - Evening low
		0.08, // 21:00 - Late evening
		0.05, // 22:00 - Night
		0.03, // 23:00 - Late night
	}
	tm.intradayWeights = weights
}

// GetActiveStart returns the hour when activity begins (0-23)
func (tm *TimezoneManager) GetActiveStart() int {
	return tm.activeStart
}

// GetActiveEnd returns the hour when activity ends (0-23)
func (tm *TimezoneManager) GetActiveEnd() int {
	return tm.activeEnd
}

// GetLocation returns a cached timezone location for the given IANA name.
// Returns UTC if the timezone is invalid.
func (tm *TimezoneManager) GetLocation(timezone string) *time.Location {
	// Check cache first
	if loc, ok := tm.cache.Load(timezone); ok {
		return loc.(*time.Location)
	}

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Invalid timezone - use UTC as fallback
		loc = time.UTC
	}

	// Cache and return
	tm.cache.Store(timezone, loc)
	return loc
}

// GetLocalTime returns the current time in the customer's timezone
func (tm *TimezoneManager) GetLocalTime(timezone string) time.Time {
	loc := tm.GetLocation(timezone)
	return time.Now().In(loc)
}

// GetLocalHour returns the current hour (0-23) in the customer's timezone
func (tm *TimezoneManager) GetLocalHour(timezone string) int {
	return tm.GetLocalTime(timezone).Hour()
}

// IsWithinActiveWindow checks if the current local hour falls within
// the configured active window (e.g., 8 AM - 4 PM)
func (tm *TimezoneManager) IsWithinActiveWindow(timezone string) bool {
	hour := tm.GetLocalHour(timezone)

	if tm.activeStart <= tm.activeEnd {
		// Normal range (e.g., 8-16)
		return hour >= tm.activeStart && hour < tm.activeEnd
	}
	// Wrapping range (e.g., 22-6 for night workers)
	return hour >= tm.activeStart || hour < tm.activeEnd
}

// GetActivityProbability returns the probability (0.0-1.0) that a customer
// in the given timezone would be active right now. This combines:
// 1. Whether they're within the active window (hard boundary)
// 2. Intraday patterns (soft weighting within active hours)
//
// This creates realistic traffic patterns where activity peaks in mid-morning
// and has a secondary peak at lunch, rather than uniform distribution.
func (tm *TimezoneManager) GetActivityProbability(timezone string) float64 {
	hour := tm.GetLocalHour(timezone)

	// Get base weight for this hour
	weight := tm.intradayWeights[hour]

	// If outside active window, apply severe penalty but don't zero out
	// (some customers do check accounts at odd hours)
	if !tm.IsWithinActiveWindow(timezone) {
		weight *= 0.1 // 90% reduction outside business hours
	}

	return weight
}

// GetWeekdayMultiplier returns a multiplier for activity based on day of week.
// Weekdays have full activity, weekends are reduced.
func (tm *TimezoneManager) GetWeekdayMultiplier(timezone string) float64 {
	weekday := tm.GetLocalTime(timezone).Weekday()

	switch weekday {
	case time.Saturday:
		return 0.4 // 40% of weekday activity
	case time.Sunday:
		return 0.25 // 25% of weekday activity
	default:
		return 1.0 // Full weekday activity
	}
}

// GetCombinedActivityProbability returns the final activity probability
// combining intraday patterns and weekday adjustments
func (tm *TimezoneManager) GetCombinedActivityProbability(timezone string) float64 {
	hourlyProb := tm.GetActivityProbability(timezone)
	weekdayMult := tm.GetWeekdayMultiplier(timezone)
	return hourlyProb * weekdayMult
}

// SetIntradayWeight allows customizing the weight for a specific hour.
// This can be used to implement special patterns like lunch bursts.
func (tm *TimezoneManager) SetIntradayWeight(hour int, weight float64) {
	if hour >= 0 && hour < 24 && weight >= 0 && weight <= 1.0 {
		tm.intradayWeights[hour] = weight
	}
}

// ApplyLunchBurst temporarily increases activity weight during lunch hours
// (typically 12 PM - 1 PM) by the given multiplier
func (tm *TimezoneManager) ApplyLunchBurst(multiplier float64) {
	// Boost lunch hours (12-13)
	tm.intradayWeights[12] = min(1.0, tm.intradayWeights[12]*multiplier)
	tm.intradayWeights[13] = min(1.0, tm.intradayWeights[13]*multiplier)
}

// GetActiveTimezones returns a list of IANA timezone names that are currently
// within business hours. Useful for "follow the sun" analytics.
func (tm *TimezoneManager) GetActiveTimezones() []string {
	// Common global banking timezones to check (using valid IANA names)
	allTimezones := []string{
		"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
		"America/Sao_Paulo", "America/Mexico_City",
		"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Moscow",
		"Asia/Dubai", "Asia/Kolkata", "Asia/Singapore", "Asia/Hong_Kong",
		"Asia/Tokyo", "Asia/Seoul", "Asia/Shanghai",
		"Australia/Sydney", "Pacific/Auckland",
	}

	active := make([]string, 0, len(allTimezones))
	for _, tz := range allTimezones {
		if tm.IsWithinActiveWindow(tz) {
			active = append(active, tz)
		}
	}
	return active
}

// TimezoneDistribution represents the proportion of customers in each timezone
// Used for scheduling decisions
type TimezoneDistribution struct {
	Timezone     string
	CustomerPct  float64 // Percentage of customers in this timezone
	CurrentProb  float64 // Current activity probability
	EffectiveWt  float64 // CustomerPct * CurrentProb = effective weight
}

// CalculateGlobalDistribution calculates how load should be distributed
// across timezones based on customer distribution and current activity levels
func (tm *TimezoneManager) CalculateGlobalDistribution(customersByTZ map[string]int) []TimezoneDistribution {
	total := 0
	for _, count := range customersByTZ {
		total += count
	}

	if total == 0 {
		return nil
	}

	dist := make([]TimezoneDistribution, 0, len(customersByTZ))
	for tz, count := range customersByTZ {
		pct := float64(count) / float64(total)
		prob := tm.GetCombinedActivityProbability(tz)
		dist = append(dist, TimezoneDistribution{
			Timezone:    tz,
			CustomerPct: pct,
			CurrentProb: prob,
			EffectiveWt: pct * prob,
		})
	}

	return dist
}
