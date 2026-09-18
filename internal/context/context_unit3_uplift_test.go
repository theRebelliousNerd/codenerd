package context

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// The atom parser is the control packet's front door: numbers must keep their
// type, strings their quotes policy, and malformed atoms must error — never
// panic or silently mis-split.
func TestParseMangleAtom_TypesAndMalformed(t *testing.T) {
	f, err := ParseMangleAtom(`user_intent("i1", "/code", "/fix", "auth.go", "none").`)
	if err != nil {
		t.Fatal(err)
	}
	if f.Predicate != "user_intent" || len(f.Args) != 5 {
		t.Fatalf("parsed = %+v, want user_intent/5", f)
	}
	if f.Args[0] != "i1" || f.Args[1] != "/code" {
		t.Errorf("args = %v, want [i1 /code ...]", f.Args)
	}

	f, err = ParseMangleAtom(`m(42, 1.5, true, /name, "1.5", "123abc").`)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := f.Args[0].(int64); !ok || v != 42 {
		t.Errorf("int arg = %v (%T), want int64(42)", f.Args[0], f.Args[0])
	}
	if v, ok := f.Args[1].(float64); !ok || v != 1.5 {
		t.Errorf("float arg = %v (%T), want float64(1.5)", f.Args[1], f.Args[1])
	}
	if v, ok := f.Args[2].(bool); !ok || !v {
		t.Errorf("bool arg = %v (%T), want true", f.Args[2], f.Args[2])
	}
	// Quoted numerics are strings, not numbers: the quotes decide.
	if v, ok := f.Args[4].(string); !ok || v != "1.5" {
		t.Errorf("quoted float = %v (%T), want string", f.Args[4], f.Args[4])
	}
	if v, ok := f.Args[5].(string); !ok || v != "123abc" {
		t.Errorf("alphanumeric = %v (%T), want the whole string", f.Args[5], f.Args[5])
	}

	// An escaped quote inside a string must not end it early.
	f, err = ParseMangleAtom(`p("a\"b", "c").`)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Args) != 2 || f.Args[0] != `a"b` || f.Args[1] != "c" {
		t.Errorf("escaped args = %v, want [a\"b c]", f.Args)
	}
	// Nested structure survives as one argument.
	if f, err = ParseMangleAtom(`p(f(1, 2), x).`); err != nil || len(f.Args) != 2 {
		t.Errorf("nested = %+v, %v; want 2 args, nil", f, err)
	}

	for _, bad := range []string{
		`no_parens.`,
		`(empty_pred).`,
		`p("unterminated).`,
		`p(a)), b).`,
		`p(").`,
		`"`,
	} {
		if _, err := ParseMangleAtom(bad); err == nil {
			t.Errorf("input %q: expected an error, got nil", bad)
		}
	}
}

// Truncation must never split a rune: the context block goes straight to the
// model, and invalid UTF-8 there is corruption, not compression.
func TestTruncateFact_RuneSafe(t *testing.T) {
	fs := NewFactSerializer()
	f := core.Fact{Predicate: "p", Args: []any{strings.Repeat("界", 100)}}
	out := fs.truncateFact(f)
	if !utf8.ValidString(out) {
		t.Errorf("truncated fact is invalid UTF-8: %q", out)
	}
	if !types.IsClamped(out) {
		t.Errorf("long arg was cut with no marker: %q", out)
	}
	// The argument body is bounded at maxFactArgChars runes; the marker that
	// follows it is not part of the body and is the point of the exercise.
	body, _, found := strings.Cut(out, " [codenerd:")
	if !found {
		t.Fatalf("no marker to measure the body against: %q", out)
	}
	if got := len([]rune(body)); got > maxFactArgChars+10 {
		t.Errorf("long arg was not truncated to ~%d runes: body is %d runes in %q",
			maxFactArgChars, got, out)
	}
	if got := NewFactSerializer().SerializeCompressedContext(nil); got != "" {
		t.Errorf("SerializeCompressedContext(nil) = %q, want empty", got)
	}
}

// LoadState refuses what it cannot restore, and neither direction of the
// state round-trip aliases engine memory.
func TestCompressor_LoadStateGuardsAndIsolation(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	if err := comp.LoadState(nil); err == nil {
		t.Error("LoadState(nil): expected an error, got nil")
	}
	if err := (&Compressor{}).LoadState(&CompressedState{}); err == nil {
		t.Error("LoadState on a zero compressor: expected an error, got nil")
	}

	intent := fact("user_intent", "i", "/c", "/fix", "a.go", "")
	state := &CompressedState{
		SessionID:  "s1",
		TurnNumber: 3,
		RecentTurns: []CompressedTurn{{
			TurnNumber: 3, Role: "user", IntentAtom: &intent,
			ResultAtoms: []core.Fact{fact("diagnostic", "boom")},
		}},
		RollingSummary: RollingSummary{Segments: []HistorySegment{{
			ID: "seg", StartTurn: 1, EndTurn: 2,
			KeyAtoms: []core.Fact{fact("k", "v")},
		}}},
	}
	if err := comp.LoadState(state); err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	// Mutate the caller's copy after the load: the engine must not follow.
	state.RecentTurns[0].Role = "mutated"
	state.RecentTurns[0].IntentAtom.Args[2] = "mutated"
	state.RecentTurns[0].ResultAtoms[0].Args[0] = "mutated"
	state.RollingSummary.Segments[0].KeyAtoms[0].Args[0] = "mutated"
	got := comp.GetState()
	if got.RecentTurns[0].Role != "user" || got.RecentTurns[0].IntentAtom.Args[2] != "/fix" {
		t.Errorf("LoadState aliased the caller's turns: %+v", got.RecentTurns[0])
	}
	if got.RecentTurns[0].ResultAtoms[0].Args[0] != "boom" {
		t.Errorf("LoadState aliased the caller's result atoms: %+v", got.RecentTurns[0].ResultAtoms)
	}
	if got.RollingSummary.Segments[0].KeyAtoms[0].Args[0] != "v" {
		t.Errorf("LoadState aliased the caller's key atoms: %+v", got.RollingSummary.Segments[0].KeyAtoms)
	}
	// Mutate the snapshot: the engine must not follow either.
	got.RecentTurns[0].Role = "mutated"
	again := comp.GetState()
	if again.RecentTurns[0].Role != "user" {
		t.Errorf("GetState aliased engine memory: %+v", again.RecentTurns[0])
	}
}

