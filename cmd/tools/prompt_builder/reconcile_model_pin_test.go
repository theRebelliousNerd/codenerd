package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"codenerd/internal/prompt"

	_ "github.com/mattn/go-sqlite3"
)

func createModelPinFixture(t *testing.T, withModel bool) string {
	t.Helper()
	f, err := os.CreateTemp("", "corpus-modelpin-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	path := f.Name()
	_ = f.Close()
	_ = os.Remove(path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var promptDDL string
	if withModel {
		promptDDL = `CREATE TABLE prompt_atoms (atom_id TEXT PRIMARY KEY, version INTEGER, content TEXT, token_count INTEGER, content_hash TEXT, category TEXT, subcategory TEXT, is_mandatory INTEGER, is_exclusive TEXT, source_file TEXT, embedding BLOB, embedding_task TEXT, embedding_model TEXT)`
	} else {
		promptDDL = `CREATE TABLE prompt_atoms (atom_id TEXT PRIMARY KEY, version INTEGER, content TEXT, token_count INTEGER, content_hash TEXT, category TEXT, subcategory TEXT, is_mandatory INTEGER, is_exclusive TEXT, source_file TEXT, embedding BLOB, embedding_task TEXT)`
	}
	if _, err := db.Exec(promptDDL); err != nil {
		t.Fatalf("create prompt_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE atom_context_tags (atom_id TEXT, dimension TEXT, tag TEXT, is_exclusion INTEGER)`); err != nil {
		t.Fatalf("create tags: %v", err)
	}
	return path
}

func TestReconcilePromptSeed_RequiresEmbeddingModelColumn(t *testing.T) {
	ctx := context.Background()
	legacyPath := createModelPinFixture(t, false)
	defer os.Remove(legacyPath)
	if _, err := reconcilePromptSeed(ctx, legacyPath, []*prompt.PromptAtom{}); err == nil || !strings.Contains(err.Error(), "invalid corpus schema") {
		t.Fatalf("legacy corpus without embedding_model must be rejected as invalid schema, got err=%v", err)
	}
}

func TestReconcilePromptSeed_AcceptsStampedSchema(t *testing.T) {
	ctx := context.Background()
	stampedPath := createModelPinFixture(t, true)
	defer os.Remove(stampedPath)
	if _, err := reconcilePromptSeed(ctx, stampedPath, []*prompt.PromptAtom{}); err != nil {
		t.Fatalf("stamped corpus with embedding_model must validate, got err=%v", err)
	}
}
