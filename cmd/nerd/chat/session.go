// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains session management, initialization, and state persistence.
package chat

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	prompt_evolution "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/browser"
	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	nerdinit "codenerd/internal/init"
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
	"codenerd/internal/store"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"codenerd/internal/ux"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================
// Functions for initializing the chat, loading/saving session state, and
// managing persistent configuration.

// InitChat initializes the interactive chat model (Lightweight UI only)
func InitChat(cfg Config) Model {
	// Load configuration from unified .nerd/config.json
	appCfg, _ := config.GlobalConfig()
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}

	// Initialize styles
	styles := ui.DefaultStyles()
	if appCfg.Theme == "dark" {
		styles = ui.NewStyles(ui.DarkTheme())
	}

	// Initialize textarea for input
	ta := textarea.New()
	ta.Placeholder = "System initializing..."
	ta.Prompt = "┃ "
	ta.CharLimit = 0 // Unlimited
	ta.SetWidth(80)
	ta.SetHeight(3) // 3 lines default
	ta.ShowLineNumbers = false

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	// Initialize viewport for chat history
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Initialize viewport for error panel (small + scrollable)
	errVP := viewport.New(76, errorPanelViewportHeight)
	errVP.SetContent("")

	// Initialize list (empty by default)
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Past Sessions"
	l.SetShowHelp(false)

	// Initialize file picker
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // All files
	fp.CurrentDirectory, _ = os.Getwd()

	// Initialize markdown renderer
	var renderer *glamour.TermRenderer
	if styles.Theme.IsDark {
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(80),
		)
	} else {
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylePath("light"),
			glamour.WithWordWrap(80),
		)
	}

	// Resolve workspace
	workspace, _ := os.Getwd()

	// Note: API key parsing is handled by perception.NewClientFromEnv() during boot
	// The perception package supports multiple providers (zai, anthropic, openai, gemini, xai, openrouter)
	// and reads configuration from .nerd/config.json or environment variables

	// Initialize Usage Tracker (lightweight)
	tracker, err := usage.NewTracker(workspace)
	if err != nil {
		fmt.Printf("⚠ Usage tracking init failed: %v\n", err)
	}

	// Initialize split-pane view
	splitPaneView := ui.NewSplitPaneView(styles, 80, 24)

	// Create shutdown context for coordinating background goroutine lifecycle
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	// Initialize Preferences Manager
	prefsMgr := ux.NewPreferencesManager(workspace)
	if err := prefsMgr.Load(); err != nil {
		fmt.Printf("⚠ Failed to load preferences: %v\n", err)
	}

	// Initialize Transparency Manager
	transparencyCfg := appCfg.GetTransparencyConfig()
	transparencyMgr := transparency.NewTransparencyManager(transparencyCfg)

	// Return the model in "Booting" state
	return Model{
		textarea:     ta,
		viewport:     vp,
		errorVP:      errVP,
		spinner:      sp,
		list:         l,
		filepicker:   fp,
		styles:       styles,
		renderer:     renderer,
		usageTracker: tracker,
		usagePage:    ui.NewUsagePageModel(tracker, styles),
		jitPage:      ui.NewJITPageModel(),
		autoPage:     ui.NewAutopoiesisPageModel(),
		shardPage:    ui.NewShardPageModel(),
		splitPane:    &splitPaneView,
		logicPane:    splitPaneView.RightPane,
		showLogic:    false,
		paneMode:     ui.ModeSinglePane,
		showError:    true,
		focusError:   false,
		// System action summaries are noisy; default to showing them only in debug mode.
		showSystemActions: appCfg != nil && appCfg.Logging != nil && appCfg.Logging.DebugMode,
		history:           []Message{},
		Config:            appCfg,
		// Rendering cache for performance
		renderedCache:    make(map[int]string),
		cacheInvalidFrom: 0, // All messages need rendering initially
		// Backend components start nil
		kernel:              nil,
		shardMgr:            nil,
		client:              nil,              // Will be set in boot
		isBooting:           true,             // Flag for UI
		bootStage:           BootStageBooting, // Startup phase
		statusChan:          make(chan string, 10),
		workspace:           workspace,
		DisableSystemShards: cfg.DisableSystemShards,
		// Mouse capture enabled by default (Alt+M to toggle for text selection)
		mouseEnabled: true,
		// Shutdown coordination (pointer to sync.Once to allow Model copy without noCopy violation)
		shutdownOnce:   &sync.Once{},
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		goroutineWg:    &sync.WaitGroup{},
		// UX components
		preferencesMgr:  prefsMgr,
		transparencyMgr: transparencyMgr,
	}
}

