package prompt_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"

	_ "github.com/mattn/go-sqlite3"
)

// Example_syncAgentPrompts demonstrates syncing one agent's prompts.yaml into
// its knowledge database. An unchanged file is skipped by content hash.
func Example_syncAgentPrompts() {
	ctx := context.Background()
	workspace := "."
	agentName := "my-custom-agent"

	// No embedding engine for this example.
	synchronizer := prsync.NewAgentSynchronizer(workspace, prompt.NewAtomLoader(nil))
	skipped, err := synchronizer.SyncAgent(ctx, agentName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Synced agent %s (skipped=%v)\n", agentName, skipped)
}

// Example_initializePromptDatabase demonstrates creating a new prompt database.
func Example_initializePromptDatabase() {
	ctx := context.Background()
	dbPath := filepath.Join(".nerd", "prompts", "corpus.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer db.Close()

	loader := prompt.NewAtomLoader(nil)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Initialized corpus database at %s\n", dbPath)
}

// Example_createAgentPrompts demonstrates writing prompts.yaml for an agent
// and syncing it.
func Example_createAgentPrompts() {
	workspace := "."
	agentDir := filepath.Join(workspace, ".nerd", "agents", "my-agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	yamlContent := `- id: "my-agent/identity"
  category: "identity"
  is_mandatory: true
  shard_types: ["/custom"]
  content: |
    You are a custom agent with specific capabilities.

    Your mission is to...

- id: "my-agent/constraints"
  category: "safety"
  is_mandatory: true
  shard_types: ["/custom"]
  depends_on: ["my-agent/identity"]
  content: |
    Safety constraints for this agent:

    1. Never...
    2. Always...
`

	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	if err := os.WriteFile(promptsPath, []byte(yamlContent), 0644); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Created prompts.yaml at %s\n", promptsPath)

	synchronizer := prsync.NewAgentSynchronizer(workspace, prompt.NewAtomLoader(nil))
	if _, err := synchronizer.SyncAgent(context.Background(), "my-agent"); err != nil {
		fmt.Printf("Error syncing: %v\n", err)
		return
	}
	fmt.Println("Synced my-agent")
}
