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
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
	"codenerd/internal/world/codemodel"
)

// =============================================================================
// STRUCTURE INDEX
// =============================================================================
// The workspace-wide, symbol-level layer of the world model: every Go,
// Python, TypeScript, TSX and JavaScript declaration and every Mangle
// statement with its line span and revision, and every call site with its line.
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
	// The fields below are filled for Python, TypeScript and JavaScript,
	// where a call is resolved in its scope. Go leaves them zero and matches
	// calls by package instead. They stay out of the tool JSON.
	Bound      bool   `json:"-"`
	Local      bool   `json:"-"`
	TargetFile string `json:"-"`
	TargetName string `json:"-"`
	TargetRecv string `json:"-"`
	JSX        bool   `json:"-"`
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

// StructureIndex holds the parsed structure of every Go, Mangle, Python,
// TypeScript and JavaScript file under a root.
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

	mem, err := workspace.For(s.root)
	if err != nil {
		return StructureStats{}, err
	}
	if err := mem.Refresh(); err != nil {
		return StructureStats{}, err
	}

	err = filepath.WalkDir(s.root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // an unreadable directory is skipped, not fatal
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		name := d.Name()
		member, admErr := mem.Admit(path, d.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if codemodel.LanguageOf(name) == "" {
			return nil
		}
		canonical := types.CanonicalPath(s.root, path)
		// A secret file (execution.secret_paths) is never parsed: its
		// signatures, doc lines and literals would reach the model through
		// find_symbol, package_outline and find_text as surely as a read.
		if tools.IsSecretPath(canonical) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
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

// assignRefsLocked gives every symbol its workspace ref. A Go, Python,
// TypeScript or JavaScript ref is its directory and key; two files of one
// directory declaring the same key (build tag twins, an init per file) get
// the file name as a discriminator. Mangle stays file-and-key.
func (s *StructureIndex) assignRefsLocked() {
	counts := make(map[string]int)
	for _, f := range s.files {
		if !dirRefs(f.lang) {
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
	if !dirRefs(f.lang) || fileScopedKey(key) {
		return canonical + ":" + key
	}
	ref := dirRefPrefix(f.dir) + key
	if ambiguous {
		ref += "@" + filepath.Base(canonical)
	}
	return ref
}

// dirRefs reports a language whose refs are directory-and-key, the way Go's are.
func dirRefs(lang string) bool {
	return lang == codemodel.LangGo || codemodel.IsScriptLang(lang)
}

// fileScopedKey is a pseudo-element the directory ref grammar does not name.
// A header and a broken region stay "file:key" in every language.
func fileScopedKey(key string) bool {
	return key == codemodel.HeaderKey || key == string(codemodel.KindSyntaxError) ||
		strings.HasPrefix(key, string(codemodel.KindSyntaxError)+"#")
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
	if codemodel.IsScriptLang(f.lang) {
		indexScriptFile(f, fsPath, canonical, string(data))
		return f
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
	if lang == codemodel.LangGo || codemodel.IsScriptLang(lang) {
		sym.ID = pkg + "." + e.Name
		if e.Receiver != "" {
			sym.ID = pkg + "." + e.Receiver + "." + e.Name
		}
	}
	// A component or a hook stays a function in the element model. The index
	// shows the role, which is the kind the tools print and filter on.
	if e.Role == "component" || e.Role == "hook" {
		sym.Kind = e.Role
	}
	return sym
}

// indexScriptFile fills one Python, TypeScript or JavaScript file from the
// element model. Imports are resolved against the file's real directory, and
// a call is owned by the innermost declaration whose lines hold it: a method
// sits inside its class, and the call belongs to the method.
func indexScriptFile(f *structFile, fsPath, canonical, src string) {
	root := workspaceRootOf(fsPath, canonical)
	model, ok := codemodel.ParseRoot(canonical, fsPath, root, src)
	if !ok || model == nil {
		f.parsed = false
		f.errors = append(f.errors, "no element model for "+canonical)
		return
	}
	f.pkg = strings.TrimSuffix(filepath.Base(canonical), filepath.Ext(canonical))
	f.parsed = model.Parsed
	for _, se := range model.Errors {
		f.errors = append(f.errors, fmt.Sprintf("line %d:%d: %s", se.Line, se.Column, se.Msg))
	}
	f.imports = model.Imports
	for _, imp := range model.Imports {
		path := imp.Path
		if len(imp.Imported) == 0 {
			if name := imp.LocalName(); name != "" {
				f.importNames[name] = path
			}
			continue
		}
		for _, n := range imp.Imported {
			if n.Local != "" {
				f.importNames[n.Local] = path
			}
		}
	}
	if model.Idents != nil {
		f.idents = model.Idents
	}
	for i := range model.Elements {
		e := &model.Elements[i]
		if e.Kind == codemodel.KindHeader {
			continue
		}
		f.symbols = append(f.symbols, symbolFromElement(e, canonical, f.pkg, f.dir, f.lang))
	}
	owner := func(l int) (id, key string) {
		bestSpan := int(^uint(0) >> 1)
		var best *StructSymbol
		for i := range f.symbols {
			sym := &f.symbols[i]
			if l < sym.StartLine || l > sym.EndLine || sym.Kind == string(codemodel.KindSyntaxError) {
				continue
			}
			span := sym.EndLine - sym.StartLine
			if span < bestSpan {
				best = sym
				bestSpan = span
			}
		}
		if best == nil {
			return f.pkg + ".init", ""
		}
		return best.ID, best.Key
	}
	for _, c := range model.Calls {
		id, key := owner(c.Line)
		f.calls = append(f.calls, StructCall{
			Caller: id, callerKey: key, File: canonical, Line: c.Line,
			Qualifier: c.Qualifier, Name: c.Name,
			Bound: c.Bound, Local: c.Local, JSX: c.JSX,
			TargetFile: c.TargetFile, TargetName: c.TargetName, TargetRecv: c.TargetRecv,
		})
	}
}

// workspaceRootOf recovers the workspace root from an absolute file path and
// the canonical identity the walk stored, so import resolution does not depend
// on the process working directory. The canonical path is a suffix of the
// absolute path, with a separator before it.
func workspaceRootOf(fsPath, canonical string) string {
	abs := filepath.Clean(fsPath)
	suffix := filepath.Clean(filepath.FromSlash(canonical))
	if suffix == "" || suffix == "." {
		return abs
	}
	if len(abs) <= len(suffix) || !strings.EqualFold(abs[len(abs)-len(suffix):], suffix) {
		return ""
	}
	if abs[len(abs)-len(suffix)-1] != os.PathSeparator {
		return ""
	}
	return abs[:len(abs)-len(suffix)-1]
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

// Outline returns every declaration in a file, or in the Go, Python,
// TypeScript and JavaScript files directly inside a directory, in file and
// line order.
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
		if filepath.ToSlash(canonical) == target || (dirRefs(f.lang) && strings.Trim(f.dir, "/.") == target) {
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
		if callableKind(sym.Kind) {
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
			candidates := names[call.Name]
			if codemodel.IsScriptLang(f.lang) && call.Bound && call.TargetName != "" && call.TargetName != call.Name {
				// An aliased import calls the local spelling (`ua()`) but
				// names the remote (`useAuth`). Both candidate lists are tried;
				// a bound call still matches only its resolved target.
				extra := names[call.TargetName]
				merged := make([]StructSymbol, 0, len(candidates)+len(extra))
				merged = append(merged, candidates...)
				merged = append(merged, extra...)
				candidates = merged
			}
			if len(candidates) == 0 {
				continue
			}
			match := ""
			for _, t := range candidates {
				if codemodel.IsScriptLang(f.lang) {
					m := scriptCallMatch(call, t)
					if m == "exact" {
						match = "exact"
						break
					}
					if m == "by-name" && match == "" {
						match = m
					}
					continue
				}
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
			if callableKind(sym.Kind) {
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
			if codemodel.IsScriptLang(f.lang) {
				callee.Candidates = scriptCalleeCandidates(call, byName)
			} else {
				for _, c := range byName[call.Name] {
					local := call.Qualifier == "" && c.receiver == "" && c.dir == f.dir
					imported := call.Qualifier != "" && c.receiver == "" && strings.HasSuffix(f.importNames[call.Qualifier], "/"+c.dir)
					method := call.Qualifier != "" && c.receiver != "" && f.importNames[call.Qualifier] == ""
					if local || imported || method {
						callee.Candidates = append(callee.Candidates, fmt.Sprintf("%s @ %s:%d", c.Ref, c.File, c.StartLine))
					}
				}
			}
			// The candidates are returned whole. callees_of pages the rendered
			// rows with an announced offset, and the working-context ledger
			// archives an oversized answer behind a recall handle; cutting here
			// hid valid resolution targets on overloaded names behind an
			// ellipsis the model could not redeem.
			out = append(out, callee)
		}
	}
	return targets, out, stats, nil
}

// Unreferenced returns the Go, Python, TypeScript and JavaScript declarations
// under a path whose name occurs nowhere in the tree except at its own
// declaration. It is conservative on purpose: a name used anywhere, as a call,
// a value or a field, counts as a reference, so everything it returns is
// unreferenced by name. Entry points the toolchain calls (main, init, Test*,
// Benchmark*, Example*, Fuzz*) are left out, as are Python dunder names.
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
			if sym.lang == codemodel.LangGo || codemodel.IsScriptLang(sym.lang) {
				decls[sym.name]++
			}
		}
	}
	scope := s.scopeLocked(path)
	var out []StructSymbol
	for canonical, f := range s.files {
		if (f.lang != codemodel.LangGo && !codemodel.IsScriptLang(f.lang)) || !scope(canonical, f) {
			continue
		}
		for _, sym := range f.symbols {
			if sym.name == "_" || sym.Kind == string(codemodel.KindSyntaxError) || isToolchainEntryPoint(sym.name) || pythonDunder(sym) || uses[sym.name] > decls[sym.name] {
				continue
			}
			out = append(out, sym)
		}
	}
	sortSymbols(out)
	return out, stats, nil
}

// callableKind is a declaration callers_of and callees_of will resolve.
// A hook or a component is still a function; the index stores the role as
// its kind so a query for the thing the tools printed finds it.
func callableKind(kind string) bool {
	switch kind {
	case "function", "method", "hook", "component":
		return true
	}
	return false
}

// scriptCallMatch ties a scoped call to one target. A bound call matches only
// the declaration scope resolution named, so an external module and a
// same-spelled local do not fall through onto every method of that name. An
// unbound qualifier is by-name, the same idea as a Go call through a value.
func scriptCallMatch(call StructCall, t StructSymbol) string {
	if call.Local {
		return ""
	}
	if call.Bound {
		if call.TargetFile != "" && call.TargetFile == t.File && call.TargetName == t.name && call.TargetRecv == t.receiver {
			return "exact"
		}
		return ""
	}
	if call.Qualifier != "" && t.receiver != "" && call.Name == t.name {
		return "by-name"
	}
	return ""
}

func scriptCalleeCandidates(call StructCall, byName map[string][]StructSymbol) []string {
	if call.Local {
		return nil
	}
	var out []string
	add := func(c StructSymbol) {
		out = append(out, fmt.Sprintf("%s @ %s:%d", c.Ref, c.File, c.StartLine))
	}
	if call.Bound && call.TargetName != "" {
		for _, c := range byName[call.TargetName] {
			if call.TargetFile != "" && c.File == call.TargetFile && c.name == call.TargetName && c.receiver == call.TargetRecv {
				add(c)
			}
		}
		return out
	}
	if call.Qualifier != "" {
		for _, c := range byName[call.Name] {
			if c.receiver != "" {
				add(c)
			}
		}
	}
	return out
}

// pythonDunder is a Python special method (__init__, __all__). The language
// calls it; a name count of one does not make it dead code.
func pythonDunder(sym StructSymbol) bool {
	return sym.lang == codemodel.LangPython && len(sym.name) > 4 && strings.HasPrefix(sym.name, "__") && strings.HasSuffix(sym.name, "__")
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
