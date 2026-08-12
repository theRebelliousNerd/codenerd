package session

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func newRealKernelForModularity(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	return k
}

func longFuncSource(name string, interior int) string {
	var b strings.Builder
	b.WriteString("package p\n")
	b.WriteString("func " + name + "() {\n")
	for i := 0; i < interior; i++ {
		b.WriteString("\tx := 1\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func manyParamsSource(name string, n int) string {
	var b strings.Builder
	b.WriteString("package p\nfunc " + name + "(")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strings.Repeat("a", i+1) + " int")
	}
	b.WriteString(") {}\n")
	return b.String()
}

func deepNestingSource(name string, depth int) string {
	var b strings.Builder
	b.WriteString("package p\nfunc " + name + "() {\n")
	for i := 0; i < depth; i++ {
		b.WriteString(strings.Repeat("\t", i+1) + "if true {\n")
	}
	for i := depth - 1; i >= 0; i-- {
		b.WriteString(strings.Repeat("\t", i+1) + "}\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func cleanSource() string {
	return "package p\nfunc Small(a, b int) int {\n\tif a > b {\n\t\treturn a\n\t}\n\treturn b\n}\n"
}

func TestModularity_TooLong(t *testing.T) {
	k := newRealKernelForModularity(t)
	path := "a/b.go"
	src := longFuncSource("LongFunc", 120)
	violations, err := evaluateModularity(k, path, src)
	if err != nil {
		t.Fatalf("evaluateModularity error: %v", err)
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "LongFunc") && strings.Contains(v, "function_too_long") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected function_too_long violation naming LongFunc, got %v", violations)
	}
	// kernel must be clean after
	facts, err := k.Query("function_metrics")
	if err != nil {
		t.Fatalf("Query after evaluate failed: %v", err)
	}
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after too_long evaluation, found %v", f)
		}
	}
}

func factFile(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case types.MangleString:
		return string(s)
	case types.MangleAtom:
		return string(s)
	default:
		return fmt.Sprint(v)
	}
}

func TestModularity_ManyParams(t *testing.T) {
	k := newRealKernelForModularity(t)
	path := "a/b.go"
	src := manyParamsSource("ManyParams", 7)
	violations, err := evaluateModularity(k, path, src)
	if err != nil {
		t.Fatalf("evaluateModularity error: %v", err)
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "ManyParams") && strings.Contains(v, "too_many_params") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected too_many_params violation naming ManyParams, got %v", violations)
	}
	facts, _ := k.Query("function_metrics")
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after many_params evaluation, found %v", f)
		}
	}
}

func TestModularity_DeepNesting(t *testing.T) {
	k := newRealKernelForModularity(t)
	path := "a/b.go"
	src := deepNestingSource("DeepFunc", 6)
	violations, err := evaluateModularity(k, path, src)
	if err != nil {
		t.Fatalf("evaluateModularity error: %v", err)
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "DeepFunc") && strings.Contains(v, "deep_nesting") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected deep_nesting violation naming DeepFunc, got %v", violations)
	}
	facts, _ := k.Query("function_metrics")
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after deep_nesting evaluation, found %v", f)
		}
	}
}

func TestModularity_CleanFunction(t *testing.T) {
	k := newRealKernelForModularity(t)
	path := "a/b.go"
	src := cleanSource()
	violations, err := evaluateModularity(k, path, src)
	if err != nil {
		t.Fatalf("evaluateModularity error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations for clean function, got %v", violations)
	}
	facts, _ := k.Query("function_metrics")
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after clean evaluation, found %v", f)
		}
	}
}

func TestModularity_Unparseable(t *testing.T) {
	k := newRealKernelForModularity(t)
	path := "a/b.go"
	src := "this is not go {{{ !@#"
	violations, err := evaluateModularity(k, path, src)
	if err != nil {
		t.Fatalf("unparseable should not error, got %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("unparseable should produce no violations, got %v", violations)
	}
	facts, _ := k.Query("function_metrics")
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after unparseable evaluation, found %v", f)
		}
	}
}

func TestModularity_LeavesKernelClean(t *testing.T) {
	k := newRealKernelForModularity(t)
	path := "a/b.go"

	// with violations
	srcLong := longFuncSource("LongOne", 120)
	violations, err := evaluateModularity(k, path, srcLong)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations for long func")
	}
	facts, err := k.Query("function_metrics")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after violation case, found %v", f)
		}
	}

	// without violations
	srcClean := cleanSource()
	violations, err = evaluateModularity(k, path, srcClean)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations for clean, got %v", violations)
	}
	facts, err = k.Query("function_metrics")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
			t.Fatalf("kernel not clean after clean case, found %v", f)
		}
	}
	// also ensure other predicates are clean
	for _, pred := range []string{"function_params", "function_nesting"} {
		f2, err := k.Query(pred)
		if err != nil {
			t.Fatalf("Query %s failed: %v", pred, err)
		}
		for _, f := range f2 {
			if len(f.Args) > 0 && strings.Contains(factFile(f.Args[0]), path) {
				t.Fatalf("kernel not clean for %s, found %v", pred, f)
			}
		}
	}
}

func TestMetrics_SlashPathIsString(t *testing.T) {
	metrics := []FunctionMetrics{{Name: "Foo", Lines: 10, ParamCount: 2, Complexity: 2, Depth: 1}}
	path := "/a/b/c.go"
	facts := metricsToFacts(path, metrics)
	if len(facts) == 0 {
		t.Fatal("no facts produced")
	}
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		arg0 := f.Args[0]
		if _, ok := arg0.(types.MangleString); !ok {
			t.Fatalf("File arg for %s should be types.MangleString, got %T (%v)", f.Predicate, arg0, arg0)
		}
		// Verify ToAtom produces a String constant, not a Name.
		atom, err := f.ToAtom()
		if err != nil {
			t.Fatalf("ToAtom failed for %s: %v", f.Predicate, err)
		}
		if len(atom.Args) == 0 {
			t.Fatalf("atom has no args")
		}
		term, ok := atom.Args[0].(ast.Constant)
		if !ok {
			t.Fatalf("first arg not ast.Constant, got %T", atom.Args[0])
		}
		if term.Type != ast.StringType {
			t.Fatalf("File arg for %s should be StringType (via MangleString), got %v (symbol %q)", f.Predicate, term.Type, term.Symbol)
		}
		if term.Symbol != path {
			t.Fatalf("symbol mismatch: got %q want %q", term.Symbol, path)
		}
		// FuncName also must be MangleString
		if _, ok := f.Args[1].(types.MangleString); !ok {
			t.Fatalf("FuncName arg for %s should be MangleString, got %T", f.Predicate, f.Args[1])
		}
	}
}

// Additional sanity: method naming
func TestParse_MethodNaming(t *testing.T) {
	src := "package p\ntype R struct{}\nfunc (r R) Method(a int) {}\nfunc Plain() {}\n"
	metrics := parseFunctionMetrics(src)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
	foundMethod := false
	foundPlain := false
	for _, m := range metrics {
		if m.Name == "R.Method" {
			foundMethod = true
			if m.ParamCount != 1 {
				t.Fatalf("method param count want 1 got %d", m.ParamCount)
			}
		}
		if m.Name == "Plain" {
			foundPlain = true
		}
	}
	if !foundMethod {
		t.Fatalf("method naming not as Receiver.Method, got %+v", metrics)
	}
	if !foundPlain {
		t.Fatalf("plain function not found")
	}
}
