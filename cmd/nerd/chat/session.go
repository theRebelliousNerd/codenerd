// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains session management, initialization, and state persistence.
package chat

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	"codenerd/internal/browser"
	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/shards"
	"codenerd/internal/shards/coder"
	"codenerd/internal/shards/researcher"
	"codenerd/internal/shards/reviewer"
	"codenerd/internal/shards/system"
	"codenerd/internal/shards/tester"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/usage"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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

	// Return the model in "Booting" state
	return Model{
		textarea:     ta,
		viewport:     vp,
		spinner:      sp,
		list:         l,
		filepicker:   fp,
		styles:       styles,
		renderer:     renderer,
		usageTracker: tracker,
		usagePage:    ui.NewUsagePageModel(tracker, styles),
		splitPane:    &splitPaneView,
		logicPane:    splitPaneView.RightPane,
		showLogic:    false,
		paneMode:     ui.ModeSinglePane,
		history:      []Message{},
		Config:       appCfg,
		// Backend components start nil
		kernel:              nil,
		shardMgr:            nil,
		client:              nil,  // Will be set in boot
		isBooting:           true, // Flag for UI
		statusChan:          make(chan string, 10),
		workspace:           workspace,
		DisableSystemShards: cfg.DisableSystemShards,
		// Mouse capture enabled by default (Alt+M to toggle for text selection)
		mouseEnabled: true,
		// Shutdown coordination
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}
}

