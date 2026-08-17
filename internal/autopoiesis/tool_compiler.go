package autopoiesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/build"
)

// =============================================================================
// TOOL COMPILER - THE FORGE
// =============================================================================
// Compiles generated tools for runtime execution.

// ToolCompiler compiles generated tools
type ToolCompiler struct {
	config OuroborosConfig
}

// NewToolCompiler creates a new tool compiler
func NewToolCompiler(config OuroborosConfig) *ToolCompiler {
	return &ToolCompiler{config: config}
}

// buildEnvRoot is the header/GOFLAGS *detection* root for a generated tool's
// compile. It is deliberately not the throwaway temp module: a tool that pulls
// in codenerd via the `replace` directive below has to be built with the same
// tags and CGO include paths as the main module, and those live at the
// workspace root. Passing the temp dir found no sqlite_headers and no
// -tags=sqlite_vec, so such a tool failed to compile for a reason that had
// nothing to do with the code the LLM wrote.
func (tc *ToolCompiler) buildEnvRoot(tmpDir string) string {
	if root := strings.TrimSpace(tc.config.WorkspaceRoot); root != "" {
		return root
	}
	return tmpDir
}

// Compile compiles a generated tool
func (tc *ToolCompiler) Compile(ctx context.Context, tool *GeneratedTool) (*CompileResult, error) {
	start := time.Now()
	tool.Name = sanitizeToolName(tool.Name)
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

	// Write generated test code if present, ensuring it lands in package main
	// so it is compiled alongside the tool code it exercises.
	if strings.TrimSpace(tool.TestCode) != "" {
		testContent := tool.TestCode
		if strings.Contains(testContent, "package tools") {
			testContent = strings.Replace(testContent, "package tools", "package main", 1)
		} else if !strings.Contains(testContent, "package ") {
			testContent = "package main\n\n" + testContent
		} else if !strings.Contains(testContent, "package main") {
			// Normalize any other package name to main so tests compile alongside the tool.
			re := regexp.MustCompile(`(?m)^\s*package\s+\w+`)
			testContent = re.ReplaceAllString(testContent, "package main")
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "tool_test.go"), []byte(testContent), 0644); err != nil {
			return result, fmt.Errorf("failed to write test source: %w", err)
		}
	}

	// Initialize go module
	modContent := fmt.Sprintf("module %s\n\ngo 1.26.0\n", tool.Name)

	// Add replace directive if needed
	mainModulePath := tc.config.WorkspaceRoot
	if mainModulePath == "" {
		mainModulePath = os.Getenv("CODE_NERD_WORKSPACE_ROOT")
	}
	if mainModulePath != "" {
		if strings.ContainsAny(mainModulePath, "\n\r") {
			return result, fmt.Errorf("security violation: workspace root path contains invalid characters")
		}
		modContent += fmt.Sprintf("replace codenerd => %s\n", strconv.Quote(mainModulePath))
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Run go mod tidy
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		result.Errors = append(result.Errors, string(out))
		return result, fmt.Errorf("go mod tidy failed: %w", err)
	}

	// If test code was provided, run it before producing the final binary.
	// This gates registration on the tool satisfying its own author's assertions.
	if strings.TrimSpace(tool.TestCode) != "" {
		testTimeout := tc.config.CompileTimeout
		if testTimeout == 0 {
			testTimeout = 60 * time.Second
		}
		testCtx, testCancel := context.WithTimeout(ctx, testTimeout)
		defer testCancel()

		testCmd := exec.CommandContext(testCtx, "go", "test", "./...")
		testCmd.Dir = tmpDir
		// Reuse the same environment the build uses, keeping CGO handling identical.
		testCmd.Env = build.MergeEnv(
			build.GetBuildEnvForCompile(tc.config.UserConfig, tc.buildEnvRoot(tmpDir), tc.config.TargetOS, tc.config.TargetArch),
			"CGO_ENABLED=0",
		)
		testOutput, err := testCmd.CombinedOutput()
		if err != nil {
			truncated := string(testOutput)
			const maxOutputLen = 4096
			if len(truncated) > maxOutputLen {
				truncated = truncated[:maxOutputLen] + "\n... (truncated)"
			}
			result.Errors = append(result.Errors, truncated)
			return result, fmt.Errorf("generated tool failed its own generated tests: %w\n%s", err, truncated)
		}
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

	buildArgs := []string{"build"}

	ldflags := "-s -w"
	if tc.config.TargetOS == "linux" {
		ldflags += " -extldflags '-static'"
	}
	buildArgs = append(buildArgs, "-ldflags", ldflags, "-o", outputPath, "--", ".")

	/* #nosec G204 */
	cmd := exec.CommandContext(compileCtx, "go", buildArgs...)
	cmd.Dir = tmpDir

	// Use unified build environment with cross-compilation support
	// CGO_ENABLED=0 for portable binaries
	cmd.Env = build.MergeEnv(
		build.GetBuildEnvForCompile(tc.config.UserConfig, tc.buildEnvRoot(tmpDir), tc.config.TargetOS, tc.config.TargetArch),
		"CGO_ENABLED=0",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Errors = append(result.Errors, string(output))
		return result, fmt.Errorf("compilation failed: %w\n%s", err, output)
	}

	// Hash
	binaryContent, err := os.ReadFile(outputPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read binary: %v", err))
		return result, fmt.Errorf("failed to read compiled binary: %w", err)
	}

	hash := sha256.Sum256(binaryContent)
	result.Hash = hex.EncodeToString(hash[:])
	result.OutputPath = outputPath
	result.Success = true
	result.CompileTime = time.Since(start)

	return result, nil
}

