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
		AllowExec:       true,
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
		// Default to user's OS environment assumption or runtime
		if os.Getenv("GOOS") != "" {
			config.TargetOS = os.Getenv("GOOS")
		} else {
			config.TargetOS = "windows"
		}
	}
	if config.TargetArch == "" {
		if os.Getenv("GOARCH") != "" {
			config.TargetArch = os.Getenv("GOARCH")
		} else {
			config.TargetArch = "amd64"
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
	Error        string
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
		result.Error = fmt.Sprintf("specification failed: %v", err)
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

		result.Error = fmt.Sprintf("safety check failed: %v", safetyReport.Violations)
		return result
	}
	result.Stage = StageCompilation

	// Stage 4: Compilation - Write and compile the tool
	if err := o.toolGen.WriteTool(tool); err != nil {
		result.Error = fmt.Sprintf("write failed: %v", err)
		return result
	}

	compileResult, err := o.compiler.Compile(ctx, tool)
	result.CompileResult = compileResult
	if err != nil {
		result.Error = fmt.Sprintf("compilation failed: %v", err)
		return result
	}
	result.Stage = StageRegistration

	// Stage 5: Registration - Register the tool for runtime use
	handle, err := o.registry.Register(tool, compileResult)
	if err != nil {
		result.Error = fmt.Sprintf("registration failed: %v", err)
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

	// Determine if tool is already package main
	isMain := strings.Contains(tool.Code, "package main")

	if isMain {
		// Write as main.go directly
		srcPath := filepath.Join(tmpDir, "main.go")
		if err := os.WriteFile(srcPath, []byte(tool.Code), 0644); err != nil {
			return result, fmt.Errorf("failed to write source: %w", err)
		}
	} else {
		// Write tool.go, changing package tools -> package main
		toolContent := tool.Code
		if strings.Contains(toolContent, "package tools") {
			toolContent = strings.Replace(toolContent, "package tools", "package main", 1)
		} else if !strings.Contains(toolContent, "package ") {
			toolContent = "package main\n\n" + toolContent
		}
		
		if err := os.WriteFile(filepath.Join(tmpDir, "tool.go"), []byte(toolContent), 0644); err != nil {
			return result, fmt.Errorf("failed to write tool source: %w", err)
		}

		// Find entry point function
		entryPoint, err := tc.findEntryPoint(toolContent)
		if err != nil {
			return result, fmt.Errorf("failed to find entry point: %w", err)
		}

		// Write wrapper main.go
		if err := tc.writeWrapper(tmpDir, entryPoint); err != nil {
			return result, fmt.Errorf("failed to write wrapper: %w", err)
		}
	}

	// Initialize go module
	modContent := fmt.Sprintf("module %s\n\ngo 1.24\n", tool.Name)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Add replace directive if needed
	mainModulePath := tc.config.WorkspaceRoot
	if mainModulePath == "" {
		mainModulePath = os.Getenv("CODE_NERD_WORKSPACE_ROOT")
	}
	if mainModulePath != "" {
		exec.CommandContext(ctx, "go", "mod", "edit", fmt.Sprintf("-replace=codenerd=%s", mainModulePath)).Run()
	}
	
	// Run go mod tidy
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		result.Errors = append(result.Errors, string(out))
		return result, fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Output path
	ext := ""
	if tc.config.TargetOS == "windows" {
		ext = ".exe"
	}
	outputPath := filepath.Join(tc.config.CompiledDir, tool.Name+ext)

	// Build
	compileCtx, cancel := context.WithTimeout(ctx, tc.config.CompileTimeout)
	defer cancel()

	ldflags := "-s -w"
	if tc.config.TargetOS == "linux" {
		ldflags += " -extldflags '-static'"
	}

	cmd := exec.CommandContext(compileCtx, "go", "build", "-ldflags", ldflags, "-o", outputPath, ".")
	cmd.Dir = tmpDir
	
	env := os.Environ()
	env = append(env, "CGO_ENABLED=0")
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

	// Hash
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

// findEntryPoint parses code to find the main tool function
func (tc *ToolCompiler) findEntryPoint(code string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", code, 0)
	if err != nil {
		return "", err
	}

	var foundFunc string
	var maxScore int

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		name := fn.Name.Name
		if name == "main" || strings.HasPrefix(name, "Register") {
			return true
		}

		score := 0
		if fn.Name.IsExported() {
			score += 5
		}
		
		// Check signature: (ctx, input) (output, error)
		if fn.Type.Params != nil && len(fn.Type.Params.List) >= 1 {
			// Heuristic check for context
			if len(fn.Type.Params.List) >= 1 {
				score += 5
			}
		}
		if fn.Type.Results != nil && len(fn.Type.Results.List) == 2 {
			score += 5
		}

		if score > maxScore {
			maxScore = score
			foundFunc = name
		}
		return true
	})

	if foundFunc == "" {
		return "", fmt.Errorf("no suitable entry point function found")
	}
	return foundFunc, nil
}

// writeWrapper generates the main.go wrapper
func (tc *ToolCompiler) writeWrapper(dir, funcName string) error {
	content := fmt.Sprintf(`package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ToolInput matches standard agent input
type ToolInput struct {
	Input string   ` + "`json:\"input\"`" + `
	Args  []string ` + "`json:\"args\"`" + `
}

type ToolOutput struct {
	Output string ` + "`json:\"output\"`" + `
	Error  string ` + "`json:\"error,omitempty\"`" + `
}

func main() {
	var input string
	
	// Check for pipe input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			var toolInput ToolInput
			if err := json.Unmarshal(scanner.Bytes(), &toolInput); err == nil {
				input = toolInput.Input
			} else {
				input = strings.TrimSpace(scanner.Text())
			}
		}
	} else if len(os.Args) > 1 {
		input = os.Args[1]
	}

	// Execute
	ctx := context.Background()
	// Assume output is string for now, tool logic handles types
	// We pass input string directly. 
	// Limitation: The generated function might expect a struct or int.
	// But our prompt asks for string input usually.
	// If it's not string, this wrapper is too simple.
	// For Ouroboros v1, we enforce string input/output interface.
	
	res, err := %s(ctx, input)
	
	output := ToolOutput{}
	if err != nil {
		output.Error = err.Error()
	} else {
		output.Output = fmt.Sprintf("%%v", res)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
	
	if err != nil {
		os.Exit(1)
	}
}
`, funcName)

	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)
}
// wrapAsMain wraps the tool code as a standalone main package.
// DEPRECATED: Use writeWrapper instead. Retained for backward compatibility if needed.
func (tc *ToolCompiler) wrapAsMain(tool *GeneratedTool) string {
    return tool.Code
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
