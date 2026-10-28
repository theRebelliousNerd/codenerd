// Package docdecl reads declarations out of source the way a documentation
// grader must: from the parse tree, never from a text match. A name that
// appears only in a comment, a string literal or a use site is not a
// declaration, and this package does not report it as one.
//
// It serves two graders that must agree on what "declared" means: the
// citation auditor (cmd/tools/audit_doc_citations) and the documentation
// evidence view (internal/docscheck). Promotion through embedded fields is
// not applied: that needs a type checker, and a citation of an embedded
// method still resolves when the method's own declaration is in the same
// package.
package docdecl

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// Kind classifies one declaration.
type Kind string

const (
	KindFunc   Kind = "func"
	KindMethod Kind = "method"
	KindType   Kind = "type"
	KindValue  Kind = "value" // var or const
	KindField  Kind = "field" // struct field or interface method
	// KindTest is a function in a _test.go file whose name and signature
	// are the ones `go test` discovers: TestXxx(*testing.T),
	// BenchmarkXxx(*testing.B), FuzzXxx(*testing.F), or ExampleXxx().
	// It is a statically visible test identity, not evidence the test ran.
	KindTest Kind = "test"
)

// Decl is one declaration site.
type Decl struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	// File is the slash-relative path the declaration was parsed from.
	File string `json:"file"`
	Line int    `json:"line"`
	// Constrained is true when the file opens with a //go:build or
	// // +build constraint, so the declaration exists only under some
	// build configurations.
	Constrained bool `json:"constrained,omitempty"`
}

// Package is the set of names go/parser can see in one directory.
type Package struct {
	// Dir is the slash-relative directory the files were parsed from.
	Dir     string
	DirBase string
	// Names holds every citeable spelling: Name, Recv.Name, (*Recv).Name,
	// (Recv).Name and Owner.Field.
	Names    map[string]bool
	PkgNames map[string]bool
	// ParseErr maps a slash-relative file to its parser error. A file that
	// failed to parse may still contribute the declarations the parser
	// recovered.
	ParseErr map[string]string
	sites    map[string][]Decl
}

// NewPackage returns an empty package for the slash-relative directory dir.
func NewPackage(dir string) *Package {
	base := dir
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		base = dir[i+1:]
	}
	return &Package{
		Dir:      dir,
		DirBase:  base,
		Names:    map[string]bool{},
		PkgNames: map[string]bool{},
		ParseErr: map[string]string{},
		sites:    map[string][]Decl{},
	}
}

// AddFile parses one Go file and records its declarations. filename is what
// the parser reports positions against and reads when src is nil; rel is the
// slash-relative key used in ParseErr and Decl.File.
func (p *Package) AddFile(fset *token.FileSet, filename, rel string, src []byte) {
	var source any
	if src != nil {
		source = src
	}
	f, perr := parser.ParseFile(fset, filename, source, parser.SkipObjectResolution)
	if perr != nil {
		p.ParseErr[rel] = perr.Error()
	}
	if f == nil {
		return
	}
	if f.Name != nil {
		p.PkgNames[f.Name.Name] = true
	}
	w := fileWriter{p: p, fset: fset, rel: rel, test: strings.HasSuffix(rel, "_test.go"), constrained: hasBuildConstraint(src)}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			w.fn(d)
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					if s.Name != nil && s.Name.Name != "_" {
						w.add(s.Name.Name, KindType, s.Name.Pos())
						w.fields(s.Name.Name, s.Type)
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n != nil && n.Name != "_" {
							w.add(n.Name, KindValue, n.Pos())
						}
					}
				}
			}
		}
	}
}

// Has reports whether cited is declared in this package.
//
// `(*Other).Clear` does not match a Clear method on Engine. The receiver is
// part of the citation. `pkg.Good` matches Good when pkg is the package name
// or the directory name. `Engine.Cache` matches the field key stored with
// the type.
func (p *Package) Has(cited string) bool {
	_, ok := p.key(cited)
	return ok
}

// Lookup returns the declaration sites cited resolves to, or nil. Sites are
// in parse order. A citation that only matches an embedded field name or a
// spelling with no recorded site returns nil even though Has is true.
func (p *Package) Lookup(cited string) []Decl {
	k, ok := p.key(cited)
	if !ok {
		return nil
	}
	return p.sites[k]
}

