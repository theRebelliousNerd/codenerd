package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// spaceEngine is a deterministic engine whose coordinate system is its own:
// text i of the vocabulary embeds to basis vector (i + shift) % dim. Two
// spaceEngines of the same dimension but different shift are different
// embedding spaces that no length check can tell apart -- the query "alpha"
// in one space is, coordinate for coordinate, the stored vector of "beta" in
// the other.
type spaceEngine struct {
	name  string
	dim   int
	shift int

	mu      sync.Mutex
	failOn  map[string]bool // text -> refuse to embed it
	onEmbed func(text string)
	embeds  []string
}

var spaceVocab = map[string]int{"alpha": 0, "beta": 1, "gamma": 2}

func (e *spaceEngine) vec(text string) []float32 {
	i, ok := spaceVocab[text]
	if !ok {
		i = len(text) // anything else: stable, arbitrary
	}
	v := make([]float32, e.dim)
	v[(i+e.shift)%e.dim] = 1
	return v
}

func (e *spaceEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	e.embeds = append(e.embeds, text)
	fail := e.failOn[text]
	hook := e.onEmbed
	e.mu.Unlock()
	if hook != nil {
		hook(text)
	}
	if fail {
		return nil, fmt.Errorf("provider refused %q", text)
	}
	return e.vec(text), nil
}

func (e *spaceEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := e.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (e *spaceEngine) Dimensions() int { return e.dim }
func (e *spaceEngine) Name() string    { return e.name }

func (e *spaceEngine) embeddedTexts() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.embeds...)
}

func newSpaceStore(t *testing.T) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func attach(t *testing.T, s *LocalStore, e *spaceEngine) {
	t.Helper()
	s.SetEmbeddingEngine(e)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatalf("waitForVecBackfill: %v", err)
	}
}

func storeAll(t *testing.T, s *LocalStore, texts ...string) {
	t.Helper()
	for _, text := range texts {
		if err := s.StoreVectorWithEmbedding(context.Background(), text, map[string]any{"path": text, "kind": "unit"}); err != nil {
			t.Fatalf("store %q: %v", text, err)
		}
	}
}

// onEveryRecallPath runs check on the sqlite-vec path (when the build has it)
// and then on the brute-force path, so a rule that holds on one and not the
// other is a failure rather than a surprise.
func onEveryRecallPath(t *testing.T, s *LocalStore, check func(t *testing.T, path string)) {
	t.Helper()
	original := s.vectorExt.Load()
	defer s.vectorExt.Store(original)
	if original {
		t.Run("vec_index", func(t *testing.T) { check(t, "vec_index") })
	}
	s.vectorExt.Store(false)
	t.Run("brute_force", func(t *testing.T) { check(t, "brute_force") })
}

func contentsOf(entries []VectorEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Content)
	}
	return out
}

func containsText(entries []VectorEntry, text string) bool {
	for _, e := range entries {
		if e.Content == text {
			return true
		}
	}
	return false
}

func recallAll(t *testing.T, s *LocalStore, query string) map[string][]VectorEntry {
	t.Helper()
	ctx := context.Background()
	out := map[string][]VectorEntry{}
	var err error
	if out["semantic"], err = s.VectorRecallSemantic(ctx, query, 10); err != nil {
		t.Fatalf("VectorRecallSemantic: %v", err)
	}
	if out["by_paths"], err = s.VectorRecallSemanticByPaths(ctx, query, 10, []string{"alpha", "beta", "gamma"}); err != nil {
		t.Fatalf("VectorRecallSemanticByPaths: %v", err)
	}
	if out["filtered"], err = s.VectorRecallSemanticFiltered(ctx, query, 10, "kind", "unit"); err != nil {
		t.Fatalf("VectorRecallSemanticFiltered: %v", err)
	}
	if out["with_task"], err = s.VectorRecallSemanticWithTask(ctx, query, 10, ""); err != nil {
		t.Fatalf("VectorRecallSemanticWithTask: %v", err)
	}
	return out
}

