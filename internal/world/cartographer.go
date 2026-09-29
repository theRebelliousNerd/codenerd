package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	gotypes "go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	facts = append(facts, goSymbolFacts(fset, node, path, fsPath, pkgName)...)

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

// goPredeclared reports a name of the universe block: builtins and
// predeclared types. A call of one of these is not a code_element the
// scanner will emit (buildRef keys on the file's package clause), so it
// must not grow an fn: row. The toolchain's own universe is the list, so a
// builtin added by a later Go release needs no edit here.
func goPredeclared(name string) bool {
	return gotypes.Universe.Lookup(name) != nil
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
// code_element (test_impact.mg). An identifier gets an fn: row when this
// package declares that function, including in a file with a dot import.
// Any other identifier gets no fn: row: it may be the dot import, and that
// package is not spelled. A conversion of a type declared in another file
// of the package is not a call. Method rows are emitted only when a
// FuncDecl on that named type exists, so a promoted method or an alias to
// another package is not guessed.
func goSymbolFacts(fset *token.FileSet, node *ast.File, path, fsPath, pkgName string) []core.Fact {
	sym := symbolsFor(node, fsPath, pkgName)
	pkgBinds := packageVarTypes(node, sym)

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
			frame.binds = funcValueBinds(x.Recv, x.Type, stack, frame.params, sym)

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
			frame.binds = funcValueBinds(nil, x.Type, stack, frame.params, sym)

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
				frame.pending = valueSpecTypes(x, stack, nil, sym)
			}

		case *ast.AssignStmt:
			if frame.inFunc && x.Tok == token.DEFINE {
				frame.pending = shortAssignTypes(x, stack, nil, sym)
			}

		case *ast.CallExpr:
			caller := frame.fn
			if caller == "" {
				break
			}
			bare, fnCallee := goCallRef(x.Fun, pkgName, stack, sym)
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

func goCallRef(fun ast.Expr, pkg string, stack []callScope, sym pkgSymbols) (bare, fnCallee string) {
	switch f := fun.(type) {
	case *ast.ParenExpr:
		return goCallRef(f.X, pkg, stack, sym)
	case *ast.IndexExpr:
		// F[T](x) is a call whose Fun is an index. m[k]() is a value index
		// and must not grow a row, so the base has to be a declared function
		// or a selector and the index has to be a type.
		if instantiatedCall(f.X, []ast.Expr{f.Index}, stack, sym) {
			return goCallRef(f.X, pkg, stack, sym)
		}
		return "", ""
	case *ast.IndexListExpr:
		if instantiatedCall(f.X, f.Indices, stack, sym) {
			return goCallRef(f.X, pkg, stack, sym)
		}
		return "", ""
	case *ast.Ident:
		bare = pkg + "." + f.Name
		if goPredeclared(f.Name) || sym.types[f.Name] || typeParamBound(stack, f.Name) || nameBound(stack, f.Name) {
			return bare, ""
		}
		if ref := identFnRef(pkg, f.Name, sym); ref != "" {
			return bare, ref
		}
		return bare, ""
	case *ast.SelectorExpr:
		// Bare spelling is unchanged: only an identifier expression, copied
		// as written. The fn: row is the method's code_element, and only when
		// this package declares that method on the receiver's named type.
		if id, ok := f.X.(*ast.Ident); ok {
			bare = id.Name + "." + f.Sel.Name
		}
		if typ, ok := selectorReceiverType(f.X, stack, sym); ok && methodKnown(sym, typ, f.Sel.Name) {
			fnCallee = "fn:" + pkg + "." + typ + "." + f.Sel.Name
		} else if typ, ok := methodExprReceiver(f.X, stack, sym); ok && methodKnown(sym, typ, f.Sel.Name) {
			fnCallee = "fn:" + pkg + "." + typ + "." + f.Sel.Name
		}
		return bare, fnCallee
	default:
		return "", ""
	}
}

// identFnRef is the fn: callee of an unqualified call. The caller has
// already rejected a predeclared name, a type conversion, a type parameter,
// and a bound value. A file with a dot import keeps fn:<pkg>.<Name> when
// this package declares the function. Any other identifier gets no fn: row:
// the imported package is not resolved and not spelled.
func identFnRef(pkg, name string, sym pkgSymbols) string {
	if sym.funcs[name] {
		return "fn:" + pkg + "." + name
	}
	return ""
}

// selectorReceiverType is the named same-package type of a call receiver
// when the AST states it: a bound value (composite literal or explicit
// type), a composite literal, or a type assert. A package qualifier
// (fmt.Println, q.Target) is an unbound identifier and returns false.
func selectorReceiverType(expr ast.Expr, stack []callScope, sym pkgSymbols) (string, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return selectorReceiverType(e.X, stack, sym)
	case *ast.StarExpr:
		// (*s).M() — the dereference keeps the named type the binding recorded.
		return selectorReceiverType(e.X, stack, sym)
	case *ast.Ident:
		return lookupBind(stack, e.Name)
	case *ast.CompositeLit:
		return samePackageTypeName(e.Type, stack, nil, sym)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return compositeLitType(e.X, stack, sym)
		}
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			return samePackageTypeName(e.Type, stack, nil, sym)
		}
	}
	return "", false
}

