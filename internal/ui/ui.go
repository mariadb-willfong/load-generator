// Package ui provides styled terminal output for the loadgen CLI.
// It uses the Charm.sh ecosystem for modern TUI styling with
// automatic fallback to plain text for non-TTY environments.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// UI holds the terminal state and provides styled output methods.
type UI struct {
	IsTTY   bool
	Width   int
	NoColor bool
}

// KV represents a key-value pair for summary displays.
type KV struct {
	Key   string
	Value string
}

// noColorEnv is the standard environment variable to disable colors.
var noColorEnv = os.Getenv("NO_COLOR") != ""

// New creates a new UI instance with TTY detection.
func New() *UI {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	width := 80
	if isTTY {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			width = w
		}
	}

	return &UI{
		IsTTY:   isTTY,
		Width:   width,
		NoColor: noColorEnv,
	}
}

// SetNoColor disables colors and animations.
func (u *UI) SetNoColor(noColor bool) {
	u.NoColor = noColor
}

// shouldStyle returns true if we should use styled output.
func (u *UI) shouldStyle() bool {
	return u.IsTTY && !u.NoColor
}

// Header renders a bordered header box.
func (u *UI) Header(title string) string {
	if !u.shouldStyle() {
		return fmt.Sprintf("=== %s ===", title)
	}

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 2)

	return style.Render(title)
}

// KeyValue renders a styled key-value pair.
func (u *UI) KeyValue(key, value string) string {
	if !u.shouldStyle() {
		return fmt.Sprintf("%-10s %s", key+":", value)
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(12)
	valueStyle := lipgloss.NewStyle().
		Bold(true)

	return "  " + keyStyle.Render(key) + " " + valueStyle.Render(value)
}

// Success renders a success message with a green checkmark.
func (u *UI) Success(msg string) string {
	if !u.shouldStyle() {
		return "[OK] " + msg
	}

	return StyleSuccess.Render(SymbolSuccess+" ") + msg
}

// Error renders an error message with a red X.
func (u *UI) Error(msg string) string {
	if !u.shouldStyle() {
		return "[FAILED] " + msg
	}

	return StyleError.Render(SymbolError+" "+msg)
}

// Warning renders a warning message.
func (u *UI) Warning(msg string) string {
	if !u.shouldStyle() {
		return "[WARN] " + msg
	}

	return StyleWarning.Render(SymbolWarning+" "+msg)
}

// Muted renders muted/dim text.
func (u *UI) Muted(msg string) string {
	if !u.shouldStyle() {
		return msg
	}

	return StyleMuted.Render(msg)
}

// Bold renders bold text.
func (u *UI) Bold(msg string) string {
	if !u.shouldStyle() {
		return msg
	}

	return lipgloss.NewStyle().Bold(true).Render(msg)
}

// SummaryBox renders a bordered summary section.
func (u *UI) SummaryBox(title string, items []KV) string {
	if !u.shouldStyle() {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\n=== %s ===\n", title))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("%-14s %s\n", item.Key+":", item.Value))
		}
		return sb.String()
	}

	// Calculate max key width
	maxKeyWidth := 0
	for _, item := range items {
		if len(item.Key) > maxKeyWidth {
			maxKeyWidth = len(item.Key)
		}
	}

	// Build content
	var lines []string
	for _, item := range items {
		keyStyle := lipgloss.NewStyle().Foreground(ColorMuted).Width(maxKeyWidth + 2)
		valueStyle := lipgloss.NewStyle().Bold(true)

		// Special handling for Status field
		value := item.Value
		if item.Key == "Status" && strings.Contains(strings.ToLower(value), "success") {
			value = StyleSuccess.Render(SymbolSuccess + " " + value)
		} else if item.Key == "Status" && strings.Contains(strings.ToLower(value), "fail") {
			value = StyleError.Render(SymbolError + " " + value)
		} else {
			value = valueStyle.Render(value)
		}

		lines = append(lines, "  "+keyStyle.Render(item.Key)+" "+value)
	}
	content := strings.Join(lines, "\n")

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSuccess)

	// Box
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorSuccess).
		Padding(0, 1)

	return "\n" + titleStyle.Render("  "+title) + "\n" + boxStyle.Render(content)
}

// TableRow renders a table row with status.
func (u *UI) TableRow(name string, value string, status Status) string {
	if !u.shouldStyle() {
		prefix := ""
		switch status {
		case StatusSuccess:
			prefix = ""
		case StatusError:
			prefix = "FAILED: "
		case StatusPending:
			prefix = ""
		case StatusProgress:
			prefix = ""
		}
		return fmt.Sprintf("  %-15s %s%s", name+":", prefix, value)
	}

	nameStyle := lipgloss.NewStyle().Width(15)
	var symbol string
	var valueStyled string

	switch status {
	case StatusSuccess:
		symbol = StyleSuccess.Render(SymbolSuccess)
		valueStyled = value
	case StatusError:
		symbol = StyleError.Render(SymbolError)
		valueStyled = StyleError.Render(value)
	case StatusPending:
		symbol = StyleMuted.Render(SymbolPending)
		valueStyled = StyleMuted.Render(value)
	case StatusProgress:
		symbol = StyleProgress.Render(SymbolProgress)
		valueStyled = value
	default:
		symbol = " "
		valueStyled = value
	}

	return fmt.Sprintf("  %s %s %s", symbol, nameStyle.Render(name), valueStyled)
}

// Status represents the status of an operation.
type Status int

const (
	StatusNone Status = iota
	StatusPending
	StatusProgress
	StatusSuccess
	StatusError
)

// ClearLine clears the current line (for TTY only).
func (u *UI) ClearLine() string {
	if !u.IsTTY {
		return ""
	}
	return "\r\033[K"
}

// MoveCursorUp moves cursor up n lines (for TTY only).
func (u *UI) MoveCursorUp(n int) string {
	if !u.IsTTY || n <= 0 {
		return ""
	}
	return fmt.Sprintf("\033[%dA", n)
}
