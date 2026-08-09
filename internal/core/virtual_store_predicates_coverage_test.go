package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

type mockGraphQuery struct {
	result any
	err    error
}

func (m *mockGraphQuery) QueryGraph(queryType string, params map[string]any) (any, error) {
	return m.result, m.err
}

type stubTransactorKernel struct {
	*stubKernel
	commitErr error
}

type failingAssertKernel struct {
	*stubTransactorKernel
	assertErr error
}

type nonTransactionalKernel struct {
	*stubKernel
}

func (k *failingAssertKernel) Assert(Fact) error {
	return k.assertErr
}

type stubKernelTransaction struct {
	k *stubTransactorKernel
}

func (s *stubKernelTransaction) Retract(predicate string)                           {}
func (s *stubKernelTransaction) RetractFact(fact Fact)                              {}
func (s *stubKernelTransaction) RetractExactFact(fact Fact)                         {}
func (s *stubKernelTransaction) RetractPredicateSet(predicates map[string]struct{}) {}
func (s *stubKernelTransaction) Assert(fact Fact) {
	s.k.asserted = append(s.k.asserted, fact)
}
func (s *stubKernelTransaction) Commit() error {
	return s.k.commitErr
}

func (s *stubTransactorKernel) Transaction() types.KernelTransaction {
	return &stubKernelTransaction{k: s}
}

