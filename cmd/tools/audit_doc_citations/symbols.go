package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// pkgDecls is the set of names go/parser can see in one directory. Promotion
// through embedded fields is not applied: that needs a type checker, and a
// citation of an embedded method still resolves when the method's own
// declaration is in the same package.
type pkgDecls struct {
	dirBase  string
	names    map[string]bool
	pkgNames map[string]bool
	parseErr map[string]string // slash-relative file -> parser error
}

// has reports whether cited is declared in this package.
//
// `(*Other).Clear` does not match a Clear method on Engine. The receiver is
// part of the citation. `pkg.Good` matches Good when pkg is the package name
// or the directory name. `Engine.Cache` matches the field key stored with
// the type.
func (p *pkgDecls) has(cited string) bool {
	if cited == "" || p == nil {
		return false
	}
	if p.names[cited] {
		return true
	}
	if strings.HasPrefix(cited, "(") {
		return false
	}
	parts := strings.Split(cited, ".")
	if len(parts) < 2 {
		return false
	}
	q := parts[0]
	sel := strings.Join(parts[1:], ".")
	if p.names[q+"."+sel] || p.names["(*"+q+")."+sel] || p.names["("+q+")."+sel] {
		return true
	}
	if (p.pkgNames[q] || q == p.dirBase) && p.names[sel] {
		return true
	}
	return false
}

func (c *checker) loadPkg(dir string) (*pkgDecls, error) {
	if p, ok := c.pkgs[dir]; ok {
		return p, nil
	}
	abs := absJoin(c.root, dir)
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read package %s: %w", dir, err)
	}
	p := &pkgDecls{
		dirBase:  slashBase(dir),
		names:    map[string]bool{},
		pkgNames: map[string]bool{},
		parseErr: map[string]string{},
	}
	fset := token.NewFileSet()
	for _, ent := range ents {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".go") {
			continue
		}
		rel := dir + "/" + ent.Name()
		f, perr := parser.ParseFile(fset, filepath.Join(abs, ent.Name()), nil, parser.SkipObjectResolution)
		if perr != nil {
			p.parseErr[rel] = perr.Error()
		}
		if f == nil {
			continue
		}
		if f.Name != nil {
			p.pkgNames[f.Name.Name] = true
		}
		addFileDecls(p.names, f)
	}
	c.pkgs[dir] = p
	return p, nil
}

func addFileDecls(names map[string]bool, f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			addFunc(names, d)
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					if s.Name != nil && s.Name.Name != "_" {
						names[s.Name.Name] = true
						addFields(names, s.Name.Name, s.Type)
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n != nil && n.Name != "_" {
							names[n.Name] = true
						}
					}
				}
			}
		}
	}
}

func addFunc(names map[string]bool, fn *ast.FuncDecl) {
	if fn == nil || fn.Name == nil || fn.Name.Name == "_" {
		return
	}
	names[fn.Name.Name] = true
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return
	}
	recv := typeIdent(fn.Recv.List[0].Type)
	if recv == "" {
		return
	}
	names[recv+"."+fn.Name.Name] = true
	names["(*"+recv+")."+fn.Name.Name] = true
	names["("+recv+")."+fn.Name.Name] = true
}

func addFields(names map[string]bool, owner string, typ ast.Expr) {
	switch t := typ.(type) {
	case *ast.StructType:
		if t.Fields == nil {
			return
		}
		for _, f := range t.Fields.List {
			addFieldNames(names, owner, f)
		}
	case *ast.InterfaceType:
		if t.Methods == nil {
			return
		}
		for _, f := range t.Methods.List {
			addFieldNames(names, owner, f)
		}
	}
}

func addFieldNames(names map[string]bool, owner string, f *ast.Field) {
	if len(f.Names) == 0 {
		if n := typeIdent(f.Type); n != "" && n != "_" {
			names[n] = true
			if owner != "" {
				names[owner+"."+n] = true
			}
		}
		return
	}
	for _, n := range f.Names {
		if n == nil || n.Name == "_" {
			continue
		}
		names[n.Name] = true
		if owner != "" {
			names[owner+"."+n.Name] = true
		}
	}
}

func typeIdent(e ast.Expr) string {
	for e != nil {
		switch t := e.(type) {
		case *ast.Ident:
			return t.Name
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.SelectorExpr:
			return t.Sel.Name
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		default:
			return ""
		}
	}
	return ""
}

