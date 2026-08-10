package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openReconcileTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS prompt_atoms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		atom_id TEXT NOT NULL UNIQUE,
		version INTEGER DEFAULT 1,
		content TEXT NOT NULL,
		token_count INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		description TEXT,
		content_concise TEXT,
		content_min TEXT,
		category TEXT NOT NULL,
		subcategory TEXT,
		priority INTEGER DEFAULT 50,
		is_mandatory BOOLEAN DEFAULT FALSE,
		is_exclusive TEXT,
		depends_on TEXT,
		conflicts_with TEXT,
		embedding BLOB,
		embedding_task TEXT DEFAULT 'RETRIEVAL_DOCUMENT',
		source_file TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS atom_context_tags (
		atom_id TEXT NOT NULL,
		dimension TEXT NOT NULL,
		tag TEXT NOT NULL,
		is_exclusion BOOLEAN DEFAULT FALSE,
		PRIMARY KEY (atom_id, dimension, tag),
		FOREIGN KEY(atom_id) REFERENCES prompt_atoms(atom_id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS vec_prompt_atoms (
		atom_id TEXT PRIMARY KEY,
		embedding BLOB
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func mustInsertAtom(t *testing.T, db *sql.DB, atomID, content, description string, sourceFile interface{}, embedding []byte) {
	t.Helper()
	hash := HashContent(content)
	if hash == "" {
		hash = "hash-" + atomID
	}
	_, err := db.Exec(`INSERT INTO prompt_atoms (atom_id, version, content, token_count, content_hash, description, category, source_file, embedding, embedding_task) VALUES (?, 1, ?, ?, ?, ?, 'methodology', ?, ?, ?)`,
		atomID, content, EstimateTokens(content), hash, nullableString(description), sourceFile, nullableBytes(embedding), nil)
	if err != nil {
		t.Fatalf("insert atom %s: %v", atomID, err)
	}
}

func mustInsertTag(t *testing.T, db *sql.DB, atomID, dim, tag string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES (?, ?, ?)`, atomID, dim, tag); err != nil {
		t.Fatalf("insert tag %s %s %s: %v", atomID, dim, tag, err)
	}
}

func mustInsertVecRow(t *testing.T, db *sql.DB, atomID string, blob []byte) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO vec_prompt_atoms (atom_id, embedding) VALUES (?, ?)`, atomID, blob); err != nil {
		t.Fatalf("insert vec row %s: %v", atomID, err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return n
}

func TestReconcile_TransactionRollback(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	mustInsertAtom(t, db, "project_keep", "keep content", "keep desc", nil, nil)
	mustInsertAtom(t, db, "reject_vec", "old content", "old desc", "embedded", []byte{9})
	mustInsertVecRow(t, db, "reject_vec", []byte{9})
	if _, err := db.Exec(`
		CREATE TRIGGER reject_vector_delete
		BEFORE DELETE ON vec_prompt_atoms
		WHEN OLD.atom_id = 'reject_vec'
		BEGIN
			SELECT RAISE(ABORT, 'forced vector delete failure');
		END;
	`); err != nil {
		t.Fatalf("create rejection trigger: %v", err)
	}

	// The first upsert succeeds, then the second atom needs a vector deletion that
	// the trigger rejects. The entire reconciliation must roll back.
	a1 := NewPromptAtom("new_valid", CategoryMethodology, "content valid")
	a1.Description = "desc valid"
	a1.OperationalModes = []string{"active"}

	a2 := NewPromptAtom("reject_vec", CategoryMethodology, "new content")
	a2.Description = "new desc"

	_, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{a1, a2})
	if err == nil {
		t.Fatalf("expected vector deletion error, got nil")
	}

	// Rollback: the new atom is absent and the existing atom keeps its old data.
	if c := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, "new_valid"); c != 0 {
		t.Fatalf("expected rollback, found new_valid persisted")
	}
	var content string
	if err := db.QueryRow(`SELECT content FROM prompt_atoms WHERE atom_id = ?`, "reject_vec").Scan(&content); err != nil {
		t.Fatalf("select reject_vec after rollback: %v", err)
	}
	if content != "old content" {
		t.Fatalf("reject_vec content=%q want old content", content)
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, "project_keep"); c != 1 {
		t.Fatalf("expected project_keep to survive rollback, count=%d", c)
	}
}

func TestReconcile_WithoutVectorTable(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()
	if _, err := db.Exec(`DROP TABLE vec_prompt_atoms`); err != nil {
		t.Fatalf("drop vector table: %v", err)
	}

	mustInsertAtom(t, db, "changed", "old body", "old desc", "embedded", []byte{1})
	atom := NewPromptAtom("changed", CategoryMethodology, "new body")
	atom.Description = "new desc"

	if _, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom}); err != nil {
		t.Fatalf("reconcile without vector table: %v", err)
	}
}