// performSystemBoot performs the heavy backend initialization in a background thread
func performSystemBoot(cfg *config.UserConfig, disableSystemShards []string, workspace string) tea.Cmd {
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
		schedulerCfg.MaxConcurrentAPICalls = coreLimits.MaxConcurrentAPICalls
		schedulerCfg.SlotAcquireTimeout = config.GetLLMTimeouts().SlotAcquisitionTimeout
		core.ConfigureGlobalAPIScheduler(schedulerCfg)
		initialMessages := []Message{}

		// Initialize LLM client using the perception package's provider detection
		// This supports all providers: zai, anthropic, openai, gemini, xai, openrouter
		// Configuration is read from .nerd/config.json or environment variables
		logStep("Detecting LLM provider...")
		baseLLMClient, clientErr := perception.NewClientFromEnv()
		if clientErr != nil {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ LLM client init failed: %v\n\nSet an API key in `.nerd/config.json` or via environment variable.", clientErr),
				Time:    time.Now(),
			})
			// Create a fallback client that will error on use
			baseLLMClient = perception.NewZAIClient("")
		} else {
			// Report which provider was detected
			providerCfg, _ := perception.DetectProvider()
			if providerCfg != nil {
				modelInfo := providerCfg.Model
				if modelInfo == "" {
					modelInfo = "default"
				}
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Using %s provider (model: %s)", providerCfg.Provider, modelInfo),
					Time:    time.Now(),
				})
			}
		}

		// HEAVY OPERATION: NewRealKernel calls Evaluate() internally
		logStep("Booting Mangle kernel...")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return fmt.Errorf("failed to create kernel: %w", err)
		}

		// NewRealKernel now properly returns errors instead of panicking.
		// The kernel is already evaluated during construction.
		logStep("Evaluating kernel rules...")
		if err := kernel.Evaluate(); err != nil {
			return bootCompleteMsg{err: fmt.Errorf("kernel boot failed: %w", err)}
		}

		// GAP-013 FIX: Consume boot intents and prompts from hybrid files
		bootIntents := kernel.ConsumeBootIntents()
		if len(bootIntents) > 0 {
			logging.Get(logging.CategoryKernel).Info("Consumed %d boot intents from hybrid files", len(bootIntents))
		}
		bootPrompts := kernel.ConsumeBootPrompts()
		if len(bootPrompts) > 0 {
			logging.Get(logging.CategoryKernel).Info("Consumed %d boot prompts from hybrid files", len(bootPrompts))
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

		// Initialize embedding engine
		logStep("Initializing embedding engine...")
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
			if engine, err := embedding.NewEngine(embConfig); err == nil {
				embeddingEngine = engine
				if localDB != nil {
					localDB.SetEmbeddingEngine(engine)
				}
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Embedding engine: %s", engine.Name()),
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
		_ = embeddingEngine

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

		shardMgr.SetLLMClient(rawLLMClient)

		// llmClient is used by non-shard components; wrap with scheduler to honor API concurrency.
		var llmClient perception.LLMClient = core.NewScheduledLLMCall("main", rawLLMClient)
		if perception.SharedTaxonomy != nil {
			perception.SharedTaxonomy.SetClient(llmClient)
		}

		// Initialize backend components that depend on the scheduled client.
		// Use LLM-first UnderstandingTransducer for intent classification.
		// The LLM describes intent, the harness validates and routes.
		logStep("Creating LLM-first transducer...")
		transducer := perception.NewUnderstandingTransducer(llmClient)
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
			logging.Boot("Warning: Failed to load embedded corpus: %v", embeddedErr)
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
				logging.Boot("Warning: Failed to create prompts directory: %v", mkdirErr)
			} else {
				corpusPath := filepath.Join(promptsDir, "corpus.db")

				if wrote, err := prompt.MaterializeDefaultPromptCorpus(corpusPath); err != nil {
					logging.Boot("Warning: Failed to materialize default prompt corpus: %v", err)
				} else if wrote {
					logging.Boot("Materialized default prompt corpus to corpus.db")
				}

				// If no embedded default corpus is available, fall back to generating embeddings at runtime.
				if _, err := os.Stat(corpusPath); os.IsNotExist(err) && embeddingEngine != nil {
					logStep("Syncing embedded corpus to SQLite...")
					if syncErr := prompt.SyncEmbeddedToSQLite(context.Background(), corpusPath, embeddingEngine); syncErr != nil {
						logging.Boot("Warning: Failed to sync embedded corpus: %v", syncErr)
					}
				}

				// Ensure schema is up-to-date and tags are present, then register with JIT compiler.
				if _, err := os.Stat(corpusPath); err == nil {
					db, err := sql.Open("sqlite3", corpusPath)
					if err != nil {
						logging.Boot("Warning: Failed to open corpus DB for migrations: %v", err)
					} else {
						loader := prompt.NewAtomLoader(nil)
						if err := loader.EnsureSchema(context.Background(), db); err != nil {
							logging.Boot("Warning: Failed to ensure corpus schema: %v", err)
						} else if embeddedCorpus != nil {
							if err := prompt.HydrateAtomContextTags(context.Background(), db, embeddedCorpus.All()); err != nil {
								logging.Boot("Warning: Failed to hydrate corpus tags: %v", err)
							}
						}
						_ = db.Close()
					}

					// Register corpus DB with JIT compiler for project-level atom queries.
					if regErr := jitCompiler.RegisterDB("corpus", corpusPath); regErr != nil {
						logging.Boot("Warning: Failed to register corpus DB: %v", regErr)
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
				logging.Boot("Warning: Failed to sync agent prompts: %v", syncErr)
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
				logging.Boot("Warning: Failed to initialize Prompt Evolution: %v", err)
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
				promptAssembler.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK)
				promptAssembler.EnableJIT(jitCfg.Enabled)
				logging.Boot("PromptAssembler created with JIT compiler")
			} else {
				logging.Boot("Warning: Failed to create PromptAssembler with JIT: %v", err)
			}
		}
		if promptAssembler != nil {
			transducer.SetPromptAssembler(promptAssembler)
		}

		// Wire kernel to transducer for Mangle-based routing derivation.
		// This enables the harness to validate LLM classifications and derive routing.
		transducer.SetKernel(kernel)
		logging.Boot("Wired kernel to LLM-first transducer for routing")

		// Inject strategic knowledge for conceptual queries
		if strategicSummary := virtualStore.GetStrategicSummary(); strategicSummary != "" {
			transducer.SetStrategicContext(strategicSummary)
			logging.Boot("Injected strategic knowledge (%d chars) into transducer", len(strategicSummary))
		}

		// Create Glass Box event bus early so shards can capture it
		glassBoxEventBus := transparency.NewGlassBoxEventBus()

		// Create Tool Event bus for always-visible tool execution notifications
		toolEventBus := transparency.NewToolEventBus()

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
		cleanLoopLLMAdapter := &sessionLLMAdapter{client: llmClient}

		// Create ConfigFactory with default config atoms
		// This provides tool sets and policies for different intent verbs
		configFactory := prompt.NewDefaultConfigFactory()

		// Create the clean execution loop
		sessionExecutor = session.NewExecutor(
			cleanLoopKernelAdapter,
			cleanLoopVSAdapter,
			cleanLoopLLMAdapter,
			jitCompiler,
			configFactory,
			transducer,
		)

		// Create the JIT-driven subagent spawner with default config
		sessionSpawner = session.NewSpawner(
			cleanLoopKernelAdapter,
			cleanLoopVSAdapter,
			cleanLoopLLMAdapter,
			jitCompiler,
			configFactory,
			transducer,
			session.DefaultSpawnerConfig(),
		)

		logging.Boot("Clean loop executor and spawner initialized")

		// Create JITExecutor - the new unified task execution interface
		// This replaces LegacyBridge which wrapped ShardManager
		taskExecutor = session.NewJITExecutor(sessionExecutor, sessionSpawner, transducer)
		virtualStore.SetTaskExecutor(taskExecutor)
		logging.Boot("JITExecutor wired to VirtualStore")

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
			shard := shardsystem.NewPerceptionFirewallShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("world_model_ingestor", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewWorldModelIngestorShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("executive_policy", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewExecutivePolicyShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("constitution_gate", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewConstitutionGateShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})
		shardMgr.RegisterShard("mangle_repair", func(id string, config types.ShardConfig) types.ShardAgent {
			shard := shardsystem.NewMangleRepairShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
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
			shard.SetLLMClient(llmClient)
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
			shard.SetLLMClient(llmClient)
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
			shard.SetLLMClient(llmClient)
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
			shard.SetLLMClient(llmClient)
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
			shard.SetLLMClient(llmClient)
			shard.SetWorkspaceRoot(workspace)
			if promptAssembler != nil {
				shard.SetPromptAssembler(promptAssembler)
			}
			return shard
		})

		shards.RegisterSystemShardProfiles(shardMgr)

		// HEAVY OPERATION: Start System Shards (Async but setup overhead)
		logStep("Starting system shards...")
		ctx := context.Background()
		disabled := make(map[string]struct{})
		for _, name := range disableSystemShards {
			disabled[name] = struct{}{}
		}
		if env := os.Getenv("NERD_DISABLE_SYSTEM_SHARDS"); env != "" {
			for _, token := range strings.Split(env, ",") {
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

		autopoiesisCtx, autopoiesisCancel := context.WithCancel(context.Background())
		autopoiesisListenerCh := autopoiesisOrch.StartKernelListener(autopoiesisCtx, 2*time.Second)

		logStep("Creating task verifier...")
		context7Key := appCfg.Context7APIKey
		if context7Key == "" {
			context7Key = os.Getenv("CONTEXT7_API_KEY")
		}
		taskVerifier := verification.NewTaskVerifier(
			llmClient,
			localDB,
			shardMgr,
			autopoiesisOrch,
			context7Key,
		)
		taskVerifier.SetTaskExecutor(taskExecutor)

		toolExecutor := NewToolExecutorAdapter(autopoiesisOrch)
		virtualStore.SetToolExecutor(toolExecutor)

		// Wire Ouroboros as ToolGenerator for coder shard self-tool routing
		if ouroborosLoop := autopoiesisOrch.GetOuroborosLoop(); ouroborosLoop != nil {
			virtualStore.SetToolGenerator(ouroborosLoop)
			logStep("Wired Ouroboros as ToolGenerator for self-tool routing")
		}

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
		observerMgr := shards.NewBackgroundObserverManager(&shardManagerObserverSpawner{shardMgr})
		// Register Northstar as a background observer (if available)
		if err := observerMgr.RegisterObserver("northstar"); err == nil {
			// Don't start yet - will be started on demand
			logging.Get(logging.CategoryBoot).Info("Northstar observer registered")

			// Wire Northstar Guardian for intelligent periodic checks
			nerdDir := filepath.Join(workspace, ".nerd")
			if northstarStore, err := northstar.NewStore(nerdDir); err == nil {
				guardianConfig := northstar.DefaultGuardianConfig()
				guardian := northstar.NewGuardian(northstarStore, guardianConfig)
				guardian.SetLLMClient(llmClient)
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
		consultationMgr := shards.NewConsultationManager(&shardManagerConsultationSpawner{shardMgr})
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
				// Clean Loop Architecture
				SessionExecutor: sessionExecutor,
				SessionSpawner:  sessionSpawner,
				// Background Observer Manager
				ObserverMgr: observerMgr,
				// Consultation Manager
				ConsultationMgr: consultationMgr,
			},
		}
	}
}

// shardManagerObserverSpawner adapts ShardManager to ObserverSpawner interface.
type shardManagerObserverSpawner struct {
	shardMgr *coreshards.ShardManager
}

func (s *shardManagerObserverSpawner) SpawnObserver(ctx context.Context, observerName, task string) (string, error) {
	if s.shardMgr == nil {
		return "", fmt.Errorf("shard manager not available")
	}
	return s.shardMgr.Spawn(ctx, observerName, task)
}

// shardManagerConsultationSpawner adapts ShardManager to ConsultationSpawner interface.
type shardManagerConsultationSpawner struct {
	shardMgr *coreshards.ShardManager
}

func (s *shardManagerConsultationSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if s.shardMgr == nil {
		return "", fmt.Errorf("shard manager not available")
	}
	return s.shardMgr.Spawn(ctx, specialistName, task)
}

// northstarHandlerAdapter adapts northstar.BackgroundEventHandler to shards.NorthstarHandler interface.
type northstarHandlerAdapter struct {
	handler *northstar.BackgroundEventHandler
}

func (a *northstarHandlerAdapter) HandleEvent(ctx context.Context, event shards.ObserverEvent) (*shards.ObserverAssessment, error) {
	// Convert shards event to northstar handler call
	assessment, err := a.handler.HandleEvent(ctx, string(event.Type), event.Source, event.Target, event.Details, event.Timestamp)
	if err != nil {
		return nil, err
	}
	if assessment == nil {
		return nil, nil
	}

	// Convert northstar assessment to shards assessment
	return &shards.ObserverAssessment{
		ObserverName: assessment.ObserverName,
		EventID:      assessment.EventID,
		Score:        assessment.Score,
		Level:        shards.AssessmentLevel(assessment.Level),
		VisionMatch:  assessment.VisionMatch,
		Deviations:   assessment.Deviations,
		Suggestions:  assessment.Suggestions,
		Metadata:     assessment.Metadata,
		Timestamp:    assessment.Timestamp,
	}, nil
}

func hydrateNerdState(workspace string, kernel *core.RealKernel, shardMgr *coreshards.ShardManager, initialMessages *[]Message) (*Session, *Preferences) {
	nerdDir := filepath.Join(workspace, ".nerd")

	// Load profile facts
	profilePath := filepath.Join(nerdDir, "profile.mg")
	if info, err := os.Stat(profilePath); err == nil && !info.IsDir() {
		if err := kernel.LoadFactsFromFile(profilePath); err != nil {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to load .nerd/profile.mg: %v", err),
				Time:    time.Now(),
			})
		}
	} else if err != nil && !os.IsNotExist(err) {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Unable to access .nerd/profile.mg: %v", err),
			Time:    time.Now(),
		})
	}

	// Load preferences
	var prefs *Preferences
	prefPath := filepath.Join(nerdDir, "preferences.json")
	if data, err := os.ReadFile(prefPath); err == nil {
		var p Preferences
		if err := json.Unmarshal(data, &p); err == nil {
			prefs = &p
		} else {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to parse .nerd/preferences.json: %v", err),
				Time:    time.Now(),
			})
		}
	}

	// QUIESCENT BOOT: Always start fresh sessions.
	// Previous sessions can be resumed explicitly via /sessions command.
	// This prevents stale state from affecting new sessions.
	var session *Session
	// Generate a new session ID for this boot
	newSessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	session = &Session{
		SessionID: newSessionID,
		StartedAt: time.Now().Format(time.RFC3339),
		TurnCount: 0,
	}
	logging.Session("Starting fresh session: %s", newSessionID)

	// Check if there are previous sessions to hint about
	if sessions, err := nerdinit.ListSessionHistories(workspace); err == nil && len(sessions) > 0 {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("*Fresh session started.* Use `/sessions` to load previous sessions (%d available).", len(sessions)),
			Time:    time.Now(),
		})
	}

	// Ensure .nerd/agents.json reflects any agents present under .nerd/agents/*.
	// This keeps the registry in sync even when agents are created/edited outside of /init.
	if err := nerdsystem.SyncAgentRegistryFromDisk(workspace); err != nil {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Warning: failed to sync agent registry from .nerd/agents: %v", err),
			Time:    time.Now(),
		})
	}

	// Load agents registry and hydrate shard profiles
	agentsPath := filepath.Join(nerdDir, "agents.json")
	if data, err := os.ReadFile(agentsPath); err == nil {
		var reg Registry
		if err := json.Unmarshal(data, &reg); err == nil {
			for _, agent := range reg.Agents {
				cfg := coreshards.DefaultSpecialistConfig(agent.Name, agent.KnowledgePath)
				if agent.Type != "" {
					cfg.Type = types.ShardType(agent.Type)
				}
				shardMgr.DefineProfile(agent.Name, cfg)
			}
		} else {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to parse .nerd/agents.json: %v", err),
				Time:    time.Now(),
			})
		}
	}

	return session, prefs
}

