// Package main implements the codeNERD CLI commands.
// This file contains the check-kb command: the knowledge-fill audit
// (internal/kbfill) over a workspace's shard-agent knowledge bases, with what
// each KB knows by source and how each knowledge source fared.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/kbfill"
	"codenerd/internal/knowledge"
	"codenerd/internal/sqlpragmas"

	"github.com/spf13/cobra"
)

var checkKBCmd = &cobra.Command{
	Use:   "check-kb",
	Short: "Name every unfilled field of the workspace's shard-agent knowledge bases",
	Long: `Audits every .nerd/shards/<agent>_knowledge.db in the workspace with the
knowledge-fill gate (internal/kbfill): each column a row should carry and does
not, each table that should hold rows and is empty, and knowledge that is only
templated placeholders. For each KB it also prints its knowledge by source and
every knowledge source's last outcome that did not acquire (failed, empty,
unavailable), so a source that did not deliver is visible. The same audit runs
over the embedded corpora in the repository's tests, against a baseline that
may only shrink. Exits 1 when any gap is found.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		reports, err := auditShardKBs(cmd.Context(), workspaceRootOrCwd())
		if err != nil {
			return err
		}
		if n := printKBReports(cmd.OutOrStdout(), reports); n > 0 {
			return fmt.Errorf("%d knowledge-base gap(s)", n)
		}
		return nil
	},
}

// kbReport is one shard-agent KB's audit.
type kbReport struct {
	Gaps []kbfill.Gap
	// AtomsBySource counts knowledge rows by source family ("web/openai",
	// "context7", "godoc", "repo"); placeholder rows count as "placeholder".
	AtomsBySource map[string]int
	Outcomes      []knowledge.Outcome
}

// auditShardKBs audits each shard agent's KB under workspace, by file.
func auditShardKBs(ctx context.Context, workspace string) (map[string]kbReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	paths, err := filepath.Glob(filepath.Join(workspace, ".nerd", "shards", "*_knowledge.db"))
	if err != nil {
		return nil, err
	}
	out := make(map[string]kbReport, len(paths))
	for _, path := range paths {
		// A read-only audit: the read-only pragma profile, like every other
		// SQLite open (the open-site guard in internal/store).
		db, err := sqlpragmas.OpenWithPragmas("sqlite3", "file:"+path+"?mode=ro", sqlpragmas.ProfileReadOnly)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		report, auditErr := auditShardKB(ctx, db)
		_ = db.Close()
		if auditErr != nil {
			return nil, fmt.Errorf("audit %s: %w", path, auditErr)
		}
		out[filepath.Base(path)] = report
	}
	return out, nil
}

func auditShardKB(ctx context.Context, db *sql.DB) (kbReport, error) {
	gaps, err := kbfill.Audit(ctx, db, kbfill.ShardAgentKB)
	if err != nil {
		return kbReport{}, err
	}
	bySource, err := atomsBySource(ctx, db)
	if err != nil {
		return kbReport{}, err
	}
	outcomes, err := knowledge.Outcomes(ctx, db)
	if err != nil {
		return kbReport{}, fmt.Errorf("read acquisition outcomes: %w", err)
	}
	return kbReport{Gaps: gaps, AtomsBySource: bySource, Outcomes: outcomes}, nil
}

// atomsBySource counts a KB's knowledge rows by source family.
func atomsBySource(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT concept, content, COALESCE(source, '') FROM knowledge_atoms`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return map[string]int{}, nil
		}
		return nil, fmt.Errorf("read knowledge: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var concept, content, source string
		if err := rows.Scan(&concept, &content, &source); err != nil {
			return nil, err
		}
		if knowledge.IsLegacyPlaceholder(concept, content) {
			out["placeholder"]++
			continue
		}
		out[knowledge.SourceFamily(source)]++
	}
	return out, rows.Err()
}

// printKBReports writes each KB's gaps, knowledge by source and the outcomes
// that did not acquire, and returns how many gaps there were.
func printKBReports(w io.Writer, byKB map[string]kbReport) int {
	names := make([]string, 0, len(byKB))
	for name := range byKB {
		names = append(names, name)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		r := byKB[name]
		if len(r.Gaps) == 0 {
			fmt.Fprintf(w, "%s: filled\n", name)
		} else {
			fmt.Fprintf(w, "%s: %d gap(s)\n", name, len(r.Gaps))
			for _, g := range r.Gaps {
				fmt.Fprintf(w, "  %s\n", g)
			}
			total += len(r.Gaps)
		}
		fmt.Fprintf(w, "  knowledge: %s\n", formatSourceCounts(r.AtomsBySource))
		if len(r.Outcomes) == 0 {
			fmt.Fprintln(w, "  sources: no acquisition recorded")
			continue
		}
		fmt.Fprintf(w, "  sources: %s\n", knowledge.SummarizeOutcomes(r.Outcomes))
		for _, o := range r.Outcomes {
			if o.Status != knowledge.StatusAcquired {
				fmt.Fprintf(w, "    %s\n", o)
			}
		}
	}
	if len(names) == 0 {
		fmt.Fprintln(w, "no shard-agent knowledge bases under .nerd/shards")
	}
	return total
}

func formatSourceCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}
