package codedom

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// =============================================================================
// STRUCTURAL QUERY TOOLS
// =============================================================================
// The model-facing surface of the world model's structure index: where a symbol
// is, what a package holds, who calls a function, what it calls, who imports a
// package, where a piece of text sits, and what nothing references. Each
// answers in one call a question that otherwise costs a grep, a read_file per
// hit, and a round of the context window for each.
//
// Every row is "path:start-end  kind  ref  signature": a repo-relative citation
// the model can use as is, and a ref every element verb accepts.

// StructureSymbol is one element, as the provider reports it.
type StructureSymbol struct {
	// Ref is the element's workspace address: the directory, receiver and
	// name for Go (internal/world.StructureIndex.Refresh), the file and key
	// for Mangle (internal/core/defaults/policy/impact.mg:decl:impact_caller/2).
	Ref string
	// Key is the element's address within its file.
	Key string
	// ID is the call graph's identity (pkg.Recv.Name), which the kernel's
	// code_calls and modified_function facts are keyed by.
	ID                                   string
	Kind, File, Signature, Doc, Revision string
	StartLine, EndLine                   int
	Exported                             bool
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

// SymbolQuery is what find_symbol asks for. Every set field narrows.
type SymbolQuery struct {
	// Names are exact names in any spelling find_symbol accepts; a symbol
	// matching any of them qualifies.
	Names []string
	// Pattern is a regular expression over the declared name.
	Pattern string
	Kind    string
	// Path is a workspace-relative directory (searched recursively) or file.
	Path string
}

// StructureImporter is one file importing a package.
type StructureImporter struct {
	File, Name string
	Line       int
}

// StructureTextHit is one literal, comment or identifier containing the
// searched text, answered as an address in the element model.
type StructureTextHit struct {
	File string
	Line int
	In   string // string, comment or identifier
	Text string
	// Ref is the enclosing element, "" at file level outside every element.
	Ref string
}

// StructureUse is one use of a symbol outside its own declaration.
type StructureUse struct {
	File         string
	Line, Column int
	// Start and End span the use in the file's LF-normalised source: the
	// qualifier and name of a selector, or the bare identifier.
	Start, End int
	Qualifier  string
	// Ref is the element the use sits in.
	Ref string
	// Match is "exact" when the package ties the use to the symbol, and
	// "by-name" for a method reached through a value of unknown type.
	Match string
}

// FileStatus is what the index knows about one file's parse.
type FileStatus struct {
	Known  bool
	Parsed bool
	Errors []string
	// LastGood are the declarations of the last clean parse, kept while the
	// file does not parse so a broken file does not vanish from the model.
	LastGood []StructureSymbol
}

// PredicateRow is one statement concerning a Mangle predicate.
type PredicateRow struct {
	Role   string // declares, derives, reads
	Symbol StructureSymbol
}

// StructureProvider is implemented by world.StructureIndex. It is an
// interface here because internal/world imports this package.
type StructureProvider interface {
	FindSymbol(ctx context.Context, q SymbolQuery) ([]StructureSymbol, string, error)
	Outline(ctx context.Context, path string) ([]StructureSymbol, string, error)
	Callers(ctx context.Context, query string) ([]StructureSymbol, []StructureCaller, string, error)
	Callees(ctx context.Context, query string) ([]StructureSymbol, []StructureCallee, string, error)
	Unreferenced(ctx context.Context, path string) ([]StructureSymbol, string, error)
	// Resolve returns the elements a ref names: one for a ref as the tools
	// print it, several for a short name that is only a search.
	Resolve(ctx context.Context, ref string) ([]StructureSymbol, string, error)
	// Importers returns the import path pkg resolved to and the files
	// importing it.
	Importers(ctx context.Context, pkg string) (string, []StructureImporter, string, error)
	FindText(ctx context.Context, text, in, path string) ([]StructureTextHit, string, error)
	// Uses returns the symbol a ref names and every use of it outside its own
	// declaration.
	Uses(ctx context.Context, ref string) ([]StructureSymbol, []StructureUse, string, error)
	FileStatus(ctx context.Context, path string) (FileStatus, error)
	// CanonicalRefs maps one file's element keys to their workspace refs.
	CanonicalRefs(ctx context.Context, path string) (map[string]string, error)
	// ImportResolver answers import derivation's questions for one file.
	ImportResolver(ctx context.Context, path string) (codemodel.Resolver, error)
	PredicateOutline(ctx context.Context, pred string) ([]PredicateRow, string, error)
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

// optionalStructureProvider is the provider when one is registered. The
// element verbs work on one file without it, and use it for what only the
// workspace knows: canonical refs, imports, uses.
func optionalStructureProvider() StructureProvider {
	structureProviderMu.RLock()
	defer structureProviderMu.RUnlock()
	return globalStructureProvider
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
	if index != "" {
		fmt.Fprintf(&sb, "-- index: %s\n", index)
	}
	return sb.String()
}

func symbolRow(sym StructureSymbol) string {
	row := fmt.Sprintf("%s:%d-%d  %s  %s", sym.File, sym.StartLine, sym.EndLine, sym.Kind, sym.Ref)
	if sym.Revision != "" {
		row += "  rev " + sym.Revision
	}
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

const symbolQueryHelp = "A ref as the tools print it (internal/world.StructureIndex.Refresh), or a short name that is searched: a bare name (evaluate), a receiver-qualified method (RealKernel.evaluate), a package-qualified name (core.NewRealKernel)."

// FindSymbolTool locates declarations by name, pattern, kind and path.
func FindSymbolTool() *tools.Tool {
	return &tools.Tool{
		Name:          "find_symbol",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description: "Find declarations anywhere in the workspace -- Go functions, methods, types, consts, vars and Mangle predicates -- with file, line span, kind, ref, rev, signature and doc line. " +
			"Ask for one or several names at once (name: \"A|B|C\"), a regular expression over the name (pattern), and narrow by kind and path. Use this instead of grep to locate code.",
		Category: tools.CategoryCode,
		Priority: 95,
		Effect:   tools.EffectRead,
		Execute:  executeFindSymbol,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"name":    {Type: "string", Description: symbolQueryHelp + " Several names separated by | are looked up together."},
				"pattern": {Type: "string", Description: "Regular expression over the declared name, e.g. ^Test(Parse|Load) or Provider$"},
				"kind":    {Type: "string", Description: "Optional filter: function, method, struct, interface, type, const, var, decl, rule, fact"},
				"path":    {Type: "string", Description: "Optional workspace-relative directory (searched recursively) or file to search within"},
				"offset":  {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeFindSymbol(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	pattern, _ := args["pattern"].(string)
	kind, _ := args["kind"].(string)
	path, _ := args["path"].(string)
	var names []string
	for n := range strings.SplitSeq(name, "|") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 && strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("name or pattern is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	q := SymbolQuery{Names: names, Pattern: strings.TrimSpace(pattern), Kind: strings.TrimSpace(kind), Path: strings.TrimSpace(path)}
	symbols, index, err := provider.FindSymbol(ctx, q)
	if err != nil {
		return "", err
	}
	logging.Tools("find_symbol: names=%q pattern=%q kind=%q path=%q -> %d declarations", names, pattern, kind, path, len(symbols))
	var header string
	switch {
	case len(names) > 0 && q.Pattern != "":
		header = fmt.Sprintf("declarations named %q or matching /%s/:", strings.Join(names, "|"), q.Pattern)
	case len(names) > 0:
		header = fmt.Sprintf("declarations named %q:", strings.Join(names, "|"))
	default:
		header = fmt.Sprintf("declarations matching /%s/:", q.Pattern)
	}
	if q.Path != "" {
		header = strings.TrimSuffix(header, ":") + " under " + q.Path + ":"
	}
	return renderPage(symbolRows(symbols), pageArgs(args), header, index,
		"none. A name is case-sensitive and must be the identifier as declared; pattern takes a regular expression; package_outline lists what a directory declares."), nil
}

// PackageOutlineTool lists every declaration in a directory or file.
func PackageOutlineTool() *tools.Tool {
	return &tools.Tool{
		Name:          "package_outline",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List every declaration in a Go package directory (or one Go or Mangle file) with file, line span, kind, ref, rev, signature and doc line, in file and line order. Use this instead of list_files plus read_file to learn what a package contains.",
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
		"none. The path holds no Go or Mangle file the index parsed; it is workspace-relative and a directory is not searched recursively."), nil
}

// CallersOfTool lists the call sites of a function or method.
func CallersOfTool() *tools.Tool {
	return &tools.Tool{
		Name:          "callers_of",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List every call site of a Go function or method across the workspace: calling element, file and line. Sites marked exact are tied to the target by package; sites marked by-name are calls through a variable to a method of that name. Use this instead of grep to find who calls something, or to establish that nothing does.",
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
