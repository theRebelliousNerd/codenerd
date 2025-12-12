package world

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ScannerConfig controls workspace scanning performance and scope.
type ScannerConfig struct {
	// MaxConcurrency limits concurrent file workers for fast parsing.
	MaxConcurrency int
	// IgnorePatterns skips matching paths/dirs (relative to workspace).
	// Supports simple dir names (e.g., "node_modules") and glob patterns (e.g., "vendor/*").
	IgnorePatterns []string
	// MaxASTFileBytes skips fast AST parsing for files larger than this size.
	// Hashing and file_topology still happen.
	MaxASTFileBytes int64
}

// DefaultScannerConfig returns sane defaults for large repositories.
func DefaultScannerConfig() ScannerConfig {
	workers := runtime.NumCPU()
	if workers > 20 {
		workers = 20
	}
	if workers < 4 {
		workers = 4
	}
	if env := os.Getenv("NERD_FAST_SCAN_WORKERS"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			workers = v
		}
	}

	maxBytes := int64(2 * 1024 * 1024) // 2MB default
	if env := os.Getenv("NERD_FAST_AST_MAX_BYTES"); env != "" {
		if v, err := strconv.ParseInt(env, 10, 64); err == nil && v > 0 {
			maxBytes = v
		}
	}

	return ScannerConfig{
		MaxConcurrency: workers,
		IgnorePatterns: []string{
			".git",
			".nerd",
			"node_modules",
			"vendor",
			"dist",
			"build",
			".next",
			"target",
			"bin",
			"obj",
			".terraform",
			".venv",
			".cache",
		},
		MaxASTFileBytes: maxBytes,
	}
}

func normalizePattern(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "/")
	p = strings.TrimSuffix(p, "\\")
	return filepath.ToSlash(p)
}

// isIgnoredRel reports whether a relative path should be ignored.
func isIgnoredRel(rel, name string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, raw := range patterns {
		p := normalizePattern(raw)
		if p == "" {
			continue
		}
		// Glob pattern
		if strings.ContainsAny(p, "*?[]") {
			if ok, _ := path.Match(p, rel); ok {
				return true
			}
			// Handle directory globs like "vendor/*"
			if strings.HasSuffix(p, "/*") {
				prefix := strings.TrimSuffix(p, "/*")
				if strings.HasPrefix(rel, prefix+"/") {
					return true
				}
			}
			continue
		}
		// Simple dir/file name
		if name == p {
			return true
		}
		// Prefix match for nested paths
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

