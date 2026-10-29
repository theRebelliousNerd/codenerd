// Package main implements the codeNERD CLI commands.
// This file contains the check-kb command: the knowledge-fill audit
// (internal/kbfill) over a workspace's shard-agent knowledge bases.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"codenerd/internal/kbfill"

	"github.com/spf13/cobra"
)

var checkKBCmd = &cobra.Command{
	Use:   "check-kb",
	Short: "Name every unfilled field of the workspace's shard-agent knowledge bases",
	Long: `Audits every .nerd/shards/<agent>_knowledge.db in the workspace with the
knowledge-fill gate (internal/kbfill): each column a row should carry and does
not, each table that should hold rows and is empty. The same audit runs over the
embedded corpora in the repository's tests, against a baseline that may only
shrink. Exits 1 when any gap is found.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		gaps, err := auditShardKBs(cmd.Context(), workspaceRootOrCwd())
		if err != nil {
			return err
		}
		if printKBGaps(cmd.OutOrStdout(), gaps) > 0 {
			return fmt.Errorf("%d knowledge-base gap(s)", len(gaps))
		}
		return nil
	},
}

// auditShardKBs audits each shard agent's KB under workspace, by file.
func auditShardKBs(ctx context.Context, workspace string) (map[string][]kbfill.Gap, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	paths, err := filepath.Glob(filepath.Join(workspace, ".nerd", "shards", "*_knowledge.db"))
	if err != nil {
		return nil, err
	}
	out := make(map[string][]kbfill.Gap, len(paths))
	for _, path := range paths {
		db, err := sql.Open("sqlite3", "file:"+path+"?mode=ro")
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		gaps, auditErr := kbfill.Audit(ctx, db, kbfill.ShardAgentKB)
		_ = db.Close()
		if auditErr != nil {
			return nil, fmt.Errorf("audit %s: %w", path, auditErr)
		}
		out[filepath.Base(path)] = gaps
	}
	return out, nil
}

// printKBGaps writes each KB's gaps and returns how many there were.
func printKBGaps(w io.Writer, byKB map[string][]kbfill.Gap) int {
	names := make([]string, 0, len(byKB))
	for name := range byKB {
		names = append(names, name)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		gaps := byKB[name]
		if len(gaps) == 0 {
			fmt.Fprintf(w, "%s: filled\n", name)
			continue
		}
		fmt.Fprintf(w, "%s: %d gap(s)\n", name, len(gaps))
		for _, g := range gaps {
			fmt.Fprintf(w, "  %s\n", g)
		}
		total += len(gaps)
	}
	if len(names) == 0 {
		fmt.Fprintln(w, "no shard-agent knowledge bases under .nerd/shards")
	}
	return total
}
