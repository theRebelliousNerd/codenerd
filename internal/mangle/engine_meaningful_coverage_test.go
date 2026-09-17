package mangle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func newMeaningfulEngine(t *testing.T, schema string) *Engine {
	t.Helper()
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if schema != "" {
		if err := eng.LoadSchemaString(schema); err != nil {
			t.Fatalf("LoadSchemaString(%q) error = %v", schema, err)
		}
	}
	return eng
}

func meaningfulStrings(t *testing.T, eng *Engine, pred string) []string {
	t.Helper()
	facts, err := eng.GetFacts(pred)
	if err != nil {
		t.Fatalf("GetFacts(%q) error = %v", pred, err)
	}
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		out = append(out, fmt.Sprint(f.Args...))
	}
	return out
}

func TestMeaningful_ClearKeepsSchema_ResetDropsIt(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl item(X).`)
	ctx := context.Background()

	if err := eng.AddFact("item", "a"); err != nil {
		t.Fatalf("AddFact before Clear error = %v", err)
	}
	eng.Clear()

	// Schema must survive Clear: predicate still declared, store empty.
	facts, err := eng.GetFacts("item")
	if err != nil {
		t.Fatalf("GetFacts after Clear should not error (schema kept), got %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("GetFacts after Clear = %d facts, want 0", len(facts))
	}
	if _, err := eng.Query(ctx, "item(X)"); err != nil {
		t.Fatalf("Query after Clear should not error (schema kept), got %v", err)
	}
	if err := eng.AddFact("item", "b"); err != nil {
		t.Fatalf("AddFact after Clear should succeed (schema kept), got %v", err)
	}

	// Reset must drop everything: next use fails closed.
	eng.Reset()
	if _, err := eng.GetFacts("item"); err == nil {
		t.Fatalf("GetFacts after Reset should error (schema dropped), got nil")
	}
	if _, err := eng.Query(ctx, "item(X)"); err == nil {
		t.Fatalf("Query after Reset should error (schema dropped), got nil")
	}
	if err := eng.AddFact("item", "c"); err == nil {
		t.Fatalf("AddFact after Reset should error (schema dropped), got nil")
	}
}

func TestMeaningful_ReplaceFactsForFile_ReplacesNotAccumulates(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl file_dep(File, Dep).`)

	seed := []Fact{
		{Predicate: "file_dep", Args: []any{"src/a.go", "src/dep1.go"}},
		{Predicate: "file_dep", Args: []any{"src/a.go", "src/dep2.go"}},
		{Predicate: "file_dep", Args: []any{"src/b.go", "src/dep3.go"}},
	}
	if err := eng.AddFacts(seed); err != nil {
		t.Fatalf("AddFacts seed error = %v", err)
	}
	replacement := []Fact{
		{Predicate: "file_dep", Args: []any{"src/a.go", "src/dep-new.go"}},
	}
	if err := eng.ReplaceFactsForFile("src/a.go", replacement); err != nil {
		t.Fatalf("ReplaceFactsForFile error = %v", err)
	}
	facts, err := eng.GetFacts("file_dep")
	if err != nil {
		t.Fatalf("GetFacts error = %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("GetFacts after replace = %d facts, want 2 (1 replaced + 1 untouched)", len(facts))
	}
	joined := ""
	for _, f := range facts {
		joined += fmt.Sprintf("%v;", f.Args)
	}
	if strings.Contains(joined, "dep1") || strings.Contains(joined, "dep2") {
		t.Fatalf("stale deps survived ReplaceFactsForFile: %s", joined)
	}
	if !strings.Contains(joined, "dep-new") {
		t.Fatalf("replacement dep missing after ReplaceFactsForFile: %s", joined)
	}
	if !strings.Contains(joined, "dep3") {
		t.Fatalf("untouched file fact lost after ReplaceFactsForFile: %s", joined)
	}
}

