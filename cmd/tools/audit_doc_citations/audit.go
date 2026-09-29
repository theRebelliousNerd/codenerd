package main

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

// adjacencyWindow is how close a backticked symbol must sit to a path before
// it is a citation of that path. It is the grammar of "next to" — a
// parenthetical `Sym` (`path`) or a short `path defines Sym` — not a cap on
// output or runtime. A sentence between them is prose.
const adjacencyWindow = 240

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

	preds, err := loadPredicates(root)
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
		pkgs:  map[string]*pkgDecls{},
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
	pkgs  map[string]*pkgDecls
	lines map[string]int
	// stdCache holds parsed standard-library packages keyed by qualifier.
	// A nil value means the qualifier was looked up and is not a stdlib package.
	stdCache map[string]*pkgDecls
}

func (c *checker) scanText(docRel, text string) (int, []Finding, error) {
	var cites int
	var findings []Finding
	for i, line := range splitLines(text) {
		n, fs, err := c.scanLine(docRel, i+1, line)
		if err != nil {
			return 0, nil, err
		}
		cites += n
		findings = append(findings, fs...)
	}
	return cites, findings, nil
}

type pathHit struct {
	start, end int
	rel        string
	raw        string
	lines      [][2]int
	hash       string
}

type symHit struct {
	start, end int
	sym        string
}