// G1. Model A and model B emit vectors of the same length. A switch from A to B
// used to leave every A row eligible for B's queries: the query "alpha" in
// space B IS the stored vector of "beta" in space A, so recall returned the
// wrong neighbor at similarity 1.0 with no outage and no malformed vector.
func TestSameDimensionOtherSpace_NeverRanksOnAnyRecallPath(t *testing.T) {
	s := newSpaceStore(t)
	a := &spaceEngine{name: "test:model-a", dim: 4}
	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1}
	attach(t, s, a)
	storeAll(t, s, "alpha", "beta", "gamma")
	attach(t, s, b)

	onEveryRecallPath(t, s, func(t *testing.T, path string) {
		for kind, got := range recallAll(t, s, "alpha") {
			if len(got) != 0 {
				t.Errorf("%s/%s: model-A rows %v ranked for a model-B query", path, kind, contentsOf(got))
			}
		}
	})
}

// The same store, once B has produced its own vectors, ranks only those -- and
// both recall paths agree on which rows and in what order.
func TestRecallPaths_RankTheSameEligibleRows(t *testing.T) {
	s := newSpaceStore(t)
	a := &spaceEngine{name: "test:model-a", dim: 4}
	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1}
	attach(t, s, a)
	storeAll(t, s, "alpha", "beta") // stay in space A
	attach(t, s, b)
	storeAll(t, s, "gamma") // produced in space B

	results := map[string]map[string][]string{}
	onEveryRecallPath(t, s, func(t *testing.T, path string) {
		got := map[string][]string{}
		for kind, entries := range recallAll(t, s, "gamma") {
			got[kind] = contentsOf(entries)
			if !containsText(entries, "gamma") {
				t.Errorf("%s/%s: the model-B row did not rank for its own query: %v", path, kind, contentsOf(entries))
			}
			if containsText(entries, "alpha") || containsText(entries, "beta") {
				t.Errorf("%s/%s: a model-A row ranked: %v", path, kind, contentsOf(entries))
			}
		}
		results[path] = got
	})
	if v, ok := results["vec_index"]; ok {
		for kind, want := range results["brute_force"] {
			if fmt.Sprint(v[kind]) != fmt.Sprint(want) {
				t.Errorf("%s: vec_index ranked %v, brute force ranked %v", kind, v[kind], want)
			}
		}
	}
}

// Legacy rows -- an embedding and no recorded space -- stay stored and
// keyword-recallable, never rank semantically, and are counted as unattributed
// so the reason is on record. Stamping them with today's engine would be a guess.
func TestLegacyUnstampedVectors_StayStoredButNeverRank(t *testing.T) {
	s := newSpaceStore(t)
	if _, err := s.db.Exec(`INSERT INTO vectors (content, embedding, metadata) VALUES ('alpha', '[1,0,0,0]', '{"path":"alpha","kind":"unit"}')`); err != nil {
		t.Fatal(err)
	}
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})

	onEveryRecallPath(t, s, func(t *testing.T, path string) {
		for kind, got := range recallAll(t, s, "alpha") {
			if len(got) != 0 {
				t.Errorf("%s/%s: an unattributed legacy vector ranked: %v", path, kind, contentsOf(got))
			}
		}
	})

	lex, err := s.VectorRecall("alpha", 5)
	if err != nil || !containsText(lex, "alpha") {
		t.Fatalf("keyword recall lost the legacy row: %v (err %v)", contentsOf(lex), err)
	}
	cov, err := s.VectorCoverage()
	if err != nil {
		t.Fatal(err)
	}
	if cov.Unattributed != 1 || cov.Eligible != 0 || cov.Total != 1 {
		t.Fatalf("coverage = %+v, want 1 unattributed, 0 eligible of 1", cov)
	}
	stats, err := s.GetVectorStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats["unattributed_vectors"] != int64(1) || stats["eligible_vectors"] != int64(0) {
		t.Fatalf("GetVectorStats hides the exclusion: %v", stats)
	}
}

