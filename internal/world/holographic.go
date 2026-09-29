package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/tools/codedom"
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
	//
	// DirectImporters is the one dimension here a model cannot get by reading
	// the file: which other files in the workspace depend on this package. It
	// comes from the world model's dependency_link facts, so it is only
	// populated when a kernel is attached and a scan has run.
	DirectImports   []ImportInfo `json:"direct_imports"`   // What this file imports
	DirectImporters []string     `json:"direct_importers"` // Files that import this package
	ExternalDeps    []string     `json:"external_deps"`    // Third-party dependencies

	// Semantic Relationships (from knowledge graph)
	CallGraph []CallEdge `json:"call_graph"` // Who calls what, up to maxCallGraphEdges

	// CallerCount is the number of distinct callers on matching code_calls
	// edges, and CallGraphEdges is the number of those edges. CallGraph stores
	// at most maxCallGraphEdges of them. These two are the full walk:
	// PromptSection's "and N more" is computed from them, so the storage cap
	// cannot shrink the pool the remainder counts.
	CallerCount    int `json:"caller_count,omitempty"`
	CallGraphEdges int `json:"call_graph_edges,omitempty"`

	// FilesUnparsed is how many non-test Go files in the package directory did
	// not parse: past maxPackageFilesToParse, over maxSiblingFileBytes, or a
	// parse error. SkippedSiblings names the oversized ones. The signature and
	// type pools are only the files that parsed, so the prompt states this
	// count beside them.
	FilesUnparsed   int              `json:"files_unparsed,omitempty"`
	SkippedSiblings []SkippedSibling `json:"skipped_siblings,omitempty"`

	// Code Quality Signals
	TestCoverage float64 `json:"test_coverage"` // If known from facts
	HasTests     bool    `json:"has_tests"`     // Does a _test.go file exist?
	TODOCount    int     `json:"todo_count"`    // Number of TODO/FIXME comments

	// Impact-Aware Priority Context (from Mangle impact analysis)
	ImpactPriority     int                 `json:"impact_priority"`     // Overall priority from Mangle analysis
	PrioritizedCallers []PrioritizedCaller `json:"prioritized_callers"` // Callers sorted by impact priority

	// ReferencedSymbols are the package-level symbols the target file uses but
	// does not define. Used to rank which sibling signatures are worth prompt
	// tokens; see rankSignaturesForTarget.
	ReferencedSymbols []string `json:"referenced_symbols,omitempty"`

	// SymbolRefCount maps a package-level symbol to how many other files in the
	// package reference it — in-package centrality, used as the last ranking
	// tier so a file with no relevance signal of its own still gets the
	// package's load-bearing API rather than its alphabetically first one.
	SymbolRefCount map[string]int `json:"symbol_ref_count,omitempty"`
}

// PrioritizedCaller represents a caller function with impact analysis metadata.
// Used by the impact-aware context builder to provide targeted review context.
//
// StartLine and EndLine are the 1-based inclusive span of the function. The
// model view names that span and does not paste the body: get_element and
// read_file return it whole. Body is set only when a caller already holds the
// source; it is never a prefix with a truncation marker.
type PrioritizedCaller struct {
	Name      string `json:"name"`                 // Function/method name
	File      string `json:"file"`                 // Source file path
	Body      string `json:"body,omitempty"`       // Whole function, when a caller supplied it
	StartLine int    `json:"start_line,omitempty"` // 1-based inclusive
	EndLine   int    `json:"end_line,omitempty"`   // 1-based inclusive
	Priority  int    `json:"priority"`             // Priority from context_priority query (higher = more important)
	Depth     int    `json:"depth"`                // Distance in call graph (1 = direct caller)
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
	DocComment string `json:"doc_comment,omitempty"` // First line of the doc comment, kept whole
}

// SkippedSibling is a package file the holographic parse did not read because
// it is larger than maxSiblingFileBytes. The prompt names it; a log line was
// the only record, and the signature list then looked complete.
type SkippedSibling struct {
	File string `json:"file"`
	Size int64  `json:"size"`
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

// CallEdge represents a caller->callee relationship.
type CallEdge struct {
	Caller string `json:"caller"`
	Callee string `json:"callee"`
}

// FactQuerier is the entire kernel surface the holographic provider needs:
// read-only predicate queries. Depending on *core.RealKernel here forced every
// caller (and every test) to build a full kernel to ask for context, and made
// the provider look like it could mutate the world model when it never does.
type FactQuerier interface {
	Query(predicate string) ([]core.Fact, error)
}

// HolographicProvider creates rich context for code analysis.
type HolographicProvider struct {
	kernel  FactQuerier
	workDir string

	// pkgCache memoises the filesystem-derived package parse. It is created
	// lazily so a zero-value &HolographicProvider{} — which the tests build
	// directly — still caches rather than silently falling back to the
	// re-parse-everything path the cache exists to remove.
	cacheOnce sync.Once
	pkgCache  *packageParseCache
}

// packageCache returns the lazily-created package-parse cache.
func (h *HolographicProvider) packageCache() *packageParseCache {
	h.cacheOnce.Do(func() {
		h.pkgCache = newPackageParseCache()
	})
	return h.pkgCache
}

// CacheStats reports package-parse cache hits and misses for this provider.
// Exposed so `nerd world` and the world cache metrics can show whether the
// per-turn holographic path is actually being served from cache.
func (h *HolographicProvider) CacheStats() (hits, misses int64) {
	if h == nil {
		return 0, 0
	}
	return h.packageCache().stats()
}

// NewHolographicProvider creates a new holographic context provider.
// kernel may be nil (context degrades to filesystem-only analysis).
func NewHolographicProvider(kernel FactQuerier, workDir string) *HolographicProvider {
	return &HolographicProvider{
		kernel:  normalizeQuerier(kernel),
		workDir: workDir,
	}
}

// normalizeQuerier flattens a typed-nil pointer to a nil interface. Callers
// hold a *core.RealKernel that may be nil; stored in an interface that is a
// NON-nil interface holding a nil pointer, so `h.kernel == nil` would be false
// and the first Query would panic. Narrowing the dependency must not turn a
// graceful degradation path into a crash.
func normalizeQuerier(q FactQuerier) FactQuerier {
	if q == nil {
		return nil
	}
	v := reflect.ValueOf(q)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if v.IsNil() {
			return nil
		}
	}
	return q
}

