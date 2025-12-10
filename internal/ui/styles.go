package ui

import "github.com/charmbracelet/lipgloss"

// Color palette - uses adaptive colors that work in both light and dark terminals.
var (
	// Primary blue for headers and highlights
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#58A6FF"}

	// Success green for completed operations
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#008000", Dark: "#3FB950"}

	// Error red for failures
	ColorError = lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#F85149"}

	// Warning yellow/orange
	ColorWarning = lipgloss.AdaptiveColor{Light: "#CC6600", Dark: "#D29922"}

	// Muted gray for secondary information
	ColorMuted = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#8B949E"}

	// Progress color for in-flight operations
	ColorProgress = lipgloss.AdaptiveColor{Light: "#6639A6", Dark: "#A371F7"}
)

// Unicode symbols for status indicators.
const (
	SymbolSuccess  = "✓"
	SymbolError    = "✗"
	SymbolWarning  = "!"
	SymbolProgress = "●"
	SymbolPending  = "○"
)

// Styles for common UI elements.
var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleProgress = lipgloss.NewStyle().Foreground(ColorProgress)
)
