package store

import (
	"path/filepath"
	"testing"
	"time"
)

// World cache: fast and deep facts for the same file are isolated by depth,
// fingerprints ride along, batch updates land atomically, and file deletion
// cascades to facts.
func TestWorldCache_DepthIsolationAndCascade(t *testing.T) {
	s := openLocalTestStore(t)
	meta := WorldFileMeta{Path: "a.go", Lang: "go", Size: 10, ModTime: 1, Hash: "h", Fingerprint: "fp1"}
	if err := s.UpsertWorldFile(meta); err != nil {
		t.Fatalf("UpsertWorldFile: %v", err)
	}
	fast := []WorldFactInput{{Predicate: "func", Args: []any{"main"}}}
	deep := []WorldFactInput{{Predicate: "calls", Args: []any{"main", "fmt.Println"}}}
	if err := s.ReplaceWorldFactsForFile("a.go", "fast", "fp1", fast); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceWorldFactsForFile("a.go", "deep", "fp1", deep); err != nil {
		t.Fatal(err)
	}
	got, fp, err := s.LoadWorldFactsForFile("a.go", "fast")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Predicate != "func" || fp != "fp1" {
		t.Fatalf("fast = %+v fp=%q, want the func fact with fp1", got, fp)
	}
	got, _, err = s.LoadWorldFactsForFile("a.go", "deep")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Predicate != "calls" {
		t.Fatalf("deep = %+v, want the calls fact", got)
	}

	// Batch update a second file, then delete the first: facts cascade.
	meta2 := WorldFileMeta{Path: "b.go", Lang: "go"}
	if err := s.UpdateWorldFilesAndFacts("fast", []FileUpdates{{Meta: meta2, Facts: fast}}); err != nil {
		t.Fatalf("UpdateWorldFilesAndFacts: %v", err)
	}
	if err := s.DeleteWorldFile("a.go"); err != nil {
		t.Fatalf("DeleteWorldFile: %v", err)
	}
	got, _, err = s.LoadWorldFactsForFile("a.go", "fast")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("facts after file delete = %+v, want none", got)
	}
	paths, err := s.ListWorldFilePaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "b.go" {
		t.Errorf("paths = %v, want [b.go]", paths)
	}
}