func TestVirtualStorePredicates_DbNil(t *testing.T) {
	vs := NewVirtualStore(nil)
	vs.SetLocalDB(nil)

	if _, err := vs.QueryLearned("foo"); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.QueryAllLearned("preference"); err == nil {
		t.Error("expected error when DB is nil")
	}

	// Should do nothing and not error
	if err := vs.PersistFactsToKnowledge([]Fact{{Predicate: "a"}}, "", 0); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := vs.PersistLink("A", "rel", "B", 0.0, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := vs.QueryKnowledgeGraph("A", "both"); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.QueryActivations(10, 0.5); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.RecallSimilar("test", 5); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.QuerySession("session_1", 10); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.QueryTraces("coder", 10); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.QueryTraceStats("coder"); err == nil {
		t.Error("expected error when DB is nil")
	}

	if _, err := vs.QueryStrategicKnowledge("vision"); err == nil {
		t.Error("expected error when DB is nil")
	}

	if count, err := vs.HydrateKnowledgeGraph(context.Background()); err != nil || count != 0 {
		t.Errorf("expected count 0 and no error, got %d, %v", count, err)
	}

	if count, err := vs.HydrateLearnings(context.Background()); err != nil || count != 0 {
		t.Errorf("expected count 0 and no error, got %d, %v", count, err)
	}

	if count, err := vs.HydrateSessionContext(context.Background(), "session_1", "query", []string{"coder"}); err != nil || count != 0 {
		t.Errorf("expected count 0 and no error, got %d, %v", count, err)
	}
}

func TestVirtualStorePredicates_DbOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_kb.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}()

	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)

	k := &stubTransactorKernel{stubKernel: &stubKernel{}}
	vs.SetKernel(k)

	// 1. Store and query facts
	f := Fact{Predicate: "test_pred", Args: []any{"arg1", "/arg2"}}
	if err := vs.PersistFactsToKnowledge([]Fact{f}, "preference", 5); err != nil {
		t.Fatalf("failed to persist facts: %v", err)
	}

	learned, err := vs.QueryLearned("test_pred")
	if err != nil {
		t.Fatalf("failed to query learned facts: %v", err)
	}
	if len(learned) != 1 || learned[0].Predicate != "test_pred" {
		t.Errorf("expected 1 fact for test_pred, got %v", learned)
	}

	allLearned, err := vs.QueryAllLearned("preference")
	if err != nil {
		t.Fatalf("failed to query all learned: %v", err)
	}
	if len(allLearned) != 1 {
		t.Errorf("expected 1 fact of type preference, got %v", allLearned)
	}

	hasL, err := vs.HasLearned("test_pred")
	if err != nil || !hasL {
		t.Errorf("expected HasLearned to be true, got %v, err=%v", hasL, err)
	}

	// 2. Persist link & Query Graph
	meta := map[string]any{"note": "test"}
	if err := vs.PersistLink("node_A", "depends_on", "node_B", 1.5, meta); err != nil {
		t.Fatalf("failed to persist link: %v", err)
	}

	links, err := vs.QueryKnowledgeGraph("node_A", "both")
	if err != nil {
		t.Fatalf("failed to query knowledge graph: %v", err)
	}
	if len(links) != 1 || links[0].Predicate != "knowledge_link" {
		t.Errorf("expected 1 knowledge_link, got %v", links)
	}

	// 3. Activations
	if err := db.LogActivation("fact_123", 0.95); err != nil {
		t.Fatalf("failed to log activation: %v", err)
	}
	acts, err := vs.QueryActivations(10, 0.5)
	if err != nil {
		t.Fatalf("failed to query activations: %v", err)
	}
	if len(acts) != 1 || acts[0].Predicate != "activation" {
		t.Errorf("expected 1 activation, got %v", acts)
	}
	if got := acts[0].Args[1]; got != int64(95) {
		t.Errorf("expected activation score on Mangle's 0..100 integer scale, got %#v", got)
	}

	// 4. Vector Recall
	if err := db.StoreVector("golang programming language", map[string]any{"doc_id": "doc_1"}); err != nil {
		t.Fatalf("failed to store vector: %v", err)
	}
	similars, err := vs.RecallSimilar("golang", 5)
	if err != nil {
		t.Fatalf("failed to recall similar: %v", err)
	}
	if len(similars) != 1 || similars[0].Predicate != "similar_content" {
		t.Errorf("expected 1 similar_content, got %v", similars)
	}

	// 5. Session History
	if err := db.StoreSessionTurn("session_A", 1, "hello", "{}", "hi there", "[]"); err != nil {
		t.Fatalf("failed to save session turn: %v", err)
	}
	turns, err := vs.QuerySession("session_A", 10)
	if err != nil {
		t.Fatalf("failed to query session: %v", err)
	}
	if len(turns) != 1 || turns[0].Predicate != "session_turn" {
		t.Errorf("expected 1 session_turn, got %v", turns)
	}
	if got := turns[0].Args[1]; got != int64(1) {
		t.Errorf("expected stored turn number 1, got %#v", got)
	}

	// 6. Shard Traces & Stats
	trace := &store.ReasoningTrace{
		ID:            "trace_1",
		ShardID:       "shard_1",
		ShardType:     "coder",
		ShardCategory: "system",
		SessionID:     "session_A",
		SystemPrompt:  "sys",
		UserPrompt:    "refactor",
		Response:      "no errors",
		Success:       true,
		DurationMs:    150,
	}
	if err := db.GetTraceStore().StoreReasoningTrace(trace); err != nil {
		t.Fatalf("failed to store trace: %v", err)
	}
	for _, additional := range []*store.ReasoningTrace{
		{
			ID: "trace_2", ShardID: "shard_2", ShardType: "coder", ShardCategory: "system",
			SessionID: "session_A", SystemPrompt: "sys", UserPrompt: "test", Response: "failed",
			Success: false, DurationMs: 250,
		},
		{
			ID: "trace_3", ShardID: "shard_3", ShardType: "reviewer", ShardCategory: "system",
			SessionID: "session_A", SystemPrompt: "sys", UserPrompt: "review", Response: "done",
			Success: true, DurationMs: 900,
		},
	} {
		if err := db.GetTraceStore().StoreReasoningTrace(additional); err != nil {
			t.Fatalf("failed to store additional trace: %v", err)
		}
	}
	traces, err := vs.QueryTraces("coder", 5)
	if err != nil {
		t.Fatalf("failed to query traces: %v", err)
	}
	if len(traces) != 2 || traces[0].Predicate != "reasoning_trace" {
		t.Errorf("expected 2 reasoning_trace facts, got %v", traces)
	}

	stats, err := vs.QueryTraceStats("coder")
	if err != nil {
		t.Fatalf("failed to query trace stats: %v", err)
	}
	if len(stats) != 1 || stats[0].Predicate != "trace_stats" {
		t.Errorf("expected 1 trace_stats, got %v", stats)
	}
	if got, want := stats[0].Args, []any{MangleAtom("/coder"), int64(1), int64(1), int64(200)}; !equalFactArgs(got, want) {
		t.Errorf("coder trace stats = %#v, want %#v", got, want)
	}
	reviewerStats, err := vs.QueryTraceStats("reviewer")
	if err != nil {
		t.Fatalf("failed to query reviewer trace stats: %v", err)
	}
	if got, want := reviewerStats[0].Args, []any{MangleAtom("/reviewer"), int64(1), int64(0), int64(900)}; !equalFactArgs(got, want) {
		t.Errorf("single-sample reviewer stats = %#v, want %#v", got, want)
	}

	// 7. Strategic Knowledge
	if err := db.StoreKnowledgeAtom("strategic/vision", "long horizon goal", 0.95); err != nil {
		t.Fatalf("failed to store knowledge atom: %v", err)
	}
	sk, err := vs.QueryStrategicKnowledge("vision")
	if err != nil {
		t.Fatalf("failed to query strategic knowledge: %v", err)
	}
	if len(sk) != 1 || sk[0].Predicate != "strategic_knowledge" {
		t.Errorf("expected 1 strategic_knowledge, got %v", sk)
	}
	if got, want := sk[0].Args, []any{MangleAtom("/vision"), "long horizon goal", int64(95)}; !equalFactArgs(got, want) {
		t.Errorf("strategic knowledge args = %#v, want %#v", got, want)
	}

	// 8. Hydrate functions
	k.asserted = nil
	hCount, err := vs.HydrateLearnings(context.Background())
	if err != nil {
		t.Fatalf("hydrate learnings failed: %v", err)
	}
	if hCount == 0 || len(k.asserted) == 0 {
		t.Errorf("expected hydrated learnings, count=%d asserted=%d", hCount, len(k.asserted))
	}
	assertedActivation := false
	for _, fact := range k.asserted {
		if fact.Predicate == "activation" {
			assertedActivation = true
			if got := fact.Args[1]; got != int64(95) {
				t.Errorf("hydrated activation score = %#v, want int64(95)", got)
			}
		}
	}
	if !assertedActivation {
		t.Error("expected HydrateLearnings to assert activation")
	}

	k.asserted = nil
	hsCount, err := vs.HydrateSessionContext(context.Background(), "session_A", "golang", []string{"coder"})
	if err != nil {
		t.Fatalf("hydrate session context failed: %v", err)
	}
	if hsCount == 0 || len(k.asserted) == 0 {
		t.Errorf("expected hydrated session context, count=%d asserted=%d", hsCount, len(k.asserted))
	}
}

