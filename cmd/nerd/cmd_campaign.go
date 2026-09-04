package main

import (
	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
// (kernel, store, shards, executors, scanner, persistence, transducer) come
// from the Cortex; only campaign-specific pieces (intelligence gatherer,
// advisory board, northstar observer, edge-case detector, tool pregenerator,
// JIT prompt provider) are built here. Optional Cortex pieces (Orchestrator,
// MCP bridge, scanner, JIT compiler, planner/worker tiers) may be nil — the
// accessors ToolGenerator/OuroborosLoop/MCPStore are nil-safe and every other
// nil is degraded gracefully, never panicked.
func buildCampaignOrchestratorConfig(cortex *coresys.Cortex, cwd string, progressChan chan campaign.Progress, eventChan chan campaign.OrchestratorEvent) (campaign.OrchestratorConfig, campaign.PromptProvider) {
	var cfg campaign.OrchestratorConfig
	cfg.Workspace = cwd
	cfg.ProgressChan = progressChan
	cfg.EventChan = eventChan
	if cortex == nil {
		cfg.ToolPregenerator = campaign.NewToolPregenerator(nil, nil, nil)
		return cfg, nil
	}
	cfg.Kernel = cortex.Kernel
	cfg.LLMClient = cortex.LLMClient
	cfg.ShardManager = cortex.ShardManager
	cfg.TaskExecutor = cortex.TaskExecutor
	cfg.Executor = cortex.Executor
	cfg.VirtualStore = cortex.VirtualStore
	cfg.Transducer = cortex.Transducer

	worldScanner := cortex.Scanner
	if worldScanner == nil {
		worldScanner = world.NewScanner()
	}

	var realKern *core.RealKernel
	if cortex.RealKernel != nil {
		realKern = cortex.RealKernel
	} else if cortex.Kernel != nil {
		if rk, ok := any(cortex.Kernel).(*core.RealKernel); ok {
			realKern = rk
		}
	}

	// Keep the store import live and make the persistence wiring explicit:
	// the gatherer reads historical patterns (learning) and the knowledge
	// graph (local) that the factory already opened.
	var learningStore *store.LearningStore
	var localStore *store.LocalStore
	learningStore = cortex.LearningStore
	localStore = cortex.LocalDB

	var consultationProvider campaign.ConsultationProvider
	if cortex.TaskExecutor != nil {
		consultationMgr := shards.NewConsultationManager(&campaignTaskExecutorConsultationSpawner{executor: cortex.TaskExecutor})
		consultationProvider = newCampaignConsultationProvider(consultationMgr)
	}

	var holographic *world.HolographicProvider
	if realKern != nil {
		holographic = world.NewHolographicProvider(realKern, cwd)
	} else if cortex.Kernel != nil {
		if q, ok := any(cortex.Kernel).(world.FactQuerier); ok {
			holographic = world.NewHolographicProvider(q, cwd)
		} else {
			holographic = world.NewHolographicProvider(nil, cwd)
		}
	} else {
		holographic = world.NewHolographicProvider(nil, cwd)
	}

	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		realKern,
		worldScanner,
		holographic,
		learningStore,
		localStore,
		cortex.ToolGenerator(),
		cortex.MCPStore(),
		consultationProvider,
	)

	advisoryBoard := campaign.NewShardAdvisoryBoard(consultationProvider)

	var kernForNorthstar types.Kernel
	if realKern != nil {
		kernForNorthstar = realKern
	} else if cortex.Kernel != nil {
		if tk, ok := any(cortex.Kernel).(types.Kernel); ok {
			kernForNorthstar = tk
		}
	}
	northstarObserver := buildNorthstarObserver(cwd, cortex.LLMClient, kernForNorthstar)

	edgeCaseDetector := campaign.NewEdgeCaseDetector(realKern, worldScanner)

	// Always constructed: its methods are nil-safe per tool_pregenerator.go,
	// so a nil Ouroboros loop degrades to gap detection instead of a nil
	// ToolPregenerator that the orchestrator would have to nil-check.
	toolPregenerator := campaign.NewToolPregenerator(
		cortex.ToolGenerator(),
		cortex.OuroborosLoop(),
		cortex.MCPStore(),
	)

	cfg.IntelligenceGatherer = intelligenceGatherer
	cfg.AdvisoryBoard = advisoryBoard
	cfg.NorthstarObserver = northstarObserver
	cfg.EdgeCaseDetector = edgeCaseDetector
	cfg.ToolPregenerator = toolPregenerator

	var promptProvider campaign.PromptProvider
	if cortex.PromptAssembler != nil {
		promptProvider = &CampaignJITProvider{assembler: cortex.PromptAssembler}
	} else if cortex.JITCompiler != nil {
		var querier articulation.KernelQuerier
		if realKern != nil {
			if q, ok := any(realKern).(articulation.KernelQuerier); ok {
				querier = q
			}
		}
		if querier == nil && cortex.Kernel != nil {
			if q, ok := any(cortex.Kernel).(articulation.KernelQuerier); ok {
				querier = q
			}
		}
		if querier != nil {
			if pa, err := articulation.NewPromptAssemblerWithJIT(querier, cortex.JITCompiler); err == nil && pa != nil {
				promptProvider = &CampaignJITProvider{assembler: pa}
			}
		}
	}

	return cfg, promptProvider
}

