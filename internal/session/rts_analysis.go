package session

import (
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Static regression test selection, part 2: which tests can reach a changed
// function.
//
// The importer packages are loaded with their tests, built to SSA, and given
// a call graph. A test is selected when a function the turn changed is
// reachable from it. The question is answered backwards: one walk per importer
// over caller edges from the changed functions marks every function that can
// reach them, and a test is selected if its own function is marked.
//
// The graph is class hierarchy analysis (go/callgraph/cha) refined by variable
// type analysis (go/callgraph/vta). CHA alone sends a call through an
// interface to every method of that name and a call through a function value
// to every function of that signature; on this repository that welded every
// test to every function (the first measurement selected 7722 of 7722 tests).
// VTA keeps CHA's edges only where the receiver's type or the function literal
// can actually flow, and is sound modulo reflection and unsafe if CHA is. It
// assumes nothing about values that come from code with no body here, so two
// things are added back for such code (a library: t.Run, sort.Slice, fmt
// calling String, json calling MarshalJSON):
//
//   - A function value passed, directly or inside a struct, map, slice or
//     closure built in the function, to a call VTA cannot see through is an
//     edge from that function to the value.
//   - A concrete type converted to an interface on its way to such a call is an
//     edge to every method of the type and of every type it holds, the set
//     reflection could reach. The type arguments of a bodiless generic function
//     whose constraint names methods are treated the same way.
//
// What the graph cannot see is run, not guessed at: package initialization that
// reaches a change selects every test of the binary, a TestMain that reaches it
// runs the package whole, and a test that runs another program, reads the
// repository's source or calls through reflect is always selected. What is
// still unsound, and not covered, is a function value or interface held in a
// global or field and handed to a library by a function other than the one
// that made it.

// testSelectionTimeoutKey carries session.test_selection_timeout, which
// bounds loading, SSA construction and the walks. A package graph past it
// is not a measurement: the gate runs every importer test.
type testSelectionTimeoutKey struct{}

// withTestSelectionTimeout attaches the bound; non-positive takes the
// session section's default.
func withTestSelectionTimeout(ctx context.Context, d time.Duration) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if d <= 0 {
		d = defaultSessionPolicy.TestSelectionTimeout
	}
	return context.WithValue(ctx, testSelectionTimeoutKey{}, d)
}

// testSelectionTimeout is the bound in force on ctx, else the default.
func testSelectionTimeout(ctx context.Context) time.Duration {
	if ctx != nil {
		if d, ok := ctx.Value(testSelectionTimeoutKey{}).(time.Duration); ok && d > 0 {
			return d
		}
	}
	return defaultSessionPolicy.TestSelectionTimeout
}

// importerTests is what the analysis decided for one importer package.
type importerTests struct {
	// Path is the import path.
	Path string
	// All is every test the package defines (Test*, Fuzz*, Example*; not
	// TestMain), sorted.
	All []string
	// Selected is the subset that can reach a change, sorted.
	Selected []string
	// Whole is true when the package must run without a -run filter:
	// TestMain, which every test runs under, can reach a change.
	Whole bool
	// Why names the reason when Whole or every test was selected without the
	// tests having reached the change one by one ("package initialization
	// reaches the change", "TestMain reaches the change").
	Why string
}

// rtsAnalysis is the analysis over the importers.
type rtsAnalysis struct {
	Packages []importerTests
	Elapsed  time.Duration
	// Phases says where the time went: loading, SSA, the call graph, the walks.
	Phases string
}

// escapeCalls are functions whose call means a test can leave the call graph:
// run another program (so the code that runs is not this binary's), or read
// the source of the program (so the test observes text the graph cannot see).
var escapeCalls = map[string]bool{
	"os/exec.Command":                     true,
	"os/exec.CommandContext":              true,
	"(*os/exec.Cmd).Run":                  true,
	"(*os/exec.Cmd).Start":                true,
	"os.StartProcess":                     true,
	"syscall.Exec":                        true,
	"syscall.ForkExec":                    true,
	"go/parser.ParseFile":                 true,
	"go/parser.ParseDir":                  true,
	"go/build.Import":                     true,
	"go/build.ImportDir":                  true,
	"(*go/build.Context).Import":          true,
	"(*go/build.Context).ImportDir":       true,
	"plugin.Open":                         true,
	"golang.org/x/tools/go/packages.Load": true,
}