func equalFactArgs(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestVirtualStorePredicates_AtomTypesMatchSchemas(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "typed-atoms.db"))
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}
	defer db.Close()

	if err := db.LogActivation("fact-typed", 0.81); err != nil {
		t.Fatalf("log activation: %v", err)
	}
	if err := db.StoreKnowledgeAtom("strategic/vision", "typed", 0.92); err != nil {
		t.Fatalf("store strategic atom: %v", err)
	}
	if err := db.GetTraceStore().StoreReasoningTrace(&store.ReasoningTrace{
		ID: "trace-typed", ShardID: "typed", ShardType: "coder", SessionID: "typed",
		SystemPrompt: "system", UserPrompt: "user", Response: "response", Success: true, DurationMs: 42,
	}); err != nil {
		t.Fatalf("store reasoning trace: %v", err)
	}

	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)

	query := func(predicate string, args ...any) []ast.Atom {
		t.Helper()
		atom, atomErr := (Fact{Predicate: predicate, Args: args}).ToAtom()
		if atomErr != nil {
			t.Fatalf("make %s query: %v", predicate, atomErr)
		}
		atoms, getErr := vs.Get(atom)
		if getErr != nil {
			t.Fatalf("query %s: %v", predicate, getErr)
		}
		if len(atoms) != 1 {
			t.Fatalf("query %s returned %d atoms, want 1", predicate, len(atoms))
		}
		return atoms
	}

	activation := query("query_activations")[0]
	if score := activation.Args[1].(ast.Constant); score.Type != ast.NumberType || score.NumValue != 81 {
		t.Errorf("activation score constant = %#v, want Number(81)", score)
	}

	strategic := query("query_strategic", "/vision")[0]
	if category := strategic.Args[0].(ast.Constant); category.Type != ast.NameType {
		t.Errorf("strategic category type = %v, want NameType", category.Type)
	}
	if confidence := strategic.Args[2].(ast.Constant); confidence.Type != ast.NumberType || confidence.NumValue != 92 {
		t.Errorf("strategic confidence constant = %#v, want Number(92)", confidence)
	}

	traceFacts, err := vs.QueryTraces("coder", 10)
	if err != nil {
		t.Fatalf("query trace facts: %v", err)
	}
	if category := traceFacts[0].Args[2]; category != MangleAtom("/unknown") {
		t.Errorf("empty trace category = %#v, want MangleAtom(/unknown)", category)
	}
	traces := query("query_traces", "/coder", 10)[0]
	if success := traces.Args[3].(ast.Constant); success.Type != ast.NameType || success.Symbol != "/true" {
		t.Errorf("trace success = %#v, want /true Name constant", success)
	}

	traceStats := query("query_trace_stats", "/coder")[0]
	if shardType := traceStats.Args[0].(ast.Constant); shardType.Type != ast.NameType {
		t.Errorf("trace stats shard type = %#v, want Name constant", shardType)
	}
	for _, index := range []int{1, 2, 3} {
		if value := traceStats.Args[index].(ast.Constant); value.Type != ast.NumberType {
			t.Errorf("trace stats arg %d = %#v, want Number constant", index, value)
		}
	}
}

