package codemodel

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// DeriveImports brings a Go file's imports in line with what an edit changed.
//
// Imports are derived, not edited: every replay commit of the R8 set edits an
// import block, and a model that must write import lines by hand spends a
// call per file on bookkeeping the parse already knows. The rule is narrow on
// purpose, so it never breaks a file that compiled:
//
//   - a qualifier the edit newly uses (absent from old, present in nf, not
//     imported, not a package-level name of the file's own package) is
//     imported when res resolves it to one path;
//   - an import whose name the old file used and nf no longer uses is dropped.
//
// An import whose package name is only guessed from its path can therefore be
// dropped only if the guess matched real uses before the edit, and a
// qualifier nobody can resolve is reported rather than imported at random.
func DeriveImports(old, nf *File, res Resolver) (string, ImportReport, error) {
	var rep ImportReport
	before, _ := qualifierUses(old.Source)
	after, err := qualifierUses(nf.Source)
	if err != nil {
		return nf.Source, rep, nil
	}

	imported := make(map[string]Import, len(nf.Imports))
	for _, imp := range nf.Imports {
		imported[imp.LocalName()] = imp
	}

	var remove []Import
	for _, imp := range nf.Imports {
		name := imp.LocalName()
		if name == "_" || name == "." || imp.Path == "C" {
			continue
		}
		if before[name] > 0 && after[name] == 0 {
			remove = append(remove, imp)
		}
	}

	var addNames []string
	for q := range after {
		if before[q] > 0 {
			continue
		}
		if _, ok := imported[q]; ok {
			continue
		}
		if q == "" || q[0] < 'a' || q[0] > 'z' || q == nf.Package || res.DeclaredInPackage(q) {
			continue
		}
		addNames = append(addNames, q)
	}
	sort.Strings(addNames)
	var add []string
	for _, q := range addNames {
		path, candidates := res.ResolveQualifier(q)
		switch {
		case path == "" && len(candidates) == 0:
			rep.Notes = append(rep.Notes, fmt.Sprintf("%s: no package by that name is imported or declared anywhere in the workspace, so no import was added", q))
		case path == "":
			rep.Notes = append(rep.Notes, fmt.Sprintf("%s: ambiguous between %s, so no import was added; name the package in the header", q, strings.Join(candidates, ", ")))
		default:
			add = append(add, path)
			if len(candidates) > 1 {
				var others []string
				for _, c := range candidates {
					if c != path {
						others = append(others, c)
					}
				}
				rep.Notes = append(rep.Notes, fmt.Sprintf("%s: imported %s (also possible: %s)", q, path, strings.Join(others, ", ")))
			}
		}
	}
	if len(remove) == 0 && len(add) == 0 {
		return nf.Source, rep, nil
	}

	src, err := rewriteImports(nf.Source, remove, add, res.ModulePath())
	if err != nil {
		return nf.Source, ImportReport{Notes: append(rep.Notes, "imports were left as they were: "+err.Error())}, nil
	}
	for _, imp := range remove {
		rep.Removed = append(rep.Removed, imp.Path)
	}
	rep.Added = add
	return src, rep, nil
}

// qualifierUses counts, per identifier, the X.Sel selectors whose X the file
// does not declare. Those are the file's package qualifiers, plus names
// declared in the package's other files, which the caller filters out.
func qualifierUses(src string) (map[string]int, error) {
	node, err := parser.ParseFile(token.NewFileSet(), "uses.go", src, 0)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int)
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
			out[id.Name]++
		}
		return true
	})
	return out, nil
}

