package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DataFlowExtractor uses scope-range analysis (program slicing heuristics) to extract
// data flow facts from Go source code. This is NOT a full control flow graph analysis;
// instead, it uses simpler scope-range heuristics that are sufficient for Mangle's
// needs while avoiding compiler-frontend complexity.
//
// The extractor emits facts for:
//   - Variable assignments with type classification (nullable, error)
//   - Guard conditions (nil checks, error checks)
//   - Variable uses (dereferences, method calls)
//   - Function call arguments
//   - Same-scope relationships for dominance analysis
type DataFlowExtractor struct {
	fset *token.FileSet
}

// NewDataFlowExtractor creates a new DataFlowExtractor for program slicing analysis.
func NewDataFlowExtractor() *DataFlowExtractor {
	logging.WorldDebug("Creating new DataFlowExtractor")
	return &DataFlowExtractor{
		fset: token.NewFileSet(),
	}
}

// ExtractDataFlow parses a Go file and extracts data flow facts using program slicing.
// Returns facts for assignments, guards, uses, and call arguments.
func (d *DataFlowExtractor) ExtractDataFlow(path string) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("DataFlowExtractor: analyzing file: %s", filepath.Base(path))

	if !strings.HasSuffix(path, ".go") {
		logging.WorldDebug("DataFlowExtractor: skipping non-Go file: %s", filepath.Base(path))
		return nil, nil
	}

	// Parse with full AST (need bodies for data flow analysis)
	d.fset = token.NewFileSet()
	node, err := parser.ParseFile(d.fset, path, nil, parser.ParseComments)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("DataFlowExtractor: parse failed: %s - %v", path, err)
		return nil, fmt.Errorf("failed to parse file %s: %w", path, err)
	}


	var facts []core.Fact
	ctx := &extractionContext{
		fset:  d.fset,
		path:  path,
		facts: &facts,
	}

	// Process each function/method declaration
	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			ctx.extractFromFunc(decl)
			return false // Don't recurse into function; we handle it ourselves
		}
		return true
	})

	logging.WorldDebug("DataFlowExtractor: analyzed %s - %d facts in %v",
		filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

// extractionContext holds state during extraction within a single file.
type extractionContext struct {
	fset        *token.FileSet
	path        string
	facts       *[]core.Fact
	currentFunc string // Current function name for scoping
	funcStart   int    // Start line of current function
	funcEnd     int    // End line of current function
}

// emit adds a fact to the collection.
func (ctx *extractionContext) emit(fact core.Fact) {
	*ctx.facts = append(*ctx.facts, fact)
}

// extractFromFunc analyzes a function declaration for data flow patterns.
func (ctx *extractionContext) extractFromFunc(decl *ast.FuncDecl) {
	if decl.Body == nil {
		return // Interface method or external declaration
	}

	// Set function context
	ctx.currentFunc = decl.Name.Name
	ctx.funcStart = ctx.fset.Position(decl.Pos()).Line
	ctx.funcEnd = ctx.fset.Position(decl.End()).Line

	logging.WorldDebug("DataFlowExtractor: analyzing function %s (lines %d-%d)",
		ctx.currentFunc, ctx.funcStart, ctx.funcEnd)

	// Emit same_scope facts for all lines in this function
	ctx.emitSameScopeFacts()

	// Walk the function body
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		switch stmt := n.(type) {
		case *ast.AssignStmt:
			ctx.extractAssignment(stmt)

		case *ast.IfStmt:
			ctx.extractIfGuard(stmt)

		case *ast.ExprStmt:
			ctx.extractExprUses(stmt.X)

		case *ast.CallExpr:
			ctx.extractCallArgs(stmt)

		case *ast.StarExpr:
			// Pointer dereference outside of other contexts
			ctx.extractDereference(stmt)

		case *ast.SelectorExpr:
			// Method call or field access
			ctx.extractSelectorUse(stmt)
		}

		return true
	})
}

// emitSameScopeFacts emits same_scope facts for all line pairs in the current function.
// This enables dominance analysis: if line A guards with early return, it dominates all
// subsequent lines in the same scope.
func (ctx *extractionContext) emitSameScopeFacts() {
	// Emit a single fact representing the function scope range
	// Individual line relationships can be derived from this range
	ctx.emit(core.Fact{
		Predicate: "function_scope",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			int64(ctx.funcStart),
			int64(ctx.funcEnd),
		},
	})
}

