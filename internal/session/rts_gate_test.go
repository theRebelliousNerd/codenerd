package session

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// A module where b's tests reach a through a plain call, a closure, a
// subtest, interface dispatch and fmt's String call, and c's tests reach
// nothing in a. Each body is a parameter so a case can write the pre-turn and
// the post-turn file from one template.
type rtsBodies struct{ leaf, sq, name, extra string }

func (b rtsBodies) aGo() string {
	return fmt.Sprintf(`package a

func Leaf() int { return %s }

func Other() int { return 10 }

type Shape interface{ Area() int }

type Sq struct{}

func (Sq) Area() int { return %s }

type Circ struct{}

func (Circ) Area() int { return 3 }
%s`, b.leaf, b.sq, b.extra)
}

func (b rtsBodies) nameGo() string {
	return fmt.Sprintf("package a\n\ntype Name struct{}\n\nfunc (Name) String() string { return %s }\n", b.name)
}

var rtsBefore = rtsBodies{leaf: "1", sq: "4", name: `"n"`}

const rtsBGo = `package b

import (
	"fmt"

	"rtsmod/a"
)

func UsesLeaf() int            { return a.Leaf() }
func UsesOther() int           { return a.Other() }
func Total(s a.Shape) int      { return s.Area() }
func Describe() string         { return fmt.Sprint(a.Name{}) }
func Unrelated() int           { return 7 }

var table = map[string]func() int{"leaf": a.Leaf}

func Dispatch(k string) int { return table[k]() }
`

const rtsBTest = `package b

import (
	"testing"

	"rtsmod/a"
)

func TestLeaf(t *testing.T)      { _ = UsesLeaf() }
func TestOther(t *testing.T)     { _ = UsesOther() }
func TestShape(t *testing.T)     { _ = Total(a.Sq{}) }
func TestShapeCirc(t *testing.T) { _ = Total(a.Circ{}) }
func TestDescribe(t *testing.T)  { _ = Describe() }
func TestUnrelated(t *testing.T) { _ = Unrelated() }
func TestDispatch(t *testing.T)  { _ = Dispatch("leaf") }

func TestSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) { _ = UsesLeaf() })
}

func TestClosure(t *testing.T) {
	f := func() int { return a.Other() }
	t.Run("y", func(t *testing.T) { _ = f() })
}
`

const rtsCGo = "package c\n\nfunc Nothing() int { return 0 }\n"

const rtsCTest = `package c

import (
	"testing"

	"rtsmod/a"
)

var _ a.Shape

func TestNothing(t *testing.T) { _ = Nothing() }
`

func rtsFiles(a rtsBodies, extra map[string]string) map[string]string {
	files := map[string]string{
		"go.mod":      "module rtsmod\n\ngo 1.21\n",
		"a/a.go":      a.aGo(),
		"a/name.go":   a.nameGo(),
		"b/b.go":      rtsBGo,
		"b/b_test.go": rtsBTest,
		"c/c.go":      rtsCGo,
		"c/c_test.go": rtsCTest,
	}
	for k, v := range extra {
		files[k] = v
	}
	return files
}

// rtsPlan writes the post-turn module, says the turn wrote the files in pre
// (whose pre-turn contents these are), and plans the importer gate.
func rtsPlan(t *testing.T, files, pre map[string]string) importerPlan {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := writeBaselineModule(t, files)
	result := mutationResult()
	result.PreWriteContents = map[string]PreImage{}
	for path, content := range pre {
		result.WrittenPaths = append(result.WrittenPaths, path)
		result.PreWriteContents[path] = existed(content)
	}
	slices.Sort(result.WrittenPaths)
	scope := discoverImporters(context.Background(), ws, result)
	if scope.why != "" {
		t.Fatalf("importers not discovered: %s", scope.why)
	}
	return selectImporterRuns(context.Background(), ws, result, scope)
}

// selectedTests reads the plan's -run patterns back as package -> test names;
// a package run whole maps to nil.
func selectedTests(plan importerPlan) map[string][]string {
	out := map[string][]string{}
	for _, run := range plan.runs {
		var names []string
		for i, a := range run.Args {
			if a == "-run" {
				names = strings.Split(strings.TrimSuffix(strings.TrimPrefix(run.Args[i+1], "^("), ")$"), "|")
				slices.Sort(names)
			}
		}
		for _, pkg := range run.Packages {
			out[pkg] = names
		}
	}
	return out
}

func requireSelection(t *testing.T, plan importerPlan, want map[string][]string) {
	t.Helper()
	if plan.selection.Fallback != "" {
		t.Fatalf("fell back to all importer tests: %s", plan.selection.Fallback)
	}
	got := selectedTests(plan)
	if len(got) != len(want) {
		t.Fatalf("packages run = %v, want %v\n%s", got, want, plan.selection.Summary())
	}
	for pkg, names := range want {
		slices.Sort(names)
		if !slices.Equal(got[pkg], names) {
			t.Fatalf("%s runs %v, want %v\n%s", pkg, got[pkg], names, plan.selection.Summary())
		}
	}
}

