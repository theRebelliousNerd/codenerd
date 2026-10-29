package autopoiesis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ValidationResult contains detailed validation results
type ValidationResult struct {
	Valid            bool
	ParseError       error
	Errors           []string
	Warnings         []string
	PackageName      string
	Functions        []string
	Imports          []string
	HasMainFunction  bool
	HasErrorHandling bool
}

// validateCode performs comprehensive AST-based validation on generated code
func (tg *ToolGenerator) validateCode(tool *GeneratedTool) error {
	result := tg.validateCodeAST(tool.Code, tool.Name)

	// Add warnings to tool
	tool.Errors = append(tool.Errors, result.Warnings...)

	// Return first error if any
	if !result.Valid {
		if result.ParseError != nil {
			return fmt.Errorf("syntax error: %w", result.ParseError)
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf("%s", result.Errors[0])
		}
		return fmt.Errorf("validation failed")
	}

	return nil
}

// validateCodeAST performs comprehensive AST-based validation
func (tg *ToolGenerator) validateCodeAST(code string, expectedToolName string) *ValidationResult {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []string{},
		Warnings:  []string{},
		Functions: []string{},
		Imports:   []string{},
	}

	// Parse the code
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, parser.ParseComments)
	if err != nil {
		result.Valid = false
		result.ParseError = err
		return result
	}

	// Check package declaration
	if file.Name == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "missing package declaration")
		return result
	}
	result.PackageName = file.Name.Name

	// Validate package name
	if result.PackageName != "tools" && result.PackageName != "main" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("package name '%s' should be 'tools' or 'main'", result.PackageName))
	}

	// Extract and validate imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, importPath)
	}

	// Check for dangerous imports (warning only - safety checker handles blocking)
	dangerousImports := []string{"unsafe", "syscall", "runtime/cgo", "plugin"}
	for _, imp := range result.Imports {
		for _, dangerous := range dangerousImports {
			if imp == dangerous || strings.HasPrefix(imp, dangerous+"/") {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("potentially dangerous import: %s", imp))
			}
		}
	}

	// Check for unused imports
	usedImports := findUsedImports(file)
	for _, imp := range result.Imports {
		// Get package name from import path
		pkgName := filepath.Base(imp)
		if !usedImports[pkgName] && !usedImports[imp] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("possibly unused import: %s", imp))
		}
	}

	// Extract functions and check for required elements
	hasContextParam := false
	hasErrorReturn := false
	expectedFuncName := toCamelCase(expectedToolName)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			funcName := node.Name.Name
			result.Functions = append(result.Functions, funcName)

			if funcName == "main" {
				result.HasMainFunction = true
			}

			// Check for expected tool function
			if funcName == expectedFuncName || strings.EqualFold(funcName, expectedFuncName) {
				// Check for context.Context parameter
				if node.Type.Params != nil {
					for _, param := range node.Type.Params.List {
						if sel, ok := param.Type.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok {
								if ident.Name == "context" && sel.Sel.Name == "Context" {
									hasContextParam = true
								}
							}
						}
					}
				}

				// Check for error return type
				if node.Type.Results != nil {
					for _, res := range node.Type.Results.List {
						if ident, ok := res.Type.(*ast.Ident); ok {
							if ident.Name == "error" {
								hasErrorReturn = true
								result.HasErrorHandling = true
							}
						}
					}
				}
			}

			// Check function body for issues
			if node.Body != nil {
				checkFunctionBody(node.Body, result)
			}
		}
		return true
	})

	// Validate required functions exist
	if len(result.Functions) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "no functions defined")
		return result
	}

	// Check for expected main tool function
	foundToolFunc := false
	for _, fn := range result.Functions {
		if fn == expectedFuncName || strings.EqualFold(fn, expectedFuncName) ||
			fn == toPascalCase(expectedToolName) {
			foundToolFunc = true
			break
		}
	}
	if !foundToolFunc && expectedToolName != "" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("expected main tool function '%s' not found", expectedFuncName))
	}

	// Warn if no context parameter
	if !hasContextParam && expectedToolName != "" {
		result.Warnings = append(result.Warnings,
			"main tool function should accept context.Context as first parameter")
	}

	// Warn if no error return
	if !hasErrorReturn && expectedToolName != "" {
		result.Warnings = append(result.Warnings,
			"main tool function should return error as last return value")
	}

	return result
}

// checkFunctionBody analyzes function body for common issues
func checkFunctionBody(body *ast.BlockStmt, result *ValidationResult) {
	hasPanic := false
	hasRecover := false
	hasErrorCheck := false

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Check for panic calls
			if ident, ok := node.Fun.(*ast.Ident); ok {
				if ident.Name == "panic" {
					hasPanic = true
				}
				if ident.Name == "recover" {
					hasRecover = true
				}
			}

			// Check for dangerous function calls
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					// os.Exit is usually bad in library code
					if ident.Name == "os" && sel.Sel.Name == "Exit" {
						result.Warnings = append(result.Warnings,
							"os.Exit() should not be used in tool code - return error instead")
					}
					// log.Fatal is similar
					if ident.Name == "log" && (sel.Sel.Name == "Fatal" || sel.Sel.Name == "Fatalf") {
						result.Warnings = append(result.Warnings,
							"log.Fatal() should not be used in tool code - return error instead")
					}
				}
			}

		case *ast.IfStmt:
			// Check for error handling patterns
			if binary, ok := node.Cond.(*ast.BinaryExpr); ok {
				if binary.Op.String() == "!=" {
					if ident, ok := binary.Y.(*ast.Ident); ok {
						if ident.Name == "nil" {
							hasErrorCheck = true
							result.HasErrorHandling = true
						}
					}
				}
			}
		}
		return true
	})

	// Warn about panic without recover
	if hasPanic && !hasRecover {
		result.Warnings = append(result.Warnings,
			"code contains panic() without recover() - consider proper error handling")
	}

	// Warn about lack of error handling
	if !hasErrorCheck && !hasPanic {
		result.Warnings = append(result.Warnings,
			"no error checking detected - ensure errors are properly handled")
	}
}

// findUsedImports finds all imported package names that are used in the code
func findUsedImports(file *ast.File) map[string]bool {
	used := make(map[string]bool)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok {
				used[ident.Name] = true
			}
		}
		return true
	})

	return used
}

// FunctionSignature represents an extracted function signature
type FunctionSignature struct {
	Name       string
	Params     string
	Returns    string
	IsExported bool
}

// extractFunctionSignatures parses code and extracts function signatures
func extractFunctionSignatures(code string) []FunctionSignature {
	funcs := []FunctionSignature{}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, 0)
	if err != nil {
		return funcs
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			sig := FunctionSignature{
				Name:       fn.Name.Name,
				IsExported: fn.Name.IsExported(),
			}
			funcs = append(funcs, sig)
		}
		return true
	})

	return funcs
}