func compositeLitType(expr ast.Expr, stack []callScope, sym pkgSymbols) (string, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return compositeLitType(e.X, stack, sym)
	case *ast.CompositeLit:
		return samePackageTypeName(e.Type, stack, nil, sym)
	default:
		return "", false
	}
}

// methodExprReceiver is the named type in a method expression S.M or (*S).M.
// The type has to be declared in this package. A package qualifier (fmt.Println)
// is an unbound identifier that is not one of those types.
func methodExprReceiver(expr ast.Expr, stack []callScope, sym pkgSymbols) (string, bool) {
	for expr != nil {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			if !indexIsType(e.Index, stack, sym, false) {
				return "", false
			}
			expr = e.X
		case *ast.IndexListExpr:
			for _, ix := range e.Indices {
				if !indexIsType(ix, stack, sym, false) {
					return "", false
				}
			}
			expr = e.X
		case *ast.Ident:
			if e.Name == "" || nameBound(stack, e.Name) || typeParamBound(stack, e.Name) || goPredeclared(e.Name) || !sym.types[e.Name] {
				return "", false
			}
			return resolveAlias(e.Name, sym)
		default:
			return "", false
		}
	}
	return "", false
}

// samePackageTypeName unwraps pointers and instantiations down to a bare
// identifier. A selector (q.S, testing.T) is another package. Predeclared
// names and in-scope type parameters are not package types.
func samePackageTypeName(expr ast.Expr, stack []callScope, extra map[string]bool, sym pkgSymbols) (string, bool) {
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
			if t.Name == "" || t.Name == "_" || goPredeclared(t.Name) || extra[t.Name] || typeParamBound(stack, t.Name) {
				return "", false
			}
			// type A = S names S's methods; an alias whose target is not a
			// named type of this package (fmt.Stringer, map[int]int) has none
			// we can spell.
			return resolveAlias(t.Name, sym)
		default:
			return "", false
		}
	}
	return "", false
}

// packageVarTypes binds file-level vars whose type is written in the AST.
// Package scope covers the whole file, so a function declared above the var
// still sees it; function-level bindings are applied when their spec ends.
func packageVarTypes(file *ast.File, sym pkgSymbols) map[string]string {
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
			for name, typ := range valueSpecTypes(vs, nil, nil, sym) {
				if binds == nil {
					binds = map[string]string{}
				}
				binds[name] = typ
			}
		}
	}
	return binds
}

