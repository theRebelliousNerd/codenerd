package prompt

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// orderKernel records the interleaving of kernel writes and reads so the
// phased runner can prove both fact sets land before either query runs.
type orderKernel struct {
	*mockKernel
	mu     sync.Mutex
	events []string
}

func (k *orderKernel) AssertBatch(facts []any) error {
	err := k.mockKernel.AssertBatch(facts)
	k.mu.Lock()
	k.events = append(k.events, "assert")
	k.mu.Unlock()
	return err
}

func (k *orderKernel) Query(predicate string) ([]Fact, error) {
	k.mu.Lock()
	k.events = append(k.events, "query:"+predicate)
	k.mu.Unlock()
	return k.mockKernel.Query(predicate)
}

func selectionFixture() ([]*PromptAtom, *CompilationContext) {
	atoms := []*PromptAtom{
		{ID: "identity/core", Category: CategoryIdentity, Content: "core", Priority: 100, IsMandatory: true},
		{ID: "protocol/base", Category: CategoryProtocol, Content: "ev", Priority: 90, IsMandatory: true},
		{ID: "context/a", Category: CategoryContext, Content: "a", Priority: 50},
		{ID: "context/b", Category: CategoryContext, Content: "b", Priority: 50},
	}
	cc := NewCompilationContext()
	cc.ShardType = "/coder"
	return atoms, cc
}

// Both phases' facts must land before either query runs: the conflict and
// dependency rules are union-sensitive, and a flesh query that runs ahead of
// the skeleton assert decides on a partial world.
func TestSelection_AssertsBeforeQueries(t *testing.T) {
	atoms, cc := selectionFixture()
	inner := &mockKernel{facts: []any{
		Fact{Predicate: "selected_result", Args: []any{"identity/core", 100, "skeleton"}},
		Fact{Predicate: "selected_result", Args: []any{"protocol/base", 90, "skeleton"}},
		Fact{Predicate: "selected_result", Args: []any{"context/a", 50, "flesh"}},
	}}
	k := &orderKernel{mockKernel: inner}
	sel := NewAtomSelector()
	sel.SetKernel(k)
	if _, err := sel.SelectAtoms(context.Background(), atoms, cc); err != nil {
		t.Fatal(err)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.events) < 3 {
		t.Fatalf("events = %v, want 2 asserts then queries", k.events)
	}
	if k.events[0] != "assert" || k.events[1] != "assert" {
		t.Fatalf("first query ran before both asserts landed: %v", k.events)
	}
	for _, e := range k.events[2:] {
		if !strings.HasPrefix(e, "query:") {
			t.Fatalf("non-query event after asserts: %v", k.events)
		}
	}
}

// The same corpus in any input order compiles to the same merged sequence:
// ties break on atom ID, never on arrival order.
func TestSelection_DeterministicAcrossInputOrders(t *testing.T) {
	atoms, cc := selectionFixture()
	run := func(order []*PromptAtom) string {
		inner := &mockKernel{facts: []any{
			Fact{Predicate: "selected_result", Args: []any{"identity/core", 100, "skeleton"}},
			Fact{Predicate: "selected_result", Args: []any{"protocol/base", 90, "skeleton"}},
			Fact{Predicate: "selected_result", Args: []any{"context/a", 50, "flesh"}},
			Fact{Predicate: "selected_result", Args: []any{"context/b", 50, "flesh"}},
		}}
		sel := NewAtomSelector()
		sel.SetKernel(inner)
		merged, err := sel.SelectAtoms(context.Background(), order, cc)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, m := range merged {
			ids = append(ids, m.Atom.ID)
		}
		return strings.Join(ids, ",")
	}
	want := run(atoms)
	rev := []*PromptAtom{atoms[3], atoms[2], atoms[1], atoms[0]}
	if got := run(rev); got != want {
		t.Fatalf("reversed input selected %q, want %q", got, want)
	}
	for i := 0; i < 20; i++ {
		if got := run(atoms); got != want {
			t.Fatalf("run %d selected %q, want %q", i, got, want)
		}
	}
}