// performSystemBoot performs the heavy backend initialization in a background thread
func performSystemBoot(cfg *config.UserConfig, disableSystemShards []string, workspace string) tea.Cmd {
	return func() tea.Msg {
		bootStart := time.Now()
		log := func(step string) {
			fmt.Printf("\r\033[K[boot] %s (%.1fs)", step, time.Since(bootStart).Seconds())
		}

		log("Loading config...")
		// Use the passed-in config or reload from disk
		appCfg := cfg
		if appCfg == nil {
			appCfg, _ = config.GlobalConfig()
			if appCfg == nil {
				appCfg = config.DefaultUserConfig()
			}
		}
		initialMessages := []Message{}

		// Initialize LLM client using the perception package's provider detection
		// This supports all providers: zai, anthropic, openai, gemini, xai, openrouter
		// Configuration is read from .nerd/config.json or environment variables
		log("Detecting LLM provider...")
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

		// Initialize backend components
		log("Creating transducer...")
		transducer := perception.NewRealTransducer(baseLLMClient)

		// HEAVY OPERATION: NewRealKernel calls Evaluate() internally?
		// We verified NewRealKernel calls evaluate().
		log("Booting Mangle kernel...")
		kernel := core.NewRealKernel()

		// If NewRealKernel didn't error (it returns *RealKernel), we check if it's usable.
		// Actually NewRealKernel swallows errors?
		// Let's assume it initializes generic state.
		// But we should explicitely Evaluate if needed or trust NewRealKernel.
		// The original code called kernel.Evaluate() explicitly.
		log("Evaluating kernel rules...")
		if err := kernel.Evaluate(); err != nil {
			return bootCompleteMsg{err: fmt.Errorf("kernel boot failed: %w", err)}
		}

		log("Creating executor & shard manager...")
		executor := tactile.NewSafeExecutor()
		shardMgr := core.NewShardManager()
		shardMgr.SetParentKernel(kernel)

		// Browser Manager is created on-demand when needed (not at boot)
		// This avoids spawning Chrome during normal TUI usage
		var browserMgr *browser.SessionManager // nil until needed
		var browserCtxCancel context.CancelFunc

		log("Creating virtual store...")
		virtualStore := core.NewVirtualStore(executor)

		log("Opening knowledge database...")
		var localDB *store.LocalStore
		knowledgeDBPath := filepath.Join(workspace, ".nerd", "knowledge.db")
		if db, err := store.NewLocalStore(knowledgeDBPath); err == nil {
			localDB = db
		}

		// Initialize embedding engine
		log("Initializing embedding engine...")
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

		if localDB != nil {
			log("Wiring virtual store...")
			virtualStore.SetLocalDB(localDB)
			virtualStore.SetKernel(kernel)

			log("Initializing taxonomy store...")
			taxStore := perception.NewTaxonomyStore(localDB)
			perception.SharedTaxonomy.SetStore(taxStore)

			if err := perception.SharedTaxonomy.EnsureDefaults(); err != nil {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("⚠ Taxonomy defaults init failed: %v", err),
					Time:    time.Now(),
				})
			}

			// HEAVY OPERATION: Rehydration
			log("Hydrating taxonomy from DB...")
			if err := perception.SharedTaxonomy.HydrateFromDB(); err != nil {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("⚠ Taxonomy rehydration failed: %v", err),
					Time:    time.Now(),
				})
			}

			log("Migrating old sessions...")
			if migratedTurns, err := MigrateOldSessionsToSQLite(workspace, localDB); err == nil && migratedTurns > 0 {
				initialMessages = append(initialMessages, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Migrated %d session turns to SQLite", migratedTurns),
					Time:    time.Now(),
				})
			}
		}

		log("Configuring LLM client...")
		var llmClient perception.LLMClient = baseLLMClient
		if localDB != nil {
			traceStore := NewLocalStoreTraceAdapter(localDB)
			tracingClient := perception.NewTracingLLMClient(baseLLMClient, traceStore)
			llmClient = tracingClient
			shardMgr.SetLLMClient(tracingClient)
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: "✓ Reasoning trace capture enabled",
				Time:    time.Now(),
			})
		} else {
			shardMgr.SetLLMClient(baseLLMClient)
		}

		log("Registering shard types...")
		shardMgr.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := coder.NewCoderShard()
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("reviewer", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := reviewer.NewReviewerShard()
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("tester", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := tester.NewTesterShard()
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("researcher", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := researcher.NewResearcherShard()
			shard.SetLLMClient(llmClient)
			if localDB != nil {
				shard.SetLocalDB(localDB)
			}
			shard.SetWorkspaceRoot(workspace)
			context7Key := appCfg.Context7APIKey
			if context7Key == "" {
				context7Key = os.Getenv("CONTEXT7_API_KEY")
			}
			if context7Key != "" {
				shard.SetContext7APIKey(context7Key)
			}
			return shard
		})

		// System Shards
		shardMgr.RegisterShard("perception_firewall", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := system.NewPerceptionFirewallShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("world_model_ingestor", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := system.NewWorldModelIngestorShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("executive_policy", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := system.NewExecutivePolicyShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("constitution_gate", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := system.NewConstitutionGateShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			return shard
		})
		shardMgr.RegisterShard("tactile_router", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := system.NewTactileRouterShard()
			shard.SetParentKernel(kernel)
			shard.SetVirtualStore(virtualStore)
			shard.SetLLMClient(llmClient)
			if browserMgr != nil {
				shard.SetBrowserManager(browserMgr)
			}
			return shard
		})
		shardMgr.RegisterShard("session_planner", func(id string, config core.ShardConfig) core.ShardAgent {
			shard := system.NewSessionPlannerShard()
			shard.SetParentKernel(kernel)
			shard.SetLLMClient(llmClient)
			return shard
		})

		shards.RegisterSystemShardProfiles(shardMgr)

		// HEAVY OPERATION: Start System Shards (Async but setup overhead)
		log("Starting system shards...")
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

		log("Creating shadow mode & scanner...")
		shadowMode := core.NewShadowMode(kernel)
		emitter := articulation.NewEmitter()
		scanner := world.NewScanner()

		log("Initializing context compressor...")
		ctxCfg := appCfg.GetContextWindowConfig()
		compressor := ctxcompress.NewCompressorWithParams(
			kernel, localDB, llmClient,
			ctxCfg.MaxTokens,
			ctxCfg.CoreReservePercent, ctxCfg.AtomReservePercent,
			ctxCfg.HistoryReservePercent, ctxCfg.WorkingReservePercent,
			ctxCfg.RecentTurnWindow,
			ctxCfg.CompressionThreshold, ctxCfg.TargetCompressionRatio, ctxCfg.ActivationThreshold,
		)

		log("Starting autopoiesis orchestrator...")
		autopoiesisConfig := autopoiesis.DefaultConfig(workspace)
		autopoiesisOrch := autopoiesis.NewOrchestrator(llmClient, autopoiesisConfig)
		kernelAdapter := core.NewAutopoiesisBridge(kernel)
		autopoiesisOrch.SetKernel(kernelAdapter)

		autopoiesisCtx, autopoiesisCancel := context.WithCancel(context.Background())
		autopoiesisListenerCh := autopoiesisOrch.StartKernelListener(autopoiesisCtx, 2*time.Second)

		log("Creating task verifier...")
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

		toolExecutor := NewToolExecutorAdapter(autopoiesisOrch)
		virtualStore.SetToolExecutor(toolExecutor)

		// Hydrate tools from disk and available_tools.json
		log("Hydrating tools from .nerd/tools/...")
		toolsNerdDir := filepath.Join(workspace, ".nerd")
		if err := hydrateAllTools(virtualStore, toolsNerdDir); err != nil {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ Tool hydration warning: %v", err),
				Time:    time.Now(),
			})
		}

		log("Hydrating session state...")
		loadedSession, _ := hydrateNerdState(workspace, kernel, shardMgr, &initialMessages)

		fmt.Printf("\r\033[K[boot] Complete! (%.1fs)\n", time.Since(bootStart).Seconds())
		return bootCompleteMsg{
			components: &SystemComponents{
				Kernel:                kernel,
				ShardMgr:              shardMgr,
				ShadowMode:            shadowMode,
				Transducer:            transducer,
				Executor:              executor,
				Emitter:               emitter,
				VirtualStore:          virtualStore,
				Scanner:               scanner,
				Workspace:             workspace,
				SessionID:             resolveSessionID(loadedSession),
				TurnCount:             resolveTurnCount(loadedSession),
				LocalDB:               localDB,
				Compressor:            compressor,
				Autopoiesis:           autopoiesisOrch,
				AutopoiesisCancel:     autopoiesisCancel,
				AutopoiesisListenerCh: autopoiesisListenerCh,
				Verifier:              taskVerifier,
				InitialMessages:       initialMessages,
				Client:                llmClient,
				BrowserManager:        browserMgr,
				BrowserCtxCancel:      browserCtxCancel,
			},
		}
	}
}

