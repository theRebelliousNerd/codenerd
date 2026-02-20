package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// HOLOGRAPHIC CONTEXT PROVIDER
// =============================================================================
// Provides rich, multi-dimensional context for AI agents analyzing code.
// This is the "X-Ray Vision" system that lets agents see beyond the single file.

// HolographicContext represents the complete context for understanding a code file.
// It aggregates package-level, architectural, and semantic information.
type HolographicContext struct {
	// Target file being analyzed
	TargetFile string `json:"target_file"`
	TargetPkg  string `json:"target_package"`

	// Package Scope (sibling files in same package)
	PackageSiblings   []string            `json:"package_siblings"`
	PackageSignatures []SymbolSignature   `json:"package_signatures"` // Exported + unexported symbols
	PackageTypes      []TypeDefinition    `json:"package_types"`      // Struct/interface definitions
	PackageConstants  []ConstDefinition   `json:"package_constants"`  // const/var blocks
	PackageImports    map[string][]string `json:"package_imports"`    // File -> imports

	// Architectural Layer (where in the system)
	Layer         string `json:"layer"`          // e.g., "core", "api", "data", "cmd"
	Module        string `json:"module"`         // e.g., "campaign", "shards", "world"
	Role          string `json:"role"`           // e.g., "service", "handler", "model", "util"
	SystemPurpose string `json:"system_purpose"` // High-level purpose deduced from patterns

	// Dependency Context (import/export relationships)
	DirectImports   []ImportInfo `json:"direct_imports"`   // What this file imports
	DirectImporters []string     `json:"direct_importers"` // Files that import this package
	ExternalDeps    []string     `json:"external_deps"`    // Third-party dependencies

	// Semantic Relationships (from knowledge graph)
	RelatedEntities []RelatedEntity `json:"related_entities"` // Semantically related code
	CallGraph       []CallEdge      `json:"call_graph"`       // Who calls what

	// Code Quality Signals
	TestCoverage    float64  `json:"test_coverage"`    // If known from facts
	HasTests        bool     `json:"has_tests"`        // Does a _test.go file exist?
	TODOCount       int      `json:"todo_count"`       // Number of TODO/FIXME comments
	ComplexityHints []string `json:"complexity_hints"` // High complexity warnings

	// Impact-Aware Priority Context (from Mangle impact analysis)
	ImpactPriority     int                 `json:"impact_priority"`     // Overall priority from Mangle analysis
	PrioritizedCallers []PrioritizedCaller `json:"prioritized_callers"` // Callers sorted by impact priority
}

// PrioritizedCaller represents a caller function with impact analysis metadata.
// Used by the impact-aware context builder to provide targeted review context.
type PrioritizedCaller struct {
	Name     string `json:"name"`     // Function/method name
	File     string `json:"file"`     // Source file path
	Body     string `json:"body"`     // Function body (may be truncated)
	Priority int    `json:"priority"` // Priority from context_priority query (higher = more important)
	Depth    int    `json:"depth"`    // Distance in call graph (1 = direct caller)
}

// SymbolSignature represents a function or method signature available in package scope.
type SymbolSignature struct {
	Name       string `json:"name"`
	Receiver   string `json:"receiver,omitempty"`    // For methods: "*Foo" or "Foo"
	Params     string `json:"params"`                // "(ctx context.Context, id string)"
	Returns    string `json:"returns"`               // "(error)" or "(string, error)"
	File       string `json:"file"`                  // Which file defines this
	Line       int    `json:"line"`                  // Line number
	Exported   bool   `json:"exported"`              // Starts with uppercase?
	DocComment string `json:"doc_comment,omitempty"` // First line of doc comment
}

