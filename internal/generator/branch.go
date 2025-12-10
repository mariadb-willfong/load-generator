package generator

import (
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// BranchGenerator creates branches and ATMs with global distribution.
type BranchGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  BranchGeneratorConfig
}

// BranchGeneratorConfig holds settings for branch/ATM generation
type BranchGeneratorConfig struct {
	NumBranches int
	NumATMs     int
	// BaseDate is used as reference for opened_at dates
	BaseDate time.Time
	// YearsBack is how many years of history (branches opened throughout this period)
	YearsBack int
}

// NewBranchGenerator creates a new branch generator
func NewBranchGenerator(rng *utils.Random, refData *data.ReferenceData, config BranchGeneratorConfig) *BranchGenerator {
	return &BranchGenerator{
		rng:     rng,
		refData: refData,
		config:  config,
	}
}

// GeneratedBranch holds a generated branch with its country info
type GeneratedBranch struct {
	Branch  models.Branch
	Country *data.Country
}

// GeneratedATM holds a generated ATM with its country info
type GeneratedATM struct {
	ATM     models.ATM
	Country *data.Country
}

// GenerateBranches creates all branches with global distribution
func (g *BranchGenerator) GenerateBranches() []GeneratedBranch {
	branches := make([]GeneratedBranch, 0, g.config.NumBranches)

	// Track branch counts per country for branch codes
	countryBranchCount := make(map[string]int)

	for i := 0; i < g.config.NumBranches; i++ {
		branch := g.generateBranch(int64(i+1), countryBranchCount)
		branches = append(branches, branch)
	}

	return branches
}

// GenerateATMs creates all ATMs, some attached to branches
func (g *BranchGenerator) GenerateATMs(branches []GeneratedBranch) []GeneratedATM {
	atms := make([]GeneratedATM, 0, g.config.NumATMs)

	// 60% of ATMs are at branches, 40% are standalone
	branchATMRatio := 0.6
	branchATMCount := int(float64(g.config.NumATMs) * branchATMRatio)

	// Track ATMs per branch for realistic distribution
	branchATMCounts := make(map[int64]int)

	for i := 0; i < g.config.NumATMs; i++ {
		var atm GeneratedATM

		if i < branchATMCount && len(branches) > 0 {
			// ATM at a branch - pick a branch with Pareto-ish distribution
			// (some branches have more ATMs)
			branchIdx := g.pickBranchForATM(branches, branchATMCounts)
			branch := branches[branchIdx]
			atm = g.generateBranchATM(int64(i+1), branch)
			branchATMCounts[branch.Branch.ID]++
		} else {
			// Standalone ATM
			atm = g.generateStandaloneATM(int64(i + 1))
		}

		atms = append(atms, atm)
	}

	return atms
}

// generateBranch creates a single branch
func (g *BranchGenerator) generateBranch(id int64, countryBranchCount map[string]int) GeneratedBranch {
	// Pick country weighted by population/banking activity
	country := g.pickCountry()

	// Get city for this country
	city := g.pickCity(country.Code)

	// Increment branch count for this country
	countryBranchCount[country.Code]++
	branchNum := countryBranchCount[country.Code]

	// Generate branch code: XX-NNNN (country code + sequential number)
	branchCode := fmt.Sprintf("%s-%04d", country.Code, branchNum)

	// Generate name based on city and type
	branchType := g.pickBranchType()
	name := g.generateBranchName(city, branchType, branchNum)

	// Pick opening date (spread across years of history)
	openedAt := g.generateOpeningDate()

	// Generate operating hours based on country/region
	hours := g.generateOperatingHours(country)

	branch := models.Branch{
		ID:               id,
		BranchCode:       branchCode,
		Name:             name,
		Type:             branchType,
		Status:           models.BranchStatusOpen,
		AddressLine1:     g.generateStreetAddress(),
		City:             city.City,
		State:            city.State,
		PostalCode:       g.generatePostalCode(country.Code, city.PostalPrefix),
		Country:          country.Code,
		Latitude:         g.generateLatitude(),
		Longitude:        g.generateLongitude(),
		Timezone:         country.Timezone,
		MondayHours:      hours.weekday,
		TuesdayHours:     hours.weekday,
		WednesdayHours:   hours.weekday,
		ThursdayHours:    hours.weekday,
		FridayHours:      hours.friday,
		SaturdayHours:    hours.saturday,
		SundayHours:      hours.sunday,
		Phone:            g.generatePhone(country.PhoneCode),
		Email:            fmt.Sprintf("branch%04d@globalbank.com", branchNum),
		CustomerCapacity: g.rng.IntRange(500, 5000),
		ATMCount:         0, // Will be updated when ATMs are assigned
		OpenedAt:         openedAt,
		UpdatedAt:        time.Now(),
	}

	return GeneratedBranch{Branch: branch, Country: country}
}

