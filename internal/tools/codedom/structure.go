package codedom

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// =============================================================================
// STRUCTURAL QUERY TOOLS
// =============================================================================
// The model-facing surface of the world model's structure index: where a symbol
// is, what a package holds, who calls a function, what it calls, and what
// nothing references. Each answers in one call a question that otherwise costs
// a grep, a read_file per hit, and a round of the context window for each.
//
// Every row is "path:start-end  kind  id  signature": a repo-relative citation
// the model can use as is.

// StructureSymbol is one declaration, as the provider reports it.
type StructureSymbol struct {
	ID, Kind, File, Signature, Doc string
	StartLine, EndLine             int
	Exported                       bool
}

// StructureCaller is one call site of a queried symbol.
type StructureCaller struct {
	Caller, File, Match string
	Line                int
}

// StructureCallee is one call made inside a queried symbol.
type StructureCallee struct {
	Call       string
	Line       int
	Candidates []string
}

// StructureProvider is implemented over world.StructureIndex. It is an
// interface here because internal/world imports this package.
type StructureProvider interface {
	FindSymbol(ctx context.Context, query, kind string) ([]StructureSymbol, string, error)
	Outline(ctx context.Context, path string) ([]StructureSymbol, string, error)
	Callers(ctx context.Context, query string) ([]StructureSymbol, []StructureCaller, string, error)
	Callees(ctx context.Context, query string) ([]StructureSymbol, []StructureCallee, string, error)
	Unreferenced(ctx context.Context, path string) ([]StructureSymbol, string, error)
}

// Guarded for the same reason globalTestProvider is: a process can hold more
// than one Cortex, and each boot registers.
var (
	structureProviderMu     sync.RWMutex
	globalStructureProvider StructureProvider
)

// RegisterStructureProvider installs the provider the structural tools query.
func RegisterStructureProvider(provider StructureProvider) {
	structureProviderMu.Lock()
	defer structureProviderMu.Unlock()
	globalStructureProvider = provider
}

func structureProvider() (StructureProvider, error) {
	structureProviderMu.RLock()
	defer structureProviderMu.RUnlock()
	if globalStructureProvider == nil {
		return nil, fmt.Errorf("the structure index is not available in this process")
	}
	return globalStructureProvider, nil
}

// structurePageSize bounds one answer. It is a page, never a cut: the footer
// says how many rows exist and which offset continues them.
const structurePageSize = 150

func pageArgs(args map[string]any) int {
	switch v := args["offset"].(type) {
	case float64:
		return max(int(v), 0)
	case int:
		return max(v, 0)
	}
	return 0
}

func renderPage(rows []string, offset int, header, index, emptyHint string) string {
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	if len(rows) == 0 {
		sb.WriteString(emptyHint)
		sb.WriteString("\n" + tools.StructuralNoRows + "\n")
	}
	end := min(offset+structurePageSize, len(rows))
	if offset < len(rows) {
		for _, row := range rows[offset:end] {
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}
	if end < len(rows) {
		fmt.Fprintf(&sb, "-- rows %d-%d of %d; call again with offset=%d for the rest\n", offset+1, end, len(rows), end)
	} else if len(rows) > 0 {
		fmt.Fprintf(&sb, "-- %d rows, complete\n", len(rows))
	}
	fmt.Fprintf(&sb, "-- index: %s\n", index)
	return sb.String()
}

func symbolRow(sym StructureSymbol) string {
	row := fmt.Sprintf("%s:%d-%d  %s  %s", sym.File, sym.StartLine, sym.EndLine, sym.Kind, sym.ID)
	if sym.Signature != "" {
		row += "  " + sym.Signature
	}
	if sym.Doc != "" {
		row += "  // " + sym.Doc
	}
	return row
}

func symbolRows(symbols []StructureSymbol) []string {
	rows := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		rows = append(rows, symbolRow(sym))
	}
	return rows
}

const symbolQueryHelp = "A bare name (evaluate), a receiver-qualified method (RealKernel.evaluate), a package-qualified name (core.NewRealKernel) or a full id (core.RealKernel.evaluate)."

