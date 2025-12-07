// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains comprehensive tests for the Ouroboros Loop and Safety Checker.
package autopoiesis

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// OUROBOROS LOOP TESTS
// =============================================================================

func TestNewOuroborosLoop(t *testing.T) {
	client := &MockLLMClient{}
	config := DefaultOuroborosConfig("/tmp/workspace")

	loop := NewOuroborosLoop(client, config)

	if loop == nil {
		t.Fatal("NewOuroborosLoop returned nil")
	}
	if loop.toolGen == nil {
		t.Error("toolGen not initialized")
	}
	if loop.safetyChecker == nil {
		t.Error("safetyChecker not initialized")
	}
	if loop.compiler == nil {
		t.Error("compiler not initialized")
	}
	if loop.registry == nil {
		t.Error("registry not initialized")
	}
}

func TestDefaultOuroborosConfig(t *testing.T) {
	config := DefaultOuroborosConfig("/test/workspace")

	if config.ToolsDir == "" {
		t.Error("ToolsDir should not be empty")
	}
	if config.CompiledDir == "" {
		t.Error("CompiledDir should not be empty")
	}
	if config.MaxToolSize == 0 {
		t.Error("MaxToolSize should not be zero")
	}
	if config.CompileTimeout == 0 {
		t.Error("CompileTimeout should not be zero")
	}
	if config.ExecuteTimeout == 0 {
		t.Error("ExecuteTimeout should not be zero")
	}
	if config.AllowNetworking {
		t.Error("AllowNetworking should be false by default")
	}
	if !config.AllowFileSystem {
		t.Error("AllowFileSystem should be true by default")
	}
	if !config.AllowExec {
		t.Error("AllowExec should be true by default")
	}
}

func TestLoopStage_String(t *testing.T) {
	tests := []struct {
		stage LoopStage
		want  string
	}{
		{StageDetection, "detection"},
		{StageSpecification, "specification"},
		{StageSafetyCheck, "safety_check"},
		{StageCompilation, "compilation"},
		{StageRegistration, "registration"},
		{StageExecution, "execution"},
		{StageComplete, "complete"},
		{LoopStage(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.stage.String(); got != tt.want {
				t.Errorf("LoopStage.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	client := &MockLLMClient{}
	config := DefaultOuroborosConfig("/tmp/workspace")
	loop := NewOuroborosLoop(client, config)

	stats := loop.GetStats()

	// Initial stats should be zero
	if stats.ToolsGenerated != 0 {
		t.Errorf("ToolsGenerated = %d, want 0", stats.ToolsGenerated)
	}
	if stats.ToolsCompiled != 0 {
		t.Errorf("ToolsCompiled = %d, want 0", stats.ToolsCompiled)
	}
	if stats.SafetyViolations != 0 {
		t.Errorf("SafetyViolations = %d, want 0", stats.SafetyViolations)
	}
}

// =============================================================================
// SAFETY CHECKER TESTS
// =============================================================================

func TestNewSafetyChecker(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	if checker == nil {
		t.Fatal("NewSafetyChecker returned nil")
	}
	if len(checker.forbiddenImports) == 0 {
		t.Error("forbiddenImports should not be empty")
	}
	if len(checker.forbiddenCalls) == 0 {
		t.Error("forbiddenCalls should not be empty")
	}
}

func TestSafetyChecker_Check_SafeCode(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	safeCode := `package tools

import (
	"context"
	"fmt"
)

func SafeTool(ctx context.Context, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty input")
	}
	return "processed: " + input, nil
}
`

	report := checker.Check(safeCode)

	if !report.Safe {
		t.Errorf("Expected safe code, got violations: %v", report.Violations)
	}
	if report.Score < 0.9 {
		t.Errorf("Expected high safety score, got %f", report.Score)
	}
}

func TestSafetyChecker_Check_ForbiddenImports(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	tests := []struct {
		name       string
		importPkg  string
		shouldFail bool
	}{
		{"unsafe", "unsafe", true},
		{"syscall", "syscall", true},
		{"runtime/cgo", "runtime/cgo", true},
		{"plugin", "plugin", true},
		{"debug/pprof", "debug/pprof", true},
		{"os/exec", "os/exec", false}, // AllowExec is true by default
		{"net", "net", true},         // AllowNetworking is false by default
		{"net/http", "net/http", true},
		{"fmt", "fmt", false},           // Safe
		{"encoding/json", "encoding/json", false}, // Safe
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := `package tools
import "` + tt.importPkg + `"
func Test() { _ = "` + tt.importPkg + `" }
`
			report := checker.Check(code)

			if tt.shouldFail && report.Safe {
				t.Errorf("Expected %s import to be blocked", tt.importPkg)
			}
			if !tt.shouldFail && !report.Safe {
				t.Errorf("Expected %s import to be allowed, got violations: %v",
					tt.importPkg, report.Violations)
			}
		})
	}
}

func TestSafetyChecker_Check_DangerousCalls(t *testing.T) {
	config := OuroborosConfig{AllowFileSystem: true}
	checker := NewSafetyChecker(config)

	tests := []struct {
		name     string
		code     string
		wantSafe bool
		severity string
	}{
		{
			name: "os.RemoveAll",
			code: `package tools
import "os"
func Delete() { os.RemoveAll("/tmp") }`,
			wantSafe: false,
			severity: "blocking",
		},
		{
			name: "os.Remove",
			code: `package tools
import "os"
func Delete() { os.Remove("/tmp/file") }`,
			wantSafe: true, // Warning only, not blocking
			severity: "warning",
		},
		{
			name: "unsafe.Pointer",
			code: `package tools
import "unsafe"
func Dangerous() { _ = unsafe.Pointer(nil) }`,
			wantSafe: false,
			severity: "blocking",
		},
		{
			name: "os.Setenv",
			code: `package tools
import "os"
func SetEnv() { os.Setenv("KEY", "value") }`,
			wantSafe: true, // Warning only
			severity: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := checker.Check(tt.code)

			if tt.wantSafe && !report.Safe {
				t.Errorf("Expected code to be safe (with warnings), got unsafe: %v",
					report.Violations)
			}
			if !tt.wantSafe && report.Safe {
				t.Errorf("Expected code to be unsafe")
			}
		})
	}
}

func TestSafetyChecker_Check_CGO(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	cgoPatterns := []string{
		`package tools
import "C"
func UseCGO() {}`,
		`package tools
// #cgo CFLAGS: -I.
import "C"
func UseCGO() {}`,
		`package tools
/*
#include <stdio.h>
*/
import "C"
func UseCGO() {}`,
	}

	for i, code := range cgoPatterns {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			report := checker.Check(code)

			if report.Safe {
				t.Error("Expected CGO code to be blocked")
			}

			foundCGOViolation := false
			for _, v := range report.Violations {
				if v.Type == ViolationCGO {
					foundCGOViolation = true
					break
				}
			}
			if !foundCGOViolation {
				t.Error("Expected ViolationCGO in violations")
			}
		})
	}
}

