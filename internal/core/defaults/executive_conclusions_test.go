package defaults

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// EXECUTIVE CONCLUSIONS ARE CONSUMED
// =============================================================================
// The dual of the starved-predicate gate. That one guards predicates a rule
// READS and nothing produces; this one guards conclusions the kernel PRODUCES
// and the executive never acts on -- while Go decides the same thing itself.
// The 2026-09-23 sweep (radical/01-executive-drift) found that drift in every
// subsystem it read: campaign_task_shard derived and never queried while three
// Go tables picked the shard; replan_needed derived, and the live replan
// decided in Go; next_action atoms routed to handlers that reported success
// without work.
//
// Two measurements, one checked-in inventory that may only shrink:
//
//  1. Liveness. A predicate is live when production Go asks the kernel for it
//     -- the predicate named, as a literal, a const or a Sprintf template, in
//     the first argument of a query-shaped call -- or when it feeds, through a
//     rule body, a predicate that is live. A rule head that is not live is a
//     conclusion nothing reads. A bare string literal elsewhere does not count:
//     a gate that a literal satisfies teaches writing literals (see the
//     starved gate's turn_age_category note).
//  2. Routing. Every constant action a rule derives as next_action(/X) must
//     reach a case of the VirtualStore's executeAction switch; otherwise the
//     kernel can decide an action that nothing executes.
//
// Regenerate: CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed

const conclusionsBaselinePath = "testdata/unconsumed_conclusions.txt"

// queryCalls are the calls through which production Go asks the kernel for a
// predicate: the kernel's own query methods and the helpers that wrap them.
// A new helper that wraps Query must be added here, or everything it reads
// shows up as unconsumed.
var queryCalls = map[string]bool{
	"Query":                true,
	"QueryCallback":        true,
	"QueryFacts":           true,
	"GetFacts":             true,
	"QueryRouting":         true,
	"QueryAllLearned":      true,
	"SubscribeToFacts":     true,
	"turnRows":             true,
	"derivedFor":           true,
	"oneDerivedFor":        true,
	"first":                true,
	"acceptanceDerived":    true,
	"askWithCampaignState": true,
	"derivedColumn":        true,
	"queryKernelStrings":   true,
	"queryOrBlock":         true,
	// The repair episode's rows (session/repair_policy.go nextRepairMove).
	"episodeRows": true,
	// Whether a turn-keyed conclusion holds (session/finish_round.go).
	"turnHolds": true,
	// A delivery round's rows (chat/request_delivery.go).
	"roundValues": true,
	"fragmentsIn": true,
}

// finiteQueryWrappers are narrow production wrappers whose query predicate
// space is constrained by a named, finite map literal. The predicates are
// always extracted from the source map; this registry never contains a list
// of predicate names.
type finiteQueryWrapper struct {
	function string
	mapName  string
}

var finiteQueryWrappers = map[string]finiteQueryWrapper{
	"internal/tools/research/browser_reasoning.go": {
		function: "queryScopedBrowserFacts",
		mapName:  "browserPredicateSpecs",
	},
}

var predicateNameRe = regexp.MustCompile(`^\s*([a-z_][a-zA-Z0-9_]*)`)

// mgRule is one rule of the corpus: its head predicate and the predicates its
// body reads.
type mgRule struct {
	head     string
	headText string
	body     []string
	file     string
}

// mangleRules returns every rule (a statement with a body) in the corpus.
func mangleRules(t *testing.T, corpusDir string) []mgRule {
	t.Helper()
	var rules []mgRule
	err := filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var lines []string
		for _, l := range strings.Split(string(data), "\n") {
			lines = append(lines, strings.SplitN(l, "#", 2)[0])
		}
		for _, stmt := range statementSplit.Split(strings.Join(lines, "\n"), -1) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "Decl") {
				continue
			}
			head, body, isRule := strings.Cut(stmt, ":-")
			if !isRule {
				continue
			}
			m := starvedHeadRe.FindStringSubmatch(strings.TrimSpace(head))
			if m == nil {
				continue
			}
			r := mgRule{head: m[1], headText: strings.TrimSpace(head), file: filepath.Base(path)}
			for _, bm := range starvedAtomRe.FindAllStringSubmatch(body, -1) {
				r.body = append(r.body, bm[1])
			}
			rules = append(rules, r)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return rules
}