// extractAssignment handles assignment statements like `x := foo()` or `x, err := foo()`.
func (ctx *extractionContext) extractAssignment(stmt *ast.AssignStmt) {
	line := ctx.fset.Position(stmt.Pos()).Line

	// Handle each LHS variable
	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		varName := ident.Name

		// Skip blank identifier
		if varName == "_" {
			continue
		}

		// Determine the type classification from RHS
		var typeClass string
		if i < len(stmt.Rhs) {
			typeClass = ctx.classifyAssignmentType(stmt.Rhs[i], i, len(stmt.Lhs))
		} else if len(stmt.Rhs) == 1 {
			// Multiple LHS, single RHS (e.g., x, err := foo())
			typeClass = ctx.classifyAssignmentType(stmt.Rhs[0], i, len(stmt.Lhs))
		}

		if typeClass != "" {
			ctx.emit(core.Fact{
				Predicate: "assigns",
				Args: []interface{}{
					core.MangleAtom("/" + varName),
					core.MangleAtom("/" + typeClass),
					ctx.path,
					int64(line),
				},
			})
		}
	}
}

// classifyAssignmentType determines if an assignment is nullable, error, or other.
func (ctx *extractionContext) classifyAssignmentType(expr ast.Expr, index, totalLHS int) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		// Function call - check if it's the last return value (typically error)
		// or if the function name suggests nullable return
		funcName := ctx.extractCallName(e)

		// Common patterns for error returns (last position in multi-return)
		if totalLHS > 1 && index == totalLHS-1 {
			// Last position in multi-return is conventionally error
			return "error"
		}

		// Common patterns for nullable returns
		if ctx.funcReturnsPointer(funcName) {
			return "nullable"
		}

		// First position in multi-return with pointer-returning function
		if totalLHS > 1 && index == 0 {
			return "nullable"
		}

	case *ast.UnaryExpr:
		// &foo creates a pointer (nullable)
		if e.Op.String() == "&" {
			return "nullable"
		}

	case *ast.Ident:
		// Assignment from another identifier - could be nil
		if e.Name == "nil" {
			return "nullable"
		}

	case *ast.CompositeLit:
		// Struct literal or slice literal - check if pointer type
		if _, ok := e.Type.(*ast.StarExpr); ok {
			return "nullable"
		}
	}

	return ""
}

// funcReturnsPointer checks if a function name suggests it returns a pointer.
// This is a heuristic based on common Go naming conventions.
func (ctx *extractionContext) funcReturnsPointer(funcName string) bool {
	// Common constructor patterns that return pointers
	pointerPatterns := []string{
		"New",
		"Create",
		"Make",
		"Open",
		"Get",
		"Find",
		"Load",
		"Parse",
		"Read",
	}

	for _, pattern := range pointerPatterns {
		if strings.HasPrefix(funcName, pattern) {
			return true
		}
	}

	return false
}

// extractCallName extracts the function name from a call expression.
func (ctx *extractionContext) extractCallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// extractIfGuard handles if statements for guard patterns.
func (ctx *extractionContext) extractIfGuard(stmt *ast.IfStmt) {
	// Check for nil check pattern: if x != nil or if x == nil
	if binExpr, ok := stmt.Cond.(*ast.BinaryExpr); ok {
		ctx.extractBinaryGuard(binExpr, stmt)
	}
}

// extractBinaryGuard handles binary comparison guards (nil checks, error checks).
func (ctx *extractionContext) extractBinaryGuard(expr *ast.BinaryExpr, ifStmt *ast.IfStmt) {
	// Check if comparing to nil
	isNilCheck := ctx.isNilComparison(expr)
	isErrCheck := ctx.isErrorCheck(expr)

	if !isNilCheck && !isErrCheck {
		return
	}

	// Get the variable being checked
	varName := ctx.extractComparedVariable(expr)
	if varName == "" {
		return
	}

	guardLine := ctx.fset.Position(ifStmt.Pos()).Line
	blockStart := ctx.fset.Position(ifStmt.Body.Lbrace).Line
	blockEnd := ctx.fset.Position(ifStmt.Body.Rbrace).Line

	// Determine the comparison operator
	isEqualNil := expr.Op.String() == "=="
	isNotEqualNil := expr.Op.String() == "!="

	// Determine if this is an early return pattern
	hasReturn := ctx.hasEarlyReturn(ifStmt.Body)

	if isNilCheck {
		// Early return guard pattern: if x == nil { return ... }
		// This is a guard that returns early when nil, protecting subsequent code
		if isEqualNil && hasReturn {
			ctx.emit(core.Fact{
				Predicate: "guards_return",
				Args: []interface{}{
					core.MangleAtom("/" + varName),
					core.MangleAtom("/nil_check"),
					ctx.path,
					int64(guardLine),
				},
			})
			// Early return dominates all subsequent lines in the function
			ctx.emitDominanceFromEarlyReturn(guardLine)
		}

		// Block guard pattern: if x != nil { ... use x ... }
		// This is a guard that protects usage within the block
		if isNotEqualNil {
			ctx.emit(core.Fact{
				Predicate: "guards_block",
				Args: []interface{}{
					core.MangleAtom("/" + varName),
					core.MangleAtom("/nil_check"),
					ctx.path,
					int64(blockStart),
					int64(blockEnd),
				},
			})
		}
	}

	if isErrCheck {
		// For error checks, the pattern is typically: if err != nil { return err }
		// or: if err != nil { handle error }
		if isNotEqualNil {
			if hasReturn {
				// error_checked_return pattern: if err != nil { return err }
				ctx.emit(core.Fact{
					Predicate: "error_checked_return",
					Args: []interface{}{
						core.MangleAtom("/" + varName),
						ctx.path,
						int64(guardLine),
					},
				})
			} else {
				// error_checked_block pattern: if err != nil { ... handle ... }
				ctx.emit(core.Fact{
					Predicate: "error_checked_block",
					Args: []interface{}{
						core.MangleAtom("/" + varName),
						ctx.path,
						int64(blockStart),
						int64(blockEnd),
					},
				})
			}
		}
	}
}

