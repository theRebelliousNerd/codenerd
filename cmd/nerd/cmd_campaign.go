package main

import (
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

	// FIX: Respect authenticated engine configuration instead of hardcoding ZAI
	rawLLMClient, clientErr := perception.NewClientFromEnv()
	if clientErr != nil {
		// Fallback to ZAI if config detection fails
		key := apiKey
		if key == "" {
			key = os.Getenv("ZAI_API_KEY")
		}
		rawLLMClient = perception.NewZAIClient(key)
		fmt.Printf("⚠ Using fallback ZAI client: %v\n", clientErr)
	}
	// Wrap with APIScheduler to enforce concurrency limits (max 5 for Z.AI)
	llmClient := core.NewScheduledLLMCall("campaign-cli", rawLLMClient)

	// Resolve .nerd directory for JIT prompt system
	nerdDir := filepath.Join(cwd, ".nerd")

	// Initialize components
	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	executor := tactile.NewDirectExecutor()
	virtualStore := core.NewVirtualStore(executor)
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

	// Initialize limits enforcer and spawn queue
	userCfgPath := filepath.Join(nerdDir, "config.json")
	appCfg, _ := config.LoadUserConfig(userCfgPath)
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	coreLimits := appCfg.GetCoreLimits()
	jitCfg := appCfg.GetEffectiveJITConfig()
	// Configure global LLM API concurrency

	// Configure global LLM API concurrency
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = coreLimits.MaxConcurrentAPICalls
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

	// Register shard factories
	shardMgr.SetLLMClient(llmClient)
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
		pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK)
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
	campaignLLMAdapter := &campaignLLMAdapter{client: llmClient}

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

	taskExecutor := session.NewJITExecutor(sessionExecutor, sessionSpawner, transducer)
	virtualStore.SetTaskExecutor(taskExecutor)

	// Initialize Intelligence Integration components (Campaign Intelligence Plan)
	fmt.Println("🧠 Initializing intelligence gathering systems...")

	// Create world.Scanner for codebase analysis
	worldScanner := world.NewScanner()
	fmt.Println("   ✓ World scanner initialized")

	// Create IntelligenceGatherer - orchestrates pre-planning intelligence from 12 systems
	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		kern,          // kernel
		worldScanner,  // worldScanner - codebase analysis
		nil,           // holographic - not yet wired in CLI mode
		learningStore, // learningStore - historical patterns
		localDB,       // localStore - knowledge graph + cold storage
		nil,           // toolGenerator - not yet wired in CLI mode
		nil,           // mcpStore - not yet wired in CLI mode
		nil,           // consultation - interface mismatch, needs adapter
	)
	fmt.Println("   ✓ Intelligence gatherer initialized")

	// Create ShardAdvisoryBoard - domain experts review plans
	var advisoryBoard *campaign.ShardAdvisoryBoard
	// Note: NewShardAdvisoryBoard expects campaign.ConsultationProvider interface
	// which requires an adapter from shards.ConsultationManager
	// For now, leave as nil - can be added later with proper adapter
	fmt.Println("   ⚠ Advisory board pending (needs interface adapter)")

	// Create EdgeCaseDetector - file action decisions (create/extend/modularize)
	edgeCaseDetector := campaign.NewEdgeCaseDetector(kern, worldScanner)
	fmt.Println("   ✓ Edge case detector initialized")

	// Create ToolPregenerator - pre-generate tools via Ouroboros
	var toolPregenerator *campaign.ToolPregenerator
	// Note: Requires autopoiesis.OuroborosLoop which isn't wired in CLI mode yet
	fmt.Println("   ⚠ Tool pregenerator pending (requires Ouroboros)")

	fmt.Println("   ✓ Intelligence systems initialized")

	orchestrator := campaign.NewOrchestrator(campaign.OrchestratorConfig{
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
			fmt.Println("\nCampaign paused. Run 'nerd campaign resume' to continue.")
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

// runCampaignPause pauses the current campaign
func runCampaignPause(cmd *cobra.Command, args []string) error {
	fmt.Println("Campaign paused. Run 'nerd campaign resume' to continue.")
	// The actual pausing happens via signal handling in the running orchestrator
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

	// Find paused campaign
	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		fmt.Println("No campaigns found.")
		return nil
	}

	var pausedCampaign *campaign.Campaign
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

			if c.Status == campaign.StatusPaused || c.Status == campaign.StatusActive {
				pausedCampaign = &c
				break
			}
		}
	}

	if pausedCampaign == nil {
		fmt.Println("No paused campaigns found.")
		return nil
	}

	fmt.Printf("Resuming campaign: %s\n", pausedCampaign.Title)

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Initialize components
	rawLLMClient, clientErr := perception.NewClientFromEnv()
	if clientErr != nil {
		rawLLMClient = perception.NewZAIClient(key)
		fmt.Printf("⚠ Using fallback ZAI client: %v\n", clientErr)
	}
	// Wrap with APIScheduler to enforce concurrency limits (max 5 for Z.AI)
	llmClient := core.NewScheduledLLMCall("campaign-resume", rawLLMClient)
	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	executor := tactile.NewDirectExecutor()
	virtualStore := core.NewVirtualStore(executor)
	virtualStore.DisableBootGuard() // CLI commands are user-initiated, disable boot guard

	// FIX(BUG-005): Hydrate modular tools so JITExecutor can use them
	if err := virtualStore.HydrateModularTools(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to hydrate modular tools: %v\n", err)
	}

	shardMgr := coreshards.NewShardManager()
	shardMgr.SetParentKernel(kern)

	// Initialize limits enforcer and spawn queue
	cfgPath := config.DefaultUserConfigPath()
	appCfg, _ := config.LoadUserConfig(cfgPath)
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	coreLimits := appCfg.GetCoreLimits()
	jitCfg := appCfg.GetEffectiveJITConfig()

	// Configure global LLM API concurrency
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = coreLimits.MaxConcurrentAPICalls
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

	// Register shard factories
	shardMgr.SetLLMClient(llmClient)
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
	resumeLLMAdapter := &campaignLLMAdapter{client: llmClient}

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

	taskExecutor := session.NewJITExecutor(sessionExecutor, sessionSpawner, transducer)
	virtualStore.SetTaskExecutor(taskExecutor)

	orchestrator := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:    cwd,
		Kernel:       kern,
		LLMClient:    llmClient,
		ShardManager: shardMgr,
		TaskExecutor: taskExecutor,
		Executor:     executor,
		VirtualStore: virtualStore,
		ProgressChan: progressChan,
		EventChan:    eventChan,
	})

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

// campaignLLMAdapter adapts perception.LLMClient to types.LLMClient.
type campaignLLMAdapter struct {
	client perception.LLMClient
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
