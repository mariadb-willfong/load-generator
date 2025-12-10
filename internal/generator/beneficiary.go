package generator

import (
	"fmt"
	"strings"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// BeneficiaryGenerator creates beneficiaries (external payees) for customers.
type BeneficiaryGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  BeneficiaryGeneratorConfig
}

// BeneficiaryGeneratorConfig holds settings for beneficiary generation
type BeneficiaryGeneratorConfig struct {
	// Average beneficiaries per customer
	AvgBeneficiariesPerCustomer int
	// Businesses to use as internal beneficiaries
	Businesses []GeneratedBusiness
}

// NewBeneficiaryGenerator creates a new beneficiary generator
func NewBeneficiaryGenerator(rng *utils.Random, refData *data.ReferenceData, config BeneficiaryGeneratorConfig) *BeneficiaryGenerator {
	if config.AvgBeneficiariesPerCustomer <= 0 {
		config.AvgBeneficiariesPerCustomer = 5
	}
	return &BeneficiaryGenerator{
		rng:     rng,
		refData: refData,
		config:  config,
	}
}

// GeneratedBeneficiary holds a generated beneficiary with metadata
type GeneratedBeneficiary struct {
	Beneficiary models.Beneficiary
}

// GenerateBeneficiariesForCustomers creates beneficiaries for all customers
func (g *BeneficiaryGenerator) GenerateBeneficiariesForCustomers(customers []GeneratedCustomer, startID int64) ([]GeneratedBeneficiary, int64) {
	beneficiaries := make([]GeneratedBeneficiary, 0, len(customers)*g.config.AvgBeneficiariesPerCustomer)
	currentID := startID

	for _, customer := range customers {
		customerBeneficiaries := g.generateBeneficiariesForCustomer(customer, &currentID)
		beneficiaries = append(beneficiaries, customerBeneficiaries...)
	}

	return beneficiaries, currentID
}

// generateBeneficiariesForCustomer creates 0-10 beneficiaries for a customer
func (g *BeneficiaryGenerator) generateBeneficiariesForCustomer(customer GeneratedCustomer, currentID *int64) []GeneratedBeneficiary {
	// Vary count based on activity score
	// Active customers have more beneficiaries
	baseCount := g.config.AvgBeneficiariesPerCustomer
	countVariation := int(float64(baseCount) * customer.Customer.ActivityScore)
	numBeneficiaries := g.rng.IntRange(1, baseCount+countVariation)

	beneficiaries := make([]GeneratedBeneficiary, 0, numBeneficiaries)

	// Distribution: 40% utilities, 30% individuals, 20% merchants, 10% government
	for i := 0; i < numBeneficiaries; i++ {
		beneficiary := g.generateBeneficiary(*currentID, customer)
		beneficiaries = append(beneficiaries, beneficiary)
		*currentID++
	}

	return beneficiaries
}

// generateBeneficiary creates a single beneficiary
func (g *BeneficiaryGenerator) generateBeneficiary(id int64, customer GeneratedCustomer) GeneratedBeneficiary {
	// Pick beneficiary type with distribution
	beneficiaryType := g.pickBeneficiaryType()

	// Determine if internal (links to a business) or external
	isInternal := g.rng.Probability(0.6) && len(g.config.Businesses) > 0

	var beneficiary models.Beneficiary

	if isInternal {
		beneficiary = g.generateInternalBeneficiary(id, customer, beneficiaryType)
	} else {
		beneficiary = g.generateExternalBeneficiary(id, customer, beneficiaryType)
	}

	return GeneratedBeneficiary{Beneficiary: beneficiary}
}

// pickBeneficiaryType determines beneficiary type
func (g *BeneficiaryGenerator) pickBeneficiaryType() models.BeneficiaryType {
	p := g.rng.Float64()
	switch {
	case p < 0.40:
		return models.BeneficiaryTypeUtility
	case p < 0.70:
		return models.BeneficiaryTypeIndividual
	case p < 0.90:
		return models.BeneficiaryTypeBusiness
	default:
		return models.BeneficiaryTypeGovernment
	}
}

