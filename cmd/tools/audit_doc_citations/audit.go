package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/docdecl"
)

// defaultDocs is the docs tree the R6 grader scores. It is the flag default
// the command documents, not a runtime tunable.
const defaultDocs = "Docs/architecture"

const (
	kindMissingPath          = "missing-path"
	kindLineOutOfRange       = "line-out-of-range"
	kindSymbolNotDeclared    = "symbol-not-declared"
	kindPredicateNotDeclared = "predicate-not-declared"
)

// Options selects the tree to grade.
type Options struct {
	// RepoRoot is the module root citations are resolved against.
	RepoRoot string
	// DocsRoot is the docs tree, relative to RepoRoot unless absolute.
	// Empty means Docs/architecture.
	DocsRoot string
}

// Report is the whole grade. Findings is one entry per broken citation, in
// document order. ByKind counts those entries and is present for every kind
// even when the count is zero.
type Report struct {
	RepoRoot    string         `json:"repo_root"`
	DocsRoot    string         `json:"docs_root"`
	DocsScanned int            `json:"docs_scanned"`
	Citations   int            `json:"citations"`
	Broken      int            `json:"broken"`
	ByKind      map[string]int `json:"by_kind"`
	Findings    []Finding      `json:"findings"`
}

// Finding is one broken citation.
type Finding struct {
	Doc      string `json:"doc"`
	Line     int    `json:"line"`
	Citation string `json:"citation"`
	Kind     string `json:"kind"`
	Reason   string `json:"reason"`
}

func newReport() Report {
	return Report{
		Findings: []Finding{},
		ByKind: map[string]int{
			kindMissingPath:          0,
			kindLineOutOfRange:       0,
			kindSymbolNotDeclared:    0,
			kindPredicateNotDeclared: 0,
		},
	}
}

func (r *Report) add(f Finding) {
	f.Reason = oneLine(f.Reason)
	r.Findings = append(r.Findings, f)
	r.Broken++
	if r.ByKind == nil {
		r.ByKind = map[string]int{}
	}
	r.ByKind[f.Kind]++
}

// Audit grades opts. A broken citation is a finding, not an error. An error
// means the tree could not be read or a cited package directory could not be
// listed; the command exits 2 rather than reporting a clean tree.
func Audit(opt Options) (Report, error) {
	if strings.TrimSpace(opt.RepoRoot) == "" {
		return Report{}, errors.New("repo root is empty")
	}
	root, err := filepath.Abs(opt.RepoRoot)
	if err != nil {
		return Report{}, err
	}
	st, err := os.Stat(root)
	if err != nil {
		return Report{}, fmt.Errorf("repo root %s: %w", opt.RepoRoot, err)
	}
	if !st.IsDir() {
		return Report{}, fmt.Errorf("repo root %s is not a directory", opt.RepoRoot)
	}
	docsSpec := opt.DocsRoot
	if strings.TrimSpace(docsSpec) == "" {
		docsSpec = defaultDocs
	}
	docs := docsSpec
	if !filepath.IsAbs(docs) {
		docs = filepath.Join(root, filepath.FromSlash(docs))
	}
	docs, err = filepath.Abs(docs)
	if err != nil {
		return Report{}, err
	}
	st, err = os.Stat(docs)
	if err != nil {
		return Report{}, fmt.Errorf("docs root %s: %w", docsSpec, err)
	}
	if !st.IsDir() {
		return Report{}, fmt.Errorf("docs root %s is not a directory", docsSpec)
	}

	preds, err := docdecl.LoadPredicates(root)
	if err != nil {
		return Report{}, fmt.Errorf("mangle corpus: %w", err)
	}
	dirs, err := indexPackageDirs(root)
	if err != nil {
		return Report{}, fmt.Errorf("package index: %w", err)
	}
	c := &checker{
		root:  root,
		preds: preds,
		dirs:  dirs,
		pkgs:  map[string]*docdecl.Package{},
		lines: map[string]int{},
	}
	files, err := markdownFiles(docs)
	if err != nil {
		return Report{}, err
	}
	rep := newReport()
	rep.RepoRoot = filepath.ToSlash(root)
	rep.DocsRoot = relSlash(root, docs)
	rep.DocsScanned = len(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return Report{}, fmt.Errorf("read %s: %w", f, err)
		}
		docRel := relSlash(root, f)
		cites, findings, err := c.scanText(docRel, string(b))
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", docRel, err)
		}
		rep.Citations += cites
		for _, finding := range findings {
			rep.add(finding)
		}
	}
	return rep, nil
}

type checker struct {
	root  string
	preds map[string]bool
	dirs  map[string][]string
	pkgs  map[string]*docdecl.Package
	lines map[string]int
	// stdCache holds parsed standard-library packages keyed by qualifier.
	// A nil value means the qualifier was looked up and is not a stdlib package.
	stdCache map[string]*docdecl.Package
}

func (c *checker) scanText(docRel, text string) (int, []Finding, error) {
	var cites int
	var findings []Finding
	for i, line := range docdecl.SplitLines(text) {
		n, fs, err := c.scanLine(docRel, i+1, line)
		if err != nil {
			return 0, nil, err
		}
		cites += n
		findings = append(findings, fs...)
	}
	return cites, findings, nil
}

