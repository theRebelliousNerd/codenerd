package prompt_evolution

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestNewFeedbackCollector_ShouldOpenAStoreCreatedBeforeServingProvenance —
// an evolution.db whose execution_records table predates the provider/model
// columns must still open. The serving index was created in the same batch as
// the table, ahead of the column migration, so every established workspace
// failed with "no such column: provider" and lost prompt evolution for the
// session.
func TestNewFeedbackCollector_ShouldOpenAStoreCreatedBeforeServingProvenance(t *testing.T) {
	nerdDir := t.TempDir()
	storePath := filepath.Join(nerdDir, "prompts", "evolution.db")
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := sql.Open("sqlite3", storePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`
		CREATE TABLE execution_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT UNIQUE,
			session_id TEXT,
			shard_id TEXT,
			shard_type TEXT,
			task_request TEXT,
			problem_type TEXT,
			actions_json TEXT,
			result_json TEXT,
			duration_ms INTEGER,
			atom_ids_json TEXT,
			verdict_json TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO execution_records (task_id, shard_type, problem_type) VALUES ('t1', 'coder', 'bug');`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	fc, err := NewFeedbackCollector(nerdDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector on a pre-provenance store: %v", err)
	}
	defer fc.Close()

	cols := map[string]bool{}
	rows, err := fc.db.Query(`PRAGMA table_info(execution_records)`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols[name] = true
	}
	rows.Close()
	for _, want := range []string{"provider", "model", "prompt_manifest_json", "thought_summary", "thinking_tokens", "grounding_sources_json"} {
		if !cols[want] {
			t.Errorf("column %s was not migrated in", want)
		}
	}

	var indexed int
	if err := fc.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_records_serving'`).Scan(&indexed); err != nil {
		t.Fatal(err)
	}
	if indexed != 1 {
		t.Errorf("serving index missing after migration")
	}
	if fc.totalRecorded != 1 {
		t.Errorf("stats did not load the pre-existing row: recorded=%d", fc.totalRecorded)
	}
}
