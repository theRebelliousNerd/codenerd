package init

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/prompt"
)

func TestDiscoveredPromptOnlyExpertGetsDurableKnowledgeSchema(t *testing.T) {
	root := t.TempDir()
	nerdDir := filepath.Join(root, ".nerd")
	agentDir := filepath.Join(nerdDir, "agents", "customexpert")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nerdDir, "shards"), 0755); err != nil {
		t.Fatal(err)
	}
	data := buildPromptsYAML("customexpert", "CustomExpert", "custom", "    - custom", "custom", "curated method", "curated domain")
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(nerdDir, "shards", "customexpert_knowledge.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := prompt.NewAtomLoader(nil).EnsureSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := ValidateAgentDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if before.Valid {
		t.Fatal("negative control: prompt-only database must fail the knowledge contract")
	}
	if err := initializeDiscoveredAgentStores(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	after, err := ValidateAgentDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Valid || after.TotalAtoms != 0 {
		t.Fatalf("want valid empty knowledge schema, got %+v", after)
	}
	got, err := os.ReadFile(filepath.Join(agentDir, "prompts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != data {
		t.Fatal("curated prompts changed")
	}
	// Reopening/repeating initialization must remain valid and invent no facts.
	if err := initializeDiscoveredAgentStores(context.Background(), root); err != nil {
		t.Fatal(err)
	}
}
