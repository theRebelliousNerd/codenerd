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
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/shards"
	"codenerd/internal/shards/researcher"
	"codenerd/internal/shards/system"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
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
	llmClient := perception.NewZAIClient(apiKey)
	transducer := perception.NewRealTransducer(llmClient)
	kernel := core.NewRealKernel()
	executor := tactile.NewSafeExecutor()
	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)
	shardMgr.SetLLMClient(llmClient)

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

	shardMgr.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := shards.NewCoderShard()
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		return shard
	})
	shardMgr.RegisterShard("reviewer", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := shards.NewReviewerShard()
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		return shard
	})
	shardMgr.RegisterShard("tester", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := shards.NewTesterShard()
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

	// Wire tool executor to VirtualStore for shard access to generated tools
	// This enables the Ouroboros Loop's generated tools to be executed via VirtualStore
	toolExecutor := NewToolExecutorAdapter(autopoiesisOrch)
	virtualStore.SetToolExecutor(toolExecutor)

	loadedSession, _ := hydrateNerdState(workspace, kernel, shardMgr, &initialMessages)

	// Initialize split-pane view (Glass Box Interface)
	splitPaneView := ui.NewSplitPaneView(styles, 80, 24)
	logicPane := ui.NewLogicPane(styles, 30, 20)

	// Preload workspace facts from .nerd/profile.gl if present
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
		scanner:               scanner,
		workspace:             workspace,
		sessionID:             resolveSessionID(loadedSession),
		turnCount:             resolveTurnCount(loadedSession),
		awaitingClarification: false,
		selectedOption:        0,
		localDB:               localDB,
		compressor:            compressor,
		autopoiesis:           autopoiesisOrch,
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
	profilePath := filepath.Join(nerdDir, "profile.gl")
	if info, err := os.Stat(profilePath); err == nil && !info.IsDir() {
		if err := kernel.LoadFactsFromFile(profilePath); err != nil {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to load .nerd/profile.gl: %v", err),
				Time:    time.Now(),
			})
		}
	} else if err != nil && !os.IsNotExist(err) {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Unable to access .nerd/profile.gl: %v", err),
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

	// Save session state
	_ = nerdinit.SaveSessionState(m.workspace, state)

	// Convert and save conversation history
	messages := make([]nerdinit.ChatMessage, len(m.history))
	for i, msg := range m.history {
		messages[i] = nerdinit.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	_ = nerdinit.SaveSessionHistory(m.workspace, m.sessionID, messages)
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