// A database that predates the stamp column opens, gains the column, and keeps
// its rows.
func TestLegacyDatabaseGainsTheStampColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY AUTOINCREMENT, content TEXT NOT NULL, embedding TEXT, metadata TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO vectors (content, embedding, metadata) VALUES ('old row', '[1,0,0,0]', '{}')`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	s, err := NewLocalStore(path)
	if err != nil {
		t.Fatalf("NewLocalStore on a pre-stamp database: %v", err)
	}
	defer s.Close()
	if !columnExists(s.db, "vectors", "embedding_model") {
		t.Fatal("vectors.embedding_model was not added")
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM vectors WHERE embedding_model IS NULL`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the legacy row must survive unstamped: n=%d err=%v", n, err)
	}
}

// G2. A force re-embed in which one row's embedding fails leaves that row at
// its OLD vector. Brute force used to rank it, and the dimension-only reconcile
// and the next engine attach used to put it back into the index. The pass
// itself returned nil.
func TestPartialForceReembed_ReportsIncompleteAndOldVectorsStayOut(t *testing.T) {
	s := newSpaceStore(t)
	a := &spaceEngine{name: "test:model-a", dim: 4}
	attach(t, s, a)
	storeAll(t, s, "alpha", "beta", "gamma")

	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1, failOn: map[string]bool{"beta": true}}
	attach(t, s, b)

	n, err := s.ReembedAllVectorsForce(context.Background())
	if !errors.Is(err, ErrIncompleteReembed) {
		t.Fatalf("a pass that left a row behind returned %v, want an IncompleteReembedError", err)
	}
	var inc *IncompleteReembedError
	if !errors.As(err, &inc) || inc.Total != 3 || inc.Done != 2 || inc.Failed != 1 || inc.RunID == "" {
		t.Fatalf("incomplete error = %+v, want 2 of 3 done, 1 failed, with a run id", inc)
	}
	if n != 2 {
		t.Fatalf("embedded %d rows, want 2", n)
	}

	// Right after the pass, then after reconcile, then after the engine is
	// attached again (which rebuilds the index from the table).
	check := func(stage string) {
		onEveryRecallPath(t, s, func(t *testing.T, path string) {
			for kind, got := range recallAll(t, s, "alpha") {
				// B's "alpha" is e1 == A's old "beta": the retained row would
				// be the perfect (wrong) match.
				if containsText(got, "beta") {
					t.Errorf("%s %s/%s: the failed row's old model-A vector ranked: %v", stage, path, kind, contentsOf(got))
				}
				if !containsText(got, "alpha") {
					t.Errorf("%s %s/%s: the migrated row is missing: %v", stage, path, kind, contentsOf(got))
				}
			}
		})
	}
	check("after force")

	if healed, err := s.ReconcileVecIndex(context.Background()); err != nil || healed != 0 {
		t.Fatalf("reconcile healed %d rows (err %v): it indexed a row in another space", healed, err)
	}
	check("after reconcile")

	attach(t, s, b)
	if s.vectorExt.Load() {
		if got := vecIndexCount(t, s); got != 2 {
			t.Fatalf("vec_index holds %d rows after re-attach, want the 2 migrated ones", got)
		}
	}
	check("after re-attach")
}