func requireFallback(t *testing.T, plan importerPlan, why string) {
	t.Helper()
	if !strings.Contains(plan.selection.Fallback, why) {
		t.Fatalf("fallback = %q, want it to say %q", plan.selection.Fallback, why)
	}
	if len(plan.runs) != 1 || len(plan.runs[0].Args) != 0 || !slices.Equal(plan.runs[0].Packages, []string{"rtsmod/b", "rtsmod/c"}) {
		t.Fatalf("a fallback must run every importer package whole, got %+v", plan.runs)
	}
}

func TestSelectImporterRuns_BodyChangeInALeafSelectsOnlyTheTestsThatReachIt(t *testing.T) {
	t.Parallel()
	after := rtsBefore
	after.leaf = "2"
	plan := rtsPlan(t, rtsFiles(after, nil), map[string]string{"a/a.go": rtsBefore.aGo()})
	// TestLeaf calls it, TestSubtest reaches it through a closure handed to
	// t.Run, TestDispatch through a function value in a package-level table
	// (which must not make package initialization reach it). TestOther, TestShape and TestUnrelated cannot, and c's one test
	// never touches it, so c is not run at all.
	requireSelection(t, plan, map[string][]string{"rtsmod/b": {"TestLeaf", "TestSubtest", "TestDispatch"}})
	if got := plan.selection.Skipped; !slices.Equal(got, []string{"rtsmod/c"}) {
		t.Errorf("skipped = %v, want [rtsmod/c]: the result must say which packages did not run", got)
	}
	if plan.selection.Selected != 3 || plan.selection.Total != 10 {
		t.Errorf("selected %d of %d tests, want 3 of 10", plan.selection.Selected, plan.selection.Total)
	}
	if plan.selection.Analysis <= 0 || !strings.Contains(plan.selection.Summary(), "selected 3 of 10") {
		t.Errorf("summary %q should carry the counts and the analysis time", plan.selection.Summary())
	}
}

func TestSelectImporterRuns_ATestReachingTheChangeThroughAClosureIsSelected(t *testing.T) {
	t.Parallel()
	// Other is only called inside f, a closure that a subtest closure calls.
	plan := rtsPlan(t, rtsFiles(rtsBefore, map[string]string{"a/a.go": strings.Replace(rtsBefore.aGo(), "return 10", "return 11", 1)}),
		map[string]string{"a/a.go": rtsBefore.aGo()})
	requireSelection(t, plan, map[string][]string{"rtsmod/b": {"TestOther", "TestClosure"}})
}

func TestSelectImporterRuns_AnInterfaceMethodChangeSelectsEveryTestThatCanDispatchToIt(t *testing.T) {
	t.Parallel()
	// Nothing calls Circ.Area by name: b.Total calls Area through a.Shape.
	// The test that hands Total a Circ can dispatch to the changed method; the
	// one that hands it a Sq cannot, and neither can the rest.
	after := strings.Replace(rtsBefore.aGo(), "func (Circ) Area() int { return 3 }", "func (Circ) Area() int { return 33 }", 1)
	plan := rtsPlan(t, rtsFiles(rtsBefore, map[string]string{"a/a.go": after}), map[string]string{"a/a.go": rtsBefore.aGo()})
	requireSelection(t, plan, map[string][]string{"rtsmod/b": {"TestShape", "TestShapeCirc"}})
}

func TestSelectImporterRuns_AMethodCalledBackByALibraryIsReached(t *testing.T) {
	t.Parallel()
	// fmt.Sprint calls Name.String; no code of b calls it. Converting the
	// value to an interface is what hands the method to fmt.
	after := rtsBefore
	after.name = `"renamed"`
	plan := rtsPlan(t, rtsFiles(after, nil), map[string]string{"a/name.go": rtsBefore.nameGo()})
	requireSelection(t, plan, map[string][]string{"rtsmod/b": {"TestDescribe"}})
}

func TestSelectImporterRuns_AFunctionRunAtInitializationReachesEveryTest(t *testing.T) {
	t.Parallel()
	seeded := rtsBefore
	seeded.extra = "\nvar seeded = Leaf()\n"
	after := seeded
	after.leaf = "2"
	plan := rtsPlan(t, rtsFiles(after, nil), map[string]string{"a/a.go": seeded.aGo()})
	if plan.selection.Fallback != "" {
		t.Fatalf("fell back: %s; initialization reach is a selection, not a fallback", plan.selection.Fallback)
	}
	got := selectedTests(plan)
	for _, pkg := range []string{"rtsmod/b", "rtsmod/c"} {
		if names, ok := got[pkg]; !ok || names != nil {
			t.Errorf("%s = %v (present %v), want run whole: a's package initializer reaches Leaf and runs before every test", pkg, names, ok)
		}
	}
}

