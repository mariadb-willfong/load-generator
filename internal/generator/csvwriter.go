package generator

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CSVWriter provides a streaming, memory-efficient CSV writer for large data files.
// It uses buffered I/O and writes rows immediately to minimize memory usage.
// Optionally supports xz compression via external xz process.
type CSVWriter struct {
	file       *os.File      // Only used for uncompressed output
	xzWriter   *XZWriter     // Only used for compressed output
	buffer     *bufio.Writer
	writer     *csv.Writer
	mu         sync.Mutex
	rowCount   int64
	headers    []string
	closed     bool
	compressed bool // Track if using compression
}

// CSVWriterConfig holds configuration for creating a CSV writer
type CSVWriterConfig struct {
	// Directory where the file will be created
	OutputDir string
	// Filename without extension (e.g., "customers")
	Filename string
	// Column headers
	Headers []string
	// Buffer size in bytes (default: 64KB)
	BufferSize int
	// Enable xz compression (creates .csv.xz files)
	Compress bool
	// XZ compression preset 0-9 (default: 6). Higher = smaller but slower
	XZPreset int
}

// NewCSVWriter creates a new streaming CSV writer.
// The file is created immediately and headers are written.
// If Compress is true, output is piped through xz for compression.
func NewCSVWriter(cfg CSVWriterConfig) (*CSVWriter, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Set buffer size
	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 64 * 1024 // 64KB default
	}

	// Determine underlying writer based on compression setting
	var underlying io.Writer
	var file *os.File
	var xzWriter *XZWriter

	if cfg.Compress {
		// Use XZ compression - pipe through external xz process
		var err error
		xzWriter, err = NewXZWriter(XZWriterConfig{
			OutputDir: cfg.OutputDir,
			Filename:  cfg.Filename,
			Preset:    cfg.XZPreset,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create xz writer: %w", err)
		}
		underlying = xzWriter
	} else {
		// Direct file writing (uncompressed)
		path := filepath.Join(cfg.OutputDir, cfg.Filename+".csv")
		var err error
		file, err = os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", path, err)
		}
		underlying = file
	}

	buffer := bufio.NewWriterSize(underlying, bufSize)
	writer := csv.NewWriter(buffer)

	cw := &CSVWriter{
		file:       file,
		xzWriter:   xzWriter,
		buffer:     buffer,
		writer:     writer,
		headers:    cfg.Headers,
		compressed: cfg.Compress,
	}

	// Write headers
	if len(cfg.Headers) > 0 {
		if err := writer.Write(cfg.Headers); err != nil {
			cw.closeUnderlying()
			return nil, fmt.Errorf("failed to write headers: %w", err)
		}
	}

	return cw, nil
}

// WriteRow writes a single row to the CSV file.
// This method is thread-safe.
func (w *CSVWriter) WriteRow(row []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	if err := w.writer.Write(row); err != nil {
		return fmt.Errorf("failed to write row: %w", err)
	}
	w.rowCount++

	return nil
}

// WriteRows writes multiple rows to the CSV file.
// This method is thread-safe.
func (w *CSVWriter) WriteRows(rows [][]string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	for _, row := range rows {
		if err := w.writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
		w.rowCount++
	}

	return nil
}

// Flush forces any buffered data to be written to disk.
// Call this periodically for long-running writes to prevent data loss.
func (w *CSVWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		return fmt.Errorf("csv flush error: %w", err)
	}
	return w.buffer.Flush()
}

// Close flushes remaining data and closes the file.
// Always call Close when done writing.
func (w *CSVWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		w.closeUnderlying()
		return fmt.Errorf("csv flush error: %w", err)
	}

	if err := w.buffer.Flush(); err != nil {
		w.closeUnderlying()
		return fmt.Errorf("buffer flush error: %w", err)
	}

	return w.closeUnderlying()
}

// closeUnderlying closes the underlying writer (file or xz process)
func (w *CSVWriter) closeUnderlying() error {
	if w.compressed {
		return w.xzWriter.Close()
	}
	return w.file.Close()
}

// RowCount returns the number of data rows written (excludes header).
func (w *CSVWriter) RowCount() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rowCount
}

// Path returns the full path to the output file (.csv or .csv.xz)
func (w *CSVWriter) Path() string {
	if w.compressed {
		return w.xzWriter.Path()
	}
	return w.file.Name()
}

// FormatBool converts a boolean to "1" or "0" for CSV/database compatibility
func FormatBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// FormatTime formats a time.Time for CSV in MySQL datetime format
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// FormatDate formats a time.Time for CSV in MySQL date format
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatTimePtr formats a *time.Time for CSV, returning empty string for nil
func FormatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return FormatTime(*t)
}

// FormatInt64 formats an int64 for CSV
func FormatInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}

// FormatInt formats an int for CSV
func FormatInt(n int) string {
	return fmt.Sprintf("%d", n)
}

// FormatFloat64 formats a float64 for CSV with appropriate precision
func FormatFloat64(f float64) string {
	return fmt.Sprintf("%.6f", f)
}

// FormatInt64Ptr formats an *int64 for CSV, returning empty string for nil
func FormatInt64Ptr(n *int64) string {
	if n == nil {
		return ""
	}
	return FormatInt64(*n)
}
