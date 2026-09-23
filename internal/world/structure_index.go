package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/world/codemodel"
)

// =============================================================================
// STRUCTURE INDEX
// =============================================================================
// The workspace-wide, symbol-level layer of the world model: every Go
// declaration and Mangle statement with its line span and revision, and every
// Go call site with its line.
//
// Why it exists. The deep facts the Cartographer produces (code_defines,
// code_calls) are computed for the active file and its one-hop neighbours, so
// "who calls X across the repository" had no facts behind it, and no tool a
// model can call read those facts anyway. Measured 2026-09-21 on a 24-task
// campaign: 166 raw filesystem calls (read_file, grep, glob, list_files)
// against 14 get_elements calls, because grep was the only tool that answered a
// question spanning files.
//
// Elements come from codemodel, the one element model every CodeDOM surface
// reads, so a declaration's span, kind, doc and revision mean the same thing
// here, in get_element, and in the edit verbs.
//
// Identity. A symbol's ID follows the Cartographer's convention (pkg.Name,
// pkg.Receiver.Name) so the kernel's code_calls and modified_function facts
// join. Its Ref is the model-facing address, keyed by directory rather than
// package name: this repository has `package main` in 20 directories, and a
// name that resolves to 20 declarations is a search key, not an address.
//
// Freshness. Every query re-stats the tree and re-parses exactly the files
// whose fingerprint moved, so an answer never describes a file as it was
// before the last edit. A file that stops parsing keeps its last clean
// element list beside its current one: the answer from before the edit is
// labelled as such, and the file stays addressable for its repair.

// StructSymbol is one declaration.
type StructSymbol struct {
	ID        string `json:"id"`
	Ref       string `json:"ref"`
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Exported  bool   `json:"exported"`

	name     string
	receiver string
	pkg      string
	dir      string
	lang     string
	// bodyPreds are the predicates a Mangle statement's body reads.
	bodyPreds []string
}