// TypeDefinition represents a struct or interface in the package.
type TypeDefinition struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`              // "struct", "interface", "alias"
	Fields   []string `json:"fields,omitempty"`  // For structs: field signatures
	Methods  []string `json:"methods,omitempty"` // For interfaces: method signatures
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Exported bool     `json:"exported"`
}

// ConstDefinition represents a const or var in the package.
type ConstDefinition struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Value    string `json:"value,omitempty"` // For simple literals
	File     string `json:"file"`
	IsConst  bool   `json:"is_const"` // true for const, false for var
	Exported bool   `json:"exported"`
}

// ImportInfo represents an import with alias information.
type ImportInfo struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
}

// RelatedEntity represents a semantically related code entity.
type RelatedEntity struct {
	EntityID string `json:"entity_id"`
	Relation string `json:"relation"` // "calls", "implements", "extends", "uses"
	File     string `json:"file"`
}

// CallEdge represents a caller->callee relationship.
type CallEdge struct {
	Caller string `json:"caller"`
	Callee string `json:"callee"`
}

// HolographicProvider creates rich context for code analysis.
type HolographicProvider struct {
	kernel  *core.RealKernel
	workDir string
}

// NewHolographicProvider creates a new holographic context provider.
func NewHolographicProvider(kernel *core.RealKernel, workDir string) *HolographicProvider {
	return &HolographicProvider{
		kernel:  kernel,
		workDir: workDir,
	}
}

// GetContext generates complete holographic context for a file.
func (h *HolographicProvider) GetContext(filePath string) (*HolographicContext, error) {
	logging.WorldDebug("HolographicProvider: generating context for %s", filepath.Base(filePath))

	ctx := &HolographicContext{
		TargetFile:     filePath,
		PackageImports: make(map[string][]string),
	}

	// Detect language and route to appropriate handler
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		if err := h.buildGoContext(ctx, filePath); err != nil {
			logging.WorldDebug("HolographicProvider: Go context failed: %v", err)
			// Continue with partial context
		}
	default:
		// For non-Go files, provide basic architectural context
		h.buildBasicContext(ctx, filePath)
	}

	// Add architectural analysis (works for any language)
	h.analyzeArchitecture(ctx, filePath)

	// Query knowledge graph for relationships
	h.queryRelationships(ctx, filePath)

	// Check for test file existence
	h.checkTestCoverage(ctx, filePath)

	logging.WorldDebug("HolographicProvider: context complete for %s - %d siblings, %d signatures",
		filepath.Base(filePath), len(ctx.PackageSiblings), len(ctx.PackageSignatures))

	return ctx, nil
}

// buildGoContext builds package-level context for Go files.
func (h *HolographicProvider) buildGoContext(ctx *HolographicContext, filePath string) error {
	// Get the directory containing this file
	dir := filepath.Dir(filePath)

	// Find all Go files in the same package
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Include .go files but skip test files for signature extraction
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			fullPath := filepath.Join(dir, name)
			if fullPath != filePath {
				ctx.PackageSiblings = append(ctx.PackageSiblings, fullPath)
			}
			goFiles = append(goFiles, fullPath)
		}
	}

	// Parse all files in the package to extract signatures
	fset := token.NewFileSet()
	for _, goFile := range goFiles {
		if err := h.extractGoSignatures(ctx, fset, goFile); err != nil {
			logging.WorldDebug("HolographicProvider: failed to parse %s: %v", goFile, err)
			// Continue with other files
		}
	}

	// Extract package name from target file
	if node, err := parser.ParseFile(fset, filePath, nil, parser.PackageClauseOnly); err == nil {
		ctx.TargetPkg = node.Name.Name
	}

	return nil
}

// extractGoSignatures parses a Go file and extracts function/type/const signatures.
func (h *HolographicProvider) extractGoSignatures(ctx *HolographicContext, fset *token.FileSet, filePath string) error {
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	fileName := filepath.Base(filePath)

	// Extract imports
	var imports []string
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		imports = append(imports, importPath)
	}
	ctx.PackageImports[fileName] = imports

	// Walk AST for definitions
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			sig := h.extractFuncSignature(fset, x, fileName)
			ctx.PackageSignatures = append(ctx.PackageSignatures, sig)

		case *ast.GenDecl:
			switch x.Tok {
			case token.TYPE:
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						typeDef := h.extractTypeDefinition(fset, ts, x, fileName)
						ctx.PackageTypes = append(ctx.PackageTypes, typeDef)
					}
				}
			case token.CONST, token.VAR:
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range vs.Names {
							constDef := ConstDefinition{
								Name:     name.Name,
								File:     fileName,
								IsConst:  x.Tok == token.CONST,
								Exported: ast.IsExported(name.Name),
							}
							if vs.Type != nil {
								constDef.Type = formatNode(fset, vs.Type)
							}
							ctx.PackageConstants = append(ctx.PackageConstants, constDef)
						}
					}
				}
			}
		}
		return true
	})

	return nil
}

// extractFuncSignature extracts a function's signature.
func (h *HolographicProvider) extractFuncSignature(fset *token.FileSet, fn *ast.FuncDecl, fileName string) SymbolSignature {
	sig := SymbolSignature{
		Name:     fn.Name.Name,
		File:     fileName,
		Line:     fset.Position(fn.Pos()).Line,
		Exported: ast.IsExported(fn.Name.Name),
	}

	// Receiver for methods
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sig.Receiver = formatNode(fset, fn.Recv.List[0].Type)
	}

	// Parameters
	if fn.Type.Params != nil {
		sig.Params = formatFieldList(fset, fn.Type.Params)
	}

	// Return types
	if fn.Type.Results != nil {
		sig.Returns = formatFieldList(fset, fn.Type.Results)
	}

	// Doc comment (first line only)
	if fn.Doc != nil && len(fn.Doc.List) > 0 {
		text := strings.TrimPrefix(fn.Doc.List[0].Text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSpace(text)
		if len(text) > 100 {
			text = text[:100] + "..."
		}
		sig.DocComment = text
	}

	return sig
}

// extractTypeDefinition extracts a type's definition.
func (h *HolographicProvider) extractTypeDefinition(fset *token.FileSet, ts *ast.TypeSpec, gd *ast.GenDecl, fileName string) TypeDefinition {
	typeDef := TypeDefinition{
		Name:     ts.Name.Name,
		File:     fileName,
		Line:     fset.Position(ts.Pos()).Line,
		Exported: ast.IsExported(ts.Name.Name),
	}

	switch t := ts.Type.(type) {
	case *ast.StructType:
		typeDef.Kind = "struct"
		if t.Fields != nil {
			for _, field := range t.Fields.List {
				fieldType := formatNode(fset, field.Type)
				for _, name := range field.Names {
					typeDef.Fields = append(typeDef.Fields, fmt.Sprintf("%s %s", name.Name, fieldType))
				}
				// Embedded field
				if len(field.Names) == 0 {
					typeDef.Fields = append(typeDef.Fields, fieldType)
				}
			}
		}
	case *ast.InterfaceType:
		typeDef.Kind = "interface"
		if t.Methods != nil {
			for _, method := range t.Methods.List {
				if len(method.Names) > 0 {
					methodSig := formatNode(fset, method.Type)
					typeDef.Methods = append(typeDef.Methods, fmt.Sprintf("%s%s", method.Names[0].Name, methodSig))
				}
			}
		}
	default:
		typeDef.Kind = "alias"
	}

	return typeDef
}

// analyzeArchitecture deduces architectural layer and role from file path patterns.
func (h *HolographicProvider) analyzeArchitecture(ctx *HolographicContext, filePath string) {
	// Normalize path separators
	normalPath := strings.ReplaceAll(filePath, "\\", "/")
	parts := strings.Split(normalPath, "/")

	// Detect layer
	for i, part := range parts {
		switch part {
		case "cmd":
			ctx.Layer = "command"
			if i+1 < len(parts) {
				ctx.Module = parts[i+1]
			}
		case "internal":
			ctx.Layer = "internal"
			if i+1 < len(parts) {
				ctx.Module = parts[i+1]
			}
		case "pkg":
			ctx.Layer = "package"
			if i+1 < len(parts) {
				ctx.Module = parts[i+1]
			}
		case "api", "apis":
			ctx.Layer = "api"
		case "web", "http", "handlers":
			ctx.Layer = "transport"
		case "store", "storage", "db", "database", "repository":
			ctx.Layer = "data"
		case "models", "entities", "domain":
			ctx.Layer = "domain"
		}
	}

	// Detect role from filename patterns
	baseName := filepath.Base(filePath)
	baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))

	switch {
	case strings.HasSuffix(baseName, "_test"):
		ctx.Role = "test"
	case strings.HasSuffix(baseName, "_handler") || strings.HasSuffix(baseName, "handler"):
		ctx.Role = "handler"
	case strings.HasSuffix(baseName, "_service") || strings.HasSuffix(baseName, "service"):
		ctx.Role = "service"
	case strings.HasSuffix(baseName, "_repo") || strings.HasSuffix(baseName, "repository"):
		ctx.Role = "repository"
	case strings.HasSuffix(baseName, "_model") || strings.HasSuffix(baseName, "models"):
		ctx.Role = "model"
	case baseName == "types" || baseName == "models":
		ctx.Role = "types"
	case baseName == "utils" || baseName == "helpers" || baseName == "common":
		ctx.Role = "utility"
	case baseName == "config" || baseName == "settings":
		ctx.Role = "config"
	case baseName == "main":
		ctx.Role = "entrypoint"
	default:
		ctx.Role = "implementation"
	}

	// Deduce system purpose from module + role
	if ctx.Module != "" {
		ctx.SystemPurpose = fmt.Sprintf("%s %s component", ctx.Module, ctx.Role)
	}
}

// queryRelationships queries the kernel for semantic relationships.
func (h *HolographicProvider) queryRelationships(ctx *HolographicContext, filePath string) {
	if h.kernel == nil {
		return
	}

	// Query code_defines for symbols in this file
	facts, err := h.kernel.Query("code_defines")
	if err != nil {
		return
	}

	normalPath := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))
	var fileSymbols []string

	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}
		factFile, _ := fact.Args[0].(string)
		if strings.Contains(strings.ToLower(factFile), normalPath) || strings.Contains(normalPath, strings.ToLower(factFile)) {
			if sym, ok := fact.Args[1].(string); ok {
				fileSymbols = append(fileSymbols, sym)
			}
		}
	}

	// Query code_calls to build call graph for these symbols
	callFacts, err := h.kernel.Query("code_calls")
	if err != nil {
		return
	}

	for _, fact := range callFacts {
		if len(fact.Args) < 2 {
			continue
		}
		caller, _ := fact.Args[0].(string)
		callee, _ := fact.Args[1].(string)

		// Check if caller or callee is in our file
		for _, sym := range fileSymbols {
			if strings.Contains(caller, sym) || strings.Contains(callee, sym) {
				ctx.CallGraph = append(ctx.CallGraph, CallEdge{
					Caller: caller,
					Callee: callee,
				})
			}
		}
	}
}

// checkTestCoverage checks if a corresponding test file exists.
func (h *HolographicProvider) checkTestCoverage(ctx *HolographicContext, filePath string) {
	if strings.HasSuffix(filePath, "_test.go") {
		ctx.HasTests = true
		return
	}

	// Check for corresponding _test.go file
	ext := filepath.Ext(filePath)
	testFile := strings.TrimSuffix(filePath, ext) + "_test" + ext
	if _, err := os.Stat(testFile); err == nil {
		ctx.HasTests = true
	}
}

// buildBasicContext provides minimal context for non-Go files.
func (h *HolographicProvider) buildBasicContext(ctx *HolographicContext, filePath string) {
	// Just set up basic file info
	dir := filepath.Dir(filePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	ext := filepath.Ext(filePath)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ext {
			fullPath := filepath.Join(dir, entry.Name())
			if fullPath != filePath {
				ctx.PackageSiblings = append(ctx.PackageSiblings, fullPath)
			}
		}
	}
}

// =============================================================================
// FORMATTING HELPERS
// =============================================================================

// formatNode formats an AST node as a string.
func formatNode(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder
	switch n := node.(type) {
	case *ast.Ident:
		sb.WriteString(n.Name)
	case *ast.StarExpr:
		sb.WriteString("*")
		sb.WriteString(formatNode(fset, n.X))
	case *ast.SelectorExpr:
		sb.WriteString(formatNode(fset, n.X))
		sb.WriteString(".")
		sb.WriteString(n.Sel.Name)
	case *ast.ArrayType:
		sb.WriteString("[]")
		sb.WriteString(formatNode(fset, n.Elt))
	case *ast.MapType:
		sb.WriteString("map[")
		sb.WriteString(formatNode(fset, n.Key))
		sb.WriteString("]")
		sb.WriteString(formatNode(fset, n.Value))
	case *ast.ChanType:
		switch n.Dir {
		case ast.SEND:
			sb.WriteString("chan<- ")
		case ast.RECV:
			sb.WriteString("<-chan ")
		default:
			sb.WriteString("chan ")
		}
		sb.WriteString(formatNode(fset, n.Value))
	case *ast.FuncType:
		sb.WriteString("func")
		sb.WriteString(formatFieldList(fset, n.Params))
		if n.Results != nil && len(n.Results.List) > 0 {
			sb.WriteString(" ")
			sb.WriteString(formatFieldList(fset, n.Results))
		}
	case *ast.InterfaceType:
		sb.WriteString("interface{}")
	case *ast.Ellipsis:
		sb.WriteString("...")
		sb.WriteString(formatNode(fset, n.Elt))
	default:
		// Fallback: just note the type
		sb.WriteString("?")
	}
	return sb.String()
}

// formatFieldList formats a field list (params or returns).
func formatFieldList(fset *token.FileSet, fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return "()"
	}

	var parts []string
	for _, field := range fl.List {
		typeStr := formatNode(fset, field.Type)
		if len(field.Names) == 0 {
			// Unnamed parameter/return
			parts = append(parts, typeStr)
		} else {
			// Named parameters
			for _, name := range field.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}
	}

	return "(" + strings.Join(parts, ", ") + ")"
}

// =============================================================================
// CONTEXT FORMATTING FOR LLM PROMPTS
// =============================================================================

// FormatForPrompt formats the holographic context for LLM injection.
func (ctx *HolographicContext) FormatForPrompt() string {
	var sb strings.Builder

	sb.WriteString("\n## Package Context\n")

	// Package info
	if ctx.TargetPkg != "" {
		sb.WriteString(fmt.Sprintf("Package: `%s`\n", ctx.TargetPkg))
	}

	// Sibling files
	if len(ctx.PackageSiblings) > 0 {
		sb.WriteString(fmt.Sprintf("Sibling files in package: %d\n", len(ctx.PackageSiblings)))
		for _, sib := range ctx.PackageSiblings {
			sb.WriteString(fmt.Sprintf("  - %s\n", filepath.Base(sib)))
		}
	}

	// Available functions in package scope
	if len(ctx.PackageSignatures) > 0 {
		sb.WriteString("\n### Functions Available in Package Scope\n")
		sb.WriteString("These are defined in sibling files and can be called without import:\n```go\n")

		// Sort by exported first, then alphabetically
		sort.Slice(ctx.PackageSignatures, func(i, j int) bool {
			if ctx.PackageSignatures[i].Exported != ctx.PackageSignatures[j].Exported {
				return ctx.PackageSignatures[i].Exported
			}
			return ctx.PackageSignatures[i].Name < ctx.PackageSignatures[j].Name
		})

		for _, sig := range ctx.PackageSignatures {
			if sig.Receiver != "" {
				sb.WriteString(fmt.Sprintf("func (%s) %s%s %s  // %s\n",
					sig.Receiver, sig.Name, sig.Params, sig.Returns, sig.File))
			} else {
				sb.WriteString(fmt.Sprintf("func %s%s %s  // %s\n",
					sig.Name, sig.Params, sig.Returns, sig.File))
			}
		}
		sb.WriteString("```\n")
	}

	// Types in package
	if len(ctx.PackageTypes) > 0 {
		sb.WriteString("\n### Types Defined in Package\n```go\n")
		for _, t := range ctx.PackageTypes {
			switch t.Kind {
			case "struct":
				sb.WriteString(fmt.Sprintf("type %s struct { ... }  // %s:%d, %d fields\n",
					t.Name, t.File, t.Line, len(t.Fields)))
			case "interface":
				sb.WriteString(fmt.Sprintf("type %s interface { ... }  // %s:%d, %d methods\n",
					t.Name, t.File, t.Line, len(t.Methods)))
			default:
				sb.WriteString(fmt.Sprintf("type %s = ...  // %s:%d\n", t.Name, t.File, t.Line))
			}
		}
		sb.WriteString("```\n")
	}

	// Constants
	exportedConsts := make([]ConstDefinition, 0)
	for _, c := range ctx.PackageConstants {
		if c.Exported {
			exportedConsts = append(exportedConsts, c)
		}
	}
	if len(exportedConsts) > 0 && len(exportedConsts) < 20 {
		sb.WriteString("\n### Exported Constants/Variables\n```go\n")
		for _, c := range exportedConsts {
			kind := "var"
			if c.IsConst {
				kind = "const"
			}
			sb.WriteString(fmt.Sprintf("%s %s  // %s\n", kind, c.Name, c.File))
		}
		sb.WriteString("```\n")
	}

	// Architectural context
	sb.WriteString("\n## Architectural Context\n")
	if ctx.Layer != "" {
		sb.WriteString(fmt.Sprintf("- Layer: %s\n", ctx.Layer))
	}
	if ctx.Module != "" {
		sb.WriteString(fmt.Sprintf("- Module: %s\n", ctx.Module))
	}
	if ctx.Role != "" {
		sb.WriteString(fmt.Sprintf("- Role: %s\n", ctx.Role))
	}
	if ctx.SystemPurpose != "" {
		sb.WriteString(fmt.Sprintf("- Purpose: %s\n", ctx.SystemPurpose))
	}
	if ctx.HasTests {
		sb.WriteString("- Has corresponding test file: yes\n")
	}

	// Call graph (if populated)
	if len(ctx.CallGraph) > 0 && len(ctx.CallGraph) < 20 {
		sb.WriteString("\n### Call Relationships\n")
		for _, edge := range ctx.CallGraph {
			sb.WriteString(fmt.Sprintf("- %s → %s\n", edge.Caller, edge.Callee))
		}
	}

	return sb.String()
}

