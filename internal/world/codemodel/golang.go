package codemodel

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	gotypes "go/types"
	"strconv"
	"strings"
)

// maxBrokenRegions bounds how many broken regions ParseGo isolates before it
// stops looking for more; each costs one re-parse of the file.
const maxBrokenRegions = 32

// ParseGo builds the element model of a Go file. src may use any line
// ending; the model holds it as LF text.
//
// A file that does not parse still gets a model: every declaration outside
// the broken regions, and one syntax_error element per region, so the file
// stays addressable and repairable by the same verbs.
//
// The parser's own error recovery is not enough for that. One missing ')'
// makes go/parser swallow the rest of the file into the broken function and
// report a dozen cascade errors against declarations that are fine. So the
// region around the first error is blanked (bytes to spaces, newlines kept,
// so every offset and line still holds) and the file parsed again, until it
// parses or no progress is made. Each region's error is the real one.
func ParseGo(path, src string) *File {
	f, _, _ := ParseGoAST(path, src)
	return f
}

// ParseGoAST is ParseGo that also returns the syntax tree the model was read
// from, for a caller that needs more from the same parse (the structure
// index collects call sites from it). In a broken file the tree is of the
// source with its broken regions blanked; its positions are the file's.
func ParseGoAST(path, src string) (*File, *token.FileSet, *ast.File) {
	src = Normalize(src)
	f := &File{Path: path, Language: LangGo, Source: src}

	masked := src
	var regions []Element
	var node *ast.File
	var fset *token.FileSet
	for {
		var err error
		fset = token.NewFileSet()
		node, err = parser.ParseFile(fset, path, masked, parser.ParseComments|parser.SkipObjectResolution|parser.AllErrors)
		if err == nil {
			break
		}
		first := firstSyntaxError(err)
		if f.Err == nil {
			f.Err = err
		}
		region, ok := brokenRegion(f, masked, first)
		if !ok || len(regions) >= maxBrokenRegions || overlapsAny(region, regions) {
			// No progress: keep what the last parse recovered cleanly.
			f.Errors = append(f.Errors, first)
			if !overlapsAny(region, regions) && ok {
				regions = append(regions, region)
			}
			break
		}
		f.Errors = append(f.Errors, first)
		regions = append(regions, region)
		masked = blank(masked, region.Start, region.End)
	}
	f.Parsed = len(regions) == 0 && f.Err == nil

	var elements []Element
	if node != nil && node.Name != nil {
		elements = goElements(f, fset, node)
	}
	if len(regions) > 0 {
		var kept []Element
		for _, e := range elements {
			if e.End <= e.Start || (e.Kind != KindHeader && overlapsAny(e, regions)) {
				continue
			}
			kept = append(kept, e)
		}
		elements = append(kept, regions...)
		sortElements(elements)
	}
	for i := range elements {
		e := &elements[i]
		if !e.Grouped {
			e.UnitStart, e.UnitEnd = e.Start, e.End
		}
		e.Exported = e.Kind != KindHeader && e.Kind != KindSyntaxError && ast.IsExported(e.Name)
		e.StartLine = f.LineOf(e.Start)
		e.EndLine = f.LineOf(max(e.End-1, e.Start))
		if e.DeclLine == 0 {
			e.DeclLine = e.StartLine
		}
		e.Revision = Revision(src[e.Start:e.End])
	}
	assignKeys(elements, goBaseKey)
	f.Elements = elements
	if node != nil && node.Name == nil {
		node = nil
	}
	return f, fset, node
}

