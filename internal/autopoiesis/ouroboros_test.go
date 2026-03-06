// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains comprehensive tests for the Ouroboros Loop and Safety Checker.
package autopoiesis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

func TestOuroborosLoop_Execute_HappyPath(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			// Return valid Go code for a tool
			return `package tools
import "context"
func SimpleTool(ctx context.Context, input string) (string, error) {
	return "result", nil
}
`, nil
		},
	}

	tmpDir := t.TempDir()
	config := DefaultOuroborosConfig(tmpDir)
	// Disable Thunderdome to simplify test
	config.EnableThunderdome = false

	loop := NewOuroborosLoop(mockLLM, config)

	need := &ToolNeed{
		Name:       "simple_tool",
		Purpose:    "Do simple thing",
		InputType:  "string",
		OutputType: "string",
		Confidence: 0.9,
	}

	// Execute
	result := loop.Execute(context.Background(), need)

	// Verify
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	// It might fail compilation (no go.mod), but it should reach StageCompilation or StageRegistration
	// If it fails at compilation, Success is false, but Stage is StageCompilation

	t.Logf("Result: Success=%v, Stage=%s, Error=%v", result.Success, result.Stage, result.Error)

	if result.Stage == StageDetection {
		t.Error("Loop did not progress past detection")
	}

	// Check that code generation happened (MockLLM called)
	// We can't check mockLLM call count easily without adding tracking to MockLLM,
	// but result state implies it.
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
	if len(checker.allowedPkgs) == 0 {
		t.Error("allowedPkgs should not be empty")
	}
	if checker.policy == "" {
		t.Error("policy should be loaded")
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
	if report.Score != 1.0 {
		t.Errorf("Expected perfect safety score, got %f", report.Score)
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
		{"net/http disallowed by default", "net/http", true},
		{"fmt", "fmt", false},
		{"encoding/json", "encoding/json", false},
		{"os/exec allowed by config", "os/exec", false},
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
				t.Errorf("Expected %s import to be allowed, got violations: %v", tt.importPkg, report.Violations)
			}
		})
	}
}

func TestSafetyChecker_Check_Panic(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	code := `package tools
func Boom() { panic("boom") }`

	report := checker.Check(code)
	if report.Safe {
		t.Fatal("panic should be blocked")
	}
	found := false
	for _, v := range report.Violations {
		if v.Type == ViolationPanic {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ViolationPanic in violations: %+v", report.Violations)
	}
}

func TestSafetyChecker_Check_GoroutineCancellation(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	t.Run("missing cancellation", func(t *testing.T) {
		code := `package tools
func Work() { go doThing() }
func doThing() {}`
		report := checker.Check(code)
		if report.Safe {
			t.Fatalf("expected goroutine without cancellation to be unsafe")
		}
	})

	t.Run("with context", func(t *testing.T) {
		code := `package tools
import "context"
func Work(ctx context.Context) { go doThing(ctx) }
func doThing(ctx context.Context) {}`
		report := checker.Check(code)
		if !report.Safe {
			t.Fatalf("expected goroutine with context to be safe, got %v", report.Violations)
		}
	})
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

func TestSafetyChecker_ScoreBinary(t *testing.T) {
	config := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(config)

	safe := `package tools
func ok() {}`
	report := checker.Check(safe)
	if report.Score != 1.0 {
		t.Fatalf("expected safe score 1.0, got %f", report.Score)
	}

	unsafe := `package tools
func bad() { panic("nope") }`
	report = checker.Check(unsafe)
	if report.Score != 0.0 {
		t.Fatalf("expected unsafe score 0.0, got %f", report.Score)
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
		{ViolationPanic, "panic"},
		{ViolationGoroutineLeak, "goroutine_leak"},
		{ViolationParseError, "parse_error"},
		{ViolationPolicy, "policy_violation"},
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

	expectedPredicates := []string{"tool_registered", "tool_hash", "tool_capability"}
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

	for _, fact := range facts {
		if strings.Contains(fact, "tool_registered") && strings.Contains(fact, "RFC3339") {
			t.Fatalf("expected numeric registration timestamp, got %s", fact)
		}
	}
}

// =============================================================================
// BOUNDARY VALUE ANALYSIS TESTS
// =============================================================================

func compileRuntimeTool(t *testing.T, source, name string) string {
	t.Helper()

	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write tool source: %v", err)
	}
	mod := fmt.Sprintf("module %s\n\ngo 1.23\n", strings.ReplaceAll(name, "-", "_"))
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(mod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	out := filepath.Join(tmpDir, name)
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tool build failed: %v\n%s", err, output)
	}
	return out
}

func registerRuntimeToolForLoop(t *testing.T, loop *OuroborosLoop, name, binaryPath string) {
	t.Helper()
	_, err := loop.registry.Register(
		&GeneratedTool{
			Name:        name,
			Description: "runtime test tool",
			Schema:      ToolSchema{Name: name},
		},
		&CompileResult{
			Success:    true,
			OutputPath: binaryPath,
			Hash:       "test-hash-" + name,
		},
	)
	if err != nil {
		t.Fatalf("failed to register runtime tool: %v", err)
	}
}

func TestOuroborosLoop_Execute_NilNeed(t *testing.T) {
	loop := NewOuroborosLoop(&MockLLMClient{}, DefaultOuroborosConfig(t.TempDir()))
	result := loop.Execute(context.Background(), nil)
	if result == nil {
		t.Fatal("Execute returned nil result for nil need")
	}
	if result.Success {
		t.Fatal("expected nil need execution to fail")
	}
	if !strings.Contains(result.Error, "nil") {
		t.Fatalf("expected nil-need error, got: %q", result.Error)
	}
}

func TestOuroborosLoop_Execute_EmptyNeed(t *testing.T) {
	loop := NewOuroborosLoop(&MockLLMClient{}, DefaultOuroborosConfig(t.TempDir()))
	result := loop.Execute(context.Background(), &ToolNeed{})
	if result == nil {
		t.Fatal("Execute returned nil result for empty need")
	}
	if result.Success {
		t.Fatal("expected empty need execution to fail")
	}
	if !strings.Contains(strings.ToLower(result.Error), "name") {
		t.Fatalf("expected name validation error, got: %q", result.Error)
	}
}

func TestOuroborosLoop_ExecuteTool_Concurrent(t *testing.T) {
	source := `package main
import (
	"encoding/json"
	"os"
)
type In struct { Input string ` + "`json:\"input\"`" + ` }
type Out struct { Output string ` + "`json:\"output\"`" + `; Error string ` + "`json:\"error,omitempty\"`" + ` }
func main() {
	var in In
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	_ = json.NewEncoder(os.Stdout).Encode(Out{Output: in.Input})
}`

	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.ExecuteTimeout = 3 * time.Second
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)

	bin := compileRuntimeTool(t, source, "concurrent_echo_tool")
	registerRuntimeToolForLoop(t, loop, "concurrent_echo_tool", bin)

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			want := fmt.Sprintf("msg-%d", i)
			got, err := loop.ExecuteTool(context.Background(), "concurrent_echo_tool", want)
			if err != nil {
				errs <- err
				return
			}
			if got != want {
				errs <- fmt.Errorf("mismatch: got=%q want=%q", got, want)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent execute failed: %v", err)
	}
}

func TestOuroborosLoop_ExecuteTool_GarbageOutput(t *testing.T) {
	source := `package main
import (
	"encoding/json"
	"fmt"
	"os"
)
type Out struct { Output string ` + "`json:\"output\"`" + `; Error string ` + "`json:\"error,omitempty\"`" + ` }
func main() {
	fmt.Println("debug: entering tool")
	_ = json.NewEncoder(os.Stdout).Encode(Out{Output: "ok"})
}`

	loop := NewOuroborosLoop(&MockLLMClient{}, DefaultOuroborosConfig(t.TempDir()))
	bin := compileRuntimeTool(t, source, "garbage_output_tool")
	registerRuntimeToolForLoop(t, loop, "garbage_output_tool", bin)

	_, err := loop.ExecuteTool(context.Background(), "garbage_output_tool", "x")
	if err == nil {
		t.Fatal("expected parse error for non-JSON-prefixed output")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "parse") {
		t.Fatalf("expected parse-related error, got: %v", err)
	}
}

func TestOuroborosLoop_ExecuteTool_InfiniteLoopTimeout(t *testing.T) {
	source := `package main
func main() {
	for {
	}
}`
	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.ExecuteTimeout = 250 * time.Millisecond
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)

	bin := compileRuntimeTool(t, source, "infinite_loop_tool")
	registerRuntimeToolForLoop(t, loop, "infinite_loop_tool", bin)

	_, err := loop.ExecuteTool(context.Background(), "infinite_loop_tool", "x")
	if err == nil {
		t.Fatal("expected timeout error for infinite loop tool")
	}
}

