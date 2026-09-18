package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func seedStoreWithAtom(t *testing.T, path, atomID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	s, err := NewLocalStore(path)
	if err != nil {
		t.Fatalf("NewLocalStore(%s): %v", path, err)
	}
	if err := s.StorePromptAtom(&PromptAtom{AtomID: atomID, Version: 1, Content: "content", Category: "test"}); err != nil {
		t.Fatalf("StorePromptAtom: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func atomStamp(t *testing.T, path, atomID string) sql.NullString {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer db.Close()
	var stamp sql.NullString
	if err := db.QueryRow("SELECT embedding_model FROM prompt_atoms WHERE atom_id = ?", atomID).Scan(&stamp); err != nil {
		// A database the re-embed never touched still has no embedding_model column.
		return sql.NullString{}
	}
	return stamp
}

// A force re-embed is workspace maintenance and must never rewrite build
// inputs. Measured 2026-09-17: the walk included the internal/ tree, whose
// *_corpus.db files are go:embed-ed into the binary, so one /embedding reembed
// dirtied five tracked databases and the next build shipped the rewritten
// copies. The roots are the .nerd tree and nothing else.
func TestReembedSearchRoots_LeaveBuildInputsAlone(t *testing.T) {
	ws := t.TempDir()
	workspaceDB := filepath.Join(ws, ".nerd", "prompts", "corpus.db")
	seedDB := filepath.Join(ws, "internal", "core", "defaults", "prompt_corpus.db")
	seedStoreWithAtom(t, workspaceDB, "identity/coder/mission")
	seedStoreWithAtom(t, seedDB, "identity/coder/mission")

	engine := &MockEmbeddingEngine{NameFunc: func() string { return "ollama/embeddinggemma:300m" }}
	if _, err := ReembedAllDBsForce(context.Background(), ReembedSearchRoots(ws), engine, nil); err != nil {
		t.Fatalf("ReembedAllDBsForce: %v", err)
	}

	if got := atomStamp(t, workspaceDB, "identity/coder/mission"); !got.Valid || got.String != "ollama/embeddinggemma:300m" {
		t.Errorf("workspace corpus was not re-embedded and stamped: %#v", got)
	}
	if got := atomStamp(t, seedDB, "identity/coder/mission"); got.Valid {
		t.Errorf("the seed database under internal/ was rewritten by the re-embed (stamp %q); build inputs must be left alone", got.String)
	}
}