// A missing kernel is an error from BuildContext, and a hostile window
// configuration must not panic any turn or build path.
func TestCompressor_NilKernelAndNegativeWindow(t *testing.T) {
	bare := NewCompressor(nil, nil, nil)
	if _, err := bare.BuildContext(context.Background()); err == nil {
		t.Error("BuildContext without a kernel: expected an error, got nil")
	}

	comp := newKernelBackedCompressor(t)
	comp.config.RecentTurnWindow = -5
	if _, err := comp.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: "hi"}); err != nil {
		t.Fatalf("ProcessTurn with negative window: %v", err)
	}
	if _, err := comp.BuildContext(context.Background()); err != nil {
		t.Fatalf("BuildContext with negative window: %v", err)
	}
	if comp.recentWindow() != 0 {
		t.Errorf("recentWindow() = %d, want 0", comp.recentWindow())
	}
}

// Feedback written outside [0,1] is clamped at the door, and blank predicate
// names never become phantom aggregation rows.
func TestFeedbackStore_ClampsAndSkipsBlanks(t *testing.T) {
	store, err := NewContextFeedbackStore(filepath.Join(t.TempDir(), "fb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.StoreFeedback(1, "", 5.0, "/fix", true, []string{"p", ""}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreFeedback(2, "", -2.0, "/fix", true, nil, []string{""}); err != nil {
		t.Fatal(err)
	}
	total, avg, err := store.GetOverallStats()
	if err != nil || total != 2 {
		t.Fatalf("stats = %d, %v, %v; want 2 rows", total, avg, err)
	}
	if avg < 0 || avg > 1 {
		t.Errorf("avg usefulness = %v after clamped writes, want within [0,1]", avg)
	}
	top, err := store.GetTopHelpfulPredicates(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, pf := range top {
		if pf.Predicate == "" {
			t.Error("blank predicate aggregated into the helpful table")
		}
	}
	if phantom, err := store.GetPredicateFeedback(""); err != nil || phantom != nil {
		t.Errorf("blank predicate lookup = %+v, %v; want nil, nil (no phantom row)", phantom, err)
	}
}

// A turn ID that is not a number is absent, not turn 0: the back-reference
// refresh must skip it instead of inventing a referenced turn. A numeric
// string counts as its number — the schema declares these slots /string.
func TestBackReference_SkipsNonNumericTurnIDs(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	byPred := map[string][]core.Fact{
		// int IDs like the session recorder emits, plus one bogus value.
		"turn_references_back": {
			fact("turn_references_back", 2, 1),
			fact("turn_references_back", 3, "bogus"),
			fact("turn_references_back", 4, "7"),
		},
		"turn_topic": {fact("turn_topic", 1, "auth")},
	}
	comp.refreshBackReferenceContextLocked(func(pred string) []core.Fact { return byPred[pred] })
	comp.activation.mu.RLock()
	defer comp.activation.mu.RUnlock()
	ctx := comp.activation.backReferenceContext
	if ctx == nil {
		t.Fatal("expected a back-reference context, got nil")
	}
	for _, id := range ctx.ReferencedTurnIDs {
		if id == 0 {
			t.Errorf("referenced turns %v contain invented turn 0", ctx.ReferencedTurnIDs)
		}
	}
	has := func(want int) bool {
		for _, id := range ctx.ReferencedTurnIDs {
			if id == want {
				return true
			}
		}
		return false
	}
	if !has(1) || !has(7) {
		t.Errorf("referenced turns = %v, want [1 7] (bogus skipped, numeric string kept)", ctx.ReferencedTurnIDs)
	}
}

// The shared fact-argument helpers: bounds-checked, type-tolerant reads.
func TestFactArgHelpers(t *testing.T) {
	f := fact("p", "s", int64(7))
	if _, ok := factArgAt(f, 5); ok {
		t.Error("factArgAt beyond the end: expected false, got true")
	}
	if s, ok := factStringAt(f, 0); !ok || s != "s" {
		t.Errorf("factStringAt(0) = %q, %v; want s, true", s, ok)
	}
	if _, ok := factStringAt(f, 9); ok {
		t.Error("factStringAt beyond the end: expected false, got true")
	}
	if n, ok := factTurnIDAt(f, 1); !ok || n != 7 {
		t.Errorf("factTurnIDAt(1) = %d, %v; want 7, true", n, ok)
	}
	if _, ok := factTurnIDAt(f, 0); ok {
		t.Error("factTurnIDAt over a non-numeric string: expected false, got true")
	}
	g := fact("p", "42")
	if n, ok := factTurnIDAt(g, 0); !ok || n != 42 {
		t.Errorf("factTurnIDAt over \"42\" = %d, %v; want 42, true", n, ok)
	}
	// A lone quote must not panic the value parser.
	if v := parseArgValue(`"`); v != `"` {
		t.Errorf("parseArgValue lone quote = %v (%T), want the string itself", v, v)
	}
}
