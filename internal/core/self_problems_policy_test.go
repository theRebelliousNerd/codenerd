package core

import (
	"strings"
	"testing"

	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

func selfProblemsKernel(t *testing.T, maxProblems int) (*RealKernel, *problemlog.Report) {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	rep, err := problemlog.ReadFile("../problemlog/testdata/problems_fixture.txt", problemlog.Options{
		MaxProblems: maxProblems,
		Skip:        func(e problemlog.Entry) bool { return strings.Contains(e.Message, "debug-self:") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := problemlog.Replace(k, rep); err != nil {
		t.Fatal(err)
	}
	return k, rep
}

func selfProblemLines(t *testing.T, k *RealKernel) []string {
	t.Helper()
	rows, err := k.Query("injectable_context")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, f := range rows {
		if s := types.ExtractString(f.Args[1]); strings.HasPrefix(s, "Own log ") {
			lines = append(lines, s)
		}
	}
	return lines
}

// The projected log is queryable: counts, whole samples, first and last seen,
// and the hot set (every error; a warning only past the configured repeats).
func TestSelfProblems_FactsAndHotDerivation(t *testing.T) {
	k, rep := selfProblemsKernel(t, 0)

	rows, err := k.Query("self_problem")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(rep.Problems) {
		t.Fatalf("self_problem holds %d rows, want %d", len(rows), len(rep.Problems))
	}
	var edit Fact
	for _, f := range rows {
		if strings.HasPrefix(types.ExtractString(f.Args[0]), "Tool call edit_element failed") {
			edit = f
		}
	}
	if types.ExtractString(edit.Args[1]) != "/error" || types.ExtractString(edit.Args[2]) != "session" {
		t.Fatalf("edit_element row = %v", edit)
	}
	if n, _ := types.ExtractInt64(edit.Args[3]); n != 2 {
		t.Fatalf("count = %v, want 2", edit.Args[3])
	}
	if !strings.Contains(types.ExtractString(edit.Args[4]), "saveNorthstar(false)") {
		t.Fatalf("the sample lost its payload lines: %q", types.ExtractString(edit.Args[4]))
	}
	if seen, _ := k.Query("self_problem_seen"); len(seen) != len(rep.Problems) {
		t.Fatalf("self_problem_seen holds %d rows, want %d", len(seen), len(rep.Problems))
	}

	hot := func() map[string]bool {
		rows, err := k.Query("self_problem_hot")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, f := range rows {
			out[types.ExtractString(f.Args[1])+" "+types.ExtractString(f.Args[2])] = true
		}
		return out
	}
	// No warn-repeat threshold in config: only errors are hot.
	h := hot()
	if len(h) != 1 || !h["/error session"] {
		t.Fatalf("hot without a threshold = %v, want the session errors only", h)
	}
	rows, _ = k.Query("self_problem_hot")
	if len(rows) != 3 {
		t.Fatalf("%d hot rows, want the 3 distinct session errors", len(rows))
	}

	// A threshold of 3 admits the thrice-repeated performance warning only.
	if err := k.Assert(Fact{Predicate: "config_param", Args: []any{types.MangleAtom("/logging_self_problem_warn_repeats"), int64(3)}}); err != nil {
		t.Fatal(err)
	}
	rows, _ = k.Query("self_problem_hot")
	if len(rows) != 4 {
		t.Fatalf("%d hot rows with threshold 3, want 3 errors + 1 warning", len(rows))
	}
	if h := hot(); !h["/warn performance"] {
		t.Fatalf("the repeated performance warning is not hot: %v", h)
	}
}

// Hot problems reach the model's context as compact lines, only while the
// chat says debug-self is on.
func TestSelfProblems_InjectsHotLinesOnlyWhileDebugSelfActive(t *testing.T) {
	k, _ := selfProblemsKernel(t, 0)
	if err := k.Assert(Fact{Predicate: "config_param", Args: []any{types.MangleAtom("/logging_self_problem_warn_repeats"), int64(3)}}); err != nil {
		t.Fatal(err)
	}
	if lines := selfProblemLines(t, k); len(lines) != 0 {
		t.Fatalf("injected without debug_self_active: %v", lines)
	}

	if err := k.Assert(Fact{Predicate: problemlog.DebugSelfActivePredicate}); err != nil {
		t.Fatal(err)
	}
	lines := selfProblemLines(t, k)
	if len(lines) != 4 {
		t.Fatalf("injected %d lines, want 4 (3 errors + 1 repeated warning): %v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"Own log ERROR [session] x2: Tool call edit_element failed: modular tool execution failed: old does not occur in cmd<path> Its text is:",
		"Own log ERROR [session] x1: Tool call run_tests failed: modular tool execution failed: exit status #",
		"Own log WARN [performance] x3: Slow Update() handler: chat.bootCompleteMsg took #ms (target: <#ms)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing line %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "saveNorthstar") || strings.Contains(joined, "\n903") {
		t.Errorf("a compact line must not carry the sample payload:\n%s", joined)
	}

	// debug-self off: the lines go with the flag, the evidence stays queryable.
	if err := k.Retract(problemlog.DebugSelfActivePredicate); err != nil {
		t.Fatal(err)
	}
	if lines := selfProblemLines(t, k); len(lines) != 0 {
		t.Fatalf("lines survived debug_self_active retraction: %v", lines)
	}
	if rows, _ := k.Query("self_problem"); len(rows) == 0 {
		t.Fatal("self_problem must stay queryable after the injection gate closes")
	}
}

// A fresh read replaces the last wholesale, and what the cut left out is a
// fact, not silence.
func TestSelfProblems_ReplaceSupersedesAndReportsOmission(t *testing.T) {
	k, rep := selfProblemsKernel(t, 3)
	if rows, _ := k.Query("self_problem"); len(rows) != 3 {
		t.Fatalf("%d problems under a bound of 3", len(rows))
	}
	om, err := k.Query("self_problem_omitted")
	if err != nil || len(om) != 1 {
		t.Fatalf("self_problem_omitted = %v, %v", om, err)
	}
	if p, _ := types.ExtractInt64(om[0].Args[0]); p != int64(rep.OmittedProblems) {
		t.Fatalf("omitted problems = %v, want %d", om[0].Args[0], rep.OmittedProblems)
	}

	full, err := problemlog.ReadFile("../problemlog/testdata/problems_fixture.txt", problemlog.Options{
		Skip: func(e problemlog.Entry) bool { return strings.Contains(e.Message, "debug-self:") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := problemlog.Replace(k, full); err != nil {
		t.Fatal(err)
	}
	if rows, _ := k.Query("self_problem"); len(rows) != len(full.Problems) {
		t.Fatalf("%d problems after replace, want %d (no stale rows, no duplicates)", len(rows), len(full.Problems))
	}
	if om, _ := k.Query("self_problem_omitted"); len(om) != 0 {
		t.Fatalf("a stale omission survived the replace: %v", om)
	}
}
