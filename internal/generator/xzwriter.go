package generator

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// XZWriter streams data through the xz compressor to produce .xz files.
// It spawns an external xz process and pipes data through stdin.
type XZWriter struct {
	file    *os.File       // Output .xz file
	cmd     *exec.Cmd      // xz subprocess
	stdin   io.WriteCloser // Pipe to xz stdin
	path    string         // Full path to output file
	mu      sync.Mutex
	closed  bool
	waitErr error         // Error from xz process
	waitCh  chan struct{} // Signal when xz completes
}

// XZWriterConfig holds configuration for the XZ writer
type XZWriterConfig struct {
	// Directory where the file will be created
	OutputDir string
	// Filename without extension (e.g., "customers" -> "customers.csv.xz")
	Filename string
	// Compression preset 0-9 (default: 6). Higher = smaller but slower
	Preset int
}

// NewXZWriter creates a streaming XZ compressor that pipes data through
// the external xz command. The output file will have .csv.xz extension.
func NewXZWriter(cfg XZWriterConfig) (*XZWriter, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file for compressed data
	path := filepath.Join(cfg.OutputDir, cfg.Filename+".csv.xz")
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", path, err)
	}

	// Set compression preset (default to 6 if not specified or invalid)
	preset := cfg.Preset
	if preset < 0 || preset > 9 {
		preset = 6
	}

	// Set up xz command: reads from stdin, writes to stdout
	// -c = write to stdout, -<N> = compression level
	cmd := exec.Command("xz", "-c", fmt.Sprintf("-%d", preset))
	cmd.Stdout = file
	cmd.Stderr = os.Stderr // Surface xz errors to user

	// Create stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		file.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start xz process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		file.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to start xz: %w", err)
	}

	w := &XZWriter{
		file:   file,
		cmd:    cmd,
		stdin:  stdin,
		path:   path,
		waitCh: make(chan struct{}),
	}

	// Wait for xz in background goroutine
	go func() {
		w.waitErr = cmd.Wait()
		close(w.waitCh)
	}()

	return w, nil
}

// Write implements io.Writer, streaming data to the xz compressor
func (w *XZWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, fmt.Errorf("writer is closed")
	}

	return w.stdin.Write(p)
}

// Close finishes compression and waits for xz to exit.
// It closes stdin to signal EOF, waits for xz to finish processing,
// then closes the output file.
func (w *XZWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Close stdin to signal EOF to xz
	if err := w.stdin.Close(); err != nil {
		w.file.Close()
		return fmt.Errorf("failed to close xz stdin: %w", err)
	}

	// Wait for xz to finish processing all data
	<-w.waitCh

	// Close output file
	fileErr := w.file.Close()

	// Return any errors (xz error takes precedence)
	if w.waitErr != nil {
		return fmt.Errorf("xz process failed: %w", w.waitErr)
	}
	if fileErr != nil {
		return fmt.Errorf("failed to close output file: %w", fileErr)
	}

	return nil
}

// Path returns the full path to the .xz file
func (w *XZWriter) Path() string {
	return w.path
}

// CheckXZAvailable verifies that xz is installed and accessible.
// Returns nil if xz is available, or an error with installation guidance.
func CheckXZAvailable() error {
	cmd := exec.Command("xz", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xz not found: %w\nInstall with: apt install xz-utils (Linux) or brew install xz (macOS)", err)
	}
	return nil
}