// goElements reads the header and every declaration out of a parsed file.
func goElements(f *File, fset *token.FileSet, node *ast.File) []Element {
	src := f.Source
	f.Package = node.Name.Name
	f.BuildConstraint = buildConstraint(node)
	tf := fset.File(node.Pos())
	off := func(p token.Pos) int {
		if !p.IsValid() {
			return -1
		}
		return min(tf.Offset(p), len(src))
	}

	var elements []Element

	// The header runs from the top of the file through the last import
	// declaration: file doc, build constraints, package clause, imports.
	hdrEnd := off(node.Name.End())
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		hdrEnd = max(hdrEnd, off(gd.End()))
		for _, spec := range gd.Specs {
			is, ok := spec.(*ast.ImportSpec)
			if !ok || is.Path == nil {
				continue
			}
			imp := Import{Start: off(is.Pos()), End: off(is.End()), Line: tf.Line(is.Pos())}
			if is.Doc != nil {
				imp.Start = off(is.Doc.Pos())
			}
			if is.Comment != nil {
				imp.End = off(is.Comment.End())
				hdrEnd = max(hdrEnd, imp.End)
			}
			if is.Name != nil {
				imp.Name = is.Name.Name
			}
			p, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				continue
			}
			imp.Path = p
			f.Imports = append(f.Imports, imp)
		}
	}
	elements = append(elements, Element{
		Key: HeaderKey, Name: HeaderKey, Kind: KindHeader,
		Start: 0, End: hdrEnd, DeclLine: tf.Line(node.Package),
		Signature: "package " + node.Name.Name,
		Doc:       firstDocLine(node.Doc),
	})

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			e := Element{Name: d.Name.Name, Kind: KindFunction, Start: off(d.Pos()), End: off(d.End()),
				DeclLine: tf.Line(d.Pos()), Signature: funcSignature(d), Doc: firstDocLine(d.Doc)}
			if d.Doc != nil {
				e.Start = off(d.Doc.Pos())
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				e.Kind = KindMethod
				e.Receiver = ReceiverTypeName(d.Recv.List[0].Type)
			}
			e.Names = []string{e.Name}
			elements = append(elements, e)
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			elements = append(elements, genDeclElements(src, tf, off, d)...)
		}
	}
	var valid []Element
	for _, e := range elements {
		if e.Start >= 0 && e.End >= e.Start {
			valid = append(valid, e)
		}
	}
	return valid
}

func genDeclElements(src string, tf *token.File, off func(token.Pos) int, d *ast.GenDecl) []Element {
	grouped := d.Lparen.IsValid()
	var out []Element
	for _, spec := range d.Specs {
		e := Element{Grouped: grouped}
		var specDoc, specComment *ast.CommentGroup
		switch sp := spec.(type) {
		case *ast.TypeSpec:
			e.Name = sp.Name.Name
			e.Names = []string{e.Name}
			e.Kind = KindType
			switch sp.Type.(type) {
			case *ast.StructType:
				e.Kind = KindStruct
			case *ast.InterfaceType:
				e.Kind = KindInterface
			}
			specDoc, specComment = sp.Doc, sp.Comment
		case *ast.ValueSpec:
			e.Kind = KindVar
			if d.Tok == token.CONST {
				e.Kind = KindConst
			}
			e.Name = "_"
			for _, n := range sp.Names {
				e.Names = append(e.Names, n.Name)
				if e.Name == "_" && n.Name != "_" {
					e.Name = n.Name
				}
			}
			specDoc, specComment = sp.Doc, sp.Comment
		default:
			continue
		}
		e.DeclLine = tf.Line(spec.Pos())
		if grouped {
			e.Start, e.End = off(spec.Pos()), off(spec.End())
			if specDoc != nil {
				e.Start = off(specDoc.Pos())
			}
			e.Doc = firstDocLine(specDoc)
			if e.Doc == "" {
				e.Doc = firstDocLine(d.Doc)
			}
			e.UnitStart, e.UnitEnd = off(d.Pos()), off(d.End())
			if d.Doc != nil {
				e.UnitStart = off(d.Doc.Pos())
			}
		} else {
			e.Start, e.End = off(d.Pos()), off(d.End())
			e.DeclLine = tf.Line(d.Pos())
			if d.Doc != nil {
				e.Start = off(d.Doc.Pos())
			}
			e.Doc = firstDocLine(d.Doc)
		}
		if specComment != nil {
			e.End = max(e.End, off(specComment.End()))
			if !grouped {
				e.UnitEnd = e.End
			}
		}
		if s, t := off(spec.Pos()), off(spec.End()); s >= 0 && t >= s {
			e.Signature = specSignature(src[s:t], d.Tok)
		}
		out = append(out, e)
	}
	return out
}