// Helper to verify exact signature: func(ctx context.Context, input string) (string, error)
func isExactEntryPoint(fn *ast.FuncDecl) bool {
	// Must have exactly 2 parameters
	var paramTypes []ast.Expr
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			namesLen := len(field.Names)
			if namesLen == 0 {
				namesLen = 1 // anonymous parameter
			}
			for i := 0; i < namesLen; i++ {
				paramTypes = append(paramTypes, field.Type)
			}
		}
	}
	if len(paramTypes) != 2 {
		return false
	}

	// First param must be context.Context (or Context if dot-imported)
	firstParam := paramTypes[0]
	isContext := false
	if selExpr, ok := firstParam.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "context" {
			if selExpr.Sel.Name == "Context" {
				isContext = true
			}
		}
	} else if ident, ok := firstParam.(*ast.Ident); ok && ident.Name == "Context" {
		isContext = true
	}
	if !isContext {
		return false
	}

	// Second param must be string
	secondParam := paramTypes[1]
	if ident, ok := secondParam.(*ast.Ident); !ok || ident.Name != "string" {
		return false
	}

	// Must have exactly 2 results
	var resultTypes []ast.Expr
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			namesLen := len(field.Names)
			if namesLen == 0 {
				namesLen = 1
			}
			for i := 0; i < namesLen; i++ {
				resultTypes = append(resultTypes, field.Type)
			}
		}
	}
	if len(resultTypes) != 2 {
		return false
	}

	// First result must be string
	firstRes := resultTypes[0]
	if ident, ok := firstRes.(*ast.Ident); !ok || ident.Name != "string" {
		return false
	}

	// Second result must be error
	secondRes := resultTypes[1]
	if ident, ok := secondRes.(*ast.Ident); !ok || ident.Name != "error" {
		return false
	}

	return true
}

// findEntryPoint parses code to find the main tool function
func (tc *ToolCompiler) findEntryPoint(code string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", code, 0)
	if err != nil {
		return "", err
	}

	var foundFunc string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		name := fn.Name.Name
		if name == "main" || strings.HasPrefix(name, "Register") {
			return true
		}

		if isExactEntryPoint(fn) {
			foundFunc = name
			return false // stop inspect
		}
		return true
	})

	if foundFunc == "" {
		return "", fmt.Errorf("no suitable entry point function found with signature 'func(context.Context, string) (string, error)'")
	}
	return foundFunc, nil
}

// writeWrapper generates the main.go wrapper
func (tc *ToolCompiler) writeWrapper(dir, funcName string) error {
	content := fmt.Sprintf(`package main

import (
	"io"
	"context"
	"encoding/json"
	"os"
	"strings"
)

type ToolOutput struct {
	Output json.RawMessage `+"`json:\"output\"`"+`
	Error  string          `+"`json:\"error,omitempty\"`"+`
}

func main() {
	var input string
	
	// Check for pipe input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Read up to 10MB to avoid OOM
		reader := io.LimitReader(os.Stdin, 10*1024*1024)
		inputBytes, err := io.ReadAll(reader)
		if err == nil && len(inputBytes) > 0 {
			var toolInput struct {
				Input json.RawMessage `+"`json:\"input\"`"+`
				Args  []string        `+"`json:\"args\"`"+`
			}
			if err := json.Unmarshal(inputBytes, &toolInput); err == nil && len(toolInput.Input) > 0 {
				var strInput string
				if err := json.Unmarshal(toolInput.Input, &strInput); err == nil {
					input = strInput
				} else {
					input = string(toolInput.Input)
				}
			} else {
				input = strings.TrimSpace(string(inputBytes))
			}
		}
	} else if len(os.Args) > 1 {
		input = os.Args[1]
	}

	// Execute
	ctx := context.Background()
	
	res, err := %s(ctx, input)
	
	output := ToolOutput{}
	if err != nil {
		output.Error = err.Error()
	} else {
		// If res is valid JSON, treat it as json.RawMessage, else marshal it
		if json.Valid([]byte(res)) {
			output.Output = json.RawMessage(res)
		} else {
			marshaled, _ := json.Marshal(res)
			output.Output = json.RawMessage(marshaled)
		}
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
