package generator

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// ProgressReporter tracks and displays progress for long-running operations.
// It provides real-time updates with percentage, rate, and ETA.
type ProgressReporter struct {
	mu sync.Mutex

	// Configuration
	output     io.Writer
	total      int64
	label      string
	width      int
	updateFreq time.Duration
	isTTY      bool

	// State
	current   int64
	startTime time.Time
	lastPrint time.Time
	done      bool
}

// ProgressConfig holds settings for the progress reporter
type ProgressConfig struct {
	// Total number of items to process (0 for indeterminate)
	Total int64
	// Label to display (e.g., "Generating transactions")
	Label string
	// Output writer (defaults to os.Stderr)
	Output io.Writer
	// Minimum time between updates (defaults to 100ms)
	UpdateFrequency time.Duration
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(cfg ProgressConfig) *ProgressReporter {
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	updateFreq := cfg.UpdateFrequency
	if updateFreq == 0 {
		updateFreq = 100 * time.Millisecond
	}

	// Check if output is a TTY
	isTTY := false
	if f, ok := output.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	// Get terminal width
	width := 60
	if isTTY {
		if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 0 {
			width = w
		}
	}

	return &ProgressReporter{
		output:     output,
		total:      cfg.Total,
		label:      cfg.Label,
		width:      width,
		updateFreq: updateFreq,
		isTTY:      isTTY,
		startTime:  time.Now(),
	}
}

// Add increments the progress by n items
func (p *ProgressReporter) Add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current += n
	p.maybeRender()
}

// Set sets the current progress to n items
func (p *ProgressReporter) Set(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = n
	p.maybeRender()
}

// Increment adds 1 to the progress
func (p *ProgressReporter) Increment() {
	p.Add(1)
}

// SetTotal updates the total (useful when total is unknown initially)
func (p *ProgressReporter) SetTotal(total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.total = total
}

// maybeRender renders progress if enough time has passed
func (p *ProgressReporter) maybeRender() {
	now := time.Now()
	if now.Sub(p.lastPrint) < p.updateFreq {
		return
	}
	p.lastPrint = now
	p.render()
}

// render outputs the current progress
func (p *ProgressReporter) render() {
	elapsed := time.Since(p.startTime)

	// Calculate rate
	rate := float64(p.current) / elapsed.Seconds()
	if elapsed.Seconds() < 0.01 {
		rate = 0
	}

	// Build progress string
	var sb strings.Builder

	if p.isTTY {
		sb.WriteString("\r")
	}

	// Label
	if p.label != "" {
		sb.WriteString(p.label)
		sb.WriteString(": ")
	}

	// Progress numbers and percentage
	if p.total > 0 {
		pct := float64(p.current) / float64(p.total) * 100
		sb.WriteString(fmt.Sprintf("%d/%d (%.1f%%)", p.current, p.total, pct))

		// Progress bar for TTY
		if p.isTTY {
			sb.WriteString(" ")
			barWidth := 20
			filled := int(float64(barWidth) * float64(p.current) / float64(p.total))
			sb.WriteString("[")
			sb.WriteString(strings.Repeat("=", filled))
			if filled < barWidth {
				sb.WriteString(">")
				sb.WriteString(strings.Repeat(" ", barWidth-filled-1))
			}
			sb.WriteString("]")
		}

		// ETA
		if rate > 0 && p.current < p.total {
			remaining := float64(p.total-p.current) / rate
			eta := time.Duration(remaining * float64(time.Second))
			sb.WriteString(fmt.Sprintf(" ETA: %s", formatDuration(eta)))
		}
	} else {
		// Indeterminate progress
		sb.WriteString(fmt.Sprintf("%d", p.current))
	}

	// Rate
	sb.WriteString(fmt.Sprintf(" (%.0f/s)", rate))

	// Clear to end of line for TTY
	if p.isTTY {
		sb.WriteString("\033[K")
	} else {
		sb.WriteString("\n")
	}

	fmt.Fprint(p.output, sb.String())
}

// Finish completes the progress and prints final stats
func (p *ProgressReporter) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done {
		return
	}
	p.done = true

	elapsed := time.Since(p.startTime)
	rate := float64(p.current) / elapsed.Seconds()
	if elapsed.Seconds() < 0.01 {
		rate = 0
	}

	var sb strings.Builder

	if p.isTTY {
		sb.WriteString("\r")
	}

	if p.label != "" {
		sb.WriteString(p.label)
		sb.WriteString(": ")
	}

	sb.WriteString(fmt.Sprintf("%d items in %s (%.0f/s)",
		p.current,
		formatDuration(elapsed),
		rate))

	if p.isTTY {
		sb.WriteString("\033[K")
	}
	sb.WriteString("\n")

	fmt.Fprint(p.output, sb.String())
}

// formatDuration formats a duration in a human-readable way
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
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// MultiProgress manages multiple progress phases
type MultiProgress struct {
	mu      sync.Mutex
	output  io.Writer
	isTTY   bool
	phases  []Phase
	current int
}