// generateBranchATM creates an ATM located at a branch
func (g *BranchGenerator) generateBranchATM(id int64, branch GeneratedBranch) GeneratedATM {
	atmID := fmt.Sprintf("ATM-%s-%05d", branch.Country.Code, id)

	branchID := branch.Branch.ID

	atm := models.ATM{
		ID:                   id,
		ATMID:                atmID,
		BranchID:             &branchID,
		Status:               models.ATMStatusOnline,
		LocationName:         branch.Branch.Name,
		AddressLine1:         branch.Branch.AddressLine1,
		City:                 branch.Branch.City,
		State:                branch.Branch.State,
		PostalCode:           branch.Branch.PostalCode,
		Country:              branch.Branch.Country,
		Latitude:             branch.Branch.Latitude,
		Longitude:            branch.Branch.Longitude,
		Timezone:             branch.Branch.Timezone,
		SupportsDeposit:      g.rng.Probability(0.7), // 70% support deposits
		SupportsTransfer:     g.rng.Probability(0.5), // 50% support transfers
		Is24Hours:            g.rng.Probability(0.3), // 30% are 24-hour
		AvgDailyTransactions: g.rng.IntRange(50, 300),
		InstalledAt:          branch.Branch.OpenedAt.Add(g.rng.Duration(0, 365*24*time.Hour)),
		UpdatedAt:            time.Now(),
	}

	return GeneratedATM{ATM: atm, Country: branch.Country}
}

// generateStandaloneATM creates a standalone ATM (mall, gas station, etc.)
func (g *BranchGenerator) generateStandaloneATM(id int64) GeneratedATM {
	country := g.pickCountry()
	city := g.pickCity(country.Code)

	atmID := fmt.Sprintf("ATM-%s-%05d", country.Code, id)

	// Standalone ATM location names
	locations := []string{
		"Shopping Mall",
		"Gas Station",
		"Convenience Store",
		"Airport Terminal",
		"Train Station",
		"University Campus",
		"Hospital",
		"Hotel Lobby",
		"Supermarket",
		"Office Building",
	}
	locationName := fmt.Sprintf("%s %s", city.City, g.rng.PickString(locations))

	installedDate := g.generateOpeningDate()

	atm := models.ATM{
		ID:                   id,
		ATMID:                atmID,
		BranchID:             nil, // Standalone
		Status:               models.ATMStatusOnline,
		LocationName:         locationName,
		AddressLine1:         g.generateStreetAddress(),
		City:                 city.City,
		State:                city.State,
		PostalCode:           g.generatePostalCode(country.Code, city.PostalPrefix),
		Country:              country.Code,
		Latitude:             g.generateLatitude(),
		Longitude:            g.generateLongitude(),
		Timezone:             country.Timezone,
		SupportsDeposit:      g.rng.Probability(0.3), // Less likely for standalone
		SupportsTransfer:     g.rng.Probability(0.2),
		Is24Hours:            g.rng.Probability(0.6), // More likely to be 24-hour
		AvgDailyTransactions: g.rng.IntRange(20, 150),
		InstalledAt:          installedDate,
		UpdatedAt:            time.Now(),
	}

	return GeneratedATM{ATM: atm, Country: country}
}

// pickCountry selects a country weighted by banking activity
func (g *BranchGenerator) pickCountry() *data.Country {
	totalWeight := g.refData.TotalWeight()
	pick := g.rng.IntRange(1, totalWeight)
	return g.refData.CountryByWeight(pick)
}

// pickCity selects a city for the given country
func (g *BranchGenerator) pickCity(countryCode string) data.City {
	cities, ok := g.refData.GetCities(countryCode)
	if !ok || len(cities) == 0 {
		return data.City{City: "Capital City", State: "", PostalPrefix: "10000"}
	}
	return cities[g.rng.IntN(len(cities))]
}

// pickBranchType determines the type of branch
func (g *BranchGenerator) pickBranchType() models.BranchType {
	types := []models.BranchType{
		models.BranchTypeFull,
		models.BranchTypeFull,
		models.BranchTypeFull,    // Weight toward full branches
		models.BranchTypeLimited,
		models.BranchTypeRegional,
	}
	return types[g.rng.IntN(len(types))]
}