// Graph: store/query both directions, BFS path finding, kernel hydration
// with /-prefixed relations, and the Mangle adapter surface.
func TestGraph_LinksPathHydrate(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreLink("a", "depends_on", "b", 1.0, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreLink("b", "depends_on", "c", 1.0, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreLink("", "x", "y", 1.0, nil); err == nil {
		t.Error("expected an error for the empty endpoint")
	}
	if err := s.StoreLink("a", "x", "y", 1.0, map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}

	out, err := s.QueryLinks("a", "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("outgoing(a) = %d links, want 2", len(out))
	}
	in, err := s.QueryLinks("b", "incoming")
	if err != nil {
		t.Fatal(err)
	}
	if len(in) != 1 || in[0].EntityA != "a" {
		t.Fatalf("incoming(b) = %+v, want the a-link", in)
	}
	path, err := s.TraversePath("a", "c", 5)
	if err != nil {
		t.Fatalf("TraversePath: %v", err)
	}
	if len(path) != 2 || path[0].EntityB != "b" || path[1].EntityB != "c" {
		t.Fatalf("path = %+v, want a->b->c", path)
	}
	if _, err := s.TraversePath("c", "a", 5); err == nil {
		t.Error("expected no-path error for c->a")
	}

	var asserted [][3]string
	n, err := s.HydrateKnowledgeGraph(func(pred string, args []any) error {
		asserted = append(asserted, [3]string{pred, args[0].(string), args[1].(string)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("hydrated = %d, want 3", n)
	}
	for _, a := range asserted {
		if a[0] != "knowledge_link" || len(a[2]) == 0 || a[2][0] != '/' {
			t.Errorf("asserted = %v, relation must be /-prefixed", a)
		}
	}

	adapter := NewLocalStoreGraphAdapter(s)
	rels, err := adapter.QueryGraph("relations", map[string]any{"arg": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rels.([]string)) != 2 {
		t.Errorf("relations(b) = %v, want 2 neighbors", rels)
	}
	exists, err := adapter.QueryGraph("path", map[string]any{"arg": "a->c"})
	if err != nil || exists != true {
		t.Errorf("path a->c = %v, %v; want true, nil", exists, err)
	}
	if _, err := adapter.QueryGraph("path", map[string]any{"arg": "nope"}); err == nil {
		t.Error("expected a format error for a malformed path arg")
	}
	if _, err := adapter.QueryGraph("bogus", map[string]any{"arg": "a"}); err == nil {
		t.Error("expected an error for an unknown query type")
	}
}

// Cold storage lifecycle: priority ordering, access tracking, archival of
// old+cold facts (young and hot survive), restore, purge, and the
// maintenance rollup.
func TestColdStorage_Lifecycle(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreFact("p", []any{"old"}, "t", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreFact("p", []any{"new"}, "t", 9); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadFacts("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Args[0] != "new" {
		t.Fatalf("facts = %+v, want priority order with new first", got)
	}
	if got[0].AccessCount != 0 {
		t.Fatalf("access count before reload = %d, want 0 (tracking lands after read)", got[0].AccessCount)
	}
	got, err = s.LoadFacts("p")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].AccessCount != 1 {
		t.Errorf("access count after one reload = %d, want 1", got[0].AccessCount)
	}

	// Age one fact 100 days back with zero touches; it archives, the hot one stays.
	if _, err := s.db.Exec(`UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 0 WHERE args LIKE '%old%'`); err != nil {
		t.Fatal(err)
	}
	n, err := s.ArchiveOldFacts(90, 5)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("archived = %d, want 1", n)
	}
	arch, err := s.GetArchivedFacts("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(arch) != 1 || arch[0].Args[0] != "old" {
		t.Fatalf("archived = %+v, want the old fact", arch)
	}
	if err := s.RestoreArchivedFact("p", []any{"old"}); err != nil {
		t.Fatalf("RestoreArchivedFact: %v", err)
	}
	if err := s.DeleteFact("p", []any{"new"}); err != nil {
		t.Fatalf("DeleteFact: %v", err)
	}
	stats, err := s.MaintenanceCleanup(MaintenanceConfig{ArchiveOlderThanDays: 90, MaxAccessCount: 5, VacuumDatabase: true})
	if err != nil {
		t.Fatalf("MaintenanceCleanup: %v", err)
	}
	if !stats.DatabaseVacuumed {
		t.Error("expected DatabaseVacuumed in maintenance stats")
	}
}

// Tool store: full CRUD surface, reference tracking, stats, and both
// cleanup budgets deleting oldest-first.
func TestToolStore_CRUDAndCleanup(t *testing.T) {
	ts, err := NewToolStore(filepath.Join(t.TempDir(), "tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()

	mk := func(call, session, tool string, size int, runtime int64, success bool) ToolExecution {
		return ToolExecution{CallID: call, SessionID: session, ToolName: tool, Result: "r",
			Success: success, DurationMs: 5, ResultSize: size, SessionRuntimeMs: runtime}
	}
	if err := ts.Store(mk("c1", "s1", "read", 100, 1000, true)); err != nil {
		t.Fatal(err)
	}
	if err := ts.Store(mk("c2", "s1", "write", 200, 2000, false)); err != nil {
		t.Fatal(err)
	}
	got, err := ts.GetByCallID("c1")
	if err != nil || got.ToolName != "read" || !got.Success {
		t.Fatalf("GetByCallID = %+v, %v", got, err)
	}
	if err := ts.IncrementReference("c1"); err != nil {
		t.Fatal(err)
	}
	got, _ = ts.GetByCallID("c1")
	if got.ReferenceCount != 1 || got.LastReferenced == nil {
		t.Errorf("after increment: count=%d last=%v, want 1 and set", got.ReferenceCount, got.LastReferenced)
	}
	bySession, err := ts.GetBySession("s1")
	if err != nil || len(bySession) != 2 {
		t.Fatalf("GetBySession = %d, want 2", len(bySession))
	}
	stats, err := ts.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalExecutions != 2 || stats.TotalSizeBytes != 300 || stats.FailureCount != 1 {
		t.Errorf("stats = %+v, want 2 execs / 300 bytes / 1 failure", stats)
	}

	// Size cleanup deletes oldest first until under budget: 300 bytes with a
	// 250 budget drops only c1 (100), landing at 200.
	cs, err := ts.CleanupBySizeLimit(250)
	if err != nil {
		t.Fatal(err)
	}
	if cs.ExecutionsDeleted != 1 {
		t.Fatalf("size cleanup deleted %d, want 1", cs.ExecutionsDeleted)
	}
	if _, err := ts.GetByCallID("c1"); err == nil {
		t.Error("c1 (oldest) should be gone after size cleanup")
	}

	// Runtime cleanup deletes whole oldest sessions.
	if err := ts.Store(mk("c3", "s2", "read", 50, 99_999_999, true)); err != nil {
		t.Fatal(err)
	}
	cs, err = ts.CleanupByRuntimeBudget(0.001)
	if err != nil {
		t.Fatal(err)
	}
	if cs.ExecutionsDeleted == 0 {
		t.Error("expected runtime cleanup to delete the over-budget session")
	}
	if ts.ShouldAutoCleanup(CleanupConfig{MaxSizeBytes: 1, AutoCleanupThreshold: 0.8, CleanupMode: "size"}) {
		t.Error("ShouldAutoCleanup should be false on a cleaned store")
	}
}

// Trace store: write/read across every index, quality updates, stats,
// insights on empty and broken tables, retention cleanup with vec purge,
// and the skipSuccess filter excluding successful traces.
func TestTraceStore_Surface(t *testing.T) {
	s := openLocalTestStore(t)
	ts := s.GetTraceStore()
	mk := func(id, shard string, success bool) *ReasoningTrace {
		return &ReasoningTrace{ID: id, ShardID: "sh", ShardType: shard, ShardCategory: "ephemeral",
			SessionID: "sess", SystemPrompt: "sys", UserPrompt: "u", Response: "r", Success: success}
	}
	if err := ts.StoreReasoningTrace(mk("t1", "coder", true)); err != nil {
		t.Fatal(err)
	}
	if err := ts.StoreReasoningTrace(mk("t2", "coder", false)); err != nil {
		t.Fatal(err)
	}
	if got, err := ts.GetShardTraces("coder", 10); err != nil || len(got) != 2 {
		t.Fatalf("GetShardTraces = %d, %v; want 2", len(got), err)
	}
	if got, err := ts.GetFailedShardTraces("coder", 10); err != nil || len(got) != 1 || got[0].ID != "t2" {
		t.Fatalf("GetFailedShardTraces = %+v, %v", got, err)
	}
	if err := ts.UpdateTraceQuality("t1", 0.9, []string{"good"}); err != nil {
		t.Fatal(err)
	}
	if got, err := ts.GetHighQualityTraces("coder", 0.5, 10); err != nil || len(got) != 1 {
		t.Fatalf("GetHighQualityTraces = %d, want 1", len(got))
	}
	if got, err := ts.GetTracesBySession("sess"); err != nil || len(got) != 2 {
		t.Fatalf("GetTracesBySession = %d, want 2", len(got))
	}
	st, err := ts.GetTraceStatsForType("coder")
	if err != nil || st.TotalCount != 2 || st.SuccessCount != 1 || st.FailCount != 1 {
		t.Fatalf("type stats = %+v, %v", st, err)
	}

	// Insights on a populated store carry real counts; on a broken table
	// they error instead of reporting a quiet zero store.
	ins, err := ts.GetLearningInsights("coder", 7)
	if err != nil {
		t.Fatal(err)
	}
	if ins["recent_trace_count"] != int64(2) {
		t.Errorf("insights = %v, want 2 recent traces", ins)
	}

	// skipSuccess must exclude the successful trace even though it lacks a
	// descriptor; the old unparenthesized AND let it through.
	cands, err := ts.ListTraceEmbeddingCandidates(10, true, "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cands {
		if c.ID == "t1" {
			t.Errorf("skipSuccess candidates include successful t1: %+v", cands)
		}
	}
}

// Empty-store insights must succeed with zeros (NULL aggregates), and a
// missing table must error rather than read as quiet.
func TestTraceStore_InsightsEmptyAndBroken(t *testing.T) {
	s := openLocalTestStore(t)
	ts := s.GetTraceStore()
	ins, err := ts.GetLearningInsights("coder", 7)
	if err != nil {
		t.Fatalf("empty insights: %v", err)
	}
	if ins["recent_trace_count"] != int64(0) {
		t.Errorf("empty insights = %v, want zero count", ins)
	}
	if _, err := s.db.Exec("DROP TABLE reasoning_traces"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.GetLearningInsights("coder", 7); err == nil {
		t.Error("expected an error for insights on a missing table")
	}
}

// Retention cleanup purges the ANN index alongside the rows: no ghosts.
func TestTraceStore_CleanupPurgesVec(t *testing.T) {
	s := openLocalTestStore(t)
	ts := s.GetTraceStore()
	old := &ReasoningTrace{ID: "old", ShardID: "sh", ShardType: "coder", ShardCategory: "e",
		SessionID: "s", SystemPrompt: "sys", UserPrompt: "u", Response: "r", Success: true}
	if err := ts.StoreReasoningTrace(old); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE reasoning_traces SET created_at = ? WHERE id = 'old'`,
		time.Now().AddDate(0, 0, -100).Format("2006-01-02 15:04:05")); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureTraceVecTable(4); err != nil {
		t.Fatalf("ensureTraceVecTable: %v", err)
	}
	seed := encodeFloat32Slice([]float32{1, 0, 0, 0})
	if _, err := s.db.Exec("INSERT INTO reasoning_traces_vec (trace_id, embedding) VALUES ('old', ?)", seed); err != nil {
		t.Fatal(err)
	}
	n, err := ts.CleanupOldTraces(90)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("cleaned = %d, want 1", n)
	}
	var vecLeft int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM reasoning_traces_vec").Scan(&vecLeft); err != nil {
		t.Fatal(err)
	}
	if vecLeft != 0 {
		t.Errorf("vec rows after cleanup = %d, want 0", vecLeft)
	}
}
