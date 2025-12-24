package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAtomYAMLFile_ListAndUnknownField(t *testing.T) {
	dir := t.TempDir()

	listPath := filepath.Join(dir, "list.yaml")
	listData := strings.Join([]string{
		"- id: /alpha",
		"  category: protocol",
		"  priority: 10",
		"  is_mandatory: true",
		"  content: \"hello\"",
		"- id: /beta",
		"  category: protocol",
		"  priority: 20",
		"  is_mandatory: false",
		"  content: \"world\"",
		"",
	}, "\n")
	if err := os.WriteFile(listPath, []byte(listData), 0644); err != nil {
		t.Fatalf("write list yaml: %v", err)
	}

	defs, issues := parseAtomYAMLFile(listPath)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}

	unknownPath := filepath.Join(dir, "unknown.yaml")
	unknownData := strings.Join([]string{
		"id: /gamma",
		"category: protocol",
		"priority: 5",
		"is_mandatory: true",
		"content: \"oops\"",
		"unknown_field: true",
		"",
	}, "\n")
	if err := os.WriteFile(unknownPath, []byte(unknownData), 0644); err != nil {
		t.Fatalf("write unknown yaml: %v", err)
	}

	defs, issues = parseAtomYAMLFile(unknownPath)
	if len(defs) != 0 {
		t.Fatalf("expected no defs for unknown field, got %d", len(defs))
	}
	if !hasIssue(issues, severityError, "YAML parse failed") {
		t.Fatalf("expected parse error issue, got %+v", issues)
	}
}

func TestValidateSelectorListAndWorldStates(t *testing.T) {
	opts := validationOptions{WarnNoncanonicalSelectors: true}
	issues := validateSelectorList("file.yaml", "/atom", "operational_modes", []string{"", "debug", "debug", "/release"}, selectorStyleSlashPref, opts)

	if !hasIssue(issues, severityError, "contains empty value") {
		t.Fatalf("expected empty value error, got %+v", issues)
	}
	if !hasIssue(issues, severityWarning, "contains duplicate value") {
		t.Fatalf("expected duplicate warning, got %+v", issues)
	}
	if !hasIssue(issues, severityWarning, "non-canonical") {
		t.Fatalf("expected non-canonical warning, got %+v", issues)
	}

	worldIssues := validateWorldStatesKnownSet("file.yaml", "/atom", []string{"/diagnostics", "/unknown"})
	if !hasIssue(worldIssues, severityWarning, "unknown value") {
		t.Fatalf("expected unknown world_states warning, got %+v", worldIssues)
	}
}

func TestValidateAtomDef_ContentFileAndRecommendedSelectors(t *testing.T) {
	dir := t.TempDir()
	contentPath := filepath.Join(dir, "content.txt")
	if err := os.WriteFile(contentPath, []byte("content body"), 0644); err != nil {
		t.Fatalf("write content file: %v", err)
	}

	priority := 10
	isMandatory := true
	def := atomDefinition{
		ID:          "/campaign_atom",
		Category:    "protocol",
		Priority:    &priority,
		IsMandatory: &isMandatory,
		ContentFile: "content.txt",
	}

	valid := map[string]struct{}{"protocol": {}}
	opts := validationOptions{CheckRecommendedSelectors: true}
	issues := validateAtomDef(filepath.Join(dir, "atom.yaml"), filepath.ToSlash("foo/campaign/atom.yaml"), def, valid, opts)

	if countSeverity(issues, severityError) != 0 {
		t.Fatalf("unexpected errors: %+v", issues)
	}
	if !hasIssue(issues, severityWarning, "missing recommended field") {
		t.Fatalf("expected recommended selector warning, got %+v", issues)
	}
}

func hasIssue(issues []issue, severity issueSeverity, contains string) bool {
	for _, it := range issues {
		if it.Severity == severity && strings.Contains(it.Message, contains) {
			return true
		}
	}
	return false
}

func countSeverity(issues []issue, severity issueSeverity) int {
	count := 0
	for _, it := range issues {
		if it.Severity == severity {
			count++
		}
	}
	return count
}
