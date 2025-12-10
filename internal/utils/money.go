package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// Money represents a monetary value in the smallest currency unit (cents).
// Using int64 provides exact arithmetic up to ~92 quadrillion dollars,
// which is more than sufficient for any banking simulation.
type Money int64

// Currency represents a currency with its formatting rules
type Currency struct {
	Code         string // ISO 4217 code (e.g., "USD")
	Symbol       string // Display symbol (e.g., "$")
	SymbolFirst  bool   // True if symbol comes before amount
	DecimalPlaces int   // Usually 2, but 0 for JPY, KRW, etc.
	ThousandsSep string // Thousands separator
	DecimalSep   string // Decimal separator
}

// Common currencies with formatting rules
var Currencies = map[string]Currency{
	"USD": {Code: "USD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"EUR": {Code: "EUR", Symbol: "€", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ".", DecimalSep: ","},
	"GBP": {Code: "GBP", Symbol: "£", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"JPY": {Code: "JPY", Symbol: "¥", SymbolFirst: true, DecimalPlaces: 0, ThousandsSep: ",", DecimalSep: "."},
	"CNY": {Code: "CNY", Symbol: "¥", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"CAD": {Code: "CAD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"AUD": {Code: "AUD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"CHF": {Code: "CHF", Symbol: "CHF", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: "'", DecimalSep: "."},
	"HKD": {Code: "HKD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"SGD": {Code: "SGD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"SEK": {Code: "SEK", Symbol: "kr", SymbolFirst: false, DecimalPlaces: 2, ThousandsSep: " ", DecimalSep: ","},
	"NOK": {Code: "NOK", Symbol: "kr", SymbolFirst: false, DecimalPlaces: 2, ThousandsSep: " ", DecimalSep: ","},
	"DKK": {Code: "DKK", Symbol: "kr", SymbolFirst: false, DecimalPlaces: 2, ThousandsSep: ".", DecimalSep: ","},
	"INR": {Code: "INR", Symbol: "₹", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"KRW": {Code: "KRW", Symbol: "₩", SymbolFirst: true, DecimalPlaces: 0, ThousandsSep: ",", DecimalSep: "."},
	"BRL": {Code: "BRL", Symbol: "R$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ".", DecimalSep: ","},
	"MXN": {Code: "MXN", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"ZAR": {Code: "ZAR", Symbol: "R", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: " ", DecimalSep: ","},
	"AED": {Code: "AED", Symbol: "AED", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"SAR": {Code: "SAR", Symbol: "SAR", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"PLN": {Code: "PLN", Symbol: "zł", SymbolFirst: false, DecimalPlaces: 2, ThousandsSep: " ", DecimalSep: ","},
	"TRY": {Code: "TRY", Symbol: "₺", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ".", DecimalSep: ","},
	"THB": {Code: "THB", Symbol: "฿", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"MYR": {Code: "MYR", Symbol: "RM", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"IDR": {Code: "IDR", Symbol: "Rp", SymbolFirst: true, DecimalPlaces: 0, ThousandsSep: ".", DecimalSep: ","},
	"PHP": {Code: "PHP", Symbol: "₱", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"VND": {Code: "VND", Symbol: "₫", SymbolFirst: false, DecimalPlaces: 0, ThousandsSep: ".", DecimalSep: ","},
	"NGN": {Code: "NGN", Symbol: "₦", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"EGP": {Code: "EGP", Symbol: "E£", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"NZD": {Code: "NZD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"ILS": {Code: "ILS", Symbol: "₪", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"CZK": {Code: "CZK", Symbol: "Kč", SymbolFirst: false, DecimalPlaces: 2, ThousandsSep: " ", DecimalSep: ","},
	"HUF": {Code: "HUF", Symbol: "Ft", SymbolFirst: false, DecimalPlaces: 0, ThousandsSep: " ", DecimalSep: ","},
	"RON": {Code: "RON", Symbol: "lei", SymbolFirst: false, DecimalPlaces: 2, ThousandsSep: ".", DecimalSep: ","},
	"QAR": {Code: "QAR", Symbol: "QAR", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"KWD": {Code: "KWD", Symbol: "KD", SymbolFirst: true, DecimalPlaces: 3, ThousandsSep: ",", DecimalSep: "."},
	"BHD": {Code: "BHD", Symbol: "BD", SymbolFirst: true, DecimalPlaces: 3, ThousandsSep: ",", DecimalSep: "."},
	"PKR": {Code: "PKR", Symbol: "₨", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"BDT": {Code: "BDT", Symbol: "৳", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"LKR": {Code: "LKR", Symbol: "Rs", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"TWD": {Code: "TWD", Symbol: "NT$", SymbolFirst: true, DecimalPlaces: 0, ThousandsSep: ",", DecimalSep: "."},
	"COP": {Code: "COP", Symbol: "$", SymbolFirst: true, DecimalPlaces: 0, ThousandsSep: ".", DecimalSep: ","},
	"CLP": {Code: "CLP", Symbol: "$", SymbolFirst: true, DecimalPlaces: 0, ThousandsSep: ".", DecimalSep: ","},
	"ARS": {Code: "ARS", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ".", DecimalSep: ","},
	"KES": {Code: "KES", Symbol: "KSh", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"MAD": {Code: "MAD", Symbol: "MAD", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
	"GHS": {Code: "GHS", Symbol: "₵", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."},
}

// DefaultCurrency is used when a currency code is not found
var DefaultCurrency = Currency{Code: "USD", Symbol: "$", SymbolFirst: true, DecimalPlaces: 2, ThousandsSep: ",", DecimalSep: "."}

// NewMoney creates a Money value from dollars/major units and cents/minor units
func NewMoney(dollars int64, cents int) Money {
	return Money(dollars*100 + int64(cents))
}

// Cents creates a Money value from cents/minor units only
func Cents(cents int64) Money {
	return Money(cents)
}

// Dollars creates a Money value from whole dollars/major units
func Dollars(dollars int64) Money {
	return Money(dollars * 100)
}

// FromFloat creates a Money value from a float64 (use with caution)
// This rounds to the nearest cent
func FromFloat(amount float64) Money {
	if amount >= 0 {
		return Money(amount*100 + 0.5)
	}
	return Money(amount*100 - 0.5)
}

// ToCents returns the value in cents (the underlying representation)
func (m Money) ToCents() int64 {
	return int64(m)
}

// ToDollars returns the value as a float64 (for display purposes only)
func (m Money) ToDollars() float64 {
	return float64(m) / 100
}

// DollarsPart returns just the whole dollars portion
func (m Money) DollarsPart() int64 {
	return int64(m) / 100
}

// CentsPart returns just the cents portion (0-99)
func (m Money) CentsPart() int {
	cents := int(int64(m) % 100)
	if cents < 0 {
		cents = -cents
	}
	return cents
}

// Add returns the sum of two Money values
func (m Money) Add(other Money) Money {
	return m + other
}

// Sub returns the difference of two Money values
func (m Money) Sub(other Money) Money {
	return m - other
}

// Mul multiplies by an integer
func (m Money) Mul(n int64) Money {
	return Money(int64(m) * n)
}

// MulFloat multiplies by a float and rounds to nearest cent
func (m Money) MulFloat(f float64) Money {
	result := float64(m) * f
	if result >= 0 {
		return Money(result + 0.5)
	}
	return Money(result - 0.5)
}

// Div divides by an integer (integer division)
func (m Money) Div(n int64) Money {
	if n == 0 {
		return m
	}
	return Money(int64(m) / n)
}

// Abs returns the absolute value
func (m Money) Abs() Money {
	if m < 0 {
		return -m
	}
	return m
}

// Neg returns the negated value
func (m Money) Neg() Money {
	return -m
}

// IsZero returns true if the value is zero
func (m Money) IsZero() bool {
	return m == 0
}

// IsPositive returns true if the value is positive
func (m Money) IsPositive() bool {
	return m > 0
}

// IsNegative returns true if the value is negative
func (m Money) IsNegative() bool {
	return m < 0
}

// Cmp compares two Money values: returns -1 if m < other, 0 if equal, 1 if m > other
func (m Money) Cmp(other Money) int {
	if m < other {
		return -1
	}
	if m > other {
		return 1
	}
	return 0
}

// Min returns the smaller of two Money values
func (m Money) Min(other Money) Money {
	if m < other {
		return m
	}
	return other
}

// Max returns the larger of two Money values
func (m Money) Max(other Money) Money {
	if m > other {
		return m
	}
	return other
}

// String returns a simple string representation (e.g., "123.45")
func (m Money) String() string {
	negative := m < 0
	if negative {
		m = -m
	}
	dollars := int64(m) / 100
	cents := int64(m) % 100

	result := fmt.Sprintf("%d.%02d", dollars, cents)
	if negative {
		result = "-" + result
	}
	return result
}

// Format formats the money value with the given currency
func (m Money) Format(currencyCode string) string {
	currency, ok := Currencies[currencyCode]
	if !ok {
		currency = DefaultCurrency
	}

	negative := m < 0
	if negative {
		m = -m
	}

	// Calculate multiplier based on decimal places
	multiplier := int64(1)
	for i := 0; i < currency.DecimalPlaces; i++ {
		multiplier *= 10
	}

	// Get whole and fractional parts
	whole := int64(m) / multiplier
	frac := int64(m) % multiplier

	// Format whole part with thousands separator
	wholeStr := formatWithSeparator(whole, currency.ThousandsSep)

	// Build result
	var result string
	if currency.DecimalPlaces > 0 {
		fracStr := fmt.Sprintf("%0*d", currency.DecimalPlaces, frac)
		result = wholeStr + currency.DecimalSep + fracStr
	} else {
		result = wholeStr
	}

	// Add symbol
	if currency.SymbolFirst {
		result = currency.Symbol + result
	} else {
		result = result + " " + currency.Symbol
	}

	if negative {
		result = "-" + result
	}

	return result
}

// FormatSimple formats the money value with a simple format (e.g., "$123.45")
func (m Money) FormatSimple(symbol string) string {
	negative := m < 0
	if negative {
		m = -m
	}
	result := fmt.Sprintf("%s%d.%02d", symbol, int64(m)/100, int64(m)%100)
	if negative {
		result = "-" + result
	}
	return result
}

// formatWithSeparator adds thousands separators to a number
func formatWithSeparator(n int64, sep string) string {
	str := strconv.FormatInt(n, 10)
	if len(str) <= 3 || sep == "" {
		return str
	}

	var result strings.Builder
	startOffset := len(str) % 3
	if startOffset == 0 {
		startOffset = 3
	}

	result.WriteString(str[:startOffset])
	for i := startOffset; i < len(str); i += 3 {
		result.WriteString(sep)
		result.WriteString(str[i : i+3])
	}

	return result.String()
}

// GetCurrency returns the currency configuration for a code, or the default if not found
func GetCurrency(code string) Currency {
	if c, ok := Currencies[code]; ok {
		return c
	}
	return DefaultCurrency
}

// Percentage calculates a percentage of the money value
// e.g., m.Percentage(15) returns 15% of m
func (m Money) Percentage(percent float64) Money {
	return m.MulFloat(percent / 100)
}

// Split divides the money into n equal parts, handling remainder correctly
// Returns a slice where the first items get the extra cents from rounding
func (m Money) Split(n int) []Money {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []Money{m}
	}

	base := Money(int64(m) / int64(n))
	remainder := int(int64(m) % int64(n))

	result := make([]Money, n)
	for i := 0; i < n; i++ {
		if i < remainder {
			result[i] = base + 1
		} else {
			result[i] = base
		}
	}
	return result
}

// RandomAmount generates a random money amount in the given range using the provided RNG
func RandomAmount(rng *Random, min, max Money) Money {
	if min >= max {
		return min
	}
	return Money(rng.Int64Range(int64(min), int64(max)))
}

// RoundToNearest rounds the money to the nearest multiple of 'nearest'
// e.g., Money(123).RoundToNearest(Dollars(5)) returns $120 or $125
func (m Money) RoundToNearest(nearest Money) Money {
	if nearest <= 0 {
		return m
	}
	half := nearest / 2
	return ((m + half) / nearest) * nearest
}
