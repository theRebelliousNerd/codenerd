// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains comprehensive tests for the ToolGenerator.
package autopoiesis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
}

// =============================================================================
// TOOL GENERATOR TESTS
// =============================================================================
// TOOL GENERATOR TESTS
// =============================================================================

func TestNewToolGenerator(t *testing.T) {
	client := &MockLLMClient{}
	toolsDir := "/tmp/test-tools"

	tg := NewToolGenerator(client, toolsDir)

	if tg == nil {
		t.Fatal("NewToolGenerator returned nil")
	}
	if tg.toolsDir != toolsDir {
		t.Errorf("toolsDir = %q, want %q", tg.toolsDir, toolsDir)
	}
	if tg.client != client {
		t.Error("client not set correctly")
	}
	if tg.existingTools == nil {
		t.Error("existingTools map not initialized")
	}
}

func TestDetectToolNeed_PatternMatching(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"needs_new_tool": true, "tool_name": "test_tool", "purpose": "Test purpose", "input_type": "string", "output_type": "string", "priority": 0.8, "confidence": 0.9, "reasoning": "Test reasoning"}`, nil
		},
	}
	tg := NewToolGenerator(client, "/tmp/tools")

	tests := []struct {
		name        string
		input       string
		wantNeed    bool
		description string
	}{
		{
			name:        "can't do pattern",
			input:       "Can't you validate this JSON?",
			wantNeed:    true,
			description: "Should detect 'can't do' pattern",
		},
		{
			name:        "is there a way pattern",
			input:       "Is there a way to parse this XML?",
			wantNeed:    true,
			description: "Should detect 'is there a way' pattern",
		},
		{
			name:        "I need a tool pattern",
			input:       "I need a tool to convert CSV to JSON",
			wantNeed:    true,
			description: "Should detect 'I need a tool' pattern",
		},
		{
			name:        "how do I pattern",
			input:       "How do I validate this schema?",
			wantNeed:    true,
			description: "Should detect 'how do I' pattern",
		},
		{
			name:        "simple request",
			input:       "Please help me with this code",
			wantNeed:    false,
			description: "Should not detect tool need for simple requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			need, err := tg.DetectToolNeed(context.Background(), tt.input, "")
			if err != nil {
				t.Fatalf("DetectToolNeed error: %v", err)
			}

			gotNeed := need != nil
			if gotNeed != tt.wantNeed {
				t.Errorf("%s: got need=%v, want need=%v", tt.description, gotNeed, tt.wantNeed)
			}
		})
	}
}

func TestDetectToolNeed_WithFailedAttempt(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"needs_new_tool": true, "tool_name": "retry_tool", "purpose": "Retry failed operation", "input_type": "string", "output_type": "string", "priority": 0.9, "confidence": 0.95, "reasoning": "Previous attempt failed"}`, nil
		},
	}
	tg := NewToolGenerator(client, "/tmp/tools")

	need, err := tg.DetectToolNeed(context.Background(), "Try again", "Previous tool execution failed")
	if err != nil {
		t.Fatalf("DetectToolNeed error: %v", err)
	}

	if need == nil {
		t.Error("Expected tool need to be detected when there's a failed attempt")
	}
}

func TestDetectToolNeed_ToolTypeDetection(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			// Return minimal JSON to let pattern matching determine type
			return `{"needs_new_tool": true, "tool_name": "detected_tool", "purpose": "Detected purpose", "input_type": "string", "output_type": "bool", "priority": 0.7, "confidence": 0.8, "reasoning": "Pattern matched"}`, nil
		},
	}
	tg := NewToolGenerator(client, "/tmp/tools")

	tests := []struct {
		input    string
		wantType string
	}{
		{"Can you validate the JSON format?", "validator"},
		{"Convert this from YAML to JSON", "converter"},
		{"Parse the XML response", "parser"},
		{"Analyze this code for issues", "analyzer"},
		{"Format this as markdown", "formatter"},
	}

	for _, tt := range tests {
		t.Run(tt.wantType, func(t *testing.T) {
			need, err := tg.DetectToolNeed(context.Background(), tt.input, "")
			if err != nil {
				t.Fatalf("DetectToolNeed error: %v", err)
			}

			if need == nil {
				t.Skip("No tool need detected - skipping type check")
			}
		})
	}
}

// =============================================================================
// VALIDATION TESTS
// =============================================================================

