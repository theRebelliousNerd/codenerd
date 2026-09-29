package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	codedom "codenerd/internal/tools/codedom"
	"codenerd/internal/usage"
)

// TestEditLines_HandlerAssertsTheSameElementModifiedAsTheRegistry dispatches
// the same line edit through VirtualStore.executeAction (the production
// switch that reaches handleEditLines / handleInsertLines / handleDeleteLines)
// and through the tool registry. impacted_test joins element_modified
// (policy/test_impact.mg); the registry records that from RecordChangedSource,
// and these handlers did not.
//
// The timestamp column is the instant each path asserted. The rows compared
// here are the ref and the session, which are what the join reads.
func TestEditLines_HandlerAssertsTheSameElementModifiedAsTheRegistry(t *testing.T) {
	const session = "sess-line"
	src := impactCalcGo

	t.Run("edit_lines", func(t *testing.T) {
		n := lineNo(t, src, "return a + b")
		handler, registry := compareLineEdit(t, session, src, ActionEditLines, map[string]any{
			"start_line": n, "end_line": n, "content": "\treturn a + b + 1",
		}, map[string]any{
			"start_line": n, "end_line": n, "new_content": "\treturn a + b + 1",
		})
		if len(handler) == 0 {
			t.Fatalf("handler element_modified is empty; registry has %v", registry)
		}
		if strings.Join(handler, ",") != strings.Join(registry, ",") {
			t.Fatalf("element_modified handler %v, registry %v", handler, registry)
		}
		if handler[0] != "fn:calc.Add|"+session {
			t.Fatalf("element_modified = %v, want fn:calc.Add", handler)
		}
	})

	t.Run("insert_lines", func(t *testing.T) {
		after := lineNo(t, src, "return a + b") + 1
		body := "\nfunc Mul(a, b int) int {\n\treturn a * b\n}\n"
		handler, registry := compareLineEdit(t, session, src, ActionInsertLines, map[string]any{
			"after_line": after, "content": body,
		}, map[string]any{
			"after_line": after, "content": body,
		})
		if len(handler) == 0 {
			t.Fatalf("handler element_modified is empty; registry has %v", registry)
		}
		if strings.Join(handler, ",") != strings.Join(registry, ",") {
			t.Fatalf("element_modified handler %v, registry %v", handler, registry)
		}
		if handler[0] != "fn:calc.Mul|"+session {
			t.Fatalf("element_modified = %v, want fn:calc.Mul", handler)
		}
	})

	t.Run("delete_lines", func(t *testing.T) {
		start, end := declSpan(t, src, "func Sub")
		handler, registry := compareLineEdit(t, session, src, ActionDeleteLines, map[string]any{
			"start_line": start, "end_line": end,
		}, map[string]any{
			"start_line": start, "end_line": end,
		})
		if len(handler) == 0 {
			t.Fatalf("handler element_modified is empty; registry has %v", registry)
		}
		if strings.Join(handler, ",") != strings.Join(registry, ",") {
			t.Fatalf("element_modified handler %v, registry %v", handler, registry)
		}
		if handler[0] != "fn:calc.Sub|"+session {
			t.Fatalf("element_modified = %v, want fn:calc.Sub", handler)
		}
	})
}

