// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file implements the Ouroboros Loop - the self-eating serpent of tool generation.
//
// The Ouroboros Loop:
// Detection → Specification → Safety Check → Compile → Register → Execute
//
// This is core autopoiesis - the system creating new capabilities at runtime.
package autopoiesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// OUROBOROS LOOP - THE SELF-EATING SERPENT
// =============================================================================
// The Ouroboros Loop enables codeNERD to generate new tools at runtime.
// Named after the ancient symbol of a serpent eating its own tail,
// representing infinite self-creation and renewal.

// OuroborosLoop orchestrates the full tool self-generation cycle
type OuroborosLoop struct {
	mu sync.RWMutex

	toolGen      *ToolGenerator
	safetyChecker *SafetyChecker
	compiler     *ToolCompiler
	registry     *RuntimeRegistry

	config       OuroborosConfig
	stats        OuroborosStats
}

// OuroborosConfig configures the Ouroboros Loop
type OuroborosConfig struct {
	ToolsDir        string        // Directory for generated tools
	CompiledDir     string        // Directory for compiled tools
	MaxToolSize     int64         // Maximum tool source size in bytes
	CompileTimeout  time.Duration // Timeout for compilation
	ExecuteTimeout  time.Duration // Timeout for tool execution
	AllowNetworking bool          // Whether tools can use networking
	AllowFileSystem bool          // Whether tools can access filesystem
	AllowExec       bool          // Whether tools can execute commands
	TargetOS        string        // Target operating system (GOOS)
	TargetArch      string        // Target architecture (GOARCH)
	WorkspaceRoot   string        // Absolute path to the main codeNERD workspace root
}

// DefaultOuroborosConfig returns safe default configuration
func DefaultOuroborosConfig(workspaceRoot string) OuroborosConfig {
	return OuroborosConfig{
		ToolsDir:        filepath.Join(workspaceRoot, ".nerd", "tools"),
		CompiledDir:     filepath.Join(workspaceRoot, ".nerd", "tools", ".compiled"),
		MaxToolSize:     100 * 1024, // 100KB max
		CompileTimeout:  30 * time.Second,
		ExecuteTimeout:  60 * time.Second,
		AllowNetworking: false,
		AllowFileSystem: true, // Read-only by default
		AllowExec:       false,
		TargetOS:        os.Getenv("GOOS"),
		TargetArch:      os.Getenv("GOARCH"),
		WorkspaceRoot:   workspaceRoot,
	}
}

// OuroborosStats tracks loop statistics
type OuroborosStats struct {
	ToolsGenerated   int
	ToolsCompiled    int
	ToolsRejected    int
	SafetyViolations int
	ExecutionCount   int
	LastGeneration   time.Time
}

// NewOuroborosLoop creates a new Ouroboros Loop instance
func NewOuroborosLoop(client LLMClient, config OuroborosConfig) *OuroborosLoop {
	// Set defaults for OS/Arch if missing
	if config.TargetOS == "" {
		config.TargetOS = "windows" // Default to user's OS environment assumption or runtime
		if os.Getenv("GOOS") != "" {
			config.TargetOS = os.Getenv("GOOS")
		}
	}
	if config.TargetArch == "" {
		config.TargetArch = "amd64"
		if os.Getenv("GOARCH") != "" {
			config.TargetArch = os.Getenv("GOARCH")
		}
	}

	loop := &OuroborosLoop{
		toolGen:       NewToolGenerator(client, config.ToolsDir),
		safetyChecker: NewSafetyChecker(config),
		compiler:      NewToolCompiler(config),
		registry:      NewRuntimeRegistry(),
		config:        config,
	}

	// Restore registry from disk
	loop.registry.Restore(config.ToolsDir, config.CompiledDir)

	return loop
}

// =============================================================================
// THE LOOP STAGES
// =============================================================================

// LoopResult contains the result of a complete Ouroboros Loop execution
type LoopResult struct {
	Success      bool
	ToolName     string
	Stage        LoopStage
	Error        error
	SafetyReport *SafetyReport
	CompileResult *CompileResult
	ToolHandle   *RuntimeTool
	Duration     time.Duration
}

// LoopStage identifies where in the loop we are
type LoopStage int