// GetContext generates complete holographic context for a file.
func (h *HolographicProvider) GetContext(filePath string) (*HolographicContext, error) {
	return h.getContextInternal(context.Background(), filePath)
}

// GetContextWithContext generates complete holographic context with support for context cancellation.
func (h *HolographicProvider) GetContextWithContext(ctx context.Context, filePath string) (*HolographicContext, error) {
	return h.getContextInternal(ctx, filePath)
}

// packageLabel names the package for the truncation lines, falling back to the
// module when the package clause could not be read.
func packageLabel(hc *HolographicContext) string {
	if hc == nil {
		return "?"
	}
	if hc.TargetPkg != "" {
		return hc.TargetPkg
	}
	if hc.Module != "" {
		return hc.Module
	}
	return "?"
}

// relevanceRank orders a package symbol against one target file.
//
// Lower is better. The order encodes what a model editing this file actually
// needs to know:
//
//	0  defined in the target file      — what this file offers
//	1  referenced by the target file   — what this file depends on
//	2  everything else in the package  — background
//
// Before this ranking existed the fallback was directory order, so a target
// file that exports nothing of its own — a package-marker file, a main.go, a
// thin wrapper — spent all eight signature slots on whichever sibling sorted
// first. On internal/core/kernel.go that was eight symbols from
// action_validator.go followed by "… and 592 more".
func relevanceRank(definedIn, name, targetBase string, referenced map[string]struct{}) int {
	if definedIn == targetBase {
		return 0
	}
	if _, ok := referenced[name]; ok {
		return 1
	}
	return 2
}

// referencedSet turns the context's sorted slice back into a lookup set.
func referencedSet(referenced []string) map[string]struct{} {
	if len(referenced) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(referenced))
	for _, name := range referenced {
		set[name] = struct{}{}
	}
	return set
}

// rankSignaturesForTarget returns the package's exported signatures ordered by
// relevance to targetBase, and the size of the pool they were drawn from.
//
// The pool size is returned separately so the "… and N more" line counts what
// was left out of the whole candidate set, not out of the ranked prefix.
func rankSignaturesForTarget(all []SymbolSignature, targetBase string, referenced []string, centrality map[string]int) ([]SymbolSignature, int) {
	refs := referencedSet(referenced)

	exported := make([]SymbolSignature, 0, len(all))
	for _, sig := range all {
		if sig.Exported {
			exported = append(exported, sig)
		}
	}
	if len(exported) == 0 {
		return nil, 0
	}

	sort.SliceStable(exported, func(i, j int) bool {
		ri := relevanceRank(exported[i].File, exported[i].Name, targetBase, refs)
		rj := relevanceRank(exported[j].File, exported[j].Name, targetBase, refs)
		if ri != rj {
			return ri < rj
		}
		// Within a tier, prefer what the rest of the package actually leans on.
		if ci, cj := centrality[exported[i].Name], centrality[exported[j].Name]; ci != cj {
			return ci > cj
		}
		// Then a documented symbol over an undocumented one, then name order so
		// the section is byte-stable between turns.
		di, dj := exported[i].DocComment != "", exported[j].DocComment != ""
		if di != dj {
			return di
		}
		if exported[i].File != exported[j].File {
			return exported[i].File < exported[j].File
		}
		return exported[i].Name < exported[j].Name
	})
	return exported, len(exported)
}

