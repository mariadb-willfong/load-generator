package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Spinner provides an animated spinner for indeterminate operations.
type Spinner struct {
	ui      *UI
	label   string
	done    chan struct{}
	wg      sync.WaitGroup
	started bool
	mu      sync.Mutex
}

// Spinner animation frames (braille pattern).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a new animated spinner.
func (u *UI) NewSpinner(label string) *Spinner {
	return &Spinner{
		ui:    u,
		label: label,
		done:  make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	if !s.ui.shouldStyle() {
		// Non-TTY: just print the message once
		fmt.Printf("%s...", s.label)
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		frame := 0
		spinnerStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stdout, "\r%s %s...",
					spinnerStyle.Render(spinnerFrames[frame]),
					s.label,
				)
				frame = (frame + 1) % len(spinnerFrames)
			}
		}
	}()
}

// Stop stops the spinner without showing a final status.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	close(s.done)
	s.wg.Wait()

	if s.ui.shouldStyle() {
		// Clear the line
		fmt.Fprint(os.Stdout, "\r\033[K")
	}
}

// Success stops the spinner and shows a success message.
func (s *Spinner) Success(msg string) {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	select {
	case <-s.done:
		// Already stopped
	default:
		close(s.done)
	}
	s.wg.Wait()

	if !s.ui.shouldStyle() {
		fmt.Printf(" %s\n", msg)
		return
	}

	fmt.Fprintf(os.Stdout, "\r\033[K%s %s... %s\n",
		StyleSuccess.Render(SymbolSuccess),
		s.label,
		msg,
	)
}

// Error stops the spinner and shows an error message.
func (s *Spinner) Error(msg string) {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	select {
	case <-s.done:
		// Already stopped
	default:
		close(s.done)
	}
	s.wg.Wait()

	if !s.ui.shouldStyle() {
		fmt.Printf(" %s\n", msg)
		return
	}

	fmt.Fprintf(os.Stdout, "\r\033[K%s %s... %s\n",
		StyleError.Render(SymbolError),
		s.label,
		StyleError.Render(msg),
	)
}