const (
	StageDetection LoopStage = iota
	StageSpecification
	StageSafetyCheck
	StageCompilation
	StageRegistration
	StageExecution
	StageComplete
)

func (s LoopStage) String() string {
	switch s {
	case StageDetection:
		return "detection"
	case StageSpecification:
		return "specification"
	case StageSafetyCheck:
		return "safety_check"
	case StageCompilation:
		return "compilation"
	case StageRegistration:
		return "registration"
	case StageExecution:
		return "execution"
	case StageComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// Execute runs the complete Ouroboros Loop for a detected tool need
func (o *OuroborosLoop) Execute(ctx context.Context, need *ToolNeed) *LoopResult {
	start := time.Now()
	result := &LoopResult{
		ToolName: need.Name,
		Stage:    StageDetection,
	}

	// Stage 1: Detection (already done - we have the need)
	result.Stage = StageSpecification

	// Stage 2: Specification - Generate the tool
	tool, err := o.toolGen.GenerateTool(ctx, need)
	if err != nil {
		result.Error = fmt.Errorf("specification failed: %w", err)
		return result
	}
	result.Stage = StageSafetyCheck

	// Stage 3: Safety Check - Verify the generated code is safe
	safetyReport := o.safetyChecker.Check(tool.Code)
	result.SafetyReport = safetyReport

	if !safetyReport.Safe {
		o.mu.Lock()
		o.stats.SafetyViolations++
		o.stats.ToolsRejected++
		o.mu.Unlock()

		result.Error = fmt.Errorf("safety check failed: %v", safetyReport.Violations)
		return result
	}
	result.Stage = StageCompilation

	// Stage 4: Compilation - Write and compile the tool
	if err := o.toolGen.WriteTool(tool); err != nil {
		result.Error = fmt.Errorf("write failed: %w", err)
		return result
	}

	compileResult, err := o.compiler.Compile(ctx, tool)
	result.CompileResult = compileResult
	if err != nil {
		result.Error = fmt.Errorf("compilation failed: %w", err)
		return result
	}
	result.Stage = StageRegistration

	// Stage 5: Registration - Register the tool for runtime use
	handle, err := o.registry.Register(tool, compileResult)
	if err != nil {
		result.Error = fmt.Errorf("registration failed: %w", err)
		return result
	}
	result.ToolHandle = handle
	result.Stage = StageComplete

	// Update stats
	o.mu.Lock()
	o.stats.ToolsGenerated++
	o.stats.ToolsCompiled++
	o.stats.LastGeneration = time.Now()
	o.mu.Unlock()

	result.Success = true
	result.Duration = time.Since(start)
	return result
}

// ExecuteTool runs a registered tool with the given input
func (o *OuroborosLoop) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	handle, exists := o.registry.Get(toolName)
	if !exists {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, o.config.ExecuteTimeout)
	defer cancel()

	o.mu.Lock()
	o.stats.ExecutionCount++
	o.mu.Unlock()

	return handle.Execute(execCtx, input)
}

// GetStats returns current loop statistics
func (o *OuroborosLoop) GetStats() OuroborosStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.stats
}

// ListTools returns all registered tools for the ToolExecutor interface
func (o *OuroborosLoop) ListTools() []ToolInfo {
	tools := o.registry.List()
	result := make([]ToolInfo, len(tools))
	for i, t := range tools {
		result[i] = ToolInfo{
			Name:         t.Name,
			Description:  t.Description,
			BinaryPath:   t.BinaryPath,
			Hash:         t.Hash,
			RegisteredAt: t.RegisteredAt,
			ExecuteCount: t.ExecuteCount,
		}
	}
	return result
}

// GetTool returns info about a specific tool for the ToolExecutor interface
func (o *OuroborosLoop) GetTool(name string) (*ToolInfo, bool) {
	rt, exists := o.registry.Get(name)
	if !exists {
		return nil, false
	}
	return &ToolInfo{
		Name:         rt.Name,
		Description:  rt.Description,
		BinaryPath:   rt.BinaryPath,
		Hash:         rt.Hash,
		RegisteredAt: rt.RegisteredAt,
		ExecuteCount: rt.ExecuteCount,
	}, true
}