func (c *checker) scanLine(doc string, lineNo int, line string) (int, []Finding, error) {
	paths := auditLexer.Paths(line)
	syms := docdecl.Symbols(line)
	bound, unbound := docdecl.Bind(line, paths, syms)
	var cites int
	var findings []Finding
	for i, p := range paths {
		n, fs, err := c.judgePath(doc, lineNo, p, bound[i])
		if err != nil {
			return 0, nil, err
		}
		cites += n
		findings = append(findings, fs...)
	}
	for _, sym := range unbound {
		n, f, err := c.judgeGlobal(doc, lineNo, sym.Sym)
		if err != nil {
			return 0, nil, err
		}
		cites += n
		if f != nil {
			findings = append(findings, *f)
		}
	}
	return cites, findings, nil
}

// judgePath checks one path citation and the symbols bound to it.
// A path that is not in the tree is one broken citation: there is no file
// or package to look a symbol up in, so the symbols are not graded again.
func (c *checker) judgePath(doc string, lineNo int, p docdecl.PathHit, syms []docdecl.SymHit) (int, []Finding, error) {
	cites := 1
	full := absJoin(c.root, p.Rel)
	full, err := filepath.Abs(full)
	if err != nil {
		return 0, nil, err
	}
	if !withinRoot(c.root, full) {
		return cites, []Finding{{
			Doc: doc, Line: lineNo, Citation: p.Raw, Kind: kindMissingPath,
			Reason: "path escapes the repository",
		}}, nil
	}
	st, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cites, []Finding{{
				Doc: doc, Line: lineNo, Citation: p.Raw, Kind: kindMissingPath,
				Reason: "file does not exist",
			}}, nil
		}
		return 0, nil, fmt.Errorf("stat %s: %w", p.Rel, err)
	}
	if !st.Mode().IsRegular() {
		return cites, []Finding{{
			Doc: doc, Line: lineNo, Citation: p.Raw, Kind: kindMissingPath,
			Reason: "not a file",
		}}, nil
	}
	var findings []Finding
	if len(p.Lines) > 0 {
		n, err := c.lineCount(p.Rel)
		if err != nil {
			return 0, nil, err
		}
		if reason := lineReason(p.Lines, n); reason != "" {
			findings = append(findings, Finding{
				Doc: doc, Line: lineNo, Citation: p.Raw, Kind: kindLineOutOfRange, Reason: reason,
			})
		}
	}
	// The line check and the symbol check are separate citations. A line past
	// EOF does not hide a symbol that is also not declared.
	seen := map[string]bool{}
	var extra []string
	if p.Hash != "" {
		extra = append(extra, p.Hash)
		seen[p.Hash] = true
	}
	for _, s := range syms {
		if seen[s.Sym] {
			continue
		}
		seen[s.Sym] = true
		extra = append(extra, s.Sym)
	}
	for _, sym := range extra {
		f, skip, bad, err := c.judgeBoundSymbol(doc, lineNo, sym, p)
		if err != nil {
			return 0, nil, err
		}
		if skip {
			continue
		}
		cites++
		if bad {
			findings = append(findings, f)
		}
	}
	return cites, findings, nil
}

// judgeBoundSymbol grades one symbol bound to a path.
// skip is true when the text is not a citation of that path: a filename, or a
// standard-library selector (`fmt.Errorf`) written next to the file that calls
// it. bad is true when it is a citation and the declaration is missing.
func (c *checker) judgeBoundSymbol(doc string, lineNo int, sym string, p docdecl.PathHit) (Finding, bool, bool, error) {
	if docdecl.FileShapedSymbol(sym) {
		return Finding{}, true, false, nil
	}
	cit := "`" + sym + "` at " + p.Raw
	switch strings.ToLower(filepath.Ext(p.Rel)) {
	case ".mg":
		name := predicateName(sym)
		if c.preds[name] {
			return Finding{}, false, false, nil
		}
		return Finding{
			Doc: doc, Line: lineNo, Citation: cit, Kind: kindPredicateNotDeclared,
			Reason: fmt.Sprintf("no Decl `%s` in the .mg corpus", name),
		}, false, true, nil
	default:
		dir := pkgDir(p.Rel)
		pk, err := c.loadPkg(dir)
		if err != nil {
			return Finding{}, false, false, err
		}
		// The qualifier is this package, or a type declared in it (`Engine.Cache`).
		// Anything else that the standard library declares is the library the
		// file calls, not a declaration the file was supposed to contain.
		if c.foreignStdlib(pk, sym) {
			return Finding{}, true, false, nil
		}
		if pk.Has(sym) {
			return Finding{}, false, false, nil
		}
		reason := fmt.Sprintf("`%s` is not declared in %s or package %s", sym, p.Rel, pk.DirBase)
		if pe, ok := pk.ParseErr[p.Rel]; ok {
			reason += " (cited file did not parse: " + pe + ")"
		}
		return Finding{
			Doc: doc, Line: lineNo, Citation: cit, Kind: kindSymbolNotDeclared,
			Reason: oneLine(reason),
		}, false, true, nil
	}
}