func TestMergeAtoms_TiebreaksOnID(t *testing.T) {
	sel := NewAtomSelector()
	mk := func(id string, src string, combined float64) *ScoredAtom {
		return &ScoredAtom{Atom: &PromptAtom{ID: id, Category: CategoryContext}, Combined: combined, Source: src}
	}
	merged := sel.mergeAtoms(
		[]*ScoredAtom{mk("skeleton/z", "skeleton", 1), mk("skeleton/a", "skeleton", 1)},
		[]*ScoredAtom{mk("flesh/z", "flesh", 0.5), mk("flesh/a", "flesh", 0.5)},
	)
	want := []string{"skeleton/a", "skeleton/z", "flesh/a", "flesh/z"}
	if len(merged) != len(want) {
		t.Fatalf("merged %d atoms, want %d", len(merged), len(want))
	}
	for i, id := range want {
		if merged[i].Atom.ID != id {
			t.Fatalf("order = %v, want %v", idsOf(merged), want)
		}
	}
}

func idsOf(ss []*ScoredAtom) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Atom.ID
	}
	return out
}

// All three string quoters must agree, stay ASCII-only, and emit only escapes
// the Mangle lexer accepts: padded \u{hhhh[hh]}, \xHH, and the short four.
func TestMangleQuoting_UnifiedAndLexable(t *testing.T) {
	cases := map[string]string{
		"":             `""`,
		"hello":        `"hello"`,
		`he said "hi"`: `"he said \"hi\""`,
		"café":         `"caf\xe9"`,
		"🎉":            `"\u{01f389}"`,
		"a\rb":         `"a\x0db"`,
		"a\x00b":       `"a\x00b"`,
		"a\nb\tc":      `"a\nb\tc"`,
		`p\q`:          `"p\\q"`,
		"日本語":          `"\u{65e5}\u{672c}\u{8a9e}"`,
	}
	for in, want := range cases {
		if got := mangleQuoteString(in); got != want {
			t.Errorf("mangleQuoteString(%q) = %s, want %s", in, got, want)
		}
		var fb factBuilder
		fb.WriteQuotedString(in)
		if got := fb.String(); got != want {
			t.Errorf("WriteQuotedString(%q) = %s, want %s", in, got, want)
		}
		fb.Reset()
		fb.writeStringLiteral(in)
		if got := fb.String(); got != want {
			t.Errorf("writeStringLiteral(%q) = %s, want %s", in, got, want)
		}
		for _, got := range []string{mangleQuoteString(in)} {
			for i := 0; i < len(got); i++ {
				if got[i] > 0x7e {
					t.Errorf("output for %q is not ASCII-only: %s", in, got)
					break
				}
			}
		}
	}
}

type stubVectorSearcher struct {
	scores map[string]float64
	delay  time.Duration
}

func (s stubVectorSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var out []SearchResult
	for id, score := range s.scores {
		out = append(out, SearchResult{AtomID: id, Score: score})
	}
	return out, nil
}

func (s stubVectorSearcher) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return []float32{0.1}, nil
}

// The reported vector milliseconds must measure the vector search, not the
// whole flesh load — and vector hits must reach the flesh scoring.
func TestSelection_VectorTimingMeasuresSearch(t *testing.T) {
	atoms, cc := selectionFixture()
	cc.SemanticQuery = "something"
	inner := &mockKernel{facts: []any{
		Fact{Predicate: "selected_result", Args: []any{"identity/core", 100, "skeleton"}},
		Fact{Predicate: "selected_result", Args: []any{"protocol/base", 90, "skeleton"}},
		Fact{Predicate: "selected_result", Args: []any{"context/a", 50, "flesh"}},
	}}
	sel := NewAtomSelector()
	sel.SetKernel(inner)
	sel.SetVectorSearcher(stubVectorSearcher{scores: map[string]float64{"context/a": 0.9}, delay: 60 * time.Millisecond})
	merged, vectorMs, err := sel.SelectAtomsWithTiming(context.Background(), atoms, cc)
	if err != nil {
		t.Fatal(err)
	}
	if vectorMs < 60 {
		t.Errorf("vectorMs = %d, want >= 60 (the stub search delay)", vectorMs)
	}
	for _, m := range merged {
		if m.Atom.ID == "context/a" && m.VectorScore != 0.9 {
			t.Errorf("context/a vector score = %v, want 0.9", m.VectorScore)
		}
	}
}

