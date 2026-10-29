package session

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// The kernel's truth table for unwired code (policy/turn_unwired.mg): which
// new functions no production path reaches, which are settled, and whether
// the turn is pushed back and its verdict withheld.
func TestUnwiredPolicy_TruthTable(t *testing.T) {
	const a, b, c = "internal/x.A", "internal/x.B", "internal/x.C"
	type fact struct {
		pred string
		args []any
	}
	newFn := func(ref string) fact {
		return fact{"turn_new_func", []any{types.MangleString(ref), types.MangleAtom("/module")}}
	}
	use := func(ref, from, side string) fact {
		return fact{"turn_func_use", []any{types.MangleString(ref), types.MangleString(from), types.MangleAtom(side)}}
	}
	reading := func(ref string, caller, asks int64) fact {
		return fact{"turn_unused_reason_reading", []any{types.MangleString(ref), caller, asks}}
	}
	for _, tc := range []struct {
		name     string
		facts    []fact
		open     []string
		pushback bool
	}{
		{"nothing calls it", []fact{newFn(a)}, []string{a}, true},
		{"existing code calls it", []fact{newFn(a), use(a, "cmd/nerd.boot", "/production")}, nil, false},
		{"only a test calls it", []fact{newFn(a), use(a, "internal/x.TestA", "/test")}, []string{a}, true},
		// The closure: B is called only by A, which nothing calls.
		{"a new helper of uncalled new code", []fact{newFn(a), newFn(b), use(b, a, "/production")}, []string{a, b}, true},
		{"a new helper of reached new code", []fact{newFn(a), newFn(b), use(b, a, "/production"), use(a, "cmd/nerd.boot", "/production")}, nil, false},
		{"a cycle of new code reaches nothing", []fact{newFn(a), newFn(b), use(a, b, "/production"), use(b, a, "/production")}, []string{a, b}, true},
		{"a method whose name production code mentions", []fact{newFn(c), {"turn_method_mentioned", []any{types.MangleString(c), types.MangleAtom("/production")}}}, nil, false},
		{"an entry point", []fact{newFn(a), {"turn_new_func_entry", []any{types.MangleString(a)}}}, nil, false},
		{"exported API of an importable package", []fact{{"turn_new_func", []any{types.MangleString(a), types.MangleAtom("/api")}}}, nil, false},
		{"generated code", []fact{newFn(a), {"turn_new_func_generated", []any{types.MangleString(a)}}}, nil, false},
		{"an absence the index could not observe", []fact{newFn(a), {"turn_new_func_unobserved", []any{types.MangleString(a)}}}, nil, false},
		{"a reason naming a current caller", []fact{newFn(a), reading(a, 86, 10)}, nil, false},
		{"a reason the task asks for", []fact{newFn(a), reading(a, 2, 98)}, nil, false},
		{"an unsettled reason", []fact{newFn(a), reading(a, 50, 53)}, []string{a}, true},
		{"a rejected reason", []fact{newFn(a), reading(a, 2, 6)}, []string{a}, true},
		{"a settled function's helper", []fact{newFn(a), newFn(b), use(b, a, "/production"), reading(a, 86, 10)}, nil, false},
		{"a reason Jev could not read is taken as stated", []fact{newFn(a), {"turn_unused_reason_unread", []any{types.MangleString(a)}}}, nil, false},
		{"an attempt that wrote nothing stops the push-back", []fact{newFn(a), {"turn_unwired_idle", nil}}, []string{a}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := roundsExecutor(t)
			result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
			if err := config.EnsureParams(e.kernel, (&config.TypeSafeConfig{}).Params()); err != nil {
				t.Fatal(err)
			}
			turn := result.turnAtom()
			for _, f := range tc.facts {
				if !e.assertTurnFact(types.Fact{Predicate: f.pred, Args: append([]any{turn}, f.args...)}) {
					t.Fatalf("assert %s%v", f.pred, f.args)
				}
			}
			var open []string
			for _, fn := range e.openUnwired(result, nil) {
				open = append(open, fn.ref)
			}
			if !slices.Equal(open, tc.open) {
				t.Errorf("open = %v, want %v", open, tc.open)
			}
			if got := e.turnHolds("turn_unwired_pushback", result); got != tc.pushback {
				t.Errorf("turn_unwired_pushback = %v, want %v", got, tc.pushback)
			}
			if got := e.turnHolds("has_red_gate", result); got != (len(tc.open) > 0) {
				t.Errorf("has_red_gate = %v, want %v: open unwired code withholds the verdict", got, len(tc.open) > 0)
			}
		})
	}
}