// runCampaignStart starts a new campaign
func runCampaignStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

	llmClient := cortex.LLMClient
	if llmClient == nil {
		return fmt.Errorf("campaign start: cortex boot did not provide an LLM client")
	}
	kern := cortex.Kernel
	if kern == nil && cortex.RealKernel != nil {
		kern = cortex.RealKernel
	}
	if kern == nil {
		return fmt.Errorf("campaign start: cortex boot did not provide a kernel")
	}
	// Worker/planner routing lives in the factory: cortex.TaskExecutor already
	// runs bulk work on the worker tier and reasoning turns on the planner
	// tier. The locals below document the fallback (nil means share the main
	// client, same as the old two-slot split) for the planning pieces kept in
	// this command.
	shardLLM := llmClient
	if w := cortex.WorkerLLM(); w != nil {
		shardLLM = w
	}
	_ = shardLLM
	plannerLLM := cortex.PlannerLLM()
	_ = plannerLLM

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
	orchCfg, campaignPromptProvider := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)
	fmt.Println("   ✓ World scanner initialized")
	fmt.Println("   ✓ Intelligence gatherer initialized")
	fmt.Println("   ✓ Advisory board initialized")
	// The northstar observer prints its own ✓/⚠ inside BuildCampaignObserver.
	fmt.Println("   ✓ Edge case detector initialized")
	if cortex.OuroborosLoop() == nil {
		fmt.Println("   Tool pregenerator: autopoiesis disabled, gap detection only")
	} else {
		fmt.Println("   ✓ Tool pregenerator initialized")
	}
	fmt.Println("   ✓ Intelligence systems initialized")

	// Create decomposer (planning stays on the main client; task execution
	// rides the worker tier inside cortex.TaskExecutor).
	decomposer := campaign.NewDecomposer(kern, llmClient, cwd)
	if cortex.ShardManager != nil {
		decomposer.SetShardLister(cortex.ShardManager)
	}
	if campaignPromptProvider != nil {
		decomposer.SetPromptProvider(campaignPromptProvider)
	}

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
		result, derr = decomposer.Decompose(ctx, req)
		if derr != nil {
			return fmt.Errorf("failed to decompose campaign: %w", derr)
		}
	}

	if !result.ValidationOK {
		fmt.Println("\n⚠️  Plan validation found issues:")
		for _, issue := range result.Issues {
			fmt.Printf("  - [%s] %s\n", issue.IssueType, issue.Description)
		}
		fmt.Println("\nAttempting to proceed anyway...")
	}

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

	// Display plan summary
	fmt.Printf("\n📊 Campaign Plan: %s\n", result.Campaign.Title)
	fmt.Printf("   Confidence: %.0f%%\n", result.Campaign.Confidence*100)
	fmt.Printf("   Phases: %d\n", result.Campaign.TotalPhases)
	fmt.Printf("   Tasks: %d\n\n", result.Campaign.TotalTasks)

	for i, phase := range result.Campaign.Phases {
		fmt.Printf("Phase %d: %s (%d tasks)\n", i+1, phase.Name, len(phase.Tasks))
		for j, task := range phase.Tasks {
			status := "⏳"
			fmt.Printf("  %s %d.%d %s\n", status, i+1, j+1, task.Description)
		}
	}

	orchestrator, err := campaign.NewOrchestrator(orchCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize campaign orchestrator: %w", err)
	}
	if campaignPromptProvider != nil {
		orchestrator.SetPromptProvider(campaignPromptProvider)
	}

	if err := orchestrator.SetCampaign(result.Campaign); err != nil {
		return fmt.Errorf("failed to set campaign: %w", err)
	}

	fmt.Println("\n🚀 Starting campaign execution...")
	fmt.Println("   Press Ctrl+C to pause")

	// Start event listener
	go func() {
		for event := range eventChan {
			switch event.Type {
			case "task_started":
				fmt.Printf("🔄 %s\n", event.Message)
			case "task_completed":
				fmt.Printf("✅ %s\n", event.Message)
			case "task_failed":
				fmt.Printf("❌ %s\n", event.Message)
			case "phase_started":
				fmt.Printf("\n📦 Phase: %s\n", event.Message)
			case "phase_completed":
				fmt.Printf("🎉 Phase completed: %s\n", event.Message)
			case "campaign_completed":
				fmt.Printf("\n🏆 %s\n", event.Message)
			case "replan_triggered":
				fmt.Printf("🔄 Replanning: %s\n", event.Message)
			}
		}
	}()

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

	c, path, err := findLatestPausableCampaign(cwd)
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

	prev := c.Status
	c.Status = campaign.StatusPaused
	c.UpdatedAt = time.Now()
	if err := writeCampaignJSON(path, c); err != nil {
		return fmt.Errorf("failed to persist paused status: %w", err)
	}

	fmt.Printf("Campaign paused: %s\n", c.Title)
	fmt.Printf("   ID: %s (was %s → %s)\n", c.ID, prev, c.Status)
	fmt.Println("Run 'nerd campaign resume' to continue.")
	return nil
}

