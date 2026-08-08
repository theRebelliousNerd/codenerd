package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	prompt_evolution "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/browser"
	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/features"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/retrieval"
	"codenerd/internal/session"
	"codenerd/internal/shards"

	// Domain shards removed - JIT clean loop handles these via prompt atoms:
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"
	shardsystem "codenerd/internal/shards/system"
	"codenerd/internal/sqlpragmas"
	"codenerd/internal/store"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
	"codenerd/internal/ux"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/mattn/go-sqlite3"
)

// performSystemBootLegacy contains the pre-cutover chat-local backend assembly path.
// New callers should go through performSystemBoot in session_shared_boot.go.
func performSystemBootLegacy(cfg *config.UserConfig, disableSystemShards []string, workspace string) tea.Cmd {
	return func() tea.Msg {
		bootStart := time.Now()

		// Initialize categorized logging system
		if err := logging.Initialize(workspace); err != nil {
			fmt.Printf("[boot] Warning: logging init failed: %v\n", err)
		}
		bootLog := logging.Get(logging.CategoryBoot)

		// Local log function for TUI status line + file logging
		logStep := func(step string) {
			elapsed := time.Since(bootStart).Seconds()
			fmt.Printf("\r\033[K[boot] %s (%.1fs)", step, elapsed)
			bootLog.Info("%s (%.1fs)", step, elapsed)
		}

		logStep("Loading config...")
		// Use the passed-in config or reload from disk
		appCfg := cfg
		if appCfg == nil {
			appCfg, _ = config.GlobalConfig()
			if appCfg == nil {
				appCfg = config.DefaultUserConfig()
			}
		}

		// Resolve core limits once for boot wiring
		coreLimits := appCfg.GetCoreLimits()

		// Initialize Preferences Manager (Backend)
		prefsMgr := ux.NewPreferencesManager(workspace)
		if err := prefsMgr.Load(); err != nil {
			logging.Get(logging.CategoryBoot).Warn("Failed to load preferences: %v", err)
		}

		// Initialize Transparency Manager
		transparencyCfg := appCfg.GetTransparencyConfig()
		transparencyMgr := transparency.NewTransparencyManager(transparencyCfg)
		if transparencyCfg.Enabled {
			logStep("Transparency enabled")
		}

		// Initialize Sparse Retriever
		logStep("Initializing sparse retriever...")
		retrieverCfg := retrieval.DefaultSparseRetrieverConfig(workspace)
		retriever := retrieval.NewSparseRetriever(retrieverCfg)

		// Configure global LLM API concurrency before any scheduled calls
		schedulerCfg := core.DefaultAPISchedulerConfig()
		schedulerCfg.MaxConcurrentAPICalls = appCfg.GetEffectiveMaxConcurrentAPICalls()
		schedulerCfg.SlotAcquireTimeout = config.GetLLMTimeouts().SlotAcquisitionTimeout
		core.ConfigureGlobalAPIScheduler(schedulerCfg)
		initialMessages := []Message{}

		// Initialize LLM client using the perception package's provider detection
		// This supports all providers: zai, anthropic, openai, gemini, xai, openrouter
		// Configuration is read from .nerd/config.json or environment variables
		logStep("Detecting LLM provider...")
		var baseLLMClient perception.LLMClient
		baseLLMClient, initialMessages = detectLLMProvider(initialMessages)

		// HEAVY OPERATION: NewRealKernel calls Evaluate() internally
		logStep("Booting Mangle kernel...")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return fmt.Errorf("failed to create kernel: %w", err)
		}

		// NewRealKernel now properly returns errors instead of panicking.
		// The kernel is already evaluated during construction.
		// Auto-enable provenance recording when the user has set
		// features.provenance=true (or CODENERD_PROVENANCE=1). This lets
		// /explain return precise proof trees from session start without
		// the first call paying for a forced re-eval. The reset is
		// observed by the next Evaluate() below.
		if features.IsProvenanceEnabled() {
			kernel.EnableProvenance()
			logStep("Provenance recording enabled via features flag")
		}
		logStep("Evaluating kernel rules...")
		if err := kernel.Evaluate(); err != nil {
			return bootCompleteMsg{err: fmt.Errorf("kernel boot failed: %w", err)}
		}

		logStep("Creating executor & shard manager...")
		executor := tactile.NewDirectExecutor()
		shardMgr := coreshards.NewShardManager()
		shardMgr.SetParentKernel(kernel)
		shardMgr.SetTransparencyManager(transparencyMgr)

		// Initialize limits enforcer and spawn queue for backpressure management
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
		if err := spawnQueue.Start(); err != nil {
			logging.Get(logging.CategoryBoot).Warn("Failed to start spawn queue: %v", err)
		}

		// TaskExecutor will be initialized later with JITExecutor
		// after the JIT components are created (sessionExecutor, sessionSpawner)
		var taskExecutor session.TaskExecutor

		// Browser Manager is created on-demand when needed (not at boot)
		// This avoids spawning Chrome during normal TUI usage
		var browserMgr *browser.SessionManager // nil until needed
		var browserCtxCancel context.CancelFunc

		logStep("Creating virtual store...")
		vsCfg := core.DefaultVirtualStoreConfig()
		vsCfg.WorkingDir = workspace
		virtualStore := core.NewVirtualStoreWithConfig(executor, vsCfg)
		virtualStore.SetKernel(kernel)
		// Note: SetTaskExecutor is called later after JITExecutor is created

		logStep("Opening knowledge database...")
		var localDB *store.LocalStore
		knowledgeDBPath := filepath.Join(workspace, ".nerd", "knowledge.db")
		if db, err := store.NewLocalStore(knowledgeDBPath); err == nil {
			localDB = db
		}

		logStep("Wiring Code DOM...")
		worldCfg := appCfg.GetWorldConfig()
		virtualStore.SetCodeScope(nerdsystem.NewHolographicCodeScope(workspace, kernel, localDB, worldCfg.DeepWorkers))
		fileEditor := tactile.NewFileEditor()
		fileEditor.SetWorkingDir(workspace)
		virtualStore.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

		// Initialize embedding engine — ALWAYS from .nerd/config.json
		// (appCfg.GetEmbeddingConfig). Do not hardcode model names here.
		logStep("Initializing embedding engine from config.json...")
		var embeddingEngine embedding.EmbeddingEngine
		embeddingEngine, initialMessages = initEmbeddingEngine(appCfg, initialMessages, localDB)
		if localDB != nil {
			localDB.SetReflectionConfig(appCfg.GetReflectionConfig())
		}

		// Ensure .nerd paths resolve to the active workspace for learned rules,
		// even if SQLite is unavailable (file-based persistence still uses .nerd/mangle).
		if perception.SharedTaxonomy != nil {
			perception.SharedTaxonomy.SetWorkspace(workspace)
		}

		// Initialize learning store for shard autopoiesis
		logStep("Initializing learning store...")
		var learningStore *store.LearningStore
		learningsPath := filepath.Join(workspace, ".nerd", "shards")
		if ls, err := store.NewLearningStore(learningsPath); err == nil {
			learningStore = ls
			if embeddingEngine != nil {
				learningStore.SetEmbeddingEngine(embeddingEngine)
			}
			learningStore.SetReflectionConfig(appCfg.GetReflectionConfig())
			virtualStore.SetLearningStore(learningStore)

			// GAP-008 FIX: Apply periodic confidence decay on session startup
			// Decay learnings older than 30 days by 10% to allow forgetting
			for _, shardType := range []string{"coder", "tester", "reviewer", "researcher"} {
				if err := ls.DecayConfidence(shardType, 0.9); err != nil {
					logging.Get(logging.CategoryStore).Debug("DecayConfidence for %s: %v", shardType, err)
				}
			}
		}

		if localDB != nil {
			logStep("Wiring virtual store...")
			virtualStore.SetLocalDB(localDB)
			// Wire knowledge graph query bridge for Mangle query_graph virtual predicate.
			if gqAdapter := store.NewLocalStoreGraphAdapter(localDB); gqAdapter != nil {
				virtualStore.SetGraphQuery(gqAdapter)
			}

			logStep("Initializing taxonomy store...")
			taxStore := perception.NewTaxonomyStore(localDB)
			if perception.SharedTaxonomy != nil {
				perception.SharedTaxonomy.SetStore(taxStore)

				if err := perception.SharedTaxonomy.EnsureDefaults(); err != nil {
					initialMessages = append(initialMessages, Message{
						Role:    "assistant",
						Content: fmt.Sprintf("⚠ Taxonomy defaults init failed: %v", err),
						Time:    time.Now(),
					})
				}

				// HEAVY OPERATION: Rehydration
				logStep("Hydrating taxonomy from DB...")
				if err := perception.SharedTaxonomy.HydrateFromDB(); err != nil {
					initialMessages = append(initialMessages, Message{
						Role:    "assistant",
						Content: fmt.Sprintf("⚠ Taxonomy rehydration failed: %v", err),
						Time:    time.Now(),
					})
				}

			}

			logStep("Migrating old sessions...")
			if migratedTurns, err := MigrateOldSessionsToSQLite(workspace, localDB); err == nil && migratedTurns > 0 {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Migrated %d session turns to SQLite", migratedTurns),
					Time:    time.Now(),
				})
			}
		}

		logStep("Configuring LLM client...")
		var rawLLMClient perception.LLMClient = baseLLMClient
		if localDB != nil {
			traceStore := NewLocalStoreTraceAdapter(localDB)
			tracingClient := perception.NewTracingLLMClient(baseLLMClient, traceStore)
			rawLLMClient = tracingClient
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: "✓ Reasoning trace capture enabled",
				Time:    time.Now(),
			})
		}

		// llmClient is used by the main TUI interactive agent (HIGH priority).
		// shardLLMClient is used for spawn/create/shards — optional worker LLM
		// (local Ollama gemma) so testing stays cheap while main stays on Grok.
		var llmClient perception.LLMClient = core.NewScheduledLLMCallWithPriority("main", rawLLMClient, types.PriorityHigh)

		shardRaw := rawLLMClient
		if worker, werr := perception.NewWorkerClientFromUserConfig(appCfg); werr != nil {
			logging.Boot("Worker LLM init failed: %v (shards share main client)", werr)
		} else if worker != nil {
			if localDB != nil {
				traceStore := NewLocalStoreTraceAdapter(localDB)
				shardRaw = perception.NewTracingLLMClient(worker, traceStore)
			} else {
				shardRaw = worker
			}
			if w := appCfg.GetWorkerLLMConfig(); w != nil {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Worker LLM for shards: %s (model: %s)", w.Provider, w.Model),
					Time:    time.Now(),
				})
			}
		}
		var shardLLMClient perception.LLMClient = core.NewScheduledLLMCall("chat_shards", shardRaw)
		shardMgr.SetLLMClient(shardLLMClient)
		// Image generator: Gemini Nano Banana 2 — excluded from Ollama worker.
		if imgClient, ierr := perception.NewImageClientFromUserConfig(appCfg); ierr != nil {
			logging.Boot("Image LLM (Nano Banana 2) unavailable: %v", ierr)
		} else if imgClient != nil {
			imgScheduled := core.NewScheduledLLMCall("image_generator", imgClient)
			shardMgr.SetImageLLMClient(imgScheduled)
			if img := appCfg.GetImageLLMConfig(); img.Model != "" {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Image generator: gemini / %s (Nano Banana 2)", img.Model),
					Time:    time.Now(),
				})
			}
		}
		if perception.SharedTaxonomy != nil {
			perception.SharedTaxonomy.SetClient(llmClient)
		}

		// Classification: prefer worker (cheap local) when configured; else
		// provider fast tier (Haiku/Flash/4o-mini).
		logStep("Configuring classification model tier...")
		var classificationClient perception.LLMClient
		if worker, werr := perception.NewWorkerClientFromUserConfig(appCfg); werr == nil && worker != nil {
			classificationClient = core.NewScheduledLLMCallWithPriority("classification", worker, types.PriorityHigh)
			logging.Boot("Classification client: worker LLM (cheap tier)")
		} else if provCfg, provErr := perception.DetectProvider(); provErr == nil {
			if cc, ccErr := perception.NewClassificationClientFromConfig(provCfg); ccErr == nil && cc != nil {
				classificationClient = core.NewScheduledLLMCallWithPriority("classification", cc, types.PriorityHigh)
				logging.Boot("Classification client enabled (fast model tier for perception)")
			}
		}
		perceptionClient := llmClient
		if classificationClient != nil {
			perceptionClient = classificationClient
		}

		// Initialize backend components that depend on the scheduled client.
		// Use LLM-first UnderstandingTransducer for intent classification.
		// The LLM describes intent, the harness validates and routes.
		logStep("Creating LLM-first transducer...")
		transducer := perception.NewUnderstandingTransducer(perceptionClient)
		// Ensure Perception layer subsystems (semantic classifier, etc.) are initialized.
		// Previously, InitPerceptionLayer existed but was never wired, leaving semantic intent
		// classification dormant even when embeddings are configured.
		if err := perception.InitPerceptionLayer(kernel, appCfg); err != nil {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("? Perception init failed: %v", err),
				Time:    time.Now(),
			})
		}

		// Initialize JIT Prompt Compiler with embedded corpus
		logStep("Initializing JIT prompt compiler...")
		var jitCompiler *prompt.JITPromptCompiler
		var promptEvolver *prompt_evolution.PromptEvolver
		jitCfg := appCfg.GetEffectiveJITConfig()

		// Load embedded corpus (baked-in prompt atoms)
		embeddedCorpus, embeddedErr := prompt.LoadEmbeddedCorpus()
		if embeddedErr != nil {
			logging.BootWarn("Failed to load embedded corpus: %v", embeddedErr)
		} else {
			logging.Boot("Loaded %d atoms from embedded corpus", embeddedCorpus.Count())
		}

		// Create JIT compiler with both embedded corpus AND kernel for skeleton selection
		// The kernel is REQUIRED for skeleton atom selection via Mangle rules
		compilerCfg := prompt.DefaultCompilerConfig()
		if jitCfg.TokenBudget > 0 {
			compilerCfg.DefaultTokenBudget = jitCfg.TokenBudget
		}
		compilerOpts := []prompt.CompilerOption{
			prompt.WithEmbeddedCorpus(embeddedCorpus),
			prompt.WithKernel(nerdsystem.NewKernelAdapter(kernel)),
			prompt.WithConfig(compilerCfg),
		}
		var defaultVectorSearcher *prompt.CompilerVectorSearcher
		if embeddingEngine != nil {
			defaultVectorSearcher = prompt.NewCompilerVectorSearcher(embeddingEngine)
			compilerOpts = append(compilerOpts, prompt.WithVectorSearcher(defaultVectorSearcher))
		}

		if jit, err := prompt.NewJITPromptCompiler(compilerOpts...); err == nil {
			jitCompiler = jit
			if defaultVectorSearcher != nil {
				defaultVectorSearcher.SetCompiler(jitCompiler)
			}

			// Ensure a project corpus DB exists for semantic retrieval.
			// Prefer the baked-in defaults corpus; fall back to SyncEmbeddedToSQLite when needed.
			nerdDir := filepath.Join(workspace, ".nerd")
			promptsDir := filepath.Join(nerdDir, "prompts")
			if mkdirErr := os.MkdirAll(promptsDir, 0755); mkdirErr != nil {
				logging.BootWarn("Failed to create prompts directory: %v", mkdirErr)
			} else {
				corpusPath := filepath.Join(promptsDir, "corpus.db")

				if wrote, err := prompt.MaterializeDefaultPromptCorpus(corpusPath); err != nil {
					logging.BootWarn("Failed to materialize default prompt corpus: %v", err)
				} else if wrote {
					logging.Boot("Materialized default prompt corpus to corpus.db")
				}

				// If no embedded default corpus is available, fall back to generating embeddings at runtime.
				if _, err := os.Stat(corpusPath); os.IsNotExist(err) && embeddingEngine != nil {
					logStep("Syncing embedded corpus to SQLite...")
					if syncErr := prompt.SyncEmbeddedToSQLite(context.Background(), corpusPath, embeddingEngine); syncErr != nil {
						logging.BootWarn("Failed to sync embedded corpus: %v", syncErr)
					}
				}

				// Ensure schema is up-to-date and tags are present, then register with JIT compiler.
				if _, err := os.Stat(corpusPath); err == nil {
					db, err := sql.Open("sqlite3", corpusPath)
					if err != nil {
						logging.BootWarn("Failed to open corpus DB for migrations: %v", err)
					} else {
						sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)
						loader := prompt.NewAtomLoader(nil)
						if err := loader.EnsureSchema(context.Background(), db); err != nil {
							logging.BootWarn("Failed to ensure corpus schema: %v", err)
						} else if embeddedCorpus != nil {
							if err := prompt.HydrateAtomContextTags(context.Background(), db, embeddedCorpus.All()); err != nil {
								logging.BootWarn("Failed to hydrate corpus tags: %v", err)
							}
						}
						_ = db.Close()
					}

					if promptCount, ingestErr := nerdsystem.IngestHybridPrompts(context.Background(), workspace, kernel, prompt.NewAtomLoader(nil)); ingestErr != nil {
						logging.BootWarn("Failed to ingest hybrid prompts during chat boot: %v", ingestErr)
					} else if promptCount > 0 {
						logging.Boot("Ingested %d hybrid PROMPT atoms during chat boot", promptCount)
					}

					// Register corpus DB with JIT compiler for project-level atom queries.
					if regErr := jitCompiler.RegisterDB("corpus", corpusPath); regErr != nil {
						logging.BootWarn("Failed to register corpus DB: %v", regErr)
					} else {
						logging.Boot("Registered corpus DB: %s", corpusPath)
					}
				}
			}

			// Wire prompt loader callback (YAML -> SQLite)
			shardMgr.SetNerdDir(nerdDir)
			shardMgr.SetPromptLoader(func(ctx context.Context, agentName, nerdDir string) (int, error) {
				return prompt.LoadAgentPrompts(ctx, agentName, nerdDir, embeddingEngine)
			})

			// Wire JIT DB registration callbacks
			shardMgr.SetJITRegistrar(prompt.CreateJITDBRegistrar(jitCompiler))
			shardMgr.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(jitCompiler))

			// Sync all agent prompts.yaml -> knowledge DBs at boot (upsert semantics)
			// This ensures edited prompts are available to the JIT compiler immediately
			logStep("Syncing agent prompts to knowledge DBs...")
			if promptCount, syncErr := prompt.ReloadAllPrompts(context.Background(), nerdDir, embeddingEngine); syncErr != nil {
				logging.BootWarn("Failed to sync agent prompts: %v", syncErr)
			} else if promptCount > 0 {
				logging.Boot("Synced %d prompt atoms from YAML to knowledge DBs", promptCount)
			}

			// Wire LocalDB for semantic knowledge atom queries (Semantic Knowledge Bridge)
			if localDB != nil {
				jitCompiler.SetLocalDB(localDB)
				logging.Boot("JIT compiler wired with LocalDB for semantic knowledge queries")
			}

			// Initialize Prompt Evolution System (System Prompt Learning) - inside JIT block for nerdDir access
			logStep("Initializing Prompt Evolution System...")
			evolverConfig := prompt_evolution.DefaultEvolverConfig()
			if pe, err := prompt_evolution.NewPromptEvolver(nerdDir, llmClient, evolverConfig); err == nil {
				promptEvolver = pe
				logging.Boot("Prompt Evolution System initialized")

				// Create and register EvolvedAtomManager with JIT compiler
				eam := prompt.NewEvolvedAtomManager(nerdDir)
				jitCompiler.RegisterEvolvedAtomManager(eam)
				logging.Boot("EvolvedAtomManager registered with JIT compiler: %d atoms", eam.Count())

				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: "✓ Prompt Evolution System initialized",
					Time:    time.Now(),
				})
			} else {
				logging.BootWarn("Failed to initialize Prompt Evolution: %v", err)
			}

			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: "✓ JIT prompt compiler initialized",
				Time:    time.Now(),
			})
		} else {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ JIT compiler init failed: %v", err),
				Time:    time.Now(),
			})
		}

		// Create PromptAssembler with JIT for dynamic prompt compilation
		var promptAssembler *articulation.PromptAssembler
		if jitCompiler != nil {
			if pa, err := articulation.NewPromptAssemblerWithJIT(kernel, jitCompiler); err == nil {
				promptAssembler = pa
				promptAssembler.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK, jitCfg.ReservedTokensFallbackRatio)
				promptAssembler.EnableJIT(jitCfg.Enabled)
				logging.Boot("PromptAssembler created with JIT compiler")
			} else {
				logging.BootWarn("Failed to create PromptAssembler with JIT: %v", err)
			}
		}
		if promptAssembler != nil {
			transducer.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(promptAssembler))
		}

		// Wire kernel to transducer for Mangle-based routing derivation.
		// This enables the harness to validate LLM classifications and derive routing.
		if tk, ok := transducer.(perception.TransducerWithKernel); ok {
			tk.SetKernel(kernel)
		}
		logging.Boot("Wired kernel to LLM-first transducer for routing")

		// Inject strategic knowledge for conceptual queries
		if strategicSummary := virtualStore.GetStrategicSummary(); strategicSummary != "" {
			transducer.SetStrategicContext(strategicSummary)
			logging.Boot("Injected strategic knowledge (%d chars) into transducer", len(strategicSummary))
		}

		// Create Glass Box event bus early so shards can capture it.
		// We enable it eagerly here so producers can emit during boot;
		// the TUI side controls subscription/display via initGlassBox.
		glassBoxEventBus := transparency.NewGlassBoxEventBus()
		glassBoxEventBus.Enable()
		shardMgr.SetGlassBoxBus(glassBoxEventBus)
		virtualStore.SetGlassBoxBus(glassBoxEventBus)

		// Create Tool Event bus for always-visible tool execution notifications
		toolEventBus := transparency.NewToolEventBus()
		virtualStore.SetToolEventBus(toolEventBus)

		// =======================================================================
		// CLEAN LOOP ARCHITECTURE: Create Session Executor and Spawner
		// These replace hardcoded shard logic with JIT-driven behavior
		// =======================================================================
		logStep("Creating clean loop executor...")
		var sessionExecutor *session.Executor
		var sessionSpawner *session.Spawner

		// Create adapters to bridge core types to session types
		// Use "cleanLoop" prefix to avoid conflicts with other adapters in this file
		cleanLoopKernelAdapter := &sessionKernelAdapter{kernel: kernel}
		cleanLoopVSAdapter := &sessionVirtualStoreAdapter{vs: virtualStore}
		// Domain persona work runs on the worker client when one is configured,
		// matching the live boot path (internal/system/factory.go:1207-1211) and
		// the contract in Docs/architecture/session/08-WIRING-AND-INTEGRATION.md
		// ("sessionLLM := sessionLLMAdapter{workerOrMainLLM}").
		//
		// NOTE: performSystemBootLegacy has no callers — the live TUI path is
		// model_lifecycle.go -> performSystemBoot (session_shared_boot.go) ->
		// BootCortexWithConfig -> internal/system/factory.go. This alignment is
		// kept so the legacy path stays correct if it is ever revived, but
		// changing it has no runtime effect today.
		cleanLoopLLMAdapter := &sessionLLMAdapter{client: shardLLMClient}

		// Create ConfigFactory with default config atoms
		// This provides tool sets and policies for different intent verbs
		configFactory := prompt.NewDefaultConfigFactory()

		// Derive a sub-agent prompt-compilation budget from the user's
		// configured context window. Without this, executor.go and
		// spawner.go used a hardcoded 8192-token budget that silently
		// dropped mandatory atoms (defensive_patterns, behavior_changes,
		// etc.) from every spawned shard's prompt — agents came up
		// amnesiac. We allocate up to half the context-window max for
		// the JIT prompt, capped at 256K so even 1M-context models leave
		// generous headroom for response + tool I/O. Falls back to
		// DefaultTokenBudget (65536) if no config is loaded.
		subAgentBudget := session.DefaultTokenBudget
		if appCfg != nil {
			ctxCfg := appCfg.GetContextWindowConfig()
			if ctxCfg.MaxTokens > 0 {
				half := ctxCfg.MaxTokens / 2
				switch {
				case half > 262144:
					subAgentBudget = 262144
				case half > session.DefaultTokenBudget:
					subAgentBudget = half
				}
			}
		}

		// Create the clean execution loop
		sessionExecutor = session.NewExecutor(
			cleanLoopKernelAdapter,
			cleanLoopVSAdapter,
			cleanLoopLLMAdapter,
			jitCompiler,
			configFactory,
			transducer,
		)
		execCfg := session.DefaultExecutorConfig()
		execCfg.TokenBudget = subAgentBudget
		sessionExecutor.SetConfig(execCfg)

		// Create the JIT-driven subagent spawner with the same budget.
		spawnCfg := session.DefaultSpawnerConfig()
		spawnCfg.TokenBudget = subAgentBudget
		sessionSpawner = session.NewSpawner(
			cleanLoopKernelAdapter,
			cleanLoopVSAdapter,
			cleanLoopLLMAdapter,
			jitCompiler,
			configFactory,
			transducer,
			spawnCfg,
		)

		logging.Boot("Sub-agent token budget set to %d (derived from context_window.max_tokens=%d)",
			subAgentBudget, appCfg.GetContextWindowConfig().MaxTokens)

		logging.Boot("Clean loop executor and spawner initialized")

		// Create JITExecutor - the new unified task execution interface
		// This replaces LegacyBridge which wrapped ShardManager
		taskExecutor = session.NewJITExecutor(sessionExecutor, sessionSpawner, transducer)
		virtualStore.SetTaskExecutor(&chatTaskDelegatorAdapter{executor: taskExecutor})
		// Same executor for ShardManager's no-factory fallback. Domain personas
		// and user-defined agents have no in-process factory since the JIT
		// migration; without this, ShardManager.Spawn on those types resolved to
		// a BaseShardAgent that returned a placeholder with a nil error.
		shardMgr.SetTaskDelegator(&chatTaskDelegatorAdapter{executor: taskExecutor})
		logging.Boot("JITExecutor wired to VirtualStore and ShardManager")

		// Create Tool Store for persisting full tool execution results
		var toolStore *store.ToolStore
		toolsDBPath := filepath.Join(workspace, ".nerd", "tools.db")
		if ts, err := store.NewToolStore(toolsDBPath); err == nil {
			toolStore = ts
			logging.Boot("Initialized ToolStore at %s", toolsDBPath)
		} else {
			logging.Get(logging.CategoryBoot).Warn("Failed to initialize ToolStore: %v", err)
		}

		logStep("Registering shard types...")
		// =========================================================================
		// DOMAIN SHARDS REMOVED - JIT CLEAN LOOP ARCHITECTURE
		// =========================================================================
		// The following domain shards have been replaced by the JIT clean loop:
		// - coder: Now handled by session.Executor with /coder persona atoms
		// - reviewer: Now handled by session.Executor with /reviewer persona atoms
		// - tester: Now handled by session.Executor with /tester persona atoms
		// - researcher: Now handled by session.Executor with /researcher persona atoms
		//
		// The JIT prompt compiler assembles the appropriate persona, skills, and
		// context based on user intent. ConfigFactory provides tool sets per intent.
		// See: internal/mangle/intent_routing.mg for routing rules
		// =========================================================================

		// System Shards
		shardMgr.RegisterShard("perception_firewall", func(id string, config types.ShardConfig) types.ShardAgent {
			perceptionCfg := shardsystem.DefaultPerceptionConfig()
			if appCfg != nil {
				perceptionCfg.LearningCandidateThreshold = appCfg.GetLearningCandidateThreshold()
				perceptionCfg.LearningCandidateAutoPromote = appCfg.GetLearningCandidateAutoPromote()
			}
			shard := shardsystem.NewPerceptionFirewallShardWithConfig(perceptionCfg)
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(shardLLMClient)
			// Model tiering: classification runs on the fast tier when available
			// (mirrors the transducer wiring above — the firewall is the primary
			// perception path on interactive turns).
			if classificationClient != nil {
				shard.SetClassificationClient(classificationClient)
			}
			if localDB != nil {
				shard.SetLearningCandidateStore(localDB)
			}
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("world_model_ingestor", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewWorldModelIngestorShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(shardLLMClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("executive_policy", func(id string, config types.ShardConfig) types.ShardAgent {
			execCfg := shardsystem.DefaultExecutiveConfig()
			if appCfg != nil {
				execCfg.LearningCandidateThreshold = appCfg.GetLearningCandidateThreshold()
			}
			shard := shardsystem.NewExecutivePolicyShardWithConfig(execCfg)
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(shardLLMClient)
			if localDB != nil {
				shard.SetLearningCandidateStore(localDB)
			}
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("constitution_gate", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewConstitutionGateShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(shardLLMClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("mangle_repair", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewMangleRepairShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(shardLLMClient)
			// Wire the predicate corpus from kernel for schema validation
			if corpus := kernel.GetPredicateCorpus(); corpus != nil {
				shard.SetCorpus(corpus)
			}
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			// Wire the repair shard as the kernel's learned rule interceptor
			// This ensures all learned rules pass through validation/repair before persistence
			kernel.SetRepairInterceptor(shard)
			return shard
		})
		shardMgr.RegisterShard("tactile_router", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewTactileRouterShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(shardLLMClient)
			shard.SetGlassBox(glassBoxEventBus) // Wire Glass Box for debug visibility
			shard.SetToolEventBus(toolEventBus) // Wire Tool Event Bus for always-visible tool execution
			shard.SetToolStore(toolStore)       // Wire Tool Store for full result persistence
			if browserMgr != nil {
				shard.SetBrowserManager(browserMgr)
			}
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("session_planner", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewSessionPlannerShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(shardLLMClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})

		// =========================================================================
		// Register remaining system shards (legislator, campaign_runner,
		// requirements_interrogator) - domain shards moved to JIT clean loop
		// =========================================================================

		// Register RequirementsInterrogator - Socratic clarification shard
		shardMgr.RegisterShard("requirements_interrogator", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shards.NewRequirementsInterrogatorShard()
			shard.SetLLMClient(shardLLMClient)
			shard.SetParentKernel(kernel)
			return shard
		})

		// =========================================================================
		// TOOL_GENERATOR AND NEMESIS REMOVED - JIT CLEAN LOOP
		// =========================================================================
		// - tool_generator: Now handled via Ouroboros through VirtualStore
		// - nemesis: Now handled via Thunderdome adversarial testing
		// The JIT system provides the appropriate tools and context.
		// =========================================================================

		// Register Legislator - Runtime rule compilation shard
		shardMgr.RegisterShard("legislator", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewLegislatorShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(shardLLMClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})

		// Register CampaignRunner - Campaign orchestration shard
		shardMgr.RegisterShard("campaign_runner", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewCampaignRunnerShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(shardLLMClient)
			shard.SetWorkspaceRoot(workspace)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})

		shards.RegisterSystemShardProfiles(shardMgr)

		// Master switch for system shards. When features.IsSystemShardsEnabled
		// returns false (CODENERD_SYSTEM_SHARDS=0 or config sets it false),
		// skip the entire boot block. The legacy NERD_DISABLE_SYSTEM_SHARDS
		// env var is still respected per-shard below.
		if !features.IsSystemShardsEnabled() {
			logStep("System shards disabled via feature flag; skipping boot")
		} else {
			// HEAVY OPERATION: Start System Shards (Async but setup overhead)
			logStep("Starting system shards...")
			ctx := context.Background()
			disabled := make(map[string]struct{})
			for _, name := range disableSystemShards {
				disabled[name] = struct{}{}
			}
			if env := os.Getenv("NERD_DISABLE_SYSTEM_SHARDS"); env != "" {
				for token := range strings.SplitSeq(env, ",") {
					name := strings.TrimSpace(token)
					if name != "" {
						disabled[name] = struct{}{}
					}
				}
			}
			for name := range disabled {
				shardMgr.DisableSystemShard(name)
			}
			if err := shardMgr.StartSystemShards(ctx); err != nil {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to start system shards: %v", err),
					Time:    time.Now(),
				})
			}
		}

		logStep("Creating shadow mode & scanner...")
		shadowMode := core.NewShadowMode(kernel)
		// GAP-011 FIX: Removed unused emitter - articulation uses PromptAssembler.JIT instead
		scanner := world.NewScanner()

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

		// GAP-003 FIX: Seed activation engine with corpus priorities
		if corpus := kernel.GetPredicateCorpus(); corpus != nil {
			if err := compressor.LoadPrioritiesFromCorpus(corpus); err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to load corpus priorities: %v", err)
			}
		}

		// Initialize Context Feedback Store (Third feedback loop: context usefulness learning)
		logStep("Initializing context feedback store...")
		feedbackDBPath := filepath.Join(workspace, ".nerd", "context_feedback.db")
		var feedbackStore *ctxcompress.ContextFeedbackStore
		if fs, err := ctxcompress.NewContextFeedbackStore(feedbackDBPath); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to create context feedback store: %v", err)
		} else {
			feedbackStore = fs
			// Wire feedback store to compressor's activation engine
			compressor.SetFeedbackStore(feedbackStore)
			logging.Context("Context feedback store initialized at %s", feedbackDBPath)
		}

		logStep("Starting autopoiesis orchestrator...")
		autopoiesisConfig := autopoiesis.DefaultConfig(workspace)
		autopoiesisOrch := autopoiesis.NewOrchestrator(llmClient, autopoiesisConfig)
		autopoiesisKernelAdapter := core.NewAutopoiesisBridge(kernel)
		autopoiesisOrch.SetKernel(autopoiesisKernelAdapter)
		if promptAssembler != nil {
			// Wire JIT-capable prompt assembly into autopoiesis tool generation/refinement.
			// jitCompiler is intentionally nil here: autopoiesis currently consumes the
			// assembler interface for JIT prompt paths.
			autopoiesisOrch.SetPromptAssembler(promptAssembler)
		}

		autopoiesisCtx, autopoiesisCancel := context.WithCancel(context.Background())
		autopoiesisListenerCh := autopoiesisOrch.StartKernelListener(autopoiesisCtx, 2*time.Second)

		logStep("Creating task verifier...")
		taskVerifier := verification.NewTaskVerifier(
			llmClient,
			localDB,
			shardMgr,
			autopoiesisOrch,
		)
		logStep("Task verifier initialized")
		taskVerifier.SetTaskExecutor(taskExecutor)
		logStep("Task verifier wired to executor")

		toolExecutor := NewToolExecutorAdapter(autopoiesisOrch)
		virtualStore.SetToolExecutor(toolExecutor)
		logStep("Tool executor wired")

		// Wire Ouroboros as ToolGenerator for coder shard self-tool routing
		if ouroborosLoop := autopoiesisOrch.GetOuroborosLoop(); ouroborosLoop != nil {
			virtualStore.SetToolGenerator(ouroborosLoop)
			logStep("Wired Ouroboros as ToolGenerator for self-tool routing")
		}

		// Wire tool registry to session executor for Piggyback++ dual-registry.
		// This enables the Executor to include Ouroboros-generated tools in its
		// tool catalog (buildToolCatalogForPiggyback) and execute them (executeToolCall).
		// The VirtualStore's toolRegistry is already synced from Ouroboros above.
		if sessionExecutor != nil {
			sessionExecutor.SetOuroborosRegistry(virtualStore.GetToolRegistry())
			logStep("Wired Ouroboros tool registry to session executor for Piggyback++ dual-registry")
		}

		// Create Dream → Ouroboros tool need bridge goroutine.
		// When Dream State identifies a capability gap (ToolNeed), this bridge
		// converts core.ToolNeed to autopoiesis.ToolNeed and dispatches to Ouroboros.
		// The goroutine is bounded by autopoiesisCtx cancellation (fail-fast on shutdown).
		var dreamToolQ chan<- core.ToolNeed
		dreamToolCh := make(chan core.ToolNeed, 16)
		dreamToolQ = dreamToolCh
		go func() {
			// Bug #16 fix: bound goroutine lifetime to autopoiesisCtx.
			// Previously this used `for need := range dreamToolCh` with no
			// cancellation arm. The channel is never closed (its sender is the
			// DreamRouter, which we don't own here), so on autopoiesis shutdown
			// the goroutine would block forever on a receive and leak.
			for {
				select {
				case <-autopoiesisCtx.Done():
					return
				case need, ok := <-dreamToolCh:
					if !ok {
						return
					}
					ctx, cancel := context.WithTimeout(autopoiesisCtx, 5*time.Minute)
					autoNeed := &autopoiesis.ToolNeed{
						Name:     need.Name,
						Purpose:  need.Description,
						Priority: need.Priority,
					}
					autopoiesisOrch.ExecuteOuroborosLoop(ctx, autoNeed)
					cancel()
				}
			}
		}()
		logStep("Dream → Ouroboros tool need bridge started")

		// Hydrate tools from disk and available_tools.json
		logStep("Hydrating tools from .nerd/tools/...")
		toolsNerdDir := filepath.Join(workspace, ".nerd")
		if err := hydrateAllTools(virtualStore, toolsNerdDir); err != nil {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ Tool hydration warning: %v", err),
				Time:    time.Now(),
			})
		}

		logStep("Hydrating session state...")
		loadedSession, _ := hydrateNerdState(workspace, kernel, shardMgr, &initialMessages)
		shardMgr.SetSessionID(resolveSessionID(loadedSession))

		shards.RegisterSystemShardProfiles(shardMgr)

		// HEAVY OPERATION: Start System Shards (Async but setup overhead)
		logStep("Starting system shards...")
		var mangleWatcher *core.MangleWatcher
		if mw, err := core.NewMangleWatcher(workspace, kernel); err == nil {
			mangleWatcher = mw
			watchCtx := context.Background()
			if err := mangleWatcher.Start(watchCtx); err != nil {
				logging.Get(logging.CategoryKernel).Warn("Failed to start Mangle watcher: %v", err)
			} else {
				logging.Kernel("Mangle file watcher started for %s/.nerd/mangle", workspace)
			}
		} else {
			logging.Get(logging.CategoryKernel).Warn("Failed to create Mangle watcher: %v", err)
		}

		// glassBoxEventBus was created earlier to allow shard factories to capture it

		// Initialize Background Observer Manager (for Northstar alignment guardian, etc.)
		logStep("Setting up background observers...")
		observerMgr := shards.NewBackgroundObserverManager(&taskExecutorObserverSpawner{taskExecutor})
		// Register Northstar as a background observer (if available)
		if err := observerMgr.RegisterObserver("northstar"); err == nil {
			// Don't start yet - will be started on demand
			logging.Get(logging.CategoryBoot).Info("Northstar observer registered")

			// Wire Northstar Guardian for intelligent periodic checks
			nerdDir := filepath.Join(workspace, ".nerd")
			if northstarStore, err := northstar.NewStore(nerdDir); err == nil {
				guardianConfig := northstar.DefaultGuardianConfig()
				guardian := northstar.NewGuardian(northstarStore, guardianConfig)
				guardian.SetLLMClient(shardLLMClient)
				if err := guardian.Initialize(); err == nil {
					sessionID := resolveSessionID(loadedSession)
					handler := northstar.NewBackgroundEventHandler(guardian, sessionID)
					observerMgr.SetNorthstarHandler(&northstarHandlerAdapter{handler})
					logging.Get(logging.CategoryNorthstar).Info("Northstar Guardian wired into background observer")
				}
			}
		}

		// Initialize Consultation Manager (cross-specialist collaboration protocol)
		logStep("Setting up consultation protocol...")
		consultationMgr := shards.NewConsultationManager(&taskExecutorConsultationSpawner{taskExecutor})
		logging.Get(logging.CategoryBoot).Info("Consultation manager initialized")

		fmt.Printf("\r\033[K[boot] Complete! (%.1fs)\n", time.Since(bootStart).Seconds())
		return bootCompleteMsg{
			components: &SystemComponents{
				Kernel:                kernel,
				ShardMgr:              shardMgr,
				TaskExecutor:          taskExecutor,
				ShadowMode:            shadowMode,
				Transducer:            transducer,
				Executor:              executor,
				Emitter:               nil, // GAP-011: Emitter unused, using JIT PromptAssembler instead
				VirtualStore:          virtualStore,
				Scanner:               scanner,
				Workspace:             workspace,
				SessionID:             resolveSessionID(loadedSession),
				TurnCount:             resolveTurnCount(loadedSession),
				LocalDB:               localDB,
				Compressor:            compressor,
				FeedbackStore:         feedbackStore,
				Autopoiesis:           autopoiesisOrch,
				AutopoiesisCancel:     autopoiesisCancel,
				AutopoiesisListenerCh: autopoiesisListenerCh,
				Verifier:              taskVerifier,
				InitialMessages:       initialMessages,
				Client:                llmClient,
				BrowserManager:        browserMgr,
				BrowserCtxCancel:      browserCtxCancel,
				JITCompiler:           jitCompiler,
				MangleWatcher:         mangleWatcher,
				TransparencyMgr:       transparencyMgr,
				PreferencesMgr:        prefsMgr,
				Retriever:             retriever,
				GlassBoxEventBus:      glassBoxEventBus,
				ToolEventBus:          toolEventBus,
				ToolStore:             toolStore,
				PromptEvolver:         promptEvolver,
				EmbeddingEngine:       embeddingEngine,
				LearningStore:         learningStore,
				// Clean Loop Architecture
				SessionExecutor: sessionExecutor,
				SessionSpawner:  sessionSpawner,
				// Background Observer Manager
				ObserverMgr: observerMgr,
				// Consultation Manager
				ConsultationMgr: consultationMgr,
				// Dream → Ouroboros bridge channel
				DreamToolQ: dreamToolQ,
			},
		}
	}
}