// FormatSignaturesCompact returns a compact signature list for context injection.
func (ctx *HolographicContext) FormatSignaturesCompact() string {
	if len(ctx.PackageSignatures) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Package-scope symbols:\n")

	for _, sig := range ctx.PackageSignatures {
		if sig.Receiver != "" {
			sb.WriteString(fmt.Sprintf("  (%s).%s%s%s [%s]\n",
				sig.Receiver, sig.Name, sig.Params, sig.Returns, sig.File))
		} else {
			sb.WriteString(fmt.Sprintf("  %s%s%s [%s]\n",
				sig.Name, sig.Params, sig.Returns, sig.File))
		}
	}

	return sb.String()
}

// CountTODOs counts TODO/FIXME comments in file content.
func CountTODOs(content string) int {
	todoPattern := regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX|BUG):?`)
	matches := todoPattern.FindAllString(content, -1)
	return len(matches)
}

// =============================================================================
// IMPACT-AWARE CONTEXT BUILDING
// =============================================================================
// These methods integrate Mangle's impact analysis with holographic context,
// providing prioritized caller information for targeted code review.

// maxPrioritizedCallers limits the number of callers included to prevent prompt explosion.
const maxPrioritizedCallers = 10

// maxCallerBodyLines limits individual caller body size.
const maxCallerBodyLines = 50

// BuildWithImpactPriorities builds holographic context enhanced with impact analysis from the kernel.
// It queries for context_priority facts to prioritize which callers to include,
// then fetches their bodies for targeted review context.
//
// The method:
// 1. Builds standard holographic context via GetContext
// 2. Queries kernel for context_priority_file facts
// 3. Fetches caller bodies for prioritized functions
// 4. Sorts by priority and limits to top N callers
// 5. Returns enhanced context ready for LLM injection
func (h *HolographicProvider) BuildWithImpactPriorities(ctx context.Context, file string) (*HolographicContext, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	logging.WorldDebug("BuildWithImpactPriorities: starting for %s", filepath.Base(file))

	// 1. Build standard holographic context
	hc, err := h.GetContext(file)
	if err != nil {
		return nil, fmt.Errorf("failed to build base context: %w", err)
	}

	// 2. If no kernel, return standard context (graceful degradation)
	if h.kernel == nil {
		logging.WorldDebug("BuildWithImpactPriorities: no kernel available, returning standard context")
		return hc, nil
	}

	// 3. Query kernel for context_priority_file facts
	// Format: context_priority_file(File, Func, Priority)
	priorityFacts, err := h.kernel.Query("context_priority_file")
	if err != nil {
		logging.WorldDebug("BuildWithImpactPriorities: context_priority_file query failed: %v", err)
		// Fall back to relevant_context_file
		priorityFacts, err = h.kernel.Query("relevant_context_file")
		if err != nil {
			logging.WorldDebug("BuildWithImpactPriorities: relevant_context_file query also failed: %v", err)
			return hc, nil // Return standard context
		}
	}

	if len(priorityFacts) == 0 {
		logging.WorldDebug("BuildWithImpactPriorities: no priority facts found, returning standard context")
		return hc, nil
	}

	// 4. Parse facts and build prioritized callers
	callers := h.parsePriorityFacts(priorityFacts)
	if len(callers) == 0 {
		return hc, nil
	}

	// 5. Fetch function bodies for prioritized callers
	for i := range callers {
		select {
		case <-ctx.Done():
			return hc, ctx.Err()
		default:
		}

		body, fetchErr := h.fetchFunctionBody(callers[i].File, callers[i].Name)
		if fetchErr != nil {
			logging.WorldDebug("BuildWithImpactPriorities: could not fetch body for %s:%s: %v",
				callers[i].File, callers[i].Name, fetchErr)
			continue
		}
		callers[i].Body = body
	}

	// 6. Sort by priority (descending) then by depth (ascending)
	sort.Slice(callers, func(i, j int) bool {
		if callers[i].Priority != callers[j].Priority {
			return callers[i].Priority > callers[j].Priority
		}
		return callers[i].Depth < callers[j].Depth
	})

	// 7. Limit to prevent context explosion
	if len(callers) > maxPrioritizedCallers {
		logging.WorldDebug("BuildWithImpactPriorities: limiting callers from %d to %d",
			len(callers), maxPrioritizedCallers)
		callers = callers[:maxPrioritizedCallers]
	}

	// 8. Calculate overall impact priority (max of all callers)
	maxPriority := 0
	for _, c := range callers {
		if c.Priority > maxPriority {
			maxPriority = c.Priority
		}
	}

	hc.PrioritizedCallers = callers
	hc.ImpactPriority = maxPriority

	logging.WorldDebug("BuildWithImpactPriorities: found %d prioritized callers (max priority: %d)",
		len(callers), maxPriority)

	return hc, nil
}

// parsePriorityFacts extracts PrioritizedCaller structs from Mangle query results.
// Handles multiple fact formats:
// - context_priority_file(File, Func, Priority)
// - relevant_context_file(File)
// - impact_graph(Target, Caller, Depth)
func (h *HolographicProvider) parsePriorityFacts(facts []core.Fact) []PrioritizedCaller {
	callers := make([]PrioritizedCaller, 0, len(facts))
	seen := make(map[string]bool)

	for _, fact := range facts {
		var caller PrioritizedCaller
		caller.Depth = 1     // Default depth
		caller.Priority = 50 // Default medium priority

		switch fact.Predicate {
		case "context_priority_file":
			// Format: context_priority_file(File, Func, Priority)
			if len(fact.Args) < 3 {
				continue
			}
			caller.File = h.stringArg(fact.Args[0])
			caller.Name = h.stringArg(fact.Args[1])
			caller.Priority = h.intArg(fact.Args[2], 50)

		case "relevant_context_file":
			// Format: relevant_context_file(File)
			if len(fact.Args) < 1 {
				continue
			}
			caller.File = h.stringArg(fact.Args[0])
			// Name will be discovered when fetching body

		case "impact_graph":
			// Format: impact_graph(Target, Caller, Depth)
			if len(fact.Args) < 3 {
				continue
			}
			caller.Name = h.stringArg(fact.Args[1])
			caller.Depth = h.intArg(fact.Args[2], 1)
			// File will need to be looked up from code_defines

		case "context_priority":
			// Format: context_priority(FactID, Priority)
			if len(fact.Args) < 2 {
				continue
			}
			caller.File = h.stringArg(fact.Args[0])
			caller.Priority = h.priorityAtomToInt(h.stringArg(fact.Args[1]))

		default:
			// Generic fallback: try to extract file and function
			if len(fact.Args) >= 2 {
				caller.File = h.stringArg(fact.Args[0])
				caller.Name = h.stringArg(fact.Args[1])
			} else if len(fact.Args) >= 1 {
				caller.File = h.stringArg(fact.Args[0])
			} else {
				continue
			}
		}

		// Skip if we don't have at least a file
		if caller.File == "" {
			continue
		}

		// Deduplicate by file:name key
		key := fmt.Sprintf("%s:%s", caller.File, caller.Name)
		if seen[key] {
			continue
		}
		seen[key] = true

		callers = append(callers, caller)
	}

	return callers
}

// stringArg safely extracts a string from an interface{} argument.
func (h *HolographicProvider) stringArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// intArg safely extracts an int from an interface{} argument.
func (h *HolographicProvider) intArg(arg interface{}, defaultVal int) int {
	switch v := arg.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		// Try to parse priority atoms
		return h.priorityAtomToInt(v)
	default:
		return defaultVal
	}
}

// priorityAtomToInt converts Mangle priority atoms to integer values.
func (h *HolographicProvider) priorityAtomToInt(atom string) int {
	// Strip leading / for Mangle name constants
	atom = strings.TrimPrefix(atom, "/")
	atom = strings.ToLower(atom)

	switch atom {
	case "critical", "highest":
		return 100
	case "high":
		return 80
	case "medium", "normal":
		return 50
	case "low":
		return 25
	case "lowest":
		return 10
	default:
		return 50 // Default medium
	}
}

// fetchFunctionBody retrieves the body of a function from a file.
// Uses AST parsing for Go files, falls back to regex for other languages.
func (h *HolographicProvider) fetchFunctionBody(file, funcName string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("empty file path")
	}

	// Resolve relative paths against workDir
	resolvedPath := file
	if !filepath.IsAbs(file) && h.workDir != "" {
		resolvedPath = filepath.Join(h.workDir, file)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", resolvedPath, err)
	}

	// For Go files, use AST parsing
	if strings.HasSuffix(file, ".go") {
		return h.extractGoFunctionBody(string(content), funcName)
	}

	// For other files, use regex-based extraction
	return h.extractFunctionBodyRegex(string(content), funcName)
}

// extractGoFunctionBody uses Go's AST parser to extract a function body.
func (h *HolographicProvider) extractGoFunctionBody(content, funcName string) (string, error) {
	if funcName == "" {
		return "", fmt.Errorf("empty function name")
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse Go file: %w", err)
	}

	var targetFunc *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == funcName {
				targetFunc = fn
				return false
			}
		}
		return true
	})

	if targetFunc == nil {
		return "", fmt.Errorf("function %s not found", funcName)
	}

	startLine := fset.Position(targetFunc.Pos()).Line
	endLine := fset.Position(targetFunc.End()).Line

	return h.extractLineRange(content, startLine, endLine)
}

// extractFunctionBodyRegex uses regex to find function bodies in non-Go files.
func (h *HolographicProvider) extractFunctionBodyRegex(content, funcName string) (string, error) {
	if funcName == "" {
		return "", fmt.Errorf("empty function name")
	}

	// Common function patterns
	patterns := []string{
		// Go: func Name(...)
		fmt.Sprintf(`(?m)^func\s+(\([^)]*\)\s+)?%s\s*\(`, regexp.QuoteMeta(funcName)),
		// Python: def name(...)
		fmt.Sprintf(`(?m)^def\s+%s\s*\(`, regexp.QuoteMeta(funcName)),
		// JavaScript/TypeScript: function name(...) or name(...) =>
		fmt.Sprintf(`(?m)(function\s+%s|%s\s*[:=]\s*(async\s+)?(\([^)]*\)|[^=])\s*=>)`,
			regexp.QuoteMeta(funcName), regexp.QuoteMeta(funcName)),
		// Java/C#: modifier type name(...)
		fmt.Sprintf(`(?m)(public|private|protected)?\s*\w+\s+%s\s*\(`, regexp.QuoteMeta(funcName)),
	}

	lines := strings.Split(content, "\n")
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		for i, line := range lines {
			if re.MatchString(line) {
				endLine := h.findFunctionEnd(lines, i)
				return h.extractLineRange(content, i+1, endLine+1)
			}
		}
	}

	return "", fmt.Errorf("function %s not found with regex patterns", funcName)
}

// findFunctionEnd finds the closing brace of a function by tracking depth.
func (h *HolographicProvider) findFunctionEnd(lines []string, startIdx int) int {
	depth := 0
	inFunction := false
	inBlockComment := false
	inString := rune(0) // 0 if not in string, else the quote char: '"', '\'', '`'

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		lineRunes := []rune(line)

		for j := 0; j < len(lineRunes); j++ {
			ch := lineRunes[j]

			// Handle block comment content
			if inBlockComment {
				if ch == '*' && j+1 < len(lineRunes) && lineRunes[j+1] == '/' {
					inBlockComment = false
					j++ // skip /
				}
				continue
			}

			// Handle string/char literal content
			if inString != 0 {
				if ch == inString {
					// Check for escape
					// Count consecutive backslashes preceding this quote
					backslashes := 0
					for k := j - 1; k >= 0; k-- {
						if lineRunes[k] != '\\' {
							break
						}
						backslashes++
					}
					// If even number of backslashes (0, 2...), the quote is NOT escaped
					if backslashes%2 == 0 {
						inString = 0
					}
				}
				continue
			}

			// Start of block comment
			if ch == '/' && j+1 < len(lineRunes) && lineRunes[j+1] == '*' {
				inBlockComment = true
				j++ // skip *
				continue
			}

			// Start of line comment
			if ch == '/' && j+1 < len(lineRunes) && lineRunes[j+1] == '/' {
				break // ignore rest of line
			}

			// Start of string/char literal
			if ch == '"' || ch == '\'' || ch == '`' {
				inString = ch
				continue
			}

			// Brace counting
			if ch == '{' {
				depth++
				inFunction = true
			} else if ch == '}' {
				depth--
				if inFunction && depth == 0 {
					return i
				}
			}
		}

		// Reset string state ONLY for single-line quotes (" and ')
		// Backticks (`) span multiple lines.
		if inString == '"' || inString == '\'' {
			inString = 0
		}
	}

	// Fallback: return a reasonable range
	endIdx := startIdx + maxCallerBodyLines
	if endIdx > len(lines)-1 {
		endIdx = len(lines) - 1
	}
	return endIdx
}

