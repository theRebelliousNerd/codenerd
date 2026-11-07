package session

import (
	"bytes"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Static regression test selection, part 1: which functions did the turn
// change, and is that answer one a call graph can be trusted with.
//
// The importer gate (importer_packages.go) used to run every test of every
// package that imports what the turn wrote. A selection is only as sound as
// its idea of "changed", so this file answers narrowly: a change is a body
// change when the file's declarations, imports, directives and every function
// signature are untouched and only the statements inside some function bodies
// differ. Anything else -- a package-level var, const or type, an import, an
// init function, a signature, a method set, a build constraint or directive,
// cgo, an embedded or generated neighbour -- can change behaviour in a place
// no call edge leads to, and the answer is "unknown": the caller runs every
// importer test, as before, and records why.
//
// Formatting and comments are not changes. Both files are parsed twice: once
// with comments, to compare compiler directives and cgo preambles; once
// without, so the printed form of every declaration is free of Doc fields and
// two spellings of the same code compare equal.

// declKey names a function declaration inside one file: the receiver's type
// name without pointer or type parameters ("" for a plain function) and the
// function's name.
type declKey struct {
	Recv string
	Name string
}

func (k declKey) String() string {
	if k.Recv == "" {
		return k.Name
	}
	return k.Recv + "." + k.Name
}

// bodyChanges is what the turn changed, file by file, when every change is a
// function body; otherwise why the selection must not be trusted.
type bodyChanges struct {
	// Funcs maps an absolute, symlink-resolved file path to the functions
	// whose bodies differ from the pre-turn file. A written file with no
	// body change has no entry.
	Funcs map[string][]declKey
	// Fallback is the reason a selection would be unsound; empty when Funcs
	// is the whole truth.
	Fallback string
}

// changedBodies compares each written non-test Go file with its pre-turn
// contents. ownDirs are the workspace-relative directories of the packages
// the turn wrote, used to notice a written non-Go file (embedded data,
// assembly, cgo headers) beside them.
func changedBodies(workspace string, written []string, pre map[string]PreImage) bodyChanges {
	out := bodyChanges{Funcs: map[string][]declKey{}}
	fail := func(why string) bodyChanges { return bodyChanges{Fallback: why} }

	ownDirs := map[string]bool{}
	for _, p := range written {
		if isGoTestSource(p) || !strings.HasSuffix(strings.ToLower(strings.TrimSpace(p)), ".go") {
			continue
		}
		ownDirs[filepath.ToSlash(filepath.Dir(canonicalOrSelf(p, workspace)))] = true
	}
	var goFiles []string
	for _, p := range written {
		clean := canonicalOrSelf(p, workspace)
		base := strings.ToLower(filepath.Base(clean))
		switch {
		case base == "go.mod" || base == "go.sum" || base == "go.work" || base == "go.work.sum":
			if writeReverted(workspace, p, pre) {
				continue
			}
			return fail("the module definition changed (" + clean + ")")
		case strings.HasSuffix(base, "_test.go"):
			// A test file is never part of what an importer links.
		case strings.HasSuffix(base, ".go"):
			goFiles = append(goFiles, p)
		case ownDirs[filepath.ToSlash(filepath.Dir(clean))]:
			if writeReverted(workspace, p, pre) {
				continue
			}
			return fail("a non-Go file was written beside Go code (" + clean + "): it may be embedded, assembled or compiled in")
		}
	}

	for _, p := range goFiles {
		abs := absWritten(p, workspace)
		img, ok := rtsPreImage(p, workspace, pre)
		switch {
		case !ok:
			return fail("no pre-write snapshot of " + p)
		case img.Unknown != "":
			return fail("pre-write snapshot of " + p + " is unknown: " + img.Unknown)
		}
		after, err := os.ReadFile(abs)
		if err != nil {
			return fail(p + " cannot be read after the turn (" + err.Error() + ")")
		}
		var before []byte
		if img.Existed {
			before = []byte(img.Content)
		}
		funcs, why := diffGoFile(abs, before, img.Existed, after)
		if why != "" {
			return fail(p + ": " + why)
		}
		if len(funcs) > 0 {
			key := resolveSymlinks(abs)
			out.Funcs[key] = append(out.Funcs[key], funcs...)
		}
	}
	return out
}

func canonicalOrSelf(p, workspace string) string {
	if c := canonicalizeWrittenPath(p, workspace); c != "" {
		return c
	}
	return filepath.ToSlash(strings.TrimSpace(p))
}

func absWritten(p, workspace string) string {
	p = strings.TrimSpace(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(workspace, filepath.FromSlash(canonicalOrSelf(p, workspace)))
}

func rtsPreImage(p, workspace string, pre map[string]PreImage) (PreImage, bool) {
	if key := canonicalizeWrittenPath(p, workspace); key != "" {
		if img, ok := pre[key]; ok {
			return img, true
		}
	}
	img, ok := pre[p]
	return img, ok
}

// resolveSymlinks spells a path the way the Go loader reports it.
func resolveSymlinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(r)
	}
	return filepath.Clean(p)
}