// key maps a citation spelling to the Names key it matched.
func (p *Package) key(cited string) (string, bool) {
	if cited == "" || p == nil {
		return "", false
	}
	if p.Names[cited] {
		return cited, true
	}
	if strings.HasPrefix(cited, "(") {
		return "", false
	}
	parts := strings.Split(cited, ".")
	if len(parts) < 2 {
		return "", false
	}
	q := parts[0]
	sel := strings.Join(parts[1:], ".")
	for _, k := range []string{q + "." + sel, "(*" + q + ")." + sel, "(" + q + ")." + sel} {
		if p.Names[k] {
			return k, true
		}
	}
	if (p.PkgNames[q] || q == p.DirBase) && p.Names[sel] {
		return sel, true
	}
	return "", false
}

type fileWriter struct {
	p           *Package
	fset        *token.FileSet
	rel         string
	test        bool
	constrained bool
}

func (w fileWriter) add(name string, kind Kind, pos token.Pos) {
	w.p.Names[name] = true
	w.p.sites[name] = append(w.p.sites[name], Decl{
		Name: name, Kind: kind, File: w.rel,
		Line: w.fset.Position(pos).Line, Constrained: w.constrained,
	})
}

func (w fileWriter) fn(fn *ast.FuncDecl) {
	if fn == nil || fn.Name == nil || fn.Name.Name == "_" {
		return
	}
	kind := KindFunc
	if w.test && (fn.Recv == nil || len(fn.Recv.List) == 0) && isTestSignature(fn) {
		kind = KindTest
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		w.add(fn.Name.Name, kind, fn.Name.Pos())
		return
	}
	w.add(fn.Name.Name, KindMethod, fn.Name.Pos())
	recv := typeIdent(fn.Recv.List[0].Type)
	if recv == "" {
		return
	}
	w.add(recv+"."+fn.Name.Name, KindMethod, fn.Name.Pos())
	w.add("(*"+recv+")."+fn.Name.Name, KindMethod, fn.Name.Pos())
	w.add("("+recv+")."+fn.Name.Name, KindMethod, fn.Name.Pos())
}

func (w fileWriter) fields(owner string, typ ast.Expr) {
	var list []*ast.Field
	switch t := typ.(type) {
	case *ast.StructType:
		if t.Fields != nil {
			list = t.Fields.List
		}
	case *ast.InterfaceType:
		if t.Methods != nil {
			list = t.Methods.List
		}
	}
	for _, f := range list {
		w.field(owner, f)
	}
}

func (w fileWriter) field(owner string, f *ast.Field) {
	if len(f.Names) == 0 {
		if n := typeIdent(f.Type); n != "" && n != "_" {
			w.add(n, KindField, f.Pos())
			if owner != "" {
				w.add(owner+"."+n, KindField, f.Pos())
			}
		}
		return
	}
	for _, n := range f.Names {
		if n == nil || n.Name == "_" {
			continue
		}
		w.add(n.Name, KindField, n.Pos())
		if owner != "" {
			w.add(owner+"."+n.Name, KindField, n.Pos())
		}
	}
}

// isTestSignature reports whether fn is shaped like a function `go test`
// runs: its prefix, the testing type of its only parameter, and no results.
func isTestSignature(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if fn.Type == nil || (fn.Type.Results != nil && len(fn.Type.Results.List) > 0) {
		return false
	}
	var params []*ast.Field
	if fn.Type.Params != nil {
		params = fn.Type.Params.List
	}
	prefix := func(p string) bool {
		if !strings.HasPrefix(name, p) {
			return false
		}
		rest := name[len(p):]
		// `go test` requires the next rune not to be lower case.
		return rest == "" || !(rest[0] >= 'a' && rest[0] <= 'z')
	}
	switch {
	case prefix("Test"):
		return singleTestingParam(params, "T")
	case prefix("Benchmark"):
		return singleTestingParam(params, "B")
	case prefix("Fuzz"):
		return singleTestingParam(params, "F")
	case prefix("Example"):
		return len(params) == 0
	}
	return false
}

func singleTestingParam(params []*ast.Field, typ string) bool {
	if len(params) != 1 || len(params[0].Names) > 1 {
		return false
	}
	star, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != typ {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "testing"
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

// hasBuildConstraint reports whether src opens with a build constraint
// comment, which must precede the package clause.
func hasBuildConstraint(src []byte) bool {
	for _, line := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			continue
		case strings.HasPrefix(t, "//go:build"), strings.HasPrefix(t, "// +build"):
			return true
		case strings.HasPrefix(t, "//"):
			continue
		default:
			return false
		}
	}
	return false
}