// testOnlyEscapes leave the call graph only when a test calls them: runtime
// callers are how a test finds the source tree it sits in, but production
// code calls them for logging everywhere.
var testOnlyEscapes = map[string]bool{
	"runtime.Caller": true,
}

// analyzeImporterTests decides, for each importer, which tests reach a
// function in changes. loadSet are the module packages whose source the
// analysis needs (the importers, the changed packages and what lies between).
// why is non-empty when no sound answer was reached; the caller runs every
// importer test then.
func analyzeImporterTests(ctx context.Context, workspace string, env []string, loadSet, importers []string, changes bodyChanges) (rtsAnalysis, string) {
	start := time.Now()
	budget := testSelectionTimeout(ctx)
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	type outcome struct {
		a   rtsAnalysis
		why string
	}
	done := make(chan outcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- outcome{why: fmt.Sprintf("the analysis panicked (%v)", r)}
			}
		}()
		a, why := analyzeLoaded(ctx, workspace, env, loadSet, importers, changes)
		done <- outcome{a, why}
	}()
	select {
	case o := <-done:
		o.a.Elapsed = time.Since(start)
		return o.a, o.why
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return rtsAnalysis{Elapsed: time.Since(start)}, "the analysis exceeded its " + budget.String() + " budget (session.test_selection_timeout)"
		}
		return rtsAnalysis{Elapsed: time.Since(start)}, "the analysis was canceled"
	}
}

func analyzeLoaded(ctx context.Context, workspace string, env []string, loadSet, importers []string, changes bodyChanges) (rtsAnalysis, string) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     workspace,
		Env:     env,
		Tests:   true,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedForTest,
	}
	t0 := time.Now()
	pkgs, err := packages.Load(cfg, loadSet...)
	loadTime := time.Since(t0)
	if err != nil {
		return rtsAnalysis{}, "the packages did not load (" + err.Error() + ")"
	}
	variants := map[string][]*packages.Package{} // importer path -> its test packages
	loaded := map[string]bool{}
	var roots []*packages.Package
	for _, p := range pkgs {
		loaded[p.PkgPath] = true
		if strings.HasSuffix(p.PkgPath, ".test") && p.Name == "main" {
			// The generated test main names every test as a value, which would
			// make each of them look reachable from every function value call.
			continue
		}
		if p.ForTest != "" && (p.PkgPath == p.ForTest || p.PkgPath == p.ForTest+"_test") {
			variants[p.ForTest] = append(variants[p.ForTest], p)
		}
		if p.Types != nil && p.TypesInfo != nil && len(p.Syntax) > 0 {
			roots = append(roots, p)
		}
	}
	for _, path := range importers {
		if !loaded[path] {
			return rtsAnalysis{}, "importer " + path + " was not loaded"
		}
	}
	var bad string
	packages.Visit(flatten(variants, importers), nil, func(p *packages.Package) {
		if bad == "" && len(p.Errors) > 0 {
			bad = p.PkgPath + " does not load (" + p.Errors[0].Error() + ")"
		}
	})
	if bad != "" {
		return rtsAnalysis{}, bad
	}
	if err := ctx.Err(); err != nil {
		return rtsAnalysis{}, err.Error()
	}

	t0 = time.Now()
	prog, ssaPkgs := ssautil.Packages(roots, ssa.InstantiateGenerics)
	prog.Build()
	built := time.Since(t0)
	ssaOf := make(map[*packages.Package]*ssa.Package, len(roots))
	for i, p := range roots {
		if ssaPkgs[i] != nil {
			ssaOf[p] = ssaPkgs[i]
		}
	}
	if err := ctx.Err(); err != nil {
		return rtsAnalysis{}, err.Error()
	}

	t0 = time.Now()
	g := newRTSGraph(prog)
	graphed := time.Since(t0)
	t0 = time.Now()
	seeds, why := g.changedFunctions(roots, changes)
	if why != "" {
		return rtsAnalysis{}, why
	}

	var out rtsAnalysis
	for _, path := range importers {
		it := importerTests{Path: path}
		// Only what the importer's test binary links can run in it: a
		// function value of a matching signature in a package it does not
		// import is no way for a test to reach anything.
		linked := map[*types.Package]bool{}
		for _, v := range variants[path] {
			g.link(v.Types, linked)
		}
		reaches := g.callersOf(seeds, linked)
		opaque := g.callersOf(g.escapes, linked)
		var initWhy string
		var whole bool
		type testFn struct {
			name string
			fn   *ssa.Function
		}
		var tests []testFn
		for _, v := range variants[path] {
			sp := ssaOf[v]
			if sp == nil {
				return rtsAnalysis{}, "the tests of " + path + " have no SSA form"
			}
			for _, init := range g.initsLinkedBy(v.Types) {
				if reaches.has(init) && initWhy == "" {
					initWhy = reaches.chain(init)
				}
			}
			for i, f := range v.Syntax {
				if !strings.HasSuffix(v.CompiledGoFiles[i], "_test.go") {
					continue
				}
				for _, decl := range f.Decls {
					fd, ok := decl.(*ast.FuncDecl)
					if !ok || fd.Recv != nil {
						continue
					}
					kind := testKind(fd.Name.Name, v.TypesInfo.Defs[fd.Name])
					if kind == "" {
						continue
					}
					fn, _ := sp.Members[fd.Name.Name].(*ssa.Function)
					if fn == nil {
						return rtsAnalysis{}, "test " + path + "." + fd.Name.Name + " has no SSA function"
					}
					if kind == "main" {
						whole = whole || reaches.has(fn) || opaque.has(fn)
						continue
					}
					tests = append(tests, testFn{fd.Name.Name, fn})
				}
			}
		}
		switch {
		case whole:
			it.Whole, it.Why = true, "TestMain reaches the change or leaves the call graph"
		case initWhy != "":
			it.Why = "package initialization reaches the change: " + initWhy
		}
		for _, t := range tests {
			it.All = append(it.All, t.name)
			if it.Whole || initWhy != "" || reaches.has(t.fn) || opaque.has(t.fn) {
				it.Selected = append(it.Selected, t.name)
			}
		}
		slices.Sort(it.All)
		slices.Sort(it.Selected)
		out.Packages = append(out.Packages, it)
	}
	out.Phases = fmt.Sprintf("load %s, ssa %s, call graph %s, walks %s",
		loadTime.Round(time.Millisecond), built.Round(time.Millisecond), graphed.Round(time.Millisecond), time.Since(t0).Round(time.Millisecond))
	return out, ""
}

