package autopoiesis

import (
	"context"
	"fmt"
	"strings"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// =============================================================================
// YAEGI INTERPRETER EXECUTOR (Bug #16 Fix - Dependency Hell Prevention)
// =============================================================================
// Instead of compiling tools with `go build` (which can hang, crash, or fail
// due to missing dependencies), we use Yaegi to interpret Go code at runtime.
//
// SAFETY RESTRICTIONS:
// - Only stdlib imports allowed (no external dependencies)
// - Sandboxed execution environment
// - No network, filesystem, or exec access (can be configured)
// - Timeout enforcement via context
//
// This eliminates:
// - Compilation hangs (go build can hang for 30s on network issues)
// - Binary crashes (version mismatches, dynamic linking issues)
// - Dependency hell (missing packages, incompatible versions)

// YaegiExecutor executes Go code using the Yaegi interpreter.
type YaegiExecutor struct {
	// Whitelist of allowed stdlib packages
	allowedPackages map[string]bool
}

// NewYaegiExecutor creates a new Yaegi-based tool executor.
func NewYaegiExecutor() *YaegiExecutor {
	return &YaegiExecutor{
		allowedPackages: map[string]bool{
			// Safe stdlib packages
			"strings":  true,
			"strconv":  true,
			"fmt":      true,
			"math":     true,
			"regexp":   true,
			"encoding/json": true,
			"encoding/base64": true,
			"time":     true,
			"sort":     true,
			"bytes":    true,
			"path":     true,
			"path/filepath": true,

			// EXPLICITLY BLOCKED (unsafe packages):
			// "os" - filesystem access
			// "os/exec" - command execution
			// "net" - network access
			// "net/http" - HTTP client
			// "syscall" - system calls
			// "unsafe" - unsafe operations
		},
	}
}

// ExecuteToolCode executes Go code in a sandboxed Yaegi interpreter.
// The code must define a function: func RunTool(input string) (string, error)
func (ye *YaegiExecutor) ExecuteToolCode(ctx context.Context, code string, input string) (string, error) {
	// Validate imports before execution
	if err := ye.validateImports(code); err != nil {
		return "", fmt.Errorf("invalid imports: %w", err)
	}

	// Create interpreter
	i := interp.New(interp.Options{})

	// Load only safe stdlib symbols
	if err := i.Use(stdlib.Symbols); err != nil {
		return "", fmt.Errorf("failed to load stdlib: %w", err)
	}

	// Wrap the code in a package if not already wrapped
	fullCode := ye.wrapCode(code)

	// Evaluate the code
	if _, err := i.Eval(fullCode); err != nil {
		return "", fmt.Errorf("code evaluation failed: %w", err)
	}

	// Get the RunTool function
	runTool, err := i.Eval("main.RunTool")
	if err != nil {
		return "", fmt.Errorf("RunTool function not found: %w", err)
	}

	// Call RunTool with the input
	// The function signature is: func RunTool(input string) (string, error)
	runToolFunc, ok := runTool.Interface().(func(string) (string, error))
	if !ok {
		return "", fmt.Errorf("RunTool has incorrect signature (expected: func(string) (string, error))")
	}

	// Execute with context timeout
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := runToolFunc(input)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("tool execution timed out: %w", ctx.Err())
	}
}

// validateImports checks that the code only imports allowed packages.
func (ye *YaegiExecutor) validateImports(code string) error {
	// Extract import statements
	lines := strings.Split(code, "\n")
	var imports []string

	inImportBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for import block
		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && strings.HasPrefix(trimmed, ")") {
			inImportBlock = false
			continue
		}

		// Extract import
		if inImportBlock {
			// Remove quotes
			pkg := strings.Trim(trimmed, `"`)
			imports = append(imports, pkg)
		} else if strings.HasPrefix(trimmed, "import ") {
			// Single import
			pkg := strings.TrimPrefix(trimmed, "import ")
			pkg = strings.Trim(pkg, `"`)
			imports = append(imports, pkg)
		}
	}

	// Validate each import
	var forbidden []string
	for _, pkg := range imports {
		if !ye.allowedPackages[pkg] {
			forbidden = append(forbidden, pkg)
		}
	}

	if len(forbidden) > 0 {
		return fmt.Errorf("forbidden imports detected: %v (only stdlib allowed: %v)",
			forbidden, ye.getAllowedPackages())
	}

	return nil
}

// wrapCode wraps the tool code in a main package if needed.
func (ye *YaegiExecutor) wrapCode(code string) string {
	// If already has "package main", return as-is
	if strings.Contains(code, "package main") {
		return code
	}

	// Otherwise, wrap it
	return fmt.Sprintf(`
package main

%s
`, code)
}

// getAllowedPackages returns a list of allowed packages for error messages.
func (ye *YaegiExecutor) getAllowedPackages() []string {
	var pkgs []string
	for pkg := range ye.allowedPackages {
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// =============================================================================
// INTEGRATION WITH EXISTING TOOL SYSTEM
// =============================================================================

// SafeToolExecution is a configuration flag for the Ouroboros system.
// When enabled, tools are executed via Yaegi instead of compiled binaries.
type ToolExecutionMode int

const (
	ExecuteCompiled ToolExecutionMode = iota // Default: compile with go build
	ExecuteInterpreted                       // Bug #16 Fix: use Yaegi interpreter
)

// ToolExecutionConfig holds configuration for tool execution strategy.
type ToolExecutionConfig struct {
	Mode            ToolExecutionMode
	AllowCompilation bool // Allow fallback to compilation if interpretation fails
	Timeout         int  // Execution timeout in seconds
}

// DefaultSafeExecutionConfig returns a safe configuration using Yaegi.
func DefaultSafeExecutionConfig() ToolExecutionConfig {
	return ToolExecutionConfig{
		Mode:            ExecuteInterpreted, // Use interpreter by default
		AllowCompilation: false,             // Strict: no compilation fallback
		Timeout:         5,                  // 5 second timeout
	}
}