func TestVirtualStorePredicateFactsMatchLiveKernelSchemas(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "live-schema.db"))
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}
	defer db.Close()

	if err := db.LogActivation("live-fact", 0.73); err != nil {
		t.Fatalf("log activation: %v", err)
	}
	if err := db.StoreKnowledgeAtom("strategic/pattern", "live pattern", 0.88); err != nil {
		t.Fatalf("store strategic atom: %v", err)
	}
	if err := db.GetTraceStore().StoreReasoningTrace(&store.ReasoningTrace{
		ID: "live-trace", ShardID: "live", ShardType: "reviewer", SessionID: "live-session",
		SystemPrompt: "system", UserPrompt: "user", Response: "response", Success: true, DurationMs: 84,
	}); err != nil {
		t.Fatalf("store reasoning trace: %v", err)
	}

	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)
	factGroups := []struct {
		predicate string
		query     func() ([]Fact, error)
	}{
		{"activation", func() ([]Fact, error) { return vs.QueryActivations(10, 0) }},
		{"strategic_knowledge", func() ([]Fact, error) { return vs.QueryStrategicKnowledge("pattern") }},
		{"reasoning_trace", func() ([]Fact, error) { return vs.QueryTraces("reviewer", 10) }},
		{"trace_stats", func() ([]Fact, error) { return vs.QueryTraceStats("reviewer") }},
	}

	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	for _, group := range factGroups {
		facts, queryErr := group.query()
		if queryErr != nil {
			t.Fatalf("query %s facts: %v", group.predicate, queryErr)
		}
		if len(facts) != 1 {
			t.Fatalf("query %s returned %d facts, want 1", group.predicate, len(facts))
		}
		if assertErr := kernel.Assert(facts[0]); assertErr != nil {
			t.Fatalf("assert %s into live kernel: %v", group.predicate, assertErr)
		}
		got, queryErr := kernel.Query(group.predicate)
		if queryErr != nil {
			t.Fatalf("query live kernel for %s: %v", group.predicate, queryErr)
		}
		if len(got) != 1 {
			t.Errorf("live kernel %s facts = %d, want 1: %+v", group.predicate, len(got), got)
		}
	}
}

