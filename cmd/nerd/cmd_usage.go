package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"codenerd/internal/usage"

	"github.com/spf13/cobra"
)

var (
	usageJSON  bool
	usageTopN  int
	usageGroup string
)

// usageGroups maps the --group flag to a stats accessor, so the command has one
// rendering path rather than a switch per dimension.
var usageGroups = []struct {
	name string
	get  func(usage.AggregatedStats) map[string]usage.TokenCounts
}{
	{"provider", func(s usage.AggregatedStats) map[string]usage.TokenCounts { return s.ByProvider }},
	{"model", func(s usage.AggregatedStats) map[string]usage.TokenCounts { return s.ByModel }},
	{"shard-type", func(s usage.AggregatedStats) map[string]usage.TokenCounts { return s.ByShardType }},
	{"shard-name", func(s usage.AggregatedStats) map[string]usage.TokenCounts { return s.ByShardName }},
	{"operation", func(s usage.AggregatedStats) map[string]usage.TokenCounts { return s.ByOperation }},
	{"session", func(s usage.AggregatedStats) map[string]usage.TokenCounts { return s.BySession }},
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show token usage and estimated spend for this workspace",
	Long: `Reads .nerd/usage.json and reports token totals broken down by provider,
model, shard type, shard name, operation and session.

Costs are estimates from a built-in list-price table, not billing truth. Tokens
spent on models absent from that table are reported separately so a low total
is never mistaken for a cheap one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := workspace
		if root == "" {
			root = "."
		}

		// NewTracker only reads; it does not start the auto-save timer until a
		// Track call, so this is a safe read-only use.
		tracker, err := usage.NewTracker(root)
		if err != nil {
			return fmt.Errorf("read usage data: %w", err)
		}
		stats := tracker.Stats()

		if usageJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(stats)
		}

		if stats.TotalProject.Total == 0 {
			fmt.Printf("No usage recorded yet for %s.\n", root)
			return nil
		}

		renderUsageSummary(os.Stdout, stats, usageGroup, usageTopN)
		return nil
	},
}

// renderUsageSummary writes the human-readable report.
func renderUsageSummary(out *os.File, stats usage.AggregatedStats, only string, topN int) {
	total := stats.TotalProject
	fmt.Fprintf(out, "Token usage\n")
	fmt.Fprintf(out, "  input   %s\n", humanInt(total.Input))
	fmt.Fprintf(out, "  output  %s\n", humanInt(total.Output))
	fmt.Fprintf(out, "  total   %s\n", humanInt(total.Total))
	fmt.Fprintf(out, "  cost    %s (estimate)\n", formatUSD(total.Cost))
	if stats.UnpricedTokens > 0 {
		fmt.Fprintf(out, "          %s tokens ran on models with no price entry and are excluded\n",
			humanInt(stats.UnpricedTokens))
	}
	fmt.Fprintln(out)

	for _, g := range usageGroups {
		if only != "" && only != g.name {
			continue
		}
		renderUsageGroup(out, g.name, g.get(stats), topN)
	}
}

// renderUsageGroup prints one breakdown table, ordered by spend, capped at topN.
// The dropped-row count is always printed: a silent cap reads as "that was
// everything".
func renderUsageGroup(out *os.File, title string, data map[string]usage.TokenCounts, topN int) {
	if len(data) == 0 {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if data[keys[i]].Total != data[keys[j]].Total {
			return data[keys[i]].Total > data[keys[j]].Total
		}
		return keys[i] < keys[j]
	})

	shown := keys
	if topN > 0 && len(shown) > topN {
		shown = shown[:topN]
	}

	fmt.Fprintf(out, "By %s\n", title)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tINPUT\tOUTPUT\tTOTAL\tCOST")
	for _, k := range shown {
		c := data[k]
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
			truncateName(k, 32), humanInt(c.Input), humanInt(c.Output), humanInt(c.Total), formatUSD(c.Cost))
	}
	w.Flush()
	if dropped := len(keys) - len(shown); dropped > 0 {
		fmt.Fprintf(out, "  … %d more row(s) not shown (use --top 0 for all)\n", dropped)
	}
	fmt.Fprintln(out)
}

// formatUSD renders an estimated cost, keeping sub-cent amounts visible.
func formatUSD(cost float64) string {
	switch {
	case cost == 0:
		return "-"
	case cost < 0.01:
		return fmt.Sprintf("$%.4f", cost)
	default:
		return fmt.Sprintf("$%.2f", cost)
	}
}

// humanInt renders n with thousands separators.
func humanInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func truncateName(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}

func init() {
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "emit aggregated stats as JSON for scripting")
	usageCmd.Flags().IntVar(&usageTopN, "top", 10, "rows per breakdown table (0 for all)")
	usageCmd.Flags().StringVar(&usageGroup, "group", "",
		"show only one breakdown: provider, model, shard-type, shard-name, operation, session")
}