func TestMeaningful_ReplaceFactsForFile_CanonicalizesPaths(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl fdep(File, Dep).`)
	if err := eng.AddFact("fdep", "src/a.go", "src/d1.go"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	// "src/./a.go" cleans to "src/a.go" and must evict the same reverse index.
	if err := eng.ReplaceFactsForFile("src/./a.go", []Fact{
		{Predicate: "fdep", Args: []any{"src/a.go", "src/d2.go"}},
	}); err != nil {
		t.Fatalf("ReplaceFactsForFile canonical error = %v", err)
	}
	facts, err := eng.GetFacts("fdep")
	if err != nil {
		t.Fatalf("GetFacts error = %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("canonical replace left %d facts, want 1", len(facts))
	}
	if got := fmt.Sprint(facts[0].Args); !strings.Contains(got, "d2") {
		t.Fatalf("canonical replace kept stale fact: %v", facts[0].Args)
	}
}

func TestMeaningful_Query_RepeatedVariableKeepsDiagonal(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl rpair(X, Y).`)
	ctx := context.Background()
	for _, ab := range [][2]string{{"a", "a"}, {"a", "b"}, {"b", "b"}} {
		if err := eng.AddFact("rpair", ab[0], ab[1]); err != nil {
			t.Fatalf("AddFact(%v) error = %v", ab, err)
		}
	}

	diag, err := eng.Query(ctx, "rpair(X, X)")
	if err != nil {
		t.Fatalf("Query diagonal error = %v", err)
	}
	if len(diag.Bindings) != 2 {
		t.Fatalf("Query rpair(X,X) = %d rows, want 2 diagonal only", len(diag.Bindings))
	}
	all, err := eng.Query(ctx, "rpair(X, Y)")
	if err != nil {
		t.Fatalf("Query full error = %v", err)
	}
	if len(all.Bindings) != 3 {
		t.Fatalf("Query rpair(X,Y) = %d rows, want 3", len(all.Bindings))
	}
	wild, err := eng.Query(ctx, "rpair(_, _)")
	if err != nil {
		t.Fatalf("Query wildcard error = %v", err)
	}
	if len(wild.Bindings) != 3 {
		t.Fatalf("Query rpair(_,_) = %d rows, want 3 (wildcard binds independently)", len(wild.Bindings))
	}
}

func TestMeaningful_Query_ConstantFiltersRows(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl npair(X, Y).`)
	ctx := context.Background()
	for _, ab := range [][2]int{{1, 2}, {1, 3}, {2, 3}} {
		if err := eng.AddFact("npair", ab[0], ab[1]); err != nil {
			t.Fatalf("AddFact(%v) error = %v", ab, err)
		}
	}

	got, err := eng.Query(ctx, "npair(1, Y)")
	if err != nil {
		t.Fatalf("Query with constant error = %v", err)
	}
	if len(got.Bindings) != 2 {
		t.Fatalf("Query npair(1,Y) = %d rows, want 2 (constant must filter)", len(got.Bindings))
	}
	seen := map[string]bool{}
	for _, row := range got.Bindings {
		seen[fmt.Sprint(row["Y"])] = true
	}
	if !seen["2"] || !seen["3"] {
		t.Fatalf("Query npair(1,Y) bindings = %v, want Y in {2,3}", got.Bindings)
	}

	none, err := eng.Query(ctx, "npair(99, Y)")
	if err != nil {
		t.Fatalf("Query no-match error = %v", err)
	}
	if len(none.Bindings) != 0 {
		t.Fatalf("Query npair(99,Y) = %d rows, want 0", len(none.Bindings))
	}

	// Fully ground query: 1 row on match, 0 on mismatch.
	groundHit, err := eng.Query(ctx, "npair(1, 2)")
	if err != nil {
		t.Fatalf("Query ground hit error = %v", err)
	}
	if len(groundHit.Bindings) != 1 {
		t.Fatalf("Query npair(1,2) = %d rows, want 1", len(groundHit.Bindings))
	}
	groundMiss, err := eng.Query(ctx, "npair(1, 99)")
	if err != nil {
		t.Fatalf("Query ground miss error = %v", err)
	}
	if len(groundMiss.Bindings) != 0 {
		t.Fatalf("Query npair(1,99) = %d rows, want 0", len(groundMiss.Bindings))
	}
}

func TestMeaningful_UndeclaredPredicate_FailsClosed(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl known(X).`)
	ctx := context.Background()

	if _, err := eng.Query(ctx, "unknown_pred_xyz(X)"); err == nil {
		t.Fatalf("Query unknown predicate should error, got nil")
	} else if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("Query unknown error = %q, want 'not declared'", err)
	}
	if _, err := eng.GetFacts("unknown_pred_xyz"); err == nil {
		t.Fatalf("GetFacts unknown predicate should error, got nil")
	} else if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("GetFacts unknown error = %q, want 'not declared'", err)
	}
	if err := eng.AddFact("unknown_pred_xyz", "x"); err == nil {
		t.Fatalf("AddFact unknown predicate should error, got nil")
	} else if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("AddFact unknown error = %q, want 'not declared'", err)
	}
	if err := eng.ReplaceControlFacts(nil, "unknown_pred_xyz"); err == nil {
		t.Fatalf("ReplaceControlFacts unknown predicate should error, got nil")
	} else if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("ReplaceControlFacts unknown error = %q, want 'not declared'", err)
	}
	// QueryFacts is the documented exception: undeclared yields nil, not an error.
	if got := eng.QueryFacts("unknown_pred_xyz"); len(got) != 0 {
		t.Fatalf("QueryFacts unknown = %d facts, want 0 (nil, no panic)", len(got))
	}
}

