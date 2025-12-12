// +build ignore

// Example test to demonstrate loader functionality (not meant to run as part of test suite)
package prompt_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/prompt"

	_ "github.com/mattn/go-sqlite3"
)

// ExampleLoadProjectPrompts demonstrates loading project-level prompt atoms
func ExampleLoadProjectPrompts() {
	ctx := context.Background()
	nerdDir := ".nerd"

	// Load project-level prompts (no embedding engine for this example)
	count, err := prompt.LoadProjectPrompts(ctx, nerdDir, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Loaded %d project-level prompt atoms\n", count)
}

// ExampleLoadAgentPrompts demonstrates loading agent-specific prompt atoms
func ExampleLoadAgentPrompts() {
	ctx := context.Background()
	nerdDir := ".nerd"
	agentName := "my-custom-agent"

	// Load agent-specific prompts
	count, err := prompt.LoadAgentPrompts(ctx, agentName, nerdDir, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Loaded %d prompts for agent %s\n", count, agentName)
}

// ExampleInitializePromptDatabase demonstrates creating a new prompt database
func ExampleInitializePromptDatabase() {
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

// ExampleCreateAgentPrompts demonstrates creating prompts.yaml for an agent
func ExampleCreateAgentPrompts() {
	// Create agent directory
	agentDir := filepath.Join(".nerd", "agents", "my-agent")
	os.MkdirAll(agentDir, 0755)

	// Create prompts.yaml
	yamlContent := `- id: "my-agent/identity"
  category: "identity"
  priority: 100
  is_mandatory: true
  shard_types: ["/custom"]
  content: |
    You are a custom agent with specific capabilities.

    Your mission is to...

- id: "my-agent/constraints"
  category: "safety"
  priority: 95
  is_mandatory: true
  shard_types: ["/custom"]
  depends_on: ["my-agent/identity"]
  content: |
    Safety constraints for this agent:

    1. Never...
    2. Always...
`

	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	err := os.WriteFile(promptsPath, []byte(yamlContent), 0644)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Created prompts.yaml at %s\n", promptsPath)

	// Now load the prompts
	ctx := context.Background()
	count, err := prompt.LoadAgentPrompts(ctx, "my-agent", ".nerd", nil)
	if err != nil {
		fmt.Printf("Error loading: %v\n", err)
		return
	}

	fmt.Printf("Successfully loaded %d atoms for my-agent\n", count)
}
