package main

import (
	"context"
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

// `nerd campaign recurse` — a self-improvement sweep over the subsystem DAG,
// wave after wave. Each wave plans deterministically (no LLM decomposition):
// base primitives first, up through the layers, then across for wiring,
// architectural review, and benchmarks. The next wave retargets from the
// previous wave's findings and rotates attack angles, so consecutive waves
// come at the same code from different directions.
//
// Bounds: --waves N runs N waves. --waves 0 with --yolo (or yolo:true in
// config) runs until stopped or stalled. Unbounded without yolo is refused:
// an infinite loop is a decision the operator must take deliberately. Even
// unbounded runs stop on the stall fuse (consecutive waves with nothing
// completed) — looping without progress is a stall, not work.

var (
	recurseWaves       int
	recurseAngles      []string
	recurseSubsystems  []string
	recurseStallWaves  int
	recurseContextSize int
	recurseDryRun      bool
	recursePlan        bool
)

var campaignRecurseCmd = &cobra.Command{
	Use:   "recurse",
	Short: "Run a self-improvement sweep over the subsystem DAG, wave after wave",
	Long: `Run a deterministic self-improvement sweep, wave after wave.

Each wave sweeps every subsystem in dependency order (Mangle first, CLI last),
then wiring, architectural review, and benchmarks across the tree. The next
wave leads with whatever failed and rotates attack angles, so the sweep keeps
coming at the codebase from different directions.

Waves are ordinary campaigns: each is resumable and inspectable on its own,
linked by a shared recurse ID. Ctrl+C stops between waves.

Examples:
  nerd campaign recurse --waves 1
  nerd campaign recurse --waves 3 --subsystem session --subsystem cli
  nerd campaign recurse --waves 0 --yolo
  nerd campaign recurse --plan
  nerd campaign recurse --angles harden,secure --plan`,
	RunE: runCampaignRecurse,
}

func init() {
	f := campaignRecurseCmd.Flags()
	f.IntVar(&recurseWaves, "waves", 1, "Waves to run (0 = unbounded, requires --yolo)")
	f.StringSliceVar(&recurseAngles, "angles", nil, "Fix the wave angles (max 2: harden,wire,review,test,bench,secure); default rotates")
	f.StringSliceVar(&recurseSubsystems, "subsystem", nil, "Focus subsystems (dependencies pulled in); repeatable")
	f.IntVar(&recurseStallWaves, "stall-waves", 0, "Stop after N consecutive waves with nothing completed (default 2)")
	f.IntVar(&recurseContextSize, "context-budget", 0, "Token budget for wave campaign context (default 200000)")
	f.BoolVar(&recursePlan, "plan", false, "Print the derived sweep order, the workspace's gates and the wave-zero plan; runs nothing")
	f.BoolVar(&recurseDryRun, "dry-run", false, "Same as --plan")
}

// resolveRecurseConfig validates flags into a campaign.RecurseConfig. Pure
// (no I/O) so tests pin every flag-to-field mapping.
func resolveRecurseConfig(waves int, angles, subsystems []string, stallWaves, contextSize int) (campaign.RecurseConfig, error) {
	cfg := campaign.RecurseConfig{
		MaxWaves:       waves,
		Subsystems:     subsystems,
		StallWaveLimit: stallWaves,
		ContextBudget:  contextSize,
	}
	for _, a := range angles {
		angle, err := campaign.ParseRecurseAngle(a)
		if err != nil {
			return cfg, err
		}
		cfg.Angles = append(cfg.Angles, angle)
	}
	return cfg.Normalize()
}

// checkRecurseYolo refuses an unbounded run without yolo mode. An infinite
// loop is a decision the operator must take deliberately, from the flag or
// from config — never the default.
func checkRecurseYolo(cfg campaign.RecurseConfig, yolo bool) error {
	if cfg.MaxWaves == 0 && !yolo {
		return fmt.Errorf("recurse: unbounded run (--waves 0) requires yolo mode (--yolo or yolo:true in .nerd/config.json)")
	}
	return nil
}

// recurseWaveConfig returns the orchestrator config for one recurse wave.
// Wave 0 uses base as built; every later wave gets a fresh Northstar
// observer, because the previous wave's orchestrator closed its own.
func recurseWaveConfig(base campaign.OrchestratorConfig, wave int, newObserver func() *northstar.CampaignObserver) campaign.OrchestratorConfig {
	if wave == 0 {
		return base
	}
	waveCfg := base
	waveCfg.NorthstarObserver = newObserver()
	return waveCfg
}

func runCampaignRecurse(cmd *cobra.Command, args []string) error {
	cfg, err := resolveRecurseConfig(recurseWaves, recurseAngles, recurseSubsystems, recurseStallWaves, recurseContextSize)
	if err != nil {
		return err
	}

	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// A plan runs nothing, so it needs no bound and no yolo.
	if recursePlan || recurseDryRun {
		return writeRecursePlan(cmd.OutOrStdout(), cwd, cfg)
	}

	yolo := yoloMode
	if !yolo {
		if appCfg := loadCampaignConfig(filepath.Join(cwd, ".nerd")); appCfg != nil {
			yolo = appCfg.YoloMode()
		}
	}
	if err := checkRecurseYolo(cfg, yolo); err != nil {
		return err
	}

	// Unbounded runs still honor --timeout like any other command; Ctrl+C
	// stops between waves via the shared context.
	ctx, cancel := operationContext(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nRecurse stopping after this wave...")
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
	if cortex.Kernel == nil && cortex.RealKernel == nil {
		return fmt.Errorf("campaign recurse: cortex boot did not provide a kernel")
	}

	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)
	orchCfg, promptProvider, err := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)
	if err != nil {
		return err
	}
	startCampaignEventPrinter(eventChan)

	runner := &campaign.RecurseRunner{
		MaxWaves:       cfg.MaxWaves,
		AllowUnbounded: yolo,
		StallWaveLimit: cfg.StallWaveLimit,
		NewWave: func(ctx context.Context, wave int, prev *campaign.Campaign) (*campaign.Campaign, error) {
			var planned *campaign.Campaign
			var err error
			if wave == 0 {
				planned, err = campaign.NewRecurseCampaign(cwd, cfg)
			} else {
				planned, err = campaign.PlanNextWave(cwd, cfg, prev)
			}
			if err != nil {
				return nil, err
			}
			fmt.Printf("\n%s Recurse wave %d: %s\n", waveBanner(wave), wave, planned.Title)
			runErr := executeCampaignPlan(ctx, cmd, recurseWaveConfig(orchCfg, wave, func() *northstar.CampaignObserver { return campaignNorthstarObserver(cortex, cwd) }), promptProvider, planned)
			return planned, runErr
		},
	}

	fmt.Printf("\nRecurse: sweeping the subsystem DAG (%s).\n", recurseBoundText(cfg, yolo))
	result, err := runner.Run(ctx)
	printRecurseResult(result)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	return nil
}

