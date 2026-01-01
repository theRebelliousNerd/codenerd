package chat

import (
	"context"
	"sync"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	prompt_evolution "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/browser"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/retrieval"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/transparency"
	"codenerd/internal/usage"
	"codenerd/internal/ux"
	"codenerd/internal/verification"
	"codenerd/internal/world"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

const (
	errorPanelViewportHeight = 4
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// Config holds configuration for initializing the chat interface.
type Config struct {
	// DisableSystemShards is a list of system shard names to disable.
	DisableSystemShards []string
}

// ViewMode determines which component is focused/active
type ViewMode int

const (
	ChatView ViewMode = iota
	ListView
	FilePickerView
	UsageView
	CampaignPage
	PromptInspector // JIT Prompt Inspector
	AutopoiesisPage // Autopoiesis Dashboard
	ShardPage       // Shard Console
)

// InputMode represents the current input handling state.
// This unifies the scattered awaiting* flags into a single state machine
// to prevent inconsistent states and simplify Update() logic.
type InputMode int

const (
	InputModeNormal         InputMode = iota // Default: process as chat input
	InputModeClarification                   // Awaiting clarification response
	InputModePatch                           // Awaiting patch input (--END-- terminated)
	InputModeAgentWizard                     // Agent definition wizard active
	InputModeConfigWizard                    // Config wizard active
	InputModeCampaignLaunch                  // Campaign launch clarification
	InputModeOnboarding                      // Onboarding wizard active (first-run experience)
)

// BootStage represents the startup phase for the interactive UI.
// While any boot stage is active, the chat input is hidden.
type BootStage int

const (
	BootStageBooting BootStage = iota
	BootStageScanning
)

// ContinuationMode controls multi-step task execution behavior.
// Cycle with Shift+Tab, stop anytime with Ctrl+X.
type ContinuationMode int

const (
	ContinuationModeAuto       ContinuationMode = iota // A: Fully automatic until complete
	ContinuationModeConfirm                            // B: Pause after each step, Enter to continue
	ContinuationModeBreakpoint                         // C: Auto for reads, pause before mutations
)

// String returns the display name for each mode
func (m ContinuationMode) String() string {
	names := []string{"Auto", "Confirm", "Breakpoint"}
	if int(m) < len(names) {
		return names[m]
	}
	return "Unknown"
}

// Subtask represents a pending subtask in the continuation queue
type Subtask struct {
	ID          string // Unique task identifier
	Description string // What needs to be done
	ShardType   string // Which shard to execute
	IsMutation  bool   // True for write/run operations (for Breakpoint mode)
}

// sessionItem is a list item for the session list
type sessionItem struct {
	id, date, desc string
}

func (i sessionItem) Title() string       { return i.date }
func (i sessionItem) Description() string { return "[" + i.id + "] " + i.desc }
func (i sessionItem) FilterValue() string { return i.id + " " + i.desc }

// =============================================================================
// CORE TYPES
// =============================================================================

// ClarificationState represents a pending clarification request
type ClarificationState struct {
	Question      string
	Options       []string
	DefaultOption string
	Context       string // Serialized kernel state
	PendingIntent *perception.Intent
}

// Model is the main model for the interactive chat interface
type Model struct {
	// UI Components
	textarea   textarea.Model
	viewport   viewport.Model
	errorVP    viewport.Model
	spinner    spinner.Model
	list       list.Model
	filepicker filepicker.Model
	styles     ui.Styles
	renderer   *glamour.TermRenderer

	viewMode ViewMode

	// Split-pane TUI (Glass Box Interface)
	splitPane         *ui.SplitPaneView
	logicPane         *ui.LogicPane
	showLogic         bool
	paneMode          ui.PaneMode
	showError         bool
	focusError        bool
	showSystemActions bool

	// Usage Page
	usagePage ui.UsagePageModel

	// Campaign Page
	campaignPage ui.CampaignPageModel

	// JIT Inspector Page
	jitPage ui.JITPageModel

	// Autopoiesis Dashboard Page
	autoPage ui.AutopoiesisPageModel

	// Shard Console Page
	shardPage ui.ShardPageModel

	// Usage Tracking
	usageTracker *usage.Tracker

	// State
	history   []Message
	isLoading bool
	err       error
	width     int
	height    int
	ready     bool
	Config    *config.UserConfig

	// Rendering Performance Cache
	// Caches rendered markdown for messages to avoid O(N) re-renders
	renderedCache    map[int]string // Maps message index to rendered output
	cacheInvalidFrom int            // Index from which cache needs refresh (-1 = all valid)

	// JIT Compiler (Observability)
	jitCompiler *prompt.JITPromptCompiler

	// Clarification Loop State (Pause/Resume Protocol)
	awaitingClarification bool
	clarificationState    *ClarificationState
	selectedOption        int // For option picker
	awaitingPatch         bool
	pendingPatchLines     []string
	lastClarifyInput      string // Track last input for clarification dedup
	lastDreamHypothetical string // Track last dream state hypothetical for learning follow-up

	// Session State
	sessionID string
	turnCount int
	// Agent creation wizard
	awaitingAgentDefinition bool

	// Backend
	client              perception.LLMClient
	kernel              *core.RealKernel
	shardMgr            *coreshards.ShardManager // DEPRECATED: Use taskExecutor instead
	taskExecutor        session.TaskExecutor     // New: unified task execution interface
	shadowMode          *core.ShadowMode
	transducer          perception.Transducer
	executor            tactile.Executor
	emitter             *articulation.Emitter
	virtualStore        *core.VirtualStore
	scanner             *world.Scanner
	workspace           string
	DisableSystemShards []string
	browserMgr          *browser.SessionManager // Browser automation manager
	browserCtxCancel    context.CancelFunc      // Cancels browser manager goroutine

	// Campaign Orchestration
	activeCampaign       *campaign.Campaign
	campaignOrch         *campaign.Orchestrator
	campaignProgress     *campaign.Progress
	campaignProgressChan chan campaign.Progress          // Real-time progress updates from orchestrator
	campaignEventChan    chan campaign.OrchestratorEvent // Real-time events from orchestrator
	showCampaignPanel    bool

	// Continuation Protocol (Multi-Step Task Execution)
	// Enables natural multi-step task chaining with three modes:
	// - Auto: Fully automatic until complete (Ctrl+X to stop)
	// - Confirm: Pause after each step (Enter to continue)
	// - Breakpoint: Auto for reads, pause before mutations
	continuationMode  ContinuationMode // Current mode (persisted to config)
	continuationStep  int              // Current step number (1-indexed)
	continuationTotal int              // Total steps detected
	pendingSubtasks   []Subtask        // Queue of pending work
	isInterrupted     bool             // User pressed Ctrl+X

	// Learning Store for Autopoiesis (§8.3)
	learningStore   *store.LearningStore
	embeddingEngine embedding.EmbeddingEngine
	lastReflection  *ReflectionState

	// Dream State Learning (§8.3.1) - Extracts learnings from multi-agent consultations
	dreamCollector *core.DreamLearningCollector
	dreamRouter    *core.DreamRouter

	// Dream Plan Execution (§8.3.2) - Execute plans from Dream State consultations
	dreamPlanManager *core.DreamPlanManager

	// Local knowledge database for research persistence
	localDB *store.LocalStore

	// Semantic Compression (§8.2) - Infinite Context
	compressor *ctxcompress.Compressor

	// Context Feedback Store (§8.4) - Third feedback loop: learns which context facts are useful
	feedbackStore *ctxcompress.ContextFeedbackStore

	// Autopoiesis (§8.3) - Self-Modification
	autopoiesis           *autopoiesis.Orchestrator
	autopoiesisCancel     context.CancelFunc // Cancels kernel listener goroutine
	autopoiesisListenerCh <-chan struct{}    // Closed when listener stops

	// Prompt Evolution (SPL - System Prompt Learning)
	promptEvolver *prompt_evolution.PromptEvolver

	// Mangle File Watcher - monitors .nerd/mangle/*.mg for changes and triggers validation/repair
	mangleWatcher *core.MangleWatcher

	// Transparency Layer (Phase 4 UX) - Makes operations visible to users
	transparencyMgr *transparency.TransparencyManager

	// Background Observer Manager - runs observer specialists (Northstar) in background
	observerMgr *shards.BackgroundObserverManager

	// Consultation Manager - cross-specialist consultation protocol
	consultationMgr *shards.ConsultationManager

	// User Preferences Manager
	preferencesMgr *ux.PreferencesManager

	// Verification Loop (Quality-Enforcing)
	verifier *verification.TaskVerifier

	// Agent Wizard State
	agentWizard *AgentWizardState

	// Config Wizard State
	awaitingConfigWizard bool
	configWizard         *ConfigWizardState

	// Northstar Wizard State
	awaitingNorthstar bool
	northstarWizard   *NorthstarWizardState

	// Onboarding Wizard State (first-run experience)
	awaitingOnboarding bool
	onboardingWizard   *OnboardingWizardState

	// CLI Config
	CLIConfig Config

	// Status Tracking
	statusMessage string      // Current operation description
	statusChan    chan string // Channel for streaming status updates

	// Glass Box Debug Mode - shows system internals inline in chat
	glassBoxEnabled   bool                              // Runtime toggle
	glassBoxEventBus  *transparency.GlassBoxEventBus    // Event collection and dispatch
	glassBoxEventChan <-chan transparency.GlassBoxEvent // Subscription channel
	glassBoxEvents    []transparency.GlassBoxEvent      // Recent events buffer (capped)

	// Tool Event Visibility - ALWAYS shows tool execution in chat (not gated by Glass Box)
	toolEventBus  *transparency.ToolEventBus    // Tool event collection
	toolEventChan <-chan transparency.ToolEvent // Tool event subscription channel

	// Tool Execution Persistence - Stores full tool results in SQLite
	toolStore *store.ToolStore // For querying past executions and cleanup

	// Boot State
	isBooting bool
	bootStage BootStage

	// Process memory usage (bytes). Updated periodically for UI.
	memAllocBytes uint64
	memSysBytes   uint64

	// Input History
	inputHistory []string
	historyIndex int

	// Campaign Launch State
	launchClarifyPending bool
	launchClarifyGoal    string
	launchClarifyAnswers string

	// Context State
	lastShardResult    *ShardResult
	shardResultHistory []*ShardResult

	// Knowledge Discovery State (LLM-first knowledge gathering)
	// Populated when the LLM emits knowledge_requests in control_packet
	pendingKnowledge  []KnowledgeResult // Results from specialist consultations
	knowledgeHistory  []KnowledgeResult // Historical knowledge for session
	awaitingKnowledge bool              // True while gathering knowledge

	// Unified Input Mode (replaces scattered awaiting* flags)
	// Use this for new code; legacy flags preserved for compatibility during migration
	inputMode InputMode

	// Mouse capture toggle (Alt+M to toggle for text selection)
	mouseEnabled bool

	// Shutdown coordination
	shutdownOnce   *sync.Once         // Ensures Shutdown() is only called once (pointer to allow Model copy without sync.Once copy)
	shutdownCtx    context.Context    // Root context for all background operations
	shutdownCancel context.CancelFunc // Cancels shutdownCtx on quit
	goroutineWg    *sync.WaitGroup    // Tracks background goroutines for clean shutdown (pointer to share across value copies)
}

// ShardResult stores the full output from a shard execution for follow-up queries.
// This enables conversational follow-ups like "show me more" or "what are the warnings?".
type ShardResult struct {
	ShardType  string           // "reviewer", "coder", "tester", "researcher"
	Task       string           // Original task sent to the shard
	RawOutput  string           // Full untruncated output
	Timestamp  time.Time        // When the shard executed
	TurnNumber int              // Which turn this was
	Findings   []map[string]any // Structured findings (for reviewer)
	Metrics    map[string]any   // Metrics (for reviewer)
	ExtraData  map[string]any   // Any additional structured data
}

// KnowledgeResult holds the output from a specialist consultation or research request.
// This is populated when the LLM emits knowledge_requests in the control_packet.
type KnowledgeResult struct {
	Specialist string    // Which specialist/shard was consulted
	Query      string    // The original query/question
	Purpose    string    // Why this knowledge was needed
	Response   string    // The specialist's response
	Timestamp  time.Time // When the knowledge was gathered
	Error      error     // Any error that occurred (nil if successful)
}

// Message represents a single message in the chat history
type Message struct {
	Role    string // "user", "assistant", or "system" (Glass Box events)
	Content string
	Time    time.Time

	// Glass Box fields (only set when Role == "system")
	GlassBoxCategory transparency.GlassBoxCategory // Event category for styling
	IsCollapsed      bool                          // Whether details are collapsed
}

// Agent represents a defined agent in the registry
type Agent struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	KnowledgePath string `json:"knowledge_path"`
	KBSize        int    `json:"kb_size"`
	Status        string `json:"status"`
}

// Registry holds the list of defined agents
type Registry struct {
	Version   string  `json:"version"`
	CreatedAt string  `json:"created_at"`
	Agents    []Agent `json:"agents"`
}

// Preferences holds user preferences
type Preferences struct {
	RequireTests     bool   `json:"require_tests"`
	RequireReview    bool   `json:"require_review"`
	Verbosity        string `json:"verbosity"`
	ExplanationLevel string `json:"explanation_level"`
}

// Session holds session state
type Session struct {
	SessionID    string `json:"session_id"`
	StartedAt    string `json:"started_at"`
	LastActiveAt string `json:"last_active_at"`
	TurnCount    int    `json:"turn_count"`
	Suspended    bool   `json:"suspended"`
}

// SystemComponents holds the initialized backend services
type SystemComponents struct {
	Kernel                *core.RealKernel
	ShardMgr              *coreshards.ShardManager // DEPRECATED: Use TaskExecutor instead
	TaskExecutor          session.TaskExecutor     // New: unified task execution interface
	VirtualStore          *core.VirtualStore
	LLMClient             perception.LLMClient
	LocalDB               *store.LocalStore
	LearningStore         *store.LearningStore
	EmbeddingEngine       embedding.EmbeddingEngine
	Transducer            perception.Transducer
	Executor              tactile.Executor
	Scanner               *world.Scanner
	Autopoiesis           *autopoiesis.Orchestrator
	Verifier              *verification.TaskVerifier
	Compressor            *ctxcompress.Compressor
	FeedbackStore         *ctxcompress.ContextFeedbackStore // Third feedback loop: context usefulness learning
	ShadowMode            *core.ShadowMode
	InitialMessages       []Message
	Client                perception.LLMClient
	Emitter               *articulation.Emitter
	AutopoiesisCancel     context.CancelFunc
	AutopoiesisListenerCh <-chan struct{}
	SessionID             string
	TurnCount             int
	BrowserManager        *browser.SessionManager
	BrowserCtxCancel      context.CancelFunc // Cancels browser manager goroutine
	Workspace             string
	JITCompiler           *prompt.JITPromptCompiler
	MangleWatcher         *core.MangleWatcher // Monitors .nerd/mangle/*.mg for changes
	TransparencyMgr       *transparency.TransparencyManager
	PreferencesMgr        *ux.PreferencesManager
	Retriever             *retrieval.SparseRetriever
	GlassBoxEventBus      *transparency.GlassBoxEventBus  // Glass Box debug mode event bus
	ToolEventBus          *transparency.ToolEventBus      // Always-visible tool execution event bus
	ToolStore             *store.ToolStore                // Tool execution persistence store
	PromptEvolver         *prompt_evolution.PromptEvolver // System Prompt Learning evolver
	// Clean Loop Architecture (replaces hardcoded shard logic)
	SessionExecutor *session.Executor // Clean execution loop (JIT-driven)
	SessionSpawner  *session.Spawner  // JIT-driven subagent spawning
	// Background Observer Manager (Northstar, etc.)
	ObserverMgr *shards.BackgroundObserverManager
	// Consultation Manager for cross-specialist consultations
	ConsultationMgr *shards.ConsultationManager
}

// OnboardingWizardStep represents the current phase of the onboarding wizard.
type OnboardingWizardStep int

const (
	OnboardingStepWelcome    OnboardingWizardStep = iota // Show welcome message
	OnboardingStepExperience                             // Ask about experience level
	OnboardingStepAPICheck                               // Check/configure API
	OnboardingStepWow                                    // Demonstrate unique capabilities
	OnboardingStepComplete                               // Finish onboarding
)

// OnboardingWizardState tracks the state of the onboarding wizard.
type OnboardingWizardState struct {
	Step            OnboardingWizardStep
	ExperienceLevel string // beginner, intermediate, advanced, expert
	APIConfigured   bool
	ShowedWow       bool
	SkipRequested   bool
}

// Messages for tea updates
type (
	responseMsg        string
	errorMsg           error
	windowSizeMsg      tea.WindowSizeMsg
	clarificationMsg   ClarificationState // Request for user clarification
	clarificationReply string             // User's response to clarification

	// ShardResultPayload carries a completed shard execution for cross-turn context.
	ShardResultPayload struct {
		ShardType string
		Task      string
		Result    string
		Facts     []core.Fact
	}

	// ClarifyUpdate carries state updates for auto-clarification / launchcampaign flow.
	ClarifyUpdate struct {
		LastClarifyInput     string
		LaunchClarifyPending bool
		LaunchClarifyGoal    string
		LaunchClarifyAnswers string
	}

	// assistantMsg is a richer response message that can also update model state.
	assistantMsg struct {
		Surface           string
		ShardResult       *ShardResultPayload
		ClarifyUpdate     *ClarifyUpdate
		DreamHypothetical string
	}

	// Memory sampling message
	memUsageMsg struct {
		Alloc uint64
		Sys   uint64
	}

	// Campaign messages
	campaignStartedMsg struct {
		campaign     *campaign.Campaign
		orch         *campaign.Orchestrator
		progressChan chan campaign.Progress
		eventChan    chan campaign.OrchestratorEvent
	}
	campaignProgressMsg  *campaign.Progress
	campaignEventMsg     campaign.OrchestratorEvent // Real-time event from orchestrator
	campaignCompletedMsg *campaign.Campaign
	campaignErrorMsg     struct{ err error }

	// Continuation messages (multi-step task execution)
	continueMsg struct {
		subtaskID            string              // Unique identifier for this subtask
		description          string              // What needs to be done
		shardType            string              // Which shard to execute
		isMutation           bool                // True for write/run operations (for Breakpoint mode)
		totalSteps           int                 // Updated total steps if discovered (optional)
		completedShardResult *ShardResultPayload // Result of the just-completed subtask (optional)
	}
	// continuationInitMsg starts a continuation chain with the first step's surface output.
	continuationInitMsg struct {
		completedSurface string
		firstResult      *ShardResultPayload
		next             continueMsg
		totalSteps       int
	}
	interruptMsg        struct{} // User pressed Ctrl+X to stop
	confirmContinueMsg  struct{} // User pressed Enter to continue (Confirm/Breakpoint mode)
	continuationDoneMsg struct { // All steps completed
		stepCount int
		summary   string
		// Result of the final completed subtask (optional)
		completedShardResult *ShardResultPayload
	}

	// Northstar document analysis message
	northstarDocsAnalyzedMsg struct {
		facts []string
		err   error
	}

	// Init messages
	initCompleteMsg struct {
		result        *nerdinit.InitResult
		learningStore *store.LearningStore
	}

	// System Boot message
	bootCompleteMsg struct {
		components *SystemComponents
		err        error
	}

	// statusMsg represents a status update from a background process
	statusMsg string

	// glassBoxEventMsg carries a Glass Box event for inline display
	glassBoxEventMsg transparency.GlassBoxEvent

	// traceUpdateMsg carries Mangle derivation trace data for the logic pane
	traceUpdateMsg struct {
		Trace       *mangle.DerivationTrace
		ShowInChat  bool   // If true, also show explanation in chat history
		QuerySource string // Original query for context
	}

	// onboardingCompleteMsg signals the onboarding wizard has finished
	onboardingCompleteMsg struct {
		ExperienceLevel string
		Skipped         bool
	}

	// onboardingCheckMsg triggers first-run detection after boot
	onboardingCheckMsg struct {
		IsFirstRun bool
		Workspace  string
	}

	// knowledgeGatheredMsg signals that specialist consultations are complete.
	// This triggers re-processing of the original input with enriched context.
	knowledgeGatheredMsg struct {
		Results         []KnowledgeResult // Results from all consulted specialists
		OriginalInput   string            // The user input that triggered knowledge gathering
		InterimResponse string            // The initial LLM response (may include "let me look that up")
	}

	// alignmentCheckMsg carries the result of a Northstar alignment check
	alignmentCheckMsg struct {
		Subject     string   // What was checked
		Result      string   // passed, warning, failed, blocked, skipped
		Score       float64  // 0.0-1.0 alignment score
		Explanation string   // Why this result
		Suggestions []string // Improvement suggestions
		Err         error    // Any error that occurred
	}
)
