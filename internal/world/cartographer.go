package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Cartographer implements the "Holographic" Code Graph projection.
// It parses code to emit rich structural facts:
// - code_defines(File, Symbol, Type, StartLine, EndLine)
// - code_calls(Caller, Callee)
// - code_implements(Struct, Interface)
//
// Data flow facts (via MultiLangDataFlowExtractor):
// - assigns(Var, TypeClass, File, Line)
// - guards_return(Var, GuardType, File, Line)
// - guards_block(Var, GuardType, File, StartLine, EndLine)
// - uses(File, Func, Var, Line)
// - safe_access(Var, AccessType, File, Line) - for language-specific safe patterns
// - function_scope(File, Func, Start, End) - function boundaries
// - guard_dominates(File, Func, GuardLine, EndLine) - early return domination
//
// Supports: Go, Python, TypeScript, JavaScript, Rust
type Cartographer struct {
	dataFlowExtractor *MultiLangDataFlowExtractor
	parsers           tsParserSet
}

// NewCartographer creates a new Cartographer for holographic code graph projection.
func NewCartographer() *Cartographer {
	logging.WorldDebug("Creating new Cartographer with MultiLangDataFlowExtractor")
	return &Cartographer{
		dataFlowExtractor: NewMultiLangDataFlowExtractor(),
	}
}

// MapFile parses a single file and returns holographic facts.
// Go is mapped with go/ast; Python, TypeScript, JavaScript and Rust with
// tree-sitter (see cartographer_multilang.go).
func (c *Cartographer) MapFile(path string) ([]core.Fact, error) {
	return c.MapFileAs(path, path)
}

// MapFileAs maps the file at fsPath but labels every emitted fact with
// factPath. Deep facts must carry the same canonical (workspace-relative)
// identity as file_topology, while the parser needs a path it can actually
// open — before this split, deep scans run from outside the workspace either
// failed to open the file or stamped absolute paths into the fact store.
func (c *Cartographer) MapFileAs(fsPath, factPath string) ([]core.Fact, error) {
	logging.WorldDebug("Cartographer mapping file: %s", filepath.Base(fsPath))
	ext := strings.ToLower(filepath.Ext(fsPath))
	if ext == ".go" {
		return c.mapGoFile(fsPath, factPath)
	}
	if lang := DetectLanguage(fsPath); lang != "" && deepMappableExt(ext) {
		return c.mapNonGoFile(fsPath, factPath, lang)
	}
	logging.WorldDebug("Cartographer: unsupported file type %s for %s", ext, filepath.Base(fsPath))
	return nil, nil
}

func (c *Cartographer) mapGoFile(fsPath, path string) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("Cartographer: mapping Go file: %s", filepath.Base(fsPath))

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fsPath, nil, parser.ParseComments)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Cartographer: Go parse failed: %s - %v", fsPath, err)
		return nil, err
	}

	var facts []core.Fact
	pkgName := node.Name.Name
	logging.WorldDebug("Cartographer: package=%s for %s", pkgName, filepath.Base(path))
	facts = append(facts, goSymbolFacts(fset, node, path, pkgName)...)

	symbolFactCount := len(facts)
	logging.WorldDebug("Cartographer: extracted %d symbol facts from %s", symbolFactCount, filepath.Base(path))

	// Extract data flow facts (enhancement, not critical - errors don't break symbol extraction)
	if c.dataFlowExtractor != nil {
		dataFlowFacts, err := c.dataFlowExtractor.ExtractDataFlow(fsPath)
		if err != nil {
			logging.WorldDebug("Cartographer: data flow extraction failed for %s: %v (continuing with symbol facts only)", filepath.Base(path), err)
			// Continue - data flow is an enhancement, not critical
		} else {
			facts = append(facts, types.RelabelPathArgs(dataFlowFacts, fsPath, path)...)
			logging.WorldDebug("Cartographer: extracted %d data flow facts from %s", len(dataFlowFacts), filepath.Base(fsPath))
		}
	}

	logging.WorldDebug("Cartographer: mapped %s - %d total facts (%d symbol, %d data flow) in %v",
		filepath.Base(path), len(facts), symbolFactCount, len(facts)-symbolFactCount, time.Since(start))
	return facts, nil
}

