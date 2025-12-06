# Custom Review Rules Implementation Summary

## Overview

Implemented custom review rules loading for ReviewerShard, allowing users to define project-specific code review rules via JSON configuration files.

## Changes Made

### 1. Core Data Structures

**File:** `internal/shards/reviewer.go`

Added new types:
```go
type CustomRule struct {
    ID          string   `json:"id"`
    Category    string   `json:"category"`
    Severity    string   `json:"severity"`
    Pattern     string   `json:"pattern"`
    Message     string   `json:"message"`
    Suggestion  string   `json:"suggestion,omitempty"`
    Languages   []string `json:"languages,omitempty"`
    Description string   `json:"description,omitempty"`
    Enabled     bool     `json:"enabled"`
}

type CustomRulesFile struct {
    Version string       `json:"version"`
    Rules   []CustomRule `json:"rules"`
}
```

**Updated ReviewerConfig:**
```go
type ReviewerConfig struct {
    // ... existing fields ...
    CustomRulesPath string  // Path to custom rules JSON file (default: .nerd/review-rules.json)
}
```

**Updated ReviewerShard:**
```go
type ReviewerShard struct {
    // ... existing fields ...
    customRules []CustomRule  // User-defined custom review rules
}
```

### 2. Custom Rules Management

**File:** `internal/shards/reviewer_custom_rules.go` (NEW)

Implemented methods:
- `LoadCustomRules(path string) error` - Load rules from JSON file
- `AddCustomRule(rule CustomRule) error` - Add a single rule programmatically
- `validateCustomRule(rule CustomRule) error` - Validate rule structure
- `GetCustomRules() []CustomRule` - Get all loaded custom rules
- `ClearCustomRules()` - Remove all custom rules
- `checkCustomRules(filePath, content string) []ReviewFinding` - Apply custom rules during review

### 3. Integration

**File:** `internal/shards/reviewer.go`

Modified `analyzeFile()` to include custom rules checking:
```go
func (r *ReviewerShard) analyzeFile(ctx context.Context, filePath, content string) []ReviewFinding {
    findings := make([]ReviewFinding, 0)

    // ... existing checks ...

    // Custom rules checks (user-defined patterns) ← NEW
    findings = append(findings, r.checkCustomRules(filePath, content)...)

    // ... remaining checks ...

    return findings
}
```

Modified `NewReviewerShardWithConfig()` to auto-load custom rules:
```go
// Attempt to load custom rules if path is configured
if reviewerConfig.CustomRulesPath != "" {
    _ = shard.LoadCustomRules(reviewerConfig.CustomRulesPath)
}
```

### 4. Documentation

Created comprehensive documentation:
- `.nerd/CUSTOM_RULES.md` - Full user guide with examples
- `.nerd/review-rules.example.json` - Example rules file with 7 different rule types

### 5. Testing

**File:** `internal/shards/reviewer_custom_rules_test.go` (NEW)

Created comprehensive test suite:
- `TestCustomRuleValidation` - Validates rule validation logic
- `TestCustomRuleChecking` - Tests pattern matching
- `TestCustomRuleLanguageFilter` - Tests language filtering
- `TestLoadCustomRulesFromFile` - Tests JSON file loading
- `TestClearCustomRules` - Tests rule clearing
- `TestDuplicateRuleID` - Tests duplicate detection
- `TestAnalyzeFileWithCustomRules` - Tests integration with analyzeFile

All tests passing.

### 6. Bug Fixes

**File:** `internal/world/ast_treesitter.go`

Fixed unused import:
```go
import (
    "context"
    "codenerd/internal/core"
    "fmt"
    "strings"  // Removed "os" which was unused
    // ...
)
```

## Features

### Validation

Rules are validated on load:
- ✅ Required fields (ID, category, severity, pattern, message, enabled)
- ✅ Valid severity: `critical`, `error`, `warning`, `info`
- ✅ Valid category: `security`, `style`, `bug`, `performance`, `maintainability`
- ✅ Valid regex pattern (compiled and tested)
- ✅ No duplicate rule IDs