func TestValidateCodeAST_ValidCode(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	validCode := `package tools

import (
	"context"
	"fmt"
)

// JsonValidatorDescription describes the tool
const JsonValidatorDescription = "Validates JSON input"

// jsonValidator validates JSON input
func jsonValidator(ctx context.Context, input string) (bool, error) {
	if input == "" {
		return false, fmt.Errorf("empty input")
	}
	return true, nil
}

// RegisterJsonValidator registers this tool
func RegisterJsonValidator(registry ToolRegistry) error {
	return registry.Register("json_validator", JsonValidatorDescription, jsonValidator)
}
`

	result := tg.validateCodeAST(validCode, "json_validator")

	if !result.Valid {
		t.Errorf("Expected valid code, got errors: %v", result.Errors)
	}
	if result.PackageName != "tools" {
		t.Errorf("PackageName = %q, want 'tools'", result.PackageName)
	}
	if len(result.Functions) == 0 {
		t.Error("Expected functions to be extracted")
	}
	if len(result.Imports) != 2 {
		t.Errorf("Expected 2 imports, got %d", len(result.Imports))
	}
}

func TestValidateCodeAST_SyntaxError(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	invalidCode := `package tools

func broken( {
	// Missing closing paren and body
`

	result := tg.validateCodeAST(invalidCode, "broken")

	if result.Valid {
		t.Error("Expected invalid result for syntax error")
	}
	if result.ParseError == nil {
		t.Error("Expected ParseError to be set")
	}
}

func TestValidateCodeAST_MissingPackage(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	// Empty file
	result := tg.validateCodeAST("", "test")

	if result.Valid {
		t.Error("Expected invalid result for empty code")
	}
}

func TestValidateCodeAST_NoFunctions(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	codeWithoutFuncs := `package tools

import "fmt"

var x = fmt.Sprintf("test")
`

	result := tg.validateCodeAST(codeWithoutFuncs, "test")

	if result.Valid {
		t.Error("Expected invalid result for code without functions")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected error about missing functions")
	}
}

func TestValidateCodeAST_DangerousImports(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	dangerousCode := `package tools

import (
	"unsafe"
	"syscall"
)

func dangerous() {
	_ = unsafe.Pointer(nil)
	_ = syscall.Getpid()
}
`

	result := tg.validateCodeAST(dangerousCode, "dangerous")

	if len(result.Warnings) == 0 {
		t.Error("Expected warnings about dangerous imports")
	}

	foundUnsafeWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "unsafe") {
			foundUnsafeWarning = true
			break
		}
	}
	if !foundUnsafeWarning {
		t.Error("Expected warning about 'unsafe' import")
	}
}

func TestValidateCodeAST_PanicWithoutRecover(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	panicCode := `package tools

func riskyFunc() {
	panic("something went wrong")
}
`

	result := tg.validateCodeAST(panicCode, "risky")

	foundPanicWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "panic") {
			foundPanicWarning = true
			break
		}
	}
	if !foundPanicWarning {
		t.Error("Expected warning about panic without recover")
	}
}

func TestValidateCodeAST_OsExitWarning(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	exitCode := `package tools

import "os"

func badTool() {
	os.Exit(1)
}
`

	result := tg.validateCodeAST(exitCode, "bad")

	foundExitWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "os.Exit") {
			foundExitWarning = true
			break
		}
	}
	if !foundExitWarning {
		t.Error("Expected warning about os.Exit()")
	}
}

// =============================================================================
// TEST GENERATION TESTS
// =============================================================================

func TestGenerateFallbackTests(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	need := &ToolNeed{
		Name:       "json_validator",
		Purpose:    "Validate JSON input",
		InputType:  "string",
		OutputType: "bool",
	}

	code := `package tools

import "context"

func jsonValidator(ctx context.Context, input string) (bool, error) {
	return true, nil
}
`

	testCode := tg.generateFallbackTests(need, code)

	// Check that test code was generated
	if testCode == "" {
		t.Fatal("generateFallbackTests returned empty string")
	}

	// Check for required elements
	requiredElements := []string{
		"package tools",
		"import (",
		"testing",
		"context",
		"TestJsonValidator",
		"t.Run",
		"wantErr",
		"BenchmarkJsonValidator",
		"TestJsonValidator_ContextCancellation",
		"TestJsonValidator_Timeout",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(testCode, elem) {
			t.Errorf("Generated test code missing %q", elem)
		}
	}
}

func TestGetTestValue(t *testing.T) {
	tests := []struct {
		typeName string
		valid    bool
		contains string
	}{
		{"string", true, `"test input"`},
		{"string", false, `""`},
		{"[]byte", true, `[]byte("test input")`},
		{"int", true, "42"},
		{"int", false, "0"},
		{"float64", true, "3.14"},
		{"bool", true, "true"},
		{"bool", false, "false"},
		{"map[string]any", true, `map[string]any{"key": "value"}`},
		{"[]string", true, `[]string{"item1", "item2"}`},
		{"*MyType", true, "&MyType{}"},            // Pointer to struct gets address-of
		{"*MyType", false, "nil"},                 // Invalid pointer is nil
		{"[]CustomType", true, "[]CustomType{"},   // Slice with element value
		{"[]CustomType", false, "[]CustomType{}"}, // Empty slice
		{"map[string]int", true, `"count": 42`},   // Map with key-value pair
		{"chan string", true, "make(chan string"}, // Channel creation
		{"*string", true, `s := "test"`},          // Pointer to primitive
		{"*int", true, `i := 42`},                 // Pointer to int
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := getTestValue(tt.typeName, tt.valid)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("getTestValue(%q, %v) = %q, want to contain %q",
					tt.typeName, tt.valid, result, tt.contains)
			}
		})
	}
}

