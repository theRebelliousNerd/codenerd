// Package system provides the core initialization and factory logic for the Cortex.
// It acts as the "Motherboard" that wires all components together.
package system

import (
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/mcp"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"codenerd/internal/types"
	"database/sql"
	"strings"

	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/usage"
	"codenerd/internal/world"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"sync"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for project corpus
)

// Global singleton Cortex instance to prevent repeated initialization (Bug #1 fix)
var (
	globalCortex     *Cortex
	globalCortexOnce sync.Once
	globalCortexErr  error
)

// GetOrBootCortex returns the global Cortex singleton, initializing it once if needed.
// This prevents the massive initialization spam (2,141 reinitializations) that was
// occurring when every command created its own Cortex instance.
//
// IMPORTANT: This function should be used instead of BootCortex() in all command handlers.
func GetOrBootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	globalCortexOnce.Do(func() {
		globalCortex, globalCortexErr = BootCortex(ctx, workspace, apiKey, disableSystemShards)
	})
	return globalCortex, globalCortexErr
}

// ResetGlobalCortex resets the global Cortex singleton. This is primarily for testing.
// WARNING: This should NOT be used in production code as it can cause inconsistent state.
func ResetGlobalCortex() {
	globalCortex = nil
	globalCortexErr = nil
	globalCortexOnce = sync.Once{}
}

// Cortex represents a fully initialized system instance.
type Cortex struct {
	Kernel         core.Kernel
	LLMClient      perception.LLMClient
	ShardManager   *coreshards.ShardManager
	VirtualStore   *core.VirtualStore
	Transducer     *perception.RealTransducer
	Orchestrator   *autopoiesis.Orchestrator
	BrowserManager *browser.SessionManager
	Scanner        *world.Scanner
	UsageTracker   *usage.Tracker
	LocalDB        *store.LocalStore
	Workspace      string
	JITCompiler    *prompt.JITPromptCompiler
}

