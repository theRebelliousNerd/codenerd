// Package main implements knowledge base CLI commands for codeNERD.
// This file handles knowledge listing, searching, and management.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	coresys "codenerd/internal/system"

	"github.com/spf13/cobra"
)

// =============================================================================
// KNOWLEDGE BASE COMMANDS
// =============================================================================

// knowledgeCmd shows knowledge base info
var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "View and search knowledge base",
	Long: `View and search the codeNERD knowledge base.

Subcommands:
  list    - List recent knowledge entries
  search  - Search knowledge semantically`,
	RunE: runKnowledgeList,
}

// knowledgeListCmd lists knowledge entries
var knowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent knowledge entries",
	RunE:  runKnowledgeList,
}

// knowledgeSearchCmd searches knowledge
var knowledgeSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search knowledge semantically",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runKnowledgeSearch,
}

func runKnowledgeList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	if cortex.LocalDB == nil {
		fmt.Println("⚠️  No knowledge database available")
		return nil
	}

	atoms, err := cortex.LocalDB.GetKnowledgeAtomsByPrefix("session/")
	if err != nil || len(atoms) == 0 {
		fmt.Println("No knowledge entries found.")
		return nil
	}

	fmt.Println("📚 Knowledge Base")
	fmt.Println(strings.Repeat("─", 60))

	limit := 10
	if len(atoms) < limit {
		limit = len(atoms)
	}

	for i := 0; i < limit; i++ {
		atom := atoms[i]
		concept := atom.Concept
		if concept == "" {
			concept = "(unknown)"
		}
		created := atom.CreatedAt.Format("2006-01-02 15:04")
		fmt.Printf("%2d. %-40s  %s\n", i+1, truncateStr(concept, 40), created)
	}

	if len(atoms) > limit {
		fmt.Printf("\n... and %d more entries\n", len(atoms)-limit)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Use: nerd knowledge search <query>")

	return nil
}

func runKnowledgeSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	if cortex.LocalDB == nil {
		fmt.Println("⚠️  No knowledge database available")
		return nil
	}

	fmt.Printf("🔍 Searching: %s\n", query)
	fmt.Println(strings.Repeat("─", 60))

	atoms, err := cortex.LocalDB.SearchKnowledgeAtomsSemantic(ctx, query, 5)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(atoms) == 0 {
		fmt.Println("No matching knowledge found.")
		return nil
	}

	for i, atom := range atoms {
		fmt.Printf("\n### %d. %s\n", i+1, atom.Concept)
		fmt.Println(strings.Repeat("─", 40))
		content := atom.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Println(content)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))

	return nil
}

// truncateStr truncates a string with ellipsis
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	knowledgeCmd.AddCommand(knowledgeListCmd, knowledgeSearchCmd)
}
