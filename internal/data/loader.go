package data

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed names/*.json addresses/*.json
var dataFiles embed.FS

// ReferenceData holds all loaded reference data for the generator
type ReferenceData struct {
	FirstNames FirstNamesData
	LastNames  LastNamesData
	Countries  CountriesData
	Cities     CitiesData

	// Lookup maps for efficient access
	countryByCode    map[string]*Country
	citiesByCountry  map[string][]City
	regionByCountry  map[string]string
	countriesByWeight []weightedCountry
	totalWeight      int
}

// weightedCountry for weighted random selection
type weightedCountry struct {
	Country         *Country
	CumulativeWeight int
}

// FirstNamesData represents the structure of first_names.json
type FirstNamesData struct {
	Regions map[string]RegionNames `json:"regions"`
}

// RegionNames holds names for a specific region
type RegionNames struct {
	Countries []string `json:"countries"`
	Male      []string `json:"male"`
	Female    []string `json:"female"`
}

// LastNamesData represents the structure of last_names.json
type LastNamesData struct {
	Regions map[string]RegionLastNames `json:"regions"`
}

// RegionLastNames holds last names for a specific region
type RegionLastNames struct {
	Countries []string `json:"countries"`
	Names     []string `json:"names"`
}

// CountriesData represents the structure of countries.json
type CountriesData struct {
	Countries []Country `json:"countries"`
}

// Country represents a single country's data
type Country struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Currency  string `json:"currency"`
	Timezone  string `json:"timezone"`
	Region    string `json:"region"`
	PhoneCode string `json:"phone_code"`
	Weight    int    `json:"weight"`
}

// CitiesData represents the structure of cities.json
type CitiesData struct {
	Countries map[string]CountryCities `json:"countries"`
}

// CountryCities holds city data for a country
type CountryCities struct {
	PostalFormat string `json:"postal_format"`
	Cities       []City `json:"cities"`
}

// City represents a single city's data
type City struct {
	City         string `json:"city"`
	State        string `json:"state"`
	PostalPrefix string `json:"postal_prefix"`
}

var (
	instance *ReferenceData
	once     sync.Once
	loadErr  error
)

// Load loads all reference data from embedded files
// This is thread-safe and will only load data once
func Load() (*ReferenceData, error) {
	once.Do(func() {
		instance = &ReferenceData{}
		loadErr = instance.loadAll()
	})

	if loadErr != nil {
		return nil, loadErr
	}
	return instance, nil
}

// loadAll loads all data files
func (r *ReferenceData) loadAll() error {
	// Load first names
	data, err := dataFiles.ReadFile("names/first_names.json")
	if err != nil {
		return fmt.Errorf("failed to read first_names.json: %w", err)
	}
	if err := json.Unmarshal(data, &r.FirstNames); err != nil {
		return fmt.Errorf("failed to parse first_names.json: %w", err)
	}

	// Load last names
	data, err = dataFiles.ReadFile("names/last_names.json")
	if err != nil {
		return fmt.Errorf("failed to read last_names.json: %w", err)
	}
	if err := json.Unmarshal(data, &r.LastNames); err != nil {
		return fmt.Errorf("failed to parse last_names.json: %w", err)
	}

	// Load countries
	data, err = dataFiles.ReadFile("addresses/countries.json")
	if err != nil {
		return fmt.Errorf("failed to read countries.json: %w", err)
	}
	if err := json.Unmarshal(data, &r.Countries); err != nil {
		return fmt.Errorf("failed to parse countries.json: %w", err)
	}

	// Load cities
	data, err = dataFiles.ReadFile("addresses/cities.json")
	if err != nil {
		return fmt.Errorf("failed to read cities.json: %w", err)
	}
	if err := json.Unmarshal(data, &r.Cities); err != nil {
		return fmt.Errorf("failed to parse cities.json: %w", err)
	}

	// Build lookup maps
	r.buildLookups()

	return nil
}

// buildLookups creates efficient lookup structures
func (r *ReferenceData) buildLookups() {
	// Country by code lookup
	r.countryByCode = make(map[string]*Country)
	for i := range r.Countries.Countries {
		c := &r.Countries.Countries[i]
		r.countryByCode[c.Code] = c
	}

	// Region by country lookup
	r.regionByCountry = make(map[string]string)
	for i := range r.Countries.Countries {
		c := &r.Countries.Countries[i]
		r.regionByCountry[c.Code] = c.Region
	}

	// Cities by country lookup
	r.citiesByCountry = make(map[string][]City)
	for code, countryCities := range r.Cities.Countries {
		r.citiesByCountry[code] = countryCities.Cities
	}

	// Build weighted country list for random selection
	r.totalWeight = 0
	r.countriesByWeight = make([]weightedCountry, 0, len(r.Countries.Countries))
	for i := range r.Countries.Countries {
		c := &r.Countries.Countries[i]
		r.totalWeight += c.Weight
		r.countriesByWeight = append(r.countriesByWeight, weightedCountry{
			Country:          c,
			CumulativeWeight: r.totalWeight,
		})
	}
}

// GetCountry returns country data by ISO code
func (r *ReferenceData) GetCountry(code string) (*Country, bool) {
	c, ok := r.countryByCode[code]
	return c, ok
}

// GetRegion returns the region for a country code
func (r *ReferenceData) GetRegion(countryCode string) (string, bool) {
	region, ok := r.regionByCountry[countryCode]
	return region, ok
}

// GetCities returns cities for a country code
func (r *ReferenceData) GetCities(countryCode string) ([]City, bool) {
	cities, ok := r.citiesByCountry[countryCode]
	return cities, ok
}

// GetPostalFormat returns the postal code format for a country
func (r *ReferenceData) GetPostalFormat(countryCode string) string {
	if cc, ok := r.Cities.Countries[countryCode]; ok {
		return cc.PostalFormat
	}
	return ""
}

// GetFirstNames returns first names for a region and gender
func (r *ReferenceData) GetFirstNames(region string, isMale bool) []string {
	if rn, ok := r.FirstNames.Regions[region]; ok {
		if isMale {
			return rn.Male
		}
		return rn.Female
	}
	return nil
}

// GetLastNames returns last names for a region
func (r *ReferenceData) GetLastNames(region string) []string {
	if rn, ok := r.LastNames.Regions[region]; ok {
		return rn.Names
	}
	return nil
}

// AllCountries returns all country data
func (r *ReferenceData) AllCountries() []Country {
	return r.Countries.Countries
}

// TotalWeight returns the sum of all country weights for weighted selection
func (r *ReferenceData) TotalWeight() int {
	return r.totalWeight
}

// CountryByWeight returns the country for a given weight value (for weighted random selection)
// weightValue should be in range [1, TotalWeight()]
func (r *ReferenceData) CountryByWeight(weightValue int) *Country {
	for _, wc := range r.countriesByWeight {
		if weightValue <= wc.CumulativeWeight {
			return wc.Country
		}
	}
	// Fallback to last country
	if len(r.countriesByWeight) > 0 {
		return r.countriesByWeight[len(r.countriesByWeight)-1].Country
	}
	return nil
}

// AllRegions returns a list of all region names
func (r *ReferenceData) AllRegions() []string {
	regions := make(map[string]bool)
	for _, c := range r.Countries.Countries {
		regions[c.Region] = true
	}
	result := make([]string, 0, len(regions))
	for region := range regions {
		result = append(result, region)
	}
	return result
}