func flatten(variants map[string][]*packages.Package, importers []string) []*packages.Package {
	var out []*packages.Package
	for _, path := range importers {
		out = append(out, variants[path]...)
	}
	return out
}

// testKind classifies a top-level function of a test file the way `go test`
// does: "test", "fuzz", "example", "main" (TestMain), or "" for anything else.
func testKind(name string, obj types.Object) string {
	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	sig := fn.Type().(*types.Signature)
	param := func(want string) bool {
		return sig.Params().Len() == 1 && sig.Results().Len() == 0 && types.TypeString(sig.Params().At(0).Type(), nil) == want
	}
	named := func(prefix string) bool {
		rest, found := strings.CutPrefix(name, prefix)
		if !found {
			return false
		}
		if rest == "" {
			return true
		}
		r, _ := utf8.DecodeRuneInString(rest)
		return !unicode.IsLower(r)
	}
	switch {
	case name == "TestMain" && param("*testing.M"):
		return "main"
	case named("Test") && param("*testing.T"):
		return "test"
	case named("Fuzz") && param("*testing.F"):
		return "fuzz"
	case named("Example") && sig.Params().Len() == 0 && sig.Results().Len() == 0:
		return "example"
	}
	return ""
}

// rtsGraph is the program's call graph plus the edges it leaves out, over one
// SSA program. The calls are CHA's, refined by variable type analysis
// (go/callgraph/vta): a call through an interface or a function value goes to
// the types and function literals that can flow to it, not to every method or
// function of a matching shape, which on a program this size welds everything
// into one blob. VTA assumes nothing about values that come from code with no
// body here, so what a library does with the program's values is added back
// by scan.
type rtsGraph struct {
	prog  *ssa.Program
	funcs map[*ssa.Function]bool
	vta   *callgraph.Graph

	// refIn maps a function to the functions that hand it to code this graph
	// has no edges through: those that pass it as (part of) a call argument,
	// or convert a type that has it as a method to an interface.
	refIn map[*ssa.Function][]*ssa.Function
	// escapes are the functions that leave the call graph.
	escapes []*ssa.Function

	methods map[*types.Named][]*ssa.Function
}