// isNilComparison checks if a binary expression compares to nil.
func (ctx *extractionContext) isNilComparison(expr *ast.BinaryExpr) bool {
	op := expr.Op.String()
	if op != "==" && op != "!=" {
		return false
	}

	// Check if either side is nil
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "nil" {
		return true
	}
	if ident, ok := expr.Y.(*ast.Ident); ok && ident.Name == "nil" {
		return true
	}

	return false
}

// isErrorCheck checks if a binary expression is an error check (err != nil).
func (ctx *extractionContext) isErrorCheck(expr *ast.BinaryExpr) bool {
	op := expr.Op.String()
	if op != "==" && op != "!=" {
		return false
	}

	// Check if comparing something named "err" or "error" to nil
	varName := ctx.extractComparedVariable(expr)
	if varName == "" {
		return false
	}

	// Check for error variable naming conventions
	isErrorVar := varName == "err" ||
		strings.HasSuffix(varName, "Err") ||
		strings.HasSuffix(varName, "Error") ||
		strings.HasPrefix(varName, "err")

	// Must also be comparing to nil
	return isErrorVar && ctx.isNilComparison(expr)
}

// extractComparedVariable extracts the variable name from a comparison expression.
func (ctx *extractionContext) extractComparedVariable(expr *ast.BinaryExpr) string {
	// Try X side (excluding nil)
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name != "nil" {
		return ident.Name
	}
	// Try Y side (excluding nil)
	if ident, ok := expr.Y.(*ast.Ident); ok && ident.Name != "nil" {
		return ident.Name
	}
	return ""
}

// hasEarlyReturn checks if a block contains an early return statement.
func (ctx *extractionContext) hasEarlyReturn(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}

	// Check if the block ends with a return statement
	lastStmt := block.List[len(block.List)-1]
	_, isReturn := lastStmt.(*ast.ReturnStmt)
	return isReturn
}

// emitDominanceFromEarlyReturn emits facts indicating that an early return guard
// dominates all subsequent lines in the function.
func (ctx *extractionContext) emitDominanceFromEarlyReturn(guardLine int) {
	// Emit a dominates fact: all lines after guardLine are dominated by this guard
	ctx.emit(core.Fact{
		Predicate: "guard_dominates",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			int64(guardLine),
			int64(ctx.funcEnd),
		},
	})
}

// extractExprUses extracts variable uses from an expression statement.
func (ctx *extractionContext) extractExprUses(expr ast.Expr) {
	ast.Inspect(expr, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			ctx.extractSelectorUse(e)
		case *ast.StarExpr:
			ctx.extractDereference(e)
		}
		return true
	})
}

// extractSelectorUse handles selector expressions like foo.Bar() or foo.Field.
func (ctx *extractionContext) extractSelectorUse(expr *ast.SelectorExpr) {
	// Extract the base variable
	varName := ""
	switch x := expr.X.(type) {
	case *ast.Ident:
		varName = x.Name
	case *ast.CallExpr:
		// Chain like foo().Bar - the result of foo() is used
		varName = ctx.extractCallName(x)
	}

	if varName == "" || varName == "_" {
		return
	}

	line := ctx.fset.Position(expr.Pos()).Line

	ctx.emit(core.Fact{
		Predicate: "uses",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			core.MangleAtom("/" + varName),
			int64(line),
		},
	})
}