// hydrateAllTools loads all tools into the VirtualStore's tool registry.
// Sources:
// 0. Built-in modular tools (core, shell, codedom, research)
// 1. available_tools.json - Static language/framework tools from init
// 2. .compiled/ directory - Autopoiesis-generated tools
func hydrateAllTools(virtualStore *core.VirtualStore, nerdDir string) error {
	var warnings []string

	// 0. Register built-in modular tools (core, shell, codedom, research)
	if err := virtualStore.HydrateModularTools(); err != nil {
		warnings = append(warnings, fmt.Sprintf("modular tools: %v", err))
	}

	// 1. Load static tools from available_tools.json
	if toolDefs, err := nerdinit.LoadToolsFromFile(nerdDir); err == nil && len(toolDefs) > 0 {
		// Convert init.ToolDefinition to core.StaticToolDef
		staticDefs := make([]core.StaticToolDef, len(toolDefs))
		for i, td := range toolDefs {
			staticDefs[i] = core.StaticToolDef{
				Name:          td.Name,
				Category:      td.Category,
				Description:   td.Description,
				Command:       td.Command,
				ShardAffinity: td.ShardAffinity,
			}
		}
		if err := virtualStore.HydrateStaticTools(staticDefs); err != nil {
			warnings = append(warnings, fmt.Sprintf("static tools: %v", err))
		}
	} else if err != nil {
		warnings = append(warnings, fmt.Sprintf("load available_tools.json: %v", err))
	}

	// 2. Restore compiled tools from disk and sync from Ouroboros
	if err := virtualStore.HydrateToolsFromDisk(nerdDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("compiled tools: %v", err))
	}

	if len(warnings) > 0 {
		return fmt.Errorf("%d issues: %s", len(warnings), strings.Join(warnings, "; "))
	}
	return nil
}

