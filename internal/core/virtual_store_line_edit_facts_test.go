package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/tools"
	codedom "codenerd/internal/tools/codedom"
	toolcore "codenerd/internal/tools/core"
)

const impactCalcGo = "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n"

const impactCalcTestGo = "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal()\n\t}\n}\n\nfunc TestSub(t *testing.T) {\n\tif Sub(1, 2) != -1 {\n\t\tt.Fatal()\n\t}\n}\n"

// TestToolFacts_LineAndFileEditsDriveTheImpactRule dispatches a real
// edit_lines and a real edit_file through the registry sink VirtualStore
// installs. The code_calls rows are the fn: spelling the rule joins; they
// are not produced by the edit. file_topology is absent, so a same-directory
// cross-product cannot make every test in the package look impacted.
func TestToolFacts_LineAndFileEditsDriveTheImpactRule(t *testing.T) {
	t.Run("edit_lines", func(t *testing.T) {
		driveImpact(t, "edit_lines", "fn:calc.Add", "fn:calc.TestAdd")
	})
	t.Run("edit_file", func(t *testing.T) {
		driveImpact(t, "edit_file", "fn:calc.Sub", "fn:calc.TestSub")
	})
}

func driveImpact(t *testing.T, verb, editedRef, testRef string) {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(impactCalcGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc_test.go"), []byte(impactCalcTestGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/calc\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	t.Cleanup(func() { _ = vs.Close() })
	vs.DisableBootGuard()
	vs.SetKernel(k)
	for _, f := range []Fact{
		{Predicate: "code_element", Args: []any{"fn:calc.Add", "/function", "calc.go", int64(3), int64(5)}},
		{Predicate: "code_element", Args: []any{"fn:calc.Sub", "/function", "calc.go", int64(7), int64(9)}},
		{Predicate: "code_element", Args: []any{"fn:calc.TestAdd", "/function", "calc_test.go", int64(5), int64(9)}},
		{Predicate: "code_element", Args: []any{"fn:calc.TestSub", "/function", "calc_test.go", int64(11), int64(15)}},
		{Predicate: "is_test_function", Args: []any{"fn:calc.TestAdd"}},
		{Predicate: "is_test_function", Args: []any{"fn:calc.TestSub"}},
		{Predicate: "code_calls", Args: []any{"fn:calc.TestAdd", "fn:calc.Add"}},
		{Predicate: "code_calls", Args: []any{"fn:calc.TestSub", "fn:calc.Sub"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}

	reg := tools.NewRegistry()
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := toolcore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	vs.AttachToolFactSink(reg)

	var execErr error
	switch verb {
	case "edit_lines":
		n := lineNo(t, impactCalcGo, "return a + b")
		_, execErr = reg.Execute(context.Background(), "edit_lines", map[string]any{
			"path": "calc.go", "start_line": n, "end_line": n, "new_content": "\treturn a + b + 1",
		})
	case "edit_file":
		_, execErr = reg.Execute(context.Background(), "edit_file", map[string]any{
			"path": "calc.go", "old_text": "return a - b", "new_text": "return a - b - 1",
		})
	default:
		t.Fatalf("unknown verb %s", verb)
	}
	if execErr != nil {
		t.Fatal(execErr)
	}

	if got := queryRefs(t, k, "element_modified"); len(got) != 1 || got[0] != editedRef {
		t.Fatalf("element_modified = %v, want [%s]", got, editedRef)
	}
	if got := queryRefs(t, k, "impacted_test"); len(got) != 1 || got[0] != testRef {
		t.Fatalf("impacted_test = %v, want [%s] (element_modified %v)", got, testRef, queryRefs(t, k, "element_modified"))
	}
}

func lineNo(t *testing.T, src, frag string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, frag) {
			return i + 1
		}
	}
	t.Fatalf("no line contains %q", frag)
	return 0
}

func queryRefs(t *testing.T, k *RealKernel, pred string) []string {
	t.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	var refs []string
	for _, f := range rows {
		if len(f.Args) == 0 {
			continue
		}
		refs = append(refs, fmt.Sprint(f.Args[0]))
	}
	sort.Strings(refs)
	return refs
}