// extractDereference handles pointer dereferences like *foo.
func (ctx *extractionContext) extractDereference(expr *ast.StarExpr) {
	// Extract the dereferenced variable
	if ident, ok := expr.X.(*ast.Ident); ok {
		varName := ident.Name
		if varName == "" || varName == "_" {
			return
		}

		line := ctx.fset.Position(expr.Pos()).Line

		ctx.emit(core.Fact{
			Predicate: "uses",
			Args: []interface{}{
				ctx.path,
				core.MangleAtom("/" + ctx.currentFunc),
				core.MangleAtom("/" + varName),
				int64(line),
			},
		})
	}
}

// extractCallArgs handles function call arguments.
func (ctx *extractionContext) extractCallArgs(call *ast.CallExpr) {
	line := ctx.fset.Position(call.Pos()).Line

	// Generate a unique callsite identifier based on function and line
	funcName := ctx.extractCallName(call)
	callsiteID := fmt.Sprintf("%s:%s:%d", ctx.currentFunc, funcName, line)

	// Extract each argument
	for i, arg := range call.Args {
		varName := ""
		switch a := arg.(type) {
		case *ast.Ident:
			varName = a.Name
		case *ast.UnaryExpr:
			// &var or *var
			if ident, ok := a.X.(*ast.Ident); ok {
				varName = ident.Name
			}
		case *ast.SelectorExpr:
			// pkg.Var or struct.Field
			if ident, ok := a.X.(*ast.Ident); ok {
				varName = ident.Name + "." + a.Sel.Name
			}
		}

		if varName == "" || varName == "_" {
			continue
		}

		ctx.emit(core.Fact{
			Predicate: "call_arg",
			Args: []interface{}{
				core.MangleAtom("/" + callsiteID),
				int64(i),
				core.MangleAtom("/" + varName),
				ctx.path,
				int64(line),
			},
		})
	}
}

// ExtractDataFlowForDirectory extracts data flow facts from all Go files in a directory.
func (d *DataFlowExtractor) ExtractDataFlowForDirectory(dir string) ([]core.Fact, error) {
	start := time.Now()
	logging.World("DataFlowExtractor: analyzing directory: %s", dir)

	var allFacts []core.Fact
	fileCount := 0
	errorCount := 0

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip hidden directories and vendor
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process Go files (skip test files for now)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		facts, extractErr := d.ExtractDataFlow(path)
		if extractErr != nil {
			logging.Get(logging.CategoryWorld).Warn("DataFlowExtractor: skipping %s: %v",
				filepath.Base(path), extractErr)
			errorCount++
			return nil
		}

		allFacts = append(allFacts, facts...)
		fileCount++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	logging.World("DataFlowExtractor: analyzed %d files (%d errors), %d total facts in %v",
		fileCount, errorCount, len(allFacts), time.Since(start))

	return allFacts, nil
}

// DataFlowSummary provides aggregated statistics about extracted data flow.
type DataFlowSummary struct {
	TotalFacts           int
	AssignmentsFacts     int
	NullableAssignments  int
	ErrorAssignments     int
	GuardsBlockFacts     int
	GuardsReturnFacts    int
	ErrorCheckedFacts    int
	UsesFacts            int
	CallArgFacts         int
	FunctionScopeFacts   int
	GuardDominatesFacts  int
}

// SummarizeDataFlow analyzes extracted facts and returns a summary.
func SummarizeDataFlow(facts []core.Fact) DataFlowSummary {
	summary := DataFlowSummary{
		TotalFacts: len(facts),
	}

	for _, fact := range facts {
		switch fact.Predicate {
		case "assigns":
			summary.AssignmentsFacts++
			if len(fact.Args) >= 2 {
				if typeClass, ok := fact.Args[1].(core.MangleAtom); ok {
					switch string(typeClass) {
					case "/nullable":
						summary.NullableAssignments++
					case "/error":
						summary.ErrorAssignments++
					}
				}
			}
		case "guards_block":
			summary.GuardsBlockFacts++
		case "guards_return":
			summary.GuardsReturnFacts++
		case "error_checked_block", "error_checked_return":
			summary.ErrorCheckedFacts++
		case "uses":
			summary.UsesFacts++
		case "call_arg":
			summary.CallArgFacts++
		case "function_scope":
			summary.FunctionScopeFacts++
		case "guard_dominates":
			summary.GuardDominatesFacts++
		}
	}

	return summary
}