// StructCall is one call site, attributed to the declaration containing it.
type StructCall struct {
	Caller    string `json:"caller"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Qualifier string `json:"-"` // identifier before the dot, empty for a bare call
	Name      string `json:"-"`
	callerKey string
}

// StructCallerMatch is a call site reported for a target symbol.
type StructCallerMatch struct {
	StructCall
	// Match says how the site was tied to the target: "exact" when the package
	// qualifier or a same-package bare call identifies it, "by-name" when only
	// the method or function name matches (a call through a variable).
	Match string `json:"match"`
}

type structFile struct {
	fingerprint string
	lang        string
	pkg         string
	dir         string
	imports     []codemodel.Import
	importNames map[string]string // local name -> path
	symbols     []StructSymbol
	calls       []StructCall
	idents      map[string]int // identifier name -> occurrences, declarations included
	parsed      bool
	errors      []string
	// lastGood is the symbol list of the last clean parse, kept while the
	// file does not parse.
	lastGood []StructSymbol
}

// StructureIndex holds the parsed structure of every Go and Mangle file under
// a root.
type StructureIndex struct {
	root string

	mu          sync.Mutex
	files       map[string]*structFile // canonical path -> parsed file
	module      string
	moduleFP    string
	lastRefresh time.Time
	lastParsed  int
}

// NewStructureIndex creates an empty index; the first query fills it.
func NewStructureIndex(root string) *StructureIndex {
	return &StructureIndex{root: root, files: make(map[string]*structFile)}
}

// StructureStats describes the index after a refresh.
type StructureStats struct {
	Files        int           `json:"files"`
	Symbols      int           `json:"symbols"`
	Calls        int           `json:"calls"`
	Reparsed     int           `json:"reparsed"`
	Broken       int           `json:"broken"`
	RefreshTook  time.Duration `json:"-"`
	RefreshTookS string        `json:"refresh_took"`
}

// Line renders the stats as the footer every structural answer carries.
func (s StructureStats) Line() string {
	line := fmt.Sprintf("%d files, %d declarations, %d call sites; %d reparsed for this answer (%s)",
		s.Files, s.Symbols, s.Calls, s.Reparsed, s.RefreshTookS)
	if s.Broken > 0 {
		line += fmt.Sprintf("; %d files do not parse (get_elements shows their errors)", s.Broken)
	}
	return line
}

// Refresh brings the index up to date with the tree. Caller holds no lock.
func (s *StructureIndex) Refresh(ctx context.Context) (StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshLocked(ctx)
}

func (s *StructureIndex) refreshLocked(ctx context.Context) (StructureStats, error) {
	started := time.Now()
	type candidate struct{ fsPath, canonical, fingerprint string }
	var stale []candidate
	seen := make(map[string]struct{}, len(s.files))

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // an unreadable directory is skipped, not fatal
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != s.root && codemodel.SkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if codemodel.LanguageOf(name) == "" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		canonical := types.CanonicalPath(s.root, path)
		seen[canonical] = struct{}{}
		fp := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
		if prev, ok := s.files[canonical]; ok && prev.fingerprint == fp {
			return nil
		}
		stale = append(stale, candidate{fsPath: path, canonical: canonical, fingerprint: fp})
		return nil
	})
	if err != nil {
		return StructureStats{}, err
	}
	for canonical := range s.files {
		if _, ok := seen[canonical]; !ok {
			delete(s.files, canonical)
		}
	}
	s.refreshModuleLocked()

	if len(stale) > 0 {
		parsed := make([]*structFile, len(stale))
		workers := max(min(runtime.NumCPU(), 16), 2)
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for i, c := range stale {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				parsed[i] = parseStructFile(c.fsPath, c.canonical, c.fingerprint)
			})
		}
		wg.Wait()
		for i, c := range stale {
			next := parsed[i]
			if next == nil {
				delete(s.files, c.canonical)
				continue
			}
			if !next.parsed {
				if prev, ok := s.files[c.canonical]; ok {
					if prev.parsed {
						next.lastGood = prev.symbols
					} else {
						next.lastGood = prev.lastGood
					}
				}
			}
			s.files[c.canonical] = next
		}
		s.assignRefsLocked()
	}

	stats := StructureStats{Files: len(s.files), Reparsed: len(stale), RefreshTook: time.Since(started)}
	for _, f := range s.files {
		stats.Symbols += len(f.symbols)
		stats.Calls += len(f.calls)
		if !f.parsed {
			stats.Broken++
		}
	}
	stats.RefreshTookS = stats.RefreshTook.Round(time.Millisecond).String()
	s.lastRefresh, s.lastParsed = time.Now(), len(stale)
	if len(stale) > 0 {
		logging.World("structure index refreshed: files=%d symbols=%d calls=%d reparsed=%d broken=%d in %v",
			stats.Files, stats.Symbols, stats.Calls, stats.Reparsed, stats.Broken, stats.RefreshTook)
	}
	return stats, nil
}

// refreshModuleLocked reads the module path from go.mod when it changed.
func (s *StructureIndex) refreshModuleLocked() {
	path := filepath.Join(s.root, "go.mod")
	info, err := os.Stat(path)
	if err != nil {
		s.module, s.moduleFP = "", ""
		return
	}
	fp := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	if fp == s.moduleFP {
		return
	}
	s.moduleFP = fp
	s.module = ""
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			s.module = strings.Trim(strings.TrimSpace(rest), `"`)
			return
		}
	}
}

// assignRefsLocked gives every symbol its workspace ref. A Go ref is its
// directory and key; two files of one directory declaring the same key (build
// tag twins, an init per file) get the file name as a discriminator.
func (s *StructureIndex) assignRefsLocked() {
	counts := make(map[string]int)
	for _, f := range s.files {
		if f.lang != codemodel.LangGo {
			continue
		}
		for _, sym := range f.symbols {
			counts[f.dir+"\x00"+sym.Key]++
		}
	}
	for canonical, f := range s.files {
		for i := range f.symbols {
			f.symbols[i].Ref = refFor(f, canonical, f.symbols[i].Key, counts[f.dir+"\x00"+f.symbols[i].Key] > 1)
		}
		for i := range f.lastGood {
			f.lastGood[i].Ref = refFor(f, canonical, f.lastGood[i].Key, true)
		}
	}
}