// The pass is resumable: a second run for the same target brings over only what
// the first left, certifies the result, and a run after a completed one is a
// true force again.
func TestForceReembed_ResumesAndCertifies(t *testing.T) {
	s := newSpaceStore(t)
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})
	storeAll(t, s, "alpha", "beta", "gamma")

	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1, failOn: map[string]bool{"beta": true}}
	attach(t, s, b)
	_, err := s.ReembedAllVectorsForce(context.Background())
	var first *IncompleteReembedError
	if !errors.As(err, &first) {
		t.Fatalf("first pass: %v, want incomplete", err)
	}

	// The provider recovers.
	b.mu.Lock()
	b.failOn = nil
	b.embeds = nil
	b.mu.Unlock()
	n, err := s.ReembedAllVectorsForce(context.Background())
	if err != nil {
		t.Fatalf("resumed pass: %v", err)
	}
	if n != 1 {
		t.Fatalf("resumed pass embedded %d rows, want only the 1 left behind", n)
	}
	if got := b.embeddedTexts(); len(got) != 1 || got[0] != "beta" {
		t.Fatalf("resumed pass called the provider for %v, want just beta", got)
	}
	var runID, state string
	if err := s.db.QueryRow(`SELECT run_id, state FROM embedding_migrations WHERE table_name = 'vectors'`).Scan(&runID, &state); err != nil {
		t.Fatal(err)
	}
	if runID != first.RunID || state != migrationComplete {
		t.Fatalf("migration record = run %q state %q, want run %q complete", runID, state, first.RunID)
	}
	cov, _ := s.VectorCoverage()
	if cov.Eligible != 3 || cov.Pending() != 0 {
		t.Fatalf("coverage after a complete pass = %+v", cov)
	}

	// After a completed run, force means force.
	b.mu.Lock()
	b.embeds = nil
	b.mu.Unlock()
	n, err = s.ReembedAllVectorsForce(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("pass after completion embedded %d rows (err %v), want a full 3", n, err)
	}
}

// A row with no text cannot be embedded; it must not make a migration
// permanently incomplete.
func TestForceReembed_BlankRowsDoNotBlockCompletion(t *testing.T) {
	s := newSpaceStore(t)
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})
	if _, err := s.db.Exec(`INSERT INTO vectors (content, metadata) VALUES ('   ', '{}')`); err != nil {
		t.Fatal(err)
	}
	storeAll(t, s, "alpha")
	if _, err := s.ReembedAllVectorsForce(context.Background()); err != nil {
		t.Fatalf("a blank row blocked completion: %v", err)
	}
}

// Conditional publication: the provider call runs without the store lock, so
// the row can be replaced while its vector is being computed. The stale vector
// is not published, and the pass says so.
func TestForceReembed_DoesNotPublishAVectorOfReplacedContent(t *testing.T) {
	s := newSpaceStore(t)
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})
	storeAll(t, s, "alpha", "beta")

	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1}
	var once sync.Once
	b.onEmbed = func(text string) {
		if text != "beta" {
			return
		}
		once.Do(func() {
			if _, err := s.db.Exec(`UPDATE vectors SET content = 'beta, rewritten' WHERE content = 'beta'`); err != nil {
				t.Errorf("concurrent edit: %v", err)
			}
		})
	}
	attach(t, s, b)
	_, err := s.ReembedAllVectorsForce(context.Background())
	if !errors.Is(err, ErrIncompleteReembed) {
		t.Fatalf("a stale publication was reported as %v, want an incomplete pass", err)
	}
	var stamp sql.NullString
	if err := s.db.QueryRow(`SELECT embedding_model FROM vectors WHERE content = 'beta, rewritten'`).Scan(&stamp); err != nil {
		t.Fatal(err)
	}
	if stamp.String == b.name {
		t.Fatal("the rewritten row was stamped with a vector computed from its old text")
	}
}

