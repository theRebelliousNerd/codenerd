package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	gotypes "go/types"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// STRUCTURE INDEX
// =============================================================================
// The workspace-wide, symbol-level layer of the world model: every Go
// declaration with its line span, and every call site with its line.
//
// Why it exists. The deep facts the Cartographer produces (code_defines,
// code_calls) are computed for the active file and its one-hop neighbours, so
// "who calls X across the repository" had no facts behind it, and no tool a
// model can call read those facts anyway. Measured 2026-09-21 on a 24-task
// campaign: 166 raw filesystem calls (read_file, grep, glob, list_files)
// against 14 get_elements calls, because grep was the only tool that answered a
// question spanning files.
//
// It is a plain go/ast pass, without the Cartographer's data-flow extraction,
// so the whole tree is cheap to hold. Symbol IDs follow the Cartographer's
// convention (pkg.Name, pkg.Receiver.Name) so a ref means the same thing here
// and in code_defines.
//
// Freshness. Every query re-stats the tree and re-parses exactly the files
// whose fingerprint moved, so an answer never describes a file as it was
// before the last edit.

// StructSymbol is one declaration.
type StructSymbol struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // function, method, struct, interface, type, const, var
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc,omitempty"`
	Exported  bool   `json:"exported"`

	name     string
	receiver string
	pkg      string
}