func newRTSGraph(prog *ssa.Program) *rtsGraph {
	funcs := ssautil.AllFunctions(prog)
	g := &rtsGraph{
		prog:    prog,
		funcs:   funcs,
		vta:     vta.CallGraph(funcs, cha.CallGraph(prog)),
		refIn:   map[*ssa.Function][]*ssa.Function{},
		methods: map[*types.Named][]*ssa.Function{},
	}
	for fn := range funcs {
		if fn != nil && len(fn.Blocks) > 0 {
			g.scan(fn)
		}
	}
	return g
}

// scan records fn's references, interface conversions, function value calls
// and escapes.
func (g *rtsGraph) scan(fn *ssa.Function) {
	refs := map[*ssa.Function]struct{}{}
	inTest := strings.HasSuffix(g.prog.Fset.Position(fn.Pos()).Filename, "_test.go")
	escapes := false
	flowSeen := map[ssa.Value]bool{}
	var ops [16]*ssa.Value
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			var calleeSlot *ssa.Value
			if site, ok := instr.(ssa.CallInstruction); ok {
				c := site.Common()
				calleeSlot = &c.Value
				static := c.StaticCallee()
				switch {
				case static != nil:
					if obj, ok := static.Object().(*types.Func); ok {
						full := obj.FullName()
						if escapeCalls[full] || (inTest && testOnlyEscapes[full]) {
							escapes = true
						}
					}
					if len(static.Blocks) == 0 {
						// A library function with no body here calls the
						// methods its type parameters' constraints name.
						g.constrainedTypeArgs(static, refs)
					}
				}
			}
			for _, op := range instr.Operands(ops[:0]) {
				if op == nil || *op == nil || op == calleeSlot {
					continue
				}
				if v, ok := (*op).(*ssa.Const); ok && inTest && v.Value != nil &&
					v.Value.Kind() == constant.String && hasParentSegment(constant.StringVal(v.Value)) {
					escapes = true
				}
			}
			if site, ok := instr.(ssa.CallInstruction); ok && g.callsUnseenCode(site) {
				// A function value handed to code with no body here (a library,
				// or a call VTA could not resolve) may be called by it. What
				// reaches the call as an argument, directly or inside a struct,
				// map, slice or closure built in this function, is reachable from
				// it. A callee with a body needs no edge: VTA follows the value
				// through its parameters to the call that uses it.
				c := site.Common()
				for _, arg := range c.Args {
					g.collectFlow(arg, flowSeen, refs)
				}
				if c.IsInvoke() {
					g.collectFlow(c.Value, flowSeen, refs)
				}
			}
		}
	}
	for ref := range refs {
		g.refIn[ref] = append(g.refIn[ref], fn)
	}
	if escapes {
		g.escapes = append(g.escapes, fn)
	}
}

// constrainedTypeArgs adds the methods of the type arguments of a bodiless
// generic function whose type parameter has a constraint with methods: those
// are the ones the function can call.
func (g *rtsGraph) constrainedTypeArgs(fn *ssa.Function, refs map[*ssa.Function]struct{}) {
	args, params := fn.TypeArgs(), fn.TypeParams()
	for i, arg := range args {
		if params != nil && params.Len() == len(args) {
			if iface, ok := params.At(i).Constraint().Underlying().(*types.Interface); ok && iface.NumMethods() == 0 {
				continue
			}
		}
		g.convertedToInterface(arg, refs)
	}
}

// callsUnseenCode reports whether site may run code this graph has no body
// for: a static call to a bodiless function, or a call VTA resolved to one or
// to nothing.
func (g *rtsGraph) callsUnseenCode(site ssa.CallInstruction) bool {
	if static := site.Common().StaticCallee(); static != nil {
		return len(static.Blocks) == 0
	}
	if _, builtin := site.Common().Value.(*ssa.Builtin); builtin {
		return false
	}
	resolved := false
	if n := g.vta.Nodes[site.Parent()]; n != nil {
		for _, e := range n.Out {
			if e.Site != site {
				continue
			}
			resolved = true
			if len(e.Callee.Func.Blocks) == 0 {
				return true
			}
		}
	}
	return !resolved
}