// FindSymbolTool locates declarations by name across the workspace.
func FindSymbolTool() *tools.Tool {
	return &tools.Tool{
		Name:          "find_symbol",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "Find where a Go symbol is declared, anywhere in the workspace: file, line span, kind, signature and doc line. Use this instead of grep to locate a function, method, type, const or var. Answers from a parsed index that follows edits.",
		Category:      tools.CategoryCode,
		Priority:      95,
		Effect:        tools.EffectRead,
		Execute:       executeFindSymbol,
		Schema: tools.ToolSchema{
			Required: []string{"name"},
			Properties: map[string]tools.Property{
				"name":   {Type: "string", Description: symbolQueryHelp},
				"kind":   {Type: "string", Description: "Optional filter: function, method, struct, interface, type, const, var"},
				"offset": {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeFindSymbol(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("name is required")
	}
	kind, _ := args["kind"].(string)
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	symbols, index, err := provider.FindSymbol(ctx, name, kind)
	if err != nil {
		return "", err
	}
	logging.Tools("find_symbol: %q kind=%q -> %d declarations", name, kind, len(symbols))
	return renderPage(symbolRows(symbols), pageArgs(args), fmt.Sprintf("declarations named %q:", name), index,
		"none. The name is case-sensitive and must be the identifier as declared; package_outline lists what a directory declares."), nil
}

// PackageOutlineTool lists every declaration in a directory or file.
func PackageOutlineTool() *tools.Tool {
	return &tools.Tool{
		Name:          "package_outline",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List every declaration in a Go package directory (or one file) with file, line span, kind, signature and doc line, in file and line order. Use this instead of list_files plus read_file to learn what a package contains.",
		Category:      tools.CategoryCode,
		Priority:      94,
		Effect:        tools.EffectRead,
		Execute:       executePackageOutline,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path":   {Type: "string", Description: "Workspace-relative directory (internal/context) or file (internal/context/activation.go)"},
				"offset": {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executePackageOutline(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	symbols, index, err := provider.Outline(ctx, path)
	if err != nil {
		return "", err
	}
	logging.Tools("package_outline: %s -> %d declarations", path, len(symbols))
	return renderPage(symbolRows(symbols), pageArgs(args), fmt.Sprintf("declarations in %s:", path), index,
		"none. The path holds no Go file the index parsed; it is workspace-relative and a directory is not searched recursively."), nil
}

// CallersOfTool lists the call sites of a function or method.
func CallersOfTool() *tools.Tool {
	return &tools.Tool{
		Name:          "callers_of",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List every call site of a Go function or method across the workspace: calling function, file and line. Sites marked exact are tied to the target by package; sites marked by-name are calls through a variable to a method of that name. Use this instead of grep to find who calls something, or to establish that nothing does.",
		Category:      tools.CategoryCode,
		Priority:      93,
		Effect:        tools.EffectRead,
		Execute:       executeCallersOf,
		Schema: tools.ToolSchema{
			Required: []string{"symbol"},
			Properties: map[string]tools.Property{
				"symbol": {Type: "string", Description: symbolQueryHelp},
				"offset": {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeCallersOf(ctx context.Context, args map[string]any) (string, error) {
	symbol, _ := args["symbol"].(string)
	if strings.TrimSpace(symbol) == "" {
		return "", fmt.Errorf("symbol is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	targets, callers, index, err := provider.Callers(ctx, symbol)
	if err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return renderPage(nil, 0, fmt.Sprintf("callers of %q:", symbol), index,
			"no function or method has that name, so there is nothing to find callers of. find_symbol shows what is declared."), nil
	}
	var header strings.Builder
	fmt.Fprintf(&header, "callers of %q, which names:", symbol)
	for _, t := range targets {
		header.WriteString("\n  " + symbolRow(t))
	}
	rows := make([]string, 0, len(callers))
	for _, c := range callers {
		rows = append(rows, fmt.Sprintf("%s:%d  %s  in %s", c.File, c.Line, c.Match, c.Caller))
	}
	logging.Tools("callers_of: %q -> %d targets, %d call sites", symbol, len(targets), len(callers))
	return renderPage(rows, pageArgs(args), header.String(), index,
		"no call site anywhere in the workspace. A function passed as a value or reached by reflection is not a call site; unreferenced_symbols checks for any use of the name."), nil
}

// CalleesOfTool lists the calls made inside a function or method.
func CalleesOfTool() *tools.Tool {
	return &tools.Tool{
		Name:          "callees_of",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List every call made inside a Go function or method, in line order, each with the workspace declarations it can refer to (file and line). Use this to follow a code path without reading each file.",
		Category:      tools.CategoryCode,
		Priority:      92,
		Effect:        tools.EffectRead,
		Execute:       executeCalleesOf,
		Schema: tools.ToolSchema{
			Required: []string{"symbol"},
			Properties: map[string]tools.Property{
				"symbol": {Type: "string", Description: symbolQueryHelp},
				"offset": {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeCalleesOf(ctx context.Context, args map[string]any) (string, error) {
	symbol, _ := args["symbol"].(string)
	if strings.TrimSpace(symbol) == "" {
		return "", fmt.Errorf("symbol is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	targets, callees, index, err := provider.Callees(ctx, symbol)
	if err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return renderPage(nil, 0, fmt.Sprintf("calls made by %q:", symbol), index,
			"no function or method has that name. find_symbol shows what is declared."), nil
	}
	var header strings.Builder
	fmt.Fprintf(&header, "calls made by %q, which names:", symbol)
	for _, t := range targets {
		header.WriteString("\n  " + symbolRow(t))
	}
	rows := make([]string, 0, len(callees))
	for _, c := range callees {
		row := fmt.Sprintf("line %d  %s", c.Line, c.Call)
		if len(c.Candidates) > 0 {
			row += "  -> " + strings.Join(c.Candidates, " | ")
		}
		rows = append(rows, row)
	}
	logging.Tools("callees_of: %q -> %d targets, %d calls", symbol, len(targets), len(callees))
	return renderPage(rows, pageArgs(args), header.String(), index, "it makes no calls."), nil
}

// UnreferencedSymbolsTool lists declarations nothing names.
func UnreferencedSymbolsTool() *tools.Tool {
	return &tools.Tool{
		Name:          "unreferenced_symbols",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List the Go declarations under a directory whose name occurs nowhere else in the workspace: code that exists and nothing uses. Conservative: any use of the name anywhere counts as a reference, so every row is a real orphan by name; methods may still satisfy an interface. Use this to answer what is built but not wired.",
		Category:      tools.CategoryCode,
		Priority:      91,
		Effect:        tools.EffectRead,
		Execute:       executeUnreferencedSymbols,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path":   {Type: "string", Description: "Workspace-relative directory, searched recursively (internal/context), or one file"},
				"offset": {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeUnreferencedSymbols(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	symbols, index, err := provider.Unreferenced(ctx, path)
	if err != nil {
		return "", err
	}
	logging.Tools("unreferenced_symbols: %s -> %d declarations", path, len(symbols))
	return renderPage(symbolRows(symbols), pageArgs(args), fmt.Sprintf("declarations under %s that nothing else names:", path), index,
		"none: every declaration under that path is named somewhere else in the workspace."), nil
}