var inclusionBlockRe = regexp.MustCompile(`inclusion\s*\[([^\]]*)\]`)

// inclusionRoots names every predicate a Decl's inclusion [...] constraint
// reads.
func inclusionRoots(t *testing.T, corpusDir string) map[string]struct{} {
	t.Helper()
	out := make(map[string]struct{})
	err := filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, l := range strings.Split(string(data), "\n") {
			l = strings.SplitN(l, "#", 2)[0]
			if !strings.HasPrefix(strings.TrimSpace(l), "Decl") {
				continue
			}
			for _, block := range inclusionBlockRe.FindAllStringSubmatch(l, -1) {
				for _, m := range starvedAtomRe.FindAllStringSubmatch(block[1], -1) {
					out[m[1]] = struct{}{}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return out
}

var mergeDescrRe = regexp.MustCompile(`merge\(\s*\[[^\]]*\]\s*,\s*"([a-z_][a-z0-9_]*)"\s*\)`)

// mergeRoots names every merge predicate a Decl's descr [merge([...], "p")]
// names: the engine runs it top-down whenever the lattice relation derives a
// fact, so no rule body or Go query mentions it. A descr may sit on a
// continuation line of its Decl, so the whole file is scanned.
func mergeRoots(t *testing.T, corpusDir string) map[string]struct{} {
	t.Helper()
	out := make(map[string]struct{})
	err := filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var code strings.Builder
		for _, l := range strings.Split(string(data), "\n") {
			code.WriteString(strings.SplitN(l, "#", 2)[0] + "\n")
		}
		for _, m := range mergeDescrRe.FindAllStringSubmatch(code.String(), -1) {
			out[m[1]] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return out
}

// predicateFromExpr names the predicate an argument asks for: a string literal
// ("p" or "p(X)"), a const holding one, a Sprintf template starting with one,
// a concatenation starting with one, or (SubscribeToFacts) a literal list.
func predicateFromExpr(expr ast.Expr, consts map[string]string) []string {
	return predicateFromExprWithLocals(expr, consts, nil)
}

func predicateFromExprWithLocals(expr ast.Expr, consts map[string]string, locals map[string][]string) []string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return nil
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return nil
		}
		if m := predicateNameRe.FindStringSubmatch(s); m != nil {
			return []string{m[1]}
		}
	case *ast.Ident:
		if preds, ok := locals[e.Name]; ok {
			return append([]string(nil), preds...)
		}
		if s, ok := consts[e.Name]; ok {
			if m := predicateNameRe.FindStringSubmatch(s); m != nil {
				return []string{m[1]}
			}
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sprintf" && len(e.Args) > 0 {
			return predicateFromExprWithLocals(e.Args[0], consts, locals)
		}
	case *ast.BinaryExpr:
		return predicateFromExprWithLocals(e.X, consts, locals)
	case *ast.CompositeLit:
		var out []string
		for _, el := range e.Elts {
			out = append(out, predicateFromExprWithLocals(el, consts, locals)...)
		}
		return uniquePredicates(out)
	}
	return nil
}

func uniquePredicates(predicates []string) []string {
	seen := make(map[string]struct{}, len(predicates))
	out := make([]string, 0, len(predicates))
	for _, predicate := range predicates {
		if _, ok := seen[predicate]; ok {
			continue
		}
		seen[predicate] = struct{}{}
		out = append(out, predicate)
	}
	return out
}

func queryCallName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	default:
		return ""
	}
}

func addQueryCallRoots(call *ast.CallExpr, consts map[string]string, locals map[string][]string, roots map[string]struct{}) {
	if !queryCalls[queryCallName(call)] {
		return
	}
	// The predicate is the first argument that names one; a context, a kernel
	// handle or an action ID comes before it in some helpers.
	for i, arg := range call.Args {
		if i > 2 {
			break
		}
		if predicates := predicateFromExprWithLocals(arg, consts, locals); len(predicates) > 0 {
			for _, predicate := range predicates {
				roots[predicate] = struct{}{}
			}
			return
		}
	}
}