// goPredeclared is the universe block: builtins and predeclared types.
// A call of one of these is not a code_element the scanner will emit
// (buildRef keys on the file's package clause), so it must not grow an fn: row.
var goPredeclared = map[string]bool{
	"any": true, "append": true, "bool": true, "byte": true, "cap": true,
	"clear": true, "close": true, "comparable": true, "complex": true,
	"complex64": true, "complex128": true, "copy": true, "delete": true,
	"error": true, "false": true, "float32": true, "float64": true,
	"imag": true, "int": true, "int8": true, "int16": true, "int32": true,
	"int64": true, "iota": true, "len": true, "make": true, "max": true,
	"min": true, "new": true, "nil": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true, "rune": true,
	"string": true, "true": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
}

// callScope is one AST node on the walk. ast.Inspect reports leaving a node
// by calling the visitor with nil, so every node pushes a frame and nil pops
// it: the enclosing function does not leak onto a later package-level call.
// A single currentFunction assignment did — var x = helper() after func F was
// credited to F (cartographer_multilang.go names that leak).
type callScope struct {
	fn      string
	inFunc  bool
	scope   bool
	binds   map[string]string
	params  map[string]bool
	pending map[string]string
}

// goSymbolFacts emits code_defines and code_calls for one Go file.
//
// The bare code_calls row keeps the spelling impact.mg joins against
// modified_function (core/codedom_modified_symbols.go symbolIDFromRef):
// <pkg>.<Name> for an identifier, <expr>.<Sel> for a selector whose
// expression is an identifier. Both slots are /string (schemas_analysis.mg).
//
// The fn: row is the code_element spelling (go_parser.go buildRef):
// fn:<pkg>.<Name> for a same-package identifier, fn:<pkg>.<Type>.<Sel>
// for a selector whose receiver type is a named type of this package
// written in the AST (composite literal or explicit type, including a
// pointer to one). t.Fail and fmt.Println do not resolve that way, and a
// package-qualified call is not spelled here either: the qualifier is the
// import name, which an alias would make different from the package clause
// buildRef records. A wrong fn: row becomes test_depends_on anyway — the
// call rule joins the strings and does not require the callee to be a
// code_element (test_impact.mg).
func goSymbolFacts(fset *token.FileSet, node *ast.File, path, pkgName string) []core.Fact {
	localTypes := fileTypeNames(node)
	pkgBinds := packageVarTypes(node)
	dotImport := hasDotImport(node)

	var facts []core.Fact
	var stack []callScope

	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			if len(stack) == 0 {
				return true
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if len(top.pending) > 0 {
				applyCallBinds(stack, top.pending)
			}
			return true
		}

		frame := callScope{}
		if len(stack) > 0 {
			frame.fn = stack[len(stack)-1].fn
			frame.inFunc = stack[len(stack)-1].inFunc
		}

		switch x := n.(type) {
		case *ast.File:
			frame.scope = true
			frame.binds = pkgBinds

		case *ast.FuncDecl:
			name := x.Name.Name
			recv := receiverTypeName(x)
			id := fmt.Sprintf("%s.%s", pkgName, name)
			if recv != "" {
				id = fmt.Sprintf("%s.%s.%s", pkgName, recv, name)
			}
			frame.fn = id
			frame.inFunc = true
			frame.scope = true
			frame.params = funcTypeParams(x.Recv, x.Type)
			frame.binds = funcValueBinds(x.Recv, x.Type, stack, frame.params)

			start := fset.Position(x.Pos()).Line
			end := fset.Position(x.End()).Line
			// Symbol is /string (schemas_analysis.mg). A MangleAtom without
			// a leading "/" falls back to a string in ToAtom anyway.
			facts = append(facts, core.Fact{
				Predicate: "code_defines",
				Args: []any{
					path,
					id,
					core.MangleAtom("/function"),
					int64(start),
					int64(end),
				},
			})

		case *ast.FuncLit:
			frame.inFunc = true
			frame.scope = true
			frame.params = funcTypeParams(nil, x.Type)
			frame.binds = funcValueBinds(nil, x.Type, stack, frame.params)

		case *ast.TypeSpec:
			name := x.Name.Name
			id := fmt.Sprintf("%s.%s", pkgName, name)
			start := fset.Position(x.Pos()).Line
			end := fset.Position(x.End()).Line
			typeType := "/type"
			if _, ok := x.Type.(*ast.StructType); ok {
				typeType = "/struct"
			} else if _, ok := x.Type.(*ast.InterfaceType); ok {
				typeType = "/interface"
			}
			facts = append(facts, core.Fact{
				Predicate: "code_defines",
				Args: []any{
					path,
					id,
					core.MangleAtom(typeType),
					int64(start),
					int64(end),
				},
			})

		case *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause:
			frame.scope = true

		case *ast.RangeStmt:
			// Range variables are in scope for the body. Their element type
			// is not on the AST, so the binding only shadows an outer name
			// and blocks a method ref for the wrong type.
			frame.scope = true
			frame.binds = rangeBinds(x.Key, x.Value)

		case *ast.ValueSpec:
			if frame.inFunc {
				frame.pending = valueSpecTypes(x, stack, nil)
			}

		case *ast.AssignStmt:
			if frame.inFunc && x.Tok == token.DEFINE {
				frame.pending = shortAssignTypes(x, stack, nil)
			}

		case *ast.CallExpr:
			caller := frame.fn
			if caller == "" {
				break
			}
			bare, fnCallee := goCallRef(x.Fun, pkgName, stack, localTypes, dotImport)
			if bare != "" {
				facts = append(facts, core.Fact{
					Predicate: "code_calls",
					Args:      []any{caller, bare},
				})
			}
			if fnCallee != "" {
				facts = append(facts, core.Fact{
					Predicate: "code_calls",
					Args:      []any{"fn:" + caller, fnCallee},
				})
			}
		}

		stack = append(stack, frame)
		return true
	})
	return facts
}