// Phase represents a single phase in a multi-phase operation
type Phase struct {
	Name      string
	Completed bool
	Count     int64
	Duration  time.Duration
}

// NewMultiProgress creates a progress tracker for multiple phases
func NewMultiProgress(output io.Writer) *MultiProgress {
	if output == nil {
		output = os.Stderr
	}

	isTTY := false
	if f, ok := output.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	return &MultiProgress{
		output: output,
		isTTY:  isTTY,
		phases: make([]Phase, 0),
	}
}

// StartPhase begins a new phase
func (m *MultiProgress) StartPhase(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.phases = append(m.phases, Phase{Name: name})
	m.current = len(m.phases) - 1

	if m.isTTY {
		fmt.Fprintf(m.output, "\r%s...\033[K", name)
	} else {
		fmt.Fprintf(m.output, "%s...\n", name)
	}
}

// CompletePhase finishes the current phase with count and duration
func (m *MultiProgress) CompletePhase(count int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current < len(m.phases) {
		m.phases[m.current].Completed = true
		m.phases[m.current].Count = count
		m.phases[m.current].Duration = duration

		phase := m.phases[m.current]
		if m.isTTY {
			fmt.Fprintf(m.output, "\r%s: %d in %s\033[K\n",
				phase.Name, count, formatDuration(duration))
		} else {
			fmt.Fprintf(m.output, "  -> %d items in %s\n",
				count, formatDuration(duration))
		}
	}
}

// Summary returns a formatted summary of all phases
func (m *MultiProgress) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	var totalDuration time.Duration
	var totalItems int64

	for _, phase := range m.phases {
		if phase.Completed {
			sb.WriteString(fmt.Sprintf("  %s: %d items\n", phase.Name, phase.Count))
			totalDuration += phase.Duration
			totalItems += phase.Count
		}
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d items in %s",
		totalItems, formatDuration(totalDuration)))

	return sb.String()
}

// ProgressCallback is a function called with progress updates
type ProgressCallback func(current, total int64, phase string)

// ProgressWriter wraps a progress reporter for use during writes
type ProgressWriter struct {
	progress *ProgressReporter
	batchSize int64
	count     int64
}

// NewProgressWriter creates a progress-tracking wrapper
func NewProgressWriter(label string, total int64) *ProgressWriter {
	return &ProgressWriter{
		progress: NewProgressReporter(ProgressConfig{
			Total: total,
			Label: label,
		}),
		batchSize: 1000, // Update every 1000 items
	}
}

// Increment adds 1 to the count
func (pw *ProgressWriter) Increment() {
	pw.count++
	if pw.count%pw.batchSize == 0 {
		pw.progress.Set(pw.count)
	}
}

// Add adds n to the count
func (pw *ProgressWriter) Add(n int64) {
	pw.count += n
	if pw.count%(pw.batchSize) == 0 || n > pw.batchSize {
		pw.progress.Set(pw.count)
	}
}

// Finish completes the progress display
func (pw *ProgressWriter) Finish() {
	pw.progress.Set(pw.count)
	pw.progress.Finish()
}

// AggregatedProgressReporter collects progress from multiple workers and
// displays combined progress. It receives updates through channels from
// parallel workers and aggregates them into a single progress display.
type AggregatedProgressReporter struct {
	mu sync.Mutex

	// Configuration
	output      io.Writer
	total       int64
	label       string
	workerCount int
	updateFreq  time.Duration
	isTTY       bool

	// State
	workerCounts []int64 // Count per worker
	current      int64   // Total count across all workers
	startTime    time.Time
	lastPrint    time.Time
	done         bool

	// Channels
	progressChan chan workerProgress
	doneChan     chan struct{}
}

// workerProgress represents a progress update from a worker
type workerProgress struct {
	workerID int
	count    int64
}

// AggregatedProgressConfig holds settings for the aggregated progress reporter
type AggregatedProgressConfig struct {
	// Total number of items to process (0 for indeterminate)
	Total int64
	// Label to display (e.g., "Generating transactions")
	Label string
	// Number of workers reporting progress
	WorkerCount int
	// Output writer (defaults to os.Stderr)
	Output io.Writer
	// Minimum time between updates (defaults to 100ms)
	UpdateFrequency time.Duration
}

// NewAggregatedProgressReporter creates a new aggregated progress reporter
func NewAggregatedProgressReporter(cfg AggregatedProgressConfig) *AggregatedProgressReporter {
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	updateFreq := cfg.UpdateFrequency
	if updateFreq == 0 {
		updateFreq = 100 * time.Millisecond
	}

	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}

	// Check if output is a TTY
	isTTY := false
	if f, ok := output.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	return &AggregatedProgressReporter{
		output:       output,
		total:        cfg.Total,
		label:        cfg.Label,
		workerCount:  workerCount,
		updateFreq:   updateFreq,
		isTTY:        isTTY,
		workerCounts: make([]int64, workerCount),
		startTime:    time.Now(),
		progressChan: make(chan workerProgress, workerCount*100), // Buffered channel
		doneChan:     make(chan struct{}),
	}
}