// generateBranchName creates a realistic branch name
func (g *BranchGenerator) generateBranchName(city data.City, branchType models.BranchType, branchNum int) string {
	switch branchType {
	case models.BranchTypeRegional:
		return fmt.Sprintf("%s Regional Office", city.City)
	case models.BranchTypeHeadquarter:
		return "Global Headquarters"
	case models.BranchTypeLimited:
		return fmt.Sprintf("%s Express Branch", city.City)
	default:
		// Use variations for full branches
		variations := []string{
			"%s Main Branch",
			"%s Central",
			"%s Downtown",
			"%s Branch",
			"GlobalBank %s",
		}
		pattern := g.rng.PickString(variations)
		return fmt.Sprintf(pattern, city.City)
	}
}

// operatingHours holds different hours for different days
type operatingHours struct {
	weekday  string
	friday   string
	saturday string
	sunday   string
}

// generateOperatingHours creates realistic operating hours
func (g *BranchGenerator) generateOperatingHours(country *data.Country) operatingHours {
	// Standard hours with regional variations
	hours := operatingHours{
		weekday:  "09:00-17:00",
		friday:   "09:00-17:00",
		saturday: "09:00-13:00",
		sunday:   "",
	}

	// Some regions have Friday as weekend
	if country.Region == "middle_east" {
		hours.friday = ""
		hours.saturday = "09:00-17:00"
		hours.sunday = "09:00-17:00"
	}

	// Some branches open earlier/later
	if g.rng.Probability(0.2) {
		hours.weekday = "08:00-18:00"
		hours.friday = "08:00-18:00"
	}

	// Some are closed Saturday
	if g.rng.Probability(0.3) {
		hours.saturday = ""
	}

	return hours
}

// pickBranchForATM selects a branch for ATM placement with Pareto-like distribution
func (g *BranchGenerator) pickBranchForATM(branches []GeneratedBranch, currentCounts map[int64]int) int {
	// Larger branches (by capacity) get more ATMs
	weights := make([]int, len(branches))
	for i, b := range branches {
		// Base weight on capacity, reduce weight if already has ATMs
		existing := currentCounts[b.Branch.ID]
		weights[i] = b.Branch.CustomerCapacity / (existing + 1)
	}
	return g.rng.WeightedPick(weights)
}

// generateStreetAddress creates a realistic street address
func (g *BranchGenerator) generateStreetAddress() string {
	streetNum := g.rng.IntRange(1, 9999)
	streets := []string{
		"Main Street",
		"High Street",
		"Market Street",
		"Park Avenue",
		"King Street",
		"Queen Street",
		"First Avenue",
		"Central Boulevard",
		"Commerce Way",
		"Financial District Road",
	}
	return fmt.Sprintf("%d %s", streetNum, g.rng.PickString(streets))
}

// generatePostalCode creates a postal code based on country format
func (g *BranchGenerator) generatePostalCode(countryCode, prefix string) string {
	format := g.refData.GetPostalFormat(countryCode)
	if format == "" {
		format = "NNNNN" // Default to 5 digits
	}

	result := ""
	prefixIdx := 0

	for _, c := range format {
		switch c {
		case 'N':
			if prefixIdx < len(prefix) && prefix[prefixIdx] >= '0' && prefix[prefixIdx] <= '9' {
				result += string(prefix[prefixIdx])
				prefixIdx++
			} else {
				result += string(g.rng.Digit())
			}
		case 'L':
			if prefixIdx < len(prefix) && prefix[prefixIdx] >= 'A' && prefix[prefixIdx] <= 'Z' {
				result += string(prefix[prefixIdx])
				prefixIdx++
			} else {
				result += string(g.rng.Letter())
			}
		default:
			result += string(c)
		}
	}

	return result
}

// generatePhone creates a phone number with country code
func (g *BranchGenerator) generatePhone(phoneCode string) string {
	return fmt.Sprintf("+%s %s", phoneCode, g.rng.NumericString(10))
}

// generateLatitude generates a random latitude
func (g *BranchGenerator) generateLatitude() float64 {
	return g.rng.Float64Range(-60, 70) // Avoid extreme latitudes
}

// generateLongitude generates a random longitude
func (g *BranchGenerator) generateLongitude() float64 {
	return g.rng.Float64Range(-180, 180)
}

// generateOpeningDate creates an opening date within the history period
func (g *BranchGenerator) generateOpeningDate() time.Time {
	yearsBack := g.config.YearsBack
	if yearsBack <= 0 {
		yearsBack = 5
	}

	// Most branches opened throughout the history period
	daysBack := yearsBack * 365
	return g.rng.DateInPast(daysBack)
}

