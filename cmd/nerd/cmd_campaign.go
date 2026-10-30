package main

import (
	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/consultation"
	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/store"
	coresys "codenerd/internal/system"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// loadCampaignConfig reads the user config for a workspace, falling back to
// defaults when it is absent or unreadable.
func loadCampaignConfig(nerdDir string) *config.UserConfig {
	appCfg, _ := config.LoadUserConfig(filepath.Join(nerdDir, "config.json"))
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	return appCfg
}

// newConfiguredLLMClient builds the main CLI LLM client from .nerd/config.json,
// falling back to ambient environment keys ONLY when the config expresses no
// choice at all. Every CLI path that makes LLM calls must go through this:
// reading an env var directly runs the user's work on a provider they did not
// configure, at that provider's expense, and the failure is silent because the
// wrong client works fine.
func newConfiguredLLMClient(appCfg *config.UserConfig, label string) (perception.LLMClient, error) {
	if pc, err := perception.ProviderConfigFromUserConfig(appCfg); err == nil {
		client, cerr := perception.NewClientFromConfig(pc)
		if cerr != nil {
			return nil, fmt.Errorf("failed to initialize LLM client from .nerd/config.json: %w", cerr)
		}
		return core.NewScheduledLLMCall(label, client), nil
	} else if appCfg.HasExplicitLLMSelection() {
		// Config expresses a choice but cannot be satisfied. Falling back to an
		// ambient key here would run on a provider the user did not ask for, so
		// fail loudly instead.
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	envClient, eerr := perception.NewClientFromEnv()
	if eerr != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", eerr)
	}
	return core.NewScheduledLLMCall(label, envClient), nil
}

// campaignCmd is the parent command for campaign operations
var campaignCmd = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "campaign",
	Short: "Campaign orchestration for long-running goals",
	Long: `Campaigns are long-running, multi-phase goals that span sessions.

Use campaigns for:
  - Greenfield builds from spec documents
  - Large feature implementations
  - Codebase-wide stability audits
  - Migration projects

Examples:
  nerd campaign start "Build REST API" --docs ./specs/
  nerd campaign status
  nerd campaign pause
  nerd campaign resume`,
	RunE: parentGroupRunE,
}

