package sync

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

type fakeBackfillEngine struct {
	calls int
	dim   int
}

func (f *fakeBackfillEngine) Name() string { return "fake-test" }
func (f *fakeBackfillEngine) Dimensions() int {
	if f.dim == 0 {
		return 3
	}
	return f.dim
}
func (f *fakeBackfillEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls++
	return []float32{0.1, 0.2, 0.3}, nil
}
func (f *fakeBackfillEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		f.calls++
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}

func openSeededTestDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_atoms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		concept TEXT NOT NULL,
		content TEXT NOT NULL,
		confidence REAL NOT NULL,
		content_hash TEXT
	)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vectors (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT,
		embedding TEXT,
		embedding_model TEXT,
		metadata TEXT
	)`); err != nil {
		t.Fatalf("create vectors: %v", err)
	}
	return db
}

func knowledgeCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM knowledge_atoms`).Scan(&n); err != nil {
		t.Fatalf("count knowledge_atoms: %v", err)
	}
	return n
}

func vectorCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vectors`).Scan(&n); err != nil {
		// No vectors table yet counts as zero.
		return 0
	}
	return n
}

func insertKnowledgeAtom(t *testing.T, db *sql.DB, concept, content string, confidence float64) {
	t.Helper()
	hash := store.ComputeContentHash(concept, content)
	if _, err := db.Exec(
		`INSERT INTO knowledge_atoms (concept, content, confidence, content_hash) VALUES (?, ?, ?, ?)`,
		concept, content, confidence, hash,
	); err != nil {
		t.Fatalf("insert atom %s: %v", concept, err)
	}
}

// Boot heals a KB by pruning the removed seed's placeholder rows and their
// vectors; it writes no knowledge of its own, so an empty KB stays empty.
func TestBackfillPrunesPlaceholdersAndSeedsNothing(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "emptyagent_knowledge.db")
	empty := openSeededTestDB(t, emptyPath)
	if err := backfillKnowledgeVectors(context.Background(), empty, emptyPath, &fakeBackfillEngine{}); err != nil {
		t.Fatalf("backfill empty: %v", err)
	}
	if got := knowledgeCount(t, empty); got != 0 {
		t.Fatalf("an empty KB was seeded with %d atom(s)", got)
	}

	thinPath := filepath.Join(dir, "coder_knowledge.db")
	thin := openSeededTestDB(t, thinPath)
	insertKnowledgeAtom(t, thin, "agent_identity", "You are coder, a specialist agent in the codeNERD ecosystem.", 0.9)
	insertKnowledgeAtom(t, thin, "agent_mission", "coder exists to carry out coder tasks in this workspace.", 0.85)
	insertKnowledgeAtom(t, thin, "methodology/overview", "coder methodology: decompose the task, locate the relevant code and docs, apply best practices, verify with the project's build and tests.", 0.8)
	insertKnowledgeAtom(t, thin, "guide/errors", "Wrap errors with %w so callers can match them.", 0.9)
	eng := &fakeBackfillEngine{}
	if err := backfillKnowledgeVectors(context.Background(), thin, thinPath, eng); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if got := knowledgeCount(t, thin); got != 1 {
		t.Fatalf("knowledgeCount() = %d, want 1 (placeholders pruned, real atom kept)", got)
	}
	if got := vectorCount(t, thin); got != 1 {
		t.Fatalf("vectorCount() = %d, want 1", got)
	}
	// A placeholder that gained a vector earlier loses it with its row.
	insertKnowledgeAtom(t, thin, "agent_identity", "You are coder, a specialist agent in the codeNERD ecosystem.", 0.9)
	if _, err := thin.Exec(`INSERT INTO vectors (content, embedding, embedding_model, metadata) VALUES (?, '[1]', 'm', ?)`,
		"You are coder, a specialist agent in the codeNERD ecosystem.", `{"content_type":"knowledge_atom","concept":"agent_identity"}`); err != nil {
		t.Fatal(err)
	}
	if err := backfillKnowledgeVectors(context.Background(), thin, thinPath, eng); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if k, v := knowledgeCount(t, thin), vectorCount(t, thin); k != 1 || v != 1 {
		t.Fatalf("after the second heal knowledge=%d vectors=%d, want 1 1", k, v)
	}
}

func TestBackfillEmbedsMissingVectors(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vectoragent_knowledge.db")
	db := openSeededTestDB(t, dbPath)

	insertKnowledgeAtom(t, db, "concept/a", "content about concurrency", 0.9)
	insertKnowledgeAtom(t, db, "concept/b", "content about testing", 0.8)
	if got := vectorCount(t, db); got != 0 {
		t.Fatalf("expected 0 vectors, got %d", got)
	}

	eng := &fakeBackfillEngine{}
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, eng); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if eng.calls != 2 {
		t.Fatalf("expected 2 embed calls, got %d", eng.calls)
	}
	if got := vectorCount(t, db); got != 2 {
		t.Fatalf("expected 2 vectors after backfill, got %d", got)
	}

	// Idempotent: already-embedded atoms are skipped, no new vectors.
	eng.calls = 0
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, eng); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if eng.calls != 0 {
		t.Fatalf("expected 0 embed calls on second run, got %d", eng.calls)
	}
	if got := vectorCount(t, db); got != 2 {
		t.Fatalf("second backfill duplicated vectors: got %d", got)
	}
}

func TestBackfillSkipsAlreadyEmbedded(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "partialagent_knowledge.db")
	db := openSeededTestDB(t, dbPath)

	insertKnowledgeAtom(t, db, "concept/a", "alpha content", 0.9)
	insertKnowledgeAtom(t, db, "concept/b", "beta content", 0.8)

	eng := &fakeBackfillEngine{}
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, eng); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if got := vectorCount(t, db); got != 2 {
		t.Fatalf("expected 2 vectors, got %d", got)
	}

	// Add one new atom; only it should be embedded.
	insertKnowledgeAtom(t, db, "concept/c", "gamma content", 0.7)
	eng.calls = 0
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, eng); err != nil {
		t.Fatalf("incremental backfill: %v", err)
	}
	if eng.calls != 1 {
		t.Fatalf("expected 1 embed call for the new atom, got %d", eng.calls)
	}
	if got := vectorCount(t, db); got != 3 {
		t.Fatalf("expected 3 vectors, got %d", got)
	}
}

func TestBackfillNilDBNoError(t *testing.T) {
	if err := backfillKnowledgeVectors(context.Background(), nil, "x_knowledge.db", nil); err != nil {
		t.Fatalf("nil db should be a no-op, got %v", err)
	}
	if err := backfillKnowledgeVectors(context.Background(), nil, "x_knowledge.db", &fakeBackfillEngine{}); err != nil {
		t.Fatalf("nil db with engine should be a no-op, got %v", err)
	}
}

func TestBackfillKnowledgeVectors_PreservesNonEmptyKBWithoutInit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "keeper_knowledge.db")
	db := openSeededTestDB(t, dbPath)
	insertKnowledgeAtom(t, db, "custom", "custom content", 0.7)
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, nil); err != nil {
		t.Fatalf("backfillKnowledgeVectors() error = %v", err)
	}
	if got := knowledgeCount(t, db); got != 1 {
		t.Fatalf("knowledgeCount() = %d, want 1 (no reseed of non-empty KB)", got)
	}
}

func TestBackfillKnowledgeVectors_CancelledContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cancelled_knowledge.db")
	db := openSeededTestDB(t, dbPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := backfillKnowledgeVectors(ctx, db, dbPath, nil); err == nil {
		t.Fatalf("backfillKnowledgeVectors() with cancelled ctx = nil, want error")
	}
}

func TestListEmbeddedKnowledgeKeys_FiltersNonKnowledgeAtoms(t *testing.T) {
	db := openSeededTestDB(t, filepath.Join(t.TempDir(), "filter_knowledge.db"))
	insertKnowledgeAtom(t, db, "keep", "keep me", 0.9)
	embedForFilter := func(content, meta string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO vectors (content, embedding, embedding_model, metadata) VALUES (?, ?, ?, ?)`,
			content, "[1,2]", "test-model", meta); err != nil {
			t.Fatalf("insert vector: %v", err)
		}
	}
	embedForFilter("keep me", `{"content_type":"knowledge_atom","concept":"keep"}`)
	embedForFilter("other", `{"content_type":"conversation","concept":"keep"}`)
	embedForFilter("bad json", `not-json`)
	embedForFilter("no concept", `{"content_type":"knowledge_atom"}`)
	embedForFilter("empty meta", ``)
	got := listEmbeddedKnowledgeKeys(db)
	if len(got) != 1 {
		t.Fatalf("listEmbeddedKnowledgeKeys() len = %d, want 1", len(got))
	}
	if !got["keep\x00keep me"] {
		t.Errorf("expected keep atom to be marked embedded")
	}
}