func goBaseKey(e *Element) string {
	switch {
	case e.Kind == KindHeader:
		return HeaderKey
	case e.Kind == KindSyntaxError:
		return string(KindSyntaxError)
	case e.Receiver != "":
		return e.Receiver + "." + e.Name
	}
	return e.Name
}

func firstSyntaxError(err error) SyntaxError {
	var list scanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		return SyntaxError{Line: list[0].Pos.Line, Column: list[0].Pos.Column, Msg: list[0].Msg}
	}
	return SyntaxError{Line: 1, Column: 1, Msg: err.Error()}
}

// brokenRegion is the span around a syntax error: from the column-0
// declaration keyword at or above it (with the doc comment above that) to the
// line before the next one below it. The parser resyncs at the same keywords,
// so the region is the declaration it could not read.
func brokenRegion(f *File, src string, se SyntaxError) (Element, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 {
		return Element{}, false
	}
	isBoundary := func(i int) bool {
		l := lines[i]
		for _, kw := range []string{"func ", "func(", "type ", "var ", "const ", "import "} {
			if strings.HasPrefix(l, kw) {
				return true
			}
		}
		return false
	}
	li := min(max(se.Line-1, 0), len(lines)-1)
	lo := li
	for lo > 0 && !isBoundary(lo) {
		lo--
	}
	for lo > 0 && strings.HasPrefix(lines[lo-1], "//") {
		lo--
	}
	hi := li + 1
	for hi < len(lines) && !isBoundary(hi) {
		hi++
	}
	for hi-1 > li && strings.HasPrefix(lines[hi-1], "//") {
		hi--
	}
	for hi-1 > li && strings.TrimSpace(lines[hi-1]) == "" {
		hi--
	}
	start := f.LineStart(lo + 1)
	end := len(src)
	if hi < len(lines) {
		end = f.LineStart(hi+1) - 1
	}
	if end <= start {
		return Element{}, false
	}
	return Element{
		Name: string(KindSyntaxError), Kind: KindSyntaxError,
		Start: start, End: end, DeclLine: se.Line,
		Signature: strings.TrimSpace(lines[lo]),
		Err:       se.Msg,
	}, true
}

func overlapsAny(e Element, regions []Element) bool {
	for _, r := range regions {
		if e.Start < r.End && r.Start < e.End {
			return true
		}
	}
	return false
}

// blank replaces src[start:end] with spaces, keeping newlines, so offsets and
// line numbers outside the span are unchanged.
func blank(src string, start, end int) string {
	b := []byte(src)
	for i := start; i < end; i++ {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
	return string(b)
}

func sortElements(elements []Element) {
	for i := 1; i < len(elements); i++ {
		for j := i; j > 0 && elements[j].Start < elements[j-1].Start; j-- {
			elements[j], elements[j-1] = elements[j-1], elements[j]
		}
	}
}

func buildConstraint(node *ast.File) string {
	for _, group := range node.Comments {
		if group.Pos() >= node.Package {
			break
		}
		for _, c := range group.List {
			if expr, ok := strings.CutPrefix(c.Text, "//go:build "); ok {
				return strings.TrimSpace(expr)
			}
		}
	}
	return ""
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

// ReceiverTypeName is the base type name of a method receiver: *T, T[K] and
// T[K, V] all name T.
func ReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return ReceiverTypeName(t.X)
	case *ast.ParenExpr:
		return ReceiverTypeName(t.X)
	case *ast.IndexExpr:
		return ReceiverTypeName(t.X)
	case *ast.IndexListExpr:
		return ReceiverTypeName(t.X)
	}
	return ""
}

func funcSignature(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString("(" + gotypes.ExprString(d.Recv.List[0].Type) + ") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString(strings.TrimPrefix(gotypes.ExprString(d.Type), "func"))
	return sb.String()
}

// specSignature is the first line of a spec with its declaration keyword, so
// every row reads as a declaration whether or not the spec is grouped.
func specSignature(text string, tok token.Token) string {
	line, _, _ := strings.Cut(text, "\n")
	return tok.String() + " " + strings.TrimSpace(line)
}
