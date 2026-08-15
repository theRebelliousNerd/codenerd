package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	internalcontext "codenerd/internal/context"

	"github.com/spf13/cobra"
)

var (
	contextStatsWorkspace string
	contextStatsTop       int
	contextStatsJSON      bool
)

// contextStatsCmd exposes the third learning loop (context learning) to
// operators. The feedback store has recorded helpful/noise ratings per
// predicate since it was wired at chat boot, but nothing ever read them back
// out: the data was write-only, so nobody could tell whether context learning
// was working or which predicates it had learned to distrust.
var contextStatsCmd = &cobra.Command{
	Use:   "context-stats",
	Short: "Inspect learned context usefulness (helpful vs noise predicates)",
	Long: `Reads the context feedback database written by the compression system and
reports which Mangle predicates the LLM found helpful versus noisy.

The store lives at <workspace>/.nerd/context_feedback.db and is populated as
chat turns complete. A predicate needs at least MinSamples observations before
its learned score is allowed to move activation scoring — predicates below that
floor are "not yet trusted", not "neutral", and are excluded from both tables.

Weighted score ranges from -1.0 (always noise) to +1.0 (always helpful) and is
mapped to -20..+20 activation points by the feedback score component.

Examples:
  nerd context-stats
  nerd context-stats --top 25
  nerd context-stats --json --workspace /path/to/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace := contextStatsWorkspace
		if workspace == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve workspace: %w", err)
			}
			workspace = cwd
		}

		dbPath := filepath.Join(workspace, ".nerd", "context_feedback.db")
		if _, err := os.Stat(dbPath); err != nil {
			return fmt.Errorf("no context feedback database at %s (run a chat session first)", dbPath)
		}

		store, err := internalcontext.NewContextFeedbackStore(dbPath)
		if err != nil {
			return fmt.Errorf("open feedback store: %w", err)
		}
		defer store.Close()

		stats := internalcontext.CollectFeedbackStats(store, contextStatsTop)
		if stats.Err != nil {
			return fmt.Errorf("read feedback stats: %w", stats.Err)
		}

		out := cmd.OutOrStdout()
		if contextStatsJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(stats)
		}

		fmt.Fprintf(out, "Context feedback: %s\n", dbPath)
		fmt.Fprintf(out, "  turns rated      : %d\n", stats.TotalFeedback)
		fmt.Fprintf(out, "  avg usefulness   : %.3f\n", stats.AvgUsefulness)
		fmt.Fprintf(out, "  min samples/pred : %d\n\n", stats.MinSamples)

		printPredicateTable(out, "HELPFUL (boosted into context)", stats.Helpful)
		fmt.Fprintln(out)
		printPredicateTable(out, "NOISE (penalised out of context)", stats.Noise)

		if len(stats.Helpful) == 0 && len(stats.Noise) == 0 {
			fmt.Fprintf(out, "\nNo predicate has reached %d observations yet; scoring is unaffected so far.\n", stats.MinSamples)
		}
		return nil
	},
}

func printPredicateTable(out io.Writer, title string, rows []internalcontext.PredicateFeedback) {
	fmt.Fprintf(out, "%s\n", title)
	if len(rows) == 0 {
		fmt.Fprintln(out, "  (none above the sample floor)")
		return
	}
	fmt.Fprintf(out, "  %-36s %8s %8s %8s %8s\n", "PREDICATE", "HELPFUL", "NOISE", "TOTAL", "SCORE")
	for _, r := range rows {
		fmt.Fprintf(out, "  %-36s %8d %8d %8d %+8.3f\n",
			r.Predicate, r.HelpfulCount, r.NoiseCount, r.TotalMentions, r.WeightedScore)
	}
}

func init() {
	contextStatsCmd.Flags().StringVar(&contextStatsWorkspace, "workspace", "", "Workspace root (default: current directory)")
	contextStatsCmd.Flags().IntVar(&contextStatsTop, "top", 15, "How many predicates to show in each table")
	contextStatsCmd.Flags().BoolVar(&contextStatsJSON, "json", false, "Emit JSON instead of tables")
}