// rankTypesForTarget is rankSignaturesForTarget for type definitions.
//
// Unexported types are kept: inside a package the model edits the unexported
// types too, and a struct's field count is the cheapest useful thing that can
// be said about it.
func rankTypesForTarget(all []TypeDefinition, targetBase string, referenced []string, centrality map[string]int) ([]TypeDefinition, int) {
	if len(all) == 0 {
		return nil, 0
	}
	refs := referencedSet(referenced)

	ranked := append([]TypeDefinition(nil), all...)
	sort.SliceStable(ranked, func(i, j int) bool {
		ri := relevanceRank(ranked[i].File, ranked[i].Name, targetBase, refs)
		rj := relevanceRank(ranked[j].File, ranked[j].Name, targetBase, refs)
		if ri != rj {
			return ri < rj
		}
		if ci, cj := centrality[ranked[i].Name], centrality[ranked[j].Name]; ci != cj {
			return ci > cj
		}
		if ranked[i].Exported != ranked[j].Exported {
			return ranked[i].Exported
		}
		if ranked[i].File != ranked[j].File {
			return ranked[i].File < ranked[j].File
		}
		return ranked[i].Name < ranked[j].Name
	})
	return ranked, len(ranked)
}

// PromptSection renders holographic context for prompt injection.
//
// Mirrors the style of internal/projectdoc/facts.go:Document.PromptSection —
// the frontmatter/facts are not readable by the model directly, and the
// holographic context is likewise invisible unless rendered into prose.
// It calls GetContextWithContext and returns "" on any error or nil context
// so callers can concatenate unconditionally.
//
// The output is token-frugal: it summarizes rather than dumps, and caps long
// lists, because it is injected into every prompt for a file-targeted turn.
func (h *HolographicProvider) PromptSection(ctx context.Context, filePath string) string {
	if h == nil {
		return ""
	}
	hc, err := h.GetContextWithContext(ctx, filePath)
	if err != nil || hc == nil {
		return ""
	}

	// substantive tracks whether any block said something the model could not
	// have worked out from the filename. Architecture facets are inferred from
	// path patterns, so they are present even for a file that does not exist;
	// emitting a header, an inferred Role and "**Tests**: no" for a missing
	// file is prompt tokens spent to say nothing.
	substantive := false

	var b strings.Builder
	b.WriteString("## Holographic Context (")
	b.WriteString(filePath)
	b.WriteString(")\n\n")

	// Architecture / package summary (one compact line).
	//
	// Built by joining the non-empty parts rather than by chaining conditional
	// separators. The old form emitted the separator whenever the *first* field
	// was present, so a file with no package clause but an inferred Role
	// rendered as a leading " · **Role**: …" — a line that looks like something
	// was dropped.
	var facets []string
	if hc.TargetPkg != "" {
		facets = append(facets, "**Package**: `"+hc.TargetPkg+"`")
	}
	if hc.Layer != "" {
		facets = append(facets, "**Layer**: `"+hc.Layer+"`")
	}
	if hc.Module != "" {
		facets = append(facets, "**Module**: `"+hc.Module+"`")
	}
	if hc.Role != "" {
		facets = append(facets, "**Role**: `"+hc.Role+"`")
	}
	if len(facets) > 0 || hc.SystemPurpose != "" {
		if len(facets) > 0 {
			b.WriteString(strings.Join(facets, " · "))
			b.WriteString("\n")
		}
		if hc.SystemPurpose != "" {
			b.WriteString(hc.SystemPurpose)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Test coverage signal.
	if hc.HasTests {
		b.WriteString("**Tests**: yes")
		if hc.TestCoverage > 0 {
			fmt.Fprintf(&b, " (coverage %.0f%%)", hc.TestCoverage*100)
		}
		b.WriteString("\n\n")
	} else {
		b.WriteString("**Tests**: no\n\n")
	}

	// Files the package parse did not read. Stated before the signature and
	// type lists those files are missing from, so "and N more" on those lists
	// is not read as the whole package.
	pkgDir := filepath.Dir(filePath)
	if hc.FilesUnparsed > 0 || len(hc.SkippedSiblings) > 0 {
		substantive = true
		writeUnparsedFiles(&b, hc, pkgDir)
	}

	// Exported signatures, ranked by relevance to the target file.
	base := filepath.Base(filePath)
	sigs, sigPool := rankSignaturesForTarget(hc.PackageSignatures, base, hc.ReferencedSymbols, hc.SymbolRefCount)
	if len(sigs) > 0 {
		substantive = true
		b.WriteString("### Exported signatures\n\n")
		// Remainder from the pool, then slice. sigPool is every exported
		// signature the parse collected, which is not the whole package when
		// FilesUnparsed is non-zero; writePoolRemainder says which.
		truncated := 0
		if sigPool > maxSigs {
			truncated = sigPool - maxSigs
		}
		shown := sigs
		if len(shown) > maxSigs {
			shown = shown[:maxSigs]
		}
		for _, sig := range shown {
			b.WriteString("- `")
			if sig.Receiver != "" {
				b.WriteString("func (")
				b.WriteString(sig.Receiver)
				b.WriteString(") ")
				b.WriteString(sig.Name)
			} else {
				b.WriteString("func ")
				b.WriteString(sig.Name)
			}
			b.WriteString(sig.Params)
			if sig.Returns != "" && sig.Returns != "()" {
				b.WriteString(" ")
				b.WriteString(sig.Returns)
			}
			b.WriteString("`")
			if sig.File != "" && sig.File != base {
				b.WriteString(" — `")
				b.WriteString(sig.File)
				b.WriteString("`")
			}
			b.WriteString("\n")
		}
		if truncated > 0 {
			// Name the pool. "and 593 more" next to eight symbols from the
			// target's own file reads like the file has 601 exports; saying
			// "in package core" makes it clear the rest is the package's
			// surface, which is what the model needs to know before it goes
			// looking for something. package_outline lists every declaration,
			// including the ones past this cap and the files that were not parsed.
			writePoolRemainder(&b, "exported", truncated, hc.FilesUnparsed, packageLabel(hc), pkgDir)
		}
		b.WriteString("\n")
	}

	// Type definitions, ranked by relevance to the target file.
	types, typePool := rankTypesForTarget(hc.PackageTypes, base, hc.ReferencedSymbols, hc.SymbolRefCount)
	if len(types) > 0 {
		substantive = true
		b.WriteString("### Type definitions\n\n")
		truncated := 0
		if typePool > maxTypes {
			truncated = typePool - maxTypes
		}
		shown := types
		if len(shown) > maxTypes {
			shown = shown[:maxTypes]
		}
		for _, td := range shown {
			b.WriteString("- `type ")
			b.WriteString(td.Name)
			b.WriteString("` — ")
			b.WriteString(td.Kind)
			if td.Kind == "struct" && len(td.Fields) > 0 {
				fmt.Fprintf(&b, " (%d fields)", len(td.Fields))
			} else if td.Kind == "interface" && len(td.Methods) > 0 {
				fmt.Fprintf(&b, " (%d methods)", len(td.Methods))
			}
			if td.File != "" && td.File != base {
				fmt.Fprintf(&b, " — `%s`", td.File)
			}
			b.WriteString("\n")
		}
		if truncated > 0 {
			writePoolRemainder(&b, "", truncated, hc.FilesUnparsed, packageLabel(hc), pkgDir)
		}
		b.WriteString("\n")
	}

	// Dependents — which files outside this package import it.
	//
	// Rendered because it is the one dimension in the context that is invisible
	// from the file itself, and the first thing worth knowing before changing
	// an exported symbol. Bounded to a handful plus a count: the question is
	// "is this load-bearing, and for whom", which six examples answer as well
	// as fifty.
	if len(hc.DirectImporters) > 0 {
		substantive = true
		b.WriteString("### Imported by\n\n")
		shown := hc.DirectImporters
		truncated := 0
		if len(hc.DirectImporters) > maxRenderedImporters {
			truncated = len(hc.DirectImporters) - maxRenderedImporters
			shown = hc.DirectImporters[:maxRenderedImporters]
		}
		for _, importer := range shown {
			b.WriteString("- `")
			b.WriteString(importer)
			b.WriteString("`\n")
		}
		if truncated > 0 {
			// importers_of lists every file that imports the package. The
			// argument is the import path when the module is known, which is
			// the form the tool resolves without guessing a package name.
			fmt.Fprintf(&b, "- … and %d more file(s); `importers_of` package=%s lists every one\n", truncated, h.importerReadArg(filePath, hc))
		}
		b.WriteString("\n")
	}

	// Callers — who calls this file (impact-aware if available).
	if len(hc.PrioritizedCallers) > 0 {
		substantive = true
		b.WriteString("### Callers (impact-prioritized)\n\n")
		truncated := 0
		if len(hc.PrioritizedCallers) > maxCallers {
			truncated = len(hc.PrioritizedCallers) - maxCallers
		}
		shown := hc.PrioritizedCallers
		if len(shown) > maxCallers {
			shown = shown[:maxCallers]
		}
		for _, c := range shown {
			b.WriteString("- `")
			b.WriteString(c.Name)
			b.WriteString("` — `")
			b.WriteString(filepath.Base(c.File))
			b.WriteString("`")
			if c.Priority != 0 {
				fmt.Fprintf(&b, " (priority %d", c.Priority)
				if c.Depth != 0 {
					fmt.Fprintf(&b, ", depth %d", c.Depth)
				}
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
		if truncated > 0 {
			// The prioritized list is the full set queryImpactPriorities
			// returned. callers_of reads every call site; this line is only
			// the impact-ranked prefix.
			writeCallerRemainder(&b, truncated, 0, 0)
		}
		b.WriteString("\n")
	} else if len(hc.CallGraph) > 0 || hc.CallGraphEdges > 0 {
		substantive = true
		b.WriteString("### Callers\n\n")
		seen := make(map[string]struct{}, len(hc.CallGraph))
		var callers []string
		for _, e := range hc.CallGraph {
			if _, ok := seen[e.Caller]; !ok {
				seen[e.Caller] = struct{}{}
				callers = append(callers, e.Caller)
			}
		}
		// Names come from the stored edges. The count comes from the full
		// walk (CallerCount), which includes callers that only appear on
		// edges past maxCallGraphEdges. Falling back to len(callers) covers a
		// context whose CallGraph was filled in directly.
		totalCallers := hc.CallerCount
		if totalCallers < len(callers) {
			totalCallers = len(callers)
		}
		shown := callers
		if len(shown) > maxCallers {
			shown = shown[:maxCallers]
		}
		for _, caller := range shown {
			b.WriteString("- `")
			b.WriteString(caller)
			b.WriteString("`\n")
		}
		edges := hc.CallGraphEdges
		if edges < len(hc.CallGraph) {
			edges = len(hc.CallGraph)
		}
		writeCallerRemainder(&b, totalCallers-len(shown), edges, len(hc.CallGraph))
		b.WriteString("\n")
	}

	// The target file's own outline: every declaration with its line range,
	// the data get_elements returns. A turn or planned step aimed at this file
	// needs it before its first read -- the CodeDOM atoms tell the model to
	// locate, read only that range, then edit by range -- so the harness serves
	// it with the file instead of the model spending its first calls finding
	// its way. It is read fresh on every call, so after an edit the ranges are
	// the current ones. Observed 2026-09-18: a two-file change spent 40
	// read_file and 13 grep calls around its 10 edits.
	if outline := h.targetOutline(filePath); outline != "" {
		substantive = true
		b.WriteString(outline)
	}

	if !substantive {
		return ""
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return ""
	}
	return result + "\n"
}

// maxOutlineElements bounds the outline a prompt carries; a file with more
// says how many it left out and points at get_elements, which lists them all.
const maxOutlineElements = 120

// maxOutlineSignature bounds one outline entry. The cut states how many
// characters were left off and names get_element, which returns the signature
// whole. A bare ellipsis was a shortened signature presented as the signature.
const maxOutlineSignature = 100

// targetOutline renders the declarations of filePath with their current line
// ranges, or "" when the file cannot be read or declares nothing.
func (h *HolographicProvider) targetOutline(filePath string) string {
	path := filePath
	if !filepath.IsAbs(path) && h.workDir != "" {
		path = filepath.Join(h.workDir, filePath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	elements := codedom.ElementsFromSource(path, string(data))
	if len(elements) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### Outline of %s (%d declarations, line ranges current as of this request)\n\n", filepath.Base(filePath), len(elements))
	shown := elements
	if len(shown) > maxOutlineElements {
		shown = shown[:maxOutlineElements]
	}
	for _, el := range shown {
		label := strings.TrimSpace(el.Signature)
		if label == "" {
			label = el.Name
		}
		// Cut on runes so the reported remainder is a character count and the
		// label stays valid UTF-8. len(label) is bytes.
		rest := 0
		if n := utf8.RuneCountInString(label); n > maxOutlineSignature {
			rest = n - maxOutlineSignature
			label = string([]rune(label)[:maxOutlineSignature])
		}
		if rest > 0 {
			fmt.Fprintf(&b, "- %d-%d %s `%s…` (%d more characters; `get_element` returns the signature whole)\n", el.StartLine, el.EndLine, el.Type, label, rest)
		} else {
			fmt.Fprintf(&b, "- %d-%d %s `%s`\n", el.StartLine, el.EndLine, el.Type, label)
		}
	}
	if rest := len(elements) - len(shown); rest > 0 {
		fmt.Fprintf(&b, "- … %d more declarations not listed; `get_elements path=%s` lists every one\n", rest, filePath)
	}
	b.WriteString("\n")
	return b.String()
}

// writeUnparsedFiles states how many package files the signature and type
// pools leave out, and names the tools that read them. package_outline lists
// every declaration in the directory; get_elements reads one file.
func writeUnparsedFiles(b *strings.Builder, hc *HolographicContext, pkgDir string) {
	if hc == nil || (hc.FilesUnparsed <= 0 && len(hc.SkippedSiblings) == 0) {
		return
	}
	b.WriteString("### Package files not parsed\n\n")
	if hc.FilesUnparsed == 1 {
		fmt.Fprintf(b, "- 1 of the package's Go files was not parsed, so the signature and type lists are not the whole package. `package_outline` path=%s lists every declaration; `get_elements` reads one file.\n", pkgDir)
	} else if hc.FilesUnparsed > 1 {
		fmt.Fprintf(b, "- %d of the package's Go files were not parsed, so the signature and type lists are not the whole package. `package_outline` path=%s lists every declaration; `get_elements` reads one file.\n", hc.FilesUnparsed, pkgDir)
	}
	for _, s := range hc.SkippedSiblings {
		fmt.Fprintf(b, "- `%s` (%d bytes) exceeds the %d-byte sibling parse bound and was not parsed.\n", filepath.Base(s.File), s.Size, maxSiblingFileBytes)
	}
	b.WriteString("\n")
}

// writePoolRemainder is the "and N more" line for signatures (kind "exported")
// and types (kind ""). N is the remainder of the parsed pool. When files were
// not parsed, the line says so instead of calling that remainder the package.
// package_outline reads the declarations either way.
func writePoolRemainder(b *strings.Builder, kind string, truncated, filesUnparsed int, pkg, pkgDir string) {
	if truncated <= 0 {
		return
	}
	qual := "in package"
	if filesUnparsed > 0 {
		qual = "among the parsed files of package"
	}
	if kind != "" {
		fmt.Fprintf(b, "- … and %d more %s %s `%s`; `package_outline` path=%s lists every declaration\n", truncated, kind, qual, pkg, pkgDir)
		return
	}
	fmt.Fprintf(b, "- … and %d more %s `%s`; `package_outline` path=%s lists every declaration\n", truncated, qual, pkg, pkgDir)
}

// writeCallerRemainder states callers left off the list. omittedCallers is the
// true remainder of distinct callers, not the remainder of the stored edges.
// When matchingEdges exceeds storedEdges, those edges were counted and not
// stored; the line says both numbers. callers_of lists every call site.
func writeCallerRemainder(b *strings.Builder, omittedCallers, matchingEdges, storedEdges int) {
	unstored := matchingEdges - storedEdges
	if unstored < 0 {
		unstored = 0
	}
	switch {
	case omittedCallers > 0 && unstored > 0:
		fmt.Fprintf(b, "- … and %d more callers (%d matching call-graph edges, %d not stored here); `callers_of` lists every call site\n", omittedCallers, matchingEdges, unstored)
	case omittedCallers > 0:
		fmt.Fprintf(b, "- … and %d more callers; `callers_of` lists every call site\n", omittedCallers)
	case unstored > 0:
		fmt.Fprintf(b, "- … %d of %d matching call-graph edges are not stored here; `callers_of` lists every call site\n", unstored, matchingEdges)
	}
}

// importerReadArg is the package argument importers_of resolves: the module
// import path when go.mod can name it, otherwise the package clause, otherwise
// the directory.
func (h *HolographicProvider) importerReadArg(filePath string, hc *HolographicContext) string {
	if arg := h.packageImportPath(filePath); arg != "" {
		return arg
	}
	if hc != nil && hc.TargetPkg != "" {
		return hc.TargetPkg
	}
	return filepath.ToSlash(filepath.Dir(filePath))
}

// getContextInternal is the shared cancellable context generator.
func (h *HolographicProvider) getContextInternal(ctx context.Context, filePath string) (*HolographicContext, error) {
	if filePath == "" {
		return &HolographicContext{
			TargetFile:     "",
			PackageImports: make(map[string][]string),
		}, nil
	}

	logging.WorldDebug("HolographicProvider: generating context for %s", filepath.Base(filePath))

	hc := &HolographicContext{
		TargetFile:     filePath,
		PackageImports: make(map[string][]string),
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Detect language and route to appropriate handler
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		if err := h.buildGoContextWithContext(ctx, hc, filePath); err != nil {
			logging.WorldDebug("HolographicProvider: Go context failed: %v", err)
			// Continue with partial context
		}
	default:
		// For non-Go files, provide basic architectural context
		h.buildBasicContextWithContext(ctx, hc, filePath)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Add architectural analysis (works for any language)
	h.analyzeArchitecture(hc, filePath)

	// Query knowledge graph for relationships
	h.queryRelationshipsWithContext(ctx, hc, filePath)

	// Attach the kernel's impact ranking. This is what makes PromptSection's
	// "Callers (impact-prioritized)" branch reachable; without it the model got
	// an unordered list of caller names on every turn. Costs one kernel query
	// and no file I/O — see queryImpactPriorities.
	h.applyImpactPriorities(ctx, hc)

	// Dependency dimensions. DirectImporters is the reverse edge — who depends
	// on this package — which is the one thing here a model cannot get by
	// reading the file.
	h.applyImportDimensions(hc, filePath)
	h.applyDirectImporters(hc, filePath)

	// Check for test file existence
	h.checkTestCoverage(hc, filePath)

	logging.WorldDebug("HolographicProvider: context complete for %s - %d siblings, %d signatures",
		filepath.Base(filePath), len(hc.PackageSiblings), len(hc.PackageSignatures))

	return hc, nil
}

// maxPackageFilesToParse caps how many sibling files one directory is parsed
// for signatures. Parsing every file of a large package sits on the turn's
// critical path. Files past the cap are not dropped from the count: PromptSection
// says how many were not parsed. Slicing the list and then counting signatures
// reported a package remainder that left those files out.
const maxPackageFilesToParse = 100

// maxSiblingFileBytes skips a generated monster. A 5 MB .go file is a generated
// table, and parsing it can dominate the package. The skip is named in the
// prompt with its size. A log line alone left a hole in the signature list.
const maxSiblingFileBytes = 5 * 1024 * 1024

// maxCallGraphEdges bounds how many matching code_calls edges are stored on
// CallGraph. The walk still counts every matching edge and every distinct
// caller past this bound. Stopping the walk here made "and N more callers" a
// count of the stored prefix.
const maxCallGraphEdges = 100

// maxSigs, maxTypes and maxCallers bound what one prompt section lists. The
// remainder is computed from the full pool, then the list is sliced.
const (
	maxSigs    = 8
	maxTypes   = 8
	maxCallers = 8
)

// buildGoContextWithContext builds package-level context for Go files with
// cancellation and limit protections, served from the package-parse cache when
// no file in the directory has changed.
func (h *HolographicProvider) buildGoContextWithContext(ctx context.Context, hc *HolographicContext, filePath string) error {
	dir := filepath.Dir(filePath)

	fingerprint, entries, err := directoryFingerprint(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	cache := h.packageCache()
	parse, hit := cache.get(dir, fingerprint)
	if !hit {
		// A cancelled or failed parse is never cached: a partial answer that
		// looks fresh is worse than paying the parse again.
		parse, err = h.parsePackage(ctx, dir, entries)
		if err != nil {
			return err
		}
		cache.put(dir, fingerprint, parse)
	}

	parse.applyTo(hc, filePath)
	if hc.TargetPkg == "" {
		h.readPackageClause(hc, filePath)
	}
	return nil
}

// parsePackage parses every non-test .go file in dir into a cacheable
// packageParse. entries is the already-read directory listing, so the caller's
// fingerprint pass and this one share a single ReadDir.
func (h *HolographicProvider) parsePackage(ctx context.Context, dir string, entries []os.DirEntry) (*packageParse, error) {
	p := &packageParse{
		imports: make(map[string][]string),
		pkgName: make(map[string]string),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Test files are excluded from signature extraction: the model asks
		// what the package offers, not what its tests happen to define.
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		p.allGoFiles = append(p.allGoFiles, filepath.Join(dir, name))
	}

	// Name order, not ReadDir order. The cap's omitted set has to be a function
	// of the directory, so "N files were not parsed" is the same on every
	// machine. ReadDir order is not.
	sort.Strings(p.allGoFiles)
	if len(p.allGoFiles) > maxPackageFilesToParse {
		logging.Get(logging.CategoryWorld).Warn("buildGoContext: package too large (%d files), limiting parsing to first %d", len(p.allGoFiles), maxPackageFilesToParse)
		p.goFiles = append([]string(nil), p.allGoFiles[:maxPackageFilesToParse]...)
	} else {
		p.goFiles = append([]string(nil), p.allGoFiles...)
	}

	parsed := 0
	fset := token.NewFileSet()
	for _, goFile := range p.goFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if info, statErr := os.Stat(goFile); statErr == nil && info.Size() > maxSiblingFileBytes {
			h.noteOversizedSibling(p, goFile, info.Size())
			continue
		}
		if err := h.parseGoFileInto(p, fset, goFile); err != nil {
			logging.WorldDebug("HolographicProvider: failed to parse %s: %v", goFile, err)
			// A file that did not parse is counted below. Continue with the rest.
			continue
		}
		parsed++
	}
	// Files past the cap were not walked by the loop above. Stat them so an
	// oversized one is named, not folded into the count with no identity.
	for _, goFile := range p.allGoFiles[len(p.goFiles):] {
		if info, statErr := os.Stat(goFile); statErr == nil && info.Size() > maxSiblingFileBytes {
			h.noteOversizedSibling(p, goFile, info.Size())
		}
	}
	// Full list minus what parsed. Computed here, after the walk, never from
	// the already-sliced goFiles length alone: a huge file inside the cap and
	// a file past the cap are both absent from the signature pool.
	p.filesUnparsed = len(p.allGoFiles) - parsed

	narrowLocalRefs(p)

	return p, nil
}

// noteOversizedSibling records a file the parse skipped for size and logs it.
// The prompt reads the record; the log is not the model's only copy.
func (h *HolographicProvider) noteOversizedSibling(p *packageParse, goFile string, size int64) {
	logging.Get(logging.CategoryWorld).Warn("buildGoContext: skipping huge sibling file: %s (%d bytes)", goFile, size)
	p.skippedSiblings = append(p.skippedSiblings, SkippedSibling{File: goFile, Size: size})
}

// readPackageClause fills TargetPkg for a file the package parse did not cover
// — a _test.go target, or one past maxPackageFilesToParse.
func (h *HolographicProvider) readPackageClause(hc *HolographicContext, filePath string) {
	fset := token.NewFileSet()
	if node, err := parser.ParseFile(fset, filePath, nil, parser.PackageClauseOnly); err == nil && node.Name != nil {
		hc.TargetPkg = node.Name.Name
	}
}

// parseGoFileInto extracts one file's signatures, types, constants, imports and
// package clause into a packageParse.
//
// This is the cacheable unit: it reads only the file's bytes and writes only
// into p, so a directory's parse is a pure function of that directory.
func (h *HolographicProvider) parseGoFileInto(p *packageParse, fset *token.FileSet, filePath string) error {
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		// Handle entirely empty .go files (0 bytes and whitespace only)
		if strings.Contains(err.Error(), "expected 'package', found 'EOF'") {
			return nil
		}
		return err
	}

	fileName := filepath.Base(filePath)
	if node.Name != nil {
		p.pkgName[fileName] = node.Name.Name
	}
	// Record every identifier this file mentions. parsePackage narrows the set
	// to package-level names once every file has been seen; keeping raw
	// identifiers past that point would cost more memory than the parse itself.
	rawRefs := make(map[string]struct{}, 64)

	// Extract imports
	var imports []string
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		imports = append(imports, importPath)
	}
	p.imports[fileName] = imports

	// Walk AST for definitions
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			rawRefs[id.Name] = struct{}{}
		}
		switch x := n.(type) {
		case *ast.FuncDecl:
			sig := h.extractFuncSignature(fset, x, fileName)
			p.signatures = append(p.signatures, sig)

		case *ast.GenDecl:
			switch x.Tok {
			case token.TYPE:
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						typeDef := h.extractTypeDefinition(fset, ts, x, fileName)
						p.types = append(p.types, typeDef)
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
							p.constants = append(p.constants, constDef)
						}
					}
				}
			}
		}
		return true
	})

	if p.localRefs == nil {
		p.localRefs = make(map[string]map[string]struct{}, 8)
	}
	p.localRefs[fileName] = rawRefs

	return nil
}