func refFor(f *structFile, canonical, key string, ambiguous bool) string {
	if f.lang != codemodel.LangGo {
		return canonical + ":" + key
	}
	ref := dirRefPrefix(f.dir) + key
	if ambiguous {
		ref += "@" + filepath.Base(canonical)
	}
	return ref
}

// dirRefPrefix is the directory part of a Go ref: "internal/world." for a
// directory, "./" for the workspace root.
func dirRefPrefix(dir string) string {
	dir = strings.Trim(dir, "/")
	if dir == "" || dir == "." {
		return "./"
	}
	return dir + "."
}

func parseStructFile(fsPath, canonical, fingerprint string) *structFile {
	data, err := os.ReadFile(fsPath)
	if err != nil {
		return nil
	}
	f := &structFile{
		fingerprint: fingerprint,
		lang:        codemodel.LanguageOf(canonical),
		dir:         filepath.ToSlash(filepath.Dir(canonical)),
		importNames: make(map[string]string),
		idents:      make(map[string]int),
	}
	if f.lang == codemodel.LangMangle {
		model := codemodel.ParseMangle(canonical, string(data))
		f.parsed = model.Parsed
		for _, se := range model.Errors {
			f.errors = append(f.errors, se.Msg)
		}
		for i := range model.Elements {
			e := &model.Elements[i]
			sym := symbolFromElement(e, canonical, "", f.dir, f.lang)
			sym.ID = e.Key
			sym.bodyPreds = codemodel.MangleBodyPredicates(model.Text(e))
			f.symbols = append(f.symbols, sym)
		}
		return f
	}

	model, fset, node := codemodel.ParseGoAST(canonical, string(data))
	f.pkg = model.Package
	f.parsed = model.Parsed
	for _, se := range model.Errors {
		f.errors = append(f.errors, fmt.Sprintf("line %d:%d: %s", se.Line, se.Column, se.Msg))
	}
	f.imports = model.Imports
	for _, imp := range model.Imports {
		f.importNames[imp.LocalName()] = imp.Path
	}
	for i := range model.Elements {
		e := &model.Elements[i]
		if e.Kind == codemodel.KindHeader {
			continue
		}
		f.symbols = append(f.symbols, symbolFromElement(e, canonical, f.pkg, f.dir, f.lang))
	}
	if node == nil {
		return f
	}

	line := func(p token.Pos) int { return fset.Position(p).Line }
	// A call belongs to the innermost element whose span holds its line;
	// calls inside a closure belong to the declaration that holds it, and
	// calls in a package-level initializer to that var: every cobra
	// command's RunE closure lives in one, and before 2026-09-21 those calls
	// were never walked, so callers_of answered "tests only" for functions
	// the CLI calls in production.
	owner := func(l int) (id, key string) {
		for i := range f.symbols {
			sym := &f.symbols[i]
			if l >= sym.StartLine && l <= sym.EndLine && sym.Kind != string(codemodel.KindSyntaxError) {
				return sym.ID, sym.Key
			}
		}
		return f.pkg + ".init", ""
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			f.idents[x.Name]++
		case *ast.CallExpr:
			l := line(x.Pos())
			caller, key := owner(l)
			switch fun := x.Fun.(type) {
			case *ast.Ident:
				f.calls = append(f.calls, StructCall{Caller: caller, callerKey: key, File: canonical, Line: l, Name: fun.Name})
			case *ast.SelectorExpr:
				qualifier := ""
				if id, ok := fun.X.(*ast.Ident); ok {
					qualifier = id.Name
				}
				f.calls = append(f.calls, StructCall{Caller: caller, callerKey: key, File: canonical, Line: l, Qualifier: qualifier, Name: fun.Sel.Name})
			}
		}
		return true
	})
	return f
}

