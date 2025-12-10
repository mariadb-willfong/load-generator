package patterns

import (
	"math"
	"time"
)

// DailyPattern provides activity multipliers based on hour of day.
// Models typical banking behavior: morning peak, lunch ATM rush, afternoon activity, evening slowdown.
type DailyPattern struct {
	// Hourly multipliers (0-23 hours), values typically 0.0-2.0
	// 1.0 = average activity, >1.0 = above average, <1.0 = below average
	hourlyMultipliers [24]float64
}

// NewDailyPattern creates a pattern with default banking behavior curves.
// The curve reflects typical retail banking patterns:
// - 6-8 AM: Ramping up as people wake and check accounts
// - 8-9 AM: Morning peak (salary checks, pre-work banking)
// - 10-11 AM: Moderate activity
// - 12-1 PM: Lunch peak (ATM withdrawals, quick transactions)
// - 2-4 PM: Steady afternoon activity
// - 4-5 PM: Evening peak (end-of-day transfers before cutoffs)
// - 6-9 PM: Declining activity (post-work)
// - 10 PM-5 AM: Very low activity (night owls and insomniacs)
func NewDailyPattern() *DailyPattern {
	dp := &DailyPattern{}

	// Define the standard hourly multipliers
	// Index 0 = midnight, 23 = 11 PM
	dp.hourlyMultipliers = [24]float64{
		0.05, // 00:00 - midnight
		0.03, // 01:00
		0.02, // 02:00
		0.02, // 03:00
		0.03, // 04:00
		0.08, // 05:00 - early risers
		0.20, // 06:00 - morning starts
		0.50, // 07:00 - commute time
		1.40, // 08:00 - morning peak
		1.60, // 09:00 - highest morning
		1.20, // 10:00 - mid-morning
		1.00, // 11:00 - approaching lunch
		1.50, // 12:00 - lunch peak (ATM rush)
		1.30, // 13:00 - post-lunch
		1.10, // 14:00 - afternoon
		1.00, // 15:00 - mid-afternoon
		1.30, // 16:00 - pre-cutoff rush
		1.20, // 17:00 - evening peak
		0.80, // 18:00 - after work
		0.50, // 19:00 - evening decline
		0.30, // 20:00 - evening low
		0.20, // 21:00 - night
		0.10, // 22:00 - late night
		0.05, // 23:00 - approaching midnight
	}

	return dp
}

// NewATMDailyPattern creates a pattern optimized for ATM transaction behavior.
// ATM usage has a stronger lunch peak and evening usage after work.
func NewATMDailyPattern() *DailyPattern {
	dp := &DailyPattern{}

	dp.hourlyMultipliers = [24]float64{
		0.08, // 00:00 - some late-night ATM usage
		0.05, // 01:00
		0.03, // 02:00
		0.02, // 03:00
		0.03, // 04:00
		0.05, // 05:00
		0.15, // 06:00
		0.40, // 07:00 - morning commute
		0.80, // 08:00
		1.00, // 09:00
		0.90, // 10:00
		1.10, // 11:00 - pre-lunch
		1.80, // 12:00 - LUNCH PEAK (ATMs are busiest)
		1.50, // 13:00 - post-lunch
		1.00, // 14:00
		0.90, // 15:00
		1.00, // 16:00
		1.40, // 17:00 - after work peak
		1.30, // 18:00 - evening ATM run
		1.00, // 19:00
		0.60, // 20:00
		0.40, // 21:00
		0.25, // 22:00
		0.15, // 23:00
	}

	return dp
}

// NewOnlineBankingPattern creates a pattern for online/mobile banking.
// Online banking has a bimodal distribution: morning check and evening management.
func NewOnlineBankingPattern() *DailyPattern {
	dp := &DailyPattern{}

	dp.hourlyMultipliers = [24]float64{
		0.10, // 00:00 - night owls online
		0.05, // 01:00
		0.03, // 02:00
		0.02, // 03:00
		0.03, // 04:00
		0.10, // 05:00
		0.30, // 06:00 - early morning checks
		0.80, // 07:00 - breakfast banking
		1.20, // 08:00 - morning peak
		1.40, // 09:00 - work starts, check accounts
		1.00, // 10:00
		0.80, // 11:00
		0.90, // 12:00 - lunch browsing
		0.80, // 13:00
		0.90, // 14:00
		1.00, // 15:00
		1.20, // 16:00 - end of day transfers
		1.00, // 17:00
		1.10, // 18:00 - evening banking
		1.50, // 19:00 - EVENING PEAK (bill pay, transfers)
		1.60, // 20:00 - highest evening
		1.30, // 21:00
		0.80, // 22:00
		0.40, // 23:00
	}

	return dp
}