func TestReconcile_StaleBuiltInRewrite(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	mustInsertAtom(t, db, "stale1", "old content", "old desc", "embedded", []byte{1, 2, 3})
	mustInsertTag(t, db, "stale1", "mode", "active")

	atom := NewPromptAtom("stale1", CategoryMethodology, "new content")
	atom.Description = "new desc"
	atom.Version = 2
	atom.OperationalModes = []string{"dream"}

	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if counts.Upserted != 1 {
		t.Fatalf("upserted=%d want 1", counts.Upserted)
	}
	var content, sourceFile sql.NullString
	var description sql.NullString
	if err := db.QueryRow(`SELECT content, description, source_file FROM prompt_atoms WHERE atom_id = ?`, "stale1").Scan(&content, &description, &sourceFile); err != nil {
		t.Fatalf("select stale1: %v", err)
	}
	if content.String != "new content" {
		t.Fatalf("content=%q want %q", content.String, "new content")
	}
	if description.String != "new desc" {
		t.Fatalf("description=%q want %q", description.String, "new desc")
	}
	if sourceFile.String != "embedded" {
		t.Fatalf("source_file=%q want embedded", sourceFile.String)
	}
	// tags should be exactly the new set
	if c := countRows(t, db, `SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ?`, "stale1"); c != 1 {
		t.Fatalf("tags count=%d want 1", c)
	}
	var tag string
	if err := db.QueryRow(`SELECT tag FROM atom_context_tags WHERE atom_id = ? AND dimension = ?`, "stale1", "mode").Scan(&tag); err != nil {
		t.Fatalf("select tag: %v", err)
	}
	if tag != "dream" {
		t.Fatalf("tag=%q want dream", tag)
	}
}