func symbolFromElement(e *codemodel.Element, canonical, pkg, dir, lang string) StructSymbol {
	sym := StructSymbol{
		Key: e.Key, Kind: string(e.Kind), File: canonical,
		StartLine: e.StartLine, EndLine: e.EndLine,
		Signature: e.Signature, Doc: e.Doc, Revision: e.Revision, Exported: e.Exported,
		name: e.Name, receiver: e.Receiver, pkg: pkg, dir: dir, lang: lang,
	}
	if e.Kind == codemodel.KindSyntaxError {
		sym.Doc = e.Err
	}
	if lang == codemodel.LangGo {
		sym.ID = pkg + "." + e.Name
		if e.Receiver != "" {
			sym.ID = pkg + "." + e.Receiver + "." + e.Name
		}
	}
	return sym
}

// matchSymbol says whether a query names this symbol. A query is a ref as
// the tools print it (internal/world.StructureIndex.Refresh, with or without
// its @file discriminator), a file-scoped ref (internal/world/x.go:Name), a
// bare name ("evaluate"), a receiver-qualified method
// ("RealKernel.evaluate"), a package-qualified name ("core.NewRealKernel"),
// a Cartographer ID, or for Mangle a predicate name or pred/arity.
func (sym *StructSymbol) matchSymbol(query string) bool {
	if query == sym.Ref || query == sym.ID || query == sym.name || query == sym.Key {
		return true
	}
	if base, _, ok := strings.Cut(sym.Ref, "@"); ok && query == base {
		return true
	}
	if query == sym.File+":"+sym.Key {
		return true
	}
	if sym.lang == codemodel.LangMangle {
		pred, _, _ := strings.Cut(strings.TrimPrefix(sym.Key, sym.Kind+":"), "@")
		return query == pred
	}
	if sym.receiver != "" && query == sym.receiver+"."+sym.name {
		return true
	}
	return query == sym.pkg+"."+sym.name
}

func sortSymbols(symbols []StructSymbol) {
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].File != symbols[j].File {
			return symbols[i].File < symbols[j].File
		}
		return symbols[i].StartLine < symbols[j].StartLine
	})
}

// SymbolFilter is what FindSymbol narrows by. Every set field narrows.
type SymbolFilter struct {
	Names   []string
	Pattern string
	Kind    string
	Path    string
}

// FindSymbol returns every declaration the filter names.
func (s *StructureIndex) FindSymbol(ctx context.Context, q SymbolFilter) ([]StructSymbol, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	var re *regexp.Regexp
	if q.Pattern != "" {
		re, err = regexp.Compile(q.Pattern)
		if err != nil {
			return nil, stats, fmt.Errorf("pattern %q is not a regular expression: %w", q.Pattern, err)
		}
	}
	scope := s.scopeLocked(q.Path)
	var out []StructSymbol
	for canonical, f := range s.files {
		if !scope(canonical, f) {
			continue
		}
		for i := range f.symbols {
			sym := &f.symbols[i]
			if sym.Kind == string(codemodel.KindSyntaxError) {
				continue
			}
			if q.Kind != "" && !strings.EqualFold(q.Kind, sym.Kind) {
				continue
			}
			matched := len(q.Names) == 0 && re != nil && re.MatchString(sym.name)
			for _, name := range q.Names {
				if sym.matchSymbol(strings.TrimSpace(name)) {
					matched = true
					break
				}
			}
			if matched && len(q.Names) > 0 && re != nil && !re.MatchString(sym.name) {
				matched = false
			}
			if matched {
				out = append(out, *sym)
			}
		}
	}
	sortSymbols(out)
	return out, stats, nil
}