// ToolInfo contains information about a registered tool (mirrors core.ToolInfo)
type ToolInfo struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	BinaryPath   string    `json:"binary_path"`
	Hash         string    `json:"hash"`
	RegisteredAt time.Time `json:"registered_at"`
	ExecuteCount int64     `json:"execute_count"`
}

// =============================================================================
// SAFETY CHECKER - THE GUARDIAN
// =============================================================================
// Ensures generated tools don't contain dangerous code.

// SafetyChecker validates generated tool code for safety
type SafetyChecker struct {
	config           OuroborosConfig
	forbiddenImports []string
	forbiddenCalls   []*regexp.Regexp
}

// SafetyReport contains the results of a safety check
type SafetyReport struct {
	Safe             bool
	Violations       []SafetyViolation
	ImportsChecked   int
	CallsChecked     int
	Score            float64 // 0.0 = unsafe, 1.0 = perfectly safe
}

// SafetyViolation describes a single safety issue
type SafetyViolation struct {
	Type        ViolationType
	Location    string // file:line
	Description string
	Severity    ViolationSeverity
}

// ViolationType categorizes violations
type ViolationType int

const (
	ViolationForbiddenImport ViolationType = iota
	ViolationDangerousCall
	ViolationUnsafePointer
	ViolationReflection
	ViolationCGO
	ViolationExec
)

func (v ViolationType) String() string {
	switch v {
	case ViolationForbiddenImport:
		return "forbidden_import"
	case ViolationDangerousCall:
		return "dangerous_call"
	case ViolationUnsafePointer:
		return "unsafe_pointer"
	case ViolationReflection:
		return "reflection"
	case ViolationCGO:
		return "cgo"
	case ViolationExec:
		return "exec"
	default:
		return "unknown"
	}
}

// ViolationSeverity indicates how serious a violation is
type ViolationSeverity int

const (
	SeverityInfo ViolationSeverity = iota
	SeverityWarning
	SeverityCritical
	SeverityBlocking
)

// NewSafetyChecker creates a new safety checker
func NewSafetyChecker(config OuroborosConfig) *SafetyChecker {
	checker := &SafetyChecker{
		config: config,
	}

	// Define forbidden imports based on config
	checker.forbiddenImports = []string{
		"unsafe",           // Always forbidden - memory safety
		"syscall",          // Always forbidden - system calls
		"runtime/cgo",      // Always forbidden - CGO
		"plugin",           // Forbidden - plugin loading
		"debug/",           // Forbidden - debugging tools
	}

	// Add conditional forbidden imports
	if !config.AllowExec {
		checker.forbiddenImports = append(checker.forbiddenImports,
			"os/exec",      // Command execution
		)
	}

	if !config.AllowNetworking {
		checker.forbiddenImports = append(checker.forbiddenImports,
			"net",          // Networking
			"net/http",     // HTTP client/server
			"net/rpc",      // RPC
			"crypto/tls",   // TLS (implies networking)
		)
	}

	// Define dangerous function call patterns
	checker.forbiddenCalls = []*regexp.Regexp{
		regexp.MustCompile(`\bos\.Setenv\b`),           // Environment modification
		regexp.MustCompile(`\bos\.Chdir\b`),            // Directory change
		regexp.MustCompile(`\bos\.Chmod\b`),            // Permission change
		regexp.MustCompile(`\bos\.Chown\b`),            // Ownership change
		regexp.MustCompile(`\bos\.Remove\b`),           // File deletion
		regexp.MustCompile(`\bos\.RemoveAll\b`),        // Recursive deletion
		regexp.MustCompile(`\bos\.Rename\b`),           // File rename/move
		regexp.MustCompile(`\breflect\.Value\b`),       // Reflection (warning only)
		regexp.MustCompile(`\bunsafe\.Pointer\b`),      // Unsafe pointers
		regexp.MustCompile(`\b\*\(\*\w+\)\(unsafe\.`),  // Unsafe type conversion
	}

	return checker
}