// runCampaignResume resumes a paused campaign
func runCampaignResume(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	sel := selectResumeCampaign(candidates, campaignRetryFailed)
	if sel == nil {
		fmt.Println("No paused, active, or blocked campaigns found.")
		return nil
	}
	pausedCampaign := sel.Campaign
	campPath := sel.Path
	resumeWasFailed := pausedCampaign.Status == campaign.StatusFailed

	// Flip to active on disk so status/list stay consistent during resume.
	// Paused campaigns only: a failed campaign must still read StatusFailed
	// when PrepareResume runs, which is what re-arms it.
	if pausedCampaign.Status == campaign.StatusPaused {
		pausedCampaign.Status = campaign.StatusActive
		pausedCampaign.UpdatedAt = time.Now()
		if campPath != "" {
			if werr := writeCampaignJSON(campPath, pausedCampaign); werr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist active status: %v\n", werr)
			}
		}
	}

	if pausedCampaign.BlockReason != "" {
		fmt.Printf("Resuming campaign %s (%s, status %s, block reason %s)\n", pausedCampaign.Title, pausedCampaign.ID, pausedCampaign.Status, pausedCampaign.BlockReason)
	} else {
		fmt.Printf("Resuming campaign: %s (status %s)\n", pausedCampaign.Title, pausedCampaign.Status)
	}

	// (Legacy fallback key resolution removed)

	// Boot through the Cortex factory, exactly like runCampaignStart, so resume
	// inherits every factory improvement (session identity, Ouroboros registry,
	// hydration, Dreamer gate adapter, worker/planner routing) and the JIT
	// prompt provider instead of hand-assembling a parallel stack with nil
	// stores and no ToolPregenerator.
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

	orchCfg, campaignPromptProvider := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)

	orchestrator, err := campaign.NewOrchestrator(orchCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize campaign orchestrator: %w", err)
	}
	if campaignPromptProvider != nil {
		orchestrator.SetPromptProvider(campaignPromptProvider)
	}

	if err := orchestrator.SetCampaign(pausedCampaign); err != nil {
		return fmt.Errorf("failed to load campaign: %w", err)
	}
	if err := orchestrator.PrepareResume(); err != nil {
		return fmt.Errorf("cannot resume campaign %s: %w", pausedCampaign.ID, err)
	}
	if resumeWasFailed && campPath != "" {
		if werr := writeCampaignJSON(campPath, pausedCampaign); werr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to persist resumed status: %v\n", werr)
		}
	}

	// Start event listener
	go func() {
		for event := range eventChan {
			switch event.Type {
			case "task_started":
				fmt.Printf("🔄 %s\n", event.Message)
			case "task_completed":
				fmt.Printf("✅ %s\n", event.Message)
			case "task_failed":
				fmt.Printf("❌ %s\n", event.Message)
			case "phase_completed":
				fmt.Printf("🎉 Phase completed: %s\n", event.Message)
			case "campaign_completed":
				fmt.Printf("\n🏆 %s\n", event.Message)
			}
		}
	}()

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