// generateInternalBeneficiary creates a beneficiary linked to an internal business
func (g *BeneficiaryGenerator) generateInternalBeneficiary(id int64, customer GeneratedCustomer, beneficiaryType models.BeneficiaryType) models.Beneficiary {
	// Find a matching business
	var matchingBiz *GeneratedBusiness
	for _, biz := range g.config.Businesses {
		switch beneficiaryType {
		case models.BeneficiaryTypeUtility:
			if biz.BusinessType == BusinessTypeUtility {
				matchingBiz = &biz
			}
		case models.BeneficiaryTypeBusiness:
			if biz.BusinessType == BusinessTypeMerchant {
				matchingBiz = &biz
			}
		case models.BeneficiaryTypeGovernment:
			if biz.BusinessType == BusinessTypeGovernment {
				matchingBiz = &biz
			}
		}
		if matchingBiz != nil {
			break
		}
	}

	// If no matching business found, pick any
	if matchingBiz == nil {
		idx := g.rng.IntN(len(g.config.Businesses))
		matchingBiz = &g.config.Businesses[idx]
	}

	// Create beneficiary based on the business
	nickname := g.generateNickname(beneficiaryType, matchingBiz.BusinessName)
	createdAt := g.generateCreatedAt(customer.Customer.CreatedAt)

	return models.Beneficiary{
		ID:               id,
		CustomerID:       customer.Customer.ID,
		Nickname:         nickname,
		Name:             matchingBiz.BusinessName,
		Type:             beneficiaryType,
		Status:           models.BeneficiaryStatusVerified,
		BankName:         "GlobalBank", // Internal
		BankCode:         "GBNK",
		AccountNumber:    fmt.Sprintf("INT-%010d", matchingBiz.Customer.ID),
		Country:          matchingBiz.Country.Code,
		Currency:         models.Currency(matchingBiz.Country.Currency),
		PaymentMethod:    "internal",
		AccountReference: g.rng.NumericString(10),
		TransferCount:    g.rng.IntRange(0, 50),
		CreatedAt:        createdAt,
		UpdatedAt:        time.Now(),
	}
}

// generateExternalBeneficiary creates a beneficiary at an external bank
func (g *BeneficiaryGenerator) generateExternalBeneficiary(id int64, customer GeneratedCustomer, beneficiaryType models.BeneficiaryType) models.Beneficiary {
	// Generate beneficiary details based on type
	var name, nickname string
	switch beneficiaryType {
	case models.BeneficiaryTypeIndividual:
		name, nickname = g.generateIndividualName(customer.Country.Region)
	case models.BeneficiaryTypeUtility:
		name, nickname = g.generateUtilityName(customer.Country)
	case models.BeneficiaryTypeBusiness:
		name, nickname = g.generateMerchantName()
	case models.BeneficiaryTypeGovernment:
		name, nickname = g.generateGovernmentName(customer.Country)
	default:
		name, nickname = g.generateIndividualName(customer.Country.Region)
	}

	// External bank details
	bankName, bankCode := g.generateExternalBankDetails(customer.Country)
	accountNumber := g.generateExternalAccountNumber()

	// Address (same country as customer 70% of the time)
	var country *data.Country
	if g.rng.Probability(0.7) {
		country = customer.Country
	} else {
		country = g.pickCountry()
	}
	city := g.pickCity(country.Code)

	// Payment method
	paymentMethod := g.pickPaymentMethod(customer.Country.Code, country.Code)

	// IBAN for European countries
	iban := ""
	if g.isEuropean(country.Code) {
		iban = g.generateIBAN(country.Code)
	}

	createdAt := g.generateCreatedAt(customer.Customer.CreatedAt)

	return models.Beneficiary{
		ID:               id,
		CustomerID:       customer.Customer.ID,
		Nickname:         nickname,
		Name:             name,
		Type:             beneficiaryType,
		Status:           models.BeneficiaryStatusVerified,
		BankName:         bankName,
		BankCode:         bankCode,
		RoutingNumber:    g.generateRoutingNumber(country.Code),
		AccountNumber:    accountNumber,
		IBAN:             iban,
		AddressLine1:     g.generateStreetAddress(),
		City:             city.City,
		State:            city.State,
		PostalCode:       g.generatePostalCode(country.Code, city.PostalPrefix),
		Country:          country.Code,
		Currency:         models.Currency(country.Currency),
		PaymentMethod:    paymentMethod,
		AccountReference: g.rng.NumericString(10),
		TransferCount:    g.rng.IntRange(0, 30),
		CreatedAt:        createdAt,
		UpdatedAt:        time.Now(),
	}
}

