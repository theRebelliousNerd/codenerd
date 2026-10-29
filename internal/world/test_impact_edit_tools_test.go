package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tools"
	codedom "codenerd/internal/tools/codedom"
	toolcore "codenerd/internal/tools/core"
)

const editImpactCalcGo = "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n"

const editImpactCalcTestGo = "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal()\n\t}\n}\n\nfunc TestSub(t *testing.T) {\n\tif Sub(1, 2) != -1 {\n\t\tt.Fatal()\n\t}\n}\n"

// kernelQuerier adapts the kernel the edit sink writes to the shape
// run_impacted_tests asks for. core.Fact and codedom.FactData are the same
// two fields; this is a copy, matching internal/system/test_impact_provider.go.
type kernelQuerier struct{ k *core.RealKernel }

func (q kernelQuerier) Query(predicate string) ([]codedom.FactData, error) {
	facts, err := q.k.Query(predicate)
	if err != nil {
		return nil, err
	}
	out := make([]codedom.FactData, len(facts))
	for i, f := range facts {
		out[i] = codedom.FactData{Predicate: f.Predicate, Args: f.Args}
	}
	return out, nil
}

// editImpactProvider is the production provider shape, pointed at this
// test's kernel and at world.TestDependencyBuilder — the analyzer
// run_impacted_tests uses when the process is booted.
type editImpactProvider struct {
	kernel codedom.KernelQuerier
	root   string
}

func (p *editImpactProvider) GetKernel() codedom.KernelQuerier { return p.kernel }
func (p *editImpactProvider) GetProjectRoot() string           { return p.root }
func (p *editImpactProvider) NewTestDependencyAnalyzer() codedom.TestDependencyAnalyzer {
	return NewTestDependencyBuilder(p.kernel, p.root)
}

// TestImpactChain_WhenEditLinesAndEditFile_ShouldNameTheCaller is the
// chain the line and file tools used to skip: a real edit, the
// element_modified fact the sink asserts from it, impacted_test derived
// by test_impact.mg, and run_impacted_tests naming that test with no
// edited_refs argument. The seed is the Go parser and the cartographer,
// not a hand-written element_modified. file_topology is not seeded, so
// the builder's same-directory cross-product cannot mark every test in
// the directory impacted.
func TestImpactChain_WhenEditLinesAndEditFile_ShouldNameTheCaller(t *testing.T) {
	t.Run("edit_lines", func(t *testing.T) {
		driveEditImpact(t, "edit_lines", "fn:calc.Add", "fn:calc.TestAdd")
	})
	t.Run("edit_file", func(t *testing.T) {
		driveEditImpact(t, "edit_file", "fn:calc.Sub", "fn:calc.TestSub")
	})
}