// resumeCandidate pairs a persisted campaign with its on-disk JSON path.
type resumeCandidate struct {
	Campaign *campaign.Campaign
	Path     string
}

// selectResumeCampaign picks which persisted campaign to resume.
// Order of preference: the newest (by UpdatedAt) StatusPaused campaign;
// else the newest StatusActive; else the newest StatusFailed campaign whose
// BlockReason is non-empty; else, only when retryFailed is true, the newest
// StatusFailed campaign regardless of BlockReason; else nil.
// It is pure (no disk, no Cortex) for testability.
func selectResumeCampaign(candidates []resumeCandidate, retryFailed bool) *resumeCandidate {
	var bestPaused, bestActive, bestBlocked, bestFailed *resumeCandidate
	for i := range candidates {
		c := &candidates[i]
		if c.Campaign == nil {
			continue
		}
		switch c.Campaign.Status {
		case campaign.StatusPaused:
			if bestPaused == nil || c.Campaign.UpdatedAt.After(bestPaused.Campaign.UpdatedAt) {
				bestPaused = c
			}
		case campaign.StatusActive:
			if bestActive == nil || c.Campaign.UpdatedAt.After(bestActive.Campaign.UpdatedAt) {
				bestActive = c
			}
		case campaign.StatusFailed:
			if bestFailed == nil || c.Campaign.UpdatedAt.After(bestFailed.Campaign.UpdatedAt) {
				bestFailed = c
			}
			if c.Campaign.BlockReason != "" {
				if bestBlocked == nil || c.Campaign.UpdatedAt.After(bestBlocked.Campaign.UpdatedAt) {
					bestBlocked = c
				}
			}
		}
	}
	if bestPaused != nil {
		return bestPaused
	}
	if bestActive != nil {
		return bestActive
	}
	if bestBlocked != nil {
		return bestBlocked
	}
	if retryFailed && bestFailed != nil {
		return bestFailed
	}
	return nil
}

// loadResumeCandidates reads every non-journal *.json campaign in campaignsDir,
// skipping unreadable or unparseable files the same way the previous
// status-filtered lookup did.
func loadResumeCandidates(campaignsDir string) ([]resumeCandidate, error) {
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		return nil, err
	}
	var out []resumeCandidate
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
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
		cp := c
		out = append(out, resumeCandidate{Campaign: &cp, Path: path})
	}
	return out, nil
}

