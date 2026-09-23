package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/world/codemodel"
)

// =============================================================================
// STRUCTURE QUERIES BEYOND DECLARATIONS
// =============================================================================
// The questions the 2026-09-22 R8 audit found answered by grep because no
// structural verb asked them: who imports a package (3 of the 4 greps in 12
// runs), where a log message or comment sits (a brief quotes one; the model's
// first moves were whole-file reads), and where a symbol is used (what a
// delete or a repoint must rewrite).

// StructImporter is one file importing a package.
type StructImporter struct {
	File string
	Name string // the local name the file uses
	Line int
}

// Importers returns the import path pkg resolves to and every file importing
// it. pkg is a workspace directory (internal/features), an import path
// (codenerd/internal/features, gopkg.in/yaml.v3) or a package name.
func (s *StructureIndex) Importers(ctx context.Context, pkg string) (string, []StructImporter, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return "", nil, stats, err
	}
	path, err := s.importPathLocked(pkg)
	if err != nil {
		return "", nil, stats, err
	}
	var out []StructImporter
	for canonical, f := range s.files {
		for _, imp := range f.imports {
			if imp.Path == path {
				out = append(out, StructImporter{File: canonical, Name: imp.LocalName(), Line: imp.Line})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return path, out, stats, nil
}

// importPathLocked resolves a package argument to an import path.
func (s *StructureIndex) importPathLocked(pkg string) (string, error) {
	pkg = strings.Trim(strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(pkg)), "./"), "/")
	if pkg == "" {
		return "", fmt.Errorf("package is required")
	}
	if s.module != "" && (pkg == s.module || strings.HasPrefix(pkg, s.module+"/")) {
		return pkg, nil
	}
	dirs := make(map[string]string) // dir -> package name
	for _, f := range s.files {
		if f.lang == codemodel.LangGo && !strings.HasSuffix(f.pkg, "_test") {
			dirs[strings.Trim(f.dir, "/.")] = f.pkg
		}
	}
	if _, ok := dirs[pkg]; ok {
		return s.importPathOfDir(pkg), nil
	}
	var named []string
	for dir, name := range dirs {
		if name == pkg {
			named = append(named, dir)
		}
	}
	sort.Strings(named)
	switch len(named) {
	case 0:
		return pkg, nil // an external or standard-library path, taken as given
	case 1:
		return s.importPathOfDir(named[0]), nil
	}
	return "", fmt.Errorf("package name %q is declared in %d directories: %s; name the directory", pkg, len(named), strings.Join(named, ", "))
}

func (s *StructureIndex) importPathOfDir(dir string) string {
	dir = strings.Trim(dir, "/.")
	switch {
	case s.module == "":
		return dir
	case dir == "":
		return s.module
	}
	return s.module + "/" + dir
}

// StructTextHit is one literal, comment or identifier holding searched text.
type StructTextHit struct {
	File string
	Line int
	In   string
	Text string
	Ref  string
}