// narrowLocalRefs reduces each file's raw identifier set to the package-level
// symbols it uses but does not define.
//
// Run once after the whole package is parsed, because "is this name defined in
// this package" is only answerable then. Dropping the rest is what keeps the
// cached parse bounded by the package's symbol count instead of by every
// identifier in every file — the difference between a few kilobytes per package
// and a few megabytes.
func narrowLocalRefs(p *packageParse) {
	if p == nil || len(p.localRefs) == 0 {
		return
	}

	// owner maps a package-level symbol to the file that defines it.
	owner := make(map[string]string, len(p.signatures)+len(p.types)+len(p.constants))
	for _, sig := range p.signatures {
		// Methods are addressed through their receiver, not by bare name, so
		// indexing them here would make every file that mentions a common verb
		// like Close or String look like it depends on all of them.
		if sig.Receiver == "" {
			owner[sig.Name] = sig.File
		}
	}
	for _, td := range p.types {
		owner[td.Name] = td.File
	}
	for _, cd := range p.constants {
		owner[cd.Name] = cd.File
	}

	p.refCount = make(map[string]int, len(owner))
	for file, raw := range p.localRefs {
		narrowed := make(map[string]struct{}, len(raw)/8+1)
		for name := range raw {
			if definedIn, ok := owner[name]; ok && definedIn != file {
				narrowed[name] = struct{}{}
				p.refCount[name]++
			}
		}
		p.localRefs[file] = narrowed
	}
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

	// First line of the doc comment, kept whole. A 100-character cut plus "..."
	// stored a shortened comment as the comment, and this field has no
	// remainder line in front of the model. The file is already refused above
	// maxSiblingFileBytes, which is the bound on how long the line can be; a
	// second cut under that would be an omission with no true count.
	if fn.Doc != nil && len(fn.Doc.List) > 0 {
		text := strings.TrimPrefix(fn.Doc.List[0].Text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSpace(text)
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

// queryRelationshipsWithContext queries the kernel with context support and graph edge caps.
func (h *HolographicProvider) queryRelationshipsWithContext(ctx context.Context, hc *HolographicContext, filePath string) {
	if h.kernel == nil {
		return
	}

	if err := ctx.Err(); err != nil {
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
		select {
		case <-ctx.Done():
			return
		default:
		}

		if len(fact.Args) < 5 {
			continue
		}
		factFile, _ := fact.Args[0].(string)
		normalFactFile := strings.ToLower(strings.ReplaceAll(factFile, "\\", "/"))
		if strings.Contains(normalFactFile, normalPath) || strings.Contains(normalPath, normalFactFile) {
			if sym, ok := fact.Args[1].(string); ok {
				fileSymbols = append(fileSymbols, sym)
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return
	}

	// Query code_calls to build call graph for these symbols
	callFacts, err := h.kernel.Query("code_calls")
	if err != nil {
		return
	}

	// Storage cap only. The loop does not break at it: edges past the cap are
	// counted into CallGraphEdges and their callers into CallerCount, which is
	// what PromptSection's remainder is computed from. fn: rows are duplicates
	// of a bare row (see below) and are not part of either count.
	edgeCount := 0
	callerSeen := make(map[string]struct{})

	for _, fact := range callFacts {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if len(fact.Args) < 2 {
			continue
		}
		caller, _ := fact.Args[0].(string)
		callee, _ := fact.Args[1].(string)

		// Dual CodeDOM-ref rows (fn:pkg.Name, emitted for the test-impact
		// joins) carry the same edge as a bare row this walk already
		// matches: counting them would double CallGraphEdges and make the
		// remainder a lie in the other direction. A bare cartographer ID
		// never starts with "fn:".
		if strings.HasPrefix(caller, "fn:") || strings.HasPrefix(callee, "fn:") {
			continue
		}

		matched := false
		for _, sym := range fileSymbols {
			if strings.Contains(caller, sym) || strings.Contains(callee, sym) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		hc.CallGraphEdges++
		if _, ok := callerSeen[caller]; !ok {
			callerSeen[caller] = struct{}{}
			hc.CallerCount++
		}
		if edgeCount < maxCallGraphEdges {
			hc.CallGraph = append(hc.CallGraph, CallEdge{
				Caller: caller,
				Callee: callee,
			})
			edgeCount++
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

// buildBasicContextWithContext provides minimal context with cancellation support.
func (h *HolographicProvider) buildBasicContextWithContext(ctx context.Context, hc *HolographicContext, filePath string) {

	// Just set up basic file info
	dir := filepath.Dir(filePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	ext := filepath.Ext(filePath)
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ext {
			fullPath := filepath.Join(dir, entry.Name())
			if fullPath != filePath {
				hc.PackageSiblings = append(hc.PackageSiblings, fullPath)
			}
		}
	}
}