func (c *checker) scanLine(doc string, lineNo int, line string) (int, []Finding, error) {
	paths := findPaths(line)
	syms := findSyms(line)
	bound, unbound := bindSyms(line, paths, syms)
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
		n, f, err := c.judgeGlobal(doc, lineNo, sym.sym)
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
func (c *checker) judgePath(doc string, lineNo int, p pathHit, syms []symHit) (int, []Finding, error) {
	cites := 1
	full := absJoin(c.root, p.rel)
	full, err := filepath.Abs(full)
	if err != nil {
		return 0, nil, err
	}
	if !withinRoot(c.root, full) {
		return cites, []Finding{{
			Doc: doc, Line: lineNo, Citation: p.raw, Kind: kindMissingPath,
			Reason: "path escapes the repository",
		}}, nil
	}
	st, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cites, []Finding{{
				Doc: doc, Line: lineNo, Citation: p.raw, Kind: kindMissingPath,
				Reason: "file does not exist",
			}}, nil
		}
		return 0, nil, fmt.Errorf("stat %s: %w", p.rel, err)
	}
	if !st.Mode().IsRegular() {
		return cites, []Finding{{
			Doc: doc, Line: lineNo, Citation: p.raw, Kind: kindMissingPath,
			Reason: "not a file",
		}}, nil
	}
	var findings []Finding
	if len(p.lines) > 0 {
		n, err := c.lineCount(p.rel)
		if err != nil {
			return 0, nil, err
		}
		if reason := lineReason(p.lines, n); reason != "" {
			findings = append(findings, Finding{
				Doc: doc, Line: lineNo, Citation: p.raw, Kind: kindLineOutOfRange, Reason: reason,
			})
		}
	}
	// The line check and the symbol check are separate citations. A line past
	// EOF does not hide a symbol that is also not declared.
	seen := map[string]bool{}
	var extra []string
	if p.hash != "" {
		extra = append(extra, p.hash)
		seen[p.hash] = true
	}
	for _, s := range syms {
		if seen[s.sym] {
			continue
		}
		seen[s.sym] = true
		extra = append(extra, s.sym)
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
func (c *checker) judgeBoundSymbol(doc string, lineNo int, sym string, p pathHit) (Finding, bool, bool, error) {
	if fileShapedSymbol(sym) {
		return Finding{}, true, false, nil
	}
	cit := "`" + sym + "` at " + p.raw
	switch strings.ToLower(filepath.Ext(p.rel)) {
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
		dir := pkgDir(p.rel)
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
		if pk.has(sym) {
			return Finding{}, false, false, nil
		}
		reason := fmt.Sprintf("`%s` is not declared in %s or package %s", sym, p.rel, pk.dirBase)
		if pe, ok := pk.parseErr[p.rel]; ok {
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
func (c *checker) foreignStdlib(pk *pkgDecls, sym string) bool {
	q, rest, ok := splitPkg(sym)
	if !ok || pk == nil {
		return false
	}
	if pk.pkgNames[q] || q == pk.dirBase || pk.names[q] {
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
	if predicateRE.MatchString(sym) {
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
	if fileShapedSymbol(sym) || c.stdlibHas(pkg, rest) {
		return 0, nil, nil
	}
	for _, dir := range dirs {
		pk, err := c.loadPkg(dir)
		if err != nil {
			return 0, nil, err
		}
		if pk.has(sym) || pk.has(rest) {
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
	n := len(splitLines(string(b)))
	c.lines[rel] = n
	return n, nil
}

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

var (
	pathRE       = regexp.MustCompile(`(?:internal|cmd)/[A-Za-z0-9_./-]+\.(?:go|mg)`)
	tickRE       = regexp.MustCompile("`([^`\n]+)`")
	tickSpan     = regexp.MustCompile("`[^`]*`")
	symbolRE     = regexp.MustCompile(`^(\(\*?[A-Za-z_][A-Za-z0-9_]*\)\.)?[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*(?:/[0-9]+)?$`)
	predicateRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*/[0-9]+$`)
	abbrevFileRE = regexp.MustCompile(`(?:^|[^A-Za-z0-9_/])[A-Za-z0-9_.-]+\.(?:go|mg)\b`)
	gapWordRE    = regexp.MustCompile(`[A-Za-z]+`)
)

// connectorWords are the words that may sit between a path and the symbol it
// cites (`defines`, `at`, `see`, ...) without the backtick becoming a
// sentence. A word that is not in this set — `documented`, `vision` as prose
// around a path — means the backtick is not a citation of that path.
var connectorWords = newConnectorWords()

func newConnectorWords() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields("defines defined define declares declared declare declaration at in of the a an and or via on called see from is by its this that to for with") {
		m[w] = true
	}
	return m
}

func findPaths(line string) []pathHit {
	var out []pathHit
	for _, ix := range pathRE.FindAllStringIndex(line, -1) {
		if !acceptsPath(line, ix[0]) || !validRel(line[ix[0]:ix[1]]) {
			continue
		}
		rel := line[ix[0]:ix[1]]
		lines, hash, n := parseSuffix(line[ix[1]:])
		end := ix[1] + n
		out = append(out, pathHit{
			start: ix[0], end: end, rel: rel, raw: line[ix[0]:end],
			lines: lines, hash: hash,
		})
	}
	return out
}

// validRel rejects `..` and empty segments. Joining `internal/../../outside.go`
// would read outside the repository; that string is not a repo-relative citation.
func validRel(rel string) bool {
	if strings.Contains(rel, "\\") || strings.Contains(rel, "//") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// acceptsPath reports whether a regex hit is a path rooted at internal/ or
// cmd/. `./internal/...` and a leading slash are rooted. `foo/internal/...`
// and `https://host/internal/...` are the tail of a longer path, which the
// python grader's `\b` would have counted and which does not exist at the
// repo root.
func acceptsPath(line string, start int) bool {
	if start == 0 {
		return true
	}
	prev := line[start-1]
	if prev != '/' {
		return !isIdentByte(prev)
	}
	if start == 1 {
		return true
	}
	if line[start-2] != '.' {
		return !isIdentByte(line[start-2]) && line[start-2] != '.' && line[start-2] != '/'
	}
	if start == 2 {
		return true
	}
	b := line[start-3]
	return !isIdentByte(b) && b != '.' && b != '/'
}

func findSyms(line string) []symHit {
	var out []symHit
	for _, m := range tickRE.FindAllStringSubmatchIndex(line, -1) {
		sym, ok := normalizeSymbol(line[m[2]:m[3]])
		if !ok {
			continue
		}
		out = append(out, symHit{start: m[0], end: m[1], sym: sym})
	}
	return out
}

func normalizeSymbol(inner string) (string, bool) {
	s := strings.TrimSpace(inner)
	if s == "" {
		return "", false
	}
	for _, p := range []string{"var ", "func ", "type ", "const "} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
			break
		}
	}
	if strings.HasSuffix(s, ")") {
		if i := strings.LastIndex(s, "("); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	if s == "" || !symbolRE.MatchString(s) || fileShapedSymbol(s) {
		return "", false
	}
	return s, true
}

// fileExt is the lowercase tail of a backticked filename. Measured against
// Docs/architecture: `nerd.md`, `feedback.go`, `usage.json`, `types.go`,
// `config.yaml`, `nerd.exe` were graded as pkg.Func because the symbol
// grammar allows a dot. An exported selector is not all-lowercase
// (`sync.RWMutex`, `pkg.Go`), so this set does not swallow declarations.
var fileExt = map[string]bool{
	"md": true, "json": true, "go": true, "mg": true, "txt": true,
	"yml": true, "yaml": true, "toml": true, "xml": true, "html": true,
	"css": true, "png": true, "jpg": true, "jpeg": true, "svg": true,
	"gif": true, "sql": true, "sh": true, "ps1": true, "mod": true,
	"sum": true, "exe": true, "lock": true, "csv": true, "tsv": true,
	"proto": true,
}

func fileShapedSymbol(sym string) bool {
	i := strings.LastIndex(sym, ".")
	if i <= 0 || i == len(sym)-1 {
		return false
	}
	ext := sym[i+1:]
	if ext != strings.ToLower(ext) {
		return false
	}
	return fileExt[ext]
}

func bindSyms(line string, paths []pathHit, syms []symHit) (map[int][]symHit, []symHit) {
	bound := map[int][]symHit{}
	var unbound []symHit
	for _, sym := range syms {
		best := -1
		bestDist := int(^uint(0) >> 1)
		bestAfter := false
		for i, p := range paths {
			var between string
			dist := 0
			after := false
			switch {
			case sym.end <= p.start:
				between = line[sym.end:p.start]
				dist = p.start - sym.end
				after = true
			case p.end <= sym.start:
				between = line[p.end:sym.start]
				dist = sym.start - p.end
			default:
				continue
			}
			if !gapBinds(between) {
				continue
			}
			if gapHasWord(between) && !codeShaped(sym.sym) {
				continue
			}
			if dist < bestDist || (dist == bestDist && after && !bestAfter) {
				best = i
				bestDist = dist
				bestAfter = after
			}
		}
		if best < 0 {
			unbound = append(unbound, sym)
			continue
		}
		bound[best] = append(bound[best], sym)
	}
	return bound, unbound
}

func gapBinds(between string) bool {
	if len(between) > adjacencyWindow {
		return false
	}
	if abbrevFileRE.MatchString(between) {
		return false
	}
	words := gapWords(between)
	if len(words) > 4 {
		return false
	}
	for _, w := range words {
		if !connectorWords[strings.ToLower(w)] {
			return false
		}
	}
	return true
}

func gapHasWord(between string) bool {
	return len(gapWords(between)) > 0
}

func gapWords(between string) []string {
	prose := tickSpan.ReplaceAllString(between, " ")
	return gapWordRE.FindAllString(prose, -1)
}

// codeShaped is the bar for a symbol separated from its path by words
// (`path defines Sym`). An all-lowercase backtick in that spot is prose
// (`the vision`). A parenthetical citation has no words between the ticks
// and the path, so a short name like `mu` still counts there.
func codeShaped(sym string) bool {
	if strings.ContainsAny(sym, "._/") || strings.Contains(sym, "(*") {
		return true
	}
	for i := 0; i < len(sym); i++ {
		if sym[i] >= 'A' && sym[i] <= 'Z' {
			return true
		}
	}
	return false
}

func parseSuffix(s string) (lines [][2]int, hash string, n int) {
	i := 0
	if strings.HasPrefix(s, "#") {
		if sel, k := readSelector(s[1:]); k > 0 {
			hash = sel
			i = 1 + k
		}
	}
	if i < len(s) && s[i] == ':' {
		if ls, k, ok := readLineList(s[i+1:]); ok {
			lines = ls
			i += 1 + k
		}
	}
	if hash == "" && i < len(s) && s[i] == '#' {
		if sel, k := readSelector(s[i+1:]); k > 0 {
			hash = sel
			i += 1 + k
		}
	}
	return lines, hash, i
}

func readSelector(s string) (string, int) {
	if s == "" || !isIdentStart(s[0]) {
		return "", 0
	}
	i := 1
	for i < len(s) && isIdentCont(s[i]) {
		i++
	}
	for i < len(s) && s[i] == '.' {
		j := i + 1
		if j >= len(s) || !isIdentStart(s[j]) {
			break
		}
		j++
		for j < len(s) && isIdentCont(s[j]) {
			j++
		}
		i = j
	}
	return s[:i], i
}

func readLineList(s string) ([][2]int, int, bool) {
	if s == "" || !isDigit(s[0]) {
		return nil, 0, false
	}
	var spans [][2]int
	i := 0
	for {
		start, k := readInt(s[i:])
		if k == 0 {
			break
		}
		i += k
		end := start
		if i < len(s) && s[i] == '-' && i+1 < len(s) && isDigit(s[i+1]) {
			n2, k2 := readInt(s[i+1:])
			i += 1 + k2
			end = n2
		}
		spans = append(spans, [2]int{start, end})
		j := i
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j >= len(s) || s[j] != ',' {
			break
		}
		j++
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j >= len(s) || !isDigit(s[j]) {
			break
		}
		i = j
	}
	if len(spans) == 0 {
		return nil, 0, false
	}
	return spans, i, true
}

func readInt(s string) (int, int) {
	if s == "" || !isDigit(s[0]) {
		return 0, 0
	}
	i := 0
	n := 0
	for i < len(s) && isDigit(s[i]) {
		d := int(s[i] - '0')
		if n > (math.MaxInt-d)/10 {
			for i < len(s) && isDigit(s[i]) {
				i++
			}
			return math.MaxInt, i
		}
		n = n*10 + d
		i++
	}
	return n, i
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

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
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

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentCont(b byte) bool { return isIdentStart(b) || isDigit(b) }

func isIdentByte(b byte) bool { return isIdentCont(b) }
