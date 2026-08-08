package main

import (
	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile"
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

// newCampaignLLMClients builds the main and worker LLM clients for a campaign
// run from the user's config.
//
// The campaign CLI previously called perception.NewClientFromEnv(), which reads
// ONLY environment variables — and it ran before the config was even loaded. So
// `nerd campaign start` silently ignored the configured provider, model and
// worker, and picked whichever key happened to be exported. Resolving from
// UserConfig first (env remains the fallback when the config expresses no
// choice) makes the CLI agree with the chat path and with `config is boss`.
//
// The returned worker is nil when none is configured; callers should fall back
// to the main client in that case.
// campaignLLMClients holds the model slots a campaign run needs. Main plans,
// worker executes bulk tasks, planner (when configured) serves the delegated
// turns the kernel derives as reasoning-intensive. A nil slot means "fall back
// to the next one up".
type campaignLLMClients struct {
	main    perception.LLMClient
	worker  perception.LLMClient
	planner perception.LLMClient
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

func newCampaignLLMClients(appCfg *config.UserConfig, label string) (campaignLLMClients, error) {
	var out campaignLLMClients

	main, err := newConfiguredLLMClient(appCfg, label)
	if err != nil {
		return out, err
	}
	out.main = main

	if worker, werr := perception.NewWorkerClientFromUserConfig(appCfg); werr != nil {
		fmt.Fprintf(os.Stderr, "Warning: worker LLM init failed: %v (shards share main client)\n", werr)
	} else if worker != nil {
		out.worker = core.NewScheduledLLMCall(label+"-worker", worker)
	}

	if planner, perr := perception.NewPlannerClientFromUserConfig(appCfg); perr != nil {
		fmt.Fprintf(os.Stderr, "Warning: planner LLM init failed: %v (reasoning tasks stay on the worker client)\n", perr)
	} else if planner != nil {
		out.planner = core.NewScheduledLLMCall(label+"-planner", planner)
	}

	return out, nil
}

// campaignCmd is the parent command for campaign operations
var campaignCmd = &cobra.Command{
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

	// Resolve .nerd directory for JIT prompt system
	nerdDir := filepath.Join(cwd, ".nerd")

	// Load user config BEFORE building the LLM client, so the client honors the
	// configured provider/model/worker, and so the VirtualStore honors the
	// user's allowed_binaries / allowed_env_vars whitelist. Previously the store
	// was built from DefaultVirtualStoreConfig (which re-adds bash/sh/cmd
	// regardless of the user's policy).
	appCfg := loadCampaignConfig(nerdDir)

	llmSlots, clientErr := newCampaignLLMClients(appCfg, "campaign-cli")
	if clientErr != nil {
		return clientErr
	}
	llmClient, workerLLMClient := llmSlots.main, llmSlots.worker

	exec := appCfg.GetExecution()
	vsCfg := core.DefaultVirtualStoreConfig()
	if len(exec.AllowedBinaries) > 0 {
		vsCfg.AllowedBinaries = exec.AllowedBinaries
	}
	if len(exec.AllowedEnvVars) > 0 {
		vsCfg.AllowedEnvVars = exec.AllowedEnvVars
	}
	if exec.WorkingDirectory != "" {
		vsCfg.WorkingDir = exec.WorkingDirectory
	}

	// Initialize components
	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	executor := tactile.NewDirectExecutor()
	virtualStore := core.NewVirtualStoreWithConfig(executor, vsCfg)
	virtualStore.DisableBootGuard() // CLI commands are user-initiated, disable boot guard

	// FIX: Wire persistence layers
	var localDB *store.LocalStore
	var learningStore *store.LearningStore

	knowledgeDBPath := filepath.Join(nerdDir, "knowledge.db")
	if db, err := store.NewLocalStore(knowledgeDBPath); err == nil {
		localDB = db
		defer localDB.Close()
		virtualStore.SetLocalDB(localDB)
		virtualStore.SetKernel(kern)
		// Wire knowledge graph query bridge for Mangle query_graph virtual predicate.
		if gqAdapter := store.NewLocalStoreGraphAdapter(localDB); gqAdapter != nil {
			virtualStore.SetGraphQuery(gqAdapter)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to open knowledge DB: %v\n", err)
	}

	learningStorePath := filepath.Join(nerdDir, "shards")
	if ls, err := store.NewLearningStore(learningStorePath); err == nil {
		learningStore = ls
		defer learningStore.Close()
		virtualStore.SetLearningStore(learningStore)
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to open learning store: %v\n", err)
	}

	// FIX(BUG-005): Hydrate modular tools so JITExecutor can use them
	if err := virtualStore.HydrateModularTools(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to hydrate modular tools: %v\n", err)
	}

	shardMgr := coreshards.NewShardManager()
	shardMgr.SetParentKernel(kern)

	// Initialize limits enforcer and spawn queue (appCfg loaded above)
	coreLimits := appCfg.GetCoreLimits()
	jitCfg := appCfg.GetEffectiveJITConfig()
	// Configure global LLM API concurrency

	// Configure global LLM API concurrency
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = appCfg.GetEffectiveMaxConcurrentAPICalls()
	schedulerCfg.SlotAcquireTimeout = config.GetLLMTimeouts().SlotAcquisitionTimeout
	core.ConfigureGlobalAPIScheduler(schedulerCfg)

	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:      coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:   coreLimits.MaxConcurrentShards,
		MaxSessionDurationMin: coreLimits.MaxSessionDurationMin,
		MaxFactsInKernel:      coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit:  coreLimits.MaxDerivedFactsLimit,
	})
	shardMgr.SetLimitsEnforcer(limitsEnforcer)
	spawnQueue := coreshards.NewSpawnQueue(shardMgr, limitsEnforcer, coreshards.DefaultSpawnQueueConfig())
	shardMgr.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	// Initialize JIT Prompt Compiler
	compilerCfg := prompt.DefaultCompilerConfig()
	if jitCfg.TokenBudget > 0 {
		compilerCfg.DefaultTokenBudget = jitCfg.TokenBudget
	}

	// FIX(BUG-004): Load embedded corpus - required for JIT atoms to be available
	// Without this, the campaign planner gets an empty system prompt
	embeddedCorpus, embeddedErr := prompt.LoadEmbeddedCorpus()
	if embeddedErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load embedded corpus: %v\n", embeddedErr)
	}

	// Build compiler options with embedded corpus
	compilerOpts := []prompt.CompilerOption{
		prompt.WithKernel(coresys.NewKernelAdapter(kern)),
		prompt.WithConfig(compilerCfg),
	}
	if embeddedCorpus != nil {
		compilerOpts = append(compilerOpts, prompt.WithEmbeddedCorpus(embeddedCorpus))
	}

	jitCompiler, err := prompt.NewJITPromptCompiler(compilerOpts...)
	if err != nil {
		return fmt.Errorf("failed to init JIT compiler: %w", err)
	}

	// Wire JIT lifecycle callbacks
	shardMgr.SetNerdDir(nerdDir)
	shardMgr.SetJITRegistrar(prompt.CreateJITDBRegistrar(jitCompiler))
	shardMgr.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(jitCompiler))

	// Shard and task execution use the worker client when one is configured;
	// campaign planning (Decomposer / Replanner, wired below) deliberately stays
	// on the main client. That is the point of the two-slot split: reason about
	// the plan with the strong model, execute the bulk work with the cheap one.
	shardLLM := llmClient
	if workerLLMClient != nil {
		shardLLM = workerLLMClient
	}

	// Register shard factories
	shardMgr.SetLLMClient(shardLLM)
	// Image generation (Nano Banana 2) stays off the campaign worker/main client.
	if imgClient, ierr := perception.NewImageClientFromUserConfig(appCfg); ierr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Image LLM (Nano Banana 2) unavailable: %v\n", ierr)
	} else if imgClient != nil {
		shardMgr.SetImageLLMClient(core.NewScheduledLLMCall("image_generator", imgClient))
	}
	shards.RegisterAllShardFactories(shardMgr, shards.RegistryContext{
		Kernel:       kern,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
		Workspace:    cwd,
		JITCompiler:  jitCompiler,
		JITConfig:    jitCfg,
	})

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║          CAMPAIGN ORCHESTRATOR - INITIALIZING             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Printf("\nGoal: %s\n", goal)
	if len(docs) > 0 {
		fmt.Printf("Source Documents: %v\n", docs)
	}
	fmt.Printf("Campaign Type: %s\n\n", campaignType)

	// Create a PromptAssembler-backed provider
	var campaignPromptProvider campaign.PromptProvider
	if pa, err := articulation.NewPromptAssemblerWithJIT(kern, jitCompiler); err == nil {
		pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK, jitCfg.ReservedTokensFallbackRatio)
		pa.EnableJIT(jitCfg.Enabled)
		campaignPromptProvider = &CampaignJITProvider{assembler: pa}
	}

	// Create decomposer
	decomposer := campaign.NewDecomposer(kern, llmClient, cwd)
	decomposer.SetShardLister(shardMgr)
	if campaignPromptProvider != nil {
		decomposer.SetPromptProvider(campaignPromptProvider)
	}

	// Build request with context budget from config
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

	fmt.Println("📋 Decomposing goal into phases and tasks...")

	// Decompose
	result, err := decomposer.Decompose(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to decompose campaign: %w", err)
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

	// Create and start orchestrator
	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	// Create JITExecutor for campaign task execution (replaces deleted domain shards)
	transducer := perception.NewUnderstandingTransducer(llmClient)
	configFactory := prompt.NewDefaultConfigFactory()
	campaignKernelAdapter := &campaignKernelAdapter{kernel: kern}
	campaignVSAdapter := &campaignVirtualStoreAdapter{vs: virtualStore}
	// Task execution rides the worker client (see shardLLM above); the campaign
	// Orchestrator is still constructed with the main client for planning.
	campaignLLMAdapter := &campaignLLMAdapter{client: shardLLM}

	sessionExecutor := session.NewExecutor(
		campaignKernelAdapter,
		campaignVSAdapter,
		campaignLLMAdapter,
		jitCompiler,
		configFactory,
		transducer,
	)

	sessionSpawner := session.NewSpawner(
		campaignKernelAdapter,
		campaignVSAdapter,
		campaignLLMAdapter,
		jitCompiler,
		configFactory,
		transducer,
		session.DefaultSpawnerConfig(),
	)

	if llmSlots.planner != nil {
		plannerAdapter := newCampaignLLMAdapter(llmSlots.planner)
		sessionExecutor.SetPlannerClient(plannerAdapter)
		sessionSpawner.SetPlannerClient(plannerAdapter)
	}

	taskExecutor := session.NewJITExecutor(sessionExecutor, sessionSpawner, transducer)
	virtualStore.SetTaskExecutor(&campaignTaskDelegatorAdapter{executor: taskExecutor})
	// ShardManager has no factory for the domain personas or user-defined
	// agents a campaign consults; route those through the clean loop instead of
	// the BaseShardAgent placeholder that reported success without doing work.
	shardMgr.SetTaskDelegator(&campaignTaskDelegatorAdapter{executor: taskExecutor})
	consultationMgr := shards.NewConsultationManager(&campaignTaskExecutorConsultationSpawner{executor: taskExecutor})
	consultationProvider := newCampaignConsultationProvider(consultationMgr)

	// Initialize Intelligence Integration components (Campaign Intelligence Plan)
	fmt.Println("🧠 Initializing intelligence gathering systems...")

	// Create world.Scanner for codebase analysis
	worldScanner := world.NewScanner()
	fmt.Println("   ✓ World scanner initialized")

	// Create IntelligenceGatherer - orchestrates pre-planning intelligence from 12 systems
	holographic := world.NewHolographicProvider(kern, cwd)
	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		kern,          // kernel
		worldScanner,  // worldScanner - codebase analysis
		holographic,   // holographic - rich codebase context
		learningStore, // learningStore - historical patterns
		localDB,       // localStore - knowledge graph + cold storage
		nil,           // toolGenerator - not yet wired in CLI mode
		nil,           // mcpStore - not yet wired in CLI mode
		consultationProvider,
	)
	fmt.Println("   ✓ Intelligence gatherer initialized")

	// Create ShardAdvisoryBoard - domain experts review plans
	advisoryBoard := campaign.NewShardAdvisoryBoard(consultationProvider)
	fmt.Println("   ✓ Advisory board initialized")

	// Create EdgeCaseDetector - file action decisions (create/extend/modularize)
	edgeCaseDetector := campaign.NewEdgeCaseDetector(kern, worldScanner)
	fmt.Println("   ✓ Edge case detector initialized")

	// Create ToolPregenerator - pre-generate tools via Ouroboros
	var toolPregenerator *campaign.ToolPregenerator
	// Note: Requires autopoiesis.OuroborosLoop which isn't wired in CLI mode yet
	fmt.Println("   ⚠ Tool pregenerator pending (requires Ouroboros)")

	fmt.Println("   ✓ Intelligence systems initialized")

	orchestrator, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:            cwd,
		Kernel:               kern,
		LLMClient:            llmClient,
		ShardManager:         shardMgr,
		TaskExecutor:         taskExecutor,
		Executor:             executor,
		VirtualStore:         virtualStore,
		ProgressChan:         progressChan,
		EventChan:            eventChan,
		IntelligenceGatherer: intelligenceGatherer,
		AdvisoryBoard:        advisoryBoard,
		EdgeCaseDetector:     edgeCaseDetector,
		ToolPregenerator:     toolPregenerator,
	})
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
			return nil
		}
		return fmt.Errorf("campaign failed: %w", err)
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

	// Prefer StatusPaused; fall back to Active (legacy interrupted runs).
	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	pausedCampaign, campPath, err := findCampaignByStatuses(campaignsDir, campaign.StatusPaused)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No campaigns found.")
			return nil
		}
		return err
	}
	if pausedCampaign == nil {
		pausedCampaign, campPath, err = findCampaignByStatuses(campaignsDir, campaign.StatusActive)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if pausedCampaign == nil {
		fmt.Println("No paused campaigns found.")
		return nil
	}

	// Flip to active on disk so status/list stay consistent during resume.
	if pausedCampaign.Status == campaign.StatusPaused {
		pausedCampaign.Status = campaign.StatusActive
		pausedCampaign.UpdatedAt = time.Now()
		if campPath != "" {
			if werr := writeCampaignJSON(campPath, pausedCampaign); werr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist active status: %v\n", werr)
			}
		}
	}

	fmt.Printf("Resuming campaign: %s\n", pausedCampaign.Title)

	// (Legacy fallback key resolution removed)

	// Load user config BEFORE building the LLM client, so the client honors the
	// configured provider/model/worker and the VirtualStore honors the user's
	// allowed_binaries / allowed_env_vars whitelist (see runCampaignStart).
	appCfg := loadCampaignConfig(filepath.Dir(config.DefaultUserConfigPath()))

	llmSlots, clientErr := newCampaignLLMClients(appCfg, "campaign-resume")
	if clientErr != nil {
		return clientErr
	}
	llmClient, workerLLMClient := llmSlots.main, llmSlots.worker
	exec := appCfg.GetExecution()
	vsCfg := core.DefaultVirtualStoreConfig()
	if len(exec.AllowedBinaries) > 0 {
		vsCfg.AllowedBinaries = exec.AllowedBinaries
	}
	if len(exec.AllowedEnvVars) > 0 {
		vsCfg.AllowedEnvVars = exec.AllowedEnvVars
	}
	if exec.WorkingDirectory != "" {
		vsCfg.WorkingDir = exec.WorkingDirectory
	}

	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	executor := tactile.NewDirectExecutor()
	virtualStore := core.NewVirtualStoreWithConfig(executor, vsCfg)
	virtualStore.DisableBootGuard() // CLI commands are user-initiated, disable boot guard

	// FIX(BUG-005): Hydrate modular tools so JITExecutor can use them
	if err := virtualStore.HydrateModularTools(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to hydrate modular tools: %v\n", err)
	}

	shardMgr := coreshards.NewShardManager()
	shardMgr.SetParentKernel(kern)

	// Initialize limits enforcer and spawn queue (appCfg loaded above)
	coreLimits := appCfg.GetCoreLimits()
	jitCfg := appCfg.GetEffectiveJITConfig()

	// Configure global LLM API concurrency
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = appCfg.GetEffectiveMaxConcurrentAPICalls()
	schedulerCfg.SlotAcquireTimeout = config.GetLLMTimeouts().SlotAcquisitionTimeout
	core.ConfigureGlobalAPIScheduler(schedulerCfg)

	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:      coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:   coreLimits.MaxConcurrentShards,
		MaxSessionDurationMin: coreLimits.MaxSessionDurationMin,
		MaxFactsInKernel:      coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit:  coreLimits.MaxDerivedFactsLimit,
	})
	shardMgr.SetLimitsEnforcer(limitsEnforcer)
	spawnQueue := coreshards.NewSpawnQueue(shardMgr, limitsEnforcer, coreshards.DefaultSpawnQueueConfig())
	shardMgr.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	// Initialize JIT Prompt Compiler
	compilerCfg := prompt.DefaultCompilerConfig()
	if jitCfg.TokenBudget > 0 {
		compilerCfg.DefaultTokenBudget = jitCfg.TokenBudget
	}

	// FIX(BUG-004): Load embedded corpus - required for JIT atoms to be available
	embeddedCorpus, embeddedErr := prompt.LoadEmbeddedCorpus()
	if embeddedErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load embedded corpus: %v\n", embeddedErr)
	}

	compilerOpts := []prompt.CompilerOption{
		prompt.WithKernel(coresys.NewKernelAdapter(kern)),
		prompt.WithConfig(compilerCfg),
	}
	if embeddedCorpus != nil {
		compilerOpts = append(compilerOpts, prompt.WithEmbeddedCorpus(embeddedCorpus))
	}

	jitCompiler, err := prompt.NewJITPromptCompiler(compilerOpts...)
	if err != nil {
		return fmt.Errorf("failed to init JIT compiler: %w", err)
	}

	// Worker client for shard/task execution; planning stays on main.
	shardLLM := llmClient
	if workerLLMClient != nil {
		shardLLM = workerLLMClient
	}

	// Register shard factories
	shardMgr.SetLLMClient(shardLLM)
	// Image generation (Nano Banana 2) stays off the campaign worker/main client.
	if imgClient, ierr := perception.NewImageClientFromUserConfig(appCfg); ierr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Image LLM (Nano Banana 2) unavailable: %v\n", ierr)
	} else if imgClient != nil {
		shardMgr.SetImageLLMClient(core.NewScheduledLLMCall("image_generator", imgClient))
	}
	shards.RegisterAllShardFactories(shardMgr, shards.RegistryContext{
		Kernel:       kern,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
		Workspace:    cwd,
		JITCompiler:  jitCompiler,
		JITConfig:    jitCfg,
	})

	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	// Create JITExecutor for campaign task execution (replaces deleted domain shards)
	transducer := perception.NewUnderstandingTransducer(llmClient)
	configFactory := prompt.NewDefaultConfigFactory()
	resumeKernelAdapter := &campaignKernelAdapter{kernel: kern}
	resumeVSAdapter := &campaignVirtualStoreAdapter{vs: virtualStore}
	// Task execution rides the worker client; the Orchestrator keeps the main
	// client for planning.
	resumeLLMAdapter := &campaignLLMAdapter{client: shardLLM}

	sessionExecutor := session.NewExecutor(
		resumeKernelAdapter,
		resumeVSAdapter,
		resumeLLMAdapter,
		jitCompiler,
		configFactory,
		transducer,
	)

	sessionSpawner := session.NewSpawner(
		resumeKernelAdapter,
		resumeVSAdapter,
		resumeLLMAdapter,
		jitCompiler,
		configFactory,
		transducer,
		session.DefaultSpawnerConfig(),
	)

	if llmSlots.planner != nil {
		plannerAdapter := newCampaignLLMAdapter(llmSlots.planner)
		sessionExecutor.SetPlannerClient(plannerAdapter)
		sessionSpawner.SetPlannerClient(plannerAdapter)
	}

	taskExecutor := session.NewJITExecutor(sessionExecutor, sessionSpawner, transducer)
	virtualStore.SetTaskExecutor(&campaignTaskDelegatorAdapter{executor: taskExecutor})
	// ShardManager has no factory for the domain personas or user-defined
	// agents a campaign consults; route those through the clean loop instead of
	// the BaseShardAgent placeholder that reported success without doing work.
	shardMgr.SetTaskDelegator(&campaignTaskDelegatorAdapter{executor: taskExecutor})
	consultationMgr := shards.NewConsultationManager(&campaignTaskExecutorConsultationSpawner{executor: taskExecutor})
	consultationProvider := newCampaignConsultationProvider(consultationMgr)

	// Initialize campaign intelligence components for deterministic gating during resume.
	worldScanner := world.NewScanner()
	holographic := world.NewHolographicProvider(kern, cwd)
	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		kern,
		worldScanner,
		holographic,
		nil,
		nil,
		nil,
		nil,
		consultationProvider,
	)
	advisoryBoard := campaign.NewShardAdvisoryBoard(consultationProvider)
	edgeCaseDetector := campaign.NewEdgeCaseDetector(kern, worldScanner)

	orchestrator, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:            cwd,
		Kernel:               kern,
		LLMClient:            llmClient,
		ShardManager:         shardMgr,
		TaskExecutor:         taskExecutor,
		Executor:             executor,
		VirtualStore:         virtualStore,
		ProgressChan:         progressChan,
		EventChan:            eventChan,
		IntelligenceGatherer: intelligenceGatherer,
		AdvisoryBoard:        advisoryBoard,
		EdgeCaseDetector:     edgeCaseDetector,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize campaign orchestrator: %w", err)
	}

	if err := orchestrator.SetCampaign(pausedCampaign); err != nil {
		return fmt.Errorf("failed to load campaign: %w", err)
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
			return nil
		}
		return fmt.Errorf("campaign failed: %w", err)
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

// findCampaignByStatuses returns the first campaign matching any of the given
// statuses (most recent UpdatedAt wins).
func findCampaignByStatuses(campaignsDir string, statuses ...campaign.CampaignStatus) (*campaign.Campaign, string, error) {
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		return nil, "", err
	}
	want := make(map[campaign.CampaignStatus]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}

	var best *campaign.Campaign
	var bestPath string
	var bestTime time.Time
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
		if !want[c.Status] {
			continue
		}
		if best == nil || c.UpdatedAt.After(bestTime) {
			cp := c
			best = &cp
			bestPath = path
			bestTime = c.UpdatedAt
		}
	}
	return best, bestPath, nil
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