// extractLineRange extracts lines from content with truncation.
func (h *HolographicProvider) extractLineRange(content string, startLine, endLine int) (string, error) {
	lines := strings.Split(content, "\n")

	startIdx := startLine - 1
	endIdx := endLine

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}
	if startIdx >= endIdx {
		return "", fmt.Errorf("invalid line range: %d-%d", startLine, endLine)
	}

	// Apply max lines limit
	lineCount := endIdx - startIdx
	truncated := false
	if lineCount > maxCallerBodyLines {
		endIdx = startIdx + maxCallerBodyLines
		truncated = true
	}

	result := strings.Join(lines[startIdx:endIdx], "\n")
	if truncated {
		result += "\n// ... (truncated)"
	}

	return result, nil
}

// FormatWithPriorities formats the holographic context with priority annotations.
// This produces a markdown-formatted string optimized for LLM injection.
func (hc *HolographicContext) FormatWithPriorities() string {
	if hc == nil {
		return ""
	}

	var sb strings.Builder

	// Include standard context first
	sb.WriteString(hc.FormatForPrompt())

	// Add prioritized callers section if present
	if len(hc.PrioritizedCallers) == 0 {
		return sb.String()
	}

	sb.WriteString("\n## Impact-Prioritized Context\n\n")
	sb.WriteString(fmt.Sprintf("Overall Impact Priority: %s\n\n",
		priorityLevelString(hc.ImpactPriority)))

	sb.WriteString("### Prioritized Callers\n")
	sb.WriteString("These functions call into the target code, sorted by impact priority:\n\n")

	for i, caller := range hc.PrioritizedCallers {
		sb.WriteString(fmt.Sprintf("#### %d. `%s`", i+1, caller.Name))
		if caller.File != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", filepath.Base(caller.File)))
		}
		sb.WriteString("\n")

		// Priority indicator
		switch {
		case caller.Priority >= 80:
			sb.WriteString("**Priority: HIGH** - Critical impact path\n")
		case caller.Priority >= 50:
			sb.WriteString("*Priority: Medium*\n")
		default:
			sb.WriteString("Priority: Low\n")
		}

		if caller.Depth > 1 {
			sb.WriteString(fmt.Sprintf("Call depth: %d hops from target\n", caller.Depth))
		}

		if caller.Body != "" {
			sb.WriteString("```go\n")
			sb.WriteString(caller.Body)
			if !strings.HasSuffix(caller.Body, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		} else {
			sb.WriteString("(body not available)\n\n")
		}
	}

	sb.WriteString(fmt.Sprintf("**Summary:** %d prioritized callers included\n",
		len(hc.PrioritizedCallers)))

	return sb.String()
}