func goCallRef(fun ast.Expr, pkg string, stack []callScope, localTypes map[string]bool, dotImport bool) (bare, fnCallee string) {
	switch f := fun.(type) {
	case *ast.ParenExpr:
		return goCallRef(f.X, pkg, stack, localTypes, dotImport)
	case *ast.Ident:
		bare = pkg + "." + f.Name
		if dotImport || goPredeclared[f.Name] || localTypes[f.Name] || typeParamBound(stack, f.Name) || nameBound(stack, f.Name) {
			return bare, ""
		}
		return bare, "fn:" + bare
	case *ast.SelectorExpr:
		// Bare spelling is unchanged: only an identifier expression, copied
		// as written. The fn: row is a different string — the method's
		// code_element — and only when the receiver type is known.
		if id, ok := f.X.(*ast.Ident); ok {
			bare = id.Name + "." + f.Sel.Name
		}
		if typ, ok := selectorReceiverType(f.X, stack); ok {
			fnCallee = "fn:" + pkg + "." + typ + "." + f.Sel.Name
		}
		return bare, fnCallee
	default:
		return "", ""
	}
}

// selectorReceiverType is the named same-package type of a call receiver
// when the AST states it: a bound value (composite literal or explicit
// type), a composite literal, or a type assert. A package qualifier
// (fmt.Println, q.Target) is an unbound identifier and returns false.
func selectorReceiverType(expr ast.Expr, stack []callScope) (string, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return selectorReceiverType(e.X, stack)
	case *ast.Ident:
		return lookupBind(stack, e.Name)
	case *ast.CompositeLit:
		return samePackageTypeName(e.Type, stack, nil)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return compositeLitType(e.X, stack)
		}
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			return samePackageTypeName(e.Type, stack, nil)
		}
	}
	return "", false
}

func compositeLitType(expr ast.Expr, stack []callScope) (string, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return compositeLitType(e.X, stack)
	case *ast.CompositeLit:
		return samePackageTypeName(e.Type, stack, nil)
	default:
		return "", false
	}
}

