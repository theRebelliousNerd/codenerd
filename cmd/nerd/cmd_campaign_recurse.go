package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"codenerd/internal/campaign"
	"codenerd/internal/gates"
	"codenerd/internal/northstar"
	coresys "codenerd/internal/system"

	"github.com/spf13/cobra"
)

// `nerd campaign recurse` -- the improve-everything loop over the workspace it
// runs in (Docs/journeys/10-forever-loop.md). One node at a time, leaves
// first: run the node's own gates, let the kernel pick a finding, hand it to
// the model as a one-task campaign, re-run the gates, and keep the change only
// if the finding is gone and no gate is worse -- otherwise revert every write.
// Then the next node, and after the top of the graph, the next pass.
//
// It runs until stopped: Ctrl+C, or `nerd campaign recurse stop` from another
// shell. --waves N bounds a run to N passes. Kept changes are commits on
// nerd/recurse; the ledger is .nerd/recurse/journal.jsonl, and a killed run
// resumes from it. `nerd campaign recurse status` reads the ledger.

var (
	recurseWaves       int
	recurseSubsystems  []string
	recurseContextSize int
	recurseDryRun      bool
	recursePlan        bool
)

var campaignRecurseCmd = &cobra.Command{
	Use:   "recurse",
	Short: "Improve the workspace node by node, bottom to top, pass after pass",
	Long: `Improve the workspace node by node, bottom to top, pass after pass.

The sweep order is derived from the workspace's own imports (Go, Python,
JS/TS, Rust), and the gates are the workspace's own: nerd.md commands and
gates, else what go.mod, pyproject.toml, package.json or Cargo.toml offer.

For each node, leaves first: run its gates; the kernel picks the finding to
attempt; the model gets one campaign to fix it; the gates run again. A fix is
kept (committed on nerd/recurse) only if its finding is gone and no gate got
worse. Anything else is reverted. A finding that fails the same way twice is
left until something in its node changes.

It runs until stopped. 'nerd campaign recurse stop' ends it after the attempt
in flight is judged; Ctrl+C ends it at once, reverting that attempt. Run it
again to resume where it stopped. 'nerd campaign recurse status' shows the
ledger.

Examples:
  nerd campaign recurse --plan
  nerd campaign recurse
  nerd campaign recurse --waves 1
  nerd campaign recurse --subsystem internal/store
  nerd campaign recurse status
  nerd campaign recurse stop`,
	RunE: runCampaignRecurse,
}

func init() {
	f := campaignRecurseCmd.Flags()
	f.IntVar(&recurseWaves, "waves", 0, "Passes over the graph (0 = until stopped)")
	f.StringSliceVar(&recurseSubsystems, "subsystem", nil, "Focus nodes (dependencies pulled in); repeatable")
	f.IntVar(&recurseContextSize, "context-budget", 0, "Token budget for each attempt's campaign context (default 200000)")
	f.BoolVar(&recursePlan, "plan", false, "Print the derived sweep order and the workspace's gates; runs nothing")
	f.BoolVar(&recurseDryRun, "dry-run", false, "Same as --plan")
	campaignRecurseCmd.AddCommand(campaignRecurseStatusCmd, campaignRecurseStopCmd)
}

var campaignRecurseStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what the recurse loop has done in this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := campaign.ReadRecurseStatus(recurseWorkspace())
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), st)
		return nil
	},
}

var campaignRecurseStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the recurse loop after the attempt in flight is judged",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := campaign.RequestRecurseStop(recurseWorkspace()); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Stop requested: the loop ends once the attempt in flight is judged.")
		return nil
	},
}

func recurseWorkspace() string {
	if workspace != "" {
		return workspace
	}
	cwd, _ := os.Getwd()
	return cwd
}

// resolveRecurseConfig validates flags into a campaign.RecurseConfig. Pure
// (no I/O) so tests pin every flag-to-field mapping.
func resolveRecurseConfig(waves int, subsystems []string, contextSize int) (campaign.RecurseConfig, error) {
	return campaign.RecurseConfig{
		MaxWaves:      waves,
		Subsystems:    subsystems,
		ContextBudget: contextSize,
	}.Normalize()
}

// recurseAttemptConfig returns the orchestrator config for the n-th attempt
// (zero-based). The first uses base as built; every later one gets a fresh
// Northstar observer, because the previous attempt's orchestrator closed its
// own.
func recurseAttemptConfig(base campaign.OrchestratorConfig, n int, newObserver func() *northstar.CampaignObserver) campaign.OrchestratorConfig {
	if n == 0 {
		return base
	}
	attemptCfg := base
	attemptCfg.NorthstarObserver = newObserver()
	return attemptCfg
}