// FindText searches the string literals, comments or identifiers of every
// parsed file under path for text, and answers with the element each hit sits
// in. The search is textual; the answer is an address in the element model,
// never a raw line range to read.
func (s *StructureIndex) FindText(ctx context.Context, text, in, path string) ([]StructTextHit, StructureStats, error) {
	if text == "" {
		return nil, StructureStats{}, fmt.Errorf("text is required")
	}
	in = strings.ToLower(strings.TrimSpace(in))
	wantStrings := in == "" || in == "all" || strings.HasPrefix(in, "string")
	wantComments := in == "" || in == "all" || strings.HasPrefix(in, "comment")
	wantIdents := in == "all" || strings.HasPrefix(in, "ident")
	if !wantStrings && !wantComments && !wantIdents {
		return nil, StructureStats{}, fmt.Errorf("in must be strings, comments, identifiers or all, got %q", in)
	}

	s.mu.Lock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, stats, err
	}
	type job struct {
		canonical string
		f         *structFile
	}
	scope := s.scopeLocked(path)
	var jobs []job
	for canonical, f := range s.files {
		if scope(canonical, f) {
			jobs = append(jobs, job{canonical, f})
		}
	}
	root := s.root
	s.mu.Unlock()

	results := make([][]StructTextHit, len(jobs))
	sem := make(chan struct{}, max(min(runtime.NumCPU(), 16), 2))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(j.canonical)))
			if err != nil {
				return
			}
			src := codemodel.Normalize(string(data))
			if !strings.Contains(src, text) {
				return
			}
			var hits []StructTextHit
			if j.f.lang == codemodel.LangGo {
				hits = scanGoText(src, text, wantStrings, wantComments, wantIdents)
			} else {
				hits = scanMangleText(src, text, wantStrings, wantComments, wantIdents)
			}
			for k := range hits {
				hits[k].File = j.canonical
				hits[k].Ref = enclosingRef(j.canonical, j.f, hits[k].Line)
			}
			results[i] = hits
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, stats, err
	}
	var out []StructTextHit
	for _, r := range results {
		out = append(out, r...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, stats, nil
}

func scanGoText(src, text string, wantStrings, wantComments, wantIdents bool) []StructTextHit {
	fset := token.NewFileSet()
	tf := fset.AddFile("find.go", -1, len(src))
	var sc scanner.Scanner
	sc.Init(tf, []byte(src), nil, scanner.ScanComments)
	var hits []StructTextHit
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		var in string
		switch {
		case (tok == token.STRING || tok == token.CHAR) && wantStrings:
			in = "string"
		case tok == token.COMMENT && wantComments:
			in = "comment"
		case tok == token.IDENT && wantIdents:
			in = "identifier"
		default:
			continue
		}
		idx := strings.Index(lit, text)
		if idx < 0 {
			continue
		}
		line := tf.Line(pos) + strings.Count(lit[:idx], "\n")
		hits = append(hits, StructTextHit{Line: line, In: in, Text: lineOfLiteral(lit, idx)})
	}
	return hits
}

// scanMangleText finds text in a Mangle file's "#" comments, string
// constants and identifiers.
func scanMangleText(src, text string, wantStrings, wantComments, wantIdents bool) []StructTextHit {
	var hits []StructTextHit
	for n, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, text) {
			continue
		}
		inString := false
		commentAt := -1
		for i := 0; i < len(line) && commentAt < 0; i++ {
			switch c := line[i]; {
			case inString && c == '\\':
				i++
			case c == '"':
				inString = !inString
			case !inString && c == '#':
				commentAt = i
			}
		}
		idx := strings.Index(line, text)
		in := ""
		switch {
		case commentAt >= 0 && idx >= commentAt && wantComments:
			in = "comment"
		case commentAt >= 0 && idx >= commentAt:
		case strings.Count(line[:idx], `"`)%2 == 1 && wantStrings:
			in = "string"
		case strings.Count(line[:idx], `"`)%2 == 0 && wantIdents:
			in = "identifier"
		}
		if in != "" {
			hits = append(hits, StructTextHit{Line: n + 1, In: in, Text: strings.TrimSpace(line)})
		}
	}
	return hits
}

// lineOfLiteral is the line of a literal or comment that holds the match,
// trimmed: the whole of a one-line literal, one line of a raw string or
// block comment.
func lineOfLiteral(lit string, idx int) string {
	start := strings.LastIndexByte(lit[:idx], '\n') + 1
	end := strings.IndexByte(lit[idx:], '\n')
	if end < 0 {
		end = len(lit)
	} else {
		end += idx
	}
	return strings.TrimSpace(lit[start:end])
}

// enclosingRef names the element holding a line: the innermost declaration,
// the file's header above the first one, or "" in a gap between them.
func enclosingRef(canonical string, f *structFile, line int) string {
	best := ""
	bestSpan := 0
	first := 0
	for _, sym := range f.symbols {
		if first == 0 || sym.StartLine < first {
			first = sym.StartLine
		}
		if line < sym.StartLine || line > sym.EndLine {
			continue
		}
		if span := sym.EndLine - sym.StartLine; best == "" || span < bestSpan {
			best, bestSpan = sym.Ref, span
		}
	}
	if best == "" && f.lang == codemodel.LangGo && (first == 0 || line < first) {
		return canonical + ":" + codemodel.HeaderKey
	}
	return best
}