// rewriteImports removes and adds import specs by line, then gofmts the
// header so the added specs land sorted within their group.
func rewriteImports(src string, remove []Import, add []string, module string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "imports.go", src, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return "", err
	}
	tf := fset.File(node.Pos())
	f := &File{Source: src}
	off := tf.Offset

	type span struct{ start, end int }
	var cuts []span
	removing := make(map[string]bool, len(remove))
	for _, imp := range remove {
		removing[imp.Path+"\x00"+imp.Name] = true
	}
	var target *ast.GenDecl
	var single *ast.GenDecl
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		isC := len(gd.Specs) == 1 && gd.Specs[0].(*ast.ImportSpec).Path.Value == `"C"`
		kept := 0
		for _, spec := range gd.Specs {
			is := spec.(*ast.ImportSpec)
			p, _ := strconv.Unquote(is.Path.Value)
			name := ""
			if is.Name != nil {
				name = is.Name.Name
			}
			if !removing[p+"\x00"+name] {
				kept++
				continue
			}
			start, end := off(is.Pos()), off(is.End())
			if is.Doc != nil {
				start = off(is.Doc.Pos())
			}
			if is.Comment != nil {
				end = off(is.Comment.End())
			}
			cuts = append(cuts, span{f.LineStart(f.LineOf(start)), lineEndIncl(src, end)})
		}
		if kept == 0 && len(gd.Specs) > 0 {
			// Every spec goes: the declaration goes with them.
			start := off(gd.Pos())
			if gd.Doc != nil {
				start = off(gd.Doc.Pos())
			}
			cuts = cuts[:len(cuts)-len(gd.Specs)]
			cuts = append(cuts, span{f.LineStart(f.LineOf(start)), lineEndIncl(src, off(gd.End()))})
			continue
		}
		if isC {
			continue
		}
		if gd.Lparen.IsValid() {
			target = gd
		} else if single == nil {
			single = gd
		}
	}

	type insertion struct {
		at   int
		text string
	}
	var ins []insertion
	if len(add) > 0 {
		var std, other []string
		for _, p := range add {
			if isStdlib(p, module) {
				std = append(std, p)
			} else {
				other = append(other, p)
			}
		}
		specLines := func(paths []string) string {
			var sb strings.Builder
			for _, p := range paths {
				sb.WriteString("\t" + strconv.Quote(p) + "\n")
			}
			return sb.String()
		}
		switch {
		case target != nil:
			// The first blank-line-separated group takes the standard
			// library; everything else joins the last group.
			rparenLine := f.LineStart(tf.Line(target.Rparen))
			firstGroupEnd := rparenLine
			prevLine := 0
			for i, spec := range target.Specs {
				is := spec.(*ast.ImportSpec)
				line := tf.Line(is.Pos())
				if is.Doc != nil {
					line = tf.Line(is.Doc.Pos())
				}
				if i > 0 && line > prevLine+1 {
					firstGroupEnd = f.LineStart(prevLine + 1)
					break
				}
				prevLine = tf.Line(is.End())
			}
			if len(std) > 0 {
				ins = append(ins, insertion{firstGroupEnd, specLines(std)})
			}
			if len(other) > 0 {
				ins = append(ins, insertion{rparenLine, specLines(other)})
			}
		case single != nil:
			spec := single.Specs[0].(*ast.ImportSpec)
			start, end := off(single.Pos()), off(single.End())
			if spec.Comment != nil {
				end = off(spec.Comment.End())
			}
			existing := "\t" + src[off(spec.Pos()):end] + "\n"
			block := "import (\n" + existing + specLines(add) + ")"
			cuts = append(cuts, span{start, end})
			ins = append(ins, insertion{start, block})
		default:
			at := lineEndIncl(src, off(node.Name.End()))
			block := "\nimport (\n" + specLines(add) + ")\n"
			ins = append(ins, insertion{at, block})
		}
	}

	type op struct {
		start, end int
		text       string
	}
	var ops []op
	for _, c := range cuts {
		ops = append(ops, op{c.start, c.end, ""})
	}
	for _, in := range ins {
		ops = append(ops, op{in.at, in.at, in.text})
	}
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].start != ops[j].start {
			return ops[i].start > ops[j].start
		}
		return ops[i].end > ops[j].end
	})
	out := src
	for _, o := range ops {
		out = out[:o.start] + o.text + out[o.end:]
	}

	nf := ParseGo("imports.go", out)
	if !nf.Parsed {
		return "", fmt.Errorf("rewriting imports produced unparseable source: %s", describeErrors(nf))
	}
	hdr := nf.Header()
	formatted, err := FormatUnit(out[hdr.Start:hdr.End], true)
	if err != nil {
		return "", err
	}
	return formatted + out[hdr.End:], nil
}

// lineEndIncl returns the offset just past the newline ending the line that
// holds off (or len(src) on the last line).
func lineEndIncl(src string, off int) int {
	if off > 0 && off <= len(src) && src[off-1] == '\n' {
		return off
	}
	if i := strings.IndexByte(src[off:], '\n'); i >= 0 {
		return off + i + 1
	}
	return len(src)
}

// isStdlib reports an import path of the standard library: no dot in its
// first element, and not the module's own.
func isStdlib(path, module string) bool {
	if module != "" && (path == module || strings.HasPrefix(path, module+"/")) {
		return false
	}
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