func hydrateNerdState(workspace string, kernel *core.RealKernel, shardMgr *core.ShardManager, initialMessages *[]Message) (*Session, *Preferences) {
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

	// Load session info
	var session *Session
	sessionPath := filepath.Join(nerdDir, "session.json")
	if data, err := os.ReadFile(sessionPath); err == nil {
		var s Session
		if err := json.Unmarshal(data, &s); err == nil {
			session = &s

			// Load conversation history for this session
			if session.SessionID != "" {
				if history, err := nerdinit.LoadSessionHistory(workspace, session.SessionID); err == nil {
					// Convert and prepend history to initialMessages
					for _, msg := range history.Messages {
						*initialMessages = append(*initialMessages, Message{
							Role:    msg.Role,
							Content: msg.Content,
							Time:    msg.Time,
						})
					}
					if len(history.Messages) > 0 {
						*initialMessages = append(*initialMessages, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("*Restored %d messages from previous session*", len(history.Messages)),
							Time:    time.Now(),
						})
					}
				}
			}
		} else {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to parse .nerd/session.json: %v", err),
				Time:    time.Now(),
			})
		}
	}

	// Load agents registry and hydrate shard profiles
	agentsPath := filepath.Join(nerdDir, "agents.json")
	if data, err := os.ReadFile(agentsPath); err == nil {
		var reg Registry
		if err := json.Unmarshal(data, &reg); err == nil {
			for _, agent := range reg.Agents {
				cfg := core.DefaultSpecialistConfig(agent.Name, agent.KnowledgePath)
				if agent.Type != "" {
					cfg.Type = core.ShardType(agent.Type)
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
// 1. available_tools.json - Static language/framework tools from init
// 2. .compiled/ directory - Autopoiesis-generated tools
func hydrateAllTools(virtualStore *core.VirtualStore, nerdDir string) error {
	var warnings []string

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
		log.Printf("[SESSION] saveSessionState: early return - workspace=%q, sessionID=%q", m.workspace, m.sessionID)
		return
	}

	// Only save if initialized
	if !nerdinit.IsInitialized(m.workspace) {
		log.Printf("[SESSION] saveSessionState: workspace not initialized")
		return
	}

	log.Printf("[SESSION] saveSessionState: saving session %s with %d messages, turnCount=%d", m.sessionID, len(m.history), m.turnCount)

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
		log.Printf("[SESSION] ERROR saving session state: %v", err)
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
		log.Printf("[SESSION] ERROR saving session history: %v", err)
	} else {
		log.Printf("[SESSION] Successfully saved %d messages to %s.json", len(messages), m.sessionID)
	}

	// ==========================================================================
	// DUAL PERSISTENCE: Sync to SQLite (knowledge.db session_history table)
	// ==========================================================================
	// This enables Mangle queries against session history via virtual predicates
	if m.localDB != nil {
		m.syncSessionToSQLite()
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