// FormatPrioritizedCallersCompact returns a compact list of prioritized callers.
func (hc *HolographicContext) FormatPrioritizedCallersCompact() string {
	if hc == nil || len(hc.PrioritizedCallers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Prioritized callers:\n")

	for _, caller := range hc.PrioritizedCallers {
		priorityMark := ""
		if caller.Priority >= 80 {
			priorityMark = "[HIGH] "
		} else if caller.Priority >= 50 {
			priorityMark = "[MED] "
		} else {
			priorityMark = "[LOW] "
		}

		sb.WriteString(fmt.Sprintf("  %s%s", priorityMark, caller.Name))
		if caller.File != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", filepath.Base(caller.File)))
		}
		if caller.Depth > 1 {
			sb.WriteString(fmt.Sprintf(" (depth=%d)", caller.Depth))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// priorityLevelString converts a numeric priority to a human-readable level.
func priorityLevelString(priority int) string {
	switch {
	case priority >= 90:
		return "CRITICAL"
	case priority >= 80:
		return "HIGH"
	case priority >= 50:
		return "MEDIUM"
	case priority >= 25:
		return "LOW"
	default:
		return "MINIMAL"
	}
}

// HasPrioritizedCallers returns true if the context has impact-prioritized callers.
func (hc *HolographicContext) HasPrioritizedCallers() bool {
	return hc != nil && len(hc.PrioritizedCallers) > 0
}

// GetHighPriorityCallers returns only callers with priority >= threshold.
func (hc *HolographicContext) GetHighPriorityCallers(threshold int) []PrioritizedCaller {
	if hc == nil || len(hc.PrioritizedCallers) == 0 {
		return nil
	}

	result := make([]PrioritizedCaller, 0)
	for _, caller := range hc.PrioritizedCallers {
		if caller.Priority >= threshold {
			result = append(result, caller)
		}
	}
	return result
}