func detectLLMProvider(initialMessages []Message) (perception.LLMClient, []Message) {
	baseLLMClient, clientErr := perception.NewClientFromEnv()
	if clientErr != nil {
		initialMessages = append(initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("⚠ LLM client init failed: %v\n\nSet an API key in `.nerd/config.json` or via environment variable.", clientErr),
			Time:    time.Now(),
		})
		baseLLMClient = perception.NewZAIClient("")
	} else {
		providerCfg, _ := perception.DetectProvider()
		if providerCfg != nil {
			providerLabel := string(providerCfg.Provider)
			modelInfo := providerCfg.Model

			switch providerCfg.Engine {
			case "claude-cli":
				providerLabel = "claude-cli"
				if providerCfg.ClaudeCLI != nil && providerCfg.ClaudeCLI.Model != "" {
					modelInfo = providerCfg.ClaudeCLI.Model
				}
			case "codex-cli":
				providerLabel = "codex-cli"
				if providerCfg.CodexCLI != nil && providerCfg.CodexCLI.Model != "" {
					modelInfo = providerCfg.CodexCLI.Model
				}
			}

			if modelInfo == "" {
				modelInfo = "default"
			}
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("✓ Using %s (model: %s)", providerLabel, modelInfo),
				Time:    time.Now(),
			})
		}
	}
	return baseLLMClient, initialMessages
}