// Start begins listening for progress updates from workers.
// Call this in a goroutine before workers start.
func (a *AggregatedProgressReporter) Start() {
	go a.listen()
}

// listen processes incoming progress updates
func (a *AggregatedProgressReporter) listen() {
	ticker := time.NewTicker(a.updateFreq)
	defer ticker.Stop()

	for {
		select {
		case update := <-a.progressChan:
			a.mu.Lock()
			if update.workerID >= 0 && update.workerID < len(a.workerCounts) {
				a.workerCounts[update.workerID] = update.count
				a.current = 0
				for _, c := range a.workerCounts {
					a.current += c
				}
			}
			a.mu.Unlock()

		case <-ticker.C:
			a.mu.Lock()
			if !a.done {
				a.render()
			}
			a.mu.Unlock()

		case <-a.doneChan:
			// Drain any remaining updates
			for {
				select {
				case update := <-a.progressChan:
					a.mu.Lock()
					if update.workerID >= 0 && update.workerID < len(a.workerCounts) {
						a.workerCounts[update.workerID] = update.count
						a.current = 0
						for _, c := range a.workerCounts {
							a.current += c
						}
					}
					a.mu.Unlock()
				default:
					return
				}
			}
		}
	}
}

// ReportProgress sends a progress update from a worker.
// This is safe to call from multiple goroutines.
func (a *AggregatedProgressReporter) ReportProgress(workerID int, count int64) {
	select {
	case a.progressChan <- workerProgress{workerID: workerID, count: count}:
	default:
		// Channel full, skip this update (next one will catch up)
	}
}

// GetProgressChan returns a channel for a specific worker to send progress updates.
// The worker should send its current count (not delta) to this channel.
func (a *AggregatedProgressReporter) GetProgressChan() chan<- workerProgress {
	return a.progressChan
}

// render outputs the current aggregated progress
func (a *AggregatedProgressReporter) render() {
	now := time.Now()
	if now.Sub(a.lastPrint) < a.updateFreq {
		return
	}
	a.lastPrint = now

	elapsed := time.Since(a.startTime)
	rate := float64(a.current) / elapsed.Seconds()
	if elapsed.Seconds() < 0.01 {
		rate = 0
	}

	var sb strings.Builder

	if a.isTTY {
		sb.WriteString("\r")
	}

	if a.label != "" {
		sb.WriteString(a.label)
		sb.WriteString(": ")
	}

	if a.total > 0 {
		pct := float64(a.current) / float64(a.total) * 100
		sb.WriteString(fmt.Sprintf("%d/%d (%.1f%%)", a.current, a.total, pct))

		if a.isTTY {
			sb.WriteString(" ")
			barWidth := 20
			filled := int(float64(barWidth) * float64(a.current) / float64(a.total))
			if filled > barWidth {
				filled = barWidth
			}
			sb.WriteString("[")
			sb.WriteString(strings.Repeat("=", filled))
			if filled < barWidth {
				sb.WriteString(">")
				sb.WriteString(strings.Repeat(" ", barWidth-filled-1))
			}
			sb.WriteString("]")
		}

		if rate > 0 && a.current < a.total {
			remaining := float64(a.total-a.current) / rate
			eta := time.Duration(remaining * float64(time.Second))
			sb.WriteString(fmt.Sprintf(" ETA: %s", formatDuration(eta)))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%d", a.current))
	}

	sb.WriteString(fmt.Sprintf(" (%.0f/s) [%d workers]", rate, a.workerCount))

	if a.isTTY {
		sb.WriteString("\033[K")
	} else {
		sb.WriteString("\n")
	}

	fmt.Fprint(a.output, sb.String())
}

// Finish completes the aggregated progress and prints final stats
func (a *AggregatedProgressReporter) Finish() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return
	}
	a.done = true

	// Signal listener to stop
	close(a.doneChan)

	// Final count
	a.current = 0
	for _, c := range a.workerCounts {
		a.current += c
	}

	elapsed := time.Since(a.startTime)
	rate := float64(a.current) / elapsed.Seconds()
	if elapsed.Seconds() < 0.01 {
		rate = 0
	}

	var sb strings.Builder

	if a.isTTY {
		sb.WriteString("\r")
	}

	if a.label != "" {
		sb.WriteString(a.label)
		sb.WriteString(": ")
	}

	sb.WriteString(fmt.Sprintf("%d items in %s (%.0f/s) [%d workers]",
		a.current,
		formatDuration(elapsed),
		rate,
		a.workerCount))

	if a.isTTY {
		sb.WriteString("\033[K")
	}
	sb.WriteString("\n")

	fmt.Fprint(a.output, sb.String())
}

// SetTotal updates the total (useful when total is calculated after creation)
func (a *AggregatedProgressReporter) SetTotal(total int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.total = total
}

// Current returns the current aggregated count
func (a *AggregatedProgressReporter) Current() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current
}