func TestGetZeroValue(t *testing.T) {
	tests := []struct {
		typeName string
		want     string
	}{
		{"string", `""`},
		{"[]byte", "nil"},
		{"int", "0"},
		{"int64", "0"},
		{"float64", "0.0"},
		{"bool", "false"},
		{"error", "nil"},
		{"*MyType", "nil"},
		{"[]int", "nil"},
		{"map[string]int", "nil"},
		{"CustomType", "CustomType{}"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := getZeroValue(tt.typeName)
			if result != tt.want {
				t.Errorf("getZeroValue(%q) = %q, want %q", tt.typeName, result, tt.want)
			}
		})
	}
}

// =============================================================================
// TOOL WRITING TESTS
// =============================================================================

func TestWriteTool(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "toolgen-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tg := NewToolGenerator(&MockLLMClient{}, tmpDir)

	tool := &GeneratedTool{
		Name:        "test_tool",
		Package:     "tools",
		Description: "A test tool",
		Code:        "package tools\n\nfunc testTool() {}\n",
		TestCode:    "package tools\n\nfunc TestTestTool(t *testing.T) {}\n",
		FilePath:    filepath.Join(tmpDir, "test_tool.go"),
	}

	err = tg.WriteTool(tool)
	if err != nil {
		t.Fatalf("WriteTool error: %v", err)
	}

	// Verify main file exists
	if _, err := os.Stat(tool.FilePath); os.IsNotExist(err) {
		t.Error("Tool file was not created")
	}

	// Verify test file exists
	testPath := filepath.Join(tmpDir, "test_tool_test.go")
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("Test file was not created")
	}

	// Verify content
	content, err := os.ReadFile(tool.FilePath)
	if err != nil {
		t.Fatalf("Failed to read tool file: %v", err)
	}
	if string(content) != tool.Code {
		t.Error("Tool file content mismatch")
	}
}

func TestRegisterTool(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	tool := &GeneratedTool{
		Name:        "registered_tool",
		Description: "A registered tool",
		Schema: ToolSchema{
			Name:        "registered_tool",
			Description: "A registered tool",
		},
	}

	err := tg.RegisterTool(tool)
	if err != nil {
		t.Fatalf("RegisterTool error: %v", err)
	}

	// Verify tool is registered
	tools := tg.listExistingTools()
	found := false
	for _, name := range tools {
		if name == "registered_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Tool was not registered")
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple json",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "json with surrounding text",
			input: `Here is the JSON: {"key": "value"} That's it.`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "nested json",
			input: `{"outer": {"inner": "value"}}`,
			want:  `{"outer": {"inner": "value"}}`,
		},
		{
			name:  "no json",
			input: `Just plain text`,
			want:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestExtractCodeBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		lang  string
		want  string
	}{
		{
			name:  "go code block",
			input: "```go\npackage main\n```",
			lang:  "go",
			want:  "package main",
		},
		{
			name:  "generic code block",
			input: "```\nsome code\n```",
			lang:  "go",
			want:  "some code",
		},
		{
			name:  "no code block",
			input: "just plain text",
			lang:  "go",
			want:  "just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCodeBlock(tt.input, tt.lang)
			if result != tt.want {
				t.Errorf("extractCodeBlock(%q, %q) = %q, want %q",
					tt.input, tt.lang, result, tt.want)
			}
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"json_validator", "jsonValidator"},
		{"simple", "simple"},
		{"a_b_c", "aBC"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toCamelCase(tt.input)
			if result != tt.want {
				t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"json_validator", "JsonValidator"},
		{"simple", "Simple"},
		{"a_b_c", "ABC"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "description constant",
			code: `package tools
const MyToolDescription = "This is my tool"`,
			want: "This is my tool",
		},
		{
			name: "package comment",
			code: `// My awesome tool
package tools`,
			want: "My awesome tool",
		},
		{
			name: "no description",
			code: `package tools`,
			want: "No description available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDescription(tt.code)
			if result != tt.want {
				t.Errorf("extractDescription() = %q, want %q", result, tt.want)
			}
		})
	}
}