// Check performs a comprehensive safety check on the code
func (sc *SafetyChecker) Check(code string) *SafetyReport {
	report := &SafetyReport{
		Safe:       true,
		Violations: []SafetyViolation{},
	}

	// Parse the Go code
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, parser.ParseComments)
	if err != nil {
		report.Safe = false
		report.Violations = append(report.Violations, SafetyViolation{
			Type:        ViolationDangerousCall,
			Description: fmt.Sprintf("Failed to parse code: %v", err),
			Severity:    SeverityBlocking,
		})
		return report
	}

	// Check imports
	sc.checkImports(file, fset, report)

	// Check for dangerous calls
	sc.checkDangerousCalls(code, report)

	// Check for CGO
	sc.checkCGO(code, report)

	// Calculate safety score
	report.Score = sc.calculateScore(report)

	return report
}

// checkImports validates all imports in the code
func (sc *SafetyChecker) checkImports(file *ast.File, fset *token.FileSet, report *SafetyReport) {
	for _, imp := range file.Imports {
		report.ImportsChecked++
		importPath := strings.Trim(imp.Path.Value, `"`)

		for _, forbidden := range sc.forbiddenImports {
			if strings.HasPrefix(importPath, forbidden) || importPath == forbidden {
				report.Safe = false
				report.Violations = append(report.Violations, SafetyViolation{
					Type:        ViolationForbiddenImport,
					Location:    fset.Position(imp.Pos()).String(),
					Description: fmt.Sprintf("Forbidden import: %s", importPath),
					Severity:    SeverityBlocking,
				})
			}
		}
	}
}

// checkDangerousCalls looks for dangerous function calls
func (sc *SafetyChecker) checkDangerousCalls(code string, report *SafetyReport) {
	lines := strings.Split(code, "\n")

	for i, line := range lines {
		report.CallsChecked++

		for _, pattern := range sc.forbiddenCalls {
			if pattern.MatchString(line) {
				severity := SeverityWarning
				// Some calls are blocking violations
				if strings.Contains(line, "RemoveAll") ||
					strings.Contains(line, "unsafe.Pointer") {
					severity = SeverityBlocking
					report.Safe = false
				}

				report.Violations = append(report.Violations, SafetyViolation{
					Type:        ViolationDangerousCall,
					Location:    fmt.Sprintf("line:%d", i+1),
					Description: fmt.Sprintf("Potentially dangerous call: %s", strings.TrimSpace(line)),
					Severity:    severity,
				})
			}
		}
	}
}

// checkCGO looks for CGO usage
func (sc *SafetyChecker) checkCGO(code string, report *SafetyReport) {
	cgoPatterns := []*regexp.Regexp{
		regexp.MustCompile(`import\s+"C"`),
		regexp.MustCompile(`#cgo\s+`),
		regexp.MustCompile(`/\*\s*#include`),
	}

	for _, pattern := range cgoPatterns {
		if pattern.MatchString(code) {
			report.Safe = false
			report.Violations = append(report.Violations, SafetyViolation{
				Type:        ViolationCGO,
				Description: "CGO usage detected - not allowed in generated tools",
				Severity:    SeverityBlocking,
			})
			break
		}
	}
}