// samePackageTypeName unwraps pointers and instantiations down to a bare
// identifier. A selector (q.S, testing.T) is another package. Predeclared
// names and in-scope type parameters are not package types.
func samePackageTypeName(expr ast.Expr, stack []callScope, extra map[string]bool) (string, bool) {
	for expr != nil {
		switch t := expr.(type) {
		case *ast.ParenExpr:
			expr = t.X
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.Ident:
			if t.Name == "" || t.Name == "_" || goPredeclared[t.Name] || extra[t.Name] || typeParamBound(stack, t.Name) {
				return "", false
			}
			return t.Name, true
		default:
			return "", false
		}
	}
	return "", false
}

func fileTypeNames(file *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				out[ts.Name.Name] = true
			}
		}
	}
	return out
}

func hasDotImport(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Name != nil && imp.Name.Name == "." {
			return true
		}
	}
	return false
}

// packageVarTypes binds file-level vars whose type is written in the AST.
// Package scope covers the whole file, so a function declared above the var
// still sees it; function-level bindings are applied when their spec ends.
func packageVarTypes(file *ast.File) map[string]string {
	var binds map[string]string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for name, typ := range valueSpecTypes(vs, nil, nil) {
				if binds == nil {
					binds = map[string]string{}
				}
				binds[name] = typ
			}
		}
	}
	return binds
}

func valueSpecTypes(vs *ast.ValueSpec, stack []callScope, extra map[string]bool) map[string]string {
	out := map[string]string{}
	put := func(name, typ string) {
		if name == "" || name == "_" {
			return
		}
		out[name] = typ
	}
	if vs.Type != nil {
		typ := ""
		if name, ok := samePackageTypeName(vs.Type, stack, extra); ok {
			typ = name
		}
		for _, n := range vs.Names {
			put(n.Name, typ)
		}
		return out
	}
	if len(vs.Values) == len(vs.Names) {
		for i, n := range vs.Names {
			typ := ""
			if name, ok := rhsTypeName(vs.Values[i], stack, extra); ok {
				typ = name
			}
			put(n.Name, typ)
		}
		return out
	}
	for _, n := range vs.Names {
		put(n.Name, "")
	}
	return out
}

func shortAssignTypes(as *ast.AssignStmt, stack []callScope, extra map[string]bool) map[string]string {
	out := map[string]string{}
	put := func(expr ast.Expr, typ string) {
		id, ok := expr.(*ast.Ident)
		if !ok {
			return
		}
		if id.Name == "" || id.Name == "_" {
			return
		}
		out[id.Name] = typ
	}
	if len(as.Lhs) == 2 && len(as.Rhs) == 1 {
		if ta, ok := as.Rhs[0].(*ast.TypeAssertExpr); ok && ta.Type != nil {
			typ := ""
			if name, ok := samePackageTypeName(ta.Type, stack, extra); ok {
				typ = name
			}
			put(as.Lhs[0], typ)
			put(as.Lhs[1], "")
			return out
		}
	}
	if len(as.Lhs) == len(as.Rhs) {
		for i, lhs := range as.Lhs {
			typ := ""
			if name, ok := rhsTypeName(as.Rhs[i], stack, extra); ok {
				typ = name
			}
			put(lhs, typ)
		}
		return out
	}
	for _, lhs := range as.Lhs {
		put(lhs, "")
	}
	return out
}

func rhsTypeName(expr ast.Expr, stack []callScope, extra map[string]bool) (string, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return rhsTypeName(e.X, stack, extra)
	case *ast.CompositeLit:
		return samePackageTypeName(e.Type, stack, extra)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return rhsTypeName(e.X, stack, extra)
		}
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			return samePackageTypeName(e.Type, stack, extra)
		}
	}
	return "", false
}

// receiverTypeName is the same base name extractReceiverTypeInfo feeds to
// buildRef, so a call inside func (b *Box[T]) Get is attributed to
// fn:<pkg>.Box.Get and not to fn:<pkg>.Get.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	name, _ := extractReceiverTypeInfo(fn.Recv.List[0].Type)
	return name
}

