package generator

import (
	"fmt"
	"path/filepath"
	"sort"
)

// ShardFilename generates a shard filename with zero-padded shard number.
// Example: ShardFilename("transactions", 1, 8) returns "transactions_001"
// The padding width is determined by the total number of shards (minimum 3 digits).
func ShardFilename(basename string, shardNum, totalShards int) string {
	width := len(fmt.Sprintf("%d", totalShards))
	if width < 3 {
		width = 3 // Minimum 3 digits for aesthetics
	}
	return fmt.Sprintf("%s_%0*d", basename, width, shardNum)
}

// ShardFilePath generates the full path to a shard file.
// If compress is true, returns path with .csv.xz extension, otherwise .csv
func ShardFilePath(outputDir, basename string, shardNum, totalShards int, compress bool) string {
	name := ShardFilename(basename, shardNum, totalShards)
	ext := ".csv"
	if compress {
		ext = ".csv.xz"
	}
	return filepath.Join(outputDir, name+ext)
}

// NewShardedCSVWriter creates a CSVWriter for a specific shard.
// The filename will be basename_NNN where NNN is the zero-padded shard number.
func NewShardedCSVWriter(cfg CSVWriterConfig, shardNum, totalShards int) (*CSVWriter, error) {
	shardedCfg := CSVWriterConfig{
		OutputDir:  cfg.OutputDir,
		Filename:   ShardFilename(cfg.Filename, shardNum, totalShards),
		Headers:    cfg.Headers,
		BufferSize: cfg.BufferSize,
		Compress:   cfg.Compress,
		XZPreset:   cfg.XZPreset,
	}
	return NewCSVWriter(shardedCfg)
}

// FindShardedFiles finds all shard files matching the pattern basename_*.csv or basename_*.csv.xz
// Returns the files sorted in order (001, 002, etc.)
func FindShardedFiles(inputDir, basename string) ([]string, error) {
	// Try both compressed and uncompressed patterns
	patterns := []string{
		filepath.Join(inputDir, basename+"_*.csv.xz"),
		filepath.Join(inputDir, basename+"_*.csv"),
	}

	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob error for pattern %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return nil, nil // No shards found
	}

	// Sort to ensure consistent order (001, 002, ...)
	sort.Strings(files)

	return files, nil
}

// ShardInfo contains information about a set of shard files
type ShardInfo struct {
	Files      []string // Sorted list of shard file paths
	ShardCount int      // Number of shards
	Compressed bool     // Whether files are compressed (.csv.xz)
}

// GetShardInfo returns information about shards for a given basename
func GetShardInfo(inputDir, basename string) (*ShardInfo, error) {
	files, err := FindShardedFiles(inputDir, basename)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	// Determine if compressed based on first file
	compressed := false
	if len(files) > 0 {
		compressed = filepath.Ext(files[0]) == ".xz"
	}

	return &ShardInfo{
		Files:      files,
		ShardCount: len(files),
		Compressed: compressed,
	}, nil
}