func TestSelectImporterRuns_ChangesOutsideFunctionBodiesRunEverything(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		pre, post string
		why       string
	}{
		{"package var", "var Limit = 1\n", "var Limit = 2\n", "package-level"},
		{"package const", "const Limit = 1\n", "const Limit = 2\n", "package-level"},
		{"type", "type T struct{ A int }\n", "type T struct{ A, B int }\n", "package-level"},
		{"interface method set", "type I interface{ M() }\n", "type I interface {\n\tM()\n\tN()\n}\n", "package-level"},
		{"init added", "", "func init() {}\n", "init function"},
		{"signature", "func Extra(x int) int { return x }\n", "func Extra(x, y int) int { return x }\n", "signature of Extra"},
		{"function removed", "func Extra() int { return 1 }\n", "", "Extra was removed"},
		{"method added", "type T struct{}\n", "type T struct{}\n\nfunc (T) M() {}\n", "method T.M was added"},
		{"directive", "", "", "directive or build constraint"},
		{"import added", "", "import _ \"fmt\"\n", "imports changed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pre := rtsBefore
			pre.extra = "\n" + tc.pre
			post := rtsBefore
			post.extra = "\n" + tc.post
			preSrc, postSrc := pre.aGo(), post.aGo()
			if tc.name == "directive" {
				postSrc = strings.Replace(preSrc, "func Leaf()", "//go:noinline\nfunc Leaf()", 1)
			}
			if tc.name == "import added" {
				postSrc = strings.Replace(preSrc, "package a\n", "package a\n\n"+tc.post, 1)
			}
			files := rtsFiles(rtsBefore, map[string]string{"a/a.go": postSrc})
			requireFallback(t, rtsPlan(t, files, map[string]string{"a/a.go": preSrc}), tc.why)
		})
	}
}

func TestSelectImporterRuns_ANewFileOrANonGoFileBesideGoCodeRunsEverything(t *testing.T) {
	t.Parallel()
	t.Run("new file with a var", func(t *testing.T) {
		t.Parallel()
		files := rtsFiles(rtsBefore, map[string]string{"a/new.go": "package a\n\nvar Fresh = 1\n"})
		ws := writeBaselineModule(t, files)
		result := mutationResult()
		result.WrittenPaths = []string{"a/new.go"}
		result.PreWriteContents = map[string]PreImage{"a/new.go": {Existed: false}}
		scope := discoverImporters(context.Background(), ws, result)
		requireFallback(t, selectImporterRuns(context.Background(), ws, result, scope), "package-level")
	})
	t.Run("data beside go code", func(t *testing.T) {
		t.Parallel()
		files := rtsFiles(rtsBefore, map[string]string{"a/data.txt": "x"})
		ws := writeBaselineModule(t, files)
		result := mutationResult()
		result.WrittenPaths = []string{"a/a.go", "a/data.txt"}
		result.PreWriteContents = map[string]PreImage{"a/a.go": existed(rtsBefore.aGo()), "a/data.txt": {Existed: false}}
		scope := discoverImporters(context.Background(), ws, result)
		requireFallback(t, selectImporterRuns(context.Background(), ws, result, scope), "non-Go file")
	})
	t.Run("no snapshot", func(t *testing.T) {
		t.Parallel()
		ws := writeBaselineModule(t, rtsFiles(rtsBefore, nil))
		result := mutationResult()
		result.WrittenPaths = []string{"a/a.go"}
		scope := discoverImporters(context.Background(), ws, result)
		requireFallback(t, selectImporterRuns(context.Background(), ws, result, scope), "no pre-write snapshot")
	})
}

func TestSelectImporterRuns_AnImporterThatDoesNotLoadRunsEverything(t *testing.T) {
	t.Parallel()
	after := rtsBefore
	after.leaf = "2"
	files := rtsFiles(after, map[string]string{"c/c.go": "package c\n\nfunc Nothing() int { return undefinedName }\n"})
	requireFallback(t, rtsPlan(t, files, map[string]string{"a/a.go": rtsBefore.aGo()}), "does not load")
}