// collectFlow adds to refs every function value that can be inside v: v
// itself, the functions a closure binds, what an instruction's operands carry,
// and what has been stored into the memory, map or slice v names. Results of
// other calls are not followed; the callee's own function says what it returns.
func (g *rtsGraph) collectFlow(v ssa.Value, seen map[ssa.Value]bool, refs map[*ssa.Function]struct{}) {
	if v == nil || seen[v] {
		return
	}
	seen[v] = true
	switch v := v.(type) {
	case *ssa.Function:
		refs[v] = struct{}{}
		return
	case *ssa.MakeClosure:
		g.collectFlow(v.Fn, seen, refs)
		for _, b := range v.Bindings {
			g.collectFlow(b, seen, refs)
		}
		return
	case *ssa.Global, *ssa.Parameter, *ssa.FreeVar, *ssa.Const, *ssa.Builtin, *ssa.Call:
		return
	case *ssa.MakeInterface:
		// The value becomes an interface on its way to code this graph has no
		// edges through, which may call any method of it.
		g.convertedToInterface(v.X.Type(), refs)
	}
	if ins, ok := v.(ssa.Instruction); ok {
		var ops [8]*ssa.Value
		for _, op := range ins.Operands(ops[:0]) {
			if op != nil {
				g.collectFlow(*op, seen, refs)
			}
		}
	}
	if referrers := v.Referrers(); referrers != nil {
		for _, r := range *referrers {
			switch r := r.(type) {
			case *ssa.Store:
				if r.Addr == v {
					g.collectFlow(r.Val, seen, refs)
				}
			case *ssa.MapUpdate:
				if r.Map == v {
					g.collectFlow(r.Key, seen, refs)
					g.collectFlow(r.Value, seen, refs)
				}
			case *ssa.FieldAddr, *ssa.IndexAddr, *ssa.Slice:
				g.collectFlow(r.(ssa.Value), seen, refs)
			}
		}
	}
}