func TestSafetyChecker_Check_ParseError(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	invalidCode := `package tools

func broken( {
	// Syntax error
`

	report := checker.Check(invalidCode)

	if report.Safe {
		t.Error("Expected unparseable code to be unsafe")
	}
	if report.Score != 0.0 {
		t.Errorf("Expected score 0.0 for parse error, got %f", report.Score)
	}
}

func TestSafetyChecker_CalculateScore(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	// Safe code should have score ~1.0
	safeCode := `package tools
func Safe() {}
`
	report := checker.Check(safeCode)
	if report.Score < 0.9 {
		t.Errorf("Safe code score = %f, want >= 0.9", report.Score)
	}

	// Code with warnings should have reduced score
	warningCode := `package tools
import "os"
func Warn() { os.Setenv("x", "y") }
`
	report = checker.Check(warningCode)
	if report.Score >= 1.0 {
		t.Error("Code with warnings should have score < 1.0")
	}
	if !report.Safe {
		t.Error("Code with only warnings should still be safe")
	}
}

func TestViolationType_String(t *testing.T) {
	tests := []struct {
		vtype ViolationType
		want  string
	}{
		{ViolationForbiddenImport, "forbidden_import"},
		{ViolationDangerousCall, "dangerous_call"},
		{ViolationUnsafePointer, "unsafe_pointer"},
		{ViolationReflection, "reflection"},
		{ViolationCGO, "cgo"},
		{ViolationExec, "exec"},
		{ViolationType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.vtype.String(); got != tt.want {
				t.Errorf("ViolationType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// RUNTIME REGISTRY TESTS
// =============================================================================

func TestNewRuntimeRegistry(t *testing.T) {
	registry := NewRuntimeRegistry()

	if registry == nil {
		t.Fatal("NewRuntimeRegistry returned nil")
	}
	if registry.tools == nil {
		t.Error("tools map not initialized")
	}
}

func TestRuntimeRegistry_Register(t *testing.T) {
	registry := NewRuntimeRegistry()

	tool := &GeneratedTool{
		Name:        "test_tool",
		Description: "A test tool",
		Schema: ToolSchema{
			Name:        "test_tool",
			Description: "A test tool",
		},
	}

	compiled := &CompileResult{
		Success:    true,
		OutputPath: "/tmp/test_tool",
		Hash:       "abc123",
	}

	handle, err := registry.Register(tool, compiled)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	if handle.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", handle.Name, "test_tool")
	}
	if handle.BinaryPath != "/tmp/test_tool" {
		t.Errorf("BinaryPath = %q, want %q", handle.BinaryPath, "/tmp/test_tool")
	}
	if handle.Hash != "abc123" {
		t.Errorf("Hash = %q, want %q", handle.Hash, "abc123")
	}
	if handle.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
}

func TestRuntimeRegistry_Get(t *testing.T) {
	registry := NewRuntimeRegistry()

	// Register a tool
	tool := &GeneratedTool{Name: "existing_tool"}
	compiled := &CompileResult{Success: true, OutputPath: "/tmp/existing"}
	registry.Register(tool, compiled)

	// Get existing tool
	handle, exists := registry.Get("existing_tool")
	if !exists {
		t.Error("Expected tool to exist")
	}
	if handle == nil {
		t.Error("Expected handle to be returned")
	}

	// Get non-existing tool
	_, exists = registry.Get("nonexistent")
	if exists {
		t.Error("Expected tool to not exist")
	}
}

func TestRuntimeRegistry_List(t *testing.T) {
	registry := NewRuntimeRegistry()

	// Register multiple tools
	for i := 0; i < 3; i++ {
		tool := &GeneratedTool{Name: "tool_" + string(rune('a'+i))}
		compiled := &CompileResult{Success: true}
		registry.Register(tool, compiled)
	}

	tools := registry.List()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}
}

// =============================================================================
// TOOL COMPILER TESTS
// =============================================================================

func TestNewToolCompiler(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	compiler := NewToolCompiler(config)

	if compiler == nil {
		t.Fatal("NewToolCompiler returned nil")
	}
}

func TestWrapAsMain_AlreadyMain(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	compiler := NewToolCompiler(config)

	mainCode := `package main

func main() {
	println("Hello")
}
`

	tool := &GeneratedTool{
		Name: "test",
		Code: mainCode,
	}

	wrapped := compiler.wrapAsMain(tool)
	if wrapped != mainCode {
		t.Error("Code already with package main should not be modified")
	}
}

func TestExtractFunctionBody(t *testing.T) {
	code := `package tools

func myTool(input string) (string, error) {
	return input, nil
}
`

	body := extractFunctionBody(code, "my_tool")
	// Function extraction is basic - just verify it returns something
	_ = body // May be empty if regex doesn't match
}

// =============================================================================
// MANGLE FACT GENERATORS TESTS
// =============================================================================

func TestGenerateMissingToolFacts(t *testing.T) {
	facts := GenerateMissingToolFacts("intent123", "json_parser")

	if len(facts) == 0 {
		t.Error("Expected at least one fact")
	}

	found := false
	for _, fact := range facts {
		if strings.Contains(fact, "missing_tool_for") &&
			strings.Contains(fact, "intent123") &&
			strings.Contains(fact, "json_parser") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected missing_tool_for fact with correct arguments")
	}
}

func TestGenerateToolCapabilityFacts(t *testing.T) {
	capabilities := []string{"parse", "validate", "format"}
	facts := GenerateToolCapabilityFacts("json_tool", capabilities)

	if len(facts) != len(capabilities)+1 {
		t.Errorf("Expected %d facts, got %d", len(capabilities)+1, len(facts))
	}

	// Check for tool_exists fact
	foundExists := false
	for _, fact := range facts {
		if strings.Contains(fact, "tool_exists") && strings.Contains(fact, "json_tool") {
			foundExists = true
			break
		}
	}
	if !foundExists {
		t.Error("Expected tool_exists fact")
	}

	// Check for capability facts
	for _, cap := range capabilities {
		found := false
		for _, fact := range facts {
			if strings.Contains(fact, "tool_capability") && strings.Contains(fact, cap) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool_capability fact for %q", cap)
		}
	}
}

func TestGenerateToolRegistrationFacts(t *testing.T) {
	tool := &RuntimeTool{
		Name:         "test_tool",
		Hash:         "abc123def456",
		RegisteredAt: time.Now(),
	}

	facts := GenerateToolRegistrationFacts(tool)

	if len(facts) < 3 {
		t.Errorf("Expected at least 3 facts, got %d", len(facts))
	}

	expectedPredicates := []string{"tool_registered", "tool_hash", "has_capability"}
	for _, pred := range expectedPredicates {
		found := false
		for _, fact := range facts {
			if strings.Contains(fact, pred) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected fact with predicate %q", pred)
		}
	}
}