// scopeLocked returns a predicate over files for a path filter: a file, or a
// directory searched recursively. An empty path is the whole workspace.
func (s *StructureIndex) scopeLocked(path string) func(string, *structFile) bool {
	target := s.target(path)
	if target == "" {
		return func(string, *structFile) bool { return true }
	}
	return func(canonical string, f *structFile) bool {
		dir := strings.Trim(f.dir, "/.")
		return canonical == target || dir == target || strings.HasPrefix(dir, target+"/")
	}
}

func (s *StructureIndex) target(path string) string {
	target := strings.Trim(filepath.ToSlash(types.CanonicalPath(s.root, strings.TrimSpace(path))), "/")
	if target == "." {
		return ""
	}
	return target
}

// Outline returns every declaration in a file, or in the Go files directly
// inside a directory, in file and line order.
func (s *StructureIndex) Outline(ctx context.Context, path string) ([]StructSymbol, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	target := s.target(path)
	var out []StructSymbol
	for canonical, f := range s.files {
		if filepath.ToSlash(canonical) == target || (f.lang == codemodel.LangGo && strings.Trim(f.dir, "/.") == target) {
			out = append(out, f.symbols...)
		}
	}
	sortSymbols(out)
	return out, stats, nil
}

// Resolve returns the declarations a ref or short name names.
func (s *StructureIndex) Resolve(ctx context.Context, ref string) ([]StructSymbol, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	return s.resolveAnyLocked(strings.TrimSpace(ref)), stats, nil
}

// resolveAnyLocked prefers an exact ref (or the ref without its @file
// discriminator) over the short-name search forms, so a printed ref always
// resolves to what was printed.
func (s *StructureIndex) resolveAnyLocked(query string) []StructSymbol {
	var exact, loose []StructSymbol
	for _, f := range s.files {
		for i := range f.symbols {
			sym := &f.symbols[i]
			if sym.Kind == string(codemodel.KindSyntaxError) {
				continue
			}
			base, _, _ := strings.Cut(sym.Ref, "@")
			switch {
			case query == sym.Ref || query == base || query == sym.File+":"+sym.Key:
				exact = append(exact, *sym)
			case sym.matchSymbol(query):
				loose = append(loose, *sym)
			}
		}
	}
	if len(exact) > 0 {
		sortSymbols(exact)
		return exact
	}
	sortSymbols(loose)
	return loose
}

func (s *StructureIndex) resolveLocked(query string) []StructSymbol {
	var targets []StructSymbol
	for _, sym := range s.resolveAnyLocked(query) {
		if sym.Kind == "function" || sym.Kind == "method" {
			targets = append(targets, sym)
		}
	}
	return targets
}

// refOfCall is the ref of the element a call sits in.
func (s *StructureIndex) refOfCall(c StructCall) string {
	f := s.files[c.File]
	if f == nil || c.callerKey == "" {
		return c.Caller
	}
	for _, sym := range f.symbols {
		if sym.Key == c.callerKey {
			return sym.Ref
		}
	}
	return c.Caller
}