func TestHydrationSurfacesFailures(t *testing.T) {
	t.Run("learning query failure", func(t *testing.T) {
		db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "closed.db"))
		if err != nil {
			t.Fatalf("create local store: %v", err)
		}
		vs := NewVirtualStore(nil)
		vs.SetLocalDB(db)
		vs.SetKernel(&stubTransactorKernel{stubKernel: &stubKernel{}})
		if err := db.Close(); err != nil {
			t.Fatalf("close local store: %v", err)
		}

		if _, err := vs.HydrateLearnings(context.Background()); err == nil {
			t.Fatal("expected closed-store hydration failure to be returned")
		}
	})

	t.Run("session kernel without transactions", func(t *testing.T) {
		db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "no-transaction.db"))
		if err != nil {
			t.Fatalf("create local store: %v", err)
		}
		defer db.Close()

		vs := NewVirtualStore(nil)
		vs.SetLocalDB(db)
		vs.SetKernel(&nonTransactionalKernel{stubKernel: &stubKernel{}})

		count, err := vs.HydrateSessionContext(context.Background(), "", "", nil)
		if err == nil || !strings.Contains(err.Error(), "does not support transactions") {
			t.Fatalf("HydrateSessionContext error = %v, want transaction capability error", err)
		}
		if count != 0 {
			t.Fatalf("HydrateSessionContext count = %d without transaction support, want 0", count)
		}
	})

	t.Run("learning assertion failure", func(t *testing.T) {
		db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "assert.db"))
		if err != nil {
			t.Fatalf("create local store: %v", err)
		}
		defer db.Close()
		if err := db.StoreFact("pref", []any{"value"}, "preference", 5); err != nil {
			t.Fatalf("store preference: %v", err)
		}

		vs := NewVirtualStore(nil)
		vs.SetLocalDB(db)
		vs.SetKernel(&failingAssertKernel{
			stubTransactorKernel: &stubTransactorKernel{stubKernel: &stubKernel{}},
			assertErr:            errors.New("assert rejected"),
		})

		count, err := vs.HydrateLearnings(context.Background())
		if err == nil || !strings.Contains(err.Error(), "assert rejected") {
			t.Fatalf("HydrateLearnings error = %v, want assertion failure", err)
		}
		if count != 0 {
			t.Fatalf("HydrateLearnings count = %d after rejected assertion, want 0", count)
		}
	})

	t.Run("session query failure commits fresh empty snapshot", func(t *testing.T) {
		db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "partial.db"))
		if err != nil {
			t.Fatalf("create local store: %v", err)
		}
		vs := NewVirtualStore(nil)
		vs.SetLocalDB(db)
		kernel := &stubTransactorKernel{stubKernel: &stubKernel{}}
		vs.SetKernel(kernel)
		if err := db.Close(); err != nil {
			t.Fatalf("close local store: %v", err)
		}

		count, err := vs.HydrateSessionContext(context.Background(), "session", "", nil)
		if err == nil || !strings.Contains(err.Error(), "committed partial snapshot") {
			t.Fatalf("HydrateSessionContext error = %v, want partial snapshot warning", err)
		}
		if count != 0 {
			t.Fatalf("HydrateSessionContext count = %d, want 0", count)
		}
	})

	t.Run("session transaction failure", func(t *testing.T) {
		db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "commit.db"))
		if err != nil {
			t.Fatalf("create local store: %v", err)
		}
		defer db.Close()

		vs := NewVirtualStore(nil)
		vs.SetLocalDB(db)
		vs.SetKernel(&stubTransactorKernel{
			stubKernel: &stubKernel{},
			commitErr:  errors.New("commit rejected"),
		})

		count, err := vs.HydrateSessionContext(context.Background(), "", "", nil)
		if err == nil || !strings.Contains(err.Error(), "commit rejected") {
			t.Fatalf("HydrateSessionContext error = %v, want commit failure", err)
		}
		if count != 0 {
			t.Fatalf("HydrateSessionContext count = %d after failed commit, want 0", count)
		}
	})
}