// saveSessionState saves the current session state and history.
// Implements dual persistence: JSON files + SQLite for redundancy and queryability.
func (m *Model) saveSessionState() {
	if m.workspace == "" || m.sessionID == "" {
		logging.Session("saveSessionState: early return - workspace=%q, sessionID=%q", m.workspace, m.sessionID)
		return
	}

	// Always persist session state/history for observability and continuity.
	// Session management should not require `/init` (which is about world-model wiring).
	nerdDir := filepath.Join(m.workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		logging.Get(logging.CategorySession).Error("saveSessionState: failed to create .nerd directory: %v", err)
		return
	}

	logging.Session("saveSessionState: saving session %s with %d messages, turnCount=%d", m.sessionID, len(m.history), m.turnCount)

	// Update session state
	state := &nerdinit.SessionState{
		SessionID:    m.sessionID,
		StartedAt:    time.Now(), // Will be overwritten if exists
		LastActiveAt: time.Now(),
		TurnCount:    m.turnCount,
		HistoryFile:  m.sessionID + ".json",
	}

	// Preserve original StartedAt if session exists
	if existing, err := nerdinit.LoadSessionState(m.workspace); err == nil {
		state.StartedAt = existing.StartedAt
	}

	// Save session state (JSON)
	if err := nerdinit.SaveSessionState(m.workspace, state); err != nil {
		logging.Get(logging.CategorySession).Error("Failed to save session state: %v", err)
	}

	// Convert and save conversation history (JSON)
	messages := make([]nerdinit.ChatMessage, len(m.history))
	for i, msg := range m.history {
		messages[i] = nerdinit.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	if err := nerdinit.SaveSessionHistory(m.workspace, m.sessionID, messages); err != nil {
		logging.Get(logging.CategorySession).Error("Failed to save session history: %v", err)
	} else {
		logging.Session("Successfully saved %d messages to %s.json", len(messages), m.sessionID)
	}

	// Persist semantic compression state (best-effort) so we can rehydrate infinite context.
	if m.localDB != nil && m.compressor != nil {
		state := m.compressor.GetState()
		if state != nil {
			if data, err := ctxcompress.MarshalCompressedState(state); err == nil {
				_ = m.localDB.StoreCompressedState(m.sessionID, state.TurnNumber, string(data), state.CompressionRatio)
			}
		}
	}

	// ==========================================================================
	// DUAL PERSISTENCE: Sync to SQLite (knowledge.db session_history table)
	// ==========================================================================
	// This enables Mangle queries against session history via virtual predicates
	if m.localDB != nil {
		m.syncSessionToSQLite()
	}
}

// hydrateCompressorForSession resets and rehydrates the semantic compressor state for a session.
// This enables infinite-context continuity across restarts and session switches.
func (m *Model) hydrateCompressorForSession(sessionID string) {
	if m.compressor == nil {
		return
	}

	// Always reset to avoid leaking state across sessions.
	m.compressor.Reset()
	m.compressor.SetSessionID(sessionID)

	// Always refresh budget at the end so status bar shows accurate context usage.
	defer m.compressor.RefreshBudget()

	if m.localDB == nil {
		return
	}

	stateJSON, turnNumber, _, err := m.localDB.LoadLatestCompressedState(sessionID)
	if err != nil || strings.TrimSpace(stateJSON) == "" {
		return
	}

	state, err := ctxcompress.UnmarshalCompressedState([]byte(stateJSON))
	if err != nil {
		logging.Session("hydrateCompressorForSession: failed to parse compressed state: %v", err)
		return
	}

	if err := m.compressor.LoadState(state); err != nil {
		logging.Session("hydrateCompressorForSession: failed to load compressed state: %v", err)
		return
	}

	// Keep turn counter monotonic if compressed state is ahead.
	if turnNumber > m.turnCount {
		m.turnCount = turnNumber
	}
}

// syncSessionToSQLite syncs conversation history to knowledge.db for query access.
// Uses turn-based storage to avoid duplicates (SQLite table has unique constraint).
func (m *Model) syncSessionToSQLite() {
	if m.localDB == nil || len(m.history) == 0 {
		return
	}

	// Process message pairs (user + assistant = 1 turn)
	// History format: [user1, asst1, user2, asst2, ...]
	for i := 0; i < len(m.history)-1; i += 2 {
		userMsg := m.history[i]
		asstMsg := m.history[i+1]

		// Skip if not a proper user-assistant pair
		if userMsg.Role != "user" || asstMsg.Role != "assistant" {
			continue
		}

		turnNumber := i / 2

		// Store to SQLite (StoreSessionTurn handles duplicates gracefully)
		// Intent and atoms JSON are empty for now - can be populated by OODA loop
		err := m.localDB.StoreSessionTurn(
			m.sessionID,
			turnNumber,
			userMsg.Content,
			"{}", // intent_json placeholder
			asstMsg.Content,
			"[]", // atoms_json placeholder
		)
		if err != nil {
			// Log but don't fail - JSON is the primary store
			// Duplicate key errors are expected and harmless
			continue
		}
	}
}

// loadSelectedSession loads a session from the sessions list and switches to it.
// Saves the current session first, then loads the selected session's history.
func (m Model) loadSelectedSession(sessionID string) (tea.Model, tea.Cmd) {
	// Save current session before switching
	m.saveSessionState()

	// Load the selected session's history
	history, err := nerdinit.LoadSessionHistory(m.workspace, sessionID)
	if err != nil {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Failed to load session: %v", err),
			Time:    time.Now(),
		})
		m.viewMode = ChatView
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	// Switch to the selected session
	m.sessionID = sessionID
	m.history = make([]Message, len(history.Messages))
	for i, msg := range history.Messages {
		m.history[i] = Message{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	m.turnCount = len(m.history) / 2 // Approximate turn count from history

	// Rehydrate compressor state for this session (if available).
	m.hydrateCompressorForSession(sessionID)

	// Update session.json to point to this session
	state := &nerdinit.SessionState{
		SessionID:    sessionID,
		StartedAt:    history.CreatedAt,
		LastActiveAt: time.Now(),
		TurnCount:    m.turnCount,
		HistoryFile:  sessionID + ".json",
	}
	_ = nerdinit.SaveSessionState(m.workspace, state)

	// Add a system message indicating session switch
	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: fmt.Sprintf("*Loaded session: `%s` (%d messages)*", sessionID, len(history.Messages)),
		Time:    time.Now(),
	})

	// Switch back to chat view
	m.viewMode = ChatView
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	return m, nil
}