// diffGoFile returns the functions whose bodies changed between before and
// after, or why the difference is not body-only. existed is false for a file
// the turn created: its functions are additions, which no existing code calls.
func diffGoFile(name string, before []byte, existed bool, after []byte) ([]declKey, string) {
	newDecls, why := inspectGoSource(name, after)
	if why != "" {
		return nil, "the new file " + why
	}
	var oldDecls goDecls
	if existed {
		if oldDecls, why = inspectGoSource(name, before); why != "" {
			return nil, "the pre-turn file " + why
		}
	}
	if newDecls.linkname || oldDecls.linkname {
		return nil, "go:linkname reaches around the call graph"
	}
	if newDecls.pkg != oldDecls.pkg && existed {
		return nil, "the package clause changed"
	}
	if !slices.Equal(newDecls.directives, oldDecls.directives) {
		return nil, "a compiler directive or build constraint changed"
	}
	if !slices.Equal(newDecls.imports, oldDecls.imports) {
		return nil, "the imports changed"
	}
	if !slices.Equal(newDecls.general, oldDecls.general) {
		return nil, "a package-level var, const or type declaration changed"
	}
	if !slices.Equal(newDecls.inits, oldDecls.inits) {
		return nil, "an init function changed"
	}
	var changed []declKey
	for key, nf := range newDecls.funcs {
		of, had := oldDecls.funcs[key]
		switch {
		case !had && key.Recv != "":
			return nil, "method " + key.String() + " was added: a method set is not a function body"
		case !had:
			// A new plain function: nothing that already exists calls it, and
			// the bodies that now do are among the changed ones.
		case of.sig != nf.sig:
			return nil, "the signature of " + key.String() + " changed"
		case of.body != nf.body:
			changed = append(changed, key)
		}
	}
	for key := range oldDecls.funcs {
		if _, kept := newDecls.funcs[key]; !kept {
			return nil, "function " + key.String() + " was removed"
		}
	}
	slices.SortFunc(changed, func(a, b declKey) int { return strings.Compare(a.String(), b.String()) })
	return changed, ""
}

type funcForm struct {
	sig  string
	body string
}

// goDecls is the comparable shape of a Go source file.
type goDecls struct {
	pkg        string
	directives []string
	imports    []string
	general    []string
	inits      []string
	funcs      map[declKey]funcForm
	linkname   bool
}

// inspectGoSource reduces a source file to what diffGoFile compares. why is
// non-empty when the file cannot be reduced soundly (it does not parse, it
// imports "C", it declares a function twice).
func inspectGoSource(name string, src []byte) (goDecls, string) {
	var d goDecls
	if len(bytes.TrimSpace(src)) == 0 {
		d.funcs = map[declKey]funcForm{}
		return d, ""
	}
	fset := token.NewFileSet()
	withComments, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return d, "does not parse (" + err.Error() + ")"
	}
	// Directives are read with the toolchain's own parsers, not by prefix:
	// ast.ParseDirective for //go: directives, go/build/constraint for
	// // +build lines. A //line directive remaps positions, so it shows as a
	// difference between the file end's adjusted and raw positions. //export
	// matters only beside import "C", which is refused as unsound already.
	for _, cg := range withComments.Comments {
		for _, c := range cg.List {
			if dir, ok := ast.ParseDirective(c.Pos(), c.Text); ok && dir.Tool == "go" {
				d.directives = append(d.directives, c.Text)
				if dir.Name == "linkname" {
					d.linkname = true
				}
				continue
			}
			if constraint.IsPlusBuild(c.Text) {
				d.directives = append(d.directives, c.Text)
			}
		}
	}
	if end := withComments.FileEnd; fset.PositionFor(end, true) != fset.PositionFor(end, false) {
		d.directives = append(d.directives, "//line")
	}

	bare, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		return d, "does not parse (" + err.Error() + ")"
	}
	d.pkg = bare.Name.Name
	d.funcs = map[declKey]funcForm{}
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	render := func(node any) string {
		var buf bytes.Buffer
		if err := cfg.Fprint(&buf, token.NewFileSet(), node); err != nil {
			return "\x00unprintable:" + err.Error()
		}
		return buf.String()
	}
	for _, decl := range bare.Decls {
		switch dd := decl.(type) {
		case *ast.GenDecl:
			if dd.Tok == token.IMPORT {
				for _, spec := range dd.Specs {
					is := spec.(*ast.ImportSpec)
					path, _ := strconv.Unquote(is.Path.Value)
					if path == "C" {
						return d, "uses cgo"
					}
					alias := ""
					if is.Name != nil {
						alias = is.Name.Name + " "
					}
					d.imports = append(d.imports, alias+path)
				}
				continue
			}
			d.general = append(d.general, render(dd))
		case *ast.FuncDecl:
			sigDecl := *dd
			sigDecl.Body = nil
			form := funcForm{sig: render(&sigDecl)}
			if dd.Body != nil {
				form.body = render(dd.Body)
			}
			key := declKey{Name: dd.Name.Name, Recv: receiverTypeName(dd)}
			if key.Name == "_" || (key.Recv == "" && key.Name == "init") {
				d.inits = append(d.inits, form.sig+form.body)
				continue
			}
			if _, dup := d.funcs[key]; dup {
				return d, "declares " + key.String() + " twice"
			}
			d.funcs[key] = form
		}
	}
	slices.Sort(d.imports)
	return d, ""
}

// receiverTypeName is the receiver's base type name: "T" for (t T), (t *T)
// and (t *T[K, V]); "" for a plain function.
func receiverTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	expr := fd.Recv.List[0].Type
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return e.Name
		default:
			return "?"
		}
	}
}