// BootCortex initializes the entire system stack for a given workspace.
// This ensures consistent wiring across CLI, TUI, and Workers.
func BootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	if workspace == "" {
		if root, err := config.FindWorkspaceRoot(); err == nil && root != "" {
			workspace = root
		} else {
			workspace, _ = os.Getwd()
		}
	}
	if perception.SharedTaxonomy != nil {
		perception.SharedTaxonomy.SetWorkspace(workspace)
	}

	// 0. Initialize Logging System (critical for debugging)
	if err := logging.Initialize(workspace); err != nil {
		// Non-fatal - continue without file logging
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
	}

	// 1. Initialize Usage Tracker
	tracker, err := usage.NewTracker(workspace)
	if err != nil {
		// Non-fatal, but worth logging
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize usage tracker: %v\n", err)
	}

	// 2. Load user config for limits and provider selection
	userCfgPath := filepath.Join(workspace, ".nerd", "config.json")
	appCfg, _ := config.LoadUserConfig(userCfgPath)
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	coreLimits := appCfg.GetCoreLimits()
	jitCfg := appCfg.GetEffectiveJITConfig()

	// Configure global LLM API concurrency before any scheduled calls
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = coreLimits.MaxConcurrentAPICalls
	schedulerCfg.SlotAcquireTimeout = config.GetLLMTimeouts().SlotAcquisitionTimeout
	core.ConfigureGlobalAPIScheduler(schedulerCfg)

	// 3. Initialize LLM client using workspace config/env detection
	var baseLLMClient perception.LLMClient
	if providerCfg, err := perception.LoadConfigJSON(userCfgPath); err == nil {
		if client, err2 := perception.NewClientFromConfig(providerCfg); err2 == nil {
			baseLLMClient = client
		}
	}
	if baseLLMClient == nil {
		if client, err := perception.NewClientFromEnv(); err == nil {
			baseLLMClient = client
		}
	}
	if baseLLMClient == nil {
		// Final fallback: explicit ZAI key (legacy flag/env)
		key := apiKey
		if key == "" {
			key = os.Getenv("ZAI_API_KEY")
		}
		baseLLMClient = perception.NewZAIClient(key)
	}

	// Tracing Layer (if local DB available)
	var rawLLMClient perception.LLMClient = baseLLMClient
	localDBPath := filepath.Join(workspace, ".nerd", "knowledge.db")
	var localDB *store.LocalStore
	if db, err := store.NewLocalStore(localDBPath); err == nil {
		localDB = db
		// Wrap with tracing
		traceStore := createTraceStoreAdapter(db)
		rawLLMClient = perception.NewTracingLLMClient(baseLLMClient, traceStore)
	}

	// llmClient is used by non-shard components; wrap with scheduler to honor API concurrency.
	var llmClient perception.LLMClient = core.NewScheduledLLMCall("main", rawLLMClient)
	if perception.SharedTaxonomy != nil {
		perception.SharedTaxonomy.SetClient(llmClient)
		if localDB != nil {
			taxStore := perception.NewTaxonomyStore(localDB)
			perception.SharedTaxonomy.SetStore(taxStore)
			if err := perception.SharedTaxonomy.EnsureDefaults(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Taxonomy defaults init failed: %v\n", err)
			}
			if err := perception.SharedTaxonomy.HydrateFromDB(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Taxonomy rehydration failed: %v\n", err)
			}
		}
	}

	// Learning Layer - Autopoiesis persistence (§8.3)
	learningStorePath := filepath.Join(workspace, ".nerd", "shards")
	var learningStore *store.LearningStore
	if ls, err := store.NewLearningStore(learningStorePath); err == nil {
		learningStore = ls
	} else {
		// Non-fatal, but worth logging
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize learning store: %v\n", err)
	}

	transducer := perception.NewRealTransducer(llmClient)
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel: %w", err)
	}
	// Force initial evaluation to boot the Mangle engine (even with 0 facts)
	// This is CRITICAL to prevent "kernel not initialized" errors when shards query it early.
	if err := kernel.Evaluate(); err != nil {
		return nil, fmt.Errorf("failed to boot kernel: %w", err)
	}
	// Ensure Perception layer subsystems (semantic classifier, etc.) are initialized.
	if err := perception.InitPerceptionLayer(kernel, appCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Perception init failed: %v\n", err)
	}

	// Load persisted world facts if available.
	// Prefer LocalStore world cache (fast depth) and fall back to scan.mg.
	loadedWorld := false
	if localDB != nil {
		if cached, err := localDB.LoadAllWorldFacts("fast"); err == nil && len(cached) > 0 {
			facts := make([]core.Fact, 0, len(cached))
			for _, cf := range cached {
				facts = append(facts, core.Fact{Predicate: cf.Predicate, Args: cf.Args})
			}
			if err := kernel.LoadFacts(facts); err == nil {
				loadedWorld = true
			}
		}
	}
	if !loadedWorld {
		scanPath := filepath.Join(workspace, ".nerd", "mangle", "scan.mg")
		if _, statErr := os.Stat(scanPath); statErr == nil {
			if loadErr := kernel.LoadFactsFromFile(scanPath); loadErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to load scan facts: %v\n", loadErr)
			}
		}
	}

	executor := tactile.NewDirectExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = workspace
	virtualStore := core.NewVirtualStoreWithConfig(executor, vsCfg)
	virtualStore.SetKernel(kernel)
	virtualStore.DisableBootGuard() // BootCortex is always user-initiated (CLI commands)
	if localDB != nil {
		virtualStore.SetLocalDB(localDB)
	}
	if learningStore != nil {
		virtualStore.SetLearningStore(learningStore)
	}

	// Wire Code DOM (CodeScope + FileEditor) for semantic editing workflows.
	worldCfg := appCfg.GetWorldConfig()
	virtualStore.SetCodeScope(NewHolographicCodeScope(workspace, kernel, localDB, worldCfg.DeepWorkers))
	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(workspace)
	virtualStore.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	shardManager := coreshards.NewShardManager()
	shardManager.SetParentKernel(kernel)
	shardManager.SetLLMClient(rawLLMClient)

	// Limits enforcement and spawn queue backpressure (config-driven)
	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:      coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:   coreLimits.MaxConcurrentShards,
		MaxSessionDurationMin: coreLimits.MaxSessionDurationMin,
		MaxFactsInKernel:      coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit:  coreLimits.MaxDerivedFactsLimit,
	})
	shardManager.SetLimitsEnforcer(limitsEnforcer)

	spawnQueue := coreshards.NewSpawnQueue(shardManager, limitsEnforcer, coreshards.DefaultSpawnQueueConfig())
	shardManager.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	// 3. Autopoiesis & Tools
	autopoiesisConfig := autopoiesis.DefaultConfig(workspace)
	poiesis := autopoiesis.NewOrchestrator(llmClient, autopoiesisConfig)
	bridge := core.NewAutopoiesisBridge(kernel)
	poiesis.SetKernel(bridge)

	// Wire Ouroboros as ToolGenerator for coder shard self-tool routing
	if ouroborosLoop := poiesis.GetOuroborosLoop(); ouroborosLoop != nil {
		virtualStore.SetToolGenerator(ouroborosLoop)
	}

	// 4. Browser Physics
	browserCfg := browser.DefaultConfig()
	browserCfg.SessionStore = filepath.Join(workspace, ".nerd", "browser", "sessions.json")
	var browserMgr *browser.SessionManager
	// We need a Mangle engine for the browser manager
	if engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil); err == nil {
		browserMgr = browser.NewSessionManager(browserCfg, engine)
		// Browser will be started lazily when needed
	}

	// 5. JIT Prompt Compiler & Distributed Storage
	// Initialize Embedding Engine (required for AtomLoader)
	var embeddingEngine embedding.EmbeddingEngine
	embedCfg := appCfg.GetEmbeddingConfig()
	engineCfg := embedding.Config{
		Provider:       embedCfg.Provider,
		OllamaEndpoint: embedCfg.OllamaEndpoint,
		OllamaModel:    embedCfg.OllamaModel,
		GenAIAPIKey:    embedCfg.GenAIAPIKey,
		GenAIModel:     embedCfg.GenAIModel,
		TaskType:       embedCfg.TaskType,
	}
	// Back-compat: if provider is genai and no key set in config, fall back to CLI key.
	if engineCfg.Provider == "genai" && engineCfg.GenAIAPIKey == "" && apiKey != "" {
		engineCfg.GenAIAPIKey = apiKey
	}
	// If provider omitted, use embedding defaults (ollama keyword/vec).
	if engineCfg.Provider == "" {
		engineCfg = embedding.DefaultConfig()
	}
	if engine, err := embedding.NewEngine(engineCfg); err == nil {
		embeddingEngine = engine
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to init embedding engine: %v (semantic features degrade)\n", err)
	}

	// Initialize Atom Loader
	atomLoader := prompt.NewAtomLoader(embeddingEngine)

	// Enable semantic vector operations on the LocalStore if possible.
	if localDB != nil && embeddingEngine != nil {
		localDB.SetEmbeddingEngine(embeddingEngine)
	}

	// 5a. MCP Integration (JIT Tool Compiler)
	// Wire MCP clients dynamically - supports arbitrary servers from config.
	integrationsCfg := appCfg.GetIntegrations()
	serverConfigs := integrationsCfg.ToMCPServerConfigs()
	if len(serverConfigs) > 0 {
		// Create LLM client adapter for tool analysis
		var mcpLLMClient mcp.LLMClient
		if llmClient != nil {
			mcpLLMClient = &perceptionLLMAdapter{client: llmClient}
		}

		mcpBridge, err := mcp.NewMCPIntegrationBridge(workspace, newMCPKernelAdapter(kernel), embeddingEngine, mcpLLMClient, serverConfigs)
		if err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to init MCP bridge: %v", err)
		} else {
			// Wire ALL configured MCP servers dynamically
			for serverID := range serverConfigs {
				virtualStore.SetMCPClient(serverID, mcpBridge.GetAdapter(serverID))
				logging.Get(logging.CategoryTools).Info("Wired MCP integration: %s", serverID)
			}

			// Connect to auto-connect servers in background
			go func() {
				if err := mcpBridge.ConnectAll(context.Background()); err != nil {
					logging.Get(logging.CategoryTools).Warn("MCP auto-connect failed: %v", err)
				}
			}()
		}
	}

	// Ingest any PROMPT directives extracted from hybrid .mg files into the
	// project prompt corpus so JIT can pick them up.
	ingestHybridPrompts(ctx, workspace, kernel, atomLoader)

	// Sync Agents (Distributed Storage)
	// This ensures .nerd/shards/*.db are up-to-date with .nerd/agents/*.yaml
	synchronizer := prsync.NewAgentSynchronizer(workspace, atomLoader)
	if err := synchronizer.SyncAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Agent sync failed: %v\n", err)
	}

	// Initialize JIT Prompt Compiler
	// Load embedded corpus from internal/prompt/atoms/ (baked into binary)
	embeddedCorpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded corpus: %w", err)
	}

	// Build compiler options
	compilerCfg := prompt.DefaultCompilerConfig()
	if jitCfg.TokenBudget > 0 {
		compilerCfg.DefaultTokenBudget = jitCfg.TokenBudget
	}
	compilerOpts := []prompt.CompilerOption{
		prompt.WithKernel(NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embeddedCorpus),
		prompt.WithConfig(compilerCfg),
	}

	// Wire default vector searcher for semantic flesh selection when embeddings are available.
	var defaultVectorSearcher *prompt.CompilerVectorSearcher
	if embeddingEngine != nil {
		defaultVectorSearcher = prompt.NewCompilerVectorSearcher(embeddingEngine)
		compilerOpts = append(compilerOpts, prompt.WithVectorSearcher(defaultVectorSearcher))
	}

	// Load project corpus.db if it exists (user-defined atoms)
	corpusPath := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
	if wrote, err := prompt.MaterializeDefaultPromptCorpus(corpusPath); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to materialize default prompt corpus: %v", err)
	} else if wrote {
		logging.Get(logging.CategoryContext).Info("Materialized default prompt corpus to %s", corpusPath)
	}
	if _, statErr := os.Stat(corpusPath); statErr == nil {
		projectDB, dbErr := sql.Open("sqlite3", corpusPath)
		if dbErr == nil {
			// Ensure schema/migrations are applied (safe/idempotent).
			if err := atomLoader.EnsureSchema(ctx, projectDB); err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to ensure project corpus schema: %v", err)
				_ = projectDB.Close()
			} else {
				// Backfill normalized tags from embedded atoms when missing.
				if embeddedCorpus != nil {
					if err := prompt.HydrateAtomContextTags(ctx, projectDB, embeddedCorpus.All()); err != nil {
						logging.Get(logging.CategoryContext).Warn("Failed to hydrate project corpus tags: %v", err)
					}
				}

				compilerOpts = append(compilerOpts, prompt.WithProjectDB(projectDB))
				logging.Get(logging.CategoryContext).Info("Registered project corpus: %s", corpusPath)
			}
		} else {
			logging.Get(logging.CategoryContext).Warn("Failed to open project corpus: %v", dbErr)
		}
	}

	jitCompiler, err := prompt.NewJITPromptCompiler(compilerOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to init JIT compiler: %w", err)
	}
	if defaultVectorSearcher != nil {
		defaultVectorSearcher.SetCompiler(jitCompiler)
	}
	var promptAssembler *articulation.PromptAssembler
	if pa, err := articulation.NewPromptAssembler(kernel); err == nil {
		pa.SetJITCompiler(jitCompiler)
		pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK)
		pa.EnableJIT(jitCfg.Enabled)
		promptAssembler = pa
		transducer.SetPromptAssembler(pa)
	}

	// 5b. Register discovered user agents with JIT compiler and ShardManager
	// This wires up agents from .nerd/agents/{name}/prompts.yaml
	discoveredAgents := synchronizer.GetDiscoveredAgents()
	if len(discoveredAgents) > 0 {
		agentsOnDisk := make([]AgentOnDisk, 0, len(discoveredAgents))
		for _, a := range discoveredAgents {
			agentsOnDisk = append(agentsOnDisk, AgentOnDisk{ID: a.ID, DBPath: a.DBPath})
		}
		if _, err := SyncAgentRegistryFromDiscovered(workspace, agentsOnDisk); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to sync .nerd/agents.json from .nerd/agents: %v", err)
		}
	}
	for _, agent := range discoveredAgents {
		// Register agent DB with JIT compiler for dynamic prompt compilation
		if err := prompt.RegisterAgentDBWithJIT(jitCompiler, agent.ID, agent.DBPath); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to register agent %s with JIT: %v", agent.ID, err)
		} else {
			logging.Get(logging.CategoryContext).Info("Registered user agent '%s' with JIT compiler", agent.ID)
		}

		// Register agent profile with ShardManager as Type U (user-defined)
		cfg := coreshards.DefaultSpecialistConfig(agent.ID, agent.DBPath)
		cfg.Type = types.ShardTypeUser
		shardManager.DefineProfile(agent.ID, cfg)
	}
	if len(discoveredAgents) > 0 {
		logging.Get(logging.CategoryContext).Info("Registered %d user-defined agents", len(discoveredAgents))
	}

	// Register Shards (The Critical Fix)
	regCtx := shards.RegistryContext{
		Kernel:       kernel,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
		Workspace:    workspace,
		JITCompiler:  jitCompiler,
		JITConfig:    jitCfg,
	}
	shards.RegisterAllShardFactories(shardManager, regCtx)

	// Wire JIT Registrars for future dynamic registration
	shardManager.SetJITRegistrar(prompt.CreateJITDBRegistrar(jitCompiler))
	shardManager.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(jitCompiler))

	// Overwrite System Shards (Manual Injection if needed, but RegistryContext handles most)
	// However, TactileRouter needs BrowserManager which isn't in RegistryContext yet
	// So we manually re-register TactileRouter to inject BrowserManager
	shardManager.RegisterShard("tactile_router", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(kernel)
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		if browserMgr != nil {
			shard.SetBrowserManager(browserMgr)
		}
		if promptAssembler != nil {
			shard.SetPromptAssembler(promptAssembler)
		}
		return shard
	})

	// CampaignRunner needs access to the shared ShardManager; inject it here.
	shardManager.RegisterShard("campaign_runner", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewCampaignRunnerShard()
		shard.SetParentKernel(kernel)
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		shard.SetWorkspaceRoot(workspace)
		shard.SetShardManager(shardManager)
		if promptAssembler != nil {
			shard.SetPromptAssembler(promptAssembler)
		}
		return shard
	})

	// 6. Start System Shards
	disabledSet := make(map[string]struct{})
	for _, name := range disableSystemShards {
		disabledSet[name] = struct{}{}
	}
	if env := os.Getenv("NERD_DISABLE_SYSTEM_SHARDS"); env != "" {
		// Parse env var... simplistic split
		// (omitted for brevity, rely on caller to pass parsed list if possible)
	}

	for name := range disabledSet {
		shardManager.DisableSystemShard(name)
	}

	if err := shardManager.StartSystemShards(ctx); err != nil {
		return nil, fmt.Errorf("failed to start system shards: %w", err)
	}

	// 7. World Model Scanning
	scanner := world.NewScannerWithConfig(world.ScannerConfig{
		MaxConcurrency:  worldCfg.FastWorkers,
		IgnorePatterns:  worldCfg.IgnorePatterns,
		MaxASTFileBytes: worldCfg.MaxFastASTBytes,
	})

	return &Cortex{
		Kernel:         kernel,
		LLMClient:      llmClient,
		ShardManager:   shardManager,
		VirtualStore:   virtualStore,
		Transducer:     transducer,
		Orchestrator:   poiesis,
		BrowserManager: browserMgr,
		Scanner:        scanner,
		UsageTracker:   tracker,
		LocalDB:        localDB,
		Workspace:      workspace,
		JITCompiler:    jitCompiler,
	}, nil
}