func runCampaignRecurse(cmd *cobra.Command, args []string) error {
	cfg, err := resolveRecurseConfig(recurseWaves, recurseSubsystems, recurseContextSize)
	if err != nil {
		return err
	}

	cwd := recurseWorkspace()
	if recursePlan || recurseDryRun {
		return writeRecursePlan(cmd.OutOrStdout(), cwd, cfg)
	}

	// The run honors --timeout like any other command. Ctrl+C cancels the
	// shared context; the loop reverts an attempt in flight.
	ctx, cancel := operationContext(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nRecurse stopping; reverting any attempt in flight...")
		cancel()
	}()

	key := resolveAPIKey(apiKey, workspace)
	cortex, err := coresys.GetOrBootCortex(ctx, cwd, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()
	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard()
	}
	if cortex.LLMClient == nil {
		return fmt.Errorf("campaign recurse: cortex boot did not provide an LLM client")
	}
	kernel := cortex.Kernel
	if kernel == nil && cortex.RealKernel != nil {
		kernel = cortex.RealKernel
	}
	if kernel == nil {
		return fmt.Errorf("campaign recurse: cortex boot did not provide a kernel")
	}

	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)
	orchCfg, promptProvider, err := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)
	if err != nil {
		return err
	}
	startCampaignEventPrinter(eventChan)

	attempts := 0
	execute := func(ctx context.Context, a campaign.RecurseAttempt) error {
		camp := campaign.RecurseAttemptCampaign(cwd, a)
		camp.ContextBudget = cfg.ContextBudget
		defer func() {
			if err := campaign.ReleaseRecurseAttempt(cwd, kernel, camp); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "recurse: %v\n", err)
			}
		}()
		attemptCfg := recurseAttemptConfig(orchCfg, attempts, func() *northstar.CampaignObserver { return campaignNorthstarObserver(cortex, cwd) })
		attempts++
		return executeCampaignPlan(ctx, cmd, attemptCfg, promptProvider, camp)
	}

	fmt.Printf("\nRecurse: improving %s node by node (%s).\n", cwd, recurseBoundText(cfg))
	result, err := campaign.RunRecurseCycles(ctx, campaign.RecurseCycleConfig{
		Workspace:  cwd,
		Kernel:     kernel,
		Execute:    execute,
		Passes:     cfg.MaxWaves,
		Subsystems: cfg.Subsystems,
		Progress:   cmd.OutOrStdout(),
	})
	printRecurseResult(cmd.OutOrStdout(), result)
	if errors.Is(err, campaign.ErrRecurseStopped) {
		return nil
	}
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	return nil
}

func recurseBoundText(cfg campaign.RecurseConfig) string {
	if cfg.MaxWaves == 0 {
		return "until stopped: `nerd campaign recurse stop`, or Ctrl+C"
	}
	if cfg.MaxWaves == 1 {
		return "1 pass"
	}
	return fmt.Sprintf("%d passes", cfg.MaxWaves)
}

func printRecurseResult(w io.Writer, r *campaign.RecurseCycleResult) {
	if r != nil {
		fmt.Fprintf(w, "\n%s\n", r.Summary())
	}
}

// writeRecursePlan prints what recurse would do in this workspace without a
// model: the sweep order derived from the workspace's own imports (bottom to
// top), the gates that will judge every change, and the gates that cannot run
// here. It is the owner's look before a real run.
func writeRecursePlan(w io.Writer, cwd string, cfg campaign.RecurseConfig) error {
	root, err := filepath.Abs(cwd)
	if err != nil {
		return err
	}
	nodes, err := campaign.RecurseSweepOrder(context.Background(), root, cfg.Subsystems)
	if err != nil {
		return err
	}
	set, err := gates.Detect(root)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Recurse plan for %s (no model; nothing runs)\n", root)
	fmt.Fprintf(w, "\nSweep order, bottom to top (%d nodes):\n", len(nodes))
	for i, n := range nodes {
		deps := "-"
		if len(n.DependsOn) > 0 {
			deps = strings.Join(n.DependsOn, ", ")
		}
		fmt.Fprintf(w, "  %3d  %-40s after: %s\n", i+1, n.Title, deps)
	}

	fmt.Fprintf(w, "\nGates (%d runnable):\n", len(set.Gates))
	if len(set.Gates) == 0 {
		fmt.Fprintln(w, "  none: every node is /unverified until nerd.md declares commands or gates")
	}
	for _, g := range set.Gates {
		fmt.Fprintf(w, "  %s   [%s]\n", g, g.Source)
	}
	if len(set.Unavailable) > 0 {
		fmt.Fprintf(w, "\nGates that cannot run here (%d; what they cover is /unverified, never a pass):\n", len(set.Unavailable))
		for _, u := range set.Unavailable {
			fmt.Fprintf(w, "  %s   (%s)\n", u.Gate, u.Reason)
		}
	}

	passes := "until stopped"
	if cfg.MaxWaves > 0 {
		passes = fmt.Sprintf("%d", cfg.MaxWaves)
	}
	fmt.Fprintf(w, "\nEach pass visits these nodes in order (passes: %s). A visit runs the node's gates,\n", passes)
	fmt.Fprintln(w, "the kernel picks a finding (policy/recurse.mg), the model gets one campaign to fix it,")
	fmt.Fprintln(w, "and the fix is kept only if its finding is gone and no gate got worse.")
	return nil
}