// campaignStartCmd starts a new campaign
var campaignStartCmd = &cobra.Command{
	Use:   "start [goal]",
	Short: "Start a new campaign",
	Long: `Starts a new campaign by decomposing the goal into phases and tasks.

The goal can be:
  - A natural language description of what you want to build
  - A reference to spec documents with --docs flag

Examples:
  nerd campaign start "Build a REST API with user auth"
  nerd campaign start "Implement the feature in spec.md" --docs ./specs/
  nerd campaign start --docs ./Docs/research/*.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCampaignStart,
}

// campaignStatusCmd shows campaign status
var campaignStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current campaign status",
	RunE:  runCampaignStatus,
}

// campaignPauseCmd pauses the current campaign
var campaignPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current campaign",
	RunE:  runCampaignPause,
}

// campaignResumeCmd resumes a paused campaign
var campaignResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused campaign",
	RunE:  runCampaignResume,
}

// campaignListCmd lists all campaigns
var campaignListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all campaigns",
	RunE:  runCampaignList,
}

// NOTE: Due to file size constraints, this file extracts campaign command handlers from main.go
// The full implementations follow. The campaign_jit_provider.go file contains the JIT adapter.

// runCampaignStart starts a new campaign
// buildCampaignOrchestratorConfig assembles the campaign OrchestratorConfig from
// a booted Cortex so the start path (and later the resume path) share one
// construction site.
//
// It is pure construction: no boot, no LLM calls. All factory-owned objects
// (kernel, store, shards, executors, scanner, persistence, transducer, LLM
// tiers) come from the Cortex; only campaign-specific pieces (intelligence
// gatherer, advisory board, northstar observer, edge-case detector, JIT
// prompt provider) are built here. Optional Cortex pieces (Orchestrator, MCP
// bridge, scanner, JIT compiler, planner/worker tiers) may be nil — the
// accessors ToolGenerator/MCPStore are nil-safe and every other nil is
// degraded gracefully, never panicked.
//
// Planning runs on cortex.PlanningLLM (the planner slot when config.json
// configures one, else main); tasks run on the worker tier inside
// cortex.TaskExecutor.
//
// The campaign policy is the workspace config's campaign section, read here
// and nowhere else on the CLI: a config that does not load is an error, never
// a campaign quietly run on defaults.
func buildCampaignOrchestratorConfig(cortex *coresys.Cortex, cwd string, progressChan chan campaign.Progress, eventChan chan campaign.OrchestratorEvent) (campaign.OrchestratorConfig, campaign.PromptProvider, error) {
	var cfg campaign.OrchestratorConfig
	cfg.Workspace = cwd
	cfg.ProgressChan = progressChan
	cfg.EventChan = eventChan
	appCfg, err := config.LoadUserConfig(filepath.Join(cwd, ".nerd", "config.json"))
	if err != nil {
		return cfg, nil, fmt.Errorf("campaign policy: %w", err)
	}
	cfg.Campaign = appCfg.GetCampaignConfig()
	if cortex == nil {
		return cfg, nil, nil
	}
	cfg.Kernel = cortex.Kernel
	cfg.LLMClient = cortex.LLMClient
	cfg.PlanningLLMClient = cortex.PlanningLLM()
	cfg.ShardManager = cortex.ShardManager
	cfg.TaskExecutor = cortex.TaskExecutor
	cfg.Executor = cortex.Executor
	cfg.VirtualStore = cortex.VirtualStore

	worldScanner := cortex.Scanner
	if worldScanner == nil {
		worldScanner = world.NewScanner()
	}

	// Every component reads the Cortex: a fact lives in the shard that owns
	// it, and the catch-all shard's kernel (cortex.RealKernel) holds only what
	// no other shard owns. Until 2026-09-23 these took the catch-all.
	kern := cortex.Kernel

	// Keep the store import live and make the persistence wiring explicit:
	// the gatherer reads historical patterns (learning) and the knowledge
	// graph (local) that the factory already opened.
	var learningStore *store.LearningStore
	var localStore *store.LocalStore
	learningStore = cortex.LearningStore
	localStore = cortex.LocalDB

	var consultationProvider campaign.ConsultationProvider
	if cortex.TaskExecutor != nil {
		consultationProvider = consultation.New(session.TaskConsultationSpawner{Executor: cortex.TaskExecutor})
	}

	holographic := world.NewHolographicProvider(kern, cwd)

	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		cwd,
		kern,
		worldScanner,
		holographic,
		learningStore,
		localStore,
		cortex.ToolGenerator(),
		cortex.MCPStore(),
		consultationProvider,
	)

	advisoryBoard := campaign.NewShardAdvisoryBoard(consultationProvider)

	northstarObserver := campaignNorthstarObserver(cortex, cwd)

	edgeCaseDetector := campaign.NewEdgeCaseDetector(kern, worldScanner)

	cfg.IntelligenceGatherer = intelligenceGatherer
	cfg.AdvisoryBoard = advisoryBoard
	cfg.NorthstarObserver = northstarObserver
	cfg.EdgeCaseDetector = edgeCaseDetector

	assembler := cortex.PromptAssembler
	if assembler == nil && cortex.JITCompiler != nil && kern != nil {
		if pa, err := articulation.NewPromptAssemblerWithJIT(kern, cortex.JITCompiler); err == nil {
			assembler = pa
		}
	}
	return cfg, campaign.NewJITPromptProvider(assembler), nil
}

// newCampaignOrchestrator builds the orchestrator one campaign plans and runs
// on, with the JIT prompt provider when there is one. The caller owns it and
// closes it, which closes the northstar observer the config carries.
func newCampaignOrchestrator(orchCfg campaign.OrchestratorConfig, promptProvider campaign.PromptProvider) (*campaign.Orchestrator, error) {
	orchestrator, err := campaign.NewOrchestrator(orchCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize campaign orchestrator: %w", err)
	}
	if promptProvider != nil {
		orchestrator.SetPromptProvider(promptProvider)
	}
	return orchestrator, nil
}

// campaignNorthstarObserver builds the vision-guardian observer for one
// campaign orchestrator. Each orchestrator owns and closes its own
// observer, so a command that runs several orchestrators (recurse waves)
// builds one per orchestrator.
func campaignNorthstarObserver(cortex *coresys.Cortex, cwd string) *northstar.CampaignObserver {
	if cortex == nil {
		return nil
	}
	var kern northstar.KernelClient
	if cortex.Kernel != nil {
		kern = cortex.Kernel
	}
	return northstar.BuildCampaignObserver(cwd, cortex.LLMClient, kern)
}

// runCampaignStart starts a new campaign
func runCampaignStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := operationContext(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nCampaign start cancelled")
		cancel()
	}()

	goal := joinArgs(args)

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Get flags
	docs, _ := cmd.Flags().GetStringArray("docs")
	campaignType, _ := cmd.Flags().GetString("type")

	// Boot through the Cortex factory so campaigns inherit every factory
	// improvement (session identity, Ouroboros registry, hydration, Dreamer
	// gate adapter, worker/planner routing) instead of mirroring it by hand.
	key := resolveAPIKey(apiKey, workspace)
	cortex, err := coresys.GetOrBootCortex(ctx, cwd, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()
	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard() // CLI commands are user-initiated, disable boot guard
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║          CAMPAIGN ORCHESTRATOR - INITIALIZING             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Printf("\nGoal: %s\n", goal)
	if len(docs) > 0 {
		fmt.Printf("Source Documents: %v\n", docs)
	}
	fmt.Printf("Campaign Type: %s\n\n", campaignType)

	// Campaign-specific wiring (advisory board, northstar observer,
	// edge-case detector, prompt provider, progress/event channels, tool
	// pregenerator) built from the Cortex-provided objects.
	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	fmt.Println("🧠 Initializing intelligence gathering systems...")
	orchCfg, campaignPromptProvider, err := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)
	if err != nil {
		return err
	}
	fmt.Println("   ✓ World scanner initialized")
	fmt.Println("   ✓ Intelligence gatherer initialized")
	fmt.Println("   ✓ Advisory board initialized")
	// The northstar observer prints its own ✓/⚠ inside BuildCampaignObserver.
	fmt.Println("   ✓ Edge case detector initialized")
	fmt.Println("   ✓ Intelligence systems initialized")

	// One orchestrator plans and runs the campaign: decomposition and every
	// replan run on orchCfg.PlanningLLMClient, tasks on the worker tier inside
	// cortex.TaskExecutor. Built before planning so it owns the northstar
	// observer on every path, a failed decomposition included.
	orchestrator, err := newCampaignOrchestrator(orchCfg, campaignPromptProvider)
	if err != nil {
		return err
	}
	defer func() { _ = orchestrator.Close() }()

	// Build request with context budget from config (request param, not boot).
	nerdDir := filepath.Join(cwd, ".nerd")
	appCfg := loadCampaignConfig(nerdDir)
	contextBudget := 200000 // Default 200k tokens
	if appCfg != nil && appCfg.ContextWindow != nil && appCfg.ContextWindow.MaxTokens > 0 {
		contextBudget = appCfg.ContextWindow.MaxTokens
	}
	req := campaign.DecomposeRequest{
		Goal:          goal,
		SourcePaths:   docs,
		CampaignType:  campaign.CampaignType("/" + campaignType),
		ContextBudget: contextBudget,
	}

	// A deterministically-built campaign (currently `nerd campaign assault`)
	// skips decomposition but needs the identical boot below: JIT executor,
	// intelligence wiring, risk preflight, event streaming and resume support.
	// Duplicating ~150 lines of that in a second command is how the two paths
	// drift; injecting the plan here keeps one implementation.
	var result *campaign.DecomposeResult
	if prebuilt := takeCampaignStartOverride(); prebuilt != nil {
		fmt.Println("📋 Using deterministic campaign plan (decomposition skipped)...")
		result = &campaign.DecomposeResult{Campaign: prebuilt, ValidationOK: true}
	} else {
		fmt.Println("📋 Decomposing goal into phases and tasks...")

		var derr error
		result, derr = orchestrator.Decompose(ctx, req)
		if derr != nil {
			return fmt.Errorf("failed to decompose campaign: %w", derr)
		}
	}

	if !result.ValidationOK {
		fmt.Println("\n⚠️  Plan validation found issues:")
		for _, issue := range result.Issues {
			fmt.Printf("  - [%s] %s\n", issue.IssueType, issue.Description)
		}
		return fmt.Errorf("campaign plan remains invalid after refinement; execution refused")
	}

	printPlanIntelligence(result.Intelligence)

	// A degraded plan is not a plan. Say so before printing anything that looks
	// like one — the generic three-phase scaffold with a plausible title and a
	// confidence number is otherwise indistinguishable from real decomposition,
	// and the campaign will go on to report phases completed while producing
	// work nobody asked for.
	if result.Campaign.PlanDegraded {
		fmt.Println("\n╔═══════════════════════════════════════════════════════════╗")
		fmt.Println("║  ⚠  DECOMPOSITION FAILED — THIS IS A PLACEHOLDER PLAN     ║")
		fmt.Println("╚═══════════════════════════════════════════════════════════╝")
		fmt.Println("The planner returned no phases, so codeNERD substituted a generic")
		fmt.Println("three-task scaffold. It CANNOT satisfy a goal that names specific")
		fmt.Println("deliverables. Check the planner response in .nerd/logs/*_llm_io.log")
		fmt.Println("for an output-contract mismatch before letting this run.")
		fmt.Println()
	}

	// The acceptance command is the user's, so it is attached here and not
	// planned: a witness the planner chose would be one more model judgement.
	if accept, _ := cmd.Flags().GetString("accept"); strings.TrimSpace(accept) != "" {
		result.Campaign.Acceptance = &campaign.Acceptance{Command: strings.Fields(accept)}
		fmt.Printf("Acceptance: %s (exit 0 to complete)\n\n", strings.Join(result.Campaign.Acceptance.Command, " "))
	}

	startCampaignEventPrinter(eventChan)

	return executeCampaignPlan(ctx, cmd, orchestrator, result.Campaign)
}

// executeCampaignPlan runs one fully-planned campaign on orchestrator:
// summary, Run, and outcome translation. runCampaignStart calls it once;
// runCampaignRecurse calls it per wave on a fresh orchestrator, so every wave
// gets the identical outcome handling. The caller owns and closes the
// orchestrator. Event printing is the caller's job too: one printer serves a
// whole command over a shared channel, since waves run sequentially.
func executeCampaignPlan(ctx context.Context, cmd *cobra.Command, orchestrator *campaign.Orchestrator, camp *campaign.Campaign) error {
	// Display plan summary
	fmt.Printf("\n📊 Campaign Plan: %s\n", camp.Title)
	fmt.Printf("   Confidence: %.0f%%\n", camp.Confidence*100)
	fmt.Printf("   Phases: %d\n", camp.TotalPhases)
	fmt.Printf("   Tasks: %d\n\n", camp.TotalTasks)

	for i, phase := range camp.Phases {
		fmt.Printf("Phase %d: %s (%d tasks)\n", i+1, phase.Name, len(phase.Tasks))
		for j, task := range phase.Tasks {
			status := "⏳"
			fmt.Printf("  %s %d.%d %s\n", status, i+1, j+1, task.Description)
		}
	}

	if err := orchestrator.SetCampaign(camp); err != nil {
		return fmt.Errorf("failed to set campaign: %w", err)
	}
	defer printCampaignMetrics(attachCampaignMetrics(orchestrator))

	fmt.Println("\n🚀 Starting campaign execution...")
	fmt.Println("   Press Ctrl+C to pause")

	// Run campaign
	if err := orchestrator.Run(ctx); err != nil {
		if ctx.Err() != nil {
			// Name the deadline. "Campaign paused" alone reads like a clean
			// stopping point; it is actually the whole command's operation
			// timeout expiring mid-phase, and decomposition can eat several
			// minutes of it before a single task runs.
			fmt.Printf("\n⏱  Operation timeout (%v) reached — campaign paused mid-run.\n", timeout)
			fmt.Println("   Run 'nerd campaign resume' to continue, or raise it with --timeout.")
			cmd.SilenceUsage = true
			return campaignOutcome(err, ctx.Err(), timeout)
		}
		return campaignOutcome(err, ctx.Err(), timeout)
	}

	fmt.Println("\n✨ Campaign completed successfully!")
	return nil
}

// attachCampaignMetrics installs an in-memory sink on the orchestrator. The
// orchestrator measures every task, checkpoint, phase and risk preflight; with
// no sink those measurements went nowhere.
func attachCampaignMetrics(orchestrator *campaign.Orchestrator) *campaign.InMemoryMetrics {
	metrics := campaign.NewInMemoryMetrics()
	orchestrator.SetMetricsSink(metrics)
	return metrics
}

// printCampaignMetrics reports what the run measured, however it ended: a
// paused or failed run's timings are what an operator tunes --timeout from.
func printCampaignMetrics(metrics *campaign.InMemoryMetrics) {
	lines := metrics.Summary()
	if len(lines) == 0 {
		return
	}
	fmt.Println("\n📈 Campaign metrics")
	for _, line := range lines {
		fmt.Println("   " + line)
	}
}

// startCampaignEventPrinter streams orchestrator events to stdout until the
// channel closes. One printer serves a whole command; recurse waves share the
// channel and the printer, since waves run sequentially.
func startCampaignEventPrinter(eventChan chan campaign.OrchestratorEvent) {
	go func() {
		for event := range eventChan {
			switch event.Type {
			case "task_started":
				fmt.Printf("\U0001F504 %s\n", event.Message)
			case "task_completed":
				fmt.Printf("\u2705 %s\n", event.Message)
			case "task_failed":
				fmt.Printf("\u274C %s\n", event.Message)
			case "phase_started":
				fmt.Printf("\n\U0001F4E6 Phase: %s\n", event.Message)
			case "phase_completed":
				fmt.Printf("\U0001F389 Phase completed: %s\n", event.Message)
			case "campaign_completed":
				fmt.Printf("\n\U0001F3C6 %s\n", event.Message)
			case "replan_triggered":
				fmt.Printf("\U0001F504 Replanning: %s\n", event.Message)
			case campaign.EventRiskGateAdvisory:
				// A soft risk finding: the campaign continues, and the
				// operator still sees what the preflight found.
				fmt.Printf("⚠️  %s\n", event.Message)
			case "snapshot_write_failed":
				// The campaign is running from memory only. Silence here is how
				// an operator ends up believing finished phases were saved.
				fmt.Printf("\u26A0\uFE0F  Campaign snapshot NOT written \u2014 progress is not on disk: %s\n", event.Message)
			}
		}
	}()
}

// runCampaignStatus shows current campaign status
func runCampaignStatus(cmd *cobra.Command, args []string) error {
	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		fmt.Println("No campaigns found. Run 'nerd campaign start' to create one.")
		return nil
	}

	// Find most recent campaign
	var latestCampaign *campaign.Campaign
	var latestTime time.Time

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(campaignsDir, entry.Name()))
			if err != nil {
				continue
			}

			var c campaign.Campaign
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			if c.UpdatedAt.After(latestTime) {
				latestTime = c.UpdatedAt
				latestCampaign = &c
			}
		}
	}

	if latestCampaign == nil {
		fmt.Println("No campaigns found.")
		return nil
	}

	// Display status
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                  CAMPAIGN STATUS                          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📋 %s\n", latestCampaign.Title)
	fmt.Printf("   ID: %s\n", latestCampaign.ID)
	fmt.Printf("   Status: %s\n", latestCampaign.Status)
	fmt.Printf("   Created: %s\n", latestCampaign.CreatedAt.Format(time.RFC822))

	// Progress bar
	progress := float64(latestCampaign.CompletedTasks) / float64(latestCampaign.TotalTasks)
	barWidth := 40
	filled := int(progress * float64(barWidth))
	bar := fmt.Sprintf("[%s%s] %.0f%%",
		repeatChar('█', filled),
		repeatChar('░', barWidth-filled),
		progress*100)
	fmt.Printf("\n   Progress: %s\n", bar)
	fmt.Printf("   Tasks: %d/%d completed\n", latestCampaign.CompletedTasks, latestCampaign.TotalTasks)
	fmt.Printf("   Phases: %d/%d completed\n", latestCampaign.CompletedPhases, latestCampaign.TotalPhases)

	// Current phase
	for _, phase := range latestCampaign.Phases {
		if phase.Status == campaign.PhaseInProgress {
			fmt.Printf("\n   Current Phase: %s\n", phase.Name)
			pendingCount := 0
			for _, task := range phase.Tasks {
				if task.Status == campaign.TaskPending || task.Status == campaign.TaskInProgress {
					pendingCount++
				}
			}
			fmt.Printf("   Remaining tasks in phase: %d\n", pendingCount)
			break
		}
	}

	// Learnings
	if len(latestCampaign.Learnings) > 0 {
		fmt.Printf("\n   Learnings applied: %d\n", len(latestCampaign.Learnings))
	}

	// Revisions
	if latestCampaign.RevisionNumber > 0 {
		fmt.Printf("   Plan revisions: %d\n", latestCampaign.RevisionNumber)
	}

	return nil
}

// runCampaignPause pauses the current campaign by persisting StatusPaused.
// Without a disk update, resume cannot find the campaign and status stays
// stuck on the previous lifecycle state (e.g. /validating).
func runCampaignPause(cmd *cobra.Command, args []string) error {
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	c, _, err := findLatestPausableCampaign(cwd)
	if err != nil {
		return err
	}
	if c == nil {
		fmt.Println("No campaigns found to pause.")
		return nil
	}
	if c.Status == campaign.StatusCompleted || c.Status == campaign.StatusFailed {
		fmt.Printf("Campaign %s is %s and cannot be paused.\n", c.Title, c.Status)
		return nil
	}
	if c.Status == campaign.StatusPaused {
		fmt.Printf("Campaign already paused: %s\n", c.Title)
		fmt.Println("Run 'nerd campaign resume' to continue.")
		return nil
	}

	if err := campaign.RequestPause(cwd, c.ID); err != nil {
		return err
	}
	fmt.Printf("Pause requested: %s\n", c.Title)
	fmt.Println("The owner will cancel active work, join it and persist its checkpoint.")

	return nil
}

// runCampaignResume resumes a paused campaign
func runCampaignResume(cmd *cobra.Command, args []string) error {
	ctx, cancel := operationContext(context.Background())
	defer cancel()

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Prefer newest paused; fall back to active, then blocked/failed.
	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	candidates, err := loadResumeCandidates(campaignsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No campaigns found.")
			return nil
		}
		return err
	}
	sel := selectResumeCampaign(candidates, campaignRetryFailed, campaignResumeID)
	if sel == nil {
		fmt.Println("No paused, active, or blocked campaigns found.")
		return nil
	}
	// Discovery only: the parsed candidate names which campaign to resume. It
	// is not the state that resumes — ResumeCampaign admits a durable
	// generation from disk, and only the orchestrator that owns the campaign
	// publishes its status. Nothing here writes the snapshot: an early status
	// write that a failed boot then strands is a snapshot the journal never
	// recorded.
	campaignID := sel.ID
	fmt.Printf("Resuming campaign: %s (%s, status %s", sel.Title, campaignID, sel.Status)
	if sel.BlockReason != "" {
		fmt.Printf(", block reason %s", sel.BlockReason)
	}
	fmt.Println(")")

	// Boot through the Cortex factory, exactly like runCampaignStart, so resume
	// inherits every factory improvement (session identity, Ouroboros registry,
	// hydration, Dreamer gate adapter, worker/planner routing) and the JIT
	// prompt provider instead of hand-assembling a parallel stack with nil
	// stores.
	key := resolveAPIKey(apiKey, workspace)
	cortex, err := coresys.GetOrBootCortex(ctx, cwd, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()
	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard() // CLI commands are user-initiated, disable boot guard
	}

	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	orchCfg, campaignPromptProvider, err := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)
	if err != nil {
		return err
	}

	orchestrator, err := newCampaignOrchestrator(orchCfg, campaignPromptProvider)
	if err != nil {
		return err
	}
	defer func() { _ = orchestrator.Close() }()

	report, err := orchestrator.ResumeCampaign(campaignID, campaign.AdmissionPolicy{AdoptSnapshot: campaignAdoptSnapshot})
	if report != nil && (err != nil || report.Provenance != campaign.ProvenanceCommitted) {
		fmt.Print(campaign.RenderRecoveryReport(report))
	}
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("cannot resume campaign %s: %w", campaignID, err)
	}
	if err := campaign.ClearPauseRequest(cwd, campaignID); err != nil {
		return err
	}
	if err := orchestrator.PrepareResume(); err != nil {
		return fmt.Errorf("cannot resume campaign %s: %w", campaignID, err)
	}
	defer printCampaignMetrics(attachCampaignMetrics(orchestrator))

	// The same printer start uses: a resumed run replans and starts phases
	// too, and its own copy of the switch dropped both events.
	startCampaignEventPrinter(eventChan)

	// Run campaign
	if err := orchestrator.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nCampaign paused.")
			cmd.SilenceUsage = true
			return campaignOutcome(err, ctx.Err(), timeout)
		}
		return campaignOutcome(err, ctx.Err(), timeout)
	}

	fmt.Println("\n✨ Campaign completed successfully!")
	return nil
}

// runCampaignList lists all campaigns
func runCampaignList(cmd *cobra.Command, args []string) error {
	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		fmt.Println("No campaigns found.")
		return nil
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    CAMPAIGNS                              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(campaignsDir, entry.Name()))
			if err != nil {
				continue
			}

			var c campaign.Campaign
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			statusIcon := "⏸️"
			switch c.Status {
			case campaign.StatusActive:
				statusIcon = "▶️"
			case campaign.StatusCompleted:
				statusIcon = "✅"
			case campaign.StatusFailed:
				statusIcon = "❌"
			case campaign.StatusPaused:
				statusIcon = "⏸️"
			case campaign.StatusPlanning:
				statusIcon = "📝"
			}

			progress := float64(c.CompletedTasks) / float64(c.TotalTasks) * 100
			fmt.Printf("%s %s\n", statusIcon, c.Title)
			fmt.Printf("   ID: %s | Progress: %.0f%% | Tasks: %d/%d\n\n",
				c.ID, progress, c.CompletedTasks, c.TotalTasks)
		}
	}

	return nil
}

// repeatChar repeats a character n times
func repeatChar(c rune, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}

// campaignOutcome converts the orchestrator result into the command's
// error return. A campaign that did not finish must not exit 0: "paused
// because time ran out" and "completed" are different outcomes and a
// caller reading only the exit code cannot tell them apart otherwise.
func campaignOutcome(runErr error, ctxErr error, timeout time.Duration) error {
	if runErr == nil {
		return nil
	}
	// A risk gate refusal is not a crash and it is not a timeout: no task ran,
	// and the operator needs to see which gate stopped it, what the score was
	// and whether the finding was hard or advisory. Until now that report
	// existed only as CategoryCampaign log lines, so the terminal showed a
	// single wrapped sentence and nothing actionable.
	if report, ok := campaign.FormatRiskBlock(runErr); ok {
		fmt.Print("\n🛑 " + report)
		return runErr
	}
	if errors.Is(runErr, context.Canceled) && !errors.Is(ctxErr, context.DeadlineExceeded) {
		return fmt.Errorf("campaign paused; run nerd campaign resume to continue: %w", runErr)
	}
	if ctxErr != nil {
		return fmt.Errorf("operation timeout (%s) reached — campaign paused mid-run: run 'nerd campaign resume' to continue or raise --timeout: %w", timeout, runErr)
	}
	return fmt.Errorf("campaign failed: %w", runErr)
}

// findLatestPausableCampaign returns the most recently updated campaign that is
// not terminal, plus its on-disk JSON path.
func findLatestPausableCampaign(workspaceRoot string) (*campaign.Campaign, string, error) {
	campaignsDir := filepath.Join(workspaceRoot, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", err
	}

	var latest *campaign.Campaign
	var latestPath string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		// Skip journals
		if strings.Contains(entry.Name(), ".journal.") {
			continue
		}
		path := filepath.Join(campaignsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var c campaign.Campaign
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		if c.Status == campaign.StatusCompleted || c.Status == campaign.StatusFailed {
			continue
		}
		if latest == nil || c.UpdatedAt.After(latestTime) {
			cp := c
			latest = &cp
			latestPath = path
			latestTime = c.UpdatedAt
		}
	}
	return latest, latestPath, nil
}

// selectResumeCampaign picks which persisted campaign to resume.
// Order of preference: the newest (by UpdatedAt) StatusPaused campaign;
// else the newest StatusActive; else the newest StatusFailed campaign whose
// BlockReason is non-empty; else, only when retryFailed is true, the newest
// StatusFailed campaign regardless of BlockReason; else nil.
// It is pure (no disk, no Cortex) for testability. It only names a campaign:
// what resumes is the generation ResumeCampaign admits.
func selectResumeCampaign(candidates []*campaign.Campaign, retryFailed bool, idFilter string) *campaign.Campaign {
	var bestPaused, bestActive, bestBlocked, bestFailed *campaign.Campaign
	newer := func(c, best *campaign.Campaign) bool { return best == nil || c.UpdatedAt.After(best.UpdatedAt) }
	for _, c := range candidates {
		if c == nil {
			continue
		}
		if idFilter != "" && !strings.Contains(strings.ToLower(c.ID), strings.ToLower(idFilter)) {
			continue
		}
		switch c.Status {
		case campaign.StatusPaused:
			if newer(c, bestPaused) {
				bestPaused = c
			}
		case campaign.StatusActive:
			if newer(c, bestActive) {
				bestActive = c
			}
		case campaign.StatusFailed:
			if newer(c, bestFailed) {
				bestFailed = c
			}
			if c.BlockReason != "" && newer(c, bestBlocked) {
				bestBlocked = c
			}
		}
	}
	switch {
	case bestPaused != nil:
		return bestPaused
	case bestActive != nil:
		return bestActive
	case bestBlocked != nil:
		return bestBlocked
	case retryFailed && bestFailed != nil:
		return bestFailed
	}
	return nil
}

// loadResumeCandidates discovers the persisted campaigns in campaignsDir:
// every parseable <id>.json, plus each journaled campaign whose snapshot is
// missing but whose interrupted replacement left a <id>.json.tmp-* or
// <id>.json.prev-* behind. Without the second pass a death inside the
// snapshot-replacement fallback made the campaign vanish from resume.
// Discovery only names candidates; ResumeCampaign decides what is admitted.
func loadResumeCandidates(campaignsDir string) ([]*campaign.Campaign, error) {
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names[entry.Name()] = true
		}
	}
	parse := func(name string) *campaign.Campaign {
		data, err := os.ReadFile(filepath.Join(campaignsDir, name))
		if err != nil {
			return nil
		}
		var c campaign.Campaign
		if err := json.Unmarshal(data, &c); err != nil {
			return nil
		}
		return &c
	}
	var out []*campaign.Campaign
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(name) == ".json" && !strings.Contains(name, ".journal.") {
			if c := parse(name); c != nil {
				out = append(out, c)
			}
			continue
		}
		id, ok := strings.CutSuffix(name, ".journal.jsonl")
		if !ok || names[id+".json"] {
			continue
		}
		var leftovers []string
		for n := range names {
			if strings.HasPrefix(n, id+".json.tmp-") || strings.HasPrefix(n, id+".json.prev-") {
				leftovers = append(leftovers, n)
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(leftovers)))
		for _, n := range leftovers {
			if c := parse(n); c != nil {
				out = append(out, c)
				break
			}
		}
	}
	return out, nil
}

// printPlanIntelligence shows what the kernel concluded about the plan: the
// risk and complexity levels policy derived from the world model, and the
// per-file warnings behind them. Nothing is printed when it concluded nothing.
func printPlanIntelligence(pi campaign.PlanIntelligence) {
	if pi.Risk == "" && pi.Complexity == "" && len(pi.Advisories) == 0 && len(pi.Strategy) == 0 && len(pi.Related) == 0 {
		return
	}
	fmt.Printf("\n🔎 Plan intelligence: risk %s, blast radius %s\n", strings.TrimPrefix(pi.Risk, "/"), strings.TrimPrefix(pi.Complexity, "/"))
	for _, reason := range pi.RiskReasons {
		fmt.Printf("  - risk: %s\n", reason)
	}
	for _, a := range pi.Advisories {
		fmt.Printf("  - %s [%s]: %s\n", a.Path, strings.TrimPrefix(a.Kind, "/"), a.Message)
	}
	for _, r := range pi.Related {
		fmt.Printf("  - %s is linked (%s) to %s\n", r.Path, strings.TrimPrefix(r.Relation, "/"), r.Entity)
	}
	for _, st := range pi.Strategy {
		fmt.Printf("  - strategy [%s]: %s\n", st.Concept, st.Content)
	}
}