func TestMeaningful_AddFacts_MonotonicKeepsPriorOnError(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl m(X).`)
	batch := []Fact{
		{Predicate: "m", Args: []any{"ok-1"}},
		{Predicate: "m", Args: []any{"too", "many"}},
	}
	if err := eng.AddFacts(batch); err == nil {
		t.Fatalf("AddFacts with bad arity should error, got nil")
	}
	// Facts are monotonic: the good prefix stays, there is no rollback.
	facts, err := eng.GetFacts("m")
	if err != nil {
		t.Fatalf("GetFacts error = %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("GetFacts after partial batch = %d, want 1 (good prefix kept)", len(facts))
	}
}

func TestMeaningful_AddFacts_EmptyBatchIsNoop(t *testing.T) {
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine error = %v", err)
	}
	// Empty batch returns before the schema check: nil even with no schemas.
	if err := eng.AddFacts(nil); err != nil {
		t.Fatalf("AddFacts(nil) without schema = %v, want nil", err)
	}
	if err := eng.AddFacts([]Fact{}); err != nil {
		t.Fatalf("AddFacts(empty) without schema = %v, want nil", err)
	}
}

func TestMeaningful_AddFactsContext_CancelledLeavesStoreEmpty(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl c(X).`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := eng.AddFactsContext(ctx, []Fact{{Predicate: "c", Args: []any{"v-1"}}})
	if err == nil {
		t.Fatalf("AddFactsContext with cancelled ctx should error, got nil")
	}
	facts, ferr := eng.GetFacts("c")
	if ferr != nil {
		t.Fatalf("GetFacts error = %v", ferr)
	}
	if len(facts) != 0 {
		t.Fatalf("cancelled batch inserted %d facts, want 0", len(facts))
	}
	// Empty batch with a cancelled context is still a no-op success.
	if err := eng.AddFactsContext(ctx, nil); err != nil {
		t.Fatalf("AddFactsContext(nil) with cancelled ctx = %v, want nil", err)
	}
}

func TestMeaningful_ParseQueryShape_Table(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantVars  int
		firstName string
		firstIdx  int
	}{
		{name: "empty fails", input: "", wantErr: true},
		{name: "lone question fails", input: "?", wantErr: true},
		{name: "blank fails", input: "   ", wantErr: true},
		{name: "single var", input: "pair(X)", wantVars: 1, firstName: "X", firstIdx: 0},
		{name: "question and period stripped", input: "? pair(X).", wantVars: 1, firstName: "X", firstIdx: 0},
		{name: "repeated var kept twice", input: "pair(X, X)", wantVars: 2, firstName: "X", firstIdx: 0},
		{name: "constant plus var", input: `pair("hi-there", Y)`, wantVars: 1, firstName: "Y", firstIdx: 1},
		{name: "garbage fails", input: "not a query (((", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape, err := parseQueryShape(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseQueryShape(%q) should error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQueryShape(%q) error = %v", tt.input, err)
			}
			if len(shape.variables) != tt.wantVars {
				t.Fatalf("parseQueryShape(%q) vars = %d, want %d", tt.input, len(shape.variables), tt.wantVars)
			}
			if tt.wantVars > 0 {
				if shape.variables[0].Name != tt.firstName || shape.variables[0].Index != tt.firstIdx {
					t.Fatalf("first var = %+v, want {Name:%s Index:%d}", shape.variables[0], tt.firstName, tt.firstIdx)
				}
			}
		})
	}
}

