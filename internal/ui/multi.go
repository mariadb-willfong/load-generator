package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// MultiProgress tracks multiple concurrent operations with live updates.
type MultiProgress struct {
	ui          *UI
	items       map[string]*ProgressItem
	order       []string // Maintains insertion order
	mu          sync.Mutex
	rendered    bool
	lineCount   int
	completions []string // Completed items to print at the end
}

// ProgressItem represents a single item being tracked.
type ProgressItem struct {
	Name      string
	Total     int64
	Current   int64
	StartTime time.Time
	Status    Status
	Message   string // Final message when complete
	Error     error
	bar       progress.Model
}

// NewMultiProgress creates a new multi-line progress tracker.
func (u *UI) NewMultiProgress() *MultiProgress {
	return &MultiProgress{
		ui:    u,
		items: make(map[string]*ProgressItem),
	}
}

// AddItem adds a new item to track.
func (m *MultiProgress) AddItem(name string, total int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bar := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(25),
		progress.WithoutPercentage(),
	)

	item := &ProgressItem{
		Name:      name,
		Total:     total,
		StartTime: time.Now(),
		Status:    StatusPending,
		bar:       bar,
	}

	m.items[name] = item
	m.order = append(m.order, name)
}

// Start marks an item as in progress.
func (m *MultiProgress) Start(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item, ok := m.items[name]; ok {
		item.Status = StatusProgress
		item.StartTime = time.Now()
	}
}

// Update sets the current progress for an item.
func (m *MultiProgress) Update(name string, current int64) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Current = current
		if item.Status == StatusPending {
			item.Status = StatusProgress
		}
	}
	m.mu.Unlock()

	m.Render()
}

// Complete marks an item as successfully completed.
func (m *MultiProgress) Complete(name string, message string) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Status = StatusSuccess
		item.Message = message
		item.Current = item.Total
	}
	m.mu.Unlock()

	m.Render()
}

// Fail marks an item as failed.
func (m *MultiProgress) Fail(name string, err error) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Status = StatusError
		item.Error = err
	}
	m.mu.Unlock()

	m.Render()
}

// Render redraws all progress lines.
func (m *MultiProgress) Render() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.ui.shouldStyle() {
		// Non-TTY: handled elsewhere
		return
	}

	// Move cursor up to overwrite previous output
	if m.rendered && m.lineCount > 0 {
		fmt.Fprintf(os.Stdout, "\033[%dA", m.lineCount)
	}

	// Render each item
	var lines []string
	for _, name := range m.order {
		item := m.items[name]
		line := m.renderItem(item)
		lines = append(lines, line)
	}

	// Print all lines
	for _, line := range lines {
		fmt.Fprintf(os.Stdout, "\033[K%s\n", line)
	}

	m.rendered = true
	m.lineCount = len(lines)
}

// renderItem renders a single progress item.
func (m *MultiProgress) renderItem(item *ProgressItem) string {
	nameStyle := lipgloss.NewStyle().Width(15)
	var symbol, detail string

	switch item.Status {
	case StatusPending:
		symbol = StyleMuted.Render(SymbolPending)
		detail = StyleMuted.Render("waiting...")

	case StatusProgress:
		symbol = StyleProgress.Render(SymbolProgress)
		if item.Total > 0 {
			pct := float64(item.Current) / float64(item.Total)
			if pct > 1 {
				pct = 1
			}
			elapsed := time.Since(item.StartTime)
			rate := float64(item.Current) / elapsed.Seconds()

			detail = fmt.Sprintf("%s %s %.1f/s",
				item.bar.ViewAs(pct),
				StyleMuted.Render(fmt.Sprintf("%d/%d", item.Current, item.Total)),
				rate,
			)
		} else {
			detail = StyleMuted.Render("loading...")
		}

	case StatusSuccess:
		symbol = StyleSuccess.Render(SymbolSuccess)
		if item.Message != "" {
			detail = item.Message
		} else {
			detail = StyleSuccess.Render("complete")
		}

	case StatusError:
		symbol = StyleError.Render(SymbolError)
		if item.Error != nil {
			detail = StyleError.Render(item.Error.Error())
		} else {
			detail = StyleError.Render("failed")
		}
	}

	return fmt.Sprintf("  %s %s %s", symbol, nameStyle.Render(item.Name), detail)
}