// A flesh build panic degrades to skeleton-only; it must never fail the
// compile or leak a partial flesh set.
func TestSelection_FleshPanicDegrades(t *testing.T) {
	atoms, cc := selectionFixture()
	cc.SemanticQuery = "something"
	inner := &mockKernel{facts: []any{
		Fact{Predicate: "selected_result", Args: []any{"identity/core", 100, "skeleton"}},
		Fact{Predicate: "selected_result", Args: []any{"protocol/base", 90, "skeleton"}},
	}}
	sel := NewAtomSelector()
	sel.SetKernel(inner)
	sel.SetVectorSearcher(panicSearcher{})
	merged, err := sel.SelectAtoms(context.Background(), atoms, cc)
	if err != nil {
		t.Fatalf("flesh panic must degrade, not fail: %v", err)
	}
	for _, m := range merged {
		if m.Source == "flesh" {
			t.Fatalf("partial flesh survived a build panic: %v", idsOf(merged))
		}
	}
}

type panicSearcher struct{ stubVectorSearcher }

func (p panicSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	panic("boom")
}

// writeAtom must never emit single quotes: the Mangle lexer rejects them, so
// a quoted fact dies at assert time and fails the compile. Whitespace and
// punctuation fold to underscores instead; blank input still yields "" so the
// caller skips the fact (fail-closed preserved).
func TestWriteAtom_WhitespaceFoldsNeverQuotes(t *testing.T) {
	if got := mangleNormalizeNameConst("Coder Shard"); got != "/coder_shard" {
		t.Errorf("Coder Shard = %q, want %q", got, "/coder_shard")
	}
	for _, in := range []string{"a b", "a\tb", "a\nb", `a"b`, "a'b", " a ", "\t"} {
		got := mangleNormalizeNameConst(in)
		if strings.Contains(got, "'") {
			t.Errorf("mangleNormalizeNameConst(%q) = %q, must not contain single quotes", in, got)
		}
	}
	if got := mangleNormalizeNameConst("   "); got != "" {
		t.Errorf("blank input = %q, want empty (skip the fact)", got)
	}
}

// A corrupt source must degrade, never panic the turn: nil atoms flow through
// collection deliberately, so every downstream loop skips them.
func TestSelection_NilAtomsDegrade(t *testing.T) {
	var nilAtom *PromptAtom
	if nilAtom.MatchesContext(NewCompilationContext()) {
		t.Error("nil atom must not match any context")
	}
	valid := NewPromptAtom("ok/id", CategoryIdentity, "content")
	sel := NewAtomSelector()
	cc := NewCompilationContext()
	// An empty kernel double: queries answer nothing, so the run exercises
	// fact building (where the nils live) and degrades to no selection.
	scored, err := sel.selectAtomsKernel(context.Background(), []*PromptAtom{nil, valid, nil}, cc, &mockKernel{})
	if err != nil {
		t.Fatalf("SelectAtoms with nils: %v", err)
	}
	for _, sa := range scored {
		if sa == nil || sa.Atom == nil {
			t.Fatal("selection must not emit nil scored atoms")
		}
	}
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("compiler: %v", err)
	}
	result := compiler.buildResultWithStats(
		[]*PromptAtom{nil, valid},
		append([]*ScoredAtom{nil}, scored...),
		[]*OrderedAtom{nil},
		"", 1000, &CompilationStats{},
	)
	if result == nil || result.Manifest == nil {
		t.Fatal("result build with nils must still produce a manifest")
	}
}