// generateIndividualName creates a person's name for beneficiary
func (g *BeneficiaryGenerator) generateIndividualName(region string) (name, nickname string) {
	// Use western names for simplicity
	firstNames := g.refData.GetFirstNames("western", g.rng.Bool())
	lastNames := g.refData.GetLastNames("western")

	firstName := "John"
	lastName := "Doe"
	if len(firstNames) > 0 {
		firstName = g.rng.PickString(firstNames)
	}
	if len(lastNames) > 0 {
		lastName = g.rng.PickString(lastNames)
	}

	name = fmt.Sprintf("%s %s", firstName, lastName)
	nickname = firstName
	return
}

// generateUtilityName creates a utility company name
func (g *BeneficiaryGenerator) generateUtilityName(country *data.Country) (name, nickname string) {
	utilities := []string{
		"Electric Company",
		"Gas & Electric",
		"Water Services",
		"Telecom",
		"Internet Services",
		"Cable TV",
		"Mobile Phone",
		"Heating Services",
	}
	utilityType := g.rng.PickString(utilities)
	name = fmt.Sprintf("%s %s", country.Name, utilityType)
	nickname = utilityType
	return
}

// generateMerchantName creates a merchant business name
func (g *BeneficiaryGenerator) generateMerchantName() (name, nickname string) {
	names := []string{
		"Amazon",
		"Netflix",
		"Spotify",
		"Apple Store",
		"Google Play",
		"Best Buy",
		"Target",
		"Walmart",
		"Home Depot",
		"Costco",
	}
	name = g.rng.PickString(names)
	nickname = name
	return
}

// generateGovernmentName creates a government agency name
func (g *BeneficiaryGenerator) generateGovernmentName(country *data.Country) (name, nickname string) {
	agencies := []string{
		"Tax Authority",
		"Revenue Service",
		"Motor Vehicles",
		"Property Tax",
		"Customs",
		"Immigration",
	}
	agencyType := g.rng.PickString(agencies)
	name = fmt.Sprintf("%s %s", country.Name, agencyType)
	nickname = agencyType
	return
}

// generateExternalBankDetails creates external bank info
func (g *BeneficiaryGenerator) generateExternalBankDetails(country *data.Country) (bankName, bankCode string) {
	banks := []struct {
		name string
		code string
	}{
		{"Chase Bank", "CHASUS33"},
		{"Bank of America", "BOFAUS3N"},
		{"Wells Fargo", "WFBIUS6S"},
		{"Citibank", "CITIUS33"},
		{"HSBC", "HSBCGB2L"},
		{"Barclays", "BARCGB22"},
		{"Deutsche Bank", "DEUTDEFF"},
		{"BNP Paribas", "BNPAFRPP"},
		{"Santander", "BSCHESMM"},
		{"ING", "INGBNL2A"},
	}
	bank := banks[g.rng.IntN(len(banks))]
	return bank.name, bank.code
}

// generateExternalAccountNumber creates an external account number
func (g *BeneficiaryGenerator) generateExternalAccountNumber() string {
	return g.rng.NumericString(12)
}

// generateRoutingNumber creates a routing number for US banks
func (g *BeneficiaryGenerator) generateRoutingNumber(countryCode string) string {
	if countryCode == "US" {
		return g.rng.NumericString(9)
	}
	return ""
}

// generateIBAN creates an IBAN for European countries
func (g *BeneficiaryGenerator) generateIBAN(countryCode string) string {
	checkDigits := g.rng.NumericString(2)
	bankCode := g.rng.NumericString(4)
	accountNum := g.rng.NumericString(14)
	return fmt.Sprintf("%s%s%s%s", countryCode, checkDigits, bankCode, accountNum)
}

// isEuropean checks if country uses IBAN
func (g *BeneficiaryGenerator) isEuropean(countryCode string) bool {
	european := map[string]bool{
		"GB": true, "DE": true, "FR": true, "ES": true, "IT": true,
		"NL": true, "BE": true, "CH": true, "AT": true, "SE": true,
		"NO": true, "DK": true, "FI": true, "IE": true, "PT": true,
		"PL": true, "CZ": true, "GR": true,
	}
	return european[countryCode]
}

// pickPaymentMethod determines payment method based on countries
func (g *BeneficiaryGenerator) pickPaymentMethod(srcCountry, dstCountry string) string {
	if srcCountry == dstCountry {
		if srcCountry == "US" {
			return "ach"
		}
		return "domestic"
	}
	return "wire"
}