// Finish clears the multi-progress display and prints final results.
func (m *MultiProgress) Finish() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.ui.shouldStyle() {
		return
	}

	// Move cursor up and clear all lines
	if m.rendered && m.lineCount > 0 {
		fmt.Fprintf(os.Stdout, "\033[%dA", m.lineCount)
		for i := 0; i < m.lineCount; i++ {
			fmt.Fprint(os.Stdout, "\033[K\n")
		}
		fmt.Fprintf(os.Stdout, "\033[%dA", m.lineCount)
	}

	// Sort by status: completed first (success, then error), then pending
	type sortedItem struct {
		name   string
		item   *ProgressItem
		sortKey int
	}
	var sorted []sortedItem
	for _, name := range m.order {
		item := m.items[name]
		key := 0
		switch item.Status {
		case StatusSuccess:
			key = 0
		case StatusError:
			key = 1
		default:
			key = 2
		}
		sorted = append(sorted, sortedItem{name, item, key})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].sortKey != sorted[j].sortKey {
			return sorted[i].sortKey < sorted[j].sortKey
		}
		return sorted[i].name < sorted[j].name
	})

	// Print final state
	for _, s := range sorted {
		line := m.renderItem(s.item)
		fmt.Println(line)
	}
}

// PrintPlain prints a plain-text line for non-TTY output.
func (m *MultiProgress) PrintPlain(format string, args ...interface{}) {
	if m.ui.shouldStyle() {
		return
	}
	fmt.Printf(format, args...)
}

// GetStatus returns the status of an item.
func (m *MultiProgress) GetStatus(name string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item, ok := m.items[name]; ok {
		return item.Status
	}
	return StatusNone
}

// HasErrors returns true if any item has failed.
func (m *MultiProgress) HasErrors() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range m.items {
		if item.Status == StatusError {
			return true
		}
	}
	return false
}

// AllComplete returns true if all items are done (success or error).
func (m *MultiProgress) AllComplete() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range m.items {
		if item.Status != StatusSuccess && item.Status != StatusError {
			return false
		}
	}
	return true
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hrs := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hrs, mins)
}

// SimpleProgressLine is a simpler single-line progress indicator.
type SimpleProgressLine struct {
	ui       *UI
	label    string
	total    int
	current  int
	mu       sync.Mutex
}

// NewSimpleProgressLine creates a simple [n/total] progress indicator.
func (u *UI) NewSimpleProgressLine(label string, total int) *SimpleProgressLine {
	return &SimpleProgressLine{
		ui:    u,
		label: label,
		total: total,
	}
}

// Update updates the progress.
func (p *SimpleProgressLine) Update(current int) {
	p.mu.Lock()
	p.current = current
	p.mu.Unlock()

	p.render()
}

func (p *SimpleProgressLine) render() {
	p.mu.Lock()
	current := p.current
	total := p.total
	p.mu.Unlock()

	if !p.ui.shouldStyle() {
		return
	}

	bar := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(25),
		progress.WithoutPercentage(),
	)

	pct := float64(current) / float64(total)
	if pct > 1 {
		pct = 1
	}

	countStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	fmt.Fprintf(os.Stdout, "\r\033[K  %s %s %s",
		p.label,
		bar.ViewAs(pct),
		countStyle.Render(fmt.Sprintf("[%d/%d]", current, total)),
	)
}

// Complete finishes the progress line.
func (p *SimpleProgressLine) Complete() {
	if !p.ui.shouldStyle() {
		return
	}

	fmt.Fprint(os.Stdout, "\r\033[K")
}

// IndexProgressDisplay shows index creation progress.
type IndexProgressDisplay struct {
	ui      *UI
	total   int
	current int
	mu      sync.Mutex
}

// NewIndexProgress creates an index progress display.
func (u *UI) NewIndexProgress(total int) *IndexProgressDisplay {
	return &IndexProgressDisplay{
		ui:    u,
		total: total,
	}
}

// Update updates the current index count.
func (p *IndexProgressDisplay) Update(current int) {
	p.mu.Lock()
	p.current = current
	p.mu.Unlock()

	if !p.ui.shouldStyle() {
		// For non-TTY, use carriage return like before
		fmt.Printf("  [%d/%d] Creating index/constraint...\r", current, p.total)
		return
	}

	bar := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(30),
		progress.WithoutPercentage(),
	)

	pct := float64(current) / float64(p.total)
	countStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	fmt.Fprintf(os.Stdout, "\r\033[K  %s %s %s",
		bar.ViewAs(pct),
		countStyle.Render(fmt.Sprintf("[%d/%d]", current, p.total)),
		StyleMuted.Render("Creating indexes..."),
	)
}