func writeCampaignJSON(path string, c *campaign.Campaign) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ============================================================================
// SESSION ADAPTERS FOR JITEXECUTOR
// These adapt internal types to the types.* interfaces required by session package.
// ============================================================================

// campaignKernelAdapter adapts core.Kernel to types.Kernel for session package.
type campaignKernelAdapter struct {
	kernel types.Kernel
}

func (a *campaignKernelAdapter) LoadFacts(facts []types.Fact) error {
	return a.kernel.LoadFacts(facts)
}

func (a *campaignKernelAdapter) Query(predicate string) ([]types.Fact, error) {
	return a.kernel.Query(predicate)
}

func (a *campaignKernelAdapter) QueryAll() (map[string][]types.Fact, error) {
	return a.kernel.QueryAll()
}

func (a *campaignKernelAdapter) Assert(fact types.Fact) error {
	return a.kernel.Assert(fact)
}

func (a *campaignKernelAdapter) AssertBatch(facts []types.Fact) error {
	return a.kernel.AssertBatch(facts)
}

func (a *campaignKernelAdapter) Retract(predicate string) error {
	return a.kernel.Retract(predicate)
}

func (a *campaignKernelAdapter) RetractFact(fact types.Fact) error {
	return a.kernel.RetractFact(fact)
}

func (a *campaignKernelAdapter) UpdateSystemFacts() error {
	return a.kernel.UpdateSystemFacts()
}

func (a *campaignKernelAdapter) Reset() {
	a.kernel.Reset()
}

func (a *campaignKernelAdapter) AppendPolicy(policy string) {
	a.kernel.AppendPolicy(policy)
}

func (a *campaignKernelAdapter) RetractExactFactsBatch(facts []types.Fact) error {
	return a.kernel.RetractExactFactsBatch(facts)
}

func (a *campaignKernelAdapter) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return a.kernel.RemoveFactsByPredicateSet(predicates)
}

// campaignVirtualStoreAdapter adapts core.VirtualStore to types.VirtualStore.
type campaignVirtualStoreAdapter struct {
	vs *core.VirtualStore
}

func (a *campaignVirtualStoreAdapter) ReadFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func (a *campaignVirtualStoreAdapter) WriteFile(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(content), 0644)
}

func (a *campaignVirtualStoreAdapter) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", fmt.Errorf("exec not implemented in campaign adapter")
}

func (a *campaignVirtualStoreAdapter) ReadRaw(path string) ([]byte, error) {
	if a.vs != nil {
		return a.vs.ReadRaw(path)
	}
	return os.ReadFile(path)
}

// Compile-time assertion that the campaign adapter exposes the Dreamer
// preflight and post-action validator seam. Without this, the session
// executor's type assertion silently skips both gates on every campaign task.
var _ session.InteractiveExecutiveGate = (*campaignVirtualStoreAdapter)(nil)

// isCampaignAdapterDestructiveTool mirrors the core gate's destructive
// classification for interactive tool names (see
// internal/core/virtual_store_interactive_gate.go interactiveToolActionType
// filtered by isDestructiveAction). Only read_file is non-destructive; every
// other mapped tool mutates files or executes code.
func isCampaignAdapterDestructiveTool(toolName string) bool {
	switch toolName {
	case "write_file", "edit_file", "delete_file",
		"run_command", "bash", "run_build",
		"edit_lines", "insert_lines", "delete_lines",
		"edit_element", "apply_edits":
		return true
	default:
		return false
	}
}