func TestVirtualStorePredicates_GetAtoms(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_kb_get.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}()

	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)

	k := &stubTransactorKernel{stubKernel: &stubKernel{}}
	vs.SetKernel(k)

	// Populate DB
	_ = vs.PersistFactsToKnowledge([]Fact{{Predicate: "pred_foo", Args: []any{"val_a"}}}, "preference", 5)
	_ = vs.PersistLink("nA", "r", "nB", 1.0, nil)
	_ = db.LogActivation("act_foo", 0.8)
	_ = db.StoreVector("content text", map[string]any{"doc_id": "vec_foo"})
	_ = db.StoreSessionTurn("sess_foo", 1, "user", "{}", "response", "[]")
	trace := &store.ReasoningTrace{
		ID:            "trace_foo",
		ShardID:       "shard_foo",
		ShardType:     "coder",
		ShardCategory: "system",
		SessionID:     "sess_foo",
		SystemPrompt:  "sys",
		UserPrompt:    "generate",
		Response:      "",
		Success:       true,
		DurationMs:    200,
	}
	_ = db.GetTraceStore().StoreReasoningTrace(trace)
	_ = db.StoreKnowledgeAtom("strategic/vision", "our vision", 0.9)

	// Helper to get query atom
	getQueryAtom := func(pred string, args ...any) ast.Atom {
		atom, err := Fact{Predicate: pred, Args: args}.ToAtom()
		if err != nil {
			t.Fatalf("failed to convert fact to atom: %v", err)
		}
		return atom
	}

	tests := []struct {
		name      string
		queryAtom ast.Atom
		wantCount int
	}{
		{"query_learned bound", getQueryAtom("query_learned", "/pred_foo"), 1},
		{"query_learned unbound", getQueryAtom("query_learned", "?Pred"), 1},
		{"has_learned bound", getQueryAtom("has_learned", "/pred_foo"), 1},
		{"has_learned unbound", getQueryAtom("has_learned", "?Pred"), 1},
		{"query_session unbound", getQueryAtom("query_session", "?Sess"), 0},
		{"query_session bound", getQueryAtom("query_session", "sess_foo", 10), 1},
		{"recall_similar unbound", getQueryAtom("recall_similar", "?Q"), 0},
		{"recall_similar bound", getQueryAtom("recall_similar", "content", 5), 1},
		{"query_knowledge_graph unbound", getQueryAtom("query_knowledge_graph", "?E"), 0},
		{"query_knowledge_graph bound", getQueryAtom("query_knowledge_graph", "nA"), 1},
		{"query_activations", getQueryAtom("query_activations"), 1},
		{"query_traces unbound", getQueryAtom("query_traces", "?Shard"), 0},
		{"query_traces bound", getQueryAtom("query_traces", "coder", 10), 1},
		{"query_trace_stats unbound", getQueryAtom("query_trace_stats", "?Shard"), 0},
		{"query_trace_stats bound", getQueryAtom("query_trace_stats", "coder"), 1},
		{"query_strategic unbound", getQueryAtom("query_strategic", "?Cat"), 1},
		{"query_strategic bound", getQueryAtom("query_strategic", "vision"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atoms, err := vs.Get(tt.queryAtom)
			if err != nil {
				t.Fatalf("Get() error: %v", err)
			}
			if len(atoms) != tt.wantCount {
				t.Errorf("Get() returned %d atoms, want %d", len(atoms), tt.wantCount)
			}
		})
	}
}