// StructUse is one use of a symbol outside its own declaration.
type StructUse struct {
	File         string
	Line, Column int
	Start, End   int
	Qualifier    string
	Ref          string
	Match        string
}

// Uses returns the symbol a ref names and every use of it outside its own
// declaration: bare uses in its package, qualified uses in every importer.
// For a method, whose uses go through values of types the index does not
// know, uses are the selectors of that name in the files that also name the
// receiver type, marked by-name. For a Mangle Decl, they are the statements
// that derive or read the predicate.
func (s *StructureIndex) Uses(ctx context.Context, ref string) (*StructSymbol, []StructUse, StructureStats, error) {
	s.mu.Lock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, nil, stats, err
	}
	matches := s.resolveAnyLocked(strings.TrimSpace(ref))
	if len(matches) != 1 {
		s.mu.Unlock()
		if len(matches) == 0 {
			return nil, nil, stats, fmt.Errorf("no element is named %q; find_symbol searches by name", ref)
		}
		return nil, nil, stats, ambiguityError(ref, matches)
	}
	target := matches[0]

	if target.lang == codemodel.LangMangle {
		defer s.mu.Unlock()
		return &target, s.mangleUsesLocked(target), stats, nil
	}

	importPath := s.importPathOfDir(target.dir)
	type candidate struct {
		canonical string
		f         *structFile
		samePkg   bool
		// qualifiers are the names the file imports the target's package
		// under; a file may import one path under two names.
		qualifiers map[string]bool
	}
	var cands []candidate
	for canonical, f := range s.files {
		if f.lang != codemodel.LangGo {
			continue
		}
		if f.dir == target.dir && f.pkg == target.pkg {
			cands = append(cands, candidate{canonical: canonical, f: f, samePkg: true})
			continue
		}
		quals := make(map[string]bool)
		for _, imp := range f.imports {
			if imp.Path == importPath {
				quals[imp.LocalName()] = true
			}
		}
		if len(quals) > 0 {
			cands = append(cands, candidate{canonical: canonical, f: f, qualifiers: quals})
		}
	}
	root := s.root
	s.mu.Unlock()

	var out []StructUse
	for _, c := range cands {
		if err := ctx.Err(); err != nil {
			return nil, nil, stats, err
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.canonical)))
		if err != nil {
			continue
		}
		src := codemodel.Normalize(string(data))
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, c.canonical, src, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		tf := fset.File(node.Pos())
		inTarget := func(p token.Pos) bool {
			l := tf.Line(p)
			return c.canonical == target.File && l >= target.StartLine && l <= target.EndLine
		}
		add := func(start, end token.Pos, qualifier, match string) {
			l := tf.Line(start)
			out = append(out, StructUse{
				File: c.canonical, Line: l, Column: tf.Position(start).Column,
				Start: tf.Offset(start), End: tf.Offset(end),
				Qualifier: qualifier, Ref: enclosingRef(c.canonical, c.f, l), Match: match,
			})
		}
		if target.receiver != "" {
			if !mentionsName(node, target.receiver) {
				continue
			}
			ast.Inspect(node, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != target.name || inTarget(sel.Pos()) {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); ok && c.f.importNames[id.Name] != "" {
					return true
				}
				add(sel.Sel.Pos(), sel.Sel.End(), "", "by-name")
				return true
			})
			continue
		}
		if !c.samePkg {
			ast.Inspect(node, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != target.name {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); ok && c.qualifiers[id.Name] {
					add(sel.Pos(), sel.End(), id.Name, "exact")
				}
				return true
			})
			continue
		}
		for _, id := range bareUses(node, target.name) {
			if !inTarget(id.Pos()) {
				add(id.Pos(), id.End(), "", "exact")
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Start < out[j].Start
	})
	return &target, out, stats, nil
}

