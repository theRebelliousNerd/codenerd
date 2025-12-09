package reviewer

import (
	"testing"
)

func TestBuildSemanticQuery_GoFiles(t *testing.T) {
	// Test with typical Go file paths
	files := []string{
		"/project/main.go",
		"/project/handler.go",
	}

	query := buildSemanticQuery(files)

	if query == "" {
		t.Error("Expected non-empty query for Go files")
	}

	// Should contain "go golang" from extension mapping
	if len(query) < 10 {
		t.Errorf("Query seems too short: %s", query)
	}
}

func TestBuildSemanticQuery_MgFiles(t *testing.T) {
	files := []string{
		"/project/policy.mg",
	}

	query := buildSemanticQuery(files)

	// Should contain technology keywords for .mg
	if query == "" {
		t.Error("Expected non-empty query for .mg files")
	}
}

func TestBuildSemanticQuery_EmptyFiles(t *testing.T) {
	query := buildSemanticQuery([]string{})
	if query != "" {
		t.Errorf("Expected empty query for empty files, got: %s", query)
	}
}

func TestExtToTechnology(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go golang"},
		{".mg", "mangle datalog logic"},
		{".py", "python"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".tsx", "react typescript"},
		{".rs", "rust"},
		{".unknown", ""},
	}

	for _, test := range tests {
		result := extToTechnology(test.ext)
		if result != test.expected {
			t.Errorf("extToTechnology(%s) = %s, expected %s", test.ext, result, test.expected)
		}
	}
}

func TestExtractKeywords_GoPatterns(t *testing.T) {
	content := `package main

import "fmt"

func main() {
	type User struct {
		Name string
	}
	fmt.Println("hello")
}
`
	keywords := extractKeywords(content)

	// Should find "func" pattern
	foundFunc := false
	for _, k := range keywords {
		if k == "func" {
			foundFunc = true
		}
	}

	if !foundFunc {
		t.Errorf("Expected to find 'func' in keywords: %v", keywords)
	}
}

func TestExtractKeywords_ManglePatterns(t *testing.T) {
	content := `Decl test_fact(X, Y).
test_rule(X) :- test_fact(X, _).
`
	keywords := extractKeywords(content)

	// Should find Mangle patterns
	foundDecl := false
	foundRule := false
	for _, k := range keywords {
		if k == "Decl" {
			foundDecl = true
		}
		if k == ":-" {
			foundRule = true
		}
	}

	if !foundDecl {
		t.Errorf("Expected to find 'Decl' in keywords: %v", keywords)
	}
	if !foundRule {
		t.Errorf("Expected to find ':-' in keywords: %v", keywords)
	}
}

func TestFormatKnowledgeContext_Empty(t *testing.T) {
	result := FormatKnowledgeContext([]RetrievedKnowledge{})
	if result != "" {
		t.Errorf("Expected empty string for empty knowledge, got: %s", result)
	}
}

func TestFormatKnowledgeContext_WithKnowledge(t *testing.T) {
	knowledge := []RetrievedKnowledge{
		{
			Content:    "Always handle errors explicitly",
			Concept:    "best_practice",
			Source:     "vector_store",
			Confidence: 0.9,
		},
		{
			Content:    "Avoid global variables",
			Concept:    "anti_pattern",
			Source:     "knowledge_atoms",
			Confidence: 0.8,
		},
	}

	result := FormatKnowledgeContext(knowledge)

	if result == "" {
		t.Error("Expected non-empty formatted knowledge")
	}

	// Should contain the content
	if len(result) < 50 {
		t.Errorf("Formatted result seems too short: %s", result)
	}
}

func TestFormatKnowledgeContext_TruncatesLongContent(t *testing.T) {
	// Create knowledge with very long content
	longContent := make([]byte, 1000)
	for i := range longContent {
		longContent[i] = 'a'
	}

	knowledge := []RetrievedKnowledge{
		{
			Content:    string(longContent),
			Concept:    "general",
			Confidence: 0.5,
		},
	}

	result := FormatKnowledgeContext(knowledge)

	// Content should be truncated (500 chars + "...")
	if len(result) > 700 {
		t.Errorf("Expected truncated content, got length: %d", len(result))
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{100, 100, 100},
	}

	for _, test := range tests {
		result := min(test.a, test.b)
		if result != test.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", test.a, test.b, result, test.expected)
		}
	}
}