// ingestHybridPrompts loads PROMPT directives extracted from hybrid .mg files
// into the project prompt corpus database (.nerd/prompts/corpus.db).
// This keeps hybrid files as a readable single source of truth while still
// routing prompt atoms into the JIT system.
func ingestHybridPrompts(ctx context.Context, workspace string, kernel *core.RealKernel, atomLoader *prompt.AtomLoader) {
	if kernel == nil || atomLoader == nil {
		return
	}

	hybridPrompts := kernel.ConsumeBootPrompts()
	if len(hybridPrompts) == 0 {
		return
	}

	corpusPath := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
	if wrote, err := prompt.MaterializeDefaultPromptCorpus(corpusPath); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to materialize default prompt corpus for hybrid ingest: %v", err)
	} else if wrote {
		logging.Get(logging.CategoryContext).Info("Materialized default prompt corpus to %s (hybrid ingest)", corpusPath)
	}
	if err := os.MkdirAll(filepath.Dir(corpusPath), 0755); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to create prompts dir for hybrid corpus: %v", err)
		return
	}

	db, err := sql.Open("sqlite3", corpusPath)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to open hybrid prompt corpus DB: %v", err)
		return
	}
	defer db.Close()

	if err := atomLoader.EnsureSchema(ctx, db); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to ensure hybrid prompt corpus schema: %v", err)
		return
	}

	stored := 0
	for _, hp := range hybridPrompts {
		cat, sub := mapHybridPromptCategory(hp.Category)
		atom := prompt.NewPromptAtom(hp.ID, cat, hp.Content)
		if sub != "" {
			atom.Subcategory = sub
		}
		if len(hp.Tags) > 1 {
			extras := strings.Join(hp.Tags[1:], ",")
			if atom.Subcategory != "" {
				atom.Subcategory = atom.Subcategory + "," + extras
			} else {
				atom.Subcategory = extras
			}
		}

		// Default priority; skeleton categories are always included by selector.
		if err := atomLoader.StoreAtom(ctx, db, atom); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to store hybrid prompt atom %s: %v", hp.ID, err)
			continue
		}
		stored++
	}

	logging.Get(logging.CategoryContext).Info("Ingested %d hybrid PROMPT atoms into %s", stored, corpusPath)
}

