package reviewer

import (
	"codenerd/internal/logging"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// SECURITY CHECKS
// =============================================================================

// checkSecurity performs security vulnerability checks.
func (r *ReviewerShard) checkSecurity(filePath, content string) []ReviewFinding {
	logging.ReviewerDebug("Running security checks on: %s", filePath)
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)
	logging.ReviewerDebug("Detected language: %s, scanning %d lines", lang, len(lines))

	// Security patterns to check
	securityPatterns := []struct {
		Pattern     *regexp.Regexp
		RuleID      string
		Severity    string
		Message     string
		Suggestion  string
		Languages   []string // Empty means all languages
		RequireSQL  bool     // If true, line must also contain SQL keywords
		ExcludeFunc []string // Skip if line contains these function names (safe contexts)
	}{
		// SQL Injection - requires SQL keywords to reduce false positives
		// This pattern looks for string interpolation in database-like contexts
		{
			Pattern:     regexp.MustCompile(`(?i)(\.Query|\.Exec|\.Raw|QueryRow|ExecContext)\s*\([^)]*(\+|fmt\.Sprintf|fmt\.Fprintf|string\()`),
			RuleID:      "SEC001",
			Severity:    "critical",
			Message:     "Potential SQL injection: string interpolation in database query",
			Suggestion:  "Use parameterized queries with ? or $1 placeholders instead",
			Languages:   []string{"go"},
			RequireSQL:  false, // Already specific to DB methods
			ExcludeFunc: nil,
		},
		// SQL Injection - generic fmt.Sprintf with SQL keywords
		{
			Pattern:     regexp.MustCompile(`(?i)fmt\.Sprintf\s*\(\s*["'][^"']*(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|INTO)\s`),
			RuleID:      "SEC001",
			Severity:    "critical",
			Message:     "Potential SQL injection: fmt.Sprintf with SQL keywords",
			Suggestion:  "Use parameterized queries instead of string formatting",
			Languages:   []string{"go"},
			RequireSQL:  false,
			ExcludeFunc: []string{"log.", "Log", "Error", "Warn", "Info", "Debug", "fmt.Print", "fmt.Fprint"},
		},
		// Command Injection
		{
			Pattern:     regexp.MustCompile(`(?i)(exec\.Command|os\.system|subprocess\.|child_process\.exec)\s*\([^)]*\+`),
			RuleID:      "SEC002",
			Severity:    "critical",
			Message:     "Potential command injection: user input in command execution",
			Suggestion:  "Sanitize inputs or use safer alternatives",
			RequireSQL:  false,
			ExcludeFunc: nil,
		},
		// Hardcoded Secrets
		{
			Pattern:     regexp.MustCompile(`(?i)(password|secret|api_key|apikey|token|credential)\s*[:=]\s*["'][^"']{8,}["']`),
			RuleID:      "SEC003",
			Severity:    "critical",
			Message:     "Hardcoded secret detected",
			Suggestion:  "Use environment variables or secret management",
			RequireSQL:  false,
			ExcludeFunc: []string{"// ", "/* ", "* ", "test", "Test", "example", "Example", "mock", "Mock"},
		},
		// XSS (JavaScript/TypeScript)
		{
			Pattern:     regexp.MustCompile(`(?i)(innerHTML|outerHTML|document\.write)\s*=`),
			RuleID:      "SEC004",
			Severity:    "error",
			Message:     "Potential XSS: unsafe DOM manipulation",
			Suggestion:  "Use textContent or sanitize HTML input",
			Languages:   []string{"javascript", "typescript"},
			RequireSQL:  false,
			ExcludeFunc: nil,
		},
		// Path Traversal
		{
			Pattern:     regexp.MustCompile(`(?i)(filepath\.Join|os\.path\.join|path\.join)\s*\([^)]*\+`),
			RuleID:      "SEC005",
			Severity:    "error",
			Message:     "Potential path traversal: unchecked path construction",
			Suggestion:  "Validate and sanitize file paths",
			RequireSQL:  false,
			ExcludeFunc: nil,
		},
		// Insecure Crypto - require word boundary to avoid false positives
		{
			Pattern:     regexp.MustCompile(`(?i)\b(md5|sha1|des|rc4)\b\s*[\.(]|\bcrypto/(md5|sha1|des|rc4)\b`),
			RuleID:      "SEC006",
			Severity:    "warning",
			Message:     "Weak cryptographic algorithm detected",
			Suggestion:  "Use SHA-256 or stronger algorithms",
			RequireSQL:  false,
			ExcludeFunc: nil,
		},
		// Unsafe Deserialization
		{
			Pattern:     regexp.MustCompile(`(?i)(pickle\.loads|yaml\.load\(|unserialize\(|eval\()`),
			RuleID:      "SEC007",
			Severity:    "critical",
			Message:     "Unsafe deserialization detected",
			Suggestion:  "Use safe_load or validate input before deserialization",
			RequireSQL:  false,
			ExcludeFunc: nil,
		},
		// Debug/Development Code - only flag in non-logging files
		{
			Pattern:     regexp.MustCompile(`(?i)(console\.log|debug\s*=\s*true)`),
			RuleID:      "SEC008",
			Severity:    "info",
			Message:     "Debug/logging code detected",
			Suggestion:  "Remove or disable in production",
			RequireSQL:  false,
			ExcludeFunc: []string{"logging", "logger", "log.go", "_test.go"},
		},
	}

	for lineNum, line := range lines {
		for _, sp := range securityPatterns {
			// Check language filter
			if len(sp.Languages) > 0 && !contains(sp.Languages, lang) {
				continue
			}

			if sp.Pattern.MatchString(line) {
				// Check exclusion patterns (safe contexts that should not be flagged)
				if len(sp.ExcludeFunc) > 0 {
					excluded := false
					for _, excl := range sp.ExcludeFunc {
						if strings.Contains(line, excl) || strings.Contains(filePath, excl) {
							excluded = true
							logging.ReviewerDebug("Security [%s]: excluded by %q at line %d", sp.RuleID, excl, lineNum+1)
							break
						}
					}
					if excluded {
						continue
					}
				}

				logging.ReviewerDebug("Security match [%s]: line %d - %s", sp.RuleID, lineNum+1, sp.Message)
				findings = append(findings, ReviewFinding{
					File:        filePath,
					Line:        lineNum + 1,
					Severity:    sp.Severity,
					Category:    "security",
					RuleID:      sp.RuleID,
					Message:     sp.Message,
					Suggestion:  sp.Suggestion,
					CodeSnippet: strings.TrimSpace(line),
				})
			}
		}
	}

	logging.ReviewerDebug("Security checks complete: %d findings", len(findings))
	return findings
}

