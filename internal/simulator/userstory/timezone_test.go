// Package userstory provides user story validation tests for the simulator.
//
// FILE: timezone_test.go
// PURPOSE: Tests for timezone-aware customer activity scheduling and global
// activity patterns across different regions.
//
// KEY TESTS:
// - TestUS_Customer_TimezoneAwareness: Local business hours and activity weighting
//
// RELATED FILES:
// - helpers_test.go: Shared test utilities
// - retail_test.go: Retail customer tests
package userstory

import (
	"testing"

	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/simulator"
)

// TestUS_Customer_TimezoneAwareness validates:
// "As a customer in any country/time zone, I want activity to occur during
// my local daytime (8:00–16:00) so that usage patterns feel natural."
//
// Acceptance: session likelihood weighted to local hours; weekend/weekday
// differences applied; intraday peaks (morning check, lunch ATM, end-of-day rush) present.
func TestUS_Customer_TimezoneAwareness(t *testing.T) {
	ac := simulator.NewActivityCalculator(8, 16)

	t.Run("active_hours_8_to_16", func(t *testing.T) {
		// Verify activity calculator uses 8-16 active hours
		tm := ac.GetTimezoneManager()
		if tm.GetActiveStart() != 8 || tm.GetActiveEnd() != 16 {
			t.Errorf("expected active hours 8-16, got %d-%d", tm.GetActiveStart(), tm.GetActiveEnd())
		}
	})

	t.Run("segment_affects_activity", func(t *testing.T) {
		regularCustomer := &models.Customer{
			Timezone:      "America/New_York",
			Segment:       models.SegmentRegular,
			ActivityScore: 0.5,
		}
		corporateCustomer := &models.Customer{
			Timezone:      "America/New_York",
			Segment:       models.SegmentCorporate,
			ActivityScore: 0.5,
		}

		regularProb := ac.CalculateActivityProbability(regularCustomer)
		corpProb := ac.CalculateActivityProbability(corporateCustomer)

		// Corporate should have higher activity multiplier
		if corpProb <= regularProb {
			t.Errorf("corporate probability (%.4f) should exceed regular (%.4f)", corpProb, regularProb)
		}
	})

	t.Run("global_activity_has_regions", func(t *testing.T) {
		snapshot := ac.GetGlobalActivitySnapshot()

		requiredRegions := []string{"Americas", "Europe", "Asia", "Pacific"}
		for _, region := range requiredRegions {
			if _, ok := snapshot.RegionalActivity[region]; !ok {
				t.Errorf("expected activity for region %s", region)
			}
		}
	})

	t.Run("activity_score_affects_probability", func(t *testing.T) {
		lowActivity := &models.Customer{
			Timezone:      "UTC",
			Segment:       models.SegmentRegular,
			ActivityScore: 0.1,
		}
		highActivity := &models.Customer{
			Timezone:      "UTC",
			Segment:       models.SegmentRegular,
			ActivityScore: 0.9,
		}

		lowProb := ac.CalculateActivityProbability(lowActivity)
		highProb := ac.CalculateActivityProbability(highActivity)

		if highProb <= lowProb {
			t.Errorf("high activity (%.4f) should exceed low activity (%.4f)", highProb, lowProb)
		}
	})
}