// mapHybridPromptCategory maps legacy hybrid category tags into JIT atom categories.
// Unknown categories default to /domain with subcategory preserved.
func mapHybridPromptCategory(raw string) (prompt.AtomCategory, string) {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if clean == "" {
		return prompt.CategoryDomain, ""
	}

	for _, cat := range prompt.AllCategories() {
		if string(cat) == clean {
			return cat, ""
		}
	}

	switch clean {
	case "role":
		return prompt.CategoryIdentity, "role"
	case "tool":
		return prompt.CategoryProtocol, "tool"
	case "safety":
		return prompt.CategorySafety, "safety"
	case "phase":
		return prompt.CategoryMethodology, "phase"
	}

	return prompt.CategoryDomain, clean
}

// LocalStoreTraceAdapter wraps LocalStore to implement perception.TraceStore.
// Duplicated from chat/session.go to avoid import cycle or dependency on `chat`.
type LocalStoreTraceAdapter struct {
	store *store.LocalStore
}

func createTraceStoreAdapter(s *store.LocalStore) *LocalStoreTraceAdapter {
	return &LocalStoreTraceAdapter{store: s}
}

func (a *LocalStoreTraceAdapter) StoreReasoningTrace(trace *perception.ReasoningTrace) error {
	// perception.TraceStore expects StoreReasoningTrace(*ReasoningTrace)
	// store.LocalStore.StoreReasoningTrace takes interface{}.
	return a.store.StoreReasoningTrace(trace)
}