func waveBanner(wave int) string {
	if wave == 0 {
		return "Starting"
	}
	return "Continuing to"
}

func recurseBoundText(cfg campaign.RecurseConfig, yolo bool) string {
	if cfg.MaxWaves == 0 {
		return "unbounded, yolo mode — Ctrl+C stops between waves"
	}
	if yolo {
		return fmt.Sprintf("%d waves, yolo mode", cfg.MaxWaves)
	}
	return fmt.Sprintf("%d waves", cfg.MaxWaves)
}

func printRecurseResult(result *campaign.RecurseResult) {
	if result == nil {
		return
	}
	fmt.Printf("\nRecurse finished: %d waves, %d tasks completed, %d failed.\n",
		result.WavesCompleted, result.CompletedTasks, result.FailedTasks)
	if result.Stalled {
		fmt.Printf("Stopped by the stall fuse: consecutive waves completed nothing (last wave %s).\n", result.LastCampaignID)
	}
	for _, e := range result.WaveErrors {
		fmt.Printf("  wave error kept in result: %s\n", e)
	}
}

// writeRecursePlan prints what recurse would do in this workspace without a
// model: the sweep order derived from the workspace's own imports (bottom to
// top), the gates that will judge every change, the gates that cannot run
// here, and the wave-zero phases. It is the owner's look before a real run.
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
	c, err := campaign.NewRecurseCampaign(root, cfg)
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

	fmt.Fprintf(w, "\nWave 0: %s\n", c.Title)
	fmt.Fprintf(w, "  phases: %d, tasks: %d, max waves: %d, stall waves: %d\n",
		c.TotalPhases, c.TotalTasks, cfg.MaxWaves, cfg.StallWaveLimit)
	for i, p := range c.Phases {
		fmt.Fprintf(w, "  phase %d: %s (%d tasks, gate %s)\n", i, p.Name, len(p.Tasks), p.Checkpoints[0].Type)
	}
	return nil
}
