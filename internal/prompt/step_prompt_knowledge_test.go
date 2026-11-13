package prompt

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// stepPromptExperts are the experts the kernel pushed into the measured step
// (a Go edit of internal/core/kernel_query.go, 2026-11-12).
var stepPromptExperts = []string{"goexpert", "mangleexpert", "mangle-logic-architect"}

// TestStepPromptKnowledgeShare recompiles a planned step's system prompt
// against copies of a workspace's real knowledge bases (the directory in
// CODENERD_STEP_PROMPT_NERD_DIR, normally the repo's .nerd) and reports the
// prompt's size and how much of it is retrieved knowledge. Rule selection
// and lexical retrieval only: no embedding engine, no Jev.
func TestStepPromptKnowledgeShare(t *testing.T) {
	nerdDir := os.Getenv("CODENERD_STEP_PROMPT_NERD_DIR")
	if nerdDir == "" {
		t.Skip("set CODENERD_STEP_PROMPT_NERD_DIR=<workspace>/.nerd to measure a step prompt against real knowledge bases")
	}
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := NewJITPromptCompiler(WithKernel(&realKernelQuerier{k: k}), WithEmbeddedCorpus(corpus))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	compiler.SetLocalDB(copyProjectKnowledge(t, filepath.Join(nerdDir, "knowledge.db")))
	for _, key := range stepPromptExperts {
		src := filepath.Join(nerdDir, "shards", key+"_knowledge.db")
		dst := filepath.Join(t.TempDir(), key+"_knowledge.db")
		copyFile(t, src, dst)
		db, err := sql.Open("sqlite3", dst)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := compiler.RegisterShardDB(key, db); err != nil {
			t.Fatal(err)
		}
	}

	cc := coderTurnContext(t, "/fix", "internal/core/kernel_query.go", "/go", kernelTargetNeeds(t, k, "/go"))
	cc.Experts = append([]string(nil), stepPromptExperts...)
	cc.SemanticQuery = "STEP internal/core/kernel_query.go :: answer a bound pattern query from the store's argument index instead of scanning the relation"
	res, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	retrievedRows, retrievedChars, pointers := 0, 0, 0
	for _, a := range res.IncludedAtoms {
		switch {
		case a.RetrievedContext:
			retrievedRows++
			retrievedChars += len(a.Content)
		case strings.HasPrefix(a.ID, "knowledge-withheld"):
			pointers++
		}
	}
	t.Logf("step prompt: %d chars total, %d retrieved rows / %d chars, %d withheld-pointer atom(s)", len(res.Prompt), retrievedRows, retrievedChars, pointers)
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

// copyProjectKnowledge copies a project knowledge.db's knowledge rows (not
// its vectors: it can be gigabytes) into a fresh store.
func copyProjectKnowledge(t *testing.T, path string) *store.LocalStore {
	t.Helper()
	src, err := sql.Open("sqlite3", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	rows, err := src.Query(`SELECT concept, content, confidence, COALESCE(source, '') FROM knowledge_atoms ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	dst, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dst.Close() })
	for rows.Next() {
		var concept, content, source string
		var confidence float64
		if err := rows.Scan(&concept, &content, &confidence, &source); err != nil {
			t.Fatal(err)
		}
		if err := dst.StoreKnowledgeAtom(concept, content, confidence, source); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}