// foreignStdlib reports whether sym is `pkg.Sel` for a standard-library
// package that is not the cited file's package and not a name the cited
// package declares. `pkg.Good` and `Engine.Cache` stay citations of the file.
func (c *checker) foreignStdlib(pk *docdecl.Package, sym string) bool {
	q, rest, ok := splitPkg(sym)
	if !ok || pk == nil {
		return false
	}
	if pk.PkgNames[q] || q == pk.DirBase || pk.Names[q] {
		return false
	}
	return c.stdlibHas(q, rest)
}

// judgeGlobal checks a symbol that is not sitting next to a path.
// `name/arity` is a predicate citation. `pkg.Func` is a Go citation when pkg
// is a directory under internal/ or cmd/ and the standard library does not
// declare that selector. `sync.Map` stays the standard library even when the
// tree has internal/prompt/sync; `sync.Local` in that package is still checked.
// Anything else (a prose backtick, encoding/json with no repo directory of
// that name, a bare CamelCase word, a filename) is not a citation.
func (c *checker) judgeGlobal(doc string, lineNo int, sym string) (int, *Finding, error) {
	if docdecl.PredicateShaped(sym) {
		name := predicateName(sym)
		if c.preds[name] {
			return 1, nil, nil
		}
		f := Finding{
			Doc: doc, Line: lineNo,
			Citation: "`" + sym + "`",
			Kind:     kindPredicateNotDeclared,
			Reason:   fmt.Sprintf("no Decl `%s` in the .mg corpus", name),
		}
		return 1, &f, nil
	}
	pkg, rest, ok := splitPkg(sym)
	if !ok {
		return 0, nil, nil
	}
	dirs := c.dirs[pkg]
	if len(dirs) == 0 {
		return 0, nil, nil
	}
	if docdecl.FileShapedSymbol(sym) || c.stdlibHas(pkg, rest) {
		return 0, nil, nil
	}
	for _, dir := range dirs {
		pk, err := c.loadPkg(dir)
		if err != nil {
			return 0, nil, err
		}
		if pk.Has(sym) || pk.Has(rest) {
			return 1, nil, nil
		}
	}
	f := Finding{
		Doc: doc, Line: lineNo,
		Citation: "`" + sym + "`",
		Kind:     kindSymbolNotDeclared,
		Reason:   fmt.Sprintf("`%s` is not declared in package %s (%s)", sym, pkg, strings.Join(dirs, ", ")),
	}
	return 1, &f, nil
}

func (c *checker) lineCount(rel string) (int, error) {
	if n, ok := c.lines[rel]; ok {
		return n, nil
	}
	b, err := os.ReadFile(absJoin(c.root, rel))
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", rel, err)
	}
	n := len(docdecl.SplitLines(string(b)))
	c.lines[rel] = n
	return n, nil
}

// auditLexer reads the paths the grader scores: Go and Mangle files under
// internal/ and cmd/.
var auditLexer = docdecl.NewLexer([]string{"internal", "cmd"}, []string{"go", "mg"})

func splitPkg(sym string) (string, string, bool) {
	if strings.HasPrefix(sym, "(") || strings.Contains(sym, "/") {
		return "", "", false
	}
	i := strings.IndexByte(sym, '.')
	if i <= 0 || i == len(sym)-1 {
		return "", "", false
	}
	return sym[:i], sym[i+1:], true
}

func lineReason(spans [][2]int, n int) string {
	var parts []string
	for _, sp := range spans {
		start, end := sp[0], sp[1]
		switch {
		case end < start:
			parts = append(parts, fmt.Sprintf("range %d-%d ends before it starts", start, end))
		case start < 1 || end < 1:
			bad := start
			if bad >= 1 {
				bad = end
			}
			parts = append(parts, fmt.Sprintf("line %d is outside the file (%d lines)", bad, n))
		case start == end && start > n:
			parts = append(parts, fmt.Sprintf("line %d is past end of file (%d lines)", start, n))
		default:
			if start > n {
				parts = append(parts, fmt.Sprintf("line %d is past end of file (%d lines)", start, n))
			}
			if end != start && end > n {
				parts = append(parts, fmt.Sprintf("line %d is past end of file (%d lines)", end, n))
			}
		}
	}
	return strings.Join(parts, "; ")
}

func markdownFiles(docs string) ([]string, error) {
	var files []string
	docs = filepath.Clean(docs)
	err := filepath.WalkDir(docs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if filepath.Clean(path) != docs && skipDocsDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func skipDocsDir(name string) bool {
	return name == ".git" || name == "vendor" || name == "node_modules"
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "\r", " ")
}

func absJoin(root, slashRel string) string {
	return filepath.Join(root, filepath.FromSlash(slashRel))
}

func pkgDir(slashRel string) string {
	return filepath.ToSlash(filepath.Dir(slashRel))
}

func slashBase(p string) string {
	p = filepath.ToSlash(p)
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func relSlash(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

func withinRoot(root, full string) bool {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
