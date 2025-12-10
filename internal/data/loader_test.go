package data

import (
	"testing"
)

func TestLoadReferenceData(t *testing.T) {
	data, err := Load()
	if err != nil {
		t.Fatalf("Failed to load reference data: %v", err)
	}

	// Test country lookup
	t.Run("GetCountry", func(t *testing.T) {
		us, ok := data.GetCountry("US")
		if !ok {
			t.Fatal("Failed to find US country")
		}
		if us.Name != "United States" {
			t.Errorf("Expected 'United States', got '%s'", us.Name)
		}
		if us.Currency != "USD" {
			t.Errorf("Expected 'USD', got '%s'", us.Currency)
		}
		if us.Timezone != "America/New_York" {
			t.Errorf("Expected 'America/New_York', got '%s'", us.Timezone)
		}
	})

	// Test region lookup
	t.Run("GetRegion", func(t *testing.T) {
		region, ok := data.GetRegion("JP")
		if !ok {
			t.Fatal("Failed to find region for JP")
		}
		if region != "east_asia" {
			t.Errorf("Expected 'east_asia', got '%s'", region)
		}
	})

	// Test cities lookup
	t.Run("GetCities", func(t *testing.T) {
		cities, ok := data.GetCities("GB")
		if !ok {
			t.Fatal("Failed to find cities for GB")
		}
		if len(cities) == 0 {
			t.Error("Expected cities for GB, got none")
		}
		// Check for London
		foundLondon := false
		for _, city := range cities {
			if city.City == "London" {
				foundLondon = true
				break
			}
		}
		if !foundLondon {
			t.Error("Expected to find London in GB cities")
		}
	})

	// Test first names lookup
	t.Run("GetFirstNames", func(t *testing.T) {
		maleNames := data.GetFirstNames("north_america", true)
		if len(maleNames) == 0 {
			t.Error("Expected male names for north_america, got none")
		}
		femaleNames := data.GetFirstNames("north_america", false)
		if len(femaleNames) == 0 {
			t.Error("Expected female names for north_america, got none")
		}
	})

	// Test last names lookup
	t.Run("GetLastNames", func(t *testing.T) {
		names := data.GetLastNames("east_asia")
		if len(names) == 0 {
			t.Error("Expected last names for east_asia, got none")
		}
	})

	// Test weighted country selection
	t.Run("CountryByWeight", func(t *testing.T) {
		totalWeight := data.TotalWeight()
		if totalWeight == 0 {
			t.Error("Expected non-zero total weight")
		}

		// Test that we can get a country for any weight value
		country := data.CountryByWeight(1)
		if country == nil {
			t.Error("Expected country for weight 1")
		}
		country = data.CountryByWeight(totalWeight)
		if country == nil {
			t.Error("Expected country for max weight")
		}
	})

	// Test all regions
	t.Run("AllRegions", func(t *testing.T) {
		regions := data.AllRegions()
		if len(regions) == 0 {
			t.Error("Expected some regions")
		}
		// Check for expected regions
		expectedRegions := map[string]bool{
			"north_america":   false,
			"western_europe":  false,
			"east_asia":       false,
			"latin_america":   false,
		}
		for _, r := range regions {
			if _, ok := expectedRegions[r]; ok {
				expectedRegions[r] = true
			}
		}
		for region, found := range expectedRegions {
			if !found {
				t.Errorf("Expected region '%s' not found", region)
			}
		}
	})
}

func TestDataConsistency(t *testing.T) {
	data, err := Load()
	if err != nil {
		t.Fatalf("Failed to load reference data: %v", err)
	}

	// Verify all countries have matching cities
	for _, country := range data.AllCountries() {
		cities, ok := data.GetCities(country.Code)
		if !ok || len(cities) == 0 {
			t.Errorf("Country %s (%s) has no cities", country.Name, country.Code)
		}
	}

	// Verify all countries have a valid region with names
	for _, country := range data.AllCountries() {
		region, ok := data.GetRegion(country.Code)
		if !ok {
			t.Errorf("Country %s has no region", country.Code)
			continue
		}

		maleNames := data.GetFirstNames(region, true)
		if len(maleNames) == 0 {
			t.Errorf("Region %s (for country %s) has no male first names", region, country.Code)
		}

		femaleNames := data.GetFirstNames(region, false)
		if len(femaleNames) == 0 {
			t.Errorf("Region %s (for country %s) has no female first names", region, country.Code)
		}

		lastNames := data.GetLastNames(region)
		if len(lastNames) == 0 {
			t.Errorf("Region %s (for country %s) has no last names", region, country.Code)
		}
	}
}