func TestSelectImporterRuns_ATestThatRunsAnotherProgramOrReadsTheTreeIsAlwaysSelected(t *testing.T) {
	t.Parallel()
	after := rtsBefore
	after.leaf = "2"
	files := rtsFiles(after, map[string]string{
		"b/spawn_test.go": "package b\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestSpawns(t *testing.T) { _ = exec.Command(\"true\").Run() }\n",
		"b/tree_test.go":  "package b\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestReadsTheTree(t *testing.T) { _, _ = os.ReadFile(\"../a/a.go\") }\n",
	})
	plan := rtsPlan(t, files, map[string]string{"a/a.go": rtsBefore.aGo()})
	requireSelection(t, plan, map[string][]string{"rtsmod/b": {"TestLeaf", "TestSubtest", "TestDispatch", "TestSpawns", "TestReadsTheTree"}})
}

func TestSelectImporterRuns_TestMainThatReachesTheChangeRunsThePackageWhole(t *testing.T) {
	t.Parallel()
	after := rtsBefore
	after.leaf = "2"
	files := rtsFiles(after, map[string]string{
		"b/main_test.go": "package b\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) { _ = UsesLeaf(); os.Exit(m.Run()) }\n",
	})
	plan := rtsPlan(t, files, map[string]string{"a/a.go": rtsBefore.aGo()})
	got := selectedTests(plan)
	if names, ok := got["rtsmod/b"]; !ok || names != nil {
		t.Fatalf("b = %v (present %v), want run whole: every test runs under a TestMain that calls Leaf", names, ok)
	}
	if _, ok := got["rtsmod/c"]; ok {
		t.Errorf("c ran, but nothing in it reaches a change")
	}
}

func TestSelectImporterRuns_ABodyChangeNoTestReachesRunsNothingAndSaysSo(t *testing.T) {
	t.Parallel()
	after := rtsBefore
	after.leaf = "2"
	files := rtsFiles(after, nil)
	files["b/b_test.go"] = "package b\n\nimport \"testing\"\n\nfunc TestUnrelated(t *testing.T) { _ = Unrelated() }\n"
	files["c/c_test.go"] = "package c\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) { _ = Nothing() }\n"
	ws := writeBaselineModule(t, files)
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(rtsBefore.aGo())}
	scope := discoverImporters(context.Background(), ws, result)
	v := verifyImporters(context.Background(), ws, result, scope)
	if v.Verdict() != VerifySkipped {
		t.Fatalf("verdict = %s (%s), want skipped: no importer test can reach the change", v.Verdict(), v.Reason)
	}
	if v.Selection == nil || len(v.Selection.Skipped) != 1 || !strings.Contains(v.Reason, "selected 0 of 1") {
		t.Fatalf("the result must say what was left out: reason %q, selection %+v", v.Reason, v.Selection)
	}
}

func TestVerifyImporters_ASelectedFailingTestFailsTheGateAndAnUnselectedOneDoesNotRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	after := rtsBefore
	after.leaf = "2"
	files := rtsFiles(after, nil)
	files["b/b_test.go"] = `package b

import "testing"

func TestLeaf(t *testing.T) {
	if UsesLeaf() != 1 {
		t.Fatalf("UsesLeaf = %d, want 1", UsesLeaf())
	}
}

func TestAlreadyRed(t *testing.T) { t.Fatal("red, and nothing the change reaches") }
`
	ws := writeBaselineModule(t, files)
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(rtsBefore.aGo())}
	scope := discoverImporters(context.Background(), ws, result)
	v := verifyImporters(context.Background(), ws, result, scope)
	if v.Verdict() != VerifyFailed {
		t.Fatalf("verdict = %s (%s), want failed: TestLeaf reaches the change and fails", v.Verdict(), v.Reason)
	}
	if !strings.Contains(v.Output, "TestLeaf") {
		t.Errorf("the failure does not name TestLeaf:\n%s", v.Output)
	}
	if strings.Contains(v.Output, "TestAlreadyRed") {
		t.Errorf("TestAlreadyRed was not selected and must not run:\n%s", v.Output)
	}
	if !strings.Contains(v.Reason, "import what this turn changed") {
		t.Errorf("reason = %q, want the importer failure's usual reason", v.Reason)
	}
	if v.Selection == nil || v.Selection.Selected != 1 || v.Selection.Total != 3 {
		t.Errorf("selection = %+v, want 1 of 3", v.Selection)
	}
	// The /test rule reads the same failing_test rows it always did.
	result.ImporterCheck = v
	result.TestCheck = TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	e := newObligationExec(t)
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifyFailed {
		t.Fatalf("test gate = %s, want failing", got)
	}
}

func TestSelectImporterRuns_AnAnalysisOverItsBudgetRunsEverything(t *testing.T) {
	old := rtsAnalysisBudget
	rtsAnalysisBudget = time.Nanosecond
	t.Cleanup(func() { rtsAnalysisBudget = old })
	after := rtsBefore
	after.leaf = "2"
	requireFallback(t, rtsPlan(t, rtsFiles(after, nil), map[string]string{"a/a.go": rtsBefore.aGo()}), "budget")
}