// =============================================================================
// STYLE CHECKS
// =============================================================================

// checkStyle performs style and formatting checks.
func (r *ReviewerShard) checkStyle(filePath, content string) []ReviewFinding {
	logging.ReviewerDebug("Running style checks on: %s", filePath)
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	// Style patterns
	stylePatterns := []struct {
		Pattern    *regexp.Regexp
		RuleID     string
		Severity   string
		Message    string
		Suggestion string
		Languages  []string
	}{
		// Long lines
		{
			Pattern:    regexp.MustCompile(`.{121,}`),
			RuleID:     "STY001",
			Severity:   "info",
			Message:    "Line exceeds 120 characters",
			Suggestion: "Break into multiple lines",
		},
		// Trailing whitespace
		{
			Pattern:    regexp.MustCompile(`\s+$`),
			RuleID:     "STY002",
			Severity:   "info",
			Message:    "Trailing whitespace",
			Suggestion: "Remove trailing whitespace",
		},
		// TODO without issue reference
		{
			Pattern:    regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX)`),
			RuleID:     "STY003",
			Severity:   "info",
			Message:    "TODO/FIXME without issue reference",
			Suggestion: "Link to an issue tracker",
		},
		// Magic numbers
		{
			Pattern:    regexp.MustCompile(`[^0-9a-zA-Z_]\d{3,}[^0-9a-zA-Z_]`),
			RuleID:     "STY004",
			Severity:   "info",
			Message:    "Magic number detected",
			Suggestion: "Extract to a named constant",
		},
		// Deep nesting (6+ levels)
		{
			Pattern:    regexp.MustCompile(`^\t{6,}|^(    ){6,}|^ {24,}`),
			RuleID:     "STY005",
			Severity:   "warning",
			Message:    "Deep nesting detected (6+ levels)",
			Suggestion: "Consider refactoring to reduce nesting - extract helper functions or use early returns",
		},
		// Go: naked returns in long functions
		{
			Pattern:    regexp.MustCompile(`^\s*return\s*$`),
			RuleID:     "STY006",
			Severity:   "info",
			Message:    "Naked return statement",
			Suggestion: "Consider explicit returns for clarity",
			Languages:  []string{"go"},
		},
	}

	for lineNum, line := range lines {
		for _, sp := range stylePatterns {
			if len(sp.Languages) > 0 && !contains(sp.Languages, lang) {
				continue
			}

			if sp.Pattern.MatchString(line) {
				logging.ReviewerDebug("Style match [%s]: line %d - %s", sp.RuleID, lineNum+1, sp.Message)
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       lineNum + 1,
					Severity:   sp.Severity,
					Category:   "style",
					RuleID:     sp.RuleID,
					Message:    sp.Message,
					Suggestion: sp.Suggestion,
				})
			}
		}
	}

	logging.ReviewerDebug("Style checks complete: %d findings", len(findings))
	return findings
}

// =============================================================================
// BUG PATTERN CHECKS
// =============================================================================

// checkBugPatterns checks for common bug patterns.
func (r *ReviewerShard) checkBugPatterns(filePath, content string) []ReviewFinding {
	logging.ReviewerDebug("Running bug pattern checks on: %s", filePath)
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	bugPatterns := []struct {
		Pattern    *regexp.Regexp
		RuleID     string
		Severity   string
		Message    string
		Suggestion string
		Languages  []string
	}{
		// Go: ignoring errors
		{
			Pattern:    regexp.MustCompile(`\s+_\s*=\s*\w+\(`),
			RuleID:     "BUG001",
			Severity:   "warning",
			Message:    "Error potentially ignored",
			Suggestion: "Handle or explicitly log the error",
			Languages:  []string{"go"},
		},
		// Null/nil comparisons
		{
			Pattern:    regexp.MustCompile(`(?i)(==\s*nil|==\s*null|===\s*null)\s*\)`),
			RuleID:     "BUG002",
			Severity:   "info",
			Message:    "Explicit null/nil check",
			Suggestion: "Consider using optional chaining or guard clauses",
		},
		// Empty catch blocks
		{
			Pattern:    regexp.MustCompile(`catch\s*\([^)]*\)\s*\{\s*\}`),
			RuleID:     "BUG003",
			Severity:   "error",
			Message:    "Empty catch block - errors silently swallowed",
			Suggestion: "Log or handle the error",
			Languages:  []string{"javascript", "typescript", "java"},
		},
		// Go: defer in loop
		{
			Pattern:    regexp.MustCompile(`for\s.*\{[^}]*defer\s`),
			RuleID:     "BUG004",
			Severity:   "warning",
			Message:    "Defer inside loop - may cause resource leak",
			Suggestion: "Move defer outside loop or use explicit cleanup",
			Languages:  []string{"go"},
		},
		// Rust: unwrap in production code
		{
			Pattern:    regexp.MustCompile(`\.unwrap\(\)|\.expect\(`),
			RuleID:     "BUG005",
			Severity:   "warning",
			Message:    "Panic-inducing unwrap/expect in code",
			Suggestion: "Use proper error handling with ? or match",
			Languages:  []string{"rust"},
		},
	}

	for lineNum, line := range lines {
		for _, bp := range bugPatterns {
			if len(bp.Languages) > 0 && !contains(bp.Languages, lang) {
				continue
			}

			if bp.Pattern.MatchString(line) {
				logging.ReviewerDebug("Bug pattern match [%s]: line %d - %s", bp.RuleID, lineNum+1, bp.Message)
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       lineNum + 1,
					Severity:   bp.Severity,
					Category:   "bug",
					RuleID:     bp.RuleID,
					Message:    bp.Message,
					Suggestion: bp.Suggestion,
				})
			}
		}
	}

	logging.ReviewerDebug("Bug pattern checks complete: %d findings", len(findings))
	return findings
}

// =============================================================================
// CODE DOM SAFETY CHECKS
// =============================================================================

// checkCodeDOMSafety checks Code DOM predicates for safety concerns.
func (r *ReviewerShard) checkCodeDOMSafety(filePath string) []ReviewFinding {
	logging.ReviewerDebug("Running Code DOM safety checks on: %s", filePath)
	findings := make([]ReviewFinding, 0)

	if r.kernel == nil {
		logging.ReviewerDebug("Kernel not available, skipping Code DOM checks")
		return findings
	}

	// Check if file is generated code
	generatedResults, _ := r.kernel.Query("generated_code")
	for _, fact := range generatedResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[0].(string); ok && file == filePath {
				generator := "unknown"
				marker := ""
				if g, ok := fact.Args[1].(string); ok {
					generator = g
				}
				if m, ok := fact.Args[2].(string); ok {
					marker = m
				}
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       1,
					Severity:   "warning",
					Category:   "generated",
					RuleID:     "CDOM001",
					Message:    fmt.Sprintf("Generated code (%s) - changes will be lost on regeneration", generator),
					Suggestion: fmt.Sprintf("Modify the generator source instead. Marker: %s", marker),
				})
			}
		}
	}

	// Check for breaking change risk
	breakingResults, _ := r.kernel.Query("breaking_change_risk")
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			ref, _ := fact.Args[0].(string)
			level, _ := fact.Args[1].(string)
			reason, _ := fact.Args[2].(string)

			if strings.Contains(ref, filePath) {
				severity := "info"
				if level == "/critical" {
					severity = "critical"
				} else if level == "/high" {
					severity = "error"
				} else if level == "/medium" {
					severity = "warning"
				}

				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   severity,
					Category:   "breaking_change",
					RuleID:     "CDOM002",
					Message:    fmt.Sprintf("Breaking change risk: %s", reason),
					Suggestion: "Review downstream consumers and update tests",
				})
			}
		}
	}

	// Check for API client functions that need integration tests
	apiClientResults, _ := r.kernel.Query("api_client_function")
	for _, fact := range apiClientResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == filePath {
				funcName := "unknown"
				pattern := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if p, ok := fact.Args[2].(string); ok {
					pattern = p
				}
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   "info",
					Category:   "api",
					RuleID:     "CDOM003",
					Message:    fmt.Sprintf("API client function detected (%s): %s", pattern, funcName),
					Suggestion: "Ensure proper error handling, timeouts, and consider integration tests",
				})
			}
		}
	}

	// Check for API handler functions
	apiHandlerResults, _ := r.kernel.Query("api_handler_function")
	for _, fact := range apiHandlerResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == filePath {
				funcName := "unknown"
				framework := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if f, ok := fact.Args[2].(string); ok {
					framework = f
				}
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   "info",
					Category:   "api",
					RuleID:     "CDOM004",
					Message:    fmt.Sprintf("API handler detected (%s framework): %s", framework, funcName),
					Suggestion: "Validate inputs, handle errors appropriately, check authentication",
				})
			}
		}
	}

	// Check for mock update suggestions
	mockResults, _ := r.kernel.Query("suggest_update_mocks")
	for _, fact := range mockResults {
		if len(fact.Args) >= 1 {
			if file, ok := fact.Args[0].(string); ok && file == filePath {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   "warning",
					Category:   "testing",
					RuleID:     "CDOM005",
					Message:    "Signature change detected - mock files may need updating",
					Suggestion: "Run 'mockgen' or update mock implementations",
				})
			}
		}
	}

	// Check for CGo code
	cgoResults, _ := r.kernel.Query("cgo_code")
	for _, fact := range cgoResults {
		if len(fact.Args) >= 1 {
			if file, ok := fact.Args[0].(string); ok && file == filePath {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       1,
					Severity:   "warning",
					Category:   "cgo",
					RuleID:     "CDOM006",
					Message:    "CGo code detected - requires careful memory management review",
					Suggestion: "Verify proper memory allocation/deallocation and type conversions",
				})
			}
		}
	}

	return findings
}

// =============================================================================
// LEARNED PATTERNS CHECK
// =============================================================================

// checkLearnedPatterns checks against patterns learned through Autopoiesis.
func (r *ReviewerShard) checkLearnedPatterns(filePath, content string) []ReviewFinding {
	logging.ReviewerDebug("Running learned pattern checks on: %s", filePath)
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")

	r.mu.RLock()
	antiPatterns := r.learnedAntiPatterns
	r.mu.RUnlock()

	logging.ReviewerDebug("Checking against %d learned anti-patterns", len(antiPatterns))

	for pattern, reason := range antiPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		for lineNum, line := range lines {
			if re.MatchString(line) {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       lineNum + 1,
					Severity:   "warning",
					Category:   "learned",
					RuleID:     "LEARN001",
					Message:    fmt.Sprintf("Learned anti-pattern: %s", reason),
					Suggestion: "This pattern was flagged in previous reviews",
				})
			}
		}
	}

	return findings
}