func initEmbeddingEngine(appCfg *config.UserConfig, initialMessages []Message, localDB *store.LocalStore) (embedding.EmbeddingEngine, []Message) {
	var embeddingEngine embedding.EmbeddingEngine
	embCfg := appCfg.GetEmbeddingConfig()
	if embCfg.Provider != "" {
		embConfig := embedding.Config{
			Provider:       embCfg.Provider,
			OllamaEndpoint: embCfg.OllamaEndpoint,
			OllamaModel:    embCfg.OllamaModel,
			GenAIAPIKey:    embCfg.GenAIAPIKey,
			GenAIModel:     embCfg.GenAIModel,
			TaskType:       embCfg.TaskType,
		}
		logging.Boot("Embedding from config.json: provider=%s model=%s endpoint=%s",
			embConfig.Provider, embConfig.OllamaModel, embConfig.OllamaEndpoint)
		if engine, err := embedding.NewEngine(embConfig); err == nil {
			embeddingEngine = engine
			if localDB != nil {
				localDB.SetEmbeddingEngine(engine)
			}
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("✓ Embedding engine: %s (from config.json)", engine.Name()),
				Time:    time.Now(),
			})
		} else {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ Embedding init failed: %v", err),
				Time:    time.Now(),
			})
		}
	}
	return embeddingEngine, initialMessages
}