func cloneLocalPredicates(locals map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(locals))
	for name, predicates := range locals {
		cloned[name] = append([]string(nil), predicates...)
	}
	return cloned
}

// queryRootsInExpr recognizes only query-shaped calls in an expression. A
// nested function literal is scanned as its own function, so its assignments
// cannot leak into the enclosing function's local environment.
func queryRootsInExpr(expr ast.Node, consts map[string]string, locals map[string][]string, roots map[string]struct{}) {
	ast.Inspect(expr, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			addQueryCallRoots(call, consts, locals, roots)
		}
		return true
	})
}

func assignLocalPredicates(lhs ast.Expr, rhs ast.Expr, consts map[string]string, locals map[string][]string) {
	id, ok := lhs.(*ast.Ident)
	if !ok || id.Name == "_" {
		return
	}
	// An unknown assignment invalidates an older statically known query value.
	// This avoids carrying a stale root through a reassignment.
	var predicates []string
	if rhs != nil {
		predicates = predicateFromExprWithLocals(rhs, consts, locals)
	}
	locals[id.Name] = predicates
}

func queryRootsInBlock(block *ast.BlockStmt, consts map[string]string, locals map[string][]string, roots map[string]struct{}) {
	for _, stmt := range block.List {
		queryRootsInStmt(stmt, consts, locals, roots)
	}
}

func queryRootsInStmt(stmt ast.Stmt, consts map[string]string, locals map[string][]string, roots map[string]struct{}) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		for _, rhs := range s.Rhs {
			queryRootsInExpr(rhs, consts, locals, roots)
		}
		for i, lhs := range s.Lhs {
			var rhs ast.Expr
			if len(s.Rhs) == len(s.Lhs) {
				rhs = s.Rhs[i]
			} else if len(s.Rhs) == 1 && len(s.Lhs) == 1 {
				rhs = s.Rhs[0]
			}
			assignLocalPredicates(lhs, rhs, consts, locals)
		}
	case *ast.DeclStmt:
		decl, ok := s.Decl.(*ast.GenDecl)
		if !ok {
			return
		}
		for _, spec := range decl.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range values.Values {
				queryRootsInExpr(value, consts, locals, roots)
			}
			for i, name := range values.Names {
				var rhs ast.Expr
				if len(values.Values) == len(values.Names) {
					rhs = values.Values[i]
				}
				assignLocalPredicates(name, rhs, consts, locals)
			}
		}
	case *ast.ExprStmt:
		queryRootsInExpr(s.X, consts, locals, roots)
	case *ast.ReturnStmt:
		for _, result := range s.Results {
			queryRootsInExpr(result, consts, locals, roots)
		}
	case *ast.DeferStmt:
		queryRootsInExpr(s.Call, consts, locals, roots)
	case *ast.GoStmt:
		queryRootsInExpr(s.Call, consts, locals, roots)
	case *ast.SendStmt:
		queryRootsInExpr(s.Value, consts, locals, roots)
	case *ast.IncDecStmt:
		queryRootsInExpr(s.X, consts, locals, roots)
	case *ast.BlockStmt:
		queryRootsInBlock(s, consts, cloneLocalPredicates(locals), roots)
	case *ast.IfStmt:
		scope := cloneLocalPredicates(locals)
		if s.Init != nil {
			queryRootsInStmt(s.Init, consts, scope, roots)
		}
		queryRootsInExpr(s.Cond, consts, scope, roots)
		queryRootsInBlock(s.Body, consts, cloneLocalPredicates(scope), roots)
		if s.Else != nil {
			queryRootsInStmt(s.Else, consts, cloneLocalPredicates(scope), roots)
		}
	case *ast.ForStmt:
		scope := cloneLocalPredicates(locals)
		if s.Init != nil {
			queryRootsInStmt(s.Init, consts, scope, roots)
		}
		if s.Cond != nil {
			queryRootsInExpr(s.Cond, consts, scope, roots)
		}
		queryRootsInBlock(s.Body, consts, cloneLocalPredicates(scope), roots)
		if s.Post != nil {
			queryRootsInStmt(s.Post, consts, scope, roots)
		}
	case *ast.RangeStmt:
		queryRootsInExpr(s.X, consts, locals, roots)
		scope := cloneLocalPredicates(locals)
		if id, ok := s.Key.(*ast.Ident); ok {
			scope[id.Name] = nil
		}
		if id, ok := s.Value.(*ast.Ident); ok {
			scope[id.Name] = nil
		}
		queryRootsInBlock(s.Body, consts, scope, roots)
	case *ast.SwitchStmt:
		scope := cloneLocalPredicates(locals)
		if s.Init != nil {
			queryRootsInStmt(s.Init, consts, scope, roots)
		}
		if s.Tag != nil {
			queryRootsInExpr(s.Tag, consts, scope, roots)
		}
		for _, item := range s.Body.List {
			clause := item.(*ast.CaseClause)
			clauseScope := cloneLocalPredicates(scope)
			for _, expr := range clause.List {
				queryRootsInExpr(expr, consts, clauseScope, roots)
			}
			for _, nested := range clause.Body {
				queryRootsInStmt(nested, consts, clauseScope, roots)
			}
		}
	case *ast.TypeSwitchStmt:
		scope := cloneLocalPredicates(locals)
		if s.Init != nil {
			queryRootsInStmt(s.Init, consts, scope, roots)
		}
		queryRootsInStmt(s.Assign, consts, scope, roots)
		for _, item := range s.Body.List {
			clause := item.(*ast.CaseClause)
			clauseScope := cloneLocalPredicates(scope)
			for _, nested := range clause.Body {
				queryRootsInStmt(nested, consts, clauseScope, roots)
			}
		}
	case *ast.SelectStmt:
		for _, item := range s.Body.List {
			clause := item.(*ast.CommClause)
			clauseScope := cloneLocalPredicates(locals)
			if clause.Comm != nil {
				queryRootsInStmt(clause.Comm, consts, clauseScope, roots)
			}
			for _, nested := range clause.Body {
				queryRootsInStmt(nested, consts, clauseScope, roots)
			}
		}
	case *ast.LabeledStmt:
		queryRootsInStmt(s.Stmt, consts, locals, roots)
	case *ast.BranchStmt:
		// No query expression.
	default:
		// Preserve query detection for statement forms not requiring local
		// environment updates, while avoiding traversal into nested scopes.
		queryRootsInExpr(stmt, consts, locals, roots)
	}
}