// calculateScore computes a safety score
func (sc *SafetyChecker) calculateScore(report *SafetyReport) float64 {
	if !report.Safe {
		return 0.0
	}

	score := 1.0
	for _, v := range report.Violations {
		switch v.Severity {
		case SeverityInfo:
			score -= 0.01
		case SeverityWarning:
			score -= 0.1
		case SeverityCritical:
			score -= 0.3
		case SeverityBlocking:
			return 0.0
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// =============================================================================
// TOOL COMPILER - THE FORGE
// =============================================================================
// Compiles generated tools for runtime execution.

// ToolCompiler compiles generated tools
type ToolCompiler struct {
	config OuroborosConfig
}

// CompileResult contains compilation output
type CompileResult struct {
	Success      bool
	OutputPath   string
	Hash         string // SHA-256 of compiled binary
	CompileTime  time.Duration
	Errors       []string
	Warnings     []string
}

// NewToolCompiler creates a new tool compiler
func NewToolCompiler(config OuroborosConfig) *ToolCompiler {
	return &ToolCompiler{config: config}
}

// Compile compiles a generated tool
func (tc *ToolCompiler) Compile(ctx context.Context, tool *GeneratedTool) (*CompileResult, error) {
	start := time.Now()
	result := &CompileResult{
		Success: false,
	}

	// Ensure compiled directory exists
	if err := os.MkdirAll(tc.config.CompiledDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create compiled dir: %w", err)
	}

	// Create a temporary directory for the build
	tmpDir, err := os.MkdirTemp("", "ouroboros-build-*")
	if err != nil {
		return result, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write the tool source to temp directory
	srcPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcPath, []byte(tc.wrapAsMain(tool)), 0644); err != nil {
		return result, fmt.Errorf("failed to write source: %w", err)
	}

	// Initialize go module
	modContent := fmt.Sprintf("module %s\n\ngo 1.24\n", tool.Name)
	modPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(modPath, []byte(modContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Add replace directive for the main 'codenerd' module if the tool uses internal packages
	// We assume the tool generated imports from the main 'codenerd' module.
	// The workspaceRoot is available from the OuroborosConfig.
	mainModulePath := tc.config.WorkspaceRoot // Assumes WorkspaceRoot is set in OuroborosConfig
	if mainModulePath == "" {
		mainModulePath = os.Getenv("CODE_NERD_WORKSPACE_ROOT") // Fallback
	}
	if mainModulePath != "" {
		replaceCmd := exec.CommandContext(ctx, "go", "mod", "edit", fmt.Sprintf("-replace=codenerd=%s", mainModulePath))
		replaceCmd.Dir = tmpDir
		replaceOutput, err := replaceCmd.CombinedOutput()
		if err != nil {
			result.Errors = append(result.Errors, string(replaceOutput))
			return result, fmt.Errorf("go mod edit -replace failed: %w\n%s", err, replaceOutput)
		}
	}
	
	// Run go mod tidy to resolve dependencies and create go.sum
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyOutput, err := tidyCmd.CombinedOutput()
	if err != nil {
		result.Errors = append(result.Errors, string(tidyOutput))
		return result, fmt.Errorf("go mod tidy failed: %w\n%s", err, tidyOutput)
	}

	// Output path for compiled binary (append .exe if windows)
	ext := ""
	if tc.config.TargetOS == "windows" {
		ext = ".exe"
	}
	outputPath := filepath.Join(tc.config.CompiledDir, tool.Name+ext)

	// Create compile context with timeout
	compileCtx, cancel := context.WithTimeout(ctx, tc.config.CompileTimeout)
	defer cancel()

	// Prepare build flags for static binary and optimization
	ldflags := "-s -w" // Strip symbol table and debug info
	if tc.config.TargetOS == "linux" {
		ldflags += " -extldflags '-static'" // Force static linking on Linux
	}

	// Run go build
	args := []string{"build", "-ldflags", ldflags, "-o", outputPath, "."}
	cmd := exec.CommandContext(compileCtx, "go", args...)
	cmd.Dir = tmpDir
	
	// Setup environment for cross-compilation
	env := os.Environ()
	env = append(env, "CGO_ENABLED=0") // Force pure Go / static
	if tc.config.TargetOS != "" {
		env = append(env, "GOOS="+tc.config.TargetOS)
	}
	if tc.config.TargetArch != "" {
		env = append(env, "GOARCH="+tc.config.TargetArch)
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Errors = append(result.Errors, string(output))
		return result, fmt.Errorf("compilation failed: %w\n%s", err, output)
	}

	// Calculate hash of compiled binary
	binaryContent, err := os.ReadFile(outputPath)
	if err != nil {
		return result, fmt.Errorf("failed to read compiled binary: %w", err)
	}

	hash := sha256.Sum256(binaryContent)
	result.Hash = hex.EncodeToString(hash[:])
	result.OutputPath = outputPath
	result.Success = true
	result.CompileTime = time.Since(start)

	return result, nil
}

// wrapAsMain wraps the tool code as a standalone main package.
// It creates a main function that can either read a JSON payload from stdin
// (for agent invocation) or parse command-line arguments (for direct CLI use).
// The generated tool code must implement func RunTool(input string, args []string) (string, error).
func (tc *ToolCompiler) wrapAsMain(tool *GeneratedTool) string {
	// If the generated code is already a full package main, return it as-is.
	// This allows highly customized CLI tools to bypass the standard wrapper.
	// The safety checker should ensure these are still safe.
	if strings.Contains(tool.Code, "package main") {
		return tool.Code
	}

	wrapper := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ToolInput is the input format for the tool when executed by the agent.
type ToolInput struct {
	Input string   ` + "`json:\"input\"`" + ` // Primary input (e.g., first CLI arg, or main JSON payload)
	Args  []string ` + "`json:\"args\"`" + `   // Additional arguments (e.g., remaining CLI args)
}

// ToolOutput is the output format from the tool.
type ToolOutput struct {
	Output string ` + "`json:\"output\"`" + `
	Error  string ` + "`json:\"error,omitempty\"`" + `
}

// RunTool is the entry point for the tool's core logic.
// The tool's generated code MUST define this function (e.g., func RunTool(input string, args []string) (string, error)).
// It will be appended below this wrapper.

func main() {
	var primaryInput string
	var cliArgs []string

	// Determine if stdin is being piped (implies agent execution with JSON payload)
	fi, _ := os.Stdin.Stat()
	isPiped := (fi.Mode() & os.ModeCharDevice) == 0

	if isPiped {
		// Agent execution: Read JSON from stdin
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			// No input from stdin, can proceed with empty primaryInput and cliArgs if tool handles it.
		} else {
			var agentInput ToolInput
			rawInput := scanner.Bytes()
			if jsonErr := json.Unmarshal(rawInput, &agentInput); jsonErr != nil {
				// If JSON unmarshal fails, treat the whole line as raw primaryInput
				primaryInput = strings.TrimSpace(string(rawInput))
				// No additional cliArgs in this case
			} else {
				primaryInput = agentInput.Input
				cliArgs = agentInput.Args
			}
		}
	} else {
		// CLI execution: Read arguments from os.Args
		if len(os.Args) > 1 {
			primaryInput = os.Args[1] // First argument is treated as primary input
			if len(os.Args) > 2 {
				cliArgs = os.Args[2:] // Remaining arguments are additional CLI args
			}
		} else {
			// No input and not piped, print usage and exit
			fmt.Fprintf(os.Stderr, "Usage: %s <primary_input> [additional_args...]\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "Or pipe JSON for agent use: echo '{\"input\": \"file.mg\", \"args\": [\"--verbose\"]}' | %s\n", os.Args[0])
			os.Exit(1)
		}
	}

	// Execute the actual tool logic
	result, runErr := RunTool(primaryInput, cliArgs)
	
	// Prepare output structure for agent consumption (always JSON)
	output := ToolOutput{Output: result}
	if runErr != nil {
		output.Error = runErr.Error()
	}

	jsonEncoder := json.NewEncoder(os.Stdout)
	jsonEncoder.SetIndent("", "  ") // Pretty print for human readability
	jsonEncoder.Encode(output)

	if runErr != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
`
	// The generated tool code is expected to define `RunTool(input string, cliArgs []string) (string, error)`.
	// It should also start with `package tools` as it will be part of the generated `main` package here.
	toolCodeWithPackage := tool.Code
	if !strings.HasPrefix(toolCodeWithPackage, "package ") {
		toolCodeWithPackage = "package tools\n\n" + toolCodeWithPackage
	} else if strings.HasPrefix(toolCodeWithPackage, "package main") {
		// Replace "package main" with "package tools" if the tool code explicitly defines main,
		// as it will now be part of the wrapper's main package.
		toolCodeWithPackage = regexp.MustCompile(`^package main\s`).ReplaceAllString(toolCodeWithPackage, "package tools\n")
	}

	return wrapper + "\n" + toolCodeWithPackage
}

// extractFunctionBody extracts the body of the main tool function
func extractFunctionBody(code, funcName string) string {
	// Simple regex extraction - production code would use AST
	pattern := regexp.MustCompile(
		fmt.Sprintf(`func\s+%s\s*\([^)]*\)\s*\([^)]*\)\s*\{([^}]+)\}`,
			regexp.QuoteMeta(toCamelCase(funcName))))

	matches := pattern.FindStringSubmatch(code)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// =============================================================================
// RUNTIME REGISTRY - THE MENAGERIE
// =============================================================================
// Manages registered tools available for runtime execution.

// RuntimeRegistry manages registered tools
type RuntimeRegistry struct {
	mu    sync.RWMutex
	tools map[string]*RuntimeTool
}

// RuntimeTool represents a compiled tool ready for execution
type RuntimeTool struct {
	Name         string
	Description  string
	BinaryPath   string
	Hash         string
	Schema       ToolSchema
	RegisteredAt time.Time
	ExecuteCount int64
}

// NewRuntimeRegistry creates a new registry
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		tools: make(map[string]*RuntimeTool),
	}
}

// Register adds a tool to the registry
func (r *RuntimeRegistry) Register(tool *GeneratedTool, compiled *CompileResult) (*RuntimeTool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt := &RuntimeTool{
		Name:         tool.Name,
		Description:  tool.Description,
		BinaryPath:   compiled.OutputPath,
		Hash:         compiled.Hash,
		Schema:       tool.Schema,
		RegisteredAt: time.Now(),
	}

	r.tools[tool.Name] = rt
	return rt, nil
}

// Get retrieves a tool by name
func (r *RuntimeRegistry) Get(name string) (*RuntimeTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *RuntimeRegistry) List() []*RuntimeTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*RuntimeTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Restore rebuilds the registry from disk
func (r *RuntimeRegistry) Restore(toolsDir, compiledDir string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// List all binaries in compiled dir
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		return // Directory might not exist yet
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Strip extension (e.g. .exe on Windows)
		name = strings.TrimSuffix(name, ".exe")

		// Check if source exists
		srcPath := filepath.Join(toolsDir, name+".go")
		if _, err := os.Stat(srcPath); err != nil {
			continue // Orphaned binary
		}

		// Create runtime tool
		binaryPath := filepath.Join(compiledDir, entry.Name())
		
		// Calculate hash
		hash := ""
		if content, err := os.ReadFile(binaryPath); err == nil {
			h := sha256.Sum256(content)
			hash = hex.EncodeToString(h[:])
		}

		rt := &RuntimeTool{
			Name:         name,
			Description:  "Restored from disk", // We could parse source to get better desc
			BinaryPath:   binaryPath,
			Hash:         hash,
			Schema:       ToolSchema{Name: name}, // Basic schema
			RegisteredAt: time.Now(),
		}

		r.tools[name] = rt
	}
}

// Execute runs the tool with the given input
func (rt *RuntimeTool) Execute(ctx context.Context, input string) (string, error) {
	// Verify binary still exists
	if _, err := os.Stat(rt.BinaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("tool binary not found: %s", rt.BinaryPath)
	}

	// Prepare input
	inputJSON, err := json.Marshal(map[string]string{"input": input})
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute the tool binary
	cmd := exec.CommandContext(ctx, rt.BinaryPath)
	cmd.Stdin = strings.NewReader(string(inputJSON))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// Parse output
	var result struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse tool output: %w", err)
	}

	if result.Error != "" {
		return result.Output, fmt.Errorf("tool error: %s", result.Error)
	}

	rt.ExecuteCount++
	return result.Output, nil
}

// =============================================================================
// MANGLE FACT GENERATORS
// =============================================================================
// Generate Mangle facts for tool detection and management.

// GenerateMissingToolFacts creates facts for Mangle missing_tool_for detection
func GenerateMissingToolFacts(intentID, capability string) []string {
	return []string{
		fmt.Sprintf(`missing_tool_for(%q, %q).`, intentID, capability),
	}
}

// GenerateToolCapabilityFacts creates facts for available tool capabilities
func GenerateToolCapabilityFacts(toolName string, capabilities []string) []string {
	facts := make([]string, 0, len(capabilities)+1)
	facts = append(facts, fmt.Sprintf(`tool_exists(%q).`, toolName))

	for _, cap := range capabilities {
		facts = append(facts, fmt.Sprintf(`tool_capability(%q, %q).`, toolName, cap))
	}
	return facts
}

// GenerateToolRegistrationFacts creates facts when a tool is registered
func GenerateToolRegistrationFacts(tool *RuntimeTool) []string {
	return []string{
		fmt.Sprintf(`tool_registered(%q, %q).`, tool.Name, tool.RegisteredAt.Format(time.RFC3339)),
		fmt.Sprintf(`tool_hash(%q, %q).`, tool.Name, tool.Hash),
		fmt.Sprintf(`has_capability(%q).`, tool.Name),
	}
}