func driveEditImpact(t *testing.T, verb, editedRef, testRef string) {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	writeWorkspaceFile(t, dir, "go.mod", "module example.com/calc\n\ngo 1.26\n")
	writeWorkspaceFile(t, dir, "calc.go", editImpactCalcGo)
	writeWorkspaceFile(t, dir, "calc_test.go", editImpactCalcTestGo)

	facts := editImpactSeed(t, dir, "calc.go", "calc_test.go")
	if !seedHas(facts, "code_element", "fn:calc.Add") || !seedHas(facts, "is_test_function", "fn:calc.TestAdd") {
		t.Fatalf("parser seed is missing the code_element spelling this test joins on")
	}
	if !seedCall(facts, "fn:calc.TestAdd", "fn:calc.Add") || !seedCall(facts, "fn:calc.TestSub", "fn:calc.Sub") {
		t.Fatalf("cartographer seed is missing the fn: call rows: %v", callRows(facts))
	}
	k := seedRealKernel(t, facts)

	vs := core.NewVirtualStoreWithConfig(nil, core.DefaultVirtualStoreConfig())
	t.Cleanup(func() { _ = vs.Close() })
	vs.DisableBootGuard()
	vs.SetKernel(k)

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
		n := editImpactLine(t, editImpactCalcGo, "return a + b")
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

	refs := elementModifiedRefs(t, k)
	if len(refs) != 1 || refs[0] != editedRef {
		t.Fatalf("element_modified = %v, want [%s]", refs, editedRef)
	}
	assertRows(t, "impacted_test", queryRows(t, k, "impacted_test"),
		row("impacted_test", testRef),
	)

	t.Cleanup(func() { codedom.RegisterTestImpactProvider(nil) })
	codedom.RegisterTestImpactProvider(&editImpactProvider{kernel: kernelQuerier{k}, root: dir})
	out, err := reg.Execute(context.Background(), "run_impacted_tests", map[string]any{
		"dry_run": true, "include_low_priority": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.Result, "no element_modified facts found") {
		t.Fatalf("run_impacted_tests saw no edit:\n%s", out.Result)
	}
	if !strings.Contains(out.Result, testRef) || strings.Contains(out.Result, otherTest(testRef)) {
		t.Fatalf("run_impacted_tests = %q, want %s and not %s", out.Result, testRef, otherTest(testRef))
	}
	if !strings.Contains(out.Result, "(dry run - tests not executed)") {
		t.Fatalf("dry run did not stay a dry run:\n%s", out.Result)
	}
}

func otherTest(testRef string) string {
	if testRef == "fn:calc.TestAdd" {
		return "fn:calc.TestSub"
	}
	return "fn:calc.TestAdd"
}

// TestImpactChain_GenericReceiverEditMatchesCodeElement checks the ref the
// sink asserts against the ref GoCodeParser emits for the same method.
// *Box[T], Box[T] and (Box[T]) all name Box; the plain func Get must not
// be the element a method-body edit records.
func TestImpactChain_GenericReceiverEditMatchesCodeElement(t *testing.T) {
	src := "package elemprobe\n\ntype Box[T any] struct{ v T }\n\nfunc Get() int { return 1 }\n\nfunc (b *Box[T]) Get() T {\n\treturn b.v\n}\n"
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	writeWorkspaceFile(t, dir, "box.go", src)
	elems, err := NewGoCodeParser(dir).Parse("box.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var want string
	for _, e := range elems {
		if e.Ref == "fn:elemprobe.Box.Get" {
			want = e.Ref
		}
	}
	if want == "" {
		t.Fatal("GoCodeParser did not emit fn:elemprobe.Box.Get")
	}

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vs := core.NewVirtualStoreWithConfig(nil, core.DefaultVirtualStoreConfig())
	t.Cleanup(func() { _ = vs.Close() })
	vs.DisableBootGuard()
	vs.SetKernel(k)
	reg := tools.NewRegistry()
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	vs.AttachToolFactSink(reg)

	n := editImpactLine(t, src, "return b.v")
	if _, err := reg.Execute(context.Background(), "edit_lines", map[string]any{
		"path": "box.go", "start_line": n, "end_line": n, "new_content": "\treturn b.v /*t*/",
	}); err != nil {
		t.Fatal(err)
	}
	refs := elementModifiedRefs(t, k)
	if len(refs) != 1 || refs[0] != want {
		t.Fatalf("element_modified = %v, want [%s]", refs, want)
	}
	for _, ref := range refs {
		if ref == "fn:elemprobe.Get" {
			t.Fatalf("method edit recorded the plain function: %v", refs)
		}
	}
}

func editImpactSeed(t *testing.T, root string, rels ...string) []core.Fact {
	t.Helper()
	parser := NewGoCodeParser(root)
	cart := NewCartographer()
	defer cart.Close()
	var facts []core.Fact
	for _, rel := range rels {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		elems, err := parser.Parse(rel, body)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for i := range elems {
			for _, f := range elems[i].ToFacts() {
				switch f.Predicate {
				case "code_element", "is_test_function":
					facts = append(facts, f)
				}
			}
		}
		deep, err := cart.MapFileAs(filepath.Join(root, filepath.FromSlash(rel)), rel)
		if err != nil {
			t.Fatalf("map %s: %v", rel, err)
		}
		for _, f := range deep {
			switch f.Predicate {
			case "code_calls", "code_defines":
				facts = append(facts, f)
			}
		}
	}
	return facts
}

func seedHas(facts []core.Fact, pred, arg0 string) bool {
	for _, f := range facts {
		if f.Predicate == pred && len(f.Args) > 0 && f.Args[0] == arg0 {
			return true
		}
	}
	return false
}

func seedCall(facts []core.Fact, caller, callee string) bool {
	for _, f := range facts {
		if f.Predicate != "code_calls" || len(f.Args) < 2 {
			continue
		}
		if f.Args[0] == caller && f.Args[1] == callee {
			return true
		}
	}
	return false
}

func callRows(facts []core.Fact) []core.Fact {
	var out []core.Fact
	for _, f := range facts {
		if f.Predicate == "code_calls" {
			out = append(out, f)
		}
	}
	return out
}

func elementModifiedRefs(t *testing.T, k *core.RealKernel) []string {
	t.Helper()
	var refs []string
	for _, f := range queryRows(t, k, "element_modified") {
		if len(f.Args) == 0 {
			continue
		}
		refs = append(refs, f.Args[0].(string))
	}
	return refs
}

func editImpactLine(t *testing.T, src, frag string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, frag) {
			return i + 1
		}
	}
	t.Fatalf("no line contains %q", frag)
	return 0
}