// Complete finishes with success.
func (p *IndexProgressDisplay) Complete() {
	if !p.ui.shouldStyle() {
		fmt.Println()
		return
	}

	fmt.Fprintf(os.Stdout, "\r\033[K  %s %s\n",
		StyleSuccess.Render(SymbolSuccess),
		fmt.Sprintf("Created %d indexes", p.total),
	)
}

// DurationSince returns formatted duration since a time.
func DurationSince(t time.Time) string {
	return formatDuration(time.Since(t))
}

// FormatBytes formats bytes into human readable form.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// PrintTableLoadResult prints a table load result line.
func (u *UI) PrintTableLoadResult(name string, rows int64, duration time.Duration, shards int, err error) {
	if !u.shouldStyle() {
		if err != nil {
			fmt.Printf("  %-15s FAILED\n", name+":")
			fmt.Printf("    Error: %v\n", err)
		} else if shards > 1 {
			fmt.Printf("  %-15s %s rows in %s (%d shards)\n", name+":", formatRowCount(rows), formatDuration(duration), shards)
		} else {
			fmt.Printf("  %-15s %s rows in %s\n", name+":", formatRowCount(rows), formatDuration(duration))
		}
		return
	}

	nameStyle := lipgloss.NewStyle().Width(15)
	if err != nil {
		fmt.Printf("  %s %s %s\n",
			StyleError.Render(SymbolError),
			nameStyle.Render(name),
			StyleError.Render("FAILED"),
		)
		fmt.Printf("    %s\n", StyleError.Render(err.Error()))
	} else {
		detail := fmt.Sprintf("%s rows in %s", formatRowCount(rows), formatDuration(duration))
		if shards > 1 {
			detail += StyleMuted.Render(fmt.Sprintf(" (%d shards)", shards))
		}
		fmt.Printf("  %s %s %s\n",
			StyleSuccess.Render(SymbolSuccess),
			nameStyle.Render(name),
			detail,
		)
	}
}

// formatRowCount formats a row count with K/M suffix.
func formatRowCount(rows int64) string {
	if rows >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(rows)/1_000_000)
	}
	if rows >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(rows)/1_000)
	}
	return fmt.Sprintf("%d", rows)
}

// PrintShardLoading prints a "loading N shards" message.
func (u *UI) PrintShardLoading(name string, shardCount int) {
	if !u.shouldStyle() {
		fmt.Printf("  %-15s loading %d shards...\n", name+":", shardCount)
		return
	}

	nameStyle := lipgloss.NewStyle().Width(15)
	fmt.Printf("  %s %s %s\n",
		StyleProgress.Render(SymbolProgress),
		nameStyle.Render(name),
		StyleMuted.Render(fmt.Sprintf("loading %d shards...", shardCount)),
	)
}

// Section prints a section header.
func (u *UI) Section(title string) {
	if !u.shouldStyle() {
		fmt.Printf("\n%s\n", title)
		return
	}

	fmt.Printf("\n%s\n", lipgloss.NewStyle().Bold(true).Render(title))
}

// Print prints a regular message.
func (u *UI) Print(msg string) {
	fmt.Println(msg)
}

// PrintSkipped prints a skipped message.
func (u *UI) PrintSkipped(name string, reason string) {
	if !u.shouldStyle() {
		fmt.Printf("  %-15s SKIPPED (%s)\n", name+":", reason)
		return
	}

	nameStyle := lipgloss.NewStyle().Width(15)
	fmt.Printf("  %s %s %s\n",
		StyleWarning.Render(SymbolWarning),
		nameStyle.Render(name),
		StyleMuted.Render("skipped: "+reason),
	)
}

// DebugBox prints a debug command box.
func (u *UI) DebugBox(title string, content string) {
	if !u.shouldStyle() {
		fmt.Println("\n    " + title)
		fmt.Println("    " + strings.Repeat("─", 45))
		for _, line := range strings.Split(content, "\n") {
			fmt.Println("    " + line)
		}
		fmt.Println("    " + strings.Repeat("─", 45))
		return
	}

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	fmt.Println()
	fmt.Println("    " + titleStyle.Render(title))
	fmt.Println(boxStyle.Render(content))
}