func TestMeaningful_IsIdentifier_Table(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"a", true},
		{"abc", true},
		{"a1", true},
		{"hello_world2", true},
		{"_", true},
		{"_x", true},
		{"Aabc", false},
		{"9abc", false},
		{"a-b", false},
		{"a/b", false},
		{"a.b", false},
		{"a b", false},
		{"src/a.go", false},
	}
	for _, tt := range tests {
		t.Run("id_"+tt.input, func(t *testing.T) {
			if got := isIdentifier(tt.input); got != tt.want {
				t.Fatalf("isIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeaningful_CanonicalPath_Table(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"src/a.go", "src/a.go"},
		{"src/./a.go", "src/a.go"},
		{"src//a.go", "src/a.go"},
		{"src/b/../a.go", "src/a.go"},
		{`src\a.go`, "src/a.go"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := canonicalPath(tt.input); got != tt.want {
				t.Fatalf("canonicalPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeaningful_QueryFacts_SlashTolerantFiltering(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl qp(X).`)
	// "alpha" is identifier-like, so it is stored as the atom /alpha.
	if err := eng.AddFact("qp", "alpha"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	if got := eng.QueryFacts("qp", "alpha"); len(got) != 1 {
		t.Fatalf("QueryFacts(qp, alpha) = %d, want 1 (slash-tolerant)", len(got))
	}
	if got := eng.QueryFacts("qp", "/alpha"); len(got) != 1 {
		t.Fatalf("QueryFacts(qp, /alpha) = %d, want 1", len(got))
	}
	if got := eng.QueryFacts("qp", ""); len(got) != 1 {
		t.Fatalf("QueryFacts(qp, empty) = %d, want 1 (empty is wildcard)", len(got))
	}
	if got := eng.QueryFacts("qp", "nomatch-1"); len(got) != 0 {
		t.Fatalf("QueryFacts(qp, nomatch) = %d, want 0", len(got))
	}
}

func TestMeaningful_GetFactsSeq_EvaluateRule_Behavior(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl seqp(X).`)
	for _, v := range []string{"s-1", "s-2"} {
		if err := eng.AddFact("seqp", v); err != nil {
			t.Fatalf("AddFact error = %v", err)
		}
	}
	countSeq := 0
	for range eng.GetFactsSeq("seqp") {
		countSeq++
	}
	if countSeq != 2 {
		t.Fatalf("GetFactsSeq(seqp) yielded %d, want 2", countSeq)
	}
	countRule := 0
	for range eng.EvaluateRule("seqp") {
		countRule++
	}
	if countRule != 2 {
		t.Fatalf("EvaluateRule(seqp) yielded %d, want 2", countRule)
	}
	countMissing := 0
	for range eng.GetFactsSeq("missing_xyz") {
		countMissing++
	}
	if countMissing != 0 {
		t.Fatalf("GetFactsSeq(missing) yielded %d, want 0", countMissing)
	}
	// Early break must not hang or panic.
	seen := 0
	for range eng.GetFactsSeq("seqp") {
		seen++
		break
	}
	if seen != 1 {
		t.Fatalf("early break yielded %d, want 1", seen)
	}
}

func TestMeaningful_FactLimit_Enforced(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FactLimit = 2
	cfg.AutoEval = false
	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine error = %v", err)
	}
	if err := eng.LoadSchemaString(`Decl lim(X).`); err != nil {
		t.Fatalf("LoadSchemaString error = %v", err)
	}
	if err := eng.AddFact("lim", "v-1"); err != nil {
		t.Fatalf("AddFact 1 error = %v", err)
	}
	if err := eng.AddFact("lim", "v-2"); err != nil {
		t.Fatalf("AddFact 2 error = %v", err)
	}
	if err := eng.AddFact("lim", "v-3"); err == nil {
		t.Fatalf("AddFact over limit should error, got nil")
	} else if !strings.Contains(err.Error(), "fact limit") {
		t.Fatalf("over-limit error = %q, want 'fact limit'", err)
	}
	facts, ferr := eng.GetFacts("lim")
	if ferr != nil {
		t.Fatalf("GetFacts error = %v", ferr)
	}
	if len(facts) != 2 {
		t.Fatalf("GetFacts after rejected insert = %d, want 2 (prior kept)", len(facts))
	}
}

func TestMeaningful_LoadSchema_BadInputFailsClosed(t *testing.T) {
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine error = %v", err)
	}
	if err := eng.LoadSchema("/definitely/not/here-meaningful.mg"); err == nil {
		t.Fatalf("LoadSchema missing file should error, got nil")
	}
	if err := eng.LoadSchemaString("Decl (broken"); err == nil {
		t.Fatalf("LoadSchemaString garbage should error, got nil")
	}
	// Failed loads must not install a half schema.
	if _, err := eng.Query(context.Background(), "anything(X)"); err == nil {
		t.Fatalf("Query after failed loads should error, got nil")
	}

	// A bad fragment must not poison a previously good schema.
	good := newMeaningfulEngine(t, `Decl keepp(X).`)
	if err := good.AddFact("keepp", "v-1"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	if err := good.LoadSchemaString("this is not mangle ((("); err == nil {
		t.Fatalf("bad fragment should error, got nil")
	}
	if _, err := good.GetFacts("keepp"); err != nil {
		t.Fatalf("good schema broken by bad fragment: %v", err)
	}
	if err := good.AddFact("keepp", "v-2"); err != nil {
		t.Fatalf("AddFact after rejected fragment error = %v", err)
	}
}

func TestMeaningful_ReplaceControlFacts_ClearsAndReplaces(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl ctrl(X).`)
	if err := eng.AddFact("ctrl", "alpha"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	if err := eng.AddFact("ctrl", "beta"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	// Empty replacement with the predicate named must clear, not accumulate.
	if err := eng.ReplaceControlFacts(nil, "ctrl"); err != nil {
		t.Fatalf("ReplaceControlFacts clear error = %v", err)
	}
	if got := meaningfulStrings(t, eng, "ctrl"); len(got) != 0 {
		t.Fatalf("after clear got %v, want empty", got)
	}
	if err := eng.ReplaceControlFacts([]Fact{{Predicate: "ctrl", Args: []any{"gamma"}}}, "ctrl"); err != nil {
		t.Fatalf("ReplaceControlFacts replace error = %v", err)
	}
	got := meaningfulStrings(t, eng, "ctrl")
	if len(got) != 1 || !strings.Contains(got[0], "gamma") {
		t.Fatalf("after replace got %v, want [gamma]", got)
	}
}

func TestMeaningful_GetStats_CountsFacts(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl statp(X).`)
	if err := eng.AddFact("statp", "v-1"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	if err := eng.AddFact("statp", "v-2"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	stats := eng.GetStats()
	if stats.TotalFacts < 2 {
		t.Fatalf("TotalFacts = %d, want >= 2", stats.TotalFacts)
	}
	if stats.PredicateCounts["statp"] != 2 {
		t.Fatalf("PredicateCounts[statp] = %d, want 2", stats.PredicateCounts["statp"])
	}
}

type meaningfulStringer struct{ s string }

func (m meaningfulStringer) String() string { return m.s }

func TestMeaningful_ConvertValue_BranchesViaEngine(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl cv(X).`)

	// Fail-closed inputs: honest errors, never silent wrong values.
	if err := eng.AddFact("cv", nil); err == nil {
		t.Fatalf("AddFact(nil) should error, got nil")
	}
	if err := eng.AddFact("cv", []any{42}); err == nil {
		t.Fatalf("AddFact([]any{int}) should error (only strings), got nil")
	} else if !strings.Contains(err.Error(), "only strings supported") {
		t.Fatalf("list-element error = %q, want 'only strings supported'", err)
	}
	badMap := map[string]any{"f": func() {}}
	if err := eng.AddFact("cv", badMap); err == nil {
		t.Fatalf("AddFact(unmarshalable map) should error, got nil")
	}

	// Success branches: every family below must insert without error.
	okArgs := []any{
		42,
		int64(7),
		int32(8),
		3.14,
		float32(1.5),
		true,
		false,
		"plain-value-1",
		"alpha",
		"/explicit-atom",
		types.MangleAtom("myatom"),
		types.MangleString("my string"),
		meaningfulStringer{s: "stringer-value-1"},
		[]string{"a-1", "b-2"},
		[]any{"x-1", "y-2"},
		map[string]string{"k-1": "v-1"},
		map[string]any{"k-1": "v-1"},
	}
	for i, arg := range okArgs {
		eng2 := newMeaningfulEngine(t, `Decl cv2(X).`)
		_ = i
		if err := eng2.AddFact("cv2", arg); err != nil {
			t.Fatalf("AddFact(cv2, %T %v) error = %v", arg, arg, err)
		}
		facts, ferr := eng2.GetFacts("cv2")
		if ferr != nil || len(facts) != 1 {
			t.Fatalf("GetFacts after %T = %d facts err %v, want 1", arg, len(facts), ferr)
		}
	}

	// Atom branch pins the encoding: identifier strings become /atoms.
	engA := newMeaningfulEngine(t, `Decl at(X).`)
	if err := engA.AddFact("at", "alpha"); err != nil {
		t.Fatalf("AddFact alpha error = %v", err)
	}
	facts, _ := engA.GetFacts("at")
	if len(facts) != 1 || fmt.Sprint(facts[0].Args[0]) != "/alpha" {
		t.Fatalf("identifier round-trip = %v, want [/alpha]", facts)
	}
	engS := newMeaningfulEngine(t, `Decl st(X).`)
	if err := engS.AddFact("st", types.MangleString("hello")); err != nil {
		t.Fatalf("AddFact MangleString error = %v", err)
	}
	sfacts, _ := engS.GetFacts("st")
	if len(sfacts) != 1 || fmt.Sprint(sfacts[0].Args[0]) != "hello" {
		t.Fatalf("MangleString round-trip = %v, want [hello]", sfacts)
	}
}

func TestMeaningful_FactString_RendersHonestly(t *testing.T) {
	f := Fact{Predicate: "p", Args: []any{
		types.MangleAtom("/yes"),
		types.MangleString("hi"),
		"plain",
		"/atom",
		42,
		true,
		false,
	}}
	s := f.String()
	for _, want := range []string{"p(", "/yes", `"hi"`, `"plain"`, "/atom", "42", "/true", "/false"} {
		if !strings.Contains(s, want) {
			t.Fatalf("Fact.String() = %q, missing %q", s, want)
		}
	}
}

func TestMeaningful_Query_EmptyAndMalformedFails(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl eq(X).`)
	ctx := context.Background()
	for _, q := range []string{"", "?", "   ", "not a query ((("} {
		if _, err := eng.Query(ctx, q); err == nil {
			t.Fatalf("Query(%q) should error, got nil", q)
		}
	}
}

func TestMeaningful_Evaluate_SchemaLifecycle(t *testing.T) {
	bare, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine error = %v", err)
	}
	if err := bare.Evaluate(); err == nil {
		t.Fatalf("Evaluate without schema should error, got nil")
	}
	eng := newMeaningfulEngine(t, `Decl ev(X).`)
	if err := eng.AddFact("ev", "v-1"); err != nil {
		t.Fatalf("AddFact error = %v", err)
	}
	if err := eng.Evaluate(); err != nil {
		t.Fatalf("Evaluate with schema error = %v", err)
	}
	if got := meaningfulStrings(t, eng, "ev"); len(got) != 1 {
		t.Fatalf("Evaluate wiped facts: got %v, want 1", got)
	}
}

func TestMeaningful_MiscWiring_PushToggleHashWarm(t *testing.T) {
	eng := newMeaningfulEngine(t, `Decl mp(X).`)
	if err := eng.PushFact("mp", "v-1"); err != nil {
		t.Fatalf("PushFact error = %v", err)
	}
	eng.ToggleAutoEval(false)
	if err := eng.AddFact("mp", "v-2"); err != nil {
		t.Fatalf("AddFact with autoeval off error = %v", err)
	}
	eng.ToggleAutoEval(true)
	if err := eng.RecomputeRules(); err != nil {
		t.Fatalf("RecomputeRules error = %v", err)
	}
	if got := meaningfulStrings(t, eng, "mp"); len(got) != 2 {
		t.Fatalf("wiring facts = %v, want 2", got)
	}
	if err := eng.ReplaceFactsForFileWithHash("mp-file", []Fact{
		{Predicate: "mp", Args: []any{"mp-file"}},
	}, "hash-1"); err != nil {
		t.Fatalf("ReplaceFactsForFileWithHash error = %v", err)
	}
	if err := eng.WarmFromPersistence(context.Background()); err != nil {
		t.Fatalf("WarmFromPersistence(nil store) = %v, want nil", err)
	}
}