// StructCall is one call site, attributed to the declaration containing it.
type StructCall struct {
	Caller    string `json:"caller"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Qualifier string `json:"-"` // identifier before the dot, empty for a bare call
	Name      string `json:"-"`
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
	pkg         string
	dir         string
	imports     map[string]string // local name -> import path
	symbols     []StructSymbol
	calls       []StructCall
	idents      map[string]int // identifier name -> occurrences, declarations included
}

// StructureIndex holds the parsed structure of every Go file under a root.
type StructureIndex struct {
	root string

	mu          sync.Mutex
	files       map[string]*structFile // canonical path -> parsed file
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
	RefreshTook  time.Duration `json:"-"`
	RefreshTookS string        `json:"refresh_took"`
}

var structureSkipDirs = map[string]struct{}{
	"vendor": {}, "node_modules": {}, "testdata": {},
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
			if path != s.root {
				if _, skip := structureSkipDirs[name]; skip || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
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
			if parsed[i] != nil {
				s.files[c.canonical] = parsed[i]
			} else {
				// A file that does not parse keeps no stale entry: an answer
				// from before the edit is worse than no answer.
				delete(s.files, c.canonical)
			}
		}
	}

	stats := StructureStats{Files: len(s.files), Reparsed: len(stale), RefreshTook: time.Since(started)}
	for _, f := range s.files {
		stats.Symbols += len(f.symbols)
		stats.Calls += len(f.calls)
	}
	stats.RefreshTookS = stats.RefreshTook.Round(time.Millisecond).String()
	s.lastRefresh, s.lastParsed = time.Now(), len(stale)
	if len(stale) > 0 {
		logging.World("structure index refreshed: files=%d symbols=%d calls=%d reparsed=%d in %v",
			stats.Files, stats.Symbols, stats.Calls, stats.Reparsed, stats.RefreshTook)
	}
	return stats, nil
}

func parseStructFile(fsPath, canonical, fingerprint string) *structFile {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fsPath, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil || node == nil {
		return nil
	}
	f := &structFile{
		fingerprint: fingerprint,
		pkg:         node.Name.Name,
		dir:         filepath.ToSlash(filepath.Dir(canonical)),
		imports:     make(map[string]string),
		idents:      make(map[string]int),
	}
	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		f.imports[local] = path
	}
	line := func(p token.Pos) int { return fset.Position(p).Line }
	add := func(sym StructSymbol) {
		sym.File = canonical
		sym.pkg = f.pkg
		sym.Exported = ast.IsExported(sym.name)
		f.symbols = append(f.symbols, sym)
	}
	// collectCalls records every call under node as made by caller, including
	// the calls inside function literals: a closure's calls belong to the
	// declaration that holds it.
	collectCalls := func(node ast.Node, caller string) {
		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				f.calls = append(f.calls, StructCall{Caller: caller, File: canonical, Line: line(call.Pos()), Name: fun.Name})
			case *ast.SelectorExpr:
				qualifier := ""
				if x, ok := fun.X.(*ast.Ident); ok {
					qualifier = x.Name
				}
				f.calls = append(f.calls, StructCall{Caller: caller, File: canonical, Line: line(call.Pos()), Qualifier: qualifier, Name: fun.Sel.Name})
			}
			return true
		})
	}

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := StructSymbol{name: d.Name.Name, Kind: "function", StartLine: line(d.Pos()), EndLine: line(d.End()),
				Doc: firstDocLine(d.Doc), Signature: funcSignature(fset, d)}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				sym.Kind = "method"
				sym.receiver = receiverTypeName(d.Recv.List[0].Type)
			}
			sym.ID = f.pkg + "." + sym.name
			if sym.receiver != "" {
				sym.ID = f.pkg + "." + sym.receiver + "." + sym.name
			}
			add(sym)
			if d.Body != nil {
				collectCalls(d.Body, sym.ID)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					kind := "type"
					switch sp.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
					doc := firstDocLine(sp.Doc)
					if doc == "" {
						doc = firstDocLine(d.Doc)
					}
					add(StructSymbol{name: sp.Name.Name, ID: f.pkg + "." + sp.Name.Name, Kind: kind,
						StartLine: line(sp.Pos()), EndLine: line(sp.End()), Doc: doc})
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					caller := ""
					for _, name := range sp.Names {
						if name.Name == "_" {
							continue
						}
						add(StructSymbol{name: name.Name, ID: f.pkg + "." + name.Name, Kind: kind,
							StartLine: line(name.Pos()), EndLine: line(sp.End())})
						if caller == "" {
							caller = f.pkg + "." + name.Name
						}
					}
					// Calls in an initializer are calls: every cobra command's
					// RunE closure lives in a package-level var, as do function
					// tables and `var x = f()`. They were never walked, so
					// callers_of answered "tests only" for functions the CLI
					// calls in production, and unreferenced_symbols called them
					// dead (2026-09-21: features.ConfigSchemaJSON, called from
					// cmd/nerd/cmd_features.go:37, reported with 3 test callers).
					if caller == "" {
						caller = f.pkg + ".init"
					}
					for _, value := range sp.Values {
						collectCalls(value, caller)
					}
				}
			}
		}
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			f.idents[id.Name]++
		}
		return true
	})
	return f
}

func firstDocLine(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	text := strings.TrimSpace(group.Text())
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return text
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

func funcSignature(_ *token.FileSet, d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString("(" + gotypes.ExprString(d.Recv.List[0].Type) + ") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString(gotypes.ExprString(d.Type)[len("func"):])
	return sb.String()
}

// matchSymbol says whether a query names this symbol. A query is a bare name
// ("evaluate"), a receiver-qualified method ("RealKernel.evaluate"), a
// package-qualified name ("core.NewRealKernel") or a full ID.
func (sym *StructSymbol) matchSymbol(query string) bool {
	if query == sym.ID || query == sym.name {
		return true
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

// FindSymbol returns every declaration the query names, optionally of one kind.
func (s *StructureIndex) FindSymbol(ctx context.Context, query, kind string) ([]StructSymbol, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	query = strings.TrimSpace(query)
	var out []StructSymbol
	for _, f := range s.files {
		for i := range f.symbols {
			sym := &f.symbols[i]
			if kind != "" && !strings.EqualFold(kind, sym.Kind) {
				continue
			}
			if sym.matchSymbol(query) {
				out = append(out, *sym)
			}
		}
	}
	sortSymbols(out)
	return out, stats, nil
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
	target := strings.Trim(filepath.ToSlash(types.CanonicalPath(s.root, path)), "/")
	if target == "." {
		target = ""
	}
	var out []StructSymbol
	for canonical, f := range s.files {
		if filepath.ToSlash(canonical) == target || strings.Trim(f.dir, "/.") == target {
			out = append(out, f.symbols...)
		}
	}
	sortSymbols(out)
	return out, stats, nil
}

func (s *StructureIndex) resolveLocked(query string) []StructSymbol {
	var targets []StructSymbol
	for _, f := range s.files {
		for i := range f.symbols {
			if f.symbols[i].matchSymbol(query) && (f.symbols[i].Kind == "function" || f.symbols[i].Kind == "method") {
				targets = append(targets, f.symbols[i])
			}
		}
	}
	sortSymbols(targets)
	return targets
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
				tdir := filepath.ToSlash(filepath.Dir(t.File))
				switch {
				case call.Qualifier == "" && t.receiver == "" && tdir == f.dir:
					match = "exact"
				case call.Qualifier != "" && t.receiver == "" && strings.HasSuffix(f.imports[call.Qualifier], "/"+tdir):
					match = "exact"
				case call.Qualifier != "" && t.receiver != "" && f.imports[call.Qualifier] == "":
					if match == "" {
						match = "by-name"
					}
				}
				if match == "exact" {
					break
				}
			}
			if match != "" {
				out = append(out, StructCallerMatch{StructCall: call, Match: match})
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
	Candidates []string `json:"candidates,omitempty"` // "id @ file:line"
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
			if call.Caller != t.ID || call.Line < t.StartLine || call.Line > t.EndLine {
				continue
			}
			label := call.Name
			if call.Qualifier != "" {
				label = call.Qualifier + "." + call.Name
			}
			callee := StructCallee{Call: label, Line: call.Line}
			for _, c := range byName[call.Name] {
				cdir := filepath.ToSlash(filepath.Dir(c.File))
				local := call.Qualifier == "" && c.receiver == "" && cdir == f.dir
				imported := call.Qualifier != "" && c.receiver == "" && strings.HasSuffix(f.imports[call.Qualifier], "/"+cdir)
				method := call.Qualifier != "" && c.receiver != "" && f.imports[call.Qualifier] == ""
				if local || imported || method {
					callee.Candidates = append(callee.Candidates, fmt.Sprintf("%s @ %s:%d", c.ID, c.File, c.StartLine))
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
			decls[sym.name]++
		}
	}
	target := strings.Trim(filepath.ToSlash(types.CanonicalPath(s.root, path)), "/")
	if target == "." {
		target = ""
	}
	var out []StructSymbol
	for canonical, f := range s.files {
		dir := strings.Trim(f.dir, "/.")
		if filepath.ToSlash(canonical) != target && dir != target && !strings.HasPrefix(dir, target+"/") && target != "" {
			continue
		}
		for _, sym := range f.symbols {
			if isToolchainEntryPoint(sym.name) || uses[sym.name] > decls[sym.name] {
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
