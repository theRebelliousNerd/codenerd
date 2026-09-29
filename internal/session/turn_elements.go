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
		if !strings.HasSuffix(strings.ToLower(path), ".go") {
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
	f, err := parser.ParseFile(token.NewFileSet(), "", cur, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	byKey := make(map[string]string, len(names))
	for _, d := range f.Decls {
		fn, isFunc := d.(*ast.FuncDecl)
		if !isFunc {
			continue
		}
		key := funcKey(fn)
		if _, dup := byKey[key]; dup {
			continue
		}
		byKey[key] = elementRef(f.Name.Name, fn)
	}
	seen := make(map[string]bool, len(names))
	var out []string
	for _, name := range names {
		ref, ok := byKey[name]
		if !ok || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	sort.Strings(out)
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

// elementReceiver is the receiver the world's Go parser carries in the ref:
// the identifier, pointers unwrapped, anything else dropped
// (extractReceiverTypeInfo in internal/world/go_parser.go). It mirrors that
// function rather than funcKey on purpose: a generic receiver's type arguments
// survive in the pin gate's key ("Box.Get") but not in the code_element ref
// ("fn:<pkg>.Get"), and the ref is what must match.
func elementReceiver(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	for {
		star, ok := expr.(*ast.StarExpr)
		if !ok {
			break
		}
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