// PreflightDestructiveToolCall delegates to the wrapped VirtualStore's Dreamer
// gate, satisfying session.InteractiveExecutiveGate so campaign task execution
// runs the safety simulation before destructive tool calls.
//
// Nil handling is deliberately fail-CLOSED for destructive tools, matching the
// core gate's documented policy (every mapped destructive tool requires a
// usable Dreamer): a nil store blocks destructive tools instead of allowing
// them. This differs from the TUI chat adapter
// (cmd/nerd/chat/session_adapters.go), which is fail-OPEN on a nil store to
// preserve its prior behavior.
func (a *campaignVirtualStoreAdapter) PreflightDestructiveToolCall(ctx context.Context, actionID, toolName string, args map[string]any) error {
	if a == nil || a.vs == nil {
		if isCampaignAdapterDestructiveTool(toolName) {
			return &core.InteractiveGateError{Reason: "interactive executive gate unavailable: VirtualStore is nil; blocked destructive tool " + toolName}
		}
		return nil
	}
	return a.vs.PreflightDestructiveToolCall(ctx, actionID, toolName, args)
}

// ValidateInteractiveToolResult delegates to the wrapped VirtualStore's
// post-action validator registry, satisfying session.InteractiveExecutiveGate.
// Nil store => no validation (a post-action check cannot fail closed without a
// side effect to verify).
func (a *campaignVirtualStoreAdapter) ValidateInteractiveToolResult(ctx context.Context, actionID, toolName string, args map[string]any, output string, success bool) error {
	if a == nil || a.vs == nil {
		return nil
	}
	return a.vs.ValidateInteractiveToolResult(ctx, actionID, toolName, args, output, success)
}

// campaignLLMAdapter adapts perception.LLMClient to types.LLMClient.
type campaignLLMAdapter struct {
	client perception.LLMClient
}

// newCampaignLLMAdapter wraps a client for the session layer. Both campaign
// paths shadow the type name with a local variable, so construction goes
// through this helper rather than a composite literal.
func newCampaignLLMAdapter(client perception.LLMClient) *campaignLLMAdapter {
	return &campaignLLMAdapter{client: client}
}

func (a *campaignLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	return a.client.Complete(ctx, prompt)
}

func (a *campaignLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (a *campaignLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return a.client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

// CompleteWithToolResults forwards multi-turn tool results so campaign coder
// shards can write files after glob/list (SuperGrok / API path). Without this,
// the session executor aborts after the first tool batch and campaigns stall
// on hollow-success / missing micro-checkpoint paths.
func (a *campaignLLMAdapter) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("campaign LLM client is nil")
	}
	if trp, ok := a.client.(types.ToolResultsProvider); ok {
		return trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	}
	// ScheduledLLMCall and perception clients may implement the method without
	// being types.ToolResultsProvider on the interface value after wrapping.
	type perceptionTRP interface {
		CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
	}
	if trp, ok := a.client.(perceptionTRP); ok {
		return trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	}
	return nil, fmt.Errorf("LLM client %T does not implement ToolResultsProvider", a.client)
}

// campaignTaskExecutorConsultationSpawner adapts TaskExecutor to ConsultationSpawner.
type campaignTaskExecutorConsultationSpawner struct {
	executor session.TaskExecutor
}

func (s *campaignTaskExecutorConsultationSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if s.executor == nil {
		return "", fmt.Errorf("task executor not available")
	}
	// Specialists arrive here as free-form names ("nemesis", "requirements_interrogator", etc.).
	// Wrap them in /consult/<name> so the executor's strict IntentVerb check accepts the request.
	intent := specialistName
	if !strings.HasPrefix(intent, "/") {
		intent = "/consult/" + specialistName
	}
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       task,
	}
	return s.executor.Execute(ctx, req)
}

// campaignConsultationProviderAdapter adapts shards.ConsultationManager to campaign.ConsultationProvider.
type campaignConsultationProviderAdapter struct {
	manager *shards.ConsultationManager
}

func newCampaignConsultationProvider(manager *shards.ConsultationManager) campaign.ConsultationProvider {
	if manager == nil {
		return nil
	}
	return &campaignConsultationProviderAdapter{manager: manager}
}