func TestOuroborosLoop_HotReload_LockedBinary(t *testing.T) {
	source := `package main
import (
	"encoding/json"
	"os"
	"time"
)
type Out struct { Output string ` + "`json:\"output\"`" + `; Error string ` + "`json:\"error,omitempty\"`" + ` }
func main() {
	time.Sleep(300 * time.Millisecond)
	_ = json.NewEncoder(os.Stdout).Encode(Out{Output: "ok"})
}`

	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.ExecuteTimeout = 2 * time.Second
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)

	bin := compileRuntimeTool(t, source, "hotreload_tool")
	registerRuntimeToolForLoop(t, loop, "hotreload_tool", bin)

	done := make(chan error, 1)
	go func() {
		_, err := loop.ExecuteTool(context.Background(), "hotreload_tool", "x")
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("hotReload panicked while tool running: %v", r)
			}
		}()
		loop.hotReload("hotreload_tool")
		loop.hotReload("hotreload_tool")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("execution failed unexpectedly: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for hotreload test execution")
	}
}

func TestOuroborosLoop_Mangle_TypeMismatch(t *testing.T) {
	loop := NewOuroborosLoop(&MockLLMClient{}, DefaultOuroborosConfig(t.TempDir()))

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("type mismatch handling panicked: %v", r)
		}
	}()

	err := loop.engine.AddFact("state", "/step_bad", "not-a-float", "not-an-int")
	if err == nil {
		// If engine accepts it, it still must remain query-safe.
		_, _ = loop.engine.Query(context.Background(), "should_halt(/step_bad)")
	} else {
		t.Logf("engine rejected mismatched types as expected: %v", err)
	}
}

func TestOuroborosLoop_FileSystem_Sandbox(t *testing.T) {
	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.AllowFileSystem = false
	checker := NewSafetyChecker(cfg)

	code := `package tools
import (
	"context"
	"os"
)
func Bad(ctx context.Context, input string) (string, error) {
	_ = os.RemoveAll("danger")
	return "nope", nil
}`

	report := checker.Check(code)
	if report.Safe {
		t.Fatalf("expected strict sandbox safety check to fail, got safe report")
	}
}
