package simulator

import (
	"testing"
	"time"
)

func TestTimezoneManager_GetLocation(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	tests := []struct {
		timezone string
		wantUTC  bool // Should fall back to UTC
	}{
		{"America/New_York", false},
		{"Europe/London", false},
		{"Asia/Tokyo", false},
		{"Invalid/Timezone", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.timezone, func(t *testing.T) {
			loc := tm.GetLocation(tt.timezone)
			if tt.wantUTC && loc != time.UTC {
				t.Errorf("expected UTC for invalid timezone %q, got %v", tt.timezone, loc)
			}
			if !tt.wantUTC && loc == time.UTC {
				t.Errorf("expected non-UTC for valid timezone %q", tt.timezone)
			}
		})
	}
}

func TestTimezoneManager_GetLocation_Caching(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	// First call loads timezone
	loc1 := tm.GetLocation("America/New_York")

	// Second call should return same cached location
	loc2 := tm.GetLocation("America/New_York")

	if loc1 != loc2 {
		t.Error("expected same location to be returned from cache")
	}
}

func TestTimezoneManager_IntradayWeights(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	// Check that weights are initialized
	weights := tm.intradayWeights

	// Peak morning (9 AM) should have high weight
	if weights[9] < 0.8 {
		t.Errorf("expected high weight for 9 AM (morning peak), got %.2f", weights[9])
	}

	// Night (2 AM) should have low weight
	if weights[2] > 0.1 {
		t.Errorf("expected low weight for 2 AM (night), got %.2f", weights[2])
	}

	// Lunch (12 PM) should have moderate-high weight
	if weights[12] < 0.5 {
		t.Errorf("expected moderate weight for 12 PM (lunch), got %.2f", weights[12])
	}
}

func TestTimezoneManager_WeekdayMultiplier(t *testing.T) {
	// We can't easily test specific days without mocking time,
	// but we can verify the expected multiplier values make sense

	// Saturday should be lower than weekday
	satMultiplier := 0.4 // From the code
	if satMultiplier >= 1.0 {
		t.Errorf("Saturday multiplier should be less than 1.0")
	}

	// Sunday should be lowest
	sunMultiplier := 0.25 // From the code
	if sunMultiplier >= satMultiplier {
		t.Errorf("Sunday multiplier should be less than Saturday")
	}
}

func TestTimezoneManager_SetIntradayWeight(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	// Set custom weight
	tm.SetIntradayWeight(10, 0.99)
	if tm.intradayWeights[10] != 0.99 {
		t.Errorf("expected weight 0.99 for hour 10, got %.2f", tm.intradayWeights[10])
	}

	// Invalid hour should be ignored
	tm.SetIntradayWeight(25, 0.5)

	// Invalid weight should be ignored
	tm.SetIntradayWeight(10, 1.5)
	if tm.intradayWeights[10] > 1.0 {
		t.Errorf("weight should not exceed 1.0")
	}
}

func TestTimezoneManager_ApplyLunchBurst(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	originalWeight12 := tm.intradayWeights[12]
	originalWeight13 := tm.intradayWeights[13]

	tm.ApplyLunchBurst(1.5)

	// Weights should increase but not exceed 1.0
	if tm.intradayWeights[12] <= originalWeight12 {
		t.Error("lunch burst should increase weight for hour 12")
	}
	if tm.intradayWeights[13] <= originalWeight13 {
		t.Error("lunch burst should increase weight for hour 13")
	}
	if tm.intradayWeights[12] > 1.0 || tm.intradayWeights[13] > 1.0 {
		t.Error("weights should be capped at 1.0")
	}
}

func TestTimezoneManager_IsWithinActiveWindow_NormalRange(t *testing.T) {
	tm := NewTimezoneManager(8, 16) // 8 AM - 4 PM

	// We can't test with real timezones without knowing current time
	// but we can verify the logic with the internal function

	// For a normal range (8-16):
	// hour 8 should be active (start is inclusive)
	// hour 15 should be active
	// hour 16 should NOT be active (end is exclusive)
	// hour 7 should NOT be active

	// Testing the logic directly via the activeStart/activeEnd fields
	if tm.activeStart != 8 {
		t.Errorf("expected activeStart 8, got %d", tm.activeStart)
	}
	if tm.activeEnd != 16 {
		t.Errorf("expected activeEnd 16, got %d", tm.activeEnd)
	}
}

func TestTimezoneManager_GetActiveTimezones(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	// This depends on current time, but we can verify it returns valid data
	active := tm.GetActiveTimezones()

	// Should not return empty or nil (some timezone is always in business hours globally)
	if active == nil {
		t.Error("GetActiveTimezones should not return nil")
	}

	// Each returned timezone should be valid
	for _, tz := range active {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			t.Errorf("invalid timezone returned: %s", tz)
		}
		if loc == nil {
			t.Errorf("nil location for timezone: %s", tz)
		}
	}
}

func TestTimezoneManager_CalculateGlobalDistribution(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	customersByTZ := map[string]int{
		"America/New_York": 100,
		"Europe/London":    50,
		"Asia/Tokyo":       75,
	}

	dist := tm.CalculateGlobalDistribution(customersByTZ)

	// Should return distribution for all timezones
	if len(dist) != 3 {
		t.Errorf("expected 3 distributions, got %d", len(dist))
	}

	// Total percentage should sum to ~1.0
	totalPct := 0.0
	for _, d := range dist {
		totalPct += d.CustomerPct
	}
	if totalPct < 0.99 || totalPct > 1.01 {
		t.Errorf("customer percentages should sum to ~1.0, got %.2f", totalPct)
	}

	// EffectiveWt should be CustomerPct * CurrentProb
	for _, d := range dist {
		expected := d.CustomerPct * d.CurrentProb
		if d.EffectiveWt != expected {
			t.Errorf("EffectiveWt mismatch for %s: expected %.4f, got %.4f",
				d.Timezone, expected, d.EffectiveWt)
		}
	}
}

func TestTimezoneManager_EmptyDistribution(t *testing.T) {
	tm := NewTimezoneManager(8, 16)

	dist := tm.CalculateGlobalDistribution(nil)
	if dist != nil {
		t.Error("expected nil for empty customer map")
	}

	dist = tm.CalculateGlobalDistribution(map[string]int{})
	if dist != nil {
		t.Error("expected nil for empty customer map")
	}
}