func (a *campaignConsultationProviderAdapter) RequestBatchConsultation(ctx context.Context, request campaign.BatchConsultRequest) ([]campaign.ConsultationResponse, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("consultation manager not configured")
	}

	question := strings.TrimSpace(request.Question)
	if topic := strings.TrimSpace(request.Topic); topic != "" {
		if question == "" {
			question = topic
		} else {
			question = "[" + topic + "] " + question
		}
	}

	targets := request.TargetSpec
	if len(targets) == 0 {
		targets = []string{"coder", "tester", "reviewer", "researcher"}
	}

	responses, err := a.manager.RequestBatchConsultation(ctx, question, request.Context, targets)
	if err != nil {
		return nil, err
	}

	converted := make([]campaign.ConsultationResponse, 0, len(responses))
	for _, resp := range responses {
		converted = append(converted, campaign.ConsultationResponse{
			RequestID:    resp.RequestID,
			FromSpec:     resp.FromSpec,
			ToSpec:       resp.ToSpec,
			Advice:       resp.Advice,
			Confidence:   resp.Confidence,
			References:   resp.References,
			Caveats:      resp.Caveats,
			Metadata:     resp.Metadata,
			ResponseTime: resp.ResponseTime,
			Duration:     resp.Duration,
		})
	}

	return converted, nil
}

func (a *campaignKernelAdapter) GetProgramInfo() *analysis.ProgramInfo {
	return a.kernel.GetProgramInfo()
}

func (a *campaignLLMAdapter) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}

type campaignTaskDelegatorAdapter struct {
	executor session.TaskExecutor
}

func (a *campaignTaskDelegatorAdapter) Execute(ctx context.Context, intent string, task string) (string, error) {
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       task,
	}
	return a.executor.Execute(ctx, req)
}

// applyCampaignExecutorBudget gives a CLI campaign's executor and spawner the
// same tool-loop budget the TUI gets from internal/system/factory.go.
//
// Neither of the two CLI campaign paths called SetConfig, so both silently ran
// on DefaultExecutorConfig: 8 iterations regardless of
// core_limits.max_tool_iterations, and an empty WorkspaceRoot. Eight is low for
// the research-heavy work a campaign task does — measured live, a turn reported
// "No exploration budget left" after eight iterations while the configured
// budget was 24.
func applyCampaignExecutorBudget(
	appCfg *config.UserConfig,
	workspace string,
	executor *session.Executor,
	spawner *session.Spawner,
) session.ExecutorConfig {
	execCfg := session.DefaultExecutorConfig()
	execCfg.WorkspaceRoot = workspace
	if appCfg != nil {
		limits := appCfg.GetCoreLimits()
		if limits.MaxToolCalls > 0 {
			execCfg.MaxToolCalls = limits.MaxToolCalls
		}
		if limits.MaxToolIterations > 0 {
			execCfg.MaxToolIterations = limits.MaxToolIterations
		}
		if limits.AdaptiveToolBudget != nil {
			execCfg.AdaptiveToolBudget = *limits.AdaptiveToolBudget
		}
		if limits.ToolIterationExtensionSize > 0 {
			execCfg.ToolIterationExtensionSize = limits.ToolIterationExtensionSize
		}
		if limits.MaxToolIterationExtensions > 0 {
			execCfg.MaxToolIterationExtensions = limits.MaxToolIterationExtensions
		}
		if limits.ToolLoopRepeatThreshold > 0 {
			execCfg.ToolLoopRepeatThreshold = limits.ToolLoopRepeatThreshold
		}
	}
	if executor != nil {
		executor.SetConfig(execCfg)
	}
	if spawner != nil {
		spawner.SetExecutorConfig(&execCfg)
	}
	return execCfg
}

// buildNorthstarObserver is a thin wrapper around northstar.BuildCampaignObserver
// kept for backwards compatibility and to avoid duplicating the construction
// logic. The canonical implementation lives in internal/northstar/campaign_observer.go.
func buildNorthstarObserver(cwd string, llmClient perception.LLMClient, kern types.Kernel) *northstar.CampaignObserver {
	return northstar.BuildCampaignObserver(cwd, llmClient, kern)
}
