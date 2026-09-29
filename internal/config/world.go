package config

import (
	"fmt"
	"runtime"
	"strings"
)

// WorldConfig controls world-model scanning and AST parsing.
type WorldConfig struct {
	// FastWorkers caps concurrent fast parse workers (tree-sitter).
	FastWorkers int `yaml:"fast_workers" json:"fast_workers,omitempty"`
	// DeepWorkers caps concurrent deep parse workers (Cartographer/Go AST).
	DeepWorkers int `yaml:"deep_workers" json:"deep_workers,omitempty"`
	// IgnorePatterns are extra exclusions applied on top of .gitignore
	// (or, outside a git work tree, the whole member test besides the
	// always-excluded .git and .nerd directories). Last match wins.
	// A trailing slash is directory-only and is not an anchor: `build/`
	// matches a directory named build anywhere, while `/build/` and
	// `src/build/` are relative to the workspace root. A pattern with no
	// slash matches that name in any directory. `*` and `?` match inside
	// one path segment; `**` is special only as its own segment. A leading
	// `!` re-includes a path an earlier pattern dropped. It does not
	// re-include a gitignored path, and it cannot bring .git or .nerd back.
	// `\` is treated as `/`. An empty pattern, a bare `!`, an empty
	// segment, or an unclosed `[` is rejected by Check.
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

// ValidateIgnorePattern reports why p cannot be compiled as an ignore pattern.
// The matcher skips the same shapes, so a pattern Check rejects never excludes
// a path by accident.
func ValidateIgnorePattern(p string) error {
	s := strings.TrimSpace(p)
	s = strings.ReplaceAll(s, "\\", "/")
	if s == "" {
		return fmt.Errorf("empty pattern")
	}
	if strings.HasPrefix(s, "!") {
		s = strings.TrimSpace(s[1:])
		if s == "" {
			return fmt.Errorf("bare negation")
		}
	}
	for strings.HasSuffix(s, "/") {
		s = strings.TrimSuffix(s, "/")
	}
	if s == "" || strings.Contains(s, "//") {
		return fmt.Errorf("empty path segment")
	}
	open := false
	for _, r := range s {
		switch r {
		case '[':
			if !open {
				open = true
			}
		case ']':
			if open {
				open = false
			}
		}
	}
	if open {
		return fmt.Errorf("unclosed '['")
	}
	return nil
}

// Check reports ignore patterns the matcher cannot compile.
// prefix is the JSON path of this object, usually "world".
func (c WorldConfig) Check(prefix string) []Problem {
	if prefix == "" {
		prefix = "world"
	}
	var out []Problem
	for i, p := range c.IgnorePatterns {
		err := ValidateIgnorePattern(p)
		if err == nil {
			continue
		}
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     fmt.Sprintf("%s.ignore_patterns[%d]", prefix, i),
			Message:  err.Error(),
			Fix:      "a name (node_modules), a directory (build/), a root-relative path (src/generated/**), or a leading ! that re-includes an earlier pattern",
		})
	}
	return out
}
