package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// ProgressBar provides an animated progress bar for determinate operations.
type ProgressBar struct {
	ui       *UI
	bar      progress.Model
	label    string
	total    int64
	current  int64
	start    time.Time
	mu       sync.Mutex
	rendered bool
}

// NewProgressBar creates a new progress bar.
func (u *UI) NewProgressBar(label string, total int64) *ProgressBar {
	bar := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(30),
		progress.WithoutPercentage(),
	)

	return &ProgressBar{
		ui:    u,
		bar:   bar,
		label: label,
		total: total,
		start: time.Now(),
	}
}

// Update sets the current progress value.
func (p *ProgressBar) Update(current int64) {
	p.mu.Lock()
	p.current = current
	p.mu.Unlock()

	p.render()
}

// render draws the progress bar.
func (p *ProgressBar) render() {
	p.mu.Lock()
	current := p.current
	total := p.total
	p.mu.Unlock()

	if !p.ui.shouldStyle() {
		// Non-TTY: print progress updates periodically
		if !p.rendered {
			fmt.Printf("%s: ", p.label)
			p.rendered = true
		}
		return
	}

	pct := float64(current) / float64(total)
	if pct > 1 {
		pct = 1
	}

	labelStyle := lipgloss.NewStyle().Width(18)
	countStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	fmt.Fprintf(os.Stdout, "\r\033[K  %s %s %s",
		labelStyle.Render(p.label),
		p.bar.ViewAs(pct),
		countStyle.Render(fmt.Sprintf("%d/%d", current, total)),
	)
}

// Complete finishes the progress bar with a success indicator.
func (p *ProgressBar) Complete() {
	p.mu.Lock()
	current := p.current
	total := p.total
	p.mu.Unlock()

	if !p.ui.shouldStyle() {
		fmt.Printf("%d/%d done\n", current, total)
		return
	}

	labelStyle := lipgloss.NewStyle().Width(18)

	fmt.Fprintf(os.Stdout, "\r\033[K  %s %s %s\n",
		StyleSuccess.Render(SymbolSuccess),
		labelStyle.Render(p.label),
		StyleSuccess.Render(fmt.Sprintf("%d/%d complete", total, total)),
	)
}

// Fail finishes the progress bar with an error indicator.
func (p *ProgressBar) Fail(err error) {
	if !p.ui.shouldStyle() {
		fmt.Printf("FAILED: %v\n", err)
		return
	}

	labelStyle := lipgloss.NewStyle().Width(18)

	fmt.Fprintf(os.Stdout, "\r\033[K  %s %s %s\n",
		StyleError.Render(SymbolError),
		labelStyle.Render(p.label),
		StyleError.Render(err.Error()),
	)
}