// MigrateOldSessionsToSQLite migrates all existing JSON session files to SQLite.
// This enables querying historical sessions via virtual predicates.
// Safe to call multiple times - uses INSERT OR IGNORE for idempotency.
func MigrateOldSessionsToSQLite(workspace string, localDB *store.LocalStore) (int, error) {
	if localDB == nil {
		return 0, nil
	}

	// List all session JSON files
	sessionIDs, err := nerdinit.ListSessionHistories(workspace)
	if err != nil {
		return 0, err
	}

	migratedTurns := 0

	for _, sessionID := range sessionIDs {
		history, err := nerdinit.LoadSessionHistory(workspace, sessionID)
		if err != nil {
			continue // Skip corrupted sessions
		}

		// Process message pairs
		for i := 0; i < len(history.Messages)-1; i += 2 {
			userMsg := history.Messages[i]
			asstMsg := history.Messages[i+1]

			if userMsg.Role != "user" || asstMsg.Role != "assistant" {
				continue
			}

			turnNumber := i / 2

			err := localDB.StoreSessionTurn(
				sessionID,
				turnNumber,
				userMsg.Content,
				"{}",
				asstMsg.Content,
				"[]",
			)
			if err == nil {
				migratedTurns++
			}
		}
	}

	return migratedTurns, nil
}