// bareUses returns the identifiers named name that refer to something rather
// than declare it or select a field: what a package-level name's uses look
// like inside its own package. A local that shadows the name is counted too;
// the answer errs toward reporting a use.
func bareUses(node *ast.File, name string) []*ast.Ident {
	skip := make(map[*ast.Ident]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			skip[x.Sel] = true
		case *ast.Field:
			for _, id := range x.Names {
				skip[id] = true
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				skip[id] = true
			}
		case *ast.TypeSpec:
			skip[x.Name] = true
		case *ast.FuncDecl:
			skip[x.Name] = true
		case *ast.ImportSpec:
			if x.Name != nil {
				skip[x.Name] = true
			}
		case *ast.LabeledStmt:
			skip[x.Label] = true
		case *ast.BranchStmt:
			if x.Label != nil {
				skip[x.Label] = true
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, lhs := range x.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						skip[id] = true
					}
				}
			}
		case *ast.RangeStmt:
			if x.Tok == token.DEFINE {
				if id, ok := x.Key.(*ast.Ident); ok {
					skip[id] = true
				}
				if id, ok := x.Value.(*ast.Ident); ok {
					skip[id] = true
				}
			}
		}
		return true
	})
	var out []*ast.Ident
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name && !skip[id] {
			out = append(out, id)
		}
		return true
	})
	return out
}