func (a *LocalStoreTraceAdapter) LoadReasoningTrace(traceID string) (*perception.ReasoningTrace, error) {
	// Not implemented for now in this adapter context
	return nil, nil
}

// KernelAdapter adapts core.RealKernel to prompt.KernelQuerier.
// It handles type conversion between []interface{} and []core.Fact.
type KernelAdapter struct {
	kernel core.Kernel
}

// NewKernelAdapter creates a new KernelAdapter for the given kernel.
// This adapter bridges core.Kernel to prompt.KernelQuerier interface,
// enabling the JIT Prompt Compiler to query the Mangle kernel for
// skeleton atom selection.
func NewKernelAdapter(kernel core.Kernel) *KernelAdapter {
	return &KernelAdapter{kernel: kernel}
}

func (ka *KernelAdapter) Query(predicate string) ([]prompt.Fact, error) {
	facts, err := ka.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	// Convert []core.Fact to []prompt.Fact
	result := make([]prompt.Fact, len(facts))
	for i, f := range facts {
		result[i] = prompt.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

func (ka *KernelAdapter) AssertBatch(facts []interface{}) error {
	var coreFacts []core.Fact
	for _, f := range facts {
		switch v := f.(type) {
		case core.Fact:
			coreFacts = append(coreFacts, v)
		case string:
			// Parse string fact
			// Mangle parser expects full clause syntax, typically ending with dot
			input := v
			if !strings.HasSuffix(input, ".") {
				input += "."
			}

			parsed, err := parse.Unit(strings.NewReader(input))
			if err != nil {
				return fmt.Errorf("failed to parse fact string '%s': %w", v, err)
			}

			if len(parsed.Clauses) != 1 {
				return fmt.Errorf("expected 1 clause in fact string, got %d", len(parsed.Clauses))
			}

			atom := parsed.Clauses[0].Head
			args := make([]interface{}, len(atom.Args))
			for i, arg := range atom.Args {
				switch t := arg.(type) {

				case ast.Constant:
					// Handle different constant types based on the Type field
					switch t.Type {
					case ast.NameType:
						// Mangle name constants (start with /)
						args[i] = core.MangleAtom(t.Symbol)
					case ast.StringType:
						// String constants - Symbol contains the raw value (no quotes)
						args[i] = t.Symbol
					case ast.BytesType:
						// Byte string constants
						args[i] = t.Symbol
					case ast.NumberType:
						// Integer constants
						args[i] = t.NumValue
					case ast.Float64Type:
						// Float constants
						args[i] = t.Float64Value
					default:
						// DEFENSIVE: Unknown constant type - log and use Symbol as fallback
						logging.Get(logging.CategoryContext).Warn("AssertBatch: unknown constant type %v, using Symbol fallback", t.Type)
						args[i] = t.Symbol
					}
				default:
					// Fallback for non-constant types (e.g., variables)
					args[i] = fmt.Sprintf("%v", arg)
				}
			}

			coreFacts = append(coreFacts, core.Fact{
				Predicate: atom.Predicate.Symbol,
				Args:      args,
			})
		default:
			return fmt.Errorf("unsupported fact type: %T", f)
		}
	}
	return ka.kernel.LoadFacts(coreFacts)
}

// perceptionLLMAdapter adapts perception.LLMClient to mcp.LLMClient.
type perceptionLLMAdapter struct {
	client perception.LLMClient
}

func (a *perceptionLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	return a.client.Complete(ctx, prompt)
}

// mcpKernelAdapter adapts core.RealKernel to mcp.KernelInterface.
// It converts string facts to core.Fact and handles query results.
type mcpKernelAdapter struct {
	kernel *core.RealKernel
}

// newMCPKernelAdapter creates a new MCP kernel adapter.
func newMCPKernelAdapter(kernel *core.RealKernel) *mcpKernelAdapter {
	return &mcpKernelAdapter{kernel: kernel}
}

func (a *mcpKernelAdapter) Assert(fact string) error {
	// Parse string fact into core.Fact
	input := fact
	if !strings.HasSuffix(input, ".") {
		input += "."
	}

	parsed, err := parse.Unit(strings.NewReader(input))
	if err != nil {
		return fmt.Errorf("failed to parse fact '%s': %w", fact, err)
	}

	if len(parsed.Clauses) != 1 {
		return fmt.Errorf("expected 1 clause, got %d", len(parsed.Clauses))
	}

	atom := parsed.Clauses[0].Head
	args := make([]interface{}, len(atom.Args))
	for i, arg := range atom.Args {
		switch t := arg.(type) {
		case ast.Constant:
			switch t.Type {
			case ast.NameType:
				args[i] = core.MangleAtom(t.Symbol)
			case ast.StringType, ast.BytesType:
				args[i] = t.Symbol
			case ast.NumberType:
				args[i] = t.NumValue
			case ast.Float64Type:
				args[i] = t.Float64Value
			default:
				args[i] = t.Symbol
			}
		default:
			args[i] = fmt.Sprintf("%v", arg)
		}
	}

	return a.kernel.LoadFacts([]core.Fact{{
		Predicate: atom.Predicate.Symbol,
		Args:      args,
	}})
}

func (a *mcpKernelAdapter) Query(predicate string) ([]map[string]interface{}, error) {
	// 1. Parse the query pattern to identify variables
	queryFact, err := core.ParseFactString(predicate)
	if err != nil {
		// Provide a more helpful error if parsing fails
		return nil, fmt.Errorf("invalid query format '%s': %w", predicate, err)
	}

	// 2. Map variable names to argument indices
	variableMap := make(map[int]string)
	for i, arg := range queryFact.Args {
		if s, ok := arg.(string); ok && strings.HasPrefix(s, "?") {
			variableMap[i] = s[1:] // Trim "?" prefix
		}
	}

	// 3. Execute query to get raw facts
	facts, err := a.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	// 4. Transform facts into variable bindings maps
	results := make([]map[string]interface{}, 0, len(facts))
	for _, f := range facts {
		binding := make(map[string]interface{})

		// If query had variables, extract them
		if len(variableMap) > 0 {
			for idx, varName := range variableMap {
				if idx < len(f.Args) {
					binding[varName] = f.Args[idx]
				}
			}
		} else {
			// Fallback for 0-arity or const-only queries: return usage of predicate as a flag?
			// Mangle convention for boolean query is strict, but here we return empty map for match
		}

		results = append(results, binding)
	}
	return results, nil
}

func (a *mcpKernelAdapter) Retract(fact string) error {
	// Parse string fact into core.Fact
	input := fact
	if !strings.HasSuffix(input, ".") {
		input += "."
	}

	parsed, err := core.ParseFactString(input)
	if err != nil {
		return fmt.Errorf("failed to parse fact '%s': %w", fact, err)
	}

	return a.kernel.RetractExactFact(parsed)
}
