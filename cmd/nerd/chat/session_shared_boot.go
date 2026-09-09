package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/retrieval"
	"codenerd/internal/shards"
	shardsystem "codenerd/internal/shards/system"
	"codenerd/internal/store"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
	"codenerd/internal/ux"
	"codenerd/internal/verification"

	tea "github.com/charmbracelet/bubbletea"
)

func performSystemBoot(cfg *config.UserConfig, disableSystemShards []string, workspace string) tea.Cmd {
	return func() tea.Msg {
		return performSystemBootShared(cfg, disableSystemShards, workspace)
	}
}

func performSystemBootShared(cfg *config.UserConfig, disableSystemShards []string, workspace string) tea.Msg {
	bootStart := time.Now()
	if err := logging.Initialize(workspace); err != nil {
		fmt.Printf("[boot] Warning: logging init failed: %v\n", err)
	}
	bootLog := logging.Get(logging.CategoryBoot)

	logStep := func(step string) {
		elapsed := time.Since(bootStart).Seconds()
		fmt.Printf("\r\033[K[boot] %s (%.1fs)", step, elapsed)
		bootLog.Info("%s (%.1fs)", step, elapsed)
	}

	logStep("Loading config...")
	appCfg := cfg
	if appCfg == nil {
		appCfg, _ = config.GlobalConfig()
		if appCfg == nil {
			appCfg = config.DefaultUserConfig()
		}
	}

	prefsMgr := ux.NewPreferencesManager(workspace)
	if err := prefsMgr.Load(); err != nil {
		logging.Get(logging.CategoryBoot).Warn("Failed to load preferences: %v", err)
	}

	transparencyCfg := appCfg.GetTransparencyConfig()
	transparencyMgr := transparency.NewTransparencyManager(transparencyCfg)
	if transparencyCfg.Enabled {
		logStep("Transparency enabled")
	}

	logStep("Initializing sparse retriever...")
	retrieverCfg := retrieval.DefaultSparseRetrieverConfig(workspace)
	retriever := retrieval.NewSparseRetriever(retrieverCfg)

	logStep("Booting shared backend...")
	cortex, err := nerdsystem.BootCortexWithConfig(context.Background(), nerdsystem.BootConfig{
		Workspace:           workspace,
		DisableSystemShards: disableSystemShards,
		UserConfigOverride:  appCfg,
	})
	if err != nil {
		return bootCompleteMsg{err: fmt.Errorf("shared bootstrap failed: %w", err)}
	}

	kernel := cortex.RealKernel
	if kernel == nil {
		return bootCompleteMsg{err: fmt.Errorf("shared bootstrap did not return a real kernel")}
	}

	shardMgr := cortex.ShardManager
	taskExecutor := cortex.TaskExecutor
	virtualStore := cortex.VirtualStore
	llmClient := cortex.LLMClient
	transducer := cortex.Transducer
	localDB := cortex.LocalDB
	learningStore := cortex.LearningStore
	embeddingEngine := cortex.EmbeddingEngine
	jitCompiler := cortex.JITCompiler
	promptAssembler := cortex.PromptAssembler
	browserMgr := cortex.BrowserManager
	autopoiesisOrch := cortex.Orchestrator
	scanner := cortex.Scanner
	executor := cortex.Executor
	sessionExecutor := cortex.SessionExecutor
	sessionSpawner := cortex.SessionSpawner

	if shardMgr != nil {
		shardMgr.SetTransparencyManager(transparencyMgr)
		if learningStore != nil {
			adapter := &coreLearningStoreAdapter{store: learningStore}
			shardMgr.SetLearningStore(adapter)
		}
	}

	initialMessages := []Message{
		{
			Role:    "assistant",
			Content: "✓ Shared bootstrap initialized",
			Time:    time.Now(),
		},
	}

	if sessionExecutor != nil && virtualStore != nil {
		sessionExecutor.SetOuroborosRegistry(virtualStore.GetToolRegistry())
	}

	shadowMode := core.NewShadowMode(kernel)

	logStep("Initializing context compressor...")
	ctxCfg := appCfg.GetContextWindowConfig()
	compressor := ctxcompress.NewCompressorWithParams(
		kernel, localDB, llmClient,
		ctxCfg.MaxTokens,
		ctxCfg.CoreReservePercent, ctxCfg.AtomReservePercent,
		ctxCfg.HistoryReservePercent, ctxCfg.WorkingReservePercent,
		ctxCfg.RecentTurnWindow,
		ctxCfg.CompressionThreshold, ctxCfg.TargetCompressionRatio, ctxCfg.ActivationThreshold,
	)
	if corpus := kernel.GetPredicateCorpus(); corpus != nil {
		if err := compressor.LoadPrioritiesFromCorpus(corpus); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to load corpus priorities: %v", err)
		}
	}

	// The context feedback store is opened by the Cortex boot, which now feeds
	// it from every path rather than only from this one. Opening a second
	// handle here would put two writers on one SQLite file.
	feedbackStore := cortex.ContextFeedback
	if feedbackStore == nil {
		logging.Get(logging.CategoryContext).Warn(
			"Context feedback store unavailable; spreading activation will not use learned predicate usefulness")
	} else {
		compressor.SetFeedbackStore(feedbackStore)
	}

	logStep("Initializing task verifier...")
	taskVerifier := verification.NewTaskVerifier(
		llmClient,
		localDB,
		shardMgr,
		autopoiesisOrch,
	)
	taskVerifier.SetTaskExecutor(taskExecutor)

	glassBoxEventBus := transparency.NewGlassBoxEventBus()
	glassBoxEventBus.Enable()
	toolEventBus := transparency.NewToolEventBus()

	if virtualStore != nil {
		virtualStore.SetGlassBoxBus(glassBoxEventBus)
		virtualStore.SetToolEventBus(toolEventBus)
	}

	if shardMgr != nil {
		shardMgr.SetGlassBoxBus(glassBoxEventBus)
		// Re-register tactile_router so future on-demand starts inherit the chat-specific
		// debug and persistence integrations while core backend ownership stays in factory.
		shardMgr.RegisterShard("tactile_router", func(id string, _ types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewTactileRouterShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			shard.SetGlassBox(glassBoxEventBus)
			shard.SetToolEventBus(toolEventBus)
			shard.SetToolStore(cortex.ToolStore)
			if browserMgr != nil {
				shard.SetBrowserManager(browserMgr)
			}
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})

		for _, agent := range shardMgr.GetActiveShards() {
			if setter, ok := agent.(interface {
				SetGlassBox(*transparency.GlassBoxEventBus)
			}); ok {
				setter.SetGlassBox(glassBoxEventBus)
			}
			if setter, ok := agent.(interface {
				SetToolEventBus(*transparency.ToolEventBus)
			}); ok {
				setter.SetToolEventBus(toolEventBus)
			}
			if setter, ok := agent.(interface{ SetToolStore(*store.ToolStore) }); ok {
				setter.SetToolStore(cortex.ToolStore)
			}
		}

		// Register a post-spawn hook so on-demand shards created after boot
		// automatically get the same chat-specific dependencies.
		shardMgr.SetPostSpawnHook(func(agent types.ShardAgent) {
			if setter, ok := agent.(interface {
				SetGlassBox(*transparency.GlassBoxEventBus)
			}); ok {
				setter.SetGlassBox(glassBoxEventBus)
			}
			if setter, ok := agent.(interface {
				SetToolEventBus(*transparency.ToolEventBus)
			}); ok {
				setter.SetToolEventBus(toolEventBus)
			}
			if setter, ok := agent.(interface{ SetToolStore(*store.ToolStore) }); ok {
				setter.SetToolStore(cortex.ToolStore)
			}
		})
	}

	nerdDir := filepath.Join(workspace, ".nerd")

	// Prompt evolution is built by the Cortex boot (internal/system's
	// initLearningLoop), not here. It used to be assembled in this file, which
	// is why only the chat TUI ever learned anything: every headless path
	// boots the same Cortex and got no evolver at all. Constructing a second
	// one here would now mean two writers on the same .nerd/ SQLite files.
	promptEvolver := cortex.PromptEvolver
	if promptEvolver == nil {
		logging.Get(logging.CategoryBoot).Warn(
			"Prompt Evolution unavailable; /evolve and prompt learning are disabled this session")
	}

	logStep("Hydrating session state...")
	loadedSession, _ := hydrateNerdState(workspace, kernel, shardMgr, &initialMessages)
	if shardMgr != nil {
		shardMgr.SetSessionID(resolveSessionID(loadedSession))
	}

	logStep("Starting Mangle watcher...")
	var mangleWatcher *core.MangleWatcher
	if mw, err := core.NewMangleWatcher(workspace, kernel); err == nil {
		mangleWatcher = mw
		if err := mangleWatcher.Start(context.Background()); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to start Mangle watcher: %v", err)
		}
	} else {
		logging.Get(logging.CategoryKernel).Warn("Failed to create Mangle watcher: %v", err)
	}

	logStep("Setting up observers...")
	observerMgr := shards.NewBackgroundObserverManager(&taskExecutorObserverSpawner{taskExecutor})
	if err := observerMgr.RegisterObserver("northstar"); err == nil {
		// Shared with /alignment and the campaign risk gate; see registry.go.
		if guardian, err := northstar.AcquireGuardian(nerdDir, northstar.DefaultGuardianConfig()); err == nil {
			guardian.SetLLMClient(llmClient)
			if kernel != nil {
				guardian.SetParentKernel(kernel)
			}
			if err := guardian.Initialize(); err == nil {
				handler := northstar.NewBackgroundEventHandler(guardian, resolveSessionID(loadedSession))
				observerMgr.SetNorthstarHandler(&northstarHandlerAdapter{handler})
			}
		}
	}

	logStep("Setting up consultation protocol...")
	consultationMgr := shards.NewConsultationManager(&taskExecutorConsultationSpawner{taskExecutor})

	fmt.Printf("\r\033[K[boot] Complete! (%.1fs)\n", time.Since(bootStart).Seconds())
	return bootCompleteMsg{
		components: &SystemComponents{
			Kernel:           kernel,
			ShardMgr:         shardMgr,
			TaskExecutor:     taskExecutor,
			ShadowMode:       shadowMode,
			Transducer:       transducer,
			Executor:         executor,
			Emitter:          nil,
			VirtualStore:     virtualStore,
			Scanner:          scanner,
			Workspace:        workspace,
			SessionID:        resolveSessionID(loadedSession),
			TurnCount:        resolveTurnCount(loadedSession),
			LocalDB:          localDB,
			Compressor:       compressor,
			FeedbackStore:    feedbackStore,
			Cortex:           cortex,
			Autopoiesis:      autopoiesisOrch,
			Verifier:         taskVerifier,
			InitialMessages:  initialMessages,
			Client:           llmClient,
			BrowserManager:   browserMgr,
			BrowserCtxCancel: nil,
			JITCompiler:      jitCompiler,
			MangleWatcher:    mangleWatcher,
			TransparencyMgr:  transparencyMgr,
			PreferencesMgr:   prefsMgr,
			Retriever:        retriever,
			GlassBoxEventBus: glassBoxEventBus,
			ToolEventBus:     toolEventBus,
			ToolStore:        cortex.ToolStore,
			PromptEvolver:    promptEvolver,
			EmbeddingEngine:  embeddingEngine,
			LearningStore:    learningStore,
			SessionExecutor:  sessionExecutor,
			SessionSpawner:   sessionSpawner,
			ObserverMgr:      observerMgr,
			ConsultationMgr:  consultationMgr,
			DreamToolQ:       cortex.OuroborosQueue,
		},
	}
}