func TestListEmbeddedKnowledgeKeys_NilDBAndMissingTable(t *testing.T) {
	if got := listEmbeddedKnowledgeKeys(nil); len(got) != 0 {
		t.Fatalf("listEmbeddedKnowledgeKeys(nil) len = %d, want 0", len(got))
	}
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "novec.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if got := listEmbeddedKnowledgeKeys(db); len(got) != 0 {
		t.Fatalf("listEmbeddedKnowledgeKeys() with missing table len = %d, want 0", len(got))
	}
}

func TestFetchKnowledgeAtoms_MissingTableReturnsNoError(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "noatoms.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	atoms, err := fetchKnowledgeAtoms(db)
	if err != nil {
		t.Fatalf("fetchKnowledgeAtoms() error = %v, want nil for missing table", err)
	}
	if len(atoms) != 0 {
		t.Fatalf("fetchKnowledgeAtoms() len = %d, want 0", len(atoms))
	}
}

type failOnceBackfillEngine struct {
	fakeBackfillEngine
	failOn string
}

func (e *failOnceBackfillEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == e.failOn {
		return nil, errBackfillTest
	}
	return e.fakeBackfillEngine.Embed(ctx, text)
}

func TestBackfillMissingVectors_ContinuesOnEmbedError(t *testing.T) {
	db := openSeededTestDB(t, filepath.Join(t.TempDir(), "continue_knowledge.db"))
	insertKnowledgeAtom(t, db, "good", "good content", 0.9)
	insertKnowledgeAtom(t, db, "bad", "bad content", 0.9)
	atoms := []knowledgeAtomRow{
		{concept: "good", content: "good content", confidence: 0.9},
		{concept: "bad", content: "bad content", confidence: 0.9},
	}
	engine := &failOnceBackfillEngine{failOn: "bad content"}
	done := backfillMissingVectors(context.Background(), db, engine, atoms, map[string]bool{})
	if done != 1 {
		t.Fatalf("backfillMissingVectors() = %d, want 1 (bad atom skipped)", done)
	}
	if got := vectorCount(t, db); got != 1 {
		t.Fatalf("vectorCount() = %d, want 1", got)
	}
}

func TestBackfillMissingVectors_CancelledContextStops(t *testing.T) {
	db := openSeededTestDB(t, filepath.Join(t.TempDir(), "stop_knowledge.db"))
	atoms := []knowledgeAtomRow{{concept: "a", content: "a content", confidence: 0.9}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if done := backfillMissingVectors(ctx, db, &fakeBackfillEngine{}, atoms, map[string]bool{}); done != 0 {
		t.Fatalf("backfillMissingVectors() with cancelled ctx = %d, want 0", done)
	}
}

var errBackfillTest = errors.New("backfill test embed failure")
