// Package main implements the codeNERD CLI commands.
// This file exposes the sparse retriever and tiered context builder directly,
// so the issue -> keywords -> candidates -> Mangle facts path can be exercised
// and inspected without starting a chat session.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/retrieval"
	"codenerd/internal/types"

	"github.com/spf13/cobra"
)

// =============================================================================
// RETRIEVE COMMAND - issue-driven sparse retrieval
// =============================================================================

var (
	retrieveWorkspace string
	retrieveTimeout   time.Duration
	retrieveMaxFiles  int
	retrieveShowFacts bool
	retrieveStats     bool
	retrieveRipgrep   bool
)

// retrieveCmd runs one issue-seeding pass and prints what it found.
//
// The retriever and the tiered builder were fully implemented and had no way to
// be run: no CLI surface, no VirtualStore action, and (until this pass) no call
// from the session loop either. This command is the operator-facing half of
// fixing that — it is also the quickest way to see whether the EDB surface a
// Mangle rule expects is actually being produced for a given issue.
var retrieveCmd = &cobra.Command{
	Use:   "retrieve [issue text]",
	Short: "Run issue-driven sparse retrieval over the workspace",
	Long: `Extracts keywords from an issue description, searches the workspace,
assembles the four context tiers, and reports what would be asserted into the
Mangle kernel.

Tiers:
  1  files named in the issue text (resolved to real workspace paths)
  2  files matching extracted keywords, ranked by weighted hit density
  3  import neighbors of tier 1-2 files (Go and Python)
  4  semantic expansion, falling back to a definition scan

With --facts the exact fact set is printed in Mangle syntax; with --assert it is
loaded into a kernel and read back, which is the only way to prove a fact
conforms to its Decl (a nonconformant one is dropped silently).`,
	Example: `  nerd retrieve "panic in internal/core/kernel.go when calling Evaluate()"
  nerd retrieve --facts "NullPointer in parser.go"
  nerd retrieve --stats --timeout 30s "flaky test in retry logic"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRetrieve,
}

func init() {
	retrieveCmd.Flags().StringVarP(&retrieveWorkspace, "workspace", "w", ".", "Workspace root to search")
	retrieveCmd.Flags().DurationVar(&retrieveTimeout, "timeout", retrieval.DefaultSeedTimeout, "Budget for the whole retrieval pass")
	retrieveCmd.Flags().IntVar(&retrieveMaxFiles, "max-files", 50, "Maximum files across all tiers")
	retrieveCmd.Flags().BoolVar(&retrieveShowFacts, "facts", false, "Print the Mangle facts the pass would assert")
	retrieveCmd.Flags().BoolVar(&retrieveStats, "stats", false, "Print retriever metrics (latency, cache hit rate, files walked)")
	retrieveCmd.Flags().BoolVar(&retrieveRipgrep, "ripgrep", false, "Use the ripgrep backend instead of the native scan")
}

func runRetrieve(cmd *cobra.Command, args []string) error {
	issueText := strings.Join(args, " ")

	workspace, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}
	if retrieveWorkspace != "" && retrieveWorkspace != "." {
		workspace = retrieveWorkspace
	}

	cfg := retrieval.DefaultSparseRetrieverConfig(workspace)
	cfg.SearchTimeout = retrieveTimeout
	if retrieveRipgrep {
		backend, berr := retrieval.NewRipgrepBackend()
		if berr != nil {
			return berr
		}
		cfg.Backend = backend
	}
	retriever := retrieval.NewSparseRetriever(cfg)

	// A real kernel is used rather than a stub sink: loading through it is what
	// proves the facts survive their Decl bounds, and reading them back is what
	// proves a Mangle rule could see them.
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("initializing kernel: %w", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), retrieveTimeout+5*time.Second)
	defer cancel()

	issueID := retrieval.NewIssueID()
	report, err := retrieval.SeedIssueFacts(ctx, kernel, retrieval.SeedRequest{
		IssueID:   issueID,
		IssueText: issueText,
		WorkDir:   workspace,
		Retriever: retriever,
		// resolveRetrieveEmbeddingEngine mirrors the cortex boot: nil when
		// Ollama is unavailable, and the seed then keeps the heuristic Tier 4.
		EmbeddingEngine: resolveRetrieveEmbeddingEngine(ctx, workspace),
		Timeout:         retrieveTimeout,
		MaxFiles:        retrieveMaxFiles,
	})
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("empty issue text")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "issue:      %s\n", issueID)
	fmt.Fprintf(out, "workspace:  %s\n", workspace)
	fmt.Fprintf(out, "elapsed:    %s", report.Duration.Round(time.Millisecond))
	if report.TimedOut {
		fmt.Fprint(out, "  (budget expired — partial result)")
	}
	fmt.Fprint(out, "\n\n")

	fmt.Fprintf(out, "tiers:      T1=%d  T2=%d  T3=%d  T4=%d\n",
		report.TierCounts[0], report.TierCounts[1], report.TierCounts[2], report.TierCounts[3])
	if report.SemanticTier == "embeddings" {
		fmt.Fprintln(out, "semantic:   embeddings tier (vector similarity)")
	} else {
		fmt.Fprintln(out, "semantic:   heuristic fallback (embeddings unavailable)")
	}
	fmt.Fprintf(out, "candidates: %d ranked files, %d keyword hits\n", report.Candidates, report.KeywordHits)
	fmt.Fprintf(out, "facts:      %d asserted, ~%d tokens of context\n\n", report.Facts, report.TotalTokens)

	printRetrievedTiers(cmd, kernel, issueID)

	if retrieveShowFacts {
		printRetrievalFacts(cmd, kernel)
	}
	if retrieveStats {
		fmt.Fprintf(out, "\nmetrics:    %s\n", report.Metrics)
	}
	return nil
}

// printRetrievedTiers reads the tier facts back out of the kernel rather than
// off the in-memory result, so what is shown is what a Mangle rule would see.
func printRetrievedTiers(cmd *cobra.Command, kernel *core.RealKernel, issueID string) {
	facts, err := kernel.Query("tiered_context_file")
	if err != nil || len(facts) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no tiered_context_file facts in the EDB")
		return
	}

	type row struct {
		tier      string
		path      string
		relevance int64
	}
	var rows []row
	for _, f := range facts {
		if types.ArgString(f, 0) != issueID {
			continue
		}
		rel, _ := types.ArgInt64(f, 3)
		// The tier arrives as a /name; kernel query results render names as
		// plain strings, so ArgName (not a MangleAtom assertion) is what reads it.
		rows = append(rows, row{types.ArgName(f, 2), types.ArgString(f, 1), rel})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].tier != rows[j].tier {
			return rows[i].tier < rows[j].tier
		}
		return rows[i].relevance > rows[j].relevance
	})

	out := cmd.OutOrStdout()
	for _, r := range rows {
		fmt.Fprintf(out, "  %-7s %3d%%  %s\n", strings.TrimPrefix(r.tier, "/"), r.relevance, r.path)
	}
}

func printRetrievalFacts(cmd *cobra.Command, kernel *core.RealKernel) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "\nfacts asserted:")
	for _, pred := range []string{
		"issue_text", "issue_keyword", "keyword_weight", "file_mentioned",
		"candidate_file", "keyword_hit", "context_tier", "tiered_context_file",
		"issue_context",
	} {
		facts, err := kernel.Query(pred)
		if err != nil {
			continue
		}
		for _, f := range facts {
			fmt.Fprintf(out, "  %s\n", f.String())
		}
	}
}

// resolveRetrieveEmbeddingEngine builds the optional vector backend for Tier 4
// from .nerd/config.json, mirroring how the cortex factory initializes
// cortex.EmbeddingEngine. It returns nil when Ollama is unavailable — the
// factory leaves the engine nil deliberately in that case, and the seed then
// keeps the heuristic Tier 4 fallback — so `nerd retrieve` never fails for
// want of embeddings.
func resolveRetrieveEmbeddingEngine(ctx context.Context, workspace string) embedding.EmbeddingEngine {
	cfg, _ := config.LoadUserConfig(filepath.Join(workspace, ".nerd", "config.json"))
	if cfg == nil {
		cfg = config.DefaultUserConfig()
	}
	ucEmb := cfg.GetEmbeddingConfig()
	engine, err := embedding.NewEngine(embedding.Config{
		Provider:       ucEmb.Provider,
		OllamaEndpoint: ucEmb.OllamaEndpoint,
		OllamaModel:    ucEmb.OllamaModel,
		GenAIAPIKey:    ucEmb.GenAIAPIKey,
		GenAIModel:     ucEmb.GenAIModel,
		TaskType:       ucEmb.TaskType,
	})
	if err != nil || engine == nil {
		return nil
	}
	// Same health gate the factory applies at boot: an engine that survives
	// construction but answers no query must not poison the pass.
	if checker, ok := engine.(embedding.HealthChecker); ok {
		hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if herr := checker.HealthCheck(hctx); herr != nil {
			if closer, ok := engine.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
			return nil
		}
	}
	return engine
}