func queryRootsInFunction(fn *ast.FuncDecl, consts map[string]string) map[string]struct{} {
	roots := make(map[string]struct{})
	if fn == nil || fn.Body == nil {
		return roots
	}
	queryRootsInBlock(fn.Body, consts, make(map[string][]string), roots)
	// Function literals have independent local environments. Scan them too so
	// direct literal/const query arguments inside callbacks remain visible.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		literal, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		queryRootsInBlock(literal.Body, consts, make(map[string][]string), roots)
		return true
	})
	return roots
}

func queryRootsInFile(file *ast.File, consts map[string]string) map[string]struct{} {
	roots := make(map[string]struct{})
	if file == nil {
		return roots
	}
	// Package-level initializers retain the previous direct-expression scan.
	// Function bodies are handled separately with local assignment tracking.
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok == token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range values.Values {
				queryRootsInExpr(value, consts, nil, roots)
			}
		}
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			for predicate := range queryRootsInFunction(fn, consts) {
				roots[predicate] = struct{}{}
			}
		}
	}
	return roots
}

func finiteWrapperRoots(files []*ast.File, wrapper finiteQueryWrapper) map[string]struct{} {
	roots := make(map[string]struct{})
	var wrapperDecl *ast.FuncDecl
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == wrapper.function {
				wrapperDecl = fn
			}
		}
	}
	if wrapperDecl == nil || wrapperDecl.Body == nil {
		return roots
	}
	called := false
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && queryCallName(call) == wrapper.function &&
				(call.Pos() < wrapperDecl.Body.Pos() || call.Pos() > wrapperDecl.Body.End()) {
				called = true
			}
			return true
		})
	}
	if !called {
		return roots
	}
	queriesKernel := false
	ast.Inspect(wrapperDecl.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && queryCalls[queryCallName(call)] {
			queriesKernel = true
			return false
		}
		return true
	})
	if !queriesKernel {
		return roots
	}
	for _, file := range files {
		for _, topLevelDecl := range file.Decls {
			decl, ok := topLevelDecl.(*ast.GenDecl)
			if !ok || decl.Tok != token.VAR {
				continue
			}
			for _, spec := range decl.Specs {
				values, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				containsMap := false
				for _, name := range values.Names {
					if name.Name == wrapper.mapName {
						containsMap = true
					}
				}
				if !containsMap {
					continue
				}
				for _, value := range values.Values {
					literal, ok := value.(*ast.CompositeLit)
					if !ok {
						continue
					}
					for _, elt := range literal.Elts {
						pair, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key, ok := pair.Key.(*ast.BasicLit)
						if !ok || key.Kind != token.STRING {
							continue
						}
						predicate := predicateFromExpr(key, nil)
						for _, name := range predicate {
							roots[name] = struct{}{}
						}
					}
				}
			}
		}
	}
	return roots
}

