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
}

// DefaultWorldConfig returns defaults for world-model scanning.
func DefaultWorldConfig() WorldConfig {
	fast := max(min(runtime.NumCPU(), 20), 4)
	deep := max(min(runtime.NumCPU(), 8), 2)
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
	}
}