// An engine replaced while an embedding is in flight must not lend its identity
// to the old engine's vector: the row keeps the producing engine's stamp and
// never enters the new engine's index.
func TestStore_StampsTheEngineThatProducedTheVector(t *testing.T) {
	s := newSpaceStore(t)
	a := &spaceEngine{name: "test:model-a", dim: 4}
	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1}
	attach(t, s, a)
	a.onEmbed = func(text string) { s.SetEmbeddingEngine(b) } // switch mid-call
	if err := s.StoreVectorWithEmbedding(context.Background(), "alpha", map[string]any{"path": "alpha", "kind": "unit"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatal(err)
	}
	var stamp string
	if err := s.db.QueryRow(`SELECT embedding_model FROM vectors WHERE content = 'alpha'`).Scan(&stamp); err != nil {
		t.Fatal(err)
	}
	if stamp != a.name {
		t.Fatalf("row stamped %q, want the producing engine %q", stamp, a.name)
	}
	if s.vectorExt.Load() {
		if got := vecIndexCount(t, s); got != 0 {
			t.Fatalf("a model-A vector entered the model-B index: %d rows", got)
		}
	}
	onEveryRecallPath(t, s, func(t *testing.T, path string) {
		for kind, got := range recallAll(t, s, "alpha") {
			if len(got) != 0 {
				t.Errorf("%s/%s: %v", path, kind, contentsOf(got))
			}
		}
	})
}

// Callers use the metadata count to decide a corpus is already indexed. A
// vector in another space is not indexed for this engine, so counting it would
// stop the caller from ever re-embedding what it stored.
func TestMetadataCountsReportWhatTheAttachedEngineCanServe(t *testing.T) {
	s := newSpaceStore(t)
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})
	storeAll(t, s, "alpha", "beta")
	if n, err := s.CountVectorsByMetadata("kind", "unit"); err != nil || n != 2 {
		t.Fatalf("count under the producing engine = %d (err %v), want 2", n, err)
	}
	attach(t, s, &spaceEngine{name: "test:model-b", dim: 4, shift: 1})
	if n, err := s.CountVectorsByMetadata("kind", "unit"); err != nil || n != 0 {
		t.Fatalf("count under another space = %d (err %v), want 0", n, err)
	}
	if got, err := s.VectorContentsByMetadata("kind", "unit"); err != nil || len(got) != 0 {
		t.Fatalf("contents under another space = %v (err %v), want none", got, err)
	}
}

// An engine with no identity cannot attribute its vectors; nothing is written.
func TestStore_RefusesAnEngineWithNoIdentity(t *testing.T) {
	s := newSpaceStore(t)
	s.SetEmbeddingEngine(&spaceEngine{name: "  ", dim: 4})
	if err := s.StoreVectorWithEmbedding(context.Background(), "alpha", nil); !errors.Is(err, ErrNoEmbeddingIdentity) {
		t.Fatalf("store with a nameless engine: %v, want ErrNoEmbeddingIdentity", err)
	}
	if _, err := s.StoreVectorBatchWithEmbedding(context.Background(), []string{"alpha"}, []map[string]any{nil}); !errors.Is(err, ErrNoEmbeddingIdentity) {
		t.Fatalf("batch store with a nameless engine: %v, want ErrNoEmbeddingIdentity", err)
	}
}

// The batch writer stamps too.
func TestBatchStore_StampsTheProducingEngine(t *testing.T) {
	s := newSpaceStore(t)
	a := &spaceEngine{name: "test:model-a", dim: 4}
	attach(t, s, a)
	if _, err := s.StoreVectorBatchWithEmbedding(context.Background(), []string{"alpha", "beta"}, []map[string]any{{"path": "alpha", "kind": "unit"}, {"path": "beta", "kind": "unit"}}); err != nil {
		t.Fatal(err)
	}
	cov, err := s.VectorCoverage()
	if err != nil || cov.Eligible != 2 {
		t.Fatalf("coverage = %+v (err %v), want 2 eligible", cov, err)
	}
	attach(t, s, &spaceEngine{name: "test:model-b", dim: 4, shift: 1})
	cov, _ = s.VectorCoverage()
	if cov.Eligible != 0 || cov.Foreign != 2 {
		t.Fatalf("after a space change coverage = %+v, want 2 foreign", cov)
	}
}