// WriteBranchesCSV writes branches to a CSV file (or .csv.xz if compress=true)
func WriteBranchesCSV(branches []GeneratedBranch, outputDir string, compress bool) error {
	return writeBranchesCSVInternal(branches, outputDir, compress, false)
}

// WriteBranchesCSVWithProgress writes branches with progress reporting
func WriteBranchesCSVWithProgress(branches []GeneratedBranch, outputDir string, compress bool) error {
	return writeBranchesCSVInternal(branches, outputDir, compress, true)
}

func writeBranchesCSVInternal(branches []GeneratedBranch, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "branch_code", "name", "type", "status",
		"address_line1", "address_line2", "city", "state", "postal_code", "country",
		"latitude", "longitude", "timezone",
		"monday_hours", "tuesday_hours", "wednesday_hours", "thursday_hours",
		"friday_hours", "saturday_hours", "sunday_hours",
		"phone", "email", "customer_capacity", "atm_count",
		"opened_at", "closed_at", "updated_at",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "branches",
		Headers:   headers,
		Compress:  compress,
	})
	if err != nil {
		return err
	}
	defer writer.Close()

	// Set up progress tracking if requested
	var progress *ProgressReporter
	if showProgress {
		progress = NewProgressReporter(ProgressConfig{
			Total: int64(len(branches)),
			Label: "  Branches",
		})
	}

	for i, gb := range branches {
		b := gb.Branch
		row := []string{
			FormatInt64(b.ID),
			b.BranchCode,
			b.Name,
			string(b.Type),
			string(b.Status),
			b.AddressLine1,
			b.AddressLine2,
			b.City,
			b.State,
			b.PostalCode,
			b.Country,
			FormatFloat64(b.Latitude),
			FormatFloat64(b.Longitude),
			b.Timezone,
			b.MondayHours,
			b.TuesdayHours,
			b.WednesdayHours,
			b.ThursdayHours,
			b.FridayHours,
			b.SaturdayHours,
			b.SundayHours,
			b.Phone,
			b.Email,
			FormatInt(b.CustomerCapacity),
			FormatInt(b.ATMCount),
			FormatTime(b.OpenedAt),
			FormatTimePtr(b.ClosedAt),
			FormatTime(b.UpdatedAt),
		}
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		// Update progress every 100 rows to balance responsiveness vs overhead
		if progress != nil && (i+1)%100 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	// Finalize progress display
	if progress != nil {
		progress.Set(int64(len(branches)))
		progress.Finish()
	}

	return writer.Close()
}

// WriteATMsCSV writes ATMs to a CSV file (or .csv.xz if compress=true)
func WriteATMsCSV(atms []GeneratedATM, outputDir string, compress bool) error {
	return writeATMsCSVInternal(atms, outputDir, compress, false)
}

// WriteATMsCSVWithProgress writes ATMs with progress reporting
func WriteATMsCSVWithProgress(atms []GeneratedATM, outputDir string, compress bool) error {
	return writeATMsCSVInternal(atms, outputDir, compress, true)
}

func writeATMsCSVInternal(atms []GeneratedATM, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "atm_id", "branch_id", "status",
		"location_name", "address_line1", "city", "state", "postal_code", "country",
		"latitude", "longitude", "timezone",
		"supports_deposit", "supports_transfer", "is_24_hours",
		"avg_daily_transactions", "installed_at", "updated_at",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "atms",
		Headers:   headers,
		Compress:  compress,
	})
	if err != nil {
		return err
	}
	defer writer.Close()

	var progress *ProgressReporter
	if showProgress {
		progress = NewProgressReporter(ProgressConfig{
			Total: int64(len(atms)),
			Label: "  ATMs",
		})
	}

	for i, ga := range atms {
		a := ga.ATM
		row := []string{
			FormatInt64(a.ID),
			a.ATMID,
			FormatInt64Ptr(a.BranchID),
			string(a.Status),
			a.LocationName,
			a.AddressLine1,
			a.City,
			a.State,
			a.PostalCode,
			a.Country,
			FormatFloat64(a.Latitude),
			FormatFloat64(a.Longitude),
			a.Timezone,
			FormatBool(a.SupportsDeposit),
			FormatBool(a.SupportsTransfer),
			FormatBool(a.Is24Hours),
			FormatInt(a.AvgDailyTransactions),
			FormatTime(a.InstalledAt),
			FormatTime(a.UpdatedAt),
		}
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		if progress != nil && (i+1)%100 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	if progress != nil {
		progress.Set(int64(len(atms)))
		progress.Finish()
	}

	return writer.Close()
}