// hasParentSegment reports a path that climbs out of the directory a test
// runs in: it names files of the repository, which the graph cannot follow.
func hasParentSegment(s string) bool {
	return slices.Contains(strings.Split(strings.ReplaceAll(s, `\`, "/"), "/"), "..")
}

// convertedToInterface adds, to refs, every method that code outside the
// program's own call edges could reach through a value of type t: the exported
// methods of t and of each type its value holds through exported fields,
// elements and pointers, since a library walks those by reflection.
func (g *rtsGraph) convertedToInterface(t types.Type, refs map[*ssa.Function]struct{}) {
	seen := map[types.Type]bool{}
	var walk func(types.Type)
	walk = func(t types.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch t := t.(type) {
		case *types.Named:
			for _, m := range g.methodsOf(t) {
				refs[m] = struct{}{}
			}
			walk(t.Underlying())
			for i := range t.TypeArgs().Len() {
				walk(t.TypeArgs().At(i))
			}
		case *types.Alias:
			walk(types.Unalias(t))
		case *types.Pointer:
			walk(t.Elem())
		case *types.Slice:
			walk(t.Elem())
		case *types.Array:
			walk(t.Elem())
		case *types.Chan:
			walk(t.Elem())
		case *types.Map:
			walk(t.Key())
			walk(t.Elem())
		case *types.Struct:
			for i := range t.NumFields() {
				// Reflection cannot hand out a value held in an unexported
				// field as an interface, so nothing can call its methods.
				if t.Field(i).Exported() {
					walk(t.Field(i).Type())
				}
			}
		}
	}
	walk(t)
}

// methodsOf is the SSA functions of the methods of N and *N.
func (g *rtsGraph) methodsOf(n *types.Named) []*ssa.Function {
	if fns, ok := g.methods[n]; ok {
		return fns
	}
	var fns []*ssa.Function
	if _, isIface := n.Underlying().(*types.Interface); !isIface {
		for _, t := range []types.Type{n, types.NewPointer(n)} {
			ms := g.prog.MethodSets.MethodSet(t)
			for i := range ms.Len() {
				obj, ok := ms.At(i).Obj().(*types.Func)
				if !ok || !obj.Exported() {
					// A library names the methods of its own interfaces
					// (String, Less, MarshalJSON, ...), all exported.
					continue
				}
				if fn := g.prog.FuncValue(obj.Origin()); fn != nil {
					fns = append(fns, fn)
				}
			}
		}
	}
	g.methods[n] = fns
	return fns
}

// changedFunctions resolves the turn's changed declarations to SSA functions,
// with the closures declared inside them and the instances of generic ones.
func (g *rtsGraph) changedFunctions(roots []*packages.Package, changes bodyChanges) ([]*ssa.Function, string) {
	want := map[string]map[declKey]bool{}
	found := map[string]map[declKey]bool{}
	total := 0
	for file, keys := range changes.Funcs {
		want[file], found[file] = map[declKey]bool{}, map[declKey]bool{}
		for _, k := range keys {
			want[file][k] = true
			total++
		}
	}
	declared := map[*ssa.Function]bool{}
	for _, p := range roots {
		for _, f := range p.Syntax {
			file := resolveSymlinks(p.Fset.Position(f.Pos()).Filename)
			keys := want[file]
			if keys == nil {
				continue
			}
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				key := declKey{Recv: receiverTypeName(fd), Name: fd.Name.Name}
				if !keys[key] {
					continue
				}
				obj, ok := p.TypesInfo.Defs[fd.Name].(*types.Func)
				if !ok {
					return nil, "no type information for " + key.String()
				}
				fn := g.prog.FuncValue(obj.Origin())
				if fn == nil {
					return nil, "no SSA function for " + key.String()
				}
				declared[fn] = true
				found[file][key] = true
			}
		}
	}
	for file, keys := range want {
		for k := range keys {
			if !found[file][k] {
				return nil, "the changed function " + k.String() + " (" + file + ") is not in the loaded packages"
			}
		}
	}
	var seeds []*ssa.Function
	for fn := range g.funcs {
		for p := fn; p != nil; p = p.Parent() {
			if declared[p] || (p.Origin() != nil && declared[p.Origin()]) {
				seeds = append(seeds, fn)
				break
			}
		}
	}
	if len(seeds) == 0 && total > 0 {
		return nil, "the changed functions have no nodes in the call graph"
	}
	return seeds, ""
}

// reachSet is a set of functions, each with the function it leads on to: the
// next step of a path to a seed (nil for a seed). Membership is "can reach a
// seed"; the links are what lets a decision say how.
type reachSet map[*ssa.Function]*ssa.Function

func (r reachSet) has(f *ssa.Function) bool {
	_, ok := r[f]
	return ok
}

// chain renders the path from f to a seed, caller first.
func (r reachSet) chain(f *ssa.Function) string {
	var steps []string
	for seen := 0; f != nil && seen <= len(r); seen++ {
		steps = append(steps, f.String())
		f = r[f]
	}
	return strings.Join(steps, " -> ")
}

// callersOf is every function from which one of seeds can be reached: the
// seeds, their callers through calls, interface dispatch and function values,
// and the functions that hand them to code with no body here.
func (g *rtsGraph) callersOf(seeds []*ssa.Function, linked map[*types.Package]bool) reachSet {
	seen := make(reachSet, len(seeds))
	queue := make([]*ssa.Function, 0, len(seeds))
	push := func(f, next *ssa.Function) {
		if f == nil || !runsIn(f, linked) {
			return
		}
		if _, done := seen[f]; !done {
			seen[f] = next
			queue = append(queue, f)
		}
	}
	for _, s := range seeds {
		push(s, nil)
	}
	for len(queue) > 0 {
		f := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if n := g.vta.Nodes[f]; n != nil {
			for _, e := range n.In {
				push(e.Caller.Func, f)
			}
		}
		for _, c := range g.refIn[f] {
			push(c, f)
		}
	}
	return seen
}

// runsIn reports whether f can run in a binary that links the given packages.
// A function whose package cannot be told (a synthetic wrapper) might.
func runsIn(f *ssa.Function, linked map[*types.Package]bool) bool {
	for p := f; p != nil; p = p.Parent() {
		if p.Pkg != nil {
			return linked[p.Pkg.Pkg]
		}
	}
	if obj := f.Object(); obj != nil && obj.Pkg() != nil {
		return linked[obj.Pkg()]
	}
	return true
}

// link adds tp and every package it imports to linked.
func (g *rtsGraph) link(tp *types.Package, linked map[*types.Package]bool) {
	if tp == nil || linked[tp] {
		return
	}
	linked[tp] = true
	for _, imp := range tp.Imports() {
		g.link(imp, linked)
	}
}

// initsLinkedBy is the package initializers of tp and everything it imports,
// which run before any test of a binary that links it.
func (g *rtsGraph) initsLinkedBy(tp *types.Package) []*ssa.Function {
	var out []*ssa.Function
	seen := map[*types.Package]bool{}
	var walk func(*types.Package)
	walk = func(p *types.Package) {
		if p == nil || seen[p] {
			return
		}
		seen[p] = true
		if sp := g.prog.Package(p); sp != nil {
			if init := sp.Func("init"); init != nil {
				out = append(out, init)
			}
		}
		for _, imp := range p.Imports() {
			walk(imp)
		}
	}
	walk(tp)
	return out
}