func TestReconcile_ObsoleteLegacyPathDeletion(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	mustInsertAtom(t, db, "keep1", "keep", "", "embedded", nil)
	mustInsertAtom(t, db, "obsolete_yaml", "old", "", "prompts/custom.yaml", nil)
	mustInsertAtom(t, db, "obsolete_embedded", "old2", "", "embedded", nil)
	mustInsertAtom(t, db, "obsolete_generated", "old3", "", "generated", nil)
	mustInsertVecRow(t, db, "obsolete_yaml", []byte{9, 9})
	mustInsertTag(t, db, "obsolete_yaml", "mode", "active")

	atom := NewPromptAtom("keep1", CategoryMethodology, "keep")
	atom.Description = ""
	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if counts.Deleted != 3 {
		t.Fatalf("deleted=%d want 3", counts.Deleted)
	}
	for _, id := range []string{"obsolete_yaml", "obsolete_embedded", "obsolete_generated"} {
		if c := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, id); c != 0 {
			t.Fatalf("expected %s deleted", id)
		}
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, "obsolete_yaml"); c != 0 {
		t.Fatalf("vec row for obsolete_yaml not deleted")
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ?`, "obsolete_yaml"); c != 0 {
		t.Fatalf("tag rows for obsolete_yaml not deleted")
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, "keep1"); c != 1 {
		t.Fatalf("keep1 should remain")
	}
}

func TestReconcile_ProjectOnlyPreservation(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	// project-owned rows: NULL, empty, whitespace
	mustInsertAtom(t, db, "proj_null", "c", "", nil, nil)
	mustInsertAtom(t, db, "proj_empty", "c", "", "", nil)
	mustInsertAtom(t, db, "proj_space", "c", "", "   ", nil)
	mustInsertAtom(t, db, "owned_old", "c", "", "embedded", nil)

	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// only owned_old should be deleted
	if counts.Deleted != 1 {
		t.Fatalf("deleted=%d want 1", counts.Deleted)
	}
	for _, id := range []string{"proj_null", "proj_empty", "proj_space"} {
		if c := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, id); c != 1 {
			t.Fatalf("project row %s should be preserved", id)
		}
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, "owned_old"); c != 0 {
		t.Fatalf("owned_old should be deleted")
	}
}

func TestReconcile_DescriptionOnlyEmbeddingInvalidation(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	// Existing atom with description "desc1" and embedding.
	mustInsertAtom(t, db, "desc_test", "body1", "desc1", "embedded", []byte{1, 2, 3, 4})
	mustInsertVecRow(t, db, "desc_test", []byte{1, 2, 3, 4})

	// Content unchanged, description changed -> embedding input changed -> cleared.
	atom := NewPromptAtom("desc_test", CategoryMethodology, "body1")
	atom.Description = "desc2"
	// Ensure hash is based on body1 (unchanged) but description drives invalidation.

	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if counts.ClearedEmbeddings != 1 {
		t.Fatalf("cleared=%d want 1", counts.ClearedEmbeddings)
	}
	if counts.RetainedEmbeddings != 0 {
		t.Fatalf("retained=%d want 0", counts.RetainedEmbeddings)
	}
	var emb []byte
	var task sql.NullString
	if err := db.QueryRow(`SELECT embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?`, "desc_test").Scan(&emb, &task); err != nil {
		t.Fatalf("select embedding: %v", err)
	}
	if len(emb) != 0 {
		t.Fatalf("embedding should be nulled, got %v", emb)
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, "desc_test"); c != 0 {
		t.Fatalf("vec row should be removed on invalidation")
	}

	// Also verify content-only invalidation: description empty, content used.
	mustInsertAtom(t, db, "content_test", "bodyA", "", "embedded", []byte{5, 6})
	mustInsertVecRow(t, db, "content_test", []byte{5, 6})
	atom2 := NewPromptAtom("content_test", CategoryMethodology, "bodyB")
	atom2.Description = "" // empty -> input is content
	counts2, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom, atom2})
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	// desc_test retained now? desc_test description desc2 unchanged -> retained
	// content_test description empty but content changed -> cleared
	// We check at least content_test cleared.
	hasCleared := counts2.ClearedEmbeddings >= 1
	if !hasCleared {
		t.Fatalf("expected content_test cleared, got cleared=%d", counts2.ClearedEmbeddings)
	}
	var emb2 []byte
	if err := db.QueryRow(`SELECT embedding FROM prompt_atoms WHERE atom_id = ?`, "content_test").Scan(&emb2); err != nil {
		t.Fatalf("select content_test embedding: %v", err)
	}
	if len(emb2) != 0 {
		t.Fatalf("content_test embedding should be nulled")
	}
}

func TestReconcile_UnchangedEmbeddingRetention(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	origEmb := []byte{7, 8, 9}
	mustInsertAtom(t, db, "retain_test", "body1", "same desc", "embedded", origEmb)
	// set task
	if _, err := db.Exec(`UPDATE prompt_atoms SET embedding_task = ? WHERE atom_id = ?`, "RETRIEVAL_DOCUMENT", "retain_test"); err != nil {
		t.Fatalf("update task: %v", err)
	}
	mustInsertVecRow(t, db, "retain_test", origEmb)

	// Change content but keep description same -> embedding should be retained.
	atom := NewPromptAtom("retain_test", CategoryMethodology, "body2 different")
	atom.Description = "same desc"
	// keep same category etc.

	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if counts.RetainedEmbeddings != 1 {
		t.Fatalf("retained=%d want 1", counts.RetainedEmbeddings)
	}
	if counts.ClearedEmbeddings != 0 {
		t.Fatalf("cleared=%d want 0", counts.ClearedEmbeddings)
	}
	var emb []byte
	var task sql.NullString
	if err := db.QueryRow(`SELECT embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?`, "retain_test").Scan(&emb, &task); err != nil {
		t.Fatalf("select: %v", err)
	}
	if string(emb) != string(origEmb) {
		t.Fatalf("embedding not retained: got %v want %v", emb, origEmb)
	}
	if !task.Valid || task.String != "RETRIEVAL_DOCUMENT" {
		t.Fatalf("embedding_task not retained: %v", task)
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, "retain_test"); c != 1 {
		t.Fatalf("vec row should remain on retained embedding")
	}
}

func TestReconcile_VectorRowInvalidation(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	mustInsertAtom(t, db, "vec_changed", "body1", "desc1", "embedded", []byte{1, 1, 1})
	mustInsertVecRow(t, db, "vec_changed", []byte{1, 1, 1})
	mustInsertAtom(t, db, "vec_orphan", "same body", "same desc", "embedded", nil)
	mustInsertVecRow(t, db, "vec_orphan", []byte{3, 3})
	mustInsertAtom(t, db, "vec_obsolete", "old", "", "prompts/old.yaml", []byte{2, 2})
	mustInsertVecRow(t, db, "vec_obsolete", []byte{2, 2})

	atom := NewPromptAtom("vec_changed", CategoryMethodology, "body1")
	atom.Description = "desc2 changed" // will invalidate
	orphan := NewPromptAtom("vec_orphan", CategoryMethodology, "same body")
	orphan.Description = "same desc"

	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom, orphan})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if counts.Deleted != 1 {
		t.Fatalf("deleted=%d want 1 for vec_obsolete", counts.Deleted)
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, "vec_changed"); c != 0 {
		t.Fatalf("vec row for changed atom not removed")
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, "vec_orphan"); c != 0 {
		t.Fatalf("orphan vec row without prompt embedding not removed")
	}
	if c := countRows(t, db, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, "vec_obsolete"); c != 0 {
		t.Fatalf("vec row for obsolete atom not removed")
	}
	// changed atom embedding should be nulled
	var emb []byte
	if err := db.QueryRow(`SELECT embedding FROM prompt_atoms WHERE atom_id = ?`, "vec_changed").Scan(&emb); err != nil {
		t.Fatalf("select vec_changed: %v", err)
	}
	if len(emb) != 0 {
		t.Fatalf("vec_changed embedding should be null")
	}
}

func TestReconcile_ExactTagReplacement(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	defer db.Close()

	mustInsertAtom(t, db, "tagtest", "content", "", "embedded", nil)
	mustInsertTag(t, db, "tagtest", "mode", "active")
	mustInsertTag(t, db, "tagtest", "phase", "planning")
	mustInsertTag(t, db, "tagtest", "lang", "python")

	atom := NewPromptAtom("tagtest", CategoryMethodology, "content")
	atom.OperationalModes = []string{"dream", "dream"}
	atom.Languages = []string{"go"}

	_, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Should have exactly 2 tags: mode=dream, lang=go
	rows, err := db.Query(`SELECT dimension, tag FROM atom_context_tags WHERE atom_id = ? ORDER BY dimension, tag`, "tagtest")
	if err != nil {
		t.Fatalf("query tags: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var dim, tag string
		if err := rows.Scan(&dim, &tag); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[dim] = tag
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("tags=%v want 2 entries", got)
	}
	if got["mode"] != "dream" {
		t.Fatalf("mode tag=%q want dream", got["mode"])
	}
	if got["lang"] != "go" {
		t.Fatalf("lang tag=%q want go", got["lang"])
	}
	if _, ok := got["phase"]; ok {
		t.Fatalf("phase tag should have been removed")
	}
}

func TestReconcile_UnexpectedSelectError(t *testing.T) {
	ctx := context.Background()
	db := openReconcileTestDB(t)
	// Close DB to cause SELECT to fail with unexpected error.
	db.Close()
	atom := NewPromptAtom("any", CategoryMethodology, "content")
	_, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{atom})
	if err == nil {
		t.Fatalf("expected error from closed DB, got nil")
	}
	if got := err.Error(); got == "" {
		t.Fatalf("expected non-empty error")
	}
}

func TestReconcile_RealEmbeddedCorpusRepairsDriftAndPreservesProjectAtom(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "prompt_corpus.db")
	written, err := MaterializeDefaultPromptCorpus(dbPath)
	if err != nil {
		t.Fatalf("materialize embedded corpus: %v", err)
	}
	if !written {
		t.Fatal("embedded prompt corpus was not available")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open materialized corpus: %v", err)
	}
	defer db.Close()
	if err := NewAtomLoader(nil).EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure materialized corpus schema: %v", err)
	}

	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("load embedded corpus: %v", err)
	}
	atoms := corpus.All()
	if len(atoms) == 0 {
		t.Fatal("embedded corpus is empty")
	}
	canonical := atoms[0]

	if _, err := db.Exec(`
		UPDATE prompt_atoms
		SET content = 'stale body', description = 'stale description',
			content_hash = 'stale-hash', source_file = 'atoms/legacy.yaml'
		WHERE atom_id = ?
	`, canonical.ID); err != nil {
		t.Fatalf("corrupt canonical atom: %v", err)
	}
	mustInsertAtom(t, db, "project_only_integration", "project body", "project description", nil, nil)
	mustInsertAtom(t, db, "obsolete_generated_integration", "obsolete body", "", "atoms/removed.yaml", nil)
	mustInsertTag(t, db, "obsolete_generated_integration", "mode", "active")

	counts, err := ReconcilePromptCorpus(ctx, db, atoms)
	if err != nil {
		t.Fatalf("reconcile real embedded corpus: %v", err)
	}
	if counts.Upserted != len(atoms) {
		t.Fatalf("upserted=%d want %d", counts.Upserted, len(atoms))
	}
	if counts.Deleted < 1 {
		t.Fatalf("deleted=%d want at least 1", counts.Deleted)
	}

	var content, contentHash, sourceFile string
	if err := db.QueryRow(`
		SELECT content, content_hash, source_file
		FROM prompt_atoms WHERE atom_id = ?
	`, canonical.ID).Scan(&content, &contentHash, &sourceFile); err != nil {
		t.Fatalf("select repaired canonical atom: %v", err)
	}
	if content != canonical.Content || contentHash != canonical.ContentHash || sourceFile != "embedded" {
		t.Fatalf("canonical atom was not repaired: content_match=%t hash_match=%t source=%q",
			content == canonical.Content, contentHash == canonical.ContentHash, sourceFile)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, "project_only_integration"); got != 1 {
		t.Fatalf("project-only atom count=%d want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?`, "obsolete_generated_integration"); got != 0 {
		t.Fatalf("obsolete generated atom count=%d want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ?`, "obsolete_generated_integration"); got != 0 {
		t.Fatalf("obsolete generated tag count=%d want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM prompt_atoms`); got != len(atoms)+1 {
		t.Fatalf("total prompt atom count=%d want %d canonical plus project atom", got, len(atoms)+1)
	}
	if got := countRows(t, db, `
		SELECT COUNT(*) FROM prompt_atoms
		WHERE atom_id != 'project_only_integration' AND source_file != 'embedded'
	`); got != 0 {
		t.Fatalf("found %d non-project atoms without canonical embedded ownership", got)
	}
}