func TestQueryArgInt(t *testing.T) {
	// Parse helper using Fact.ToAtom
	getTerms := func(args ...any) []ast.BaseTerm {
		atom, err := Fact{Predicate: "dummy", Args: args}.ToAtom()
		if err != nil {
			panic(err)
		}
		return atom.Args
	}

	tests := []struct {
		name    string
		terms   []ast.BaseTerm
		idx     int
		wantVal int
		wantOk  bool
	}{
		{"int value", getTerms(42), 0, 42, true},
		{"int64 value", getTerms(int64(100)), 0, 100, true},
		{"float64 value", getTerms(12.34), 0, 12, true},
		{"string parseable", getTerms("250"), 0, 250, true},
		{"string unparseable", getTerms("abc"), 0, 0, false},
		{"string variable", getTerms("?Var"), 0, 0, false},
		{"out of bounds", getTerms(42), 1, 0, false},
		{"bool type invalid", getTerms(true), 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOk := queryArgInt(tt.terms, tt.idx)
			if gotOk != tt.wantOk || (gotOk && gotVal != tt.wantVal) {
				t.Errorf("queryArgInt() = (%v, %v), want (%v, %v)", gotVal, gotOk, tt.wantVal, tt.wantOk)
			}
		})
	}
}

func TestVirtualStoreGraph(t *testing.T) {
	vs := NewVirtualStore(nil)
	vs.SetGraphQuery(nil)

	getQueryAtom := func(pred string, args ...any) ast.Atom {
		atom, err := Fact{Predicate: pred, Args: args}.ToAtom()
		if err != nil {
			panic(err)
		}
		return atom
	}

	// 1. Unwired GraphQuery
	atoms, err := vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if len(atoms) != 0 {
		t.Errorf("expected 0 atoms, got %d", len(atoms))
	}

	// 2. Wired GraphQuery - success
	mockGQ := &mockGraphQuery{
		result: []string{"depA", "depB"},
	}
	vs.SetGraphQuery(mockGQ)

	atoms, err = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	resTerm := atoms[0].Args[2]
	if resTerm.String() != `["depA", "depB"]` {
		t.Errorf("unexpected result term: %s", resTerm.String())
	}

	// 3. Wired GraphQuery - other types
	mockGQ.result = 42
	atoms, _ = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if len(atoms) == 1 && atoms[0].Args[2].String() != "42" {
		t.Errorf("unexpected int conversion: %v", atoms[0].Args[2])
	}

	mockGQ.result = float32(3.14)
	atoms, _ = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if len(atoms) == 1 && !strings.Contains(atoms[0].Args[2].String(), "3.14") {
		t.Errorf("unexpected float32 conversion: %v", atoms[0].Args[2])
	}

	mockGQ.result = true
	atoms, _ = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if len(atoms) == 1 && atoms[0].Args[2].String() != "/true" {
		t.Errorf("unexpected bool conversion: %v", atoms[0].Args[2])
	}

	mockGQ.result = false
	atoms, _ = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if len(atoms) == 1 && atoms[0].Args[2].String() != "/false" {
		t.Errorf("unexpected bool conversion: %v", atoms[0].Args[2])
	}

	// 4. QueryGraph errors
	mockGQ.err = errors.New("query failed")
	atoms, err = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val", "?Res"))
	if err != nil || len(atoms) != 0 {
		t.Errorf("expected 0 atoms and no error propagated on query failure, got %d, %v", len(atoms), err)
	}

	// 5. Malformed args length
	atoms, _ = vs.Get(getQueryAtom("query_graph", "dependencies", "arg_val"))
	if len(atoms) != 0 {
		t.Error("expected 0 atoms for invalid arg length")
	}

	// 6. Malformed arg type
	// First arg is unbound / not constant
	atoms, _ = vs.Get(getQueryAtom("query_graph", "?Type", "arg_val", "?Res"))
	if len(atoms) != 0 {
		t.Error("expected 0 atoms when first arg is not constant")
	}
}