// loadPredicates collects Decl names from the .mg corpus. Crash dumps under
// dot-directories (cmd/nerd/chat/.nerd/debug/debug_program_ERROR.mg re-declares
// the kernel) and testdata fixtures are not the corpus: counting them would
// make a predicate that has no real Decl look declared.
func loadPredicates(root string) (map[string]bool, error) {
	names := map[string]bool{}
	root = filepath.Clean(root)
	first := true
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !first && skipCorpusDir(d.Name()) {
				return filepath.SkipDir
			}
			first = false
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mg") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for n := range declNames(string(b)) {
			names[n] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// declNames returns predicate names introduced by a Decl line. A Decl inside
// a `#` comment, a `//` comment, or a block comment is not a declaration —
// the corpus is full of notes that mention Decl without making one.
func declNames(src string) map[string]bool {
	names := map[string]bool{}
	inBlock := false
	for _, line := range splitLines(src) {
		t := line
		if inBlock {
			if i := strings.Index(t, "*/"); i >= 0 {
				inBlock = false
				t = t[i+2:]
			} else {
				continue
			}
		}
		for {
			i := strings.Index(t, "/*")
			if i < 0 {
				break
			}
			j := strings.Index(t[i+2:], "*/")
			if j < 0 {
				inBlock = true
				t = t[:i]
				break
			}
			t = t[:i] + " " + t[i+2+j+2:]
		}
		if i := strings.Index(t, "#"); i >= 0 {
			t = t[:i]
		}
		if i := strings.Index(t, "//"); i >= 0 {
			t = t[:i]
		}
		t = strings.TrimSpace(t)
		if !strings.HasPrefix(t, "Decl") {
			continue
		}
		rest := t[len("Decl"):]
		if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		if name := predIdent(strings.TrimSpace(rest)); name != "" {
			names[name] = true
		}
	}
	return names
}

func predIdent(s string) string {
	if s == "" || !isIdentStart(s[0]) {
		return ""
	}
	i := 1
	for i < len(s) && isIdentCont(s[i]) {
		i++
	}
	return s[:i]
}

func predicateName(sym string) string {
	if i := strings.IndexByte(sym, '/'); i > 0 {
		return sym[:i]
	}
	if i := strings.LastIndex(sym, "."); i >= 0 && i < len(sym)-1 {
		return sym[i+1:]
	}
	return sym
}

// indexPackageDirs maps a directory base name (the `pkg` in `pkg.Func`) to
// the slash-relative directories under internal/ and cmd/ that contain Go
// files. Package clause `main` is recorded later, when the directory is
// parsed; the index itself is the directory name docs use.
func indexPackageDirs(root string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, top := range []string{"internal", "cmd"} {
		base := filepath.Clean(filepath.Join(root, top))
		st, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !st.IsDir() {
			continue
		}
		err = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if filepath.Clean(path) != base && skipCorpusDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			out[slashBase(rel)] = appendUnique(out[slashBase(rel)], rel)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, dirs := range out {
		sort.Strings(dirs)
	}
	return out, nil
}

func skipCorpusDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" || name == "node_modules"
}

func appendUnique(ss []string, s string) []string {
	for _, e := range ss {
		if e == s {
			return ss
		}
	}
	return append(ss, s)
}

// stdlibHas reports whether pkg.rest is declared in the standard library.
// rest may be a selector chain (`TokenCounter.charsPerToken`); only an exact
// declaration matches, so a repo type that the library does not have is still
// graded against the repo package.
//
// The lookup is the directory base name under GOROOT/src, which is the
// qualifier docs write: `filepath.Join` lives in path/filepath, `json.Marshal`
// in encoding/json. A missing GOROOT leaves the name as a repo citation
// rather than failing the run.
func (c *checker) stdlibHas(pkg, rest string) bool {
	if pkg == "" || rest == "" {
		return false
	}
	p, ok := c.stdCache[pkg]
	if !ok {
		p = loadStdlib(pkg)
		if c.stdCache == nil {
			c.stdCache = map[string]*pkgDecls{}
		}
		c.stdCache[pkg] = p
	}
	if p == nil {
		return false
	}
	return p.has(rest) || p.has(pkg+"."+rest)
}

func loadStdlib(pkg string) *pkgDecls {
	dir, ok := stdlibDirs()[pkg]
	if !ok {
		return nil
	}
	p, err := parsePackageDir(dir)
	if err != nil || p == nil || !p.pkgNames[pkg] {
		return nil
	}
	return p
}

var (
	stdlibIndexOnce sync.Once
	stdlibIndex     map[string]string
)

func stdlibDirs() map[string]string {
	stdlibIndexOnce.Do(func() {
		stdlibIndex = indexStdlib()
	})
	return stdlibIndex
}

// indexStdlib maps a package qualifier to the GOROOT directory that declares
// it. internal, vendor, testdata, and src/cmd are skipped: src/cmd is the Go
// tool (package main), and a nested internal copy of a name must not beat
// src/sync or src/go/types. When two directories share a base name, the
// shorter path wins, so path/filepath is `filepath` and go/types is `types`.
func indexStdlib() map[string]string {
	out := map[string]string{}
	score := map[string]int{}
	root := runtime.GOROOT()
	if root == "" {
		return out
	}
	src := filepath.Join(root, "src")
	_ = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if path != src && skipStdlibDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.EqualFold(filepath.Ext(name), ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		base := filepath.Base(dir)
		rel, err := filepath.Rel(src, dir)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		sc := strings.Count(rel, "/")
		prev, seen := score[base]
		if seen && (sc > prev || (sc == prev && dir >= out[base])) {
			return nil
		}
		score[base] = sc
		out[base] = dir
		return nil
	})
	return out
}

func skipStdlibDir(name string) bool {
	return name == "internal" || name == "vendor" || name == "testdata" || name == "cmd" || strings.HasPrefix(name, ".")
}

// parsePackageDir collects declarations in one directory. _test.go is left
// out so a test-only name is not a standard-library declaration. A file that
// does not parse contributes nothing; the files that do still count.
func parsePackageDir(abs string) (*pkgDecls, error) {
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	p := &pkgDecls{
		dirBase:  filepath.Base(abs),
		names:    map[string]bool{},
		pkgNames: map[string]bool{},
		parseErr: map[string]string{},
	}
	fset := token.NewFileSet()
	any := false
	for _, ent := range ents {
		name := ent.Name()
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(name), ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		any = true
		f, perr := parser.ParseFile(fset, filepath.Join(abs, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			p.parseErr[name] = perr.Error()
		}
		if f == nil {
			continue
		}
		if f.Name != nil {
			p.pkgNames[f.Name.Name] = true
		}
		addFileDecls(p.names, f)
	}
	if !any {
		return nil, nil
	}
	return p, nil
}