// TestEditLines_HandlerDrivesImpactedTest is the failure the line handlers
// had: a real edit of Add produced no element_modified, so impacted_test
// stayed empty even though TestAdd calls Add.
func TestEditLines_HandlerDrivesImpactedTest(t *testing.T) {
	dir := resolvedTempDir(t)
	writeCalcFixture(t, dir)
	k := newImpactKernel(t)
	vs := lineEditVirtualStore(t, dir, k)

	n := lineNo(t, impactCalcGo, "return a + b")
	res, err := vs.executeAction(context.Background(), ActionRequest{
		ActionID: "edit-add", SessionID: "sess-line", Type: ActionEditLines, Target: "calc.go",
		Payload: map[string]any{"start_line": n, "end_line": n, "content": "\treturn a + b + 1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("edit failed: %s", res.Error)
	}
	if got := factRefs(res.FactsToAdd, "element_modified"); len(got) != 1 || got[0] != "fn:calc.Add" {
		t.Fatalf("FactsToAdd element_modified = %v, want [fn:calc.Add]", got)
	}
	vs.injectFacts(res.FactsToAdd)
	if got := queryRefs(t, k, "impacted_test"); len(got) != 1 || got[0] != "fn:calc.TestAdd" {
		t.Fatalf("impacted_test = %v, want [fn:calc.TestAdd] (element_modified %v)", got, queryRefs(t, k, "element_modified"))
	}
}

func compareLineEdit(t *testing.T, session, src string, action ActionType, handlerPayload, registryArgs map[string]any) (handler, registry []string) {
	t.Helper()
	handler = elementModifiedIdentity(t, runHandlerLineEdit(t, session, src, action, handlerPayload))
	registry = elementModifiedIdentity(t, runRegistryLineEdit(t, session, src, string(action), registryArgs))
	return handler, registry
}

func runHandlerLineEdit(t *testing.T, session, src string, action ActionType, payload map[string]any) *RealKernel {
	t.Helper()
	dir := resolvedTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vs := lineEditVirtualStore(t, dir, k)
	res, err := vs.executeAction(context.Background(), ActionRequest{
		ActionID: "line", SessionID: session, Type: action, Target: "calc.go", Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("%s failed: %s", action, res.Error)
	}
	vs.injectFacts(res.FactsToAdd)
	return k
}

func runRegistryLineEdit(t *testing.T, session, src, verb string, args map[string]any) *RealKernel {
	t.Helper()
	dir := resolvedTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	t.Cleanup(func() { _ = vs.Close() })
	vs.DisableBootGuard()
	vs.SetKernel(k)
	reg := tools.NewRegistry()
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	vs.AttachToolFactSink(reg)
	args = cloneArgs(args)
	args["path"] = "calc.go"
	if _, err := reg.Execute(usage.WithSessionID(context.Background(), session), verb, args); err != nil {
		t.Fatal(err)
	}
	return k
}

func lineEditVirtualStore(t *testing.T, dir string, k *RealKernel) *VirtualStore {
	t.Helper()
	vs := NewVirtualStore(nil)
	t.Cleanup(func() { _ = vs.Close() })
	vs.workingDir = dir
	vs.workspaceRoot = dir
	vs.SetKernel(k)
	editor := tactile.NewFileEditor()
	editor.SetWorkingDir(dir)
	vs.SetFileEditor(NewTactileFileEditorAdapter(editor))
	return vs
}

func newImpactKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []Fact{
		{Predicate: "code_element", Args: []any{"fn:calc.Add", "/function", "calc.go", int64(3), int64(5)}},
		{Predicate: "code_element", Args: []any{"fn:calc.TestAdd", "/function", "calc_test.go", int64(5), int64(9)}},
		{Predicate: "is_test_function", Args: []any{"fn:calc.TestAdd"}},
		{Predicate: "code_calls", Args: []any{"fn:calc.TestAdd", "fn:calc.Add"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	return k
}

func writeCalcFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(impactCalcGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc_test.go"), []byte(impactCalcTestGo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir
}

func elementModifiedIdentity(t *testing.T, k *RealKernel) []string {
	t.Helper()
	rows, err := k.Query("element_modified")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, f := range rows {
		if len(f.Args) < 2 {
			t.Fatalf("element_modified arity %d: %v", len(f.Args), f.Args)
		}
		ids = append(ids, fmt.Sprint(f.Args[0])+"|"+fmt.Sprint(f.Args[1]))
	}
	sort.Strings(ids)
	return ids
}

func factRefs(facts []Fact, pred string) []string {
	var refs []string
	for _, f := range facts {
		if f.Predicate != pred || len(f.Args) == 0 {
			continue
		}
		refs = append(refs, fmt.Sprint(f.Args[0]))
	}
	sort.Strings(refs)
	return refs
}

func declSpan(t *testing.T, src, decl string) (int, int) {
	t.Helper()
	lines := strings.Split(src, "\n")
	start := 0
	for i, line := range lines {
		if strings.Contains(line, decl) {
			start = i + 1
			break
		}
	}
	if start == 0 {
		t.Fatalf("declaration %q not found", decl)
	}
	depth := 0
	seen := false
	for i := start - 1; i < len(lines); i++ {
		depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		if strings.Contains(lines[i], "{") {
			seen = true
		}
		if seen && depth == 0 {
			return start, i + 1
		}
	}
	t.Fatalf("declaration %q has no closing brace", decl)
	return 0, 0
}

func cloneArgs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
