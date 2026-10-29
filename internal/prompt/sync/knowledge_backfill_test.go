package sync

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
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

func TestAgentIDFromDBPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{filepath.Join("x", ".nerd", "shards", "bubbleteaexpert_knowledge.db"), "bubbleteaexpert"},
		{"/tmp/agentone_knowledge.db", "agentone"},
		{"/tmp/prompts.db", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := agentIDFromDBPath(tc.path); got != tc.want {
			t.Errorf("agentIDFromDBPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestBackfillSeedsEmptyKBWithoutInit(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "emptyagent_knowledge.db")
	db := openSeededTestDB(t, dbPath)

	if got := knowledgeCount(t, db); got != 0 {
		t.Fatalf("expected empty KB, got %d atoms", got)
	}

	// Nil engine must still seed: atoms are keyword-searchable without vectors.
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, nil); err != nil {
		t.Fatalf("backfill with nil engine: %v", err)
	}
	if got := knowledgeCount(t, db); got == 0 {
		t.Fatal("empty KB was not seeded on restart")
	}
	if got := vectorCount(t, db); got != 0 {
		t.Fatalf("nil engine must not create vectors, got %d", got)
	}

	// Seeding is idempotent: a second run must not duplicate atoms.
	before := knowledgeCount(t, db)
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, nil); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if got := knowledgeCount(t, db); got != before {
		t.Fatalf("second seed duplicated atoms: before=%d got=%d", before, got)
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

func TestBaselineSeedAtoms_DefaultsToSpecialist(t *testing.T) {
	tests := []struct {
		name     string
		agentID  string
		wantName string
	}{
		{name: "empty defaults to specialist", agentID: "", wantName: "specialist"},
		{name: "whitespace defaults to specialist", agentID: "   ", wantName: "specialist"},
		{name: "trims surrounding space", agentID: "  coder  ", wantName: "coder"},
		{name: "keeps inner case for identity", agentID: "Reviewer", wantName: "Reviewer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := baselineSeedAtoms(tt.agentID)
			if len(got) != 3 {
				t.Fatalf("baselineSeedAtoms() len = %d, want 3", len(got))
			}
			wantConcepts := []string{"agent_identity", "agent_mission", "methodology/overview"}
			for i, want := range wantConcepts {
				if got[i].concept != want {
					t.Errorf("atom[%d].concept = %q, want %q", i, got[i].concept, want)
				}
			}
			for _, a := range got {
				if a.content == "" {
					t.Errorf("concept %q has empty content", a.concept)
				}
				if !containsFold(a.content, tt.wantName) {
					t.Errorf("concept %q content %q does not mention %q", a.concept, a.content, tt.wantName)
				}
				if a.confidence <= 0 || a.confidence > 1 {
					t.Errorf("concept %q confidence = %v, want in (0,1]", a.concept, a.confidence)
				}
			}
		})
	}
}

func TestEnsureSeededBaseline_SeedsEmptyKBOnce(t *testing.T) {
	db := openSeededTestDB(t, filepath.Join(t.TempDir(), "coder_knowledge.db"))
	ctx := context.Background()
	seeded, err := ensureSeededBaseline(ctx, db, "coder")
	if err != nil {
		t.Fatalf("ensureSeededBaseline() error = %v", err)
	}
	if seeded != 3 {
		t.Fatalf("ensureSeededBaseline() = %d, want 3", seeded)
	}
	if got := knowledgeCount(t, db); got != 3 {
		t.Fatalf("knowledgeCount() = %d, want 3", got)
	}
	// Second call must not reseed a non-empty KB.
	seeded, err = ensureSeededBaseline(ctx, db, "coder")
	if err != nil {
		t.Fatalf("second ensureSeededBaseline() error = %v", err)
	}
	if seeded != 0 {
		t.Fatalf("second ensureSeededBaseline() = %d, want 0", seeded)
	}
	if got := knowledgeCount(t, db); got != 3 {
		t.Fatalf("knowledgeCount() after reseed = %d, want 3", got)
	}
}

func TestEnsureSeededBaseline_EmptyAgentDefaultsToSpecialist(t *testing.T) {
	db := openSeededTestDB(t, filepath.Join(t.TempDir(), "specialist_knowledge.db"))
	if _, err := ensureSeededBaseline(context.Background(), db, ""); err != nil {
		t.Fatalf("ensureSeededBaseline() error = %v", err)
	}
	rows, err := db.Query(`SELECT content FROM knowledge_atoms WHERE concept = 'agent_identity'`)
	if err != nil {
		t.Fatalf("query identity atom: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if containsFold(content, "specialist") {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	if !found {
		t.Errorf("expected baseline identity atom mentioning specialist")
	}
}

func TestEnsureSeededBaseline_CancelledContext(t *testing.T) {
	db := openSeededTestDB(t, filepath.Join(t.TempDir(), "coder_knowledge.db"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ensureSeededBaseline(ctx, db, "coder"); err == nil {
		t.Fatalf("ensureSeededBaseline() with cancelled ctx = nil, want error")
	}
	if got := knowledgeCount(t, db); got != 0 {
		t.Fatalf("knowledgeCount() after cancelled seed = %d, want 0", got)
	}
}

func TestEnsureSeededBaseline_MissingTableIsNoop(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notables.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seeded, err := ensureSeededBaseline(context.Background(), db, "coder")
	if err != nil {
		t.Fatalf("ensureSeededBaseline() with missing table error = %v", err)
	}
	if seeded != 0 {
		t.Fatalf("ensureSeededBaseline() = %d, want 0 when table is missing", seeded)
	}
}

func TestBackfillKnowledgeVectors_NilEngineStillSeedsRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "healer_knowledge.db")
	db := openSeededTestDB(t, dbPath)
	// Restart with no embedding engine must still heal an empty KB.
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, nil); err != nil {
		t.Fatalf("backfillKnowledgeVectors() error = %v", err)
	}
	if got := knowledgeCount(t, db); got != 3 {
		t.Fatalf("knowledgeCount() = %d, want 3 seeded without engine", got)
	}
	if got := vectorCount(t, db); got != 0 {
		t.Fatalf("vectorCount() = %d, want 0 with nil engine", got)
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

func TestBackfillKnowledgeVectors_HealsOnRestartIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart_knowledge.db")
	db := openSeededTestDB(t, dbPath)
	engine := &fakeBackfillEngine{}
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, engine); err != nil {
		t.Fatalf("first backfill error = %v", err)
	}
	if got := knowledgeCount(t, db); got != 3 {
		t.Fatalf("knowledgeCount() after first backfill = %d, want 3", got)
	}
	firstVectors := vectorCount(t, db)
	if firstVectors != 3 {
		t.Fatalf("vectorCount() after first backfill = %d, want 3", firstVectors)
	}
	// Second restart heals nothing new: idempotent.
	if err := backfillKnowledgeVectors(context.Background(), db, dbPath, engine); err != nil {
		t.Fatalf("second backfill error = %v", err)
	}
	if got := knowledgeCount(t, db); got != 3 {
		t.Fatalf("knowledgeCount() after second backfill = %d, want 3", got)
	}
	if got := vectorCount(t, db); got != firstVectors {
		t.Fatalf("vectorCount() after second backfill = %d, want %d", got, firstVectors)
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

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