// The measurement on a real module: a function the turn added is new, a
// changed one is not; a use from existing production code reaches it, a test
// does not, and a method is reached by its name.
func TestUnwiredMeasure_NewFunctionsAndTheirUses(t *testing.T) {
	ws := t.TempDir()
	write := func(rel, content string) string {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("go.mod", "module probe\n\ngo 1.25\n")
	const before = "package lib\n\nfunc Old() int { return 1 }\n"
	lib := write("internal/lib/lib.go", "package lib\n\nfunc Old() int { return 2 }\n\n"+
		"func Wired() int { return helper() }\n\nfunc helper() int { return 3 }\n\n"+
		"func Orphan() int { return 4 }\n\nfunc TestedOnly() int { return 5 }\n\n"+
		"type T struct{}\n\nfunc (T) Serve() error { return nil }\n\nfunc (T) Lonely() {}\n")
	write("internal/lib/iface.go", "package lib\n\ntype Server interface{ Serve() error }\n")
	write("internal/lib/lib_test.go", "package lib\n\nimport \"testing\"\n\nfunc TestTestedOnly(t *testing.T) { TestedOnly() }\n")
	write("main.go", "package main\n\nimport \"probe/internal/lib\"\n\nfunc main() { lib.Wired() }\n")

	e := roundsExecutor(t)
	e.config.WorkspaceRoot = ws
	result := &ExecutionResult{
		Intent:           perception.Intent{Verb: "/fix"},
		WriteToolEffects: 1,
		WrittenPaths:     []string{lib},
		PreWriteContents: map[string]PreImage{lib: {Existed: true, Content: before}},
	}
	funcs, err := e.measureNewFuncs(context.Background(), result)
	if err != nil {
		t.Fatal(err)
	}
	var measured []string
	for ref := range funcs {
		measured = append(measured, ref)
	}
	slices.Sort(measured)
	want := []string{"internal/lib.Orphan", "internal/lib.T.Lonely", "internal/lib.T.Serve", "internal/lib.TestedOnly", "internal/lib.Wired", "internal/lib.helper"}
	if !slices.Equal(measured, want) {
		t.Fatalf("new functions = %v, want %v (Old changed, it is not new)", measured, want)
	}
	open := e.openUnwired(result, funcs)
	var refs []string
	testOnly := map[string]bool{}
	for _, fn := range open {
		refs = append(refs, fn.ref)
		testOnly[fn.ref] = fn.testOnly
	}
	if want := []string{"internal/lib.Orphan", "internal/lib.T.Lonely", "internal/lib.TestedOnly"}; !slices.Equal(refs, want) {
		t.Fatalf("unwired = %v, want %v", refs, want)
	}
	if !testOnly["internal/lib.TestedOnly"] || testOnly["internal/lib.Orphan"] {
		t.Errorf("test-only marks = %v, want TestedOnly alone", testOnly)
	}
	// A second measurement replaces the first instead of adding to it.
	if _, err := e.measureNewFuncs(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	if again := e.openUnwired(result, funcs); len(again) != len(open) {
		t.Fatalf("re-measured: %d open, want %d", len(again), len(open))
	}
}

// The reason format the push-back asks for, read back for the functions it
// named only.
func TestUnwiredReasons_Parse(t *testing.T) {
	open := []newFunc{{ref: "internal/x.A"}, {ref: "internal/x.B"}}
	text := "Wired B? No.\n- unused `internal/x.A`: the plugins module imports it.\n" +
		"unused internal/x.Z: not asked about\nUNUSED internal/x.B : the task asks for the helper only\n"
	got := parseUnusedReasons(text, open)
	want := map[string]string{"internal/x.A": "the plugins module imports it.", "internal/x.B": "the task asks for the helper only"}
	if len(got) != len(want) || got["internal/x.A"] != want["internal/x.A"] || got["internal/x.B"] != want["internal/x.B"] {
		t.Fatalf("parseUnusedReasons = %v, want %v", got, want)
	}
}

// The live failure, driven through the production chain: a turn adds a
// repair function, tests it, and reports done with nothing in production
// calling it. Before the /unwired round the turn closed with the function
// unwired; now it is pushed back until the model wires it, or gives a reason
// Jev reads as real, and a reason that is not leaves the verdict withheld.
func TestUnwiredRound_UnreachedCodeIsPushedBack(t *testing.T) {
	const mainGo = "package main\n\nimport \"repairprobe/internal/atoms\"\n\nfunc main() { atoms.Generate() }\n"
	const wiredMain = "package main\n\nimport \"repairprobe/internal/atoms\"\n\nfunc main() {\n\tatoms.RepairExisting()\n\tatoms.Generate()\n}\n"
	const atomsBefore = "package atoms\n\n// Generate writes atoms.\nfunc Generate() int { return 1 }\n"
	const atomsAfter = atomsBefore + "\n// RepairExisting re-atomizes existing files.\nfunc RepairExisting() int { return 2 }\n"
	const testBefore = "package atoms\n\nimport \"testing\"\n\nfunc TestGenerate(t *testing.T) {\n\tif Generate() != 1 {\n\t\tt.Fatal(\"Generate\")\n\t}\n}\n"
	const testAfter = testBefore + "\nfunc TestRepairExisting(t *testing.T) {\n\tif RepairExisting() != 2 {\n\t\tt.Fatal(\"RepairExisting\")\n\t}\n}\n"
	const task = "fix the atom generator, and repair the existing atom files too. add tests."
	pushbackRef := regexp.MustCompile(`(?m)^- (\S+) \(`)

	drive := func(t *testing.T, judge *ReportJudge, onPushback string) (*ExecutionResult, []string, string) {
		t.Helper()
		h := newRepairHarness(t, func(e *Executor) {
			if judge != nil {
				e.SetReportJudge(judge)
			}
		})
		for rel, content := range map[string]string{"main.go": mainGo, "internal/atoms/atoms.go": atomsBefore, "internal/atoms/atoms_test.go": testBefore} {
			p := filepath.Join(h.ws, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		atomsPath := filepath.Join(h.ws, "internal", "atoms", "atoms.go")
		testPath := filepath.Join(h.ws, "internal", "atoms", "atoms_test.go")
		mainPath := filepath.Join(h.ws, "main.go")
		var pushed []string
		firstWrite := func() *types.LLMToolResponse {
			return &types.LLMToolResponse{ToolCalls: []types.ToolCall{
				h.writeCall("c1", "write_file", atomsPath, atomsAfter),
				h.writeCall("c2", "write_file", testPath, testAfter),
			}}
		}
		h.executor.llmClient = &MockToolResultsLLM{
			MockLLMClient: &MockLLMClient{
				CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
					return firstWrite(), nil
				},
			},
			CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
				last := history[len(history)-1].Text
				if strings.Contains(last, "no production path calls") {
					for _, m := range pushbackRef.FindAllStringSubmatch(last, -1) {
						pushed = append(pushed, m[1])
					}
					switch onPushback {
					case "wire":
						return &types.LLMToolResponse{ToolCalls: []types.ToolCall{h.writeCall("c3", "write_file", mainPath, wiredMain)}}, nil
					case "real-reason":
						return &types.LLMToolResponse{Text: "unused internal/atoms.RepairExisting: the nerd-plugins module imports this package and calls it at its own boot; that module is not in this repository."}, nil
					default:
						return &types.LLMToolResponse{Text: "unused internal/atoms.RepairExisting: I ran it once from a temporary test to repair the existing files, then deleted the test."}, nil
					}
				}
				for _, m := range history {
					if len(m.ToolResults) > 0 {
						return &types.LLMToolResponse{Text: "Added RepairExisting with a test. Done."}, nil
					}
				}
				return firstWrite(), nil
			},
		}
		result, err := h.drive(t, task)
		if err != nil {
			t.Fatalf("drive: %v", err)
		}
		got, _ := os.ReadFile(mainPath)
		return result, pushed, string(got)
	}
	const ref = "internal/atoms.RepairExisting"

	t.Run("wired", func(t *testing.T) {
		result, pushed, mainNow := drive(t, nil, "wire")
		if !slices.Equal(pushed, []string{ref}) {
			t.Fatalf("pushed back %v, want once for %s", pushed, ref)
		}
		if !strings.Contains(mainNow, "atoms.RepairExisting()") {
			t.Fatalf("the wiring is not in main.go:\n%s", mainNow)
		}
		if slices.Contains(result.MissingEvidence, "/code_unwired") {
			t.Fatalf("wired code still charged: %v", result.MissingEvidence)
		}
	})

	t.Run("anExcuseIsNotAReason", func(t *testing.T) {
		judge, _ := liveReportJudge(t)
		result, pushed, _ := drive(t, judge, "excuse")
		if len(pushed) != 1 {
			t.Fatalf("pushed back %d time(s); an attempt that writes nothing ends the round", len(pushed))
		}
		if result.TurnOutcome == "/done" || !slices.Contains(result.MissingEvidence, "/code_unwired") {
			t.Fatalf("outcome %s missing %v: unwired code must withhold the verdict", result.TurnOutcome, result.MissingEvidence)
		}
	})

	t.Run("aRealReasonSettlesIt", func(t *testing.T) {
		judge, _ := liveReportJudge(t)
		result, pushed, _ := drive(t, judge, "real-reason")
		if len(pushed) != 1 {
			t.Fatalf("pushed back %d time(s), want once", len(pushed))
		}
		if slices.Contains(result.MissingEvidence, "/code_unwired") {
			t.Fatalf("a reason naming a current caller still charged: %v", result.MissingEvidence)
		}
		if !strings.Contains(result.Response, "nerd-plugins") {
			t.Fatalf("the reason is not in the turn's answer: %q", result.Response)
		}
	})
}