func mentionsName(node *ast.File, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func (s *StructureIndex) mangleUsesLocked(target StructSymbol) []StructUse {
	if target.Kind != string(codemodel.KindDecl) {
		return nil
	}
	var out []StructUse
	for canonical, f := range s.files {
		if f.lang != codemodel.LangMangle {
			continue
		}
		for _, sym := range f.symbols {
			if canonical == target.File && sym.Key == target.Key {
				continue
			}
			if sym.name == target.name || containsString(sym.bodyPreds, target.name) {
				out = append(out, StructUse{File: canonical, Line: sym.StartLine, Column: 1, Ref: sym.Ref, Match: "exact"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func ambiguityError(ref string, matches []StructSymbol) error {
	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		refs = append(refs, fmt.Sprintf("%s (%s:%d)", m.Ref, m.File, m.StartLine))
	}
	return fmt.Errorf("%q names %d elements; use one of these refs: %s", ref, len(matches), strings.Join(refs, ", "))
}

// StructFileStatus is what the index knows about one file's parse.
type StructFileStatus struct {
	Known    bool
	Parsed   bool
	Errors   []string
	LastGood []StructSymbol
}

// FileStatus reports one file's parse, with the declarations of its last
// clean parse when it does not parse now.
func (s *StructureIndex) FileStatus(ctx context.Context, path string) (StructFileStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.refreshLocked(ctx); err != nil {
		return StructFileStatus{}, err
	}
	f := s.files[s.target(path)]
	if f == nil {
		return StructFileStatus{}, nil
	}
	return StructFileStatus{Known: true, Parsed: f.parsed, Errors: append([]string(nil), f.errors...), LastGood: append([]StructSymbol(nil), f.lastGood...)}, nil
}

// CanonicalRefs maps the keys of one file's elements to their workspace
// refs, so a tool that parsed the file itself prints the refs the index
// resolves.
func (s *StructureIndex) CanonicalRefs(ctx context.Context, path string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.refreshLocked(ctx); err != nil {
		return nil, err
	}
	canonical := s.target(path)
	f := s.files[canonical]
	if f == nil {
		return nil, nil
	}
	out := make(map[string]string, len(f.symbols)+1)
	for _, sym := range f.symbols {
		out[sym.Key] = sym.Ref
	}
	if f.lang == codemodel.LangGo {
		out[codemodel.HeaderKey] = canonical + ":" + codemodel.HeaderKey
	}
	return out, nil
}

// importCandidate is one import path a qualifier could name.
type importCandidate struct {
	path    string
	uses    int // files importing it under that name
	sameDir int // of those, files in the asking file's directory
}

// indexResolver answers import derivation's questions for one file from a
// snapshot of the index, so it holds no lock while the edit runs.
type indexResolver struct {
	module   string
	dir      string
	declared map[string]bool
	byName   map[string]map[string]*importCandidate
}

// ImportResolver builds the resolver for the file at path.
func (s *StructureIndex) ImportResolver(ctx context.Context, path string) (codemodel.Resolver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.refreshLocked(ctx); err != nil {
		return nil, err
	}
	canonical := s.target(path)
	dir := filepath.ToSlash(filepath.Dir(canonical))
	pkg := ""
	if f := s.files[canonical]; f != nil {
		pkg = f.pkg
	}
	r := &indexResolver{module: s.module, dir: dir, declared: make(map[string]bool), byName: make(map[string]map[string]*importCandidate)}
	ownPath := s.importPathOfDir(dir)
	candidate := func(name, path string) *importCandidate {
		m := r.byName[name]
		if m == nil {
			m = make(map[string]*importCandidate)
			r.byName[name] = m
		}
		c := m[path]
		if c == nil {
			c = &importCandidate{path: path}
			m[path] = c
		}
		return c
	}
	for fc, f := range s.files {
		if f.lang != codemodel.LangGo {
			continue
		}
		if f.dir == dir && (pkg == "" || f.pkg == pkg) && fc != canonical {
			for _, sym := range f.symbols {
				r.declared[sym.name] = true
			}
		}
		if f.pkg != "" && f.pkg != "main" && !strings.HasSuffix(f.pkg, "_test") {
			if p := s.importPathOfDir(f.dir); p != ownPath {
				candidate(f.pkg, p)
			}
		}
		for _, imp := range f.imports {
			name := imp.LocalName()
			if name == "_" || name == "." || imp.Path == ownPath {
				continue
			}
			c := candidate(name, imp.Path)
			c.uses++
			if f.dir == dir {
				c.sameDir++
			}
		}
	}
	return r, nil
}

func (r *indexResolver) ModulePath() string { return r.module }

func (r *indexResolver) DeclaredInPackage(name string) bool { return r.declared[name] }

// ResolveQualifier picks the import path a qualifier names: the only
// candidate; else the one the file's own directory already imports under that
// name; else one imported at least twice as often as any other. Otherwise it
// does not choose.
func (r *indexResolver) ResolveQualifier(q string) (string, []string) {
	m := r.byName[q]
	if len(m) == 0 {
		return "", nil
	}
	cands := make([]*importCandidate, 0, len(m))
	for _, c := range m {
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].sameDir != cands[j].sameDir {
			return cands[i].sameDir > cands[j].sameDir
		}
		if cands[i].uses != cands[j].uses {
			return cands[i].uses > cands[j].uses
		}
		return cands[i].path < cands[j].path
	})
	paths := make([]string, 0, len(cands))
	for _, c := range cands {
		paths = append(paths, c.path)
	}
	switch {
	case len(cands) == 1:
		return cands[0].path, paths
	case cands[0].sameDir > 0 && cands[0].sameDir > cands[1].sameDir:
		return cands[0].path, paths
	case cands[0].uses >= 2*max(cands[1].uses, 1):
		return cands[0].path, paths
	}
	return "", paths
}

// StructPredicateRow is one statement concerning a Mangle predicate.
type StructPredicateRow struct {
	Role   string
	Symbol StructSymbol
}

// PredicateOutline returns every statement that declares, derives or reads a
// Mangle predicate, across all Mangle files: the Mangle analogue of
// find_symbol and callers_of together.
func (s *StructureIndex) PredicateOutline(ctx context.Context, pred string) ([]StructPredicateRow, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	pred = strings.TrimSpace(pred)
	if i := strings.IndexByte(pred, '/'); i >= 0 {
		pred = pred[:i]
	}
	order := map[string]int{"declares": 0, "derives": 1, "reads": 2}
	var out []StructPredicateRow
	for _, f := range s.files {
		if f.lang != codemodel.LangMangle {
			continue
		}
		for _, sym := range f.symbols {
			switch {
			case sym.name == pred && sym.Kind == string(codemodel.KindDecl):
				out = append(out, StructPredicateRow{"declares", sym})
			case sym.name == pred:
				out = append(out, StructPredicateRow{"derives", sym})
			case containsString(sym.bodyPreds, pred):
				out = append(out, StructPredicateRow{"reads", sym})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if order[out[i].Role] != order[out[j].Role] {
			return order[out[i].Role] < order[out[j].Role]
		}
		if out[i].Symbol.File != out[j].Symbol.File {
			return out[i].Symbol.File < out[j].Symbol.File
		}
		return out[i].Symbol.StartLine < out[j].Symbol.StartLine
	})
	return out, stats, nil
}