// The all-databases pass answers "did everything migrate" with a manifest, not
// a count: a database whose vectors table was left partly behind makes the
// whole run incomplete and names the table, the numbers and the run to resume.
func TestReembedAllDBsForce_ManifestIsNotCompleteWhenARowWasLeftBehind(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "knowledge.db")
	s, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})
	storeAll(t, s, "alpha", "beta", "gamma")
	_ = s.Close()

	b := &spaceEngine{name: "test:model-b", dim: 4, shift: 1, failOn: map[string]bool{"beta": true}}
	res, err := ReembedAllDBsForce(context.Background(), []string{root}, b, nil)
	if err != nil {
		t.Fatalf("ReembedAllDBsForce: %v", err)
	}
	if res.Complete {
		t.Fatal("a run that left a vector behind reported Complete")
	}
	var vec *StoreMigration
	for i := range res.Stores {
		if res.Stores[i].Table == "vectors" {
			vec = &res.Stores[i]
		}
	}
	if vec == nil || vec.Complete || vec.Failed != 1 || vec.Total != 3 || vec.RunID == "" {
		t.Fatalf("vectors manifest entry = %+v, want incomplete, 1 of 3 failed, with a run id", vec)
	}
	if len(res.Skipped) == 0 {
		t.Fatal("the incomplete table is missing from Skipped")
	}

	// Resumed against a recovered provider, the same call certifies.
	b.mu.Lock()
	b.failOn = nil
	b.mu.Unlock()
	res, err = ReembedAllDBsForce(context.Background(), []string{root}, b, nil)
	if err != nil || !res.Complete {
		t.Fatalf("resumed run: complete=%v err=%v stores=%+v", res.Complete, err, res.Stores)
	}
}

// An empty scan has nothing left behind.
func TestReembedAllDBsForce_NothingToMigrateIsComplete(t *testing.T) {
	res, err := ReembedAllDBsForce(context.Background(), []string{t.TempDir()}, &spaceEngine{name: "test:model-a", dim: 4}, nil)
	if err != nil || !res.Complete {
		t.Fatalf("complete=%v err=%v", res.Complete, err)
	}
}

// The predicate selector indexes its corpus once and trusts the count: after a
// model switch the count drops to zero, so it re-embeds, and the unique
// predicate-content index makes each re-store replace the old row rather than
// leave a model-A twin beside it.
func TestPredicateCorpusReindexesAfterASpaceChange(t *testing.T) {
	s := newSpaceStore(t)
	attach(t, s, &spaceEngine{name: "test:model-a", dim: 4})
	contents := []string{"alpha", "beta", "gamma"}
	meta := func() []map[string]any {
		m := make([]map[string]any, len(contents))
		for i := range m {
			m[i] = map[string]any{"kind": "predicate", "path": contents[i]}
		}
		return m
	}
	if _, err := s.StoreVectorBatchWithEmbedding(context.Background(), contents, meta()); err != nil {
		t.Fatal(err)
	}
	attach(t, s, &spaceEngine{name: "test:model-b", dim: 4, shift: 1})
	if n, _ := s.CountVectorsByMetadata("kind", "predicate"); n != 0 {
		t.Fatalf("model-A predicate vectors counted as indexed for model B: %d", n)
	}
	if _, err := s.StoreVectorBatchWithEmbedding(context.Background(), contents, meta()); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountVectorsByMetadata("kind", "predicate"); n != 3 {
		t.Fatalf("after re-indexing, %d predicate vectors serve model B, want 3", n)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM vectors`).Scan(&total); err != nil || total != 3 {
		t.Fatalf("vectors holds %d rows after re-indexing (err %v), want 3: a model-A twin survived", total, err)
	}
	onEveryRecallPath(t, s, func(t *testing.T, path string) {
		got, err := s.VectorRecallSemanticFiltered(context.Background(), "alpha", 5, "kind", "predicate")
		if err != nil || len(got) == 0 || got[0].Content != "alpha" {
			t.Fatalf("%s: re-indexed corpus recall = %v (err %v), want alpha first", path, contentsOf(got), err)
		}
	})
}
