// Package main implements knowledge base CLI commands for codeNERD.
// This file handles knowledge listing, searching, and management.
package main

import (
	"context"
	"fmt"
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

	key := resolveAPIKey(apiKey, workspace)

	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	if cortex.LocalDB == nil {
		fmt.Println("⚠️  No knowledge database available")
		return nil
	}

	// List from the vector store, which is what the live semantic path reads.
	//
	// This used to call GetKnowledgeAtomsByPrefix("session/"), which reads the
	// knowledge_atoms table and keeps only concepts starting with "session/".
	// Both halves of that were wrong here: the atoms with embeddings live in
	// the vectors table (1,417 of them in this workspace, every one
	// content_type=knowledge_atom, while knowledge_atoms held 0), and no atom
	// the system writes uses a "session/" prefix. So `nerd knowledge` reported
	// an empty knowledge base while `nerd knowledge search` answered every
	// query from the same database.
	const listLimit = 10
	atoms, err := cortex.LocalDB.RecentKnowledgeAtoms(listLimit)
	if err != nil {
		// An error is not an empty knowledge base. Reporting both the same way
		// is how a broken read gets mistaken for a cold start.
		return fmt.Errorf("failed to read the knowledge base: %w", err)
	}
	if len(atoms) == 0 {
		fmt.Println("No knowledge entries found.")
		fmt.Println("Run 'nerd init' to build the knowledge base, or 'nerd scan' to refresh the index.")
		return nil
	}

	fmt.Println("📚 Knowledge Base")
	fmt.Println(strings.Repeat("─", 60))

	for i, atom := range atoms {
		concept := atom.Concept
		if concept == "" {
			// Corpus atoms carry their subject in the content, not always in a
			// concept tag; showing "(unknown)" for all of them hides the entry.
			concept = truncateStr(strings.TrimSpace(strings.ReplaceAll(atom.Content, "\n", " ")), 48)
		}
		created := atom.CreatedAt.Format("2006-01-02 15:04")
		fmt.Printf("%2d. %-50s  %s\n", i+1, truncateStr(concept, 50), created)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Use: nerd knowledge search <query>")

	return nil
}

func runKnowledgeSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	key := resolveAPIKey(apiKey, workspace)

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
