package autopoiesis

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
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

// NewYaegiExecutor creates a new Yaegi-based tool executor with the default
// standalone allowlist.
//
// Prefer NewYaegiExecutorForPolicy inside the Ouroboros loop: this list is a
// second, independent answer to "what may a generated tool import", and a tool
// that passed the SafetyChecker could still be refused here (the old list had
// no "context", which every tool the compiler accepts must import).
func NewYaegiExecutor() *YaegiExecutor {
	return NewYaegiExecutorForPolicy([]string{
		"bytes",
		"context",
		"encoding/base64",
		"encoding/json",
		"errors",
		"fmt",
		"math",
		"path",
		"path/filepath",
		"regexp",
		"sort",
		"strconv",
		"strings",
		"time",
		// EXPLICITLY BLOCKED (unsafe packages):
		// "os" - filesystem access
		// "os/exec" - command execution
		// "net" - network access
		// "net/http" - HTTP client
		// "syscall" - system calls
		// "unsafe" - unsafe operations
	})
}

// NewYaegiExecutorForPolicy builds an executor whose import allowlist is the
// one the SafetyChecker already enforced, so the interpreter and the compiler
// answer to a single policy instead of drifting apart.
//
// Packages that hand the interpreter ambient authority are stripped regardless
// of what the caller passes: an interpreted tool has no separate process, no
// scrubbed environment and no binary boundary, so os/exec and networking are
// materially more dangerous here than in the compiled path even when the
// compiled path is configured to allow them.
func NewYaegiExecutorForPolicy(pkgs []string) *YaegiExecutor {
	blocked := map[string]bool{
		"os": true, "os/exec": true, "syscall": true, "unsafe": true,
		"net": true, "net/http": true, "net/url": true, "plugin": true,
		"io/ioutil": true, "reflect": true,
	}
	allowed := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		if blocked[pkg] {
			continue
		}
		allowed[pkg] = true
	}
	// The entry-point contract is func(context.Context, string) (string, error),
	// so context is structurally required no matter what the policy says.
	allowed["context"] = true
	return &YaegiExecutor{allowedPackages: allowed}
}

// AllowedPackages returns the interpreter's import allowlist, sorted.
func (ye *YaegiExecutor) AllowedPackages() []string {
	return ye.getAllowedPackages()
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

	invoke, err := ye.resolveEntryPoint(i, code)
	if err != nil {
		return "", err
	}

	// Execute with context timeout
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := invoke(ctx, input)
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

// resolveEntryPoint finds the tool's callable inside the interpreter.
//
// Two contracts are accepted. `RunTool(input string) (string, error)` is the
// interpreter's historical shape; `func Name(ctx context.Context, input string)
// (string, error)` is the one the SafetyChecker, the compiler wrapper and the
// Thunderdome harness all require. Only supporting the first is why the
// interpreted path could never run a tool this pipeline actually produces.
func (ye *YaegiExecutor) resolveEntryPoint(i *interp.Interpreter, code string) (func(context.Context, string) (string, error), error) {
	if v, err := i.Eval("main.RunTool"); err == nil {
		if fn, ok := v.Interface().(func(string) (string, error)); ok {
			return func(_ context.Context, input string) (string, error) { return fn(input) }, nil
		}
		if fn, ok := v.Interface().(func(context.Context, string) (string, error)); ok {
			return fn, nil
		}
	}

	for _, name := range entryPointCandidates(code) {
		v, err := i.Eval("main." + name)
		if err != nil {
			continue
		}
		if fn, ok := v.Interface().(func(context.Context, string) (string, error)); ok {
			return fn, nil
		}
	}

	return nil, fmt.Errorf("no entry point found: expected RunTool(string) (string, error) or " +
		"an exported func(context.Context, string) (string, error)")
}

// entryPointCandidates lists exported top-level function names in source
// order, excluding names the compiler also excludes.
func entryPointCandidates(code string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, 0)
	if err != nil {
		return nil
	}
	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name == nil {
			continue
		}
		name := fn.Name.Name
		if name == "main" || name == "init" || strings.HasPrefix(name, "Register") {
			continue
		}
		if !ast.IsExported(name) {
			continue
		}
		names = append(names, name)
	}
	return names
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
		} else if after, ok := strings.CutPrefix(trimmed, "import "); ok {
			// Single import
			pkg := after
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
	pkgs := make([]string, 0, len(ye.allowedPackages))
	for pkg := range ye.allowedPackages {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	return pkgs
}

// =============================================================================
// EXECUTION POLICY
// =============================================================================

// ToolExecutionMode selects the backend that runs a registered tool.
//
// The policy lives on OuroborosConfig (ExecutionMode /
// AllowCompilationFallback) so there is exactly one place to configure it.
// A parallel ToolExecutionConfig struct used to sit here, declaring
// ExecuteInterpreted as "the safe default" while nothing read it and the
// product actually ran compiled binaries — two contradictory policies, neither
// wired. It is gone; OuroborosConfig owns this.
type ToolExecutionMode int

const (
	// ExecuteCompiled builds the tool with `go build` and runs the binary in
	// a separate process with a scrubbed environment. Product default: it is
	// the only mode with a process boundary and a hard context kill.
	ExecuteCompiled ToolExecutionMode = iota
	// ExecuteInterpreted runs the tool's source in the Yaegi sandbox inside
	// this process. Needs no Go toolchain, but a timeout abandons the tool's
	// goroutine rather than killing it.
	ExecuteInterpreted
)