func valueSpecTypes(vs *ast.ValueSpec, stack []callScope, extra map[string]bool, sym pkgSymbols) map[string]string {
	out := map[string]string{}
	put := func(name, typ string) {
		if name == "" || name == "_" {
			return
		}
		out[name] = typ
	}
	if vs.Type != nil {
		typ := ""
		if name, ok := samePackageTypeName(vs.Type, stack, extra, sym); ok {
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
			if name, ok := rhsTypeName(vs.Values[i], stack, extra, sym); ok {
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

func shortAssignTypes(as *ast.AssignStmt, stack []callScope, extra map[string]bool, sym pkgSymbols) map[string]string {
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
			if name, ok := samePackageTypeName(ta.Type, stack, extra, sym); ok {
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
			if name, ok := rhsTypeName(as.Rhs[i], stack, extra, sym); ok {
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

func rhsTypeName(expr ast.Expr, stack []callScope, extra map[string]bool, sym pkgSymbols) (string, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return rhsTypeName(e.X, stack, extra, sym)
	case *ast.CompositeLit:
		return samePackageTypeName(e.Type, stack, extra, sym)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return rhsTypeName(e.X, stack, extra, sym)
		}
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			return samePackageTypeName(e.Type, stack, extra, sym)
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

func funcValueBinds(recv *ast.FieldList, typ *ast.FuncType, stack []callScope, extra map[string]bool, sym pkgSymbols) map[string]string {
	out := map[string]string{}
	addList := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			typName := ""
			if name, ok := samePackageTypeName(f.Type, stack, extra, sym); ok {
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

func methodKnown(sym pkgSymbols, typ, sel string) bool {
	if typ == "" || sel == "" {
		return false
	}
	return sym.methods[typ][sel]
}

func resolveAlias(name string, sym pkgSymbols) (string, bool) {
	seen := map[string]bool{}
	for {
		if seen[name] || sym.aliasBad[name] {
			return "", false
		}
		seen[name] = true
		next, ok := sym.aliases[name]
		if !ok {
			return name, true
		}
		name = next
	}
}

// instantiatedCall reports whether Fun is a generic call F[T](x) rather than
// a value index m[k](). The base is a function this package declares, or a
// selector (a method instantiation). Every index has to be a type.
func instantiatedCall(base ast.Expr, indices []ast.Expr, stack []callScope, sym pkgSymbols) bool {
	kind := instBase(base, stack, sym)
	if kind == instNone || len(indices) == 0 {
		return false
	}
	allowSel := kind == instFunc
	for _, ix := range indices {
		if !indexIsType(ix, stack, sym, allowSel) {
			return false
		}
	}
	return true
}

const (
	instNone = iota
	instFunc
	instSel
)

func instBase(expr ast.Expr, stack []callScope, sym pkgSymbols) int {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return instBase(e.X, stack, sym)
	case *ast.Ident:
		if nameBound(stack, e.Name) || typeParamBound(stack, e.Name) || goPredeclared(e.Name) {
			return instNone
		}
		if sym.funcs[e.Name] {
			return instFunc
		}
		return instNone
	case *ast.SelectorExpr:
		return instSel
	default:
		return instNone
	}
}

func indexIsType(expr ast.Expr, stack []callScope, sym pkgSymbols, allowPkgSel bool) bool {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return indexIsType(e.X, stack, sym, allowPkgSel)
	case *ast.Ident:
		if nameBound(stack, e.Name) {
			return false
		}
		if typeParamBound(stack, e.Name) || sym.types[e.Name] {
			return true
		}
		return goPredeclaredType(e.Name)
	case *ast.StarExpr:
		return indexIsType(e.X, stack, sym, allowPkgSel)
	case *ast.ArrayType, *ast.MapType, *ast.StructType, *ast.InterfaceType, *ast.FuncType, *ast.ChanType:
		return true
	case *ast.IndexExpr:
		return indexIsType(e.X, stack, sym, allowPkgSel) && indexIsType(e.Index, stack, sym, allowPkgSel)
	case *ast.IndexListExpr:
		if !indexIsType(e.X, stack, sym, allowPkgSel) {
			return false
		}
		for _, ix := range e.Indices {
			if !indexIsType(ix, stack, sym, allowPkgSel) {
				return false
			}
		}
		return true
	case *ast.SelectorExpr:
		// pkg.T is a type argument only on a call we already know is a
		// function instantiation. A selector index of a method value is a
		// field, and spelling it as a type would invent a call.
		if !allowPkgSel {
			return false
		}
		id, ok := e.X.(*ast.Ident)
		if !ok || nameBound(stack, id.Name) || typeParamBound(stack, id.Name) || sym.types[id.Name] || sym.funcs[id.Name] {
			return false
		}
		return true
	case *ast.BinaryExpr:
		if e.Op != token.OR {
			return false
		}
		return indexIsType(e.X, stack, sym, allowPkgSel) && indexIsType(e.Y, stack, sym, allowPkgSel)
	default:
		return false
	}
}

func goPredeclaredType(name string) bool {
	_, ok := gotypes.Universe.Lookup(name).(*gotypes.TypeName)
	return ok
}

// pkgSymbols is the named types, functions and methods of one compilation
// package. MapFile sees a single file; a conversion T(x) or a method
// expression S.M is only certain once the rest of the directory has been
// read. HolographicProvider.parsePackage already walks that directory, but
// it drops _test.go files and it is not on the MapFile path, so the call
// graph collects the names itself.
type pkgSymbols struct {
	types    map[string]bool
	funcs    map[string]bool
	methods  map[string]map[string]bool
	aliases  map[string]string
	aliasBad map[string]bool
}

func (s *pkgSymbols) init() {
	if s.types == nil {
		s.types = map[string]bool{}
	}
	if s.funcs == nil {
		s.funcs = map[string]bool{}
	}
	if s.methods == nil {
		s.methods = map[string]map[string]bool{}
	}
	if s.aliases == nil {
		s.aliases = map[string]string{}
	}
	if s.aliasBad == nil {
		s.aliasBad = map[string]bool{}
	}
}

func (s pkgSymbols) clone() pkgSymbols {
	out := pkgSymbols{}
	out.init()
	for k, v := range s.types {
		out.types[k] = v
	}
	for k, v := range s.funcs {
		out.funcs[k] = v
	}
	for k, v := range s.aliases {
		out.aliases[k] = v
	}
	for k, v := range s.aliasBad {
		out.aliasBad[k] = v
	}
	for recv, methods := range s.methods {
		m := make(map[string]bool, len(methods))
		for name := range methods {
			m[name] = true
		}
		out.methods[recv] = m
	}
	return out
}

type cachedSymbols struct {
	stamp string
	sym   pkgSymbols
}

var (
	symbolCacheMu sync.Mutex
	symbolCache   = map[string]cachedSymbols{}
)

func symbolsFor(node *ast.File, fsPath, pkgName string) pkgSymbols {
	includeTests := strings.HasSuffix(filepath.Base(fsPath), "_test.go")
	base, ok := loadSymbols(filepath.Dir(fsPath), pkgName, includeTests)
	if !ok {
		base.init()
	} else {
		base = base.clone()
	}
	if node != nil && node.Name != nil && node.Name.Name == pkgName {
		absorbNode(&base, node)
	}
	return base
}

func loadSymbols(dir, pkgName string, includeTests bool) (pkgSymbols, bool) {
	stamp, files, err := goFileStamp(dir)
	if err != nil {
		return pkgSymbols{}, false
	}
	key := dir + "\x00" + pkgName + "\x00"
	if includeTests {
		key += "test"
	} else {
		key += "lib"
	}
	symbolCacheMu.Lock()
	if ent, ok := symbolCache[key]; ok && ent.stamp == stamp {
		symbolCacheMu.Unlock()
		return ent.sym, true
	}
	symbolCacheMu.Unlock()

	sym := pkgSymbols{}
	sym.init()
	for _, path := range files {
		if !includeTests && strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil || file == nil || file.Name == nil || file.Name.Name != pkgName {
			continue
		}
		absorbNode(&sym, file)
	}
	symbolCacheMu.Lock()
	symbolCache[key] = cachedSymbols{stamp: stamp, sym: sym}
	symbolCacheMu.Unlock()
	return sym, true
}

// goFileStamp lists the .go files directly in dir. os.ReadDir does not walk
// parents, child directories, or any path outside dir, so the loader cannot
// open GOROOT or the module cache. The stamp is each file's name, size, and
// mtime — the same size and mtime pair ScanWorkspaceIncremental uses to
// decide a file changed (incremental_scan.go). A rewrite, add, or removal
// therefore misses this cache instead of reusing symbols parsed before it.
func goFileStamp(dir string) (string, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s %d %d\n", name, info.Size(), info.ModTime().UnixNano())
		files = append(files, filepath.Join(dir, name))
	}
	return b.String(), files, nil
}

func absorbNode(sym *pkgSymbols, file *ast.File) {
	if sym == nil || file == nil {
		return
	}
	sym.init()
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name == nil || d.Name.Name == "" || d.Name.Name == "_" {
				continue
			}
			if d.Recv == nil {
				sym.funcs[d.Name.Name] = true
				continue
			}
			base := receiverTypeName(d)
			if base == "" {
				continue
			}
			if sym.methods[base] == nil {
				sym.methods[base] = map[string]bool{}
			}
			sym.methods[base][d.Name.Name] = true
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil || ts.Name.Name == "" {
					continue
				}
				sym.types[ts.Name.Name] = true
				if !ts.Assign.IsValid() {
					continue
				}
				if target, ok := aliasTargetIdent(ts.Type); ok {
					sym.aliases[ts.Name.Name] = target
				} else {
					sym.aliasBad[ts.Name.Name] = true
				}
			}
		}
	}
}

func aliasTargetIdent(expr ast.Expr) (string, bool) {
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
			if t.Name == "" || t.Name == "_" || goPredeclared(t.Name) {
				return "", false
			}
			return t.Name, true
		default:
			return "", false
		}
	}
	return "", false
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