// pickCountry selects a country weighted by economic activity
func (g *BeneficiaryGenerator) pickCountry() *data.Country {
	totalWeight := g.refData.TotalWeight()
	pick := g.rng.IntRange(1, totalWeight)
	return g.refData.CountryByWeight(pick)
}

// pickCity selects a city for the given country
func (g *BeneficiaryGenerator) pickCity(countryCode string) data.City {
	cities, ok := g.refData.GetCities(countryCode)
	if !ok || len(cities) == 0 {
		return data.City{City: "Capital City", State: "", PostalPrefix: "10000"}
	}
	return cities[g.rng.IntN(len(cities))]
}

// generateNickname creates a friendly nickname for a beneficiary
func (g *BeneficiaryGenerator) generateNickname(beneficiaryType models.BeneficiaryType, name string) string {
	// Shorten to first meaningful word
	parts := strings.Fields(name)
	if len(parts) > 0 {
		short := parts[0]
		if len(short) > 15 {
			short = short[:15]
		}
		return short
	}
	return name
}

// generateStreetAddress creates a street address
func (g *BeneficiaryGenerator) generateStreetAddress() string {
	streetNum := g.rng.IntRange(1, 9999)
	streets := []string{
		"Main Street", "Oak Avenue", "Park Lane", "High Street",
		"Market Street", "Church Road", "Mill Lane", "Station Road",
	}
	return fmt.Sprintf("%d %s", streetNum, g.rng.PickString(streets))
}

// generatePostalCode creates a postal code
func (g *BeneficiaryGenerator) generatePostalCode(countryCode, prefix string) string {
	format := g.refData.GetPostalFormat(countryCode)
	if format == "" {
		format = "NNNNN"
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

// generateCreatedAt creates a beneficiary creation date
func (g *BeneficiaryGenerator) generateCreatedAt(customerCreatedAt time.Time) time.Time {
	// Beneficiary added sometime after customer joined
	daysAfter := g.rng.IntRange(1, 365)
	return customerCreatedAt.Add(time.Duration(daysAfter) * 24 * time.Hour)
}

// WriteBeneficiariesCSV writes beneficiaries to a CSV file (or .csv.xz if compress=true)
func WriteBeneficiariesCSV(beneficiaries []GeneratedBeneficiary, outputDir string, compress bool) error {
	return writeBeneficiariesCSVInternal(beneficiaries, outputDir, compress, false)
}

// WriteBeneficiariesCSVWithProgress writes beneficiaries with progress reporting
func WriteBeneficiariesCSVWithProgress(beneficiaries []GeneratedBeneficiary, outputDir string, compress bool) error {
	return writeBeneficiariesCSVInternal(beneficiaries, outputDir, compress, true)
}

func writeBeneficiariesCSVInternal(beneficiaries []GeneratedBeneficiary, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "customer_id", "nickname", "name", "type", "status",
		"bank_name", "bank_code", "routing_number", "account_number", "iban",
		"address_line1", "address_line2", "city", "state", "postal_code", "country",
		"currency", "payment_method", "account_reference",
		"last_used_at", "transfer_count",
		"created_at", "updated_at",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "beneficiaries",
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
			Total: int64(len(beneficiaries)),
			Label: "  Beneficiaries",
		})
	}

	for i, gb := range beneficiaries {
		b := gb.Beneficiary
		row := []string{
			FormatInt64(b.ID),
			FormatInt64(b.CustomerID),
			b.Nickname,
			b.Name,
			string(b.Type),
			string(b.Status),
			b.BankName,
			b.BankCode,
			b.RoutingNumber,
			b.AccountNumber,
			b.IBAN,
			b.AddressLine1,
			b.AddressLine2,
			b.City,
			b.State,
			b.PostalCode,
			b.Country,
			string(b.Currency),
			b.PaymentMethod,
			b.AccountReference,
			FormatTimePtr(b.LastUsedAt),
			FormatInt(b.TransferCount),
			FormatTime(b.CreatedAt),
			FormatTime(b.UpdatedAt),
		}
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		if progress != nil && (i+1)%100 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	if progress != nil {
		progress.Set(int64(len(beneficiaries)))
		progress.Finish()
	}

	return writer.Close()
}
