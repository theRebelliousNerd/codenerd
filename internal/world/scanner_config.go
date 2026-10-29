package world

import (
	"runtime"

	"codenerd/internal/config"
	"codenerd/internal/features"
)

// ScannerConfig controls workspace scanning performance and scope.
type ScannerConfig struct {
	// MaxConcurrency limits concurrent file workers for fast parsing.
	MaxConcurrency int
	// IgnorePatterns is the user's extra exclusion list (world.ignore_patterns).
	// The copy in config.DefaultWorldConfig is the only default. Membership
	// compiles these; this field is not matched on its own.
	IgnorePatterns []string
	// MaxASTFileBytes skips fast AST parsing for files larger than this size.
	// Hashing and file_topology still happen.
	MaxASTFileBytes int64
}

// DefaultScannerConfig returns sane defaults for large repositories.
// Tunables (worker count, max AST file size) come from the
// internal/features registry, which honours .nerd/config.json keys and
// the NERD_FAST_SCAN_WORKERS / NERD_FAST_AST_MAX_BYTES env vars
// (env wins over config, config wins over compile-time defaults).
func DefaultScannerConfig() ScannerConfig {
	workers := max(min(runtime.NumCPU(), 20), 4)
	if w := features.FastScanWorkers(); w > 0 {
		workers = w
	}

	maxBytes := int64(2 * 1024 * 1024) // 2MB default
	if b := features.FastASTMaxBytes(); b > 0 {
		maxBytes = b
	}

	return ScannerConfig{
		MaxConcurrency:  workers,
		IgnorePatterns:  append([]string(nil), config.DefaultWorldConfig().IgnorePatterns...),
		MaxASTFileBytes: maxBytes,
	}
}
