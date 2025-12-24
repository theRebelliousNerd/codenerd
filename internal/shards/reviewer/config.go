// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains configuration types and defaults.
package reviewer

// =============================================================================
// CONFIGURATION
// =============================================================================

// ReviewerConfig holds configuration for the reviewer shard.
type ReviewerConfig struct {
	StyleGuide      string   // Path to style guide or preset name
	SecurityRules   []string // Security patterns to check (OWASP categories)
	MaxFindings     int      // Max findings before abort (default: 100)
	BlockOnCritical bool     // Block commit if critical issues found (default: true)
	IncludeMetrics  bool     // Include complexity metrics (default: true)
	SeverityFilter  string   // Minimum severity to report: "info", "warning", "error", "critical"
	WorkingDir      string   // Workspace directory
	IgnorePatterns  []string // File patterns to ignore
	MaxFileSize     int64    // Max file size to review in bytes (default: 1MB)
	CustomRulesPath string   // Path to custom rules JSON file (default: .nerd/review-rules.json)

	// Neuro-symbolic pipeline configuration
	UseNeuroSymbolic bool // Enable neuro-symbolic pipeline (default: true for Go files)
}

// DefaultReviewerConfig returns sensible defaults for code review.
func DefaultReviewerConfig() ReviewerConfig {
	return ReviewerConfig{
		StyleGuide: "default",
		SecurityRules: []string{
			"sql_injection",
			"xss",
			"command_injection",
			"path_traversal",
			"hardcoded_secrets",
			"insecure_crypto",
			"unsafe_deserialization",
		},
		MaxFindings:      100,
		BlockOnCritical:  true,
		IncludeMetrics:   true,
		SeverityFilter:   "info",
		WorkingDir:       ".",
		IgnorePatterns:   []string{"vendor/", "node_modules/", ".git/", "*.min.js"},
		MaxFileSize:      1024 * 1024, // 1MB
		CustomRulesPath:  ".nerd/review-rules.json",
		UseNeuroSymbolic: true, // Enable by default for Go files
	}
}
