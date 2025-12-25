package main

import (
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// queryCmd queries the Mangle fact store
var queryCmd = &cobra.Command{
	Use:   "query [predicate]",
	Short: "Query facts from the Mangle kernel",
	Long: `Queries the fact store for matching predicates.
Returns all derived facts for the given predicate.

Example:
  nerd query next_action
  nerd query impacted`,
	Args: cobra.ExactArgs(1),
	RunE: queryFacts,
}

// statusCmd shows system status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show codeNERD system status",
	RunE:  showStatus,
}

// whyCmd explains the reasoning behind the last action
var whyCmd = &cobra.Command{
	Use:   "why [predicate]",
	Short: "Explain why an action was taken or blocked",
	Long: `Shows the derivation trace (proof tree) for a logical conclusion.

This implements the "Glass Box" interface, allowing you to understand
why codeNERD made a specific decision.

Examples:
  nerd why blocked      # Why was the last action blocked?
  nerd why next_action  # Why was this action chosen?
  nerd why impacted     # What files would be impacted?`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWhy,
}

// queryFacts queries the Mangle kernel
func queryFacts(cmd *cobra.Command, args []string) error {
	predicate := args[0]
	logger.Info("Querying facts", zap.String("predicate", predicate))

	baseCtx := cmd.Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex to load all persisted facts (including scan.mg)
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	facts, err := cortex.Kernel.Query(predicate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(facts) == 0 {
		fmt.Printf("No facts found for predicate '%s'\n", predicate)
		return nil
	}

	fmt.Printf("Facts for '%s':\n", predicate)
	for _, fact := range facts {
		fmt.Printf("  %s\n", fact.String())
	}
	return nil
}

// showStatus displays system status
func showStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("codeNERD System Status")
	fmt.Println("======================")
	fmt.Printf("Version: Cortex 1.5.0\n")
	fmt.Printf("Kernel:  Google Mangle (Datalog)\n")
	fmt.Printf("Runtime: Go %s\n", "1.24")
	fmt.Println()

	// Check API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}
	if key != "" {
		fmt.Println("✓ Z.AI API key configured")
	} else {
		fmt.Println("✗ Z.AI API key not configured")
	}

	// Check workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	fmt.Printf("✓ Workspace: %s\n", cwd)

	// Initialize kernel and show stats
	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	fmt.Printf("✓ Mangle kernel initialized\n")
	fmt.Printf("  Schemas: %d bytes\n", len(kern.GetSchemas()))
	fmt.Printf("  Policy:  %d bytes\n", len(kern.GetPolicy()))

	return nil
}

// runWhy explains the reasoning behind decisions
func runWhy(cmd *cobra.Command, args []string) error {
	predicate := "next_action"
	if len(args) > 0 {
		predicate = args[0]
	}

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if initialized
	if !nerdinit.IsInitialized(cwd) {
		fmt.Println("Project not initialized. Run 'nerd init' first.")
		return nil
	}

	fmt.Printf("Explaining derivation for: %s\n", predicate)
	fmt.Println(strings.Repeat("=", 40))

	// Initialize kernel
	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	if err := kern.LoadFacts(nil); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Query for facts
	facts, err := kern.Query(predicate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(facts) == 0 {
		fmt.Printf("No facts found for predicate '%s'\n", predicate)
		fmt.Println("\nPossible reasons:")
		fmt.Println("  - No matching rules were triggered")
		fmt.Println("  - Required preconditions were not met")
		fmt.Println("  - The workspace has not been scanned recently")
		return nil
	}

	// Display derivation
	fmt.Printf("\nDerived %d fact(s):\n\n", len(facts))
	for i, fact := range facts {
		fmt.Printf("  %d. %s\n", i+1, fact.String())
	}

	// Show related rules (simplified - full proof tree would require Mangle integration)
	fmt.Println("\nRelated Policy Rules:")
	switch predicate {
	case "next_action":
		fmt.Println("  - next_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).")
		fmt.Println("  - next_action(/ask_user) :- clarification_needed(_).")
	case "block_commit":
		fmt.Println("  - block_commit(R) :- diagnostic(_, _, _, /error, R).")
		fmt.Println("  - block_commit(\"Untested\") :- impacted(F), !test_coverage(F).")
	case "impacted":
		fmt.Println("  - impacted(X) :- modified(Y), depends_on(X, Y).")
		fmt.Println("  - impacted(X) :- impacted(Y), depends_on(X, Y). # Transitive")
	case "permitted":
		fmt.Println("  - permitted(A) :- safe_action(A).")
		fmt.Println("  - permitted(A) :- dangerous_action(A), user_override(A).")
	default:
		fmt.Println("  (No specific rules documented for this predicate)")
	}

	return nil
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

// sanitizeFactForMangle sanitizes fact arguments to be Mangle-parser friendly.
// Mangle's parser doesn't handle certain escape sequences like \r \n \t in strings.
func sanitizeFactForMangle(fact types.Fact) types.Fact {
	sanitized := types.Fact{
		Predicate: fact.Predicate,
		Args:      make([]interface{}, len(fact.Args)),
	}

	for i, arg := range fact.Args {
		switch v := arg.(type) {
		case string:
			// Replace newlines, carriage returns, tabs with spaces
			s := strings.ReplaceAll(v, "\r\n", " ")
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.ReplaceAll(s, "\r", " ")
			s = strings.ReplaceAll(s, "\t", " ")
			// Collapse multiple spaces into one
			for strings.Contains(s, "  ") {
				s = strings.ReplaceAll(s, "  ", " ")
			}
			sanitized.Args[i] = strings.TrimSpace(s)
		default:
			sanitized.Args[i] = arg
		}
	}

	return sanitized
}
