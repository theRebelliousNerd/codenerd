package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// assertTurnElements records which code elements this turn changed, as
// turn_changed_element facts beside the turn_written facts the closure asserts:
// for each written .go file, the functions and methods whose bodies or
// signatures the turn changed or added, each as the code_element ref a future
// witness rule will join against ("you changed fn:session.gateTests, so a
// passing test that executes it is owed"). No policy reads the predicate yet;
// it is measurement waiting for its consumer.
//
// A file the turn deleted contributes nothing: a deleted function is not an
// element to witness. A file whose preimage is unknown or missing is skipped
// rather than read as empty, the way the pin gate treats it.
func (e *Executor) assertTurnElements(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	workspace := e.workspaceForVerification()
	for _, path := range result.WrittenPaths {
		// pinUnits skips the same paths: a _test.go change is a test of a test,
		// and a file under testdata or a directory named with a leading "_" or
		// "." is one the go tool never builds.
		if !strings.HasSuffix(strings.ToLower(path), ".go") || isTestPath(path) || ignoredByGoTool(path) {
			continue
		}
		pre, ok := preImageFor(workspace, path, result.PreWriteContents)
		if !ok || !pre.Known() {
			continue
		}
		data, err := os.ReadFile(turnFilePath(workspace, path))
		if err != nil {
			continue
		}
		for _, ref := range changedElementRefs(string(data), pre, path) {
			e.assertTurnFact(types.Fact{
				Predicate: "turn_changed_element",
				Args:      []any{turn, types.MangleString(ref)},
			})
		}
	}
}

// changedElementRefs is the turn's changed elements in one file: the names the
// pin gate's changed-function detection reports (fileUnits in pin_gate.go --
// changed or added, token-compared so a reformatted function is the same
// function, deleted ones excluded because nothing witnesses them), spelled as
// code_element refs. A file whose only changes are outside its functions (a
// constant, a type) reports the file, not an element, and contributes nothing
// here: only functions and methods have refs to witness.
//
// A changed init asserts nothing. funcDecls leaves every init out of those
// names: one file may declare several, and the name does not say which one
// changed, so there is no single ref to assert. fn:<pkg>.init would name all
// of them.
func changedElementRefs(cur string, pre PreImage, path string) []string {
	var names []string
	for _, u := range fileUnits(path, cur, pre) {
		if u.name == "" {
			continue
		}
		names = append(names, u.name)
	}
	if len(names) == 0 {
		return nil
	}
	return elementRefs(cur, names)
}

// elementRefs spells pin-gate function keys ("Name", "Recv.Name") as the refs
// world.GoCodeParser.buildRef emits for the same declarations
// (internal/world/go_parser.go): fn:<pkg>.<Name>, fn:<pkg>.<Recv>.<Name> for a
// method, where <pkg> is the file's package clause. Byte-identical on purpose:
// a future witness rule joins turn_changed_element against code_element, and a
// near-match joins nothing (internal/mangle/agents.md: a Decl is a contract).
func elementRefs(cur string, names []string) []string {
	byKey := elementFuncs(cur)
	if len(byKey) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(names))
	var out []string
	for _, name := range names {
		fn, ok := byKey[name]
		if !ok || seen[fn.ref] {
			continue
		}
		seen[fn.ref] = true
		out = append(out, fn.ref)
	}
	sort.Strings(out)
	return out
}

// elementFunc is one top-level function or method: the code_element ref and
// the declaration's line span. One parse produces both so the coverage
// mapping joins a profile block to the same element turn_changed_element
// names, not to a span a second parser might draw differently.
type elementFunc struct {
	ref  string
	span LineRange
}

func elementFuncs(src string) map[string]elementFunc {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	byKey := make(map[string]elementFunc)
	for _, d := range f.Decls {
		fn, isFunc := d.(*ast.FuncDecl)
		if !isFunc {
			continue
		}
		key := funcKey(fn)
		// A second declaration with this key is the same ref: valid Go has one
		// function or method per key, and an illegal redeclaration (or a
		// receiver the spec rejects, which funcKey spells as the bare name)
		// does not name a different element.
		if _, dup := byKey[key]; dup {
			continue
		}
		byKey[key] = elementFunc{
			ref: elementRef(f.Name.Name, fn),
			span: LineRange{
				Start: fset.Position(fn.Pos()).Line,
				End:   fset.Position(fn.End()).Line,
			},
		}
	}
	return byKey
}

// elementSpans is elementFuncs keyed by ref, for the coverage mapping.
func elementSpans(src string) map[string]LineRange {
	byKey := elementFuncs(src)
	if len(byKey) == 0 {
		return nil
	}
	out := make(map[string]LineRange, len(byKey))
	for _, fn := range byKey {
		if _, ok := out[fn.ref]; ok {
			continue
		}
		out[fn.ref] = fn.span
	}
	return out
}

// elementRef is one declaration's code_element ref: the package clause, the
// name, and the receiver between them when the world's parser keeps one.
func elementRef(pkg string, fn *ast.FuncDecl) string {
	if recv := elementReceiver(fn); recv != "" {
		return "fn:" + pkg + "." + recv + "." + fn.Name.Name
	}
	return "fn:" + pkg + "." + fn.Name.Name
}

// elementReceiver is the receiver base type the world's Go parser carries in
// the ref (extractReceiverTypeInfo in internal/world/go_parser.go). Stars,
// type arguments and parentheses unwrap, the same shapes funcKey drops, so
// func (b *Box[T]) Get, func (b Box[T]) Get and func (b (Box[T])) Get all
// name Box. The ref is fn:<pkg>.Box.Get, not fn:<pkg>.Get: a plain func Get
// is a different element. A receiver that is not a named type after that
// unwrap is dropped, and the ref is then the bare function, matching funcKey.
func elementReceiver(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
			continue
		case *ast.IndexExpr:
			expr = t.X
			continue
		case *ast.IndexListExpr:
			expr = t.X
			continue
		case *ast.ParenExpr:
			expr = t.X
			continue
		}
		break
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