// goQueryRoots returns every predicate production Go asks the kernel for.
func goQueryRoots(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	roots := make(map[string]struct{})
	// Files grouped by directory, so a const declared in one file of a package
	// resolves in another.
	byDir := make(map[string][]*ast.File)
	fset := token.NewFileSet()
	for _, sub := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isProductionGo(path) {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], f)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
	for _, files := range byDir {
		consts := make(map[string]string)
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				gd, ok := n.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					return true
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								if s, err := strconv.Unquote(lit.Value); err == nil {
									consts[name.Name] = s
								}
							}
						}
					}
				}
				return true
			})
		}
		for _, f := range files {
			for predicate := range queryRootsInFile(f, consts) {
				roots[predicate] = struct{}{}
			}
		}
		packageFile := fset.Position(files[0].Pos()).Filename
		relativeDir, relErr := filepath.Rel(root, filepath.Dir(packageFile))
		if relErr == nil {
			for path, wrapper := range finiteQueryWrappers {
				if filepath.Dir(filepath.FromSlash(path)) != filepath.Clean(relativeDir) {
					continue
				}
				for predicate := range finiteWrapperRoots(files, wrapper) {
					roots[predicate] = struct{}{}
				}
			}
		}
	}
	return roots
}

// liveConclusions closes the roots over the rule graph: a rule whose head is
// live makes every predicate its body reads live.
func liveConclusions(rules []mgRule, roots map[string]struct{}) map[string]struct{} {
	live := make(map[string]struct{}, len(roots))
	for p := range roots {
		live[p] = struct{}{}
	}
	for changed := true; changed; {
		changed = false
		for _, r := range rules {
			if _, ok := live[r.head]; !ok {
				continue
			}
			for _, b := range r.body {
				if _, ok := live[b]; !ok {
					live[b] = struct{}{}
					changed = true
				}
			}
		}
	}
	return live
}

var nextActionConstRe = regexp.MustCompile(`^next_action\(\s*/([a-z_][a-z0-9_]*)\s*\)$`)

// routedActions returns the ActionType values executeAction dispatches: the
// consts named by the cases of its switch, resolved to their string values.
func routedActions(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	coreDir := filepath.Join(root, "internal", "core")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, coreDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", coreDir, err)
	}
	values := make(map[string]string)
	var execute *ast.FuncDecl
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					if d.Tok != token.CONST {
						continue
					}
					for _, spec := range d.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for i, name := range vs.Names {
							if i < len(vs.Values) {
								if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									if s, err := strconv.Unquote(lit.Value); err == nil {
										values[name.Name] = s
									}
								}
							}
						}
					}
				case *ast.FuncDecl:
					if d.Name.Name == "executeAction" && d.Recv != nil {
						execute = d
					}
				}
			}
		}
	}
	if execute == nil {
		t.Fatal("VirtualStore.executeAction not found; the routing measurement has nothing to read")
	}
	routed := make(map[string]struct{})
	ast.Inspect(execute.Body, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, e := range cc.List {
			if id, ok := e.(*ast.Ident); ok {
				if v, ok := values[id.Name]; ok {
					routed[strings.TrimPrefix(v, "/")] = struct{}{}
				}
			}
		}
		return true
	})
	if len(routed) == 0 {
		t.Fatal("executeAction routes no ActionType; the routing measurement has nothing to read")
	}
	return routed
}

