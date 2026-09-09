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
	//
	// DirectImporters is the one dimension here a model cannot get by reading
	// the file: which other files in the workspace depend on this package. It
	// comes from the world model's dependency_link facts, so it is only
	// populated when a kernel is attached and a scan has run.
	DirectImports   []ImportInfo `json:"direct_imports"`   // What this file imports
	DirectImporters []string     `json:"direct_importers"` // Files that import this package
	ExternalDeps    []string     `json:"external_deps"`    // Third-party dependencies

	// Semantic Relationships (from knowledge graph)
	CallGraph []CallEdge `json:"call_graph"` // Who calls what

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

	// Exported signatures, ranked by relevance to the target file.
	const maxSigs = 8
	base := filepath.Base(filePath)
	sigs, sigPool := rankSignaturesForTarget(hc.PackageSignatures, base, hc.ReferencedSymbols, hc.SymbolRefCount)
	if len(sigs) > 0 {
		substantive = true
		b.WriteString("### Exported signatures\n\n")
		shown := sigs
		truncated := 0
		if len(shown) > maxSigs {
			truncated = sigPool - maxSigs
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
			// looking for something.
			fmt.Fprintf(&b, "- … and %d more exported in package `%s`\n", truncated, packageLabel(hc))
		}
		b.WriteString("\n")
	}

	// Type definitions, ranked by relevance to the target file.
	const maxTypes = 8
	types, typePool := rankTypesForTarget(hc.PackageTypes, base, hc.ReferencedSymbols, hc.SymbolRefCount)
	if len(types) > 0 {
		substantive = true
		b.WriteString("### Type definitions\n\n")
		shown := types
		truncated := 0
		if len(shown) > maxTypes {
			truncated = typePool - maxTypes
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
			fmt.Fprintf(&b, "- … and %d more in package `%s`\n", truncated, packageLabel(hc))
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
		if len(shown) > maxRenderedImporters {
			truncated = len(shown) - maxRenderedImporters
			shown = shown[:maxRenderedImporters]
		}
		for _, importer := range shown {
			b.WriteString("- `")
			b.WriteString(importer)
			b.WriteString("`\n")
		}
		if truncated > 0 {
			fmt.Fprintf(&b, "- … and %d more file(s)\n", truncated)
		}
		b.WriteString("\n")
	}

	// Callers — who calls this file (impact-aware if available).
	const maxCallers = 8
	if len(hc.PrioritizedCallers) > 0 {
		substantive = true
		b.WriteString("### Callers (impact-prioritized)\n\n")
		shown := hc.PrioritizedCallers
		truncated := 0
		if len(shown) > maxCallers {
			truncated = len(shown) - maxCallers
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
			fmt.Fprintf(&b, "- … and %d more\n", truncated)
		}
		b.WriteString("\n")
	} else if len(hc.CallGraph) > 0 {
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
		truncated := 0
		shown := callers
		if len(shown) > maxCallers {
			truncated = len(shown) - maxCallers
			shown = shown[:maxCallers]
		}
		for _, caller := range shown {
			b.WriteString("- `")
			b.WriteString(caller)
			b.WriteString("`\n")
		}
		if truncated > 0 {
			fmt.Fprintf(&b, "- … and %d more\n", truncated)
		}
		b.WriteString("\n")
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

// buildGoContext builds package-level context for Go files.
func (h *HolographicProvider) buildGoContext(ctx *HolographicContext, filePath string) error {
	return h.buildGoContextWithContext(context.Background(), ctx, filePath)
}

// maxPackageFilesToParse caps how many sibling files one directory contributes
// to a holographic context. A package with more files than this is already past
// the point where listing its symbols helps the model, and parsing all of them
// costs real time on the turn's critical path.
const maxPackageFilesToParse = 100

// maxSiblingFileBytes skips generated monsters. A 5 MB .go file is a generated
// table, not something whose signatures the model needs, and parsing it can
// dominate the whole package.
const maxSiblingFileBytes = 5 * 1024 * 1024

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

	p.goFiles = p.allGoFiles
	if len(p.goFiles) > maxPackageFilesToParse {
		logging.Get(logging.CategoryWorld).Warn("buildGoContext: package too large (%d files), limiting parsing to first %d", len(p.goFiles), maxPackageFilesToParse)
		p.goFiles = p.goFiles[:maxPackageFilesToParse]
	}

	fset := token.NewFileSet()
	for _, goFile := range p.goFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if info, statErr := os.Stat(goFile); statErr == nil && info.Size() > maxSiblingFileBytes {
			logging.Get(logging.CategoryWorld).Warn("buildGoContext: skipping huge sibling file: %s (%d bytes)", goFile, info.Size())
			continue
		}
		if err := h.parseGoFileInto(p, fset, goFile); err != nil {
			logging.WorldDebug("HolographicProvider: failed to parse %s: %v", goFile, err)
			// Continue with other files
		}
	}

	narrowLocalRefs(p)

	return p, nil
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

// extractGoSignatures parses one Go file directly into a HolographicContext.
//
// Kept as the single-file entry point for callers that hold a context rather
// than a package parse; it is a thin adapter over parseGoFileInto so the two
// paths cannot drift in what they extract.
func (h *HolographicProvider) extractGoSignatures(ctx *HolographicContext, fset *token.FileSet, filePath string) error {
	p := &packageParse{
		imports: make(map[string][]string, 1),
		pkgName: make(map[string]string, 1),
	}
	if err := h.parseGoFileInto(p, fset, filePath); err != nil {
		return err
	}
	ctx.PackageSignatures = append(ctx.PackageSignatures, p.signatures...)
	ctx.PackageTypes = append(ctx.PackageTypes, p.types...)
	ctx.PackageConstants = append(ctx.PackageConstants, p.constants...)
	if ctx.PackageImports == nil {
		ctx.PackageImports = make(map[string][]string, len(p.imports))
	}
	for k, v := range p.imports {
		ctx.PackageImports[k] = v
	}
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
	h.queryRelationshipsWithContext(context.Background(), ctx, filePath)
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

	const maxCallGraphEdges = 100 // Cap to prevent prompt & serialization bloat
	edgeCount := 0

	for _, fact := range callFacts {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if edgeCount >= maxCallGraphEdges {
			break
		}

		if len(fact.Args) < 2 {
			continue
		}
		caller, _ := fact.Args[0].(string)
		callee, _ := fact.Args[1].(string)

		// Check if caller or callee is in our file
		for _, sym := range fileSymbols {
			if strings.Contains(caller, sym) || strings.Contains(callee, sym) {
				hc.CallGraph = append(hc.CallGraph, CallEdge{
					Caller: caller,
					Callee: callee,
				})
				edgeCount++
				break
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
	h.buildBasicContextWithContext(context.Background(), ctx, filePath)
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