func funcTypeParams(recv *ast.FieldList, typ *ast.FuncType) map[string]bool {
	var out map[string]bool
	add := func(name string) {
		if name == "" || name == "_" {
			return
		}
		if out == nil {
			out = map[string]bool{}
		}
		out[name] = true
	}
	if typ != nil && typ.TypeParams != nil {
		for _, f := range typ.TypeParams.List {
			for _, n := range f.Names {
				add(n.Name)
			}
		}
	}
	if recv != nil {
		for _, f := range recv.List {
			for name := range receiverTypeArgIdents(f.Type) {
				add(name)
			}
		}
	}
	return out
}

func receiverTypeArgIdents(expr ast.Expr) map[string]bool {
	for expr != nil {
		switch t := expr.(type) {
		case *ast.ParenExpr:
			expr = t.X
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			return identArg(t.Index)
		case *ast.IndexListExpr:
			out := map[string]bool{}
			for _, ix := range t.Indices {
				for name := range identArg(ix) {
					out[name] = true
				}
			}
			if len(out) == 0 {
				return nil
			}
			return out
		default:
			return nil
		}
	}
	return nil
}

func identArg(expr ast.Expr) map[string]bool {
	id, ok := expr.(*ast.Ident)
	if !ok || id.Name == "" || id.Name == "_" {
		return nil
	}
	return map[string]bool{id.Name: true}
}

func funcValueBinds(recv *ast.FieldList, typ *ast.FuncType, stack []callScope, extra map[string]bool) map[string]string {
	out := map[string]string{}
	addList := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			typName := ""
			if name, ok := samePackageTypeName(f.Type, stack, extra); ok {
				typName = name
			}
			for _, n := range f.Names {
				if n.Name == "" || n.Name == "_" {
					continue
				}
				out[n.Name] = typName
			}
		}
	}
	addList(recv)
	if typ != nil {
		addList(typ.Params)
		addList(typ.Results)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func rangeBinds(key, value ast.Expr) map[string]string {
	out := map[string]string{}
	for _, e := range []ast.Expr{key, value} {
		id, ok := e.(*ast.Ident)
		if !ok || id.Name == "" || id.Name == "_" {
			continue
		}
		out[id.Name] = ""
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyCallBinds(stack []callScope, pending map[string]string) {
	for i := len(stack) - 1; i >= 0; i-- {
		if !stack[i].scope {
			continue
		}
		if stack[i].binds == nil {
			stack[i].binds = make(map[string]string, len(pending))
		}
		for name, typ := range pending {
			if name == "" || name == "_" {
				continue
			}
			// Already declared in this scope: := reassigns, it does not
			// change the type. A name that exists only further out is a
			// new binding and must shadow, even when the type is unknown.
			if _, exists := stack[i].binds[name]; exists {
				continue
			}
			stack[i].binds[name] = typ
		}
		return
	}
}

func lookupBind(stack []callScope, name string) (string, bool) {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].binds == nil {
			continue
		}
		typ, ok := stack[i].binds[name]
		if !ok {
			continue
		}
		if typ == "" {
			return "", false
		}
		return typ, true
	}
	return "", false
}

func nameBound(stack []callScope, name string) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].binds == nil {
			continue
		}
		if _, ok := stack[i].binds[name]; ok {
			return true
		}
	}
	return false
}

func typeParamBound(stack []callScope, name string) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].params[name] {
			return true
		}
	}
	return false
}

// Close releases resources held by the Cartographer.
func (c *Cartographer) Close() {
	if c.dataFlowExtractor != nil {
		c.dataFlowExtractor.Close()
	}
	c.parsers.close()
}

// SupportedLanguages returns the list of languages supported for data flow extraction.
func (c *Cartographer) SupportedLanguages() []string {
	return []string{"go", "python", "typescript", "javascript", "rust"}
}

// IsLanguageSupported checks if a file's language is supported for data flow extraction.
func (c *Cartographer) IsLanguageSupported(path string) bool {
	lang := DetectLanguage(path)
	return slices.Contains(c.SupportedLanguages(), lang)
}