func persistAgentProfile(workspace, name, agentType, knowledgePath string, kbSize int, status string) error {
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(filepath.Join(nerdDir, "shards"), 0755); err != nil {
		return err
	}

	agentsPath := filepath.Join(nerdDir, "agents.json")
	reg := Registry{
		Version:   "1.0",
		CreatedAt: time.Now().Format(time.RFC3339),
		Agents:    []Agent{},
	}

	if data, err := os.ReadFile(agentsPath); err == nil {
		_ = json.Unmarshal(data, &reg)
	}

	// Upsert
	found := false
	for i, a := range reg.Agents {
		if strings.EqualFold(a.Name, name) {
			reg.Agents[i].Type = agentType
			reg.Agents[i].KnowledgePath = knowledgePath
			reg.Agents[i].KBSize = kbSize
			reg.Agents[i].Status = status
			found = true
			break
		}
	}
	if !found {
		reg.Agents = append(reg.Agents, Agent{
			Name:          name,
			Type:          agentType,
			KnowledgePath: knowledgePath,
			KBSize:        kbSize,
			Status:        status,
		})
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(agentsPath, data, 0644)
}

func resolveSessionID(session *Session) string {
	if session != nil && strings.TrimSpace(session.SessionID) != "" {
		return session.SessionID
	}
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func resolveTurnCount(session *Session) int {
	if session != nil && session.TurnCount > 0 {
		return session.TurnCount
	}
	return 0
}

// =============================================================================
// TRACE STORE ADAPTER
// =============================================================================
// Adapts store.LocalStore to implement perception.TraceStore interface for
// reasoning trace persistence.

// LocalStoreTraceAdapter wraps LocalStore to implement perception.TraceStore.
type LocalStoreTraceAdapter struct {
	store *store.LocalStore
}

// NewLocalStoreTraceAdapter creates a new trace store adapter.
func NewLocalStoreTraceAdapter(s *store.LocalStore) *LocalStoreTraceAdapter {
	return &LocalStoreTraceAdapter{store: s}
}

// StoreReasoningTrace implements perception.TraceStore.
// Note: perception.TraceStore expects StoreReasoningTrace(*ReasoningTrace)
// but store.LocalStore.StoreReasoningTrace takes interface{}.
func (a *LocalStoreTraceAdapter) StoreReasoningTrace(trace *perception.ReasoningTrace) error {
	if a.store == nil || trace == nil {
		return nil
	}
	// Pass the trace directly - LocalStore accepts interface{} and handles conversion
	return a.store.StoreReasoningTrace(trace)
}

// =============================================================================
// LEARNING STORE ADAPTER (GAP-001 FIX)
// =============================================================================
// Adapts store.LearningStore to implement core.LearningStore interface for
// shard autopoiesis.

// coreLearningStoreAdapter wraps store.LearningStore to implement core.LearningStore.
type coreLearningStoreAdapter struct {
	store *store.LearningStore
}

func (a *coreLearningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	if a.store == nil {
		return nil
	}
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *coreLearningStoreAdapter) Load(shardType string) ([]types.ShardLearning, error) {
	if a.store == nil {
		return nil, nil
	}
	// store.LearningStore.Load already returns []types.ShardLearning
	return a.store.Load(shardType)
}

func (a *coreLearningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	if a.store == nil {
		return nil, nil
	}
	// store.LearningStore.LoadByPredicate already returns []types.ShardLearning
	return a.store.LoadByPredicate(shardType, predicate)
}

func (a *coreLearningStoreAdapter) DecayConfidence(shardType string, decayFactor float64) error {
	if a.store == nil {
		return nil
	}
	return a.store.DecayConfidence(shardType, decayFactor)
}

func (a *coreLearningStoreAdapter) Close() error {
	if a.store == nil {
		return nil
	}
	return a.store.Close()
}

// =============================================================================
// SESSION ADAPTERS (Clean Loop Architecture)
// =============================================================================
// These adapters bridge core.* types to types.* interfaces required by
// the session.Executor and session.Spawner.

// sessionKernelAdapter adapts *core.RealKernel to types.Kernel.
type sessionKernelAdapter struct {
	kernel *core.RealKernel
}

func (a *sessionKernelAdapter) LoadFacts(facts []types.Fact) error {
	return a.kernel.LoadFacts(facts)
}

func (a *sessionKernelAdapter) Query(predicate string) ([]types.Fact, error) {
	return a.kernel.Query(predicate)
}

func (a *sessionKernelAdapter) QueryAll() (map[string][]types.Fact, error) {
	return a.kernel.QueryAll()
}

func (a *sessionKernelAdapter) Assert(fact types.Fact) error {
	return a.kernel.Assert(fact)
}

func (a *sessionKernelAdapter) Retract(predicate string) error {
	return a.kernel.Retract(predicate)
}

func (a *sessionKernelAdapter) RetractFact(fact types.Fact) error {
	return a.kernel.RetractFact(fact)
}

func (a *sessionKernelAdapter) UpdateSystemFacts() error {
	return a.kernel.UpdateSystemFacts()
}

func (a *sessionKernelAdapter) Reset() {
	a.kernel.Reset()
}

func (a *sessionKernelAdapter) AppendPolicy(policy string) {
	a.kernel.AppendPolicy(policy)
}

func (a *sessionKernelAdapter) RetractExactFactsBatch(facts []types.Fact) error {
	return a.kernel.RetractExactFactsBatch(facts)
}

func (a *sessionKernelAdapter) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return a.kernel.RemoveFactsByPredicateSet(predicates)
}

// sessionVirtualStoreAdapter adapts *core.VirtualStore to types.VirtualStore.
// NOTE: VirtualStore doesn't directly expose these methods yet.
// The executor's tool execution is TODO and will route through VirtualStore.
// For now, this adapter provides stub implementations.
type sessionVirtualStoreAdapter struct {
	vs *core.VirtualStore
}

func (a *sessionVirtualStoreAdapter) ReadFile(path string) ([]string, error) {
	// TODO: Route through VirtualStore's FileEditor when wired
	// For now, use os.ReadFile directly as a fallback
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func (a *sessionVirtualStoreAdapter) WriteFile(path string, content []string) error {
	// TODO: Route through VirtualStore's FileEditor when wired
	return os.WriteFile(path, []byte(strings.Join(content, "\n")), 0644)
}

func (a *sessionVirtualStoreAdapter) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	// TODO: Route through VirtualStore's executor when wired
	// For now, return an error indicating the method is not yet wired
	return "", "", fmt.Errorf("exec not yet wired through VirtualStore")
}

// sessionLLMAdapter adapts perception.LLMClient to types.LLMClient.
type sessionLLMAdapter struct {
	client perception.LLMClient
}

func (a *sessionLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	return a.client.Complete(ctx, prompt)
}

func (a *sessionLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (a *sessionLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return a.client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}
