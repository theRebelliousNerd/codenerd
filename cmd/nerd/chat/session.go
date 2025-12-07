// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains session management, initialization, and state persistence.
package chat

import (
	"codenerd/cmd/nerd/config"
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
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
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
)

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================
// Functions for initializing the chat, loading/saving session state, and
// managing persistent configuration.

// InitChat initializes the interactive chat model
func InitChat(cfg Config) Model {
	// Load configuration
	appCfg, _ := config.Load()

	initialMessages := []Message{}

	// Initialize styles
	styles := ui.DefaultStyles()
	if appCfg.Theme == "dark" {
		styles = ui.NewStyles(ui.DarkTheme())
	}

	// Initialize textinput for input
	ti := textinput.New()
	ti.Placeholder = "Ask me anything... (Enter to send, Ctrl+C to exit)"
	ti.Focus()
	ti.Prompt = "| "
	ti.CharLimit = 4096
	ti.Width = 80
	ti.PromptStyle = styles.Prompt
	ti.TextStyle = styles.UserInput

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	// Initialize viewport for chat history
	vp := viewport.New(80, 20)
	vp.SetContent("")

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

	// Resolve API key
	apiKey := os.Getenv("ZAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = appCfg.APIKey
	}
	if apiKey == "" {
		initialMessages = append(initialMessages, Message{
			Role:    "assistant",
			Content: "No API key detected. Set `ZAI_API_KEY` or `GEMINI_API_KEY`, or run `/config set-key <key>` for best results.",
			Time:    time.Now(),
		})
	}

	// Initialize backend components
	baseLLMClient := perception.NewZAIClient(apiKey)
	transducer := perception.NewRealTransducer(baseLLMClient)
	kernel := core.NewRealKernel()
	executor := tactile.NewSafeExecutor()
	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)

	// Note: LLM client will be set after TracingLLMClient is created below
	// We need localDB first for the tracing store

	// Register Shard Factories (External Injection)
	// Each shard gets its own kernel, VirtualStore, and LLM client injected
	virtualStore := core.NewVirtualStore(executor)

	// Initialize local knowledge database for research persistence
	// This enables knowledge atoms to persist across sessions
	var localDB *store.LocalStore
	knowledgeDBPath := filepath.Join(workspace, ".nerd", "knowledge.db")
	if db, err := store.NewLocalStore(knowledgeDBPath); err == nil {
		localDB = db
	}

	// Initialize embedding engine from config
	// Supports Ollama (local) and GenAI (cloud) backends
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
				Content: fmt.Sprintf("⚠ Embedding init failed: %v (using keyword search)", err),
				Time:    time.Now(),
			})
		}
	}
	// Suppress unused variable warning - embeddingEngine will be used for shard DBs
	_ = embeddingEngine

	// Wire LocalDB to VirtualStore for virtual predicate queries
	// This enables Mangle rules to query knowledge.db via VirtualStore FFI
	if localDB != nil {
		virtualStore.SetLocalDB(localDB)
		virtualStore.SetKernel(kernel)

		// WIRE TAXONOMY ENGINE TO DB (Persistence & Rehydration)
		taxStore := perception.NewTaxonomyStore(localDB)
		perception.SharedTaxonomy.SetStore(taxStore)

		// 1. Ensure DB is populated with defaults if empty
		if err := perception.SharedTaxonomy.EnsureDefaults(); err != nil {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ Taxonomy defaults init failed: %v", err),
				Time:    time.Now(),
			})
		}

		// 2. Rehydrate engine from DB (loads learned rules too)
		if err := perception.SharedTaxonomy.HydrateFromDB(); err != nil {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⚠ Taxonomy rehydration failed: %v", err),
				Time:    time.Now(),
			})
		} else {
			// Optional: Confirm success
			// initialMessages = append(initialMessages, Message{
			// 	Role:    "assistant",
			// 	Content: "✓ Taxonomy rehydrated from knowledge.db",
			// 	Time:    time.Now(),
			// })
		}

		// Migrate old JSON sessions to SQLite for query access
		// Safe to call multiple times - uses INSERT OR IGNORE
		if migratedTurns, err := MigrateOldSessionsToSQLite(workspace, localDB); err == nil && migratedTurns > 0 {
			initialMessages = append(initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("✓ Migrated %d session turns to SQLite", migratedTurns),
				Time:    time.Now(),
			})
		}
	}

	// ==========================================================================
	// REASONING TRACE CAPTURE (Task 4)
	// ==========================================================================
	// Wrap the LLM client with TracingLLMClient to capture all shard reasoning.
	// Traces are stored in SQLite and made available to shards for self-learning.
	var llmClient perception.LLMClient = baseLLMClient
	if localDB != nil {
		// Create trace store adapter that implements perception.TraceStore
		traceStore := NewLocalStoreTraceAdapter(localDB)
		tracingClient := perception.NewTracingLLMClient(baseLLMClient, traceStore)
		llmClient = tracingClient

		// Wire tracing client to ShardManager for context-aware trace attribution
		shardMgr.SetLLMClient(tracingClient)

		initialMessages = append(initialMessages, Message{
			Role:    "assistant",
			Content: "✓ Reasoning trace capture enabled",
			Time:    time.Now(),
		})
	} else {
		// Fallback: no tracing, use base client directly
		shardMgr.SetLLMClient(baseLLMClient)
	}

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
		// Provide workspace root so local dependency queries avoid external calls
		shard.SetWorkspaceRoot(workspace)
		// Set Context7 API key from config or environment
		context7Key := appCfg.Context7APIKey
		if context7Key == "" {
			context7Key = os.Getenv("CONTEXT7_API_KEY")
		}
		if context7Key != "" {
			shard.SetContext7APIKey(context7Key)
		}
		return shard
	})

	// =========================================================================
	// Type 1: System Shards (Permanent, Continuous)
	// =========================================================================
	// System shards form the OODA loop and run continuously in the background.
	// They require dependency injection for kernel, LLM client, and virtual store.

	// Perception Firewall - AUTO-START, LLM-primary (NL → atoms transduction)
	shardMgr.RegisterShard("perception_firewall", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewPerceptionFirewallShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient)
		return shard
	})

	// World Model Ingestor - ON-DEMAND, Hybrid (file_topology, symbol_graph)
	shardMgr.RegisterShard("world_model_ingestor", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewWorldModelIngestorShard()
		shard.SetParentKernel(kernel)
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		return shard
	})

	// Executive Policy - AUTO-START, Logic-primary (next_action derivation)
	shardMgr.RegisterShard("executive_policy", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewExecutivePolicyShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient) // For autopoiesis edge cases
		return shard
	})

	// Constitution Gate - AUTO-START, Logic-primary (safety enforcement)
	shardMgr.RegisterShard("constitution_gate", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewConstitutionGateShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient) // For autopoiesis rule proposals
		return shard
	})

	// Tactile Router - ON-DEMAND, Logic-primary (action → tool routing)
	shardMgr.RegisterShard("tactile_router", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(kernel)
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient) // For autopoiesis routing gaps
		return shard
	})

	// Session Planner - ON-DEMAND, LLM-primary (goal decomposition)
	shardMgr.RegisterShard("session_planner", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewSessionPlannerShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient)
		return shard
	})

	// Define system shard profiles (configurations)
	shards.RegisterSystemShardProfiles(shardMgr)

	ctx := context.Background()
	disabled := make(map[string]struct{})
	for _, name := range cfg.DisableSystemShards {
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
	shadowMode := core.NewShadowMode(kernel)
	emitter := articulation.NewEmitter()
	scanner := world.NewScanner()

	// Initialize Semantic Compression (§8.2)
	ctxCfg := appCfg.GetContextWindowConfig()
	compressor := ctxcompress.NewCompressorWithParams(
		kernel, localDB, llmClient,
		ctxCfg.MaxTokens,
		ctxCfg.CoreReservePercent, ctxCfg.AtomReservePercent,
		ctxCfg.HistoryReservePercent, ctxCfg.WorkingReservePercent,
		ctxCfg.RecentTurnWindow,
		ctxCfg.CompressionThreshold, ctxCfg.TargetCompressionRatio, ctxCfg.ActivationThreshold,
	)

	// Initialize Autopoiesis (§8.3) - Self-Modification Capabilities
	autopoiesisConfig := autopoiesis.DefaultConfig(workspace)
	autopoiesisOrch := autopoiesis.NewOrchestrator(llmClient, autopoiesisConfig)

	// Wire kernel to autopoiesis for logic-driven orchestration
	kernelAdapter := core.NewKernelAdapter(kernel)
	autopoiesisOrch.SetKernel(kernelAdapter)

	// Start kernel listener for delegate_task(/tool_generator, ...) facts
	// This enables campaign orchestration to trigger tool generation via Mangle policy
	autopoiesisCtx, autopoiesisCancel := context.WithCancel(context.Background())
	autopoiesisListenerCh := autopoiesisOrch.StartKernelListener(autopoiesisCtx, 2*time.Second)

	// Initialize Verification Loop (Quality-Enforcing)
	// This ensures tasks are completed PROPERLY with automatic retry and corrective action
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

	// Wire tool executor to VirtualStore for shard access to generated tools
	// This enables the Ouroboros Loop's generated tools to be executed via VirtualStore
	toolExecutor := NewToolExecutorAdapter(autopoiesisOrch)
	virtualStore.SetToolExecutor(toolExecutor)

	loadedSession, _ := hydrateNerdState(workspace, kernel, shardMgr, &initialMessages)

	// Initialize split-pane view (Glass Box Interface)
	splitPaneView := ui.NewSplitPaneView(styles, 80, 24)
	logicPane := ui.NewLogicPane(styles, 30, 20)

	// Preload workspace facts from .nerd/profile.mg if present
	// (Already done in hydrateNerdState)

	model := Model{
		textinput:             ti,
		viewport:              vp,
		spinner:               sp,
		styles:                styles,
		renderer:              renderer,
		splitPane:             &splitPaneView,
		logicPane:             &logicPane,
		showLogic:             false,
		paneMode:              ui.ModeSinglePane,
		history:               []Message{},
		Config:                appCfg,
		client:                llmClient,
		kernel:                kernel,
		shardMgr:              shardMgr,
		shadowMode:            shadowMode,
		transducer:            transducer,
		executor:              executor,
		emitter:               emitter,
		virtualStore:          virtualStore,
		scanner:               scanner,
		workspace:             workspace,
		sessionID:             resolveSessionID(loadedSession),
		turnCount:             resolveTurnCount(loadedSession),
		awaitingClarification: false,
		selectedOption:        0,
		localDB:               localDB,
		compressor:            compressor,
		autopoiesis:           autopoiesisOrch,
		autopoiesisCancel:     autopoiesisCancel,
		autopoiesisListenerCh: autopoiesisListenerCh,
		verifier:              taskVerifier,
	}

	if len(initialMessages) > 0 {
		model.history = append(model.history, initialMessages...)
		model.viewport.SetContent(model.renderHistory())
	}

	return model
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

// saveSessionState saves the current session state and history.
// Implements dual persistence: JSON files + SQLite for redundancy and queryability.
func (m *Model) saveSessionState() {
	if m.workspace == "" || m.sessionID == "" {
		return
	}

	// Only save if initialized
	if !nerdinit.IsInitialized(m.workspace) {
		return
	}

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
	_ = nerdinit.SaveSessionState(m.workspace, state)

	// Convert and save conversation history (JSON)
	messages := make([]nerdinit.ChatMessage, len(m.history))
	for i, msg := range m.history {
		messages[i] = nerdinit.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	_ = nerdinit.SaveSessionHistory(m.workspace, m.sessionID, messages)

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
