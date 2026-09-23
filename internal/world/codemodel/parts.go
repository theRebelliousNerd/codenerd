package codemodel

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// Part is a nested block of an element, addressed by the AST rather than by
// line: "3" is the element's third top-level statement, "3.2" the second
// child of that statement (the second case clause of a switch, the else
// branch of an if, the second statement of a loop body).
//
// It exists for the giant functions. 26 functions in this repository are over
// 300 lines and one is over 1,000; a view of the whole element costs the whole
// element, and an anchored edit inside it needs an anchor unique across all of
// it. A part narrows both without falling back to line numbers, which go
// stale on the first edit above them.
type Part struct {
	Path               string
	Label              string
	Start, End         int
	StartLine, EndLine int
	Children           []Part
}

// Parts returns the nested blocks of an element: the statements of a
// function's body, or of the function literals in a var initializer. Other
// elements have none.
func (f *File) Parts(e *Element) []Part {
	if f.Language != LangGo || e == nil {
		return nil
	}
	var prefix, suffix string
	switch {
	case e.Kind == KindFunction || e.Kind == KindMethod:
		prefix = "package p\n\n"
	case (e.Kind == KindVar || e.Kind == KindConst) && e.Grouped:
		prefix, suffix = "package p\n\n"+string(e.Kind)+" (\n", "\n)\n"
	case e.Kind == KindVar || e.Kind == KindConst:
		prefix = "package p\n\n"
	default:
		return nil
	}
	text := f.Text(e)
	src := prefix + text + suffix
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "part.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil || len(node.Decls) == 0 {
		return nil
	}
	tf := fset.File(node.Pos())
	b := partBuilder{f: f, src: src, tf: tf, base: e.Start - len(prefix)}

	var top []partNode
	switch d := node.Decls[len(node.Decls)-1].(type) {
	case *ast.FuncDecl:
		if d.Body == nil {
			return nil
		}
		top = stmtNodes(d.Body.List)
	case *ast.GenDecl:
		var lits []ast.Node
		for _, spec := range d.Specs {
			if vs, ok := spec.(*ast.ValueSpec); ok {
				for _, v := range vs.Values {
					lits = append(lits, funcLitsIn(v)...)
				}
			}
		}
		if len(lits) == 1 {
			top = b.children(lits[0])
		} else {
			for _, l := range lits {
				top = append(top, partNode{node: l})
			}
		}
	}
	return b.build(top, "")
}

// FindPart resolves a part path ("3", "3.2") against an element's parts.
func (f *File) FindPart(e *Element, path string) (*Part, error) {
	parts := f.Parts(e)
	if len(parts) == 0 {
		return nil, fmt.Errorf("%s has no nested parts (only functions and closures in var initializers do)", e.Key)
	}
	var found *Part
	level := parts
	for i, seg := range strings.Split(strings.TrimSpace(path), ".") {
		n, err := strconv.Atoi(strings.TrimSpace(seg))
		if err != nil || n < 1 || n > len(level) {
			where := "the element"
			if found != nil {
				where = "part " + found.Path
			}
			return nil, fmt.Errorf("part %q: segment %d (%q) is not one of the %d children of %s", path, i+1, seg, len(level), where)
		}
		found = &level[n-1]
		level = found.Children
	}
	return found, nil
}

type partNode struct {
	node  ast.Node
	label string
}

type partBuilder struct {
	f    *File
	src  string
	tf   *token.File
	base int
}

func (b partBuilder) off(p token.Pos) int {
	return b.base + b.tf.Offset(p)
}

func (b partBuilder) build(nodes []partNode, parent string) []Part {
	out := make([]Part, 0, len(nodes))
	for i, pn := range nodes {
		path := strconv.Itoa(i + 1)
		if parent != "" {
			path = parent + "." + path
		}
		p := Part{Path: path, Start: b.off(pn.node.Pos()), End: b.off(pn.node.End())}
		p.StartLine = b.f.LineOf(p.Start)
		p.EndLine = b.f.LineOf(max(p.End-1, p.Start))
		p.Label = pn.label
		if p.Label == "" {
			line, _, _ := strings.Cut(b.f.Source[p.Start:p.End], "\n")
			p.Label = strings.TrimSpace(line)
		}
		p.Children = b.build(b.children(pn.node), path)
		out = append(out, p)
	}
	return out
}

// children lists what is nested directly inside a node. A statement holding
// exactly one block (a loop body, a lone closure) lists that block's
// statements; one holding several (if/else, the clauses of a switch, two
// closures) lists the blocks.
func (b partBuilder) children(n ast.Node) []partNode {
	switch s := n.(type) {
	case *ast.BlockStmt:
		return stmtNodes(s.List)
	case *ast.LabeledStmt:
		return b.children(s.Stmt)
	case *ast.IfStmt:
		if s.Else == nil {
			return stmtNodes(s.Body.List)
		}
		blocks := []partNode{{node: s.Body, label: "then"}}
		if elif, ok := s.Else.(*ast.IfStmt); ok {
			line, _, _ := strings.Cut(b.f.Source[b.off(elif.Pos()):b.off(elif.End())], "\n")
			blocks = append(blocks, partNode{node: elif, label: "else " + strings.TrimSpace(line)})
		} else {
			blocks = append(blocks, partNode{node: s.Else, label: "else"})
		}
		return blocks
	case *ast.ForStmt:
		return stmtNodes(s.Body.List)
	case *ast.RangeStmt:
		return stmtNodes(s.Body.List)
	case *ast.SwitchStmt:
		return stmtNodes(s.Body.List)
	case *ast.TypeSwitchStmt:
		return stmtNodes(s.Body.List)
	case *ast.SelectStmt:
		return stmtNodes(s.Body.List)
	case *ast.CaseClause:
		return stmtNodes(s.Body)
	case *ast.CommClause:
		return stmtNodes(s.Body)
	case *ast.FuncLit:
		return stmtNodes(s.Body.List)
	}
	lits := funcLitsIn(n)
	if len(lits) == 1 {
		return b.children(lits[0])
	}
	out := make([]partNode, 0, len(lits))
	for _, l := range lits {
		out = append(out, partNode{node: l})
	}
	return out
}

func stmtNodes(list []ast.Stmt) []partNode {
	out := make([]partNode, 0, len(list))
	for _, s := range list {
		out = append(out, partNode{node: s})
	}
	return out
}

// funcLitsIn returns the function literals under n that are not nested in
// another function literal under n.
func funcLitsIn(n ast.Node) []ast.Node {
	var out []ast.Node
	ast.Inspect(n, func(c ast.Node) bool {
		if lit, ok := c.(*ast.FuncLit); ok && c != n {
			out = append(out, lit)
			return false
		}
		return true
	})
	return out
}

// RenderParts lists parts as rows, expanding any part longer than expandOver
// lines into its children, so a giant element's outline names the blocks a
// caller can ask for without listing every statement of every block.
func RenderParts(parts []Part, expandOver int) []string {
	var rows []string
	var walk func([]Part, int)
	walk = func(level []Part, depth int) {
		for _, p := range level {
			label := p.Label
			rows = append(rows, fmt.Sprintf("%spart %s  lines %d-%d  %s", strings.Repeat("  ", depth), p.Path, p.StartLine, p.EndLine, label))
			if len(p.Children) > 0 && p.EndLine-p.StartLine+1 > expandOver {
				walk(p.Children, depth+1)
			}
		}
	}
	walk(parts, 0)
	return rows
}