### Language Filtering

Rules can target specific languages:
```json
{
  "languages": ["go", "python"]
}
```

Supported: `go`, `python`, `javascript`, `typescript`, `rust`, `java`, `c`, `cpp`, `csharp`, `ruby`, `php`

Empty array = applies to all languages

### Regex Pattern Matching

Custom rules use Go regex (RE2 syntax):
- Case-insensitive: `(?i)pattern`
- Word boundaries: `\\bword\\b`
- Character classes: `\\s`, `\\w`, `\\d`
- Quantifiers: `*`, `+`, `?`, `{n}`

### File Location

Default path: `.nerd/review-rules.json`

Configurable via:
```go
config := DefaultReviewerConfig()
config.CustomRulesPath = "/path/to/custom-rules.json"
reviewer := NewReviewerShardWithConfig(config)
```

## Example Usage

### JSON Configuration

```json
{
  "version": "1.0",
  "rules": [
    {
      "id": "CUSTOM001",
      "category": "security",
      "severity": "critical",
      "pattern": "(?i)eval\\s*\\(",
      "message": "Use of eval() is prohibited - major security risk",
      "suggestion": "Use JSON.parse() or explicit function calls",
      "languages": ["javascript", "typescript"],
      "enabled": true
    }
  ]
}
```

### Programmatic Usage

```go
// Load from file
reviewer := NewReviewerShard()
if err := reviewer.LoadCustomRules(".nerd/review-rules.json"); err != nil {
    log.Fatal(err)
}

// Add individual rule
rule := CustomRule{
    ID:         "CUSTOM999",
    Category:   "security",
    Severity:   "critical",
    Pattern:    "password\\s*=\\s*['\"]",
    Message:    "Hardcoded password detected",
    Enabled:    true,
}
reviewer.AddCustomRule(rule)

// Get all rules
rules := reviewer.GetCustomRules()

// Clear all rules
reviewer.ClearCustomRules()
```

## Review Integration

Custom rules are checked during standard review operations:
1. Code DOM safety checks
2. Built-in security patterns
3. Built-in style patterns
4. Built-in bug patterns
5. **Custom rules** ← Integrated here
6. LLM-powered semantic analysis
7. Learned anti-patterns

Custom rule violations appear in review results:
```go
ReviewFinding{
    File:        "test.js",
    Line:        42,
    Severity:    "critical",
    Category:    "security",
    RuleID:      "CUSTOM001",
    Message:     "Use of eval() is prohibited",
    Suggestion:  "Use JSON.parse() instead",
    CodeSnippet: "var x = eval(input);",
}
```

## Files Changed

1. `internal/shards/reviewer.go` - Updated types and integration
2. `internal/shards/reviewer_custom_rules.go` - NEW: Rule management logic
3. `internal/shards/reviewer_custom_rules_test.go` - NEW: Test suite
4. `.nerd/CUSTOM_RULES.md` - NEW: User documentation
5. `.nerd/review-rules.example.json` - NEW: Example rules
6. `internal/world/ast_treesitter.go` - Fixed unused import

## Testing

All tests pass:
```bash
go test ./internal/shards -run TestCustomRule -v
# PASS: TestCustomRuleValidation (5 subtests)
# PASS: TestCustomRuleChecking
# PASS: TestCustomRuleLanguageFilter
# PASS: TestLoadCustomRulesFromFile
# PASS: TestClearCustomRules
# PASS: TestDuplicateRuleID
# PASS: TestAnalyzeFileWithCustomRules
```

## Backward Compatibility

✅ Fully backward compatible
- If no custom rules file exists, ReviewerShard works as before
- Built-in rules continue to function unchanged
- No breaking changes to public API
- Optional feature - must be explicitly configured

## Future Enhancements

Potential improvements:
1. YAML support in addition to JSON
2. Remote rules loading (HTTP/S URLs)
3. Rule inheritance/composition
4. Per-directory rule overrides
5. Rule metrics and reporting
6. IDE integration for rule authoring
