package main

import (
	"fmt"
	"strings"

	working "codenerd/internal/context"
	nerdinit "codenerd/internal/init"

	"github.com/spf13/cobra"
)

// renderWorkspaceState is the workspace half of `nerd status`: what `nerd
// init` recorded about the project, the session a chat would resume, and the
// working-context archives with the retention policy's verdict on them. All
// three are read from .nerd/ without booting anything.
//
// The project profile (.nerd/profile.json) and the session state had writers
// and no reader outside init; status printed neither, and the archive count
// that program-of-work item 15 is about was visible only to `du`.
func renderWorkspaceState(ws string) string {
	var b strings.Builder
	if profile, err := nerdinit.LoadProjectProfile(ws); err == nil && profile != nil {
		parts := []string{}
		for _, p := range []string{profile.Language, profile.Framework, profile.BuildSystem} {
			if p = strings.TrimSpace(p); p != "" {
				parts = append(parts, p)
			}
		}
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			name = "(unnamed)"
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "✓ Project: %s (%s)\n", name, strings.Join(parts, ", "))
		} else {
			fmt.Fprintf(&b, "✓ Project: %s\n", name)
		}
	} else {
		b.WriteString("✗ Project: no .nerd/profile.json (run `nerd init`)\n")
	}
	if id, err := nerdinit.GetLatestSession(ws); err == nil && strings.TrimSpace(id) != "" {
		fmt.Fprintf(&b, "  Latest session: %s\n", id)
	}
	if report, err := working.SurveyWorkingArchives(ws); err != nil {
		fmt.Fprintf(&b, "✗ Working context: could not survey .nerd/context: %v\n", err)
	} else {
		fmt.Fprintf(&b, "  Working context: %s\n", report)
		if report.Prunable > 0 {
			b.WriteString("    `nerd memory prune` removes the prunable ones; the next session start does too\n")
		}
	}
	return b.String()
}

var memoryPruneDryRun bool

// memoryPruneCmd removes the working-context archives the retention policy
// (internal/context/working_retention.mg) derives prunable.
var memoryPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove working-context archives nothing can redeem",
	Long: `Every tool loop keeps its observations in an archive under .nerd/context/,
one per working scope, so recall_context can serve them after they leave the
window. A scope is named at random and held only by the executor that minted
it, so an archive whose owner process is gone, or that its executor retired
when its task ended, can never be read again.

This removes exactly those, as working_retention.mg derives them. Archives of
running processes, and of processes on another host, are kept. Session start
does the same prune; this command is for doing it now.

  nerd memory prune            # remove what is prunable
  nerd memory prune --dry-run  # only report`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ws := workspace
		if strings.TrimSpace(ws) == "" {
			ws = "."
		}
		report, err := runMemoryPrune(ws, memoryPruneDryRun)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), report)
		if len(report.Failures) > 0 {
			return fmt.Errorf("%d archive(s) could not be removed: %s", len(report.Failures), strings.Join(report.Failures, "; "))
		}
		return nil
	},
}

func runMemoryPrune(ws string, dryRun bool) (working.WorkingArchiveReport, error) {
	if dryRun {
		return working.SurveyWorkingArchives(ws)
	}
	return working.PruneWorkingArchives(ws)
}

func init() {
	memoryPruneCmd.Flags().BoolVar(&memoryPruneDryRun, "dry-run", false, "report what would be removed, remove nothing")
}
