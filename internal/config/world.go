package config

import "runtime"

// WorldConfig controls world-model scanning and AST parsing.
type WorldConfig struct {
	// FastWorkers caps concurrent fast parse workers (tree-sitter).
	FastWorkers int `yaml:"fast_workers" json:"fast_workers,omitempty"`
	// DeepWorkers caps concurrent deep parse workers (Cartographer/Go AST).
	DeepWorkers int `yaml:"deep_workers" json:"deep_workers,omitempty"`
	// IgnorePatterns skips matching paths/dirs (relative to workspace).
	IgnorePatterns []string `yaml:"ignore_patterns" json:"ignore_patterns,omitempty"`
	// MaxFastASTBytes skips fast AST parsing for large files.
	MaxFastASTBytes int64 `yaml:"max_fast_ast_bytes" json:"max_fast_ast_bytes,omitempty"`
	// MaxFilesPerScan bounds changed files the world-model ingestor processes
	// per tick. Files past the cap keep stale entries and are picked up on
	// the next tick, so the cap defers work instead of dropping it.
	// Nil means the key was absent and resolves to 100. A pointer is required
	// because 0 is meaningful: an explicit 0 or a negative value stays
	// unbounded (the ingestor treats non-positive as no cap). Filling a
	// non-positive value with the default would make that setting impossible.
	MaxFilesPerScan *int `yaml:"max_files_per_scan" json:"max_files_per_scan,omitempty"`
}

// DefaultWorldConfig returns defaults for world-model scanning.
func DefaultWorldConfig() WorldConfig {
	fast := max(min(runtime.NumCPU(), 20), 4)
	deep := max(min(runtime.NumCPU(), 8), 2)
	filesPerScan := 100
	return WorldConfig{
		FastWorkers: fast,
		DeepWorkers: deep,
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
		MaxFastASTBytes: 2 * 1024 * 1024,
		MaxFilesPerScan: &filesPerScan,
	}
}

// ResolvedMaxFilesPerScan is the cap the ingestor should use.
// Nil (the key was absent) is the default 100. An explicit 0 or a negative
// value is returned unchanged so the ingestor can treat it as unbounded.
func (c WorldConfig) ResolvedMaxFilesPerScan() int {
	if c.MaxFilesPerScan == nil {
		return *DefaultWorldConfig().MaxFilesPerScan
	}
	return *c.MaxFilesPerScan
}