// currentUnconsumedConclusions computes the inventory: "unconsumed <pred>" for
// each rule head nothing reads, "unrouted /<action>" for each constant
// next_action nothing executes.
func currentUnconsumedConclusions(t *testing.T) []string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, cwd)
	rules := mangleRules(t, filepath.Join(root, "internal", "core", "defaults"))
	if len(rules) == 0 {
		t.Fatal("no rule found in the corpus")
	}
	roots := goQueryRoots(t, root)
	if len(roots) == 0 {
		t.Fatal("no query root found in production Go; the liveness measurement has nothing to start from")
	}
	// A predicate an inclusion constraint names is read by the kernel's
	// inclusion checker at the batch boundary (core/kernel_inclusion.go).
	for p := range inclusionRoots(t, filepath.Join(root, "internal", "core", "defaults")) {
		roots[p] = struct{}{}
	}
	for p := range mergeRoots(t, filepath.Join(root, "internal", "core", "defaults")) {
		roots[p] = struct{}{}
	}
	live := liveConclusions(rules, roots)
	routed := routedActions(t, root)

	seen := make(map[string]struct{})
	for _, r := range rules {
		if _, ok := live[r.head]; !ok {
			seen["unconsumed "+r.head] = struct{}{}
		}
		if r.head == "next_action" {
			if m := nextActionConstRe.FindStringSubmatch(r.headText); m != nil {
				if _, ok := routed[m[1]]; !ok {
					seen["unrouted /"+m[1]] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for line := range seen {
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

// TestExecutiveConclusionsAreConsumed fails when a rule starts deriving a
// conclusion nothing reads, or a next_action nothing routes, and when an entry
// on the list is wired without being removed from it.
func TestExecutiveConclusionsAreConsumed(t *testing.T) {
	current := currentUnconsumedConclusions(t)

	if os.Getenv("CODENERD_UPDATE_CONCLUSIONS") == "1" {
		writeConclusionsBaseline(t, current)
		t.Logf("rewrote %s with %d entr(ies)", conclusionsBaselinePath, len(current))
		return
	}

	data, err := os.ReadFile(conclusionsBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed",
			conclusionsBaselinePath, err)
	}
	baseline := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseline[line] = struct{}{}
	}
	currentSet := make(map[string]struct{}, len(current))
	var added []string
	for _, line := range current {
		currentSet[line] = struct{}{}
		if _, known := baseline[line]; !known {
			added = append(added, line)
		}
	}
	var wired []string
	for line := range baseline {
		if _, still := currentSet[line]; !still {
			wired = append(wired, line)
		}
	}
	sort.Strings(wired)

	if len(added) > 0 {
		t.Errorf(`%d new unconsumed conclusion(s): %v

"unconsumed p": a rule derives p and nothing reads it -- no production Go
query names it, and no rule whose head is read has it in its body. Either the
executive should act on it (query it where the decision is made, and delete
the Go that decides the same thing), or the rule should go.
"unrouted /x": a rule derives next_action(/x) and VirtualStore.executeAction
has no case for it.

A new query helper that wraps Query belongs in queryCalls.
Baseline: %s`, len(added), added, conclusionsBaselinePath)
	}
	if len(wired) > 0 {
		t.Errorf(`%d entr(ies) on the list are consumed now: %v

Good. Refresh the baseline so the count stays a measurement:
CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed

Baseline: %s`, len(wired), wired, conclusionsBaselinePath)
	}
}

func writeConclusionsBaseline(t *testing.T, lines []string) {
	t.Helper()
	const header = `# Unconsumed executive conclusions -- derived by the kernel, acted on by nothing.
#
# Regenerate: CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed
# See executive_conclusions_test.go. "unconsumed p": no production query reads p,
# directly or through a rule whose head is read. "unrouted /x": next_action(/x)
# is derived and VirtualStore.executeAction has no case for it.
#
# The list may only shrink. Each line is either a decision Go makes itself
# while the kernel's answer goes unread, or a rule nobody needs.

`
	body := header + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(conclusionsBaselinePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", conclusionsBaselinePath, err)
	}
}