// Callers returns the call sites of the functions or methods the query names.
func (s *StructureIndex) Callers(ctx context.Context, query string) ([]StructSymbol, []StructCallerMatch, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, nil, stats, err
	}
	targets := s.resolveLocked(strings.TrimSpace(query))
	if len(targets) == 0 {
		return nil, nil, stats, nil
	}
	names := make(map[string][]StructSymbol)
	for _, t := range targets {
		names[t.name] = append(names[t.name], t)
	}
	var out []StructCallerMatch
	for _, f := range s.files {
		for _, call := range f.calls {
			candidates, ok := names[call.Name]
			if !ok {
				continue
			}
			match := ""
			for _, t := range candidates {
				switch {
				case call.Qualifier == "" && t.receiver == "" && t.dir == f.dir:
					match = "exact"
				case call.Qualifier != "" && t.receiver == "" && strings.HasSuffix(f.importNames[call.Qualifier], "/"+t.dir):
					match = "exact"
				case call.Qualifier != "" && t.receiver != "" && f.importNames[call.Qualifier] == "":
					if match == "" {
						match = "by-name"
					}
				}
				if match == "exact" {
					break
				}
			}
			if match != "" {
				m := StructCallerMatch{StructCall: call, Match: match}
				m.Caller = s.refOfCall(call)
				out = append(out, m)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Match != out[j].Match {
			return out[i].Match == "exact"
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return targets, out, stats, nil
}

// StructCallee is one call made from inside a symbol, with the declarations
// its name could refer to.
type StructCallee struct {
	Call       string   `json:"call"`
	Line       int      `json:"line"`
	Candidates []string `json:"candidates,omitempty"` // "ref @ file:line"
}

// Callees returns the calls made inside the functions or methods the query names.
func (s *StructureIndex) Callees(ctx context.Context, query string) ([]StructSymbol, []StructCallee, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, nil, stats, err
	}
	targets := s.resolveLocked(strings.TrimSpace(query))
	if len(targets) == 0 {
		return nil, nil, stats, nil
	}
	byName := make(map[string][]StructSymbol)
	for _, f := range s.files {
		for _, sym := range f.symbols {
			if sym.Kind == "function" || sym.Kind == "method" {
				byName[sym.name] = append(byName[sym.name], sym)
			}
		}
	}
	var out []StructCallee
	for _, t := range targets {
		f := s.files[t.File]
		if f == nil {
			continue
		}
		for _, call := range f.calls {
			if call.callerKey != t.Key || call.Line < t.StartLine || call.Line > t.EndLine {
				continue
			}
			label := call.Name
			if call.Qualifier != "" {
				label = call.Qualifier + "." + call.Name
			}
			callee := StructCallee{Call: label, Line: call.Line}
			for _, c := range byName[call.Name] {
				local := call.Qualifier == "" && c.receiver == "" && c.dir == f.dir
				imported := call.Qualifier != "" && c.receiver == "" && strings.HasSuffix(f.importNames[call.Qualifier], "/"+c.dir)
				method := call.Qualifier != "" && c.receiver != "" && f.importNames[call.Qualifier] == ""
				if local || imported || method {
					callee.Candidates = append(callee.Candidates, fmt.Sprintf("%s @ %s:%d", c.Ref, c.File, c.StartLine))
				}
			}
			if len(callee.Candidates) > 6 {
				callee.Candidates = append(callee.Candidates[:6], fmt.Sprintf("... %d more share the name", len(callee.Candidates)-6))
			}
			out = append(out, callee)
		}
	}
	return targets, out, stats, nil
}

// Unreferenced returns the declarations under a path whose name occurs
// nowhere in the tree except at its own declaration. It is conservative on
// purpose: a name used anywhere, as a call, a value or a field, counts as a
// reference, so everything it returns is unreferenced by name. Entry points the
// toolchain calls (main, init, Test*, Benchmark*, Example*, Fuzz*) are left out.
func (s *StructureIndex) Unreferenced(ctx context.Context, path string) ([]StructSymbol, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	uses := make(map[string]int)
	decls := make(map[string]int)
	for _, f := range s.files {
		for name, n := range f.idents {
			uses[name] += n
		}
		for _, sym := range f.symbols {
			if sym.lang == codemodel.LangGo {
				decls[sym.name]++
			}
		}
	}
	scope := s.scopeLocked(path)
	var out []StructSymbol
	for canonical, f := range s.files {
		if f.lang != codemodel.LangGo || !scope(canonical, f) {
			continue
		}
		for _, sym := range f.symbols {
			if sym.name == "_" || sym.Kind == string(codemodel.KindSyntaxError) || isToolchainEntryPoint(sym.name) || uses[sym.name] > decls[sym.name] {
				continue
			}
			out = append(out, sym)
		}
	}
	sortSymbols(out)
	return out, stats, nil
}

func isToolchainEntryPoint(name string) bool {
	if name == "main" || name == "init" || name == "TestMain" {
		return true
	}
	for _, prefix := range []string{"Test", "Benchmark", "Example", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