// NewBusinessBankingPattern creates a pattern for business account activity.
// Business activity is concentrated during business hours with cutoff awareness.
func NewBusinessBankingPattern() *DailyPattern {
	dp := &DailyPattern{}

	dp.hourlyMultipliers = [24]float64{
		0.02, // 00:00
		0.01, // 01:00
		0.01, // 02:00
		0.01, // 03:00
		0.02, // 04:00
		0.05, // 05:00
		0.10, // 06:00
		0.30, // 07:00
		0.80, // 08:00 - business day starts
		1.40, // 09:00 - morning business peak
		1.60, // 10:00 - highest activity
		1.40, // 11:00
		0.80, // 12:00 - lunch lull
		1.20, // 13:00 - post-lunch pickup
		1.40, // 14:00 - afternoon peak
		1.50, // 15:00 - pre-cutoff activity
		1.80, // 16:00 - CUTOFF RUSH (same-day transfers)
		1.00, // 17:00 - after cutoff decline
		0.30, // 18:00
		0.10, // 19:00
		0.05, // 20:00
		0.03, // 21:00
		0.02, // 22:00
		0.02, // 23:00
	}

	return dp
}

// GetMultiplier returns the activity multiplier for a given hour (0-23).
func (dp *DailyPattern) GetMultiplier(hour int) float64 {
	if hour < 0 || hour > 23 {
		return 0.0
	}
	return dp.hourlyMultipliers[hour]
}

// GetMultiplierForTime returns the activity multiplier for a specific time.
// Interpolates between hours for smoother transitions.
func (dp *DailyPattern) GetMultiplierForTime(t time.Time) float64 {
	hour := t.Hour()
	minute := t.Minute()

	// Get current and next hour multipliers
	currentMultiplier := dp.hourlyMultipliers[hour]
	nextHour := (hour + 1) % 24
	nextMultiplier := dp.hourlyMultipliers[nextHour]

	// Linear interpolation based on minutes
	fraction := float64(minute) / 60.0
	return currentMultiplier + (nextMultiplier-currentMultiplier)*fraction
}

// IsActiveHour returns true if the hour is within typical banking hours (6 AM - 10 PM).
func (dp *DailyPattern) IsActiveHour(hour int) bool {
	return hour >= 6 && hour <= 22
}

// IsPeakHour returns true if the hour is a peak activity period.
func (dp *DailyPattern) IsPeakHour(hour int) bool {
	multiplier := dp.GetMultiplier(hour)
	return multiplier >= 1.3
}

// GetPeakHours returns all hours that are considered peak periods.
func (dp *DailyPattern) GetPeakHours() []int {
	peaks := make([]int, 0, 6)
	for hour := 0; hour < 24; hour++ {
		if dp.IsPeakHour(hour) {
			peaks = append(peaks, hour)
		}
	}
	return peaks
}

// ShouldGenerateTransaction uses the pattern to probabilistically determine
// if a transaction should occur at a given time, based on activity multiplier.
// The probability is scaled against a baseline rate.
func (dp *DailyPattern) ShouldGenerateTransaction(t time.Time, baseRate float64, rngValue float64) bool {
	multiplier := dp.GetMultiplierForTime(t)
	probability := baseRate * multiplier
	// Cap at 1.0 to prevent over-generation
	if probability > 1.0 {
		probability = 1.0
	}
	return rngValue < probability
}

// AdjustedRate returns the base rate adjusted by the time-of-day multiplier.
// Use this for calculating expected transaction counts for an hour.
func (dp *DailyPattern) AdjustedRate(hour int, baseRate float64) float64 {
	return baseRate * dp.GetMultiplier(hour)
}

// ExpectedTransactionsPerHour returns the expected number of transactions
// for each hour given a daily target and this pattern.
func (dp *DailyPattern) ExpectedTransactionsPerHour(dailyTarget int) [24]float64 {
	var result [24]float64

	// Calculate total multiplier sum for normalization
	var totalMultiplier float64
	for _, m := range dp.hourlyMultipliers {
		totalMultiplier += m
	}

	// Distribute transactions according to pattern
	for hour := 0; hour < 24; hour++ {
		result[hour] = float64(dailyTarget) * (dp.hourlyMultipliers[hour] / totalMultiplier)
	}

	return result
}

// TimeInActiveWindow returns a time within the active banking window (6 AM - 10 PM)
// weighted by the pattern's hourly multipliers.
// The returned hour is 0-23, and minute is 0-59.
func (dp *DailyPattern) TimeInActiveWindow(rngValue float64) (hour int, minute int) {
	// Calculate cumulative weights for active hours only (6-22)
	var weights []float64
	var hours []int
	var totalWeight float64

	for h := 6; h <= 22; h++ {
		w := dp.hourlyMultipliers[h]
		totalWeight += w
		weights = append(weights, totalWeight)
		hours = append(hours, h)
	}

	// Pick hour based on weighted distribution
	target := rngValue * totalWeight
	selectedHour := 6 // default

	for i, cumulativeWeight := range weights {
		if target < cumulativeWeight {
			selectedHour = hours[i]
			break
		}
	}

	// Random minute within the hour (use remaining rng fraction)
	minuteFraction := math.Mod(rngValue*1000, 1.0)
	selectedMinute := int(minuteFraction * 60)

	return selectedHour, selectedMinute
}
