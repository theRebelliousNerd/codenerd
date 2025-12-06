package shards

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomRuleValidation(t *testing.T) {
	reviewer := NewReviewerShard()

	tests := []struct {
		name      string
		rule      CustomRule
		wantError bool
	}{
		{
			name: "valid rule",
			rule: CustomRule{
				ID:         "TEST001",
				Category:   "security",
				Severity:   "critical",
				Pattern:    "eval\\(",
				Message:    "No eval allowed",
				Suggestion: "Use safer alternatives",
				Enabled:    true,
			},
			wantError: false,
		},
		{
			name: "missing ID",
			rule: CustomRule{
				Category: "security",
				Severity: "critical",
				Pattern:  "eval\\(",
				Message:  "No eval allowed",
				Enabled:  true,
			},
			wantError: true,
		},
		{
			name: "invalid severity",
			rule: CustomRule{
				ID:       "TEST002",
				Category: "security",
				Severity: "super-critical",
				Pattern:  "eval\\(",
				Message:  "No eval allowed",
				Enabled:  true,
			},
			wantError: true,
		},
		{
			name: "invalid category",
			rule: CustomRule{
				ID:       "TEST003",
				Category: "awesome",
				Severity: "critical",
				Pattern:  "eval\\(",
				Message:  "No eval allowed",
				Enabled:  true,
			},
			wantError: true,
		},
		{
			name: "invalid regex",
			rule: CustomRule{
				ID:       "TEST004",
				Category: "security",
				Severity: "critical",
				Pattern:  "([unclosed",
				Message:  "No eval allowed",
				Enabled:  true,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reviewer.AddCustomRule(tt.rule)
			if (err != nil) != tt.wantError {
				t.Errorf("AddCustomRule() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCustomRuleChecking(t *testing.T) {
	reviewer := NewReviewerShard()

	// Add test rule
	rule := CustomRule{
		ID:         "TEST001",
		Category:   "security",
		Severity:   "critical",
		Pattern:    "eval\\(",
		Message:    "eval() is forbidden",
		Suggestion: "Use JSON.parse() instead",
		Languages:  []string{"javascript"},
		Enabled:    true,
	}

	if err := reviewer.AddCustomRule(rule); err != nil {
		t.Fatalf("Failed to add custom rule: %v", err)
	}

	// Test file content
	testCode := `
function processData(input) {
    var result = eval(input); // Should trigger rule
    return result;
}
`

	findings := reviewer.checkCustomRules("test.js", testCode)

	if len(findings) == 0 {
		t.Error("Expected findings, got none")
	}

	if len(findings) > 0 {
		finding := findings[0]
		if finding.RuleID != "TEST001" {
			t.Errorf("Expected RuleID TEST001, got %s", finding.RuleID)
		}
		if finding.Severity != "critical" {
			t.Errorf("Expected severity critical, got %s", finding.Severity)
		}
		if finding.Category != "security" {
			t.Errorf("Expected category security, got %s", finding.Category)
		}
	}
}

func TestCustomRuleLanguageFilter(t *testing.T) {
	reviewer := NewReviewerShard()

	// Add JavaScript-only rule
	rule := CustomRule{
		ID:        "TEST002",
		Category:  "style",
		Severity:  "warning",
		Pattern:   "var ",
		Message:   "Use let/const instead of var",
		Languages: []string{"javascript"},
		Enabled:   true,
	}

	if err := reviewer.AddCustomRule(rule); err != nil {
		t.Fatalf("Failed to add custom rule: %v", err)
	}

	jsCode := "var x = 10;"
	goCode := "var x = 10"

	// Should match JavaScript
	jsFindings := reviewer.checkCustomRules("test.js", jsCode)
	if len(jsFindings) == 0 {
		t.Error("Expected finding for JavaScript file, got none")
	}

	// Should NOT match Go
	goFindings := reviewer.checkCustomRules("test.go", goCode)
	if len(goFindings) > 0 {
		t.Error("Expected no findings for Go file, got some")
	}
}

func TestLoadCustomRulesFromFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "test-rules.json")

	// Create test rules file
	rulesFile := CustomRulesFile{
		Version: "1.0",
		Rules: []CustomRule{
			{
				ID:         "FILE001",
				Category:   "security",
				Severity:   "error",
				Pattern:    "password",
				Message:    "Password detected",
				Suggestion: "Use secrets manager",
				Enabled:    true,
			},
			{
				ID:       "FILE002",
				Category: "style",
				Severity: "info",
				Pattern:  "TODO",
				Message:  "TODO comment",
				Enabled:  false, // Disabled
			},
		},
	}

	// Write to file
	data, err := json.MarshalIndent(rulesFile, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal rules: %v", err)
	}

	if err := os.WriteFile(rulesPath, data, 0644); err != nil {
		t.Fatalf("Failed to write rules file: %v", err)
	}

	// Load rules
	reviewer := NewReviewerShard()
	if err := reviewer.LoadCustomRules(rulesPath); err != nil {
		t.Fatalf("Failed to load custom rules: %v", err)
	}

	// Check loaded rules
	loadedRules := reviewer.GetCustomRules()
	if len(loadedRules) != 1 { // Only enabled rules
		t.Errorf("Expected 1 enabled rule, got %d", len(loadedRules))
	}

	if len(loadedRules) > 0 && loadedRules[0].ID != "FILE001" {
		t.Errorf("Expected rule FILE001, got %s", loadedRules[0].ID)
	}
}

func TestClearCustomRules(t *testing.T) {
	reviewer := NewReviewerShard()

	// Add some rules
	rule1 := CustomRule{
		ID:       "CLEAR001",
		Category: "security",
		Severity: "error",
		Pattern:  "test",
		Message:  "Test",
		Enabled:  true,
	}
	rule2 := CustomRule{
		ID:       "CLEAR002",
		Category: "style",
		Severity: "info",
		Pattern:  "test2",
		Message:  "Test2",
		Enabled:  true,
	}

	reviewer.AddCustomRule(rule1)
	reviewer.AddCustomRule(rule2)

	rules := reviewer.GetCustomRules()
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// Clear
	reviewer.ClearCustomRules()

	rules = reviewer.GetCustomRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules after clear, got %d", len(rules))
	}
}

func TestDuplicateRuleID(t *testing.T) {
	reviewer := NewReviewerShard()

	rule1 := CustomRule{
		ID:       "DUP001",
		Category: "security",
		Severity: "error",
		Pattern:  "test",
		Message:  "Test",
		Enabled:  true,
	}

	// Add first time - should succeed
	if err := reviewer.AddCustomRule(rule1); err != nil {
		t.Fatalf("First add should succeed: %v", err)
	}

	// Add duplicate - should fail
	if err := reviewer.AddCustomRule(rule1); err == nil {
		t.Error("Expected error for duplicate rule ID, got nil")
	}
}

func TestAnalyzeFileWithCustomRules(t *testing.T) {
	reviewer := NewReviewerShard()

	// Add custom rule
	rule := CustomRule{
		ID:         "ANALYZE001",
		Category:   "bug",
		Severity:   "warning",
		Pattern:    "FIXME",
		Message:    "FIXME comment found",
		Suggestion: "Create a ticket and fix this",
		Enabled:    true,
	}

	if err := reviewer.AddCustomRule(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Test code with FIXME
	testCode := `package main

func main() {
    // FIXME: This is broken
    println("hello")
}
`

	findings := reviewer.analyzeFile(context.Background(), "test.go", testCode)

	// Should find at least the custom rule violation
	found := false
	for _, f := range findings {
		if f.RuleID == "ANALYZE001" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find custom rule violation in analyzeFile results")
	}
}
