// Package session implements the clean execution loop for codeNERD.
//
// This package replaces the shard-based architecture with a unified executor
// that relies entirely on JIT-compiled prompts and configs for specialization.
// The LLM is treated as the creative center; the executor just provides context,
// tools, and safety guardrails.
//
// Architecture:
//
//	User Input → Transducer → JIT Prompt → LLM → VirtualStore → Response
//
// No shards. No spawn. No factories. Clean.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/broker"
	"codenerd/internal/core"
	"codenerd/internal/evidence"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/projectdoc"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// JITCompiler compiles prompt atoms for the current context.
type JITCompiler interface {
	Compile(ctx context.Context, compilationCtx *prompt.CompilationContext) (*prompt.CompilationResult, error)
}

// ConfigFactory creates EffectiveAgentRuntimeConfig from compilation results.
type ConfigFactory interface {
	Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error)
	// ResolveAllowedTools reports the effective executable tool catalog for
	// intents without compiling a prompt. The executor resolves this BEFORE
	// prompt selection so capability atoms can be gated on the same envelope
	// the tool loop will enforce. Implementations must return exactly what
	// Generate would grant for the same intents (including /general fallback).
	ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error)
}

// SessionPersister stores session turn data for cross-session continuity.
type SessionPersister interface {
	StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error
	StoreCompressedState(sessionID string, turnNumber int, stateJSON string, ratio float64) error
}

// InteractiveExecutiveGate is the mandatory capability for effectful tools a VirtualStore can expose
// to bring the Dreamer destructive-action gate and the post-action validator
// registry onto the interactive tool-execution path. The clean executor runs
// modular tools directly via tools.Global(), bypassing RouteAction, so without
// this seam those executive layers never fire on a live coding turn.
//
// The executor type-asserts e.virtualStore against this interface; when the
// store does not implement it (e.g. a nil store or a stub adapter), the gate is
// simply skipped and behavior is identical to before — a graceful fallback.
//
// Implemented by *core.VirtualStore (see virtual_store_interactive_gate.go).
type InteractiveExecutiveGate interface {
	// PreflightDestructiveToolCall runs the Dreamer safety simulation BEFORE a
	// destructive tool executes. A non-nil error means the action is unsafe and
	// must be blocked.
	PreflightDestructiveToolCall(ctx context.Context, actionID, toolName string, args map[string]any) error
	// ValidateInteractiveToolResult runs post-action validators AFTER the tool
	// executes and asserts validation facts to the kernel. A non-nil error means
	// a validator failed with high confidence (the side effect did not land).
	ValidateInteractiveToolResult(ctx context.Context, actionID, toolName string, args map[string]any, output string, success bool) error
}

// warnInteractiveGateUnavailable logs the missing-gate fallback exactly once
// per executor lifetime. The flag gates the log, not the behavior: ungated
// calls still proceed unsimulated (fail-open), but the single warning makes
// the bypass visible instead of silent. Returns true when this call emitted
// the warning.
func (e *Executor) warnInteractiveGateUnavailable() bool {
	if e == nil {
		return false
	}
	if e.gateUnavailableWarned.CompareAndSwap(false, true) {
		logging.Get(logging.CategorySession).Warn("interactive executive gate unavailable: adapter %T does not implement it; effectful calls will be blocked", e.virtualStore)
		return true
	}
	return false
}

// interactiveGate returns the VirtualStore's executive gate when available.
// On the fail-open fallback (store nil or adapter without the interface) it
// logs the one-time warning and reports unavailable. Both the pre-execution
// Dreamer preflight and the post-execution validation seams go through here
// so the warning fires once no matter which seam is hit first.
func (e *Executor) interactiveGate() (InteractiveExecutiveGate, bool) {
	if e == nil || e.virtualStore == nil {
		e.warnInteractiveGateUnavailable()
		return nil, false
	}
	if gate, ok := e.virtualStore.(InteractiveExecutiveGate); ok && gate != nil {
		return gate, true
	}
	e.warnInteractiveGateUnavailable()
	return nil, false
}

// MangleAtom wraps a string as a Mangle name constant (avoids core import).
type MangleAtom string

func (m MangleAtom) String() string { return string(m) }

// catalogBuilderPool reuses strings.Builder instances across
// buildToolCatalogForPiggyback calls. The Piggyback tool catalog is assembled
// once per LLM turn for grounding-capable clients (e.g. Gemini), so the
// builder is hot on long-running sessions. Each Get must be paired with a
// Reset before use and a Put after String() has been read.
var catalogBuilderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

// Executor implements the clean execution loop.
// It replaces all hardcoded shard logic with JIT-driven behavior.
type Executor struct {
	mu sync.RWMutex

	// Core dependencies
	kernel       types.Kernel
	virtualStore types.VirtualStore
	llmClient    types.LLMClient

	// plannerClient serves turns the kernel derives as reasoning-intensive
	// (intent_requires_reasoning_model/1). Nil means every turn stays on
	// llmClient, which is the behaviour when no planner slot is configured.
	plannerClient types.LLMClient

	// reasoningVerbCache memoizes the kernel's reasoning-model verdict per
	// intent verb. The verb set is tiny and the policy is static for the life
	// of the process, so this turns a per-turn Mangle query into one query per
	// distinct verb. map[string]bool.
	reasoningVerbCache sync.Map

	// JIT components
	jitCompiler   JITCompiler
	configFactory ConfigFactory

	// Perception
	transducer perception.Transducer

	// Tool registries (dual-registry Piggyback++ architecture)
	// ouroborosRegistry holds Ouroboros-generated compiled binary tools
	// Modular tools from tools.Global() are accessed directly
	ouroborosRegistry *core.ToolRegistry

	// Context management
	conversationHistory []perception.ConversationTurn
	sessionContext      *types.SessionContext

	// evictedHistory holds the messages the last priorTurnMessages call
	// dropped from the generation window. The window is a view, not a
	// deletion: what leaves it is announced in the window's own text and
	// kept here so it can be brought back.
	evictedHistory []types.Message

	// Session persistence
	sessionPersister SessionPersister
	sessionID        string

	// usageTracker is the process-wide usage meter (shared handle over
	// usage.json). SnapshotTurnUsage reads per-session spend through the
	// context, so ProcessWithIntent tags its ctx with this tracker via
	// meteredContext; without it every turn_cost delta reads zero.
	usageTracker *usage.Tracker

	// Configuration
	config ExecutorConfig

	// Precompiled EffectiveAgentRuntimeConfig (injected by SubAgent)
	EffectiveAgentRuntimeConfig *config.EffectiveAgentRuntimeConfig

	// projectDoc is the workspace's parsed nerd.md, or nil. Used only to render
	// instructions into the prompt; enforcement reads the kernel.
	projectDoc *projectdoc.Document

	// fileContext is the holographic per-file context provider, or nil. Used only
	// to render file-targeted context into the prompt. Narrow interface so no
	// import of internal/world is needed and no import cycle is possible.
	fileContext  FileContextProvider
	workingWorld WorkingWorld
	workingScope string

	// turnCreatedSources and turnCreatedTests are the Go files this turn
	// created, as recordGoFileCreations saw them: a source, and a test with
	// the source it pairs with. They are this executor's own record; nothing
	// reaches the shared kernel until the turn's verdict is asked, when
	// assertTurnEvidence asserts them keyed by the turn. Managed under mu and
	// cleared by checkHollowSuccess, which runs on every path including an
	// errored turn (TestErroredTurnStillClosesAndRetractsItsFacts).
	turnCreatedSources []string
	turnCreatedTests   [][2]string

	// turnFacts is every kernel fact this turn's verdict asserted -- the
	// turn-keyed evidence and the session-global build_state/test_state the
	// same gates write -- and cleanupTurnFacts retracts exactly these on every
	// path. Nothing else is ever retracted: the kernel is shared with every
	// executor CloneForTask made and with the world scanner, and their facts
	// are not this turn's to clear (external audit F1, 2026-09-19).
	turnFacts []types.Fact

	// gateUnavailableWarned ensures the "interactive executive gate
	// unavailable" warning is logged exactly once per executor lifetime, no
	// matter how many turns hit the fail-open fallback. The flag gates the
	// log, not the behavior: ungated calls still proceed unsimulated.
	gateUnavailableWarned atomic.Bool

	// learningsOnce ensures HydrateLearnings runs exactly once per executor
	// lifetime (first Process call); per-turn session context hydrates on
	// every turn via hydrateMemory.
	learningsOnce sync.Once

	// turnRecorder is the optional learning sink every finished turn is
	// reported to (see executor_learning.go). Nil by default, so an executor
	// with nothing wired in behaves as it did before the seam existed.
	turnRecorder TurnRecorder
	// learningPolicy, when set, decides per shard type whether a turn is
	// handed to turnRecorder at all (the shard profile's enable_learning).
	// Nil records every turn.
	learningPolicy func(shardType string) bool

	// contextFeedbackRecorder receives the model's rating of the context it
	// was given; pendingContextFeedback holds that rating between the
	// piggyback parse and persistTurn, where the turn number and manifest are
	// available. Both under mu.
	contextFeedbackRecorder ContextFeedbackRecorder
	pendingContextFeedback  *articulation.ContextFeedback
}

// ExecutorConfig holds configuration for the executor.
//
// It carries no count of tool calls or tool-loop rounds. Until 2026-09-18 it
// held MaxToolCalls, MaxToolIterations, ProgressDrivenTools,
// AdaptiveToolBudget, ToolIterationExtensionSize, MaxToolIterationExtensions
// and ToolLoopRepeatThreshold. The tool loop now continues while the working
// policy derives no stop, so the only time bounds left here are a single
// tool execution's (ToolTimeout) and the tail kept for a final answer when the
// user gave the turn a deadline (FinalAnswerReserve) — a count of calls was
// never a fact about whether the task was done, and neither is a wall clock
// on a repair episode (RepairWallClock, removed 2026-09-19).
type ExecutorConfig struct {
	// ToolTimeout is the maximum time for a single tool execution.
	ToolTimeout time.Duration

	// RepairMaxAttempts bounds one build/test repair episode. Zero falls
	// back to DefaultRepairMaxAttempts; repair is always bounded.
	RepairMaxAttempts int

	// FinalAnswerReserve keeps the tail of a deadline-bound turn available for
	// one tool-free completion. Ordinary tool exploration is cancelled at the
	// deadline minus this reserve so a progressing review cannot consume its
	// entire operation budget and exit without a verdict. When the remaining
	// turn budget is shorter than twice this value, half of the remainder is
	// reserved instead. Zero falls back to the default.
	FinalAnswerReserve time.Duration

	// EnableSafetyGate enables constitutional safety checks.
	EnableSafetyGate bool

	// TokenBudget is the JIT prompt compilation budget passed via
	// CompilationContext. The old hardcoded 8192 was choking spawned
	// shards: when the user's context_window.max_tokens was 1M, the
	// compiler had to drop mandatory atoms (defensive_patterns,
	// behavior_changes, etc.) just to fit prompts into a tiny 8K window.
	// Zero falls back to DefaultTokenBudget at use time, which is set
	// generously for modern long-context models. Callers that load a
	// UserConfig should derive this from
	// ContextWindow.MaxTokens / appropriate fraction.
	TokenBudget int

	// VerifyBuildAfterEdits compiles the workspace after a turn that wrote Go
	// files, and gives the model one round to fix the build with the compiler's
	// output if it broke. See internal/session/build_verify.go.
	//
	// Default ON. codeNERD shipped four compile errors and reported
	// task_status(/manual_instruction, /complete) with exit 0 before this
	// existed; an agent that cannot be trusted to compile its own edits cannot
	// be left unattended.
	VerifyBuildAfterEdits bool

	// VerifyTestsAfterEdits runs `go test` on the packages a turn touched and
	// gives the model one round to fix them if they fail. See
	// internal/session/test_verify.go.
	//
	// Default ON, and deliberately separate from VerifyBuildAfterEdits: they
	// answer different questions. Compiling proves the code parses and type-
	// checks; it proves nothing about whether the code does what it is supposed
	// to. A turn can ship a green build and a broken program.
	VerifyTestsAfterEdits bool

	// CriticReviewAfterEdits runs one adversarial review of the code a turn
	// wrote and, on a high or medium finding, one advisory round to respond.
	// See verifyAndUpliftWithCritic.
	//
	// Default ON, but unlike the other two this gate can never fail a turn. The
	// build and test gates are backed by a compiler and a test runner, which
	// have no opinions. This one is a model reviewing a model, and it will
	// sometimes be confidently wrong — a critic that can fail a turn on a
	// hallucinated defect costs more than it saves.
	CriticReviewAfterEdits bool

	// WorkspaceRoot is the directory the verification build runs in. When empty
	// the Executor resolves the workspace itself rather than silently skipping
	// verification — see workspaceForVerification.
	WorkspaceRoot string

	// HistoryTurnWindow caps how many prior conversation messages reach the
	// generating model. 6 messages = 3 user/assistant exchanges. 0 disables
	// multi-turn memory (prior turns reach perception only, the pre-fix
	// behaviour). Negative is treated as disabled.
	HistoryTurnWindow int

	// HistoryCharBudget caps the total characters of prior-turn text sent to
	// the generating model. Oldest turns are dropped first, never splitting a
	// pair, so a trimmed window always ends on a complete exchange. Zero or
	// negative falls back to DefaultHistoryCharBudget.
	HistoryCharBudget int
}

// DefaultTokenBudget is the prompt-compilation budget used when no
// ExecutorConfig/SpawnerConfig override is set. 65,536 tokens is a
// safe sub-agent default that survives on Claude/Gemini/GPT context
// windows ≥128K and still leaves headroom for response + tool I/O.
const defaultTokenBudgetFallback = 65536

// DefaultTokenBudget is the prompt-compilation allowance for a session turn.
//
// It was a bare const at 65536 — a number chosen conservatively because nothing
// in the codebase could tell what the real window was or who else was spending
// it. It now derives from the one ledger, taking half the enforced window and
// leaving the rest for the context block, the history, and the tool schemas
// that share it.
func DefaultTokenBudget() int {
	return broker.Default().PromptBudget(0.5, defaultTokenBudgetFallback)
}

const (
	defaultToolTimeout        = 5 * time.Minute
	defaultFinalAnswerReserve = 5 * time.Minute
)

// DefaultHistoryTurnWindow is the number of prior conversation messages that
// reach the generating model when ExecutorConfig leaves HistoryTurnWindow
// unset via DefaultExecutorConfig. 6 messages = 3 user/assistant exchanges.
const DefaultHistoryTurnWindow = 6

// DefaultHistoryCharBudget caps prior-turn text sent to the generating model
// when ExecutorConfig leaves HistoryCharBudget unset. 24000 characters holds
// several substantive exchanges without crowding the prompt budget.
const DefaultHistoryCharBudget = 24000

// defaultSemanticTopK matches the value NewCompilationContext applies. This
// path builds the CompilationContext as a literal, so it gets no defaults from
// that constructor and must supply its own — a zero here reaches
// VectorSearcher.Search as topK=0.
const defaultSemanticTopK = 20

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		ToolTimeout:        defaultToolTimeout,
		RepairMaxAttempts:  DefaultRepairMaxAttempts,
		FinalAnswerReserve: defaultFinalAnswerReserve,
		EnableSafetyGate:   true,
		TokenBudget:        DefaultTokenBudget(),
		HistoryTurnWindow:  DefaultHistoryTurnWindow,
		HistoryCharBudget:  DefaultHistoryCharBudget,
		// On by default: the failure this prevents (confident, non-compiling
		// edits reported as complete) is silent, and a default-off guard against
		// a silent failure protects nobody.
		VerifyBuildAfterEdits:  true,
		VerifyTestsAfterEdits:  true,
		CriticReviewAfterEdits: true,
	}
}

// NewExecutor creates a new executor with the given dependencies.
func NewExecutor(
	kernel types.Kernel,
	virtualStore types.VirtualStore,
	llmClient types.LLMClient,
	jitCompiler JITCompiler,
	configFactory ConfigFactory,
	transducer perception.Transducer,
) *Executor {
	logging.Session("Creating new Executor")

	return &Executor{
		kernel:              kernel,
		virtualStore:        virtualStore,
		llmClient:           llmClient,
		jitCompiler:         jitCompiler,
		configFactory:       configFactory,
		transducer:          transducer,
		conversationHistory: make([]perception.ConversationTurn, 0),
		config:              DefaultExecutorConfig(),
	}
}

// SetPlannerClient installs the high-reasoning client used for turns the
// kernel derives as reasoning-intensive. Passing nil (or the same client as
// llmClient) leaves every turn on the default client.
func (e *Executor) SetPlannerClient(c types.LLMClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c == e.llmClient {
		c = nil
	}
	e.plannerClient = c
}

// llmForVerb resolves which client serves this turn. Callers must resolve ONCE
// per turn and thread the result through the whole tool loop: the initial
// generation and its tool-result follow-ups share a conversation history, so
// splitting them across two models would feed one vendor's tool_use IDs to
// another.
func (e *Executor) llmForVerb(verb string) types.LLMClient {
	e.mu.RLock()
	base, planner := e.llmClient, e.plannerClient
	e.mu.RUnlock()

	if planner == nil || !e.intentRequiresReasoningModel(verb) {
		return base
	}
	logging.SessionDebug("Routing %q to the planner LLM slot", verb)
	return planner
}

// servingIdentity reports the provider and model that will serve this verb's
// turn, for prompt-atom pinning.
//
// It routes through llmForVerb rather than reading the main client directly,
// because a verb the kernel derives as reasoning-intensive is served by the
// planner slot -- frequently a different vendor entirely. Compiling the prompt
// against the main client's identity would pin-match atoms for a model that
// never sees them, and block the ones the planner actually needs.
//
// A client that does not implement types.ModelIdentifier yields empty strings,
// which leaves every pinned atom blocked (the dimensions are fail-closed) while
// unpinned atoms are unaffected. That is logged, because the symptom otherwise
// is a quietly smaller prompt.
func (e *Executor) servingIdentity(verb string) (provider, model string) {
	client := e.llmForVerb(verb)
	if client == nil {
		return "", ""
	}

	var identifier types.ModelIdentifier
	broker.Walk(client, func(layer types.LLMClient) bool {
		if id, ok := layer.(types.ModelIdentifier); ok {
			identifier = id
			return false
		}
		return true
	})
	if identifier == nil {
		logging.SessionDebug(
			"LLM client %T does not report a model identity; provider/model-pinned prompt atoms will be skipped this turn",
			client)
		return "", ""
	}

	provider, model = identifier.ModelIdentity()
	logging.SessionDebug("Compiling prompt for provider=%q model=%q", provider, model)
	return provider, model
}

// intentRequiresReasoningModel asks the kernel whether this verb's turn should
// be served by the reasoning tier. The decision lives in the policy corpus
// (delegation.mg → intent_requires_reasoning_model/1); this helper only asks.
//
// A missing kernel or a failed query answers false, which routes the turn to
// the cheap client. That is the safe direction for cost but the wrong one for
// quality, so a query failure is logged at Warn rather than swallowed —
// otherwise a broken policy corpus would silently demote every planning turn.
func (e *Executor) intentRequiresReasoningModel(verb string) bool {
	verb = strings.TrimSpace(verb)
	if verb == "" {
		return false
	}
	if cached, ok := e.reasoningVerbCache.Load(verb); ok {
		return cached.(bool)
	}
	if isConsultIntentVerb(verb) {
		e.reasoningVerbCache.Store(verb, false)
		return false
	}
	if e.kernel == nil {
		return false
	}

	// Intent verbs arrive as Mangle atoms already ("/review", "/campaign"), so
	// they need no quoting — but an invalid atom would become query source, so
	// reject anything unexpected rather than interpolate it.
	if !validMangleVerb(verb) {
		logging.SessionDebug("intentRequiresReasoningModel: %q is not an atom, defaulting to false", verb)
		e.reasoningVerbCache.Store(verb, false)
		return false
	}

	facts, err := e.kernel.Query(fmt.Sprintf("intent_requires_reasoning_model(%s)", verb))
	if err != nil {
		logging.Get(logging.CategorySession).Warn(
			"intentRequiresReasoningModel(%s) query failed: %v — turn stays on the default LLM slot", verb, err)
		return false
	}
	requires := len(facts) > 0
	e.reasoningVerbCache.Store(verb, requires)
	return requires
}

// SetSessionContext sets the session context for dream mode and shared state.
func (e *Executor) SetSessionContext(ctx *types.SessionContext) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionContext = ctx
}

// CloneForTask returns a fresh executor sharing this executor's dependencies
// (kernel, store, clients, compilers) but with ISOLATED per-run state: empty
// conversation history, no session context, no turn persistence.
//
// Inline task execution used to run directly on the shared session executor,
// which (a) was not thread-safe (SetSessionContext races) and (b) appended
// every delegated task and its output to the session's conversation history,
// contaminating perception and articulation context for unrelated later
// turns. Cloning costs one struct allocation and removes both failure modes.
func (e *Executor) CloneForTask() *Executor {
	e.mu.RLock()
	defer e.mu.RUnlock()

	clone := NewExecutor(e.kernel, e.virtualStore, e.llmClient, e.jitCompiler, e.configFactory, e.transducer)
	clone.config = e.config
	clone.ouroborosRegistry = e.ouroborosRegistry
	clone.sessionID = e.sessionID
	clone.usageTracker = e.usageTracker
	clone.plannerClient = e.plannerClient
	clone.projectDoc = e.projectDoc
	clone.fileContext = e.fileContext
	// turnRecorder IS inherited, and it is the one place where inheriting
	// differs from sessionPersister on purpose. Persistence is session
	// bookkeeping, which a delegated task has no business writing into. The
	// learning sink is the opposite: delegated tasks ARE the work — a campaign
	// is thousands of them and a chat turn is a handful — so an executor clone
	// that dropped the recorder would leave the system learning only from the
	// paths a human happens to be watching.
	clone.turnRecorder = e.turnRecorder
	clone.learningPolicy = e.learningPolicy
	// Same reasoning for the context rating: a delegated task compiles its own
	// prompt, so its verdict on that prompt is exactly as informative as a
	// chat turn's.
	clone.contextFeedbackRecorder = e.contextFeedbackRecorder
	// The working world is the kernel view the per-task working set selects
	// context from; a delegated task acts on the same workspace, so it reads
	// the same world.
	clone.workingWorld = e.workingWorld
	// Deliberately NOT copied: conversationHistory, sessionContext,
	// sessionPersister (task runs must not be recorded as session turns),
	// EffectiveAgentRuntimeConfig (set per task by the caller).
	// sessionID IS inherited so audit boundaries correlate the task with its
	// parent session; without a persister the clone still records nothing.
	// Per-workspace context (projectDoc, fileContext) IS inherited because a
	// delegated task acts on the same workspace as the session that spawned it.
	return clone
}

// SetConfig updates the executor configuration.
func (e *Executor) SetConfig(cfg ExecutorConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = cfg
}

// configSnapshot returns one coherent executor configuration. SetConfig is
// legal after construction and is used by campaign/e2e surfaces; every runtime
// read must therefore go through the same lock rather than racing a multi-field
// struct assignment.
func (e *Executor) configSnapshot() ExecutorConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// SetAgentConfig injects a pre-compiled agent config, bypassing JIT config compilation.
func (e *Executor) SetAgentConfig(cfg *config.EffectiveAgentRuntimeConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.EffectiveAgentRuntimeConfig = cfg
}

// SetHistory sets the conversation history.
func (e *Executor) SetHistory(history []perception.ConversationTurn) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.conversationHistory = make([]perception.ConversationTurn, len(history))
	copy(e.conversationHistory, history)
}

// SetOuroborosRegistry sets the Ouroboros tool registry for generated tools.
// This enables Piggyback++ to include Ouroboros-generated tools in the catalog.
// A nil registry clears the slot instead of panicking so non-TUI boot paths
// that have no generated tools yet can still call this unconditionally.
func (e *Executor) SetOuroborosRegistry(registry *core.ToolRegistry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ouroborosRegistry = registry
	if registry == nil {
		return
	}
	logging.Session("Ouroboros registry configured with %d tools", len(registry.ListTools()))
}

// OuroborosRegistry returns the configured Ouroboros tool registry, or nil
// when no generated-tool registry has been wired (e.g. before boot).
func (e *Executor) OuroborosRegistry() *core.ToolRegistry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ouroborosRegistry
}

// SetSessionPersister sets the store for persisting session turns.
// When set, each Process() call records the turn for cross-session continuity.
func (e *Executor) SetSessionPersister(persister SessionPersister) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionPersister = persister
}

// SetSessionID sets the session identifier for turn persistence.
func (e *Executor) SetSessionID(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionID = id
}

// SessionID returns the session identifier used for turn persistence and
// audit boundaries. Empty means the executor falls back to "default".
func (e *Executor) SessionID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessionID
}

// SetUsageTracker installs the process-wide usage meter this executor's turns
// read through meteredContext. Nil-safe: a nil tracker clears the slot so
// boot paths without metering keep the previous zero-cost behaviour instead
// of panicking.
func (e *Executor) SetUsageTracker(t *usage.Tracker) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.usageTracker = t
}

// meteredContext returns ctx carrying this executor's usage tracker so
// snapshotTurnUsage can read the session's spend. A tracker already present
// on ctx wins (callers that tag their own context keep it); a nil executor
// or a nil owned tracker leaves ctx unchanged.
func (e *Executor) meteredContext(ctx context.Context) context.Context {
	if usage.FromContext(ctx) != nil {
		return ctx
	}
	if e == nil {
		return ctx
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.usageTracker == nil {
		return ctx
	}
	return usage.NewContext(ctx, e.usageTracker)
}

// ExecutionResult holds the result of processing user input.
type ExecutionResult struct {
	// Acceptance is populated only by caller-authorized, revision-bound checks.
	Acceptance *evidence.Report
	BuildCheck BuildVerification
	TestCheck  TestVerification
	// VetCheck is `go vet` over the packages the turn wrote, judged on the
	// turn's own files: a finding in a file the turn did not touch is not
	// this turn's evidence.
	VetCheck              BuildVerification
	ChecksSnapshot        string
	ChangeStage           string
	acceptanceTransaction *evidence.Transaction
	// Response is the text response to show the user.
	Response string

	// Intent is the parsed user intent.
	Intent perception.Intent

	// ToolCallsExecuted is the number of tool calls made.
	ToolCallsExecuted int

	// SuccessfulToolCalls counts tool calls that completed without an execution,
	// safety, validation, or budget error. Hollow-success checks use this rather
	// than ToolCallsExecuted so a failed command cannot satisfy a side-effecting
	// intent merely because it was attempted.
	SuccessfulToolCalls int

	// SuccessfulWriteTools counts write_file/edit_file (and peers) that
	// completed without error. Used to block hollow success on write-oriented
	// intents that only produced prose or non-mutating tool calls.
	SuccessfulWriteTools int

	// TestRunCalls counts tool calls that started a test process, as the tool
	// layer recorded it (tools.TestRun) -- whether its tests passed or failed.
	// It exists so a claimed test result can be checked against a real run
	// rather than taken on trust; a tool's name, a dry run or an empty
	// selection is not a run.
	TestRunCalls int

	// TestRunSinceLastWrite is the last test process the tool layer recorded
	// after this turn's last successful write, or nil when none has run since
	// it; every successful write resets it. It is the /test_run gate's
	// measurement: a write the Go gates do not cover -- anything but Go and
	// documentation -- is verified by a test run the model started once it had
	// stopped writing, and that run's exit decides (coder_safety.mg
	// turn_owes_gate, external audit N01).
	TestRunSinceLastWrite *tools.TestRun

	// turn is this turn's key in the kernel (turnAtom), minted on first use:
	// a forcing round asks the policy what the turn's writes owe before the
	// closure asserts its verdict, and both ask about the same turn.
	turn types.MangleAtom

	// writtenAsserted holds each written path whose turn_written fact is
	// asserted for this turn (assertTurnWrites).
	writtenAsserted map[string]bool

	// WrittenPaths records the target of every successful write mutation, so
	// post-edit build verification can tell a turn that touched Go source from
	// one that only wrote markdown and skip the compile it does not need.
	WrittenPaths []string

	// PreWriteContents holds each written path's preimage, keyed by the same
	// workspace-relative slash path as WrittenPaths: what it held before this
	// turn's first successful or attempted write to it. Post-edit signals are
	// narrowed to the lines the turn changed, and an undone round puts it
	// back -- an absent file, an empty one and an unreadable one are three
	// different preimages (see PreImage).
	PreWriteContents map[string]PreImage

	// UntestedPaths records production Go files this turn wrote with no test
	// file alongside them. A warning, not a failure — a turn may legitimately
	// edit a file whose tests were written long ago — but it is the signal that
	// makes "always ship a test" auditable rather than aspirational.
	UntestedPaths []string

	// UncoveredBlocks records blocks in this turn's own files that no test
	// executed. This is the signal `go test` cannot give: a turn can add a
	// function, add a test file that never calls it, and go green.
	UncoveredBlocks []UncoveredBlock

	// CriticFindings records what the adversarial review reported, including
	// low-severity items that did not trigger an uplift round. Advisory only —
	// nothing here fails a turn.
	CriticFindings []CriticFinding

	// StaticDiagnostics is gopls output for this turn's files, when gopls is
	// installed. It reports a class of defect the compiler is deliberately
	// silent about; advisory, and empty on machines without gopls.
	StaticDiagnostics string

	// StepReport is the executive's ledger of a planned task: every step,
	// whether it edited, and the model's closing word on it. Empty for a
	// single-pass turn. It is appended to the response so the user reads the
	// truth about a task that was run in steps, whatever the model's last
	// sentence claimed.
	StepReport string

	// Duration is how long the execution took.
	Duration time.Duration

	// Error is set if execution failed.
	Error error

	// TurnOutcome is the kernel's verdict for this turn (/done, /hollow,
	// /failed, /unverified), captured by checkHollowSuccess before per-turn
	// facts are retracted so turn_cost can record it. Empty until determined.
	TurnOutcome types.MangleAtom

	// MissingEvidence are the turn_missing_evidence atoms the corpus derived
	// for an /unverified turn (/build_not_green, /tests_not_green). It is why
	// the turn is not done, in the kernel's words rather than the executor's,
	// and it is what the closing evidence sentence names. Empty for a verified
	// turn and for a turn that changed nothing.
	MissingEvidence []string
}

// Process handles user input through the clean loop.
//
// The loop:
//  1. Transducer: NL → Intent
//  2. JIT: Compile prompt (persona + skills + context)
//  3. JIT: Compile config (tools, policies)
//  4. LLM: Generate response with tool calls
//  5. Execute: Route tool calls through VirtualStore
//  6. Articulate: Response to user
func (e *Executor) Process(ctx context.Context, input string) (*ExecutionResult, error) {
	return e.ProcessWithIntent(ctx, input, nil)
}

// taskIntentCounter feeds unique task-scoped intent IDs (see ProcessWithIntent).
var taskIntentCounter uint64

// wrapToolLoopError separates two error classes sharing one return path.
// runToolLoop returns genuine LLM failures and post-edit verification gate
// failures (build/test/critic re-verification, marked with
// ErrVerificationFailed). Only the former is an LLM failure; a turn whose
// edits do not compile must not tell the user "LLM generation failed".
func wrapToolLoopError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrVerificationFailed) || errors.Is(err, ErrStepsIncomplete) {
		return err
	}
	return fmt.Errorf("LLM generation failed: %w", err)
}

// ProcessWithIntent runs the clean loop with an optional pre-classified
// intent. When preset is non-nil the OBSERVE phase is skipped entirely:
//
//   - No perception LLM call is made. Delegated tasks arrive with their
//     intent verb already decided by the routing layer; re-classifying the
//     machine-generated task string burned a classification call per
//     delegation AND could select the wrong persona prompt when the
//     re-classification disagreed with the original routing.
//
//   - The intent fact is asserted under a unique task-scoped ID (and
//     retracted afterwards) instead of /current_intent. SubAgents share the
//     session kernel; writing /current_intent from a concurrent task would
//     clobber the interactive turn's routing facts mid-flight.
func (e *Executor) ProcessWithIntent(ctx context.Context, input string, preset *perception.Intent) (*ExecutionResult, error) {
	start := time.Now()
	logging.Session("Processing input: %d chars (preset_intent=%v)", len(input), preset != nil)

	if strings.TrimSpace(input) == "" {
		return nil, errors.New("empty input provided")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error before processing: %w", err)
	}

	result := &ExecutionResult{}
	if contract, ok := evidence.ContractFromContext(ctx); ok {
		tx, err := evidence.Begin(ctx, e.workspaceForVerification(), contract)
		if err != nil {
			return nil, fmt.Errorf("acceptance baseline: %w", err)
		}
		result.acceptanceTransaction = tx
	}
	// Audit: turn boundaries (sessionID/turnNum derived from executor state).
	e.mu.RLock()
	auditSessionID := e.sessionID
	auditTurnNum := len(e.conversationHistory)/2 + 1
	e.mu.RUnlock()
	if auditSessionID == "" {
		auditSessionID = "default"
	}
	logging.Audit().TurnStart(auditSessionID, auditTurnNum, len(input))
	// TurnEnd is deferred so every exit path (including early errors after this
	// point) emits a matching session_event. Success is derived from result.Error
	// at defer time; for early returns that bypass result, success remains false.
	defer func() {
		success := result != nil && result.Error == nil
		// hollow-success and tool-error cases already surface via result.Error
		logging.Audit().TurnEnd(auditSessionID, auditTurnNum, time.Since(start).Milliseconds(), success)
	}()

	// Memory is read back on the executor path, not just persisted: learned
	// facts and prior-session context hydrate the kernel before prompt
	// compilation so logic can use persisted context. Best-effort — hydration
	// failures are logged and never fail the turn — and skipped entirely when
	// the store does not implement the hydration interface (stub adapters).
	// Tag the turn with its session identity so LLM clients record spend under
	// this session (BySession) rather than "unknown"; turn_cost reads that
	// entry, never the cross-process project total.
	ctx = usage.WithSessionID(ctx, e.SessionID())
	// The default account for a turn. Subsystems that run inside it --
	// perception, compression, verification, spawned shards -- override this
	// on their own sub-context, so what stays tagged "session" is the turn's
	// own reasoning rather than everything it triggered.
	ctx = broker.WithPurpose(ctx, broker.PurposeSession)
	// Carry the owned usage meter on the turn context so snapshotTurnUsage
	// below (and every sub-agent/clone turn derived from this executor) reads
	// this session's spend instead of zeros. A tracker already on ctx wins.
	ctx = e.meteredContext(ctx)
	// A per-turn identity for cost attribution: every executor in a campaign
	// shares the session id, so a session-level delta absorbed sibling shards'
	// tokens (one coder turn read 4.9 M). Per-turn counts are exact and local.
	ctx = usage.WithTurnID(ctx, fmt.Sprintf("%s#%d#%d", e.SessionID(), auditTurnNum, time.Now().UnixNano()))
	// Settle this turn's prompt-atom selections against its outcome. The
	// compiler records what was selected; only here is it known whether the
	// selection worked. The id is read into a local rather than out of ctx at
	// defer time because ctx is reassigned repeatedly below, and a closure
	// reading it later would settle against whatever the last reassignment
	// left behind.
	coUseTurnID := usage.TurnIDFromContext(ctx)
	defer func() {
		outcome := prompt.OutcomeSuccess
		if result == nil || result.Error != nil {
			outcome = prompt.OutcomeFailure
		}
		prompt.CoUse().Settle(coUseTurnID, outcome)
	}()
	e.hydrateMemory(ctx, input)
	usageBefore := snapshotTurnUsage(ctx, e.SessionID())

	// 1. OBSERVE: Transducer converts NL → Intent (skipped for preset intents)
	var intent perception.Intent
	if preset != nil {
		intent = *preset
	} else {
		observed, err := e.observe(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("observation failed: %w", err)
		}
		intent = observed
	}
	result.Intent = intent

	// What the system understood the user to mean is the first link in every
	// derivation chain that follows, and it was the one step no durable record
	// captured — IntentParsed had zero callers. Without it, a turn that went
	// wrong in an unattended run cannot be told apart from a turn that was
	// asked for the wrong thing.
	logging.Audit().IntentParsed(intent.Category, intent.Verb, intent.Target, intent.Confidence)

	// Assert intent to kernel for Mangle policy evaluation. Interactive runs
	// own /current_intent; task runs get a unique ID that is cleaned up when
	// the run ends so concurrent subagents can never fight over routing facts.
	intentID := "/current_intent"
	if preset != nil {
		intentID = fmt.Sprintf("/task_intent_%d", atomic.AddUint64(&taskIntentCounter, 1))
	}
	if e.kernel != nil {
		intentFact := types.Fact{
			Predicate: "user_intent",
			Args: []any{
				types.MangleAtom(intentID),
				types.MangleAtom(intent.Category),
				types.MangleAtom(intent.Verb),
				intent.Target,
				intent.Constraint,
			},
		}
		if assertErr := e.kernel.Assert(intentFact); assertErr != nil {
			logging.Get(logging.CategorySession).Warn("Failed to assert intent: %v", assertErr)
		} else if preset != nil {
			defer func() {
				if retractErr := e.kernel.RetractFact(intentFact); retractErr != nil {
					logging.SessionDebug("Failed to retract task intent %s: %v", intentID, retractErr)
				}
			}()
		}
	}

	// 2. ORIENT: Build compilation context from intent + world state
	compilationCtx := e.buildCompilationContext(ctx, intent)
	// Delegation frequently supplies only a verb in its structured intent.
	// The actual task must reach retrieval; searching for the expert's name
	// instead made every task share a cache key and discard task-specific memory.
	if query := strings.TrimSpace(input); query != "" {
		const maxRetrievalQueryRunes = 4096
		runes := []rune(query)
		if len(runes) > maxRetrievalQueryRunes {
			query = string(runes[:maxRetrievalQueryRunes])
		}
		compilationCtx.SemanticQuery = query
	}

	// 3. JIT: Compile prompt with persona, skills, context
	var compileResult *prompt.CompilationResult
	if e.jitCompiler == nil {
		logging.Get(logging.CategorySession).Warn("JIT compiler unavailable, using baseline prompt")
		compileResult = &prompt.CompilationResult{
			Prompt: "You are an AI assistant helping with software development.",
		}
	} else {
		compiled, compileErr := e.jitCompiler.Compile(ctx, compilationCtx)
		if compileErr != nil {
			// FLAGGED, not changed: this fallback and main's 938cd87 disagree,
			// and the disagreement is worth someone's decision rather than
			// mine.
			//
			// That commit made the compiler REFUSE when the mandatory skeleton
			// does not fit the budget, and its reasoning is explicit: "A cut
			// identity or safety atom is not a smaller prompt, it is a
			// different constitution, and a turn that runs on one is worse
			// than a turn that refuses." This line catches that refusal and
			// runs the turn on fifty-two characters — which is a different
			// constitution by any reading, and the Warn above is the only
			// trace.
			//
			// The compiler cannot help a caller tell those apart: the refusal
			// is a plain fmt.Errorf, so "the budget cannot hold the skeleton"
			// and "the corpus database is locked" arrive as the same error.
			// Distinguishing them wants a typed error in internal/prompt and a
			// decision about what a refused compile should do to a turn, which
			// belongs with whoever owns this loop.
			//
			// The narrower improvement available without that decision is to
			// degrade to prompt.AssembleEmbeddedBaselinePrompt, the embedded
			// mandatory atoms, as internal/articulation already does in the
			// same situation — a real constitution rather than one sentence.
			logging.Get(logging.CategorySession).Warn("JIT compilation failed, using baseline: %v", compileErr)
			// Fall back to baseline prompt if JIT fails
			compileResult = &prompt.CompilationResult{
				Prompt: "You are an AI assistant helping with software development.",
			}
		} else {
			compileResult = compiled
		}
	}

	// 4. JIT: Compile config (tools, policies)
	EffectiveAgentRuntimeConfig, err := e.compileConfig(ctx, compileResult, intent)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("Config compilation failed: %v", err)
		// Continue with empty config - LLM can still respond
		EffectiveAgentRuntimeConfig = &config.EffectiveAgentRuntimeConfig{}
	} else {
		logging.Session("Config compiled: %d tools allowed", len(EffectiveAgentRuntimeConfig.AllowedTools))
	}

	// 4b. Project instructions from nerd.md.
	//
	// Appended after JIT compilation rather than modelled as a prompt atom
	// because it is per-workspace user content, not part of the shipped corpus:
	// the atom selector has no way to score a document it has never seen, and
	// budget-driven eviction could silently drop the project's own rules.
	systemPrompt := e.withCompiledFileContext(ctx, e.withProjectInstructions(compileResult.Prompt), intent.Target)

	// 5+6. LLM ↔ tools loop. The model may request tools, we execute them, then
	// feed the results back as a new turn — repeated until the model returns a
	// final answer with no tool calls, or we hit the iteration cap.
	llmResponse, toolErrs, err := e.runToolLoop(ctx, systemPrompt, input, EffectiveAgentRuntimeConfig, compilationCtx, result)
	if err != nil {
		return nil, wrapToolLoopError(err)
	}

	// 7. Articulate response — process Piggyback control packet (best-effort)
	result.Response = e.processPiggybackControlPacket(llmResponse.Text)
	if result.StepReport != "" {
		result.Response = strings.TrimSpace(result.Response) + "\n\n" + result.StepReport
	}
	result.Duration = time.Since(start)

	// Surface unrecovered tool failures as the execution error. See
	// surfaceToolErrors: whether a mid-turn tool error survived is decided by
	// the closing evidence, not by the shape of the final response.
	surfaceToolErrors(result, toolErrs)

	// Close the turn on every path: checkHollowSuccess asserts the turn's
	// evidence, reads the kernel's verdict once and retracts the per-turn
	// facts. Until 2026-09-18 it sat behind `if result.Error == nil`, so a
	// turn whose tool error survived never had its verdict captured and never
	// had the facts its tool loop asserted (created_source) retracted -- one
	// errored turn could raise "new source was created without a test file"
	// against every later turn (REVIEW-wave1 F5 / open item 7).
	//
	// Block hollow success: mutation intents that require real side effects
	// must not report completion when the model only returned planning prose
	// (or only non-mutating tools). Dream/shadow runs skip this gate. A real
	// error already on the result outranks the hollow reason.
	//
	// Hollow failures are hard errors (non-nil return) so CLI one-shots exit
	// non-zero. Other soft tool failures stay on result.Error with a nil
	// return for interactive chat compatibility; TaskExecutor still surfaces
	// result.Error for SpawnTask callers.
	e.closeAcceptanceEvidence(ctx, result)
	hollowErr := e.checkHollowSuccess(result)
	if result.Error == nil && hollowErr != nil {
		result.Error = hollowErr
	}
	// The closing evidence sentence comes LAST, because it is a function of
	// the verdict checkHollowSuccess just captured. Written before it (where
	// it used to live) the only thing it could say was "unverified", and it
	// said so on every write turn regardless of what the gates measured.
	e.appendEvidenceSummary(result)

	// Update conversation history
	e.appendToHistory(perception.ConversationTurn{
		Role:    "user",
		Content: input,
	})
	e.appendToHistory(perception.ConversationTurn{
		Role:             "assistant",
		Content:          result.Response,
		ThoughtSummary:   llmResponse.ThoughtSummary,
		ThoughtSignature: llmResponse.ThoughtSignature,
	})

	// Dispatch asynchronous learning over the conversation WINDOW, not this
	// turn alone. See criticWindow for why one trace could never work.
	e.queueTaxonomyLearning(result, toolErrs)

	// Persist session turn for cross-session continuity
	e.persistTurn(ctx, input, intent, result, turnTelemetry{compileResult: compileResult, usageBefore: usageBefore})

	logging.Session("Execution complete: %d tool calls, %v duration", result.ToolCallsExecuted, result.Duration)

	if result.Error != nil && isHollowSuccessError(result.Error) {
		return result, result.Error
	}
	return result, nil
}

// observe uses the transducer to convert natural language to intent.
func (e *Executor) observe(ctx context.Context, input string) (perception.Intent, error) {
	e.mu.RLock()
	transducer := e.transducer
	history := e.conversationHistory
	e.mu.RUnlock()

	// A missing transducer fails closed as an error, never as a nil-pointer
	// panic: Process must stay callable on partially-wired executors.
	if transducer == nil {
		return perception.Intent{}, fmt.Errorf("cannot perceive intent %q: no transducer configured", input)
	}
	return transducer.ParseIntentWithContext(ctx, input, history)
}

// buildCompilationContext creates a CompilationContext from the current state.
func (e *Executor) buildCompilationContext(ctx context.Context, intent perception.Intent) *prompt.CompilationContext {
	budget := e.configSnapshot().TokenBudget
	if budget <= 0 {
		budget = DefaultTokenBudget()
	}
	cc := &prompt.CompilationContext{
		IntentVerb:      intent.Verb,
		IntentTarget:    intent.Target,
		OperationalMode: "/active",
		TokenBudget:     budget,
	}

	// Name the LLM that will consume this prompt, so provider/model-pinned
	// atoms can be matched against it. This is not optional bookkeeping: both
	// dimensions are fail-closed regime dimensions in jit_compiler.mg, so a
	// context that leaves them empty blocks every pinned atom rather than
	// admitting them all. Leaving this unset would silently retire the entire
	// evolved corpus.
	cc.Provider, cc.Model = e.servingIdentity(intent.Verb)

	if e.kernel != nil {
		if langFacts, err := e.kernel.Query("project_language"); err != nil {
			logging.Get(logging.CategorySession).Warn("buildCompilationContext: project_language query failed: %v", err)
		} else if len(langFacts) > 0 && len(langFacts[0].Args) > 0 {
			lang := types.ExtractString(langFacts[0].Args[0])
			if lang != "" && !strings.HasPrefix(lang, "/") {
				lang = "/" + lang
			}
			cc.Language = lang
		}
		if fwFacts, err := e.kernel.Query("project_framework"); err != nil {
			logging.Get(logging.CategorySession).Warn("buildCompilationContext: project_framework query failed: %v", err)
		} else {
			seen := make(map[string]struct{}, len(fwFacts))
			for _, f := range fwFacts {
				if len(f.Args) == 0 {
					continue
				}
				fw := types.ExtractString(f.Args[0])
				if fw == "" {
					continue
				}
				if !strings.HasPrefix(fw, "/") {
					fw = "/" + fw
				}
				if _, ok := seen[fw]; !ok {
					seen[fw] = struct{}{}
					cc.Frameworks = append(cc.Frameworks, fw)
				}
			}
		}
	}

	// The file the turn is aimed at outranks the project's language: a turn on
	// a policy file in a Go project needs the /mangle corpus, and the project's
	// language would hand it the Go one (planned steps do the same per step,
	// work_steps.go stepSystemPrompt).
	if lang := languageOfFile(intent.Target); lang != "" {
		cc.Language = lang
	}
	cc.DerivedNeeds = e.targetNeeds(cc.Language)

	// Drive vector atom selection.
	//
	// This struct is built literally, which bypasses the SemanticTopK default of
	// 20 that NewCompilationContext applies, and SemanticQuery was never set at
	// all. AtomSelector gates vector search on
	// `s.vectorSearcher != nil && cc.SemanticQuery != ""`, so on the session
	// executor path — every interactive turn and every CLI verb — the gate was
	// always false and semantic search never ran. Observed live on one
	// `nerd explain`: "vector=0ms | atoms=23 (skel=23 flesh=0)". The entire
	// probabilistic half of the skeleton/flesh architecture was inert, with an
	// embedding engine attached and 1,417 atoms indexed.
	//
	// The query is the target plus any constraint, which is the richest text
	// available here — the same choice PromptAssembler.toCompilationContext
	// makes (SemanticQuery = UserIntent.Target), widened because a bare file
	// path embeds poorly and the constraint usually carries the real prose.
	cc.SemanticQuery = strings.TrimSpace(intent.Target + " " + intent.Constraint)
	if cc.SemanticQuery == "" {
		// A verb alone is a weak query, but it is not nothing, and an empty
		// string disables retrieval entirely.
		cc.SemanticQuery = strings.TrimPrefix(intent.Verb, "/")
	}
	if cc.SemanticTopK <= 0 {
		cc.SemanticTopK = defaultSemanticTopK
	}

	// Derive the persona this turn is acting as. Without it the compilation
	// context carries no /shard dimension, and jit_compiler.mg's
	// blocked_by_context only blocks an atom when the context HAS that
	// dimension — so every shard-gated atom in the corpus was admitted. A
	// single "explain this file" turn arrived with 114 mandatory atoms and
	// ~60k tokens containing 25+ contradictory identities (Nemesis, Coder,
	// Tester, Legislator, Perception Firewall, ...). The model then obeyed
	// whichever it latched onto — usually the Perception Layer's "you describe
	// what the user wants, the harness fulfills it" — and returned an intent
	// announcement instead of doing the work. That is the hollow-output class
	// previously treated as a model failure and papered over with retries.
	//
	// GetShardTypeForVerb is the same verb->persona mapping the routing layer
	// uses, so the prompt agrees with the router about who is acting.
	if shardType := perception.GetShardTypeForVerb(intent.Verb); shardType != "" {
		cc.ShardType = "/" + strings.TrimPrefix(shardType, "/")
		// ShardID selects the per-shard atom DB and must be the bare agent
		// name, not an instance id.
		cc.ShardID = strings.TrimPrefix(shardType, "/")
	} else if agent := UserAgentFromIntentVerb(intent.Verb); agent != "" {
		// The verb is not in the built-in taxonomy, which leaves exactly one
		// population: user-defined agents from .nerd/agents/<name>/prompts.yaml,
		// reached as "/consult/<name>" (chat delegation) or "/<name>"
		// (`nerd spawn <name>`, Cortex.SpawnTask).
		//
		// Two separate breakages were fixed by these two lines.
		//
		// ShardID: boot parses the agent's prompts.yaml into
		// .nerd/shards/<name>_knowledge.db and registers it with the JIT
		// compiler under the agent's name (internal/system/factory.go
		// RegisterAgentDBWithJIT). collectAtomsWithStats only reads that DB when
		// CompilationContext.ShardID names it. Nothing ever set ShardID to a
		// user agent's name, so every custom agent ran with a generic prompt and
		// its authored identity/methodology/domain atoms were dead weight on
		// disk.
		//
		// ShardType: jit_compiler.mg's blocked_by_context only excludes an atom
		// when the context HAS the shard dimension. With ShardType empty, every
		// shard-gated atom in the corpus was admitted, so a custom agent was
		// handed 25+ contradictory built-in identities (Coder, Nemesis, Tester,
		// Perception Firewall, ...) and answered as whichever it latched onto.
		// That is the same hollow-output failure documented above for the
		// persona-less case.
		cc.ShardType = "/" + agent
		cc.ShardID = agent
	}

	// Determine world states from kernel facts
	if e.kernel != nil {
		// Check for failing tests
		if facts, err := e.kernel.Query("test_state(/failing)"); err == nil {
			cc.FailingTestCount = len(facts)
		}

		// Check for active diagnostics
		if facts, err := e.kernel.Query("diagnostic"); err == nil {
			cc.DiagnosticCount = len(facts)
		}
	}

	// Set session context if available
	// Priority 1: Check context (thread-safe, request-scoped)
	if sCtx := types.GetSessionContext(ctx); sCtx != nil {
		cc.SessionContext = sCtx
		if sCtx.DreamMode {
			cc.OperationalMode = "/dream"
		}
		e.resolveAvailableTools(ctx, cc, intent)
		return cc
	}

	// Priority 2: Fallback to stateful context (legacy)
	e.mu.RLock()
	if e.sessionContext != nil {
		cc.SessionContext = e.sessionContext
		if e.sessionContext.DreamMode {
			cc.OperationalMode = "/dream"
		}
	}
	e.mu.RUnlock()
	e.resolveAvailableTools(ctx, cc, intent)
	return cc
}

// resolveAvailableTools populates cc.AvailableTools with the effective
// executable tool catalog BEFORE prompt compilation so Mangle can gate
// tool-specific atoms on the same envelope the tool loop will enforce.
// Precompiled EffectiveAgentRuntimeConfig (SubAgent injection) wins; otherwise
// the factory resolves [verb] exactly as compileConfig will. It never widens
// authority: failures leave the catalog empty (fail-closed, no tools).
func (e *Executor) resolveAvailableTools(ctx context.Context, cc *prompt.CompilationContext, intent perception.Intent) {
	if cc == nil {
		return
	}
	e.mu.RLock()
	precompiled := e.EffectiveAgentRuntimeConfig
	e.mu.RUnlock()
	if precompiled != nil {
		cc.AvailableTools = append([]string(nil), precompiled.AllowedTools...)
		return
	}
	if e.configFactory == nil {
		cc.AvailableTools = nil
		return
	}
	verb := intent.Verb
	if verb == "" {
		verb = "/general"
	}
	tools, err := e.resolveAllowedToolsSafely(ctx, verb)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("Tool envelope resolution failed for %q: %v (compiling with empty catalog)", verb, err)
		cc.AvailableTools = nil
		return
	}
	cc.AvailableTools = tools
}

// resolveAllowedToolsSafely calls the config factory with panic recovery. A
// factory that panics degrades exactly like one that errors — empty catalog —
// instead of crashing the turn; fail-closed parity with the error path above.
func (e *Executor) resolveAllowedToolsSafely(ctx context.Context, verb string) (resolved []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			resolved = nil
			err = fmt.Errorf("config factory ResolveAllowedTools panicked: %v", r)
		}
	}()
	return e.configFactory.ResolveAllowedTools(ctx, verb)
}

// compileConfig creates an EffectiveAgentRuntimeConfig from the compilation result and intent.
func (e *Executor) compileConfig(ctx context.Context, result *prompt.CompilationResult, intent perception.Intent) (*config.EffectiveAgentRuntimeConfig, error) {
	e.mu.RLock()
	if e.EffectiveAgentRuntimeConfig != nil {
		cfg := e.EffectiveAgentRuntimeConfig
		e.mu.RUnlock()
		return cfg, nil
	}
	e.mu.RUnlock()

	if e.configFactory == nil {
		return &config.EffectiveAgentRuntimeConfig{}, nil
	}

	// Use intent verb as the primary intent for config lookup
	intentVerb := intent.Verb
	if intentVerb == "" {
		intentVerb = "/general"
	}

	return e.generateAgentConfigSafely(ctx, result, intentVerb)
}

// generateAgentConfigSafely calls the config factory with panic recovery. A
// factory that panics degrades exactly like one that errors — the caller
// continues with an empty config — instead of crashing the turn.
func (e *Executor) generateAgentConfigSafely(ctx context.Context, result *prompt.CompilationResult, intentVerb string) (cfg *config.EffectiveAgentRuntimeConfig, err error) {
	defer func() {
		if r := recover(); r != nil {
			cfg = nil
			err = fmt.Errorf("config factory Generate panicked: %v", r)
		}
	}()
	return e.configFactory.Generate(ctx, result, intentVerb)
}

// generateResponse calls the LLM with the compiled prompt and tools for tool calling.
// Uses Piggyback Protocol for tools when the client supports it (e.g., Gemini with grounding).
//
// client is the turn's resolved LLM (see llmForVerb) and is passed explicitly
// rather than read from the struct so that every call in one tool loop provably
// hits the same model.
func (e *Executor) generateResponse(ctx context.Context, client types.LLMClient, systemPrompt, userInput string, cfg *config.EffectiveAgentRuntimeConfig) (*types.LLMToolResponse, error) {
	// A missing model client fails closed as an error, never as a nil-pointer
	// panic on the completion call below.
	if client == nil {
		return nil, fmt.Errorf("cannot generate response: no LLM client configured")
	}
	// Check if client should use Piggyback for tools (e.g., Gemini with grounding enabled)
	if ptp, ok := client.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		return e.generateResponseWithPiggybackTools(ctx, client, systemPrompt, userInput, cfg)
	}

	prior := e.priorTurnMessages()
	logging.SessionDebug("history window: %d prior messages (%d chars)", len(prior), historyMessageChars(prior))

	// Convert EffectiveAgentRuntimeConfig tool names to ToolDefinition structs
	toolDefs := e.buildToolDefinitions(cfg)
	if activeWorkingLoop(ctx) != nil {
		if provider, ok := client.(types.ToolResultsProvider); ok {
			return e.completeWithWorkingContext(ctx, provider, systemPrompt, []types.Message{{Role: "user", Text: userInput}}, toolDefs)
		}
		var prepareErr error
		systemPrompt, _, prepareErr = e.prepareWorkingRequest(ctx, systemPrompt, nil, nil)
		if prepareErr != nil {
			return nil, prepareErr
		}
	}

	// If we have tools, use native function calling; otherwise fall back to simple completion
	if len(toolDefs) > 0 {
		// When prior turns exist and the client speaks native multi-turn
		// tool calling, send them as messages so the model sees the
		// conversation instead of only the current input.
		if len(prior) > 0 {
			if trp, ok := client.(types.ToolResultsProvider); ok {
				history := make([]types.Message, 0, len(prior)+1)
				history = append(history, prior...)
				history = append(history, types.Message{Role: "user", Text: userInput})
				logging.Session("Calling LLM with %d tools via CompleteWithToolResults (%d prior messages)", len(toolDefs), len(prior))
				return e.completeWithWorkingContext(ctx, trp, systemPrompt, history, toolDefs)
			}
			// No native history channel: degrade to a compact transcript
			// prepended to the user prompt.
			userInput = renderHistoryTranscript(prior, userInput)
		}
		logging.Session("Calling LLM with %d tools via CompleteWithTools", len(toolDefs))
		return client.CompleteWithTools(ctx, systemPrompt, userInput, toolDefs)
	}
	logging.Session("No tools configured, using CompleteWithSystem")

	if len(prior) > 0 {
		userInput = renderHistoryTranscript(prior, userInput)
	}
	// No tools configured - use simple completion
	text, err := client.CompleteWithSystem(ctx, systemPrompt, userInput)
	if err != nil {
		return nil, err
	}
	e.warnOnDroppedToolRequests(text, cfg)
	return &types.LLMToolResponse{
		Text:       text,
		StopReason: "end_turn",
	}, nil
}

// warnOnDroppedToolRequests surfaces the case where the model asked for tools we
// never offered it.
//
// This path runs with an empty tool catalog, so a tool_request in the envelope
// cannot be executed -- but it MUST NOT be silent. Live, `nerd explain <file>`
// hit a verb with no config atom, got zero tools, and the model responded with
// required tool_requests for read_file and get_elements plus the surface text
// "reading the file now...". The harness printed that sentence, logged
// "Execution complete: 0 tool calls", and exited 0. To every downstream
// consumer -- including the meta-cognitive supervisor, which was told
// "Success: true" -- the turn had succeeded.
//
// The tool grant is the real fix (see NewDefaultConfigAtomProvider). This is the
// detector that keeps the next instance of it from being invisible.
func (e *Executor) warnOnDroppedToolRequests(text string, cfg *config.EffectiveAgentRuntimeConfig) {
	processed := articulation.ProcessLLMResponse(text)
	if processed.Control == nil || len(processed.Control.ToolRequests) == 0 {
		return
	}

	names := make([]string, 0, len(processed.Control.ToolRequests))
	required := 0
	for _, req := range processed.Control.ToolRequests {
		names = append(names, req.ToolName)
		if req.Required {
			required++
		}
	}

	verb := "<unknown>"
	if cfg != nil && cfg.IntentVerb != "" {
		verb = cfg.IntentVerb
	}
	logging.Get(logging.CategorySession).Warn(
		"Dropped %d tool_request(s) (%d required) for intent %s: the model asked for [%s] "+
			"but no tools were configured, so this turn answered blind and still reports success",
		len(names), required, verb, strings.Join(names, ", "))
}

// generateResponseWithPiggybackTools uses structured output for tool invocation.
// This enables tool use to coexist with Gemini's built-in grounding tools
// (Google Search, URL Context) which cannot be combined with native function calling.
func (e *Executor) generateResponseWithPiggybackTools(ctx context.Context, client types.LLMClient, systemPrompt, userInput string, cfg *config.EffectiveAgentRuntimeConfig) (*types.LLMToolResponse, error) {
	// Piggyback path keeps its structured-output envelope contract: history
	// arrives only as a rendered transcript in the user prompt, never as
	// native messages.
	prior := e.priorTurnMessages()
	logging.SessionDebug("history window: %d prior messages (%d chars)", len(prior), historyMessageChars(prior))
	effectiveInput := userInput
	if len(prior) > 0 {
		effectiveInput = renderHistoryTranscript(prior, userInput)
	}
	// Build tool catalog for injection into system prompt
	toolCatalog := e.buildToolCatalogForPiggyback(cfg)
	if toolCatalog != "" {
		systemPrompt = systemPrompt + "\n\n" + toolCatalog
		logging.Session("Injected tool catalog into system prompt for Piggyback++ (%d chars)", len(toolCatalog))
	}

	// Use CompleteWithSystem (supports grounding + structured output)
	// The Piggyback envelope will contain tool_requests
	schemaLen := len(articulation.GetPiggybackSchema(false))
	logging.Session("Using Piggyback++ for tool invocation (grounding-compatible mode, schema_len=%d)", schemaLen)
	text, err := client.CompleteWithSystem(ctx, systemPrompt, effectiveInput)
	if err != nil {
		return nil, err
	}

	processed := articulation.ProcessLLMResponse(text)
	if processed.Control != nil {
		envelope := &articulation.PiggybackEnvelope{
			Surface: processed.Surface,
			Control: *processed.Control,
		}
		// Process mangle_updates (including missing_tool_for for Ouroboros)
		e.processMangleUpdatesFromEnvelope(envelope)
		processed.Surface = envelope.Surface
		processed.Control = &envelope.Control
	}

	// Parse tool_requests from Piggyback envelope
	toolCalls := e.parseToolRequestsFromControl(processed.Control)
	logging.Session("Parsed %d tool_requests from Piggyback response", len(toolCalls))

	// Extract surface response (user-facing text)
	surfaceResponse := processed.Surface

	return &types.LLMToolResponse{
		Text:       surfaceResponse,
		ToolCalls:  toolCalls,
		StopReason: "end_turn",
	}, nil
}

// buildToolCatalogForPiggyback creates a unified tool catalog for prompt injection.
// This merges tools from both registries:
// 1. Modular tools (tools.Global()) - Go function handlers
// 2. Ouroboros tools (core.ToolRegistry) - compiled binary tools
func (e *Executor) buildToolCatalogForPiggyback(cfg *config.EffectiveAgentRuntimeConfig) string {
	// Keep prompt presentation aligned with the execution gate. In particular,
	// a registered Ouroboros tool is not a capability grant, and nil/empty JIT
	// configs expose no tools.
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		return ""
	}

	// Use json.MarshalIndent to ensure the example is always valid JSON
	exampleRequest := []map[string]any{{
		"id":        "req_1",
		"tool_name": "<tool_name>",
		"tool_args": map[string]any{"arg_name": "arg_value"},
		"purpose":   "why this tool is needed",
	}}

	exampleJSON, _ := json.MarshalIndent(exampleRequest, "", "  ")

	catalog := catalogBuilderPool.Get().(*strings.Builder)
	catalog.Reset()
	defer catalogBuilderPool.Put(catalog)
	catalog.WriteString("\n## Available Tools\n\n")
	catalog.WriteString("Request tools via `tool_requests` in control_packet:\n")
	catalog.WriteString("```json\n")
	catalog.WriteString("\"tool_requests\": ")
	catalog.Write(exampleJSON)
	catalog.WriteString("\n```\n\n")

	toolCount := 0

	// 1. Add modular tools from tools.Global()
	modularRegistry := tools.Global()
	modularToolCount := 0
	for _, toolName := range cfg.AllowedTools {
		tool := modularRegistry.Get(toolName)
		if tool == nil {
			continue
		}
		if modularToolCount == 0 {
			catalog.WriteString("### Built-in Tools\n\n")
		}
		catalog.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))
		// Add parameter hints if schema exists
		if len(tool.Schema.Required) > 0 {
			catalog.WriteString(fmt.Sprintf("  Required: %s\n", strings.Join(tool.Schema.Required, ", ")))
		}
		toolCount++
		modularToolCount++
	}
	if modularToolCount > 0 {
		catalog.WriteString("\n")
	}

	// 2. Add Ouroboros-generated tools
	e.mu.RLock()
	ouroborosReg := e.ouroborosRegistry
	e.mu.RUnlock()

	if ouroborosReg != nil {
		ouroborosTools := ouroborosReg.ListTools()
		ouroborosToolCount := 0
		for _, tool := range ouroborosTools {
			if !e.isToolAllowed(tool.Name, cfg) {
				continue
			}
			if ouroborosToolCount == 0 {
				catalog.WriteString("### Generated Tools (Ouroboros)\n\n")
			}
			catalog.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))
			if len(tool.Capabilities) > 0 {
				catalog.WriteString(fmt.Sprintf("  Capabilities: %s\n", strings.Join(tool.Capabilities, ", ")))
			}
			toolCount++
			ouroborosToolCount++
		}
		if ouroborosToolCount > 0 {
			catalog.WriteString("\n")
		}
	}

	// If no tools at all, return minimal catalog
	if toolCount == 0 {
		return ""
	}

	// Add tool generation encouragement
	catalog.WriteString("### Missing a Tool?\n\n")
	catalog.WriteString("If you need a capability not available above:\n")
	catalog.WriteString("1. Add a mangle_update: `missing_tool_for(\"<capability>\", \"<description>\")`\n")
	catalog.WriteString("2. The Ouroboros system will generate, compile, and register the tool\n")
	catalog.WriteString("3. The tool will be available in subsequent turns\n\n")
	catalog.WriteString("Example:\n")
	catalog.WriteString("```json\n")
	catalog.WriteString("\"mangle_updates\": [\"missing_tool_for(\\\"/parse_yaml\\\", \\\"Parse YAML files and return structured data\\\")\"]\n")
	catalog.WriteString("```\n")

	logging.Session("Built Piggyback++ tool catalog: %d tools (%d modular, %d ouroboros)",
		toolCount, modularToolCount, toolCount-modularToolCount)

	return catalog.String()
}

// parseToolRequestsFromControl extracts tool_requests from a control packet.
func (e *Executor) parseToolRequestsFromControl(control *articulation.ControlPacket) []types.ToolCall {
	if control == nil || len(control.ToolRequests) == 0 {
		return nil
	}

	var calls []types.ToolCall
	for _, req := range control.ToolRequests {
		calls = append(calls, types.ToolCall{
			ID:    req.ID,
			Name:  req.ToolName,
			Input: req.ToolArgs,
		})
	}
	return calls
}

// processMangleUpdatesFromEnvelope extracts and processes mangle_updates.
// This includes:
// 1. Asserting allowed Mangle facts to the kernel
// 2. Detecting missing_tool_for facts and triggering Ouroboros tool generation
func (e *Executor) processMangleUpdatesFromEnvelope(envelope *articulation.PiggybackEnvelope) {
	if envelope == nil || len(envelope.Control.MangleUpdates) == 0 {
		return
	}

	if e.kernel == nil {
		logging.SessionDebug("Skipping mangle_updates: no kernel configured")
		return
	}

	facts, blocked := core.FilterMangleUpdates(e.kernel, envelope.Control.MangleUpdates, core.ModelObservationPolicy())
	if len(blocked) > 0 {
		blockedAtoms := make([]string, 0, len(blocked))
		for _, b := range blocked {
			logging.SessionDebug("Blocked mangle_update %q: %s", b.Update, b.Reason)
			blockedAtoms = append(blockedAtoms, b.Update)
		}
		articulation.ApplyConstitutionalOverride(envelope, blockedAtoms, "blocked unsafe mangle_updates")
	}

	if len(facts) == 0 {
		return
	}

	if batcher, ok := e.kernel.(interface{ AssertBatch([]types.Fact) error }); ok {
		if err := batcher.AssertBatch(facts); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to assert mangle_updates batch: %v", err)
		}
	} else {
		for _, fact := range facts {
			if err := e.kernel.Assert(fact); err != nil {
				logging.Get(logging.CategorySession).Warn("Failed to assert mangle update: %v", err)
			}
		}
	}

	for _, fact := range facts {
		if fact.Predicate == "missing_tool_for" && len(fact.Args) >= 2 {
			intent := types.ExtractString(fact.Args[0])
			capability := types.ExtractString(fact.Args[1])
			logging.Session("Detected missing_tool_for: intent=%s capability=%s", intent, capability)
		}
	}
}

// parseMangleArgs parses comma-separated Mangle arguments.
// Handles quoted strings and atom constants.
func (e *Executor) parseMangleArgs(argsStr string) []any {
	var args []any
	var current strings.Builder
	inString := false
	escaped := false

	for _, ch := range argsStr {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			current.WriteRune(ch)
			continue
		}
		if ch == ',' && !inString {
			arg := strings.TrimSpace(current.String())
			if arg != "" {
				args = append(args, e.parseMangleArg(arg))
			}
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}

	// Add final argument
	arg := strings.TrimSpace(current.String())
	if arg != "" {
		args = append(args, e.parseMangleArg(arg))
	}

	return args
}

// parseMangleArg parses a single Mangle argument.
func (e *Executor) parseMangleArg(arg string) any {
	// String literal
	if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
		return arg[1 : len(arg)-1] // Remove quotes
	}
	// Atom constant
	if strings.HasPrefix(arg, "/") {
		return types.MangleAtom(arg)
	}
	// Number
	if n, err := fmt.Sscanf(arg, "%d", new(int)); n == 1 && err == nil {
		var i int
		fmt.Sscanf(arg, "%d", &i)
		return i
	}
	// Default: treat as string
	return arg
}

// buildToolDefinitions converts tool names from EffectiveAgentRuntimeConfig to ToolDefinition structs.
func (e *Executor) buildToolDefinitions(cfg *config.EffectiveAgentRuntimeConfig) []types.ToolDefinition {
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		logging.SessionDebug("buildToolDefinitions: no tools configured (cfg=%v)", cfg != nil)
		return nil
	}
	logging.Session("buildToolDefinitions: building %d tool definitions", len(cfg.AllowedTools))

	registry := tools.Global()
	var defs []types.ToolDefinition

	for _, toolName := range cfg.AllowedTools {
		tool := registry.Get(toolName)
		if tool == nil {
			logging.SessionDebug("Tool %s not found in registry", toolName)
			continue
		}

		// Build input schema from tool's schema
		inputSchema := make(map[string]any)
		inputSchema["type"] = "object"
		inputSchema["properties"] = tool.Schema.Properties
		if len(tool.Schema.Required) > 0 {
			inputSchema["required"] = tool.Schema.Required
		}

		defs = append(defs, types.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchema,
		})
	}

	logging.Session("Built %d tool definitions from %d allowed tools", len(defs), len(cfg.AllowedTools))
	return defs
}

// Bounds on the in-turn tool-result transcript.
//
// Tool results arrive whole (nothing cuts them any more; the working context
// archives each one and pages it), but the tool loop's `history` slice is
// append-only and is re-sent WHOLE on every CompleteWithToolResults call, so
// the sum of an exploration session's output would be replayed on every
// provider round-trip. Nothing bounded that sum.
//
// Eviction REWRITES the oldest tool results in place rather than deleting the
// messages that carry them. Anthropic-style APIs reject a request in which a
// tool_use block has no matching tool_result, so dropping a message is a hard
// 400 mid-turn; replacing its content with a marker keeps the pairing valid and
// tells the model, in the exact slot where the output used to be, that the
// output existed and is gone. The most recent results — the ones the next
// decision actually depends on — are never touched.
const (
	// maxToolLoopHistoryBytes caps the replayed tool-loop transcript
	// (~64k tokens). Sized to hold roughly 16 full-size tool results, which
	// covers the working set of a normal multi-step turn while refusing to
	// resend a whole exploration session on every round-trip.
	maxToolLoopHistoryBytes = 256 * 1024

	// evictedToolResultNotice replaces an evicted result's content. It names
	// what happened so the model re-reads the file rather than inventing what
	// the result said.
	evictedToolResultNotice = "[codenerd: this tool result was evicted from the transcript to stay inside the context budget. " +
		"Re-run the tool if you still need its output.]"

	// toolResultMarkerBudget reserves bytes for a clamped result's truncation
	// marker inside that result's share of the ceiling.
	toolResultMarkerBudget = 160

	// minClampedToolResultBytes is the floor a clamped current result keeps.
	// A batch of 50 pathological results would otherwise divide the ceiling
	// into slices too small to say anything, and a result trimmed to nothing
	// is indistinguishable from a tool that returned nothing.
	minClampedToolResultBytes = 2048
)

// boundToolLoopHistory caps the total bytes of a tool-loop transcript by
// blanking the oldest tool-result payloads, oldest-first, until the transcript
// fits. Message structure, ordering, and every ToolUseID are preserved.
//
// It is a no-op for a transcript already inside the ceiling, which is every
// ordinary turn.
func boundToolLoopHistory(history []types.Message) []types.Message {
	total := toolLoopHistoryBytes(history)
	if total <= maxToolLoopHistoryBytes {
		return history
	}

	// Copy-on-write: the caller's slice is shared with the provider call in
	// flight on the deadline path, and mutating a ToolResult in place would
	// change a message that has already been serialized.
	bounded := make([]types.Message, len(history))
	copy(bounded, history)

	// The newest tool-result message is the one the next decision is made
	// from; it is clamped, never blanked. Everything older is expendable in
	// full — if the model still needs it, re-running the tool is cheaper than
	// carrying the payload through every remaining round-trip.
	newest := -1
	for i := len(bounded) - 1; i >= 0; i-- {
		if len(bounded[i].ToolResults) > 0 {
			newest = i
			break
		}
	}

	evicted := 0
	for i := 0; i < len(bounded) && total > maxToolLoopHistoryBytes; i++ {
		if i == newest || len(bounded[i].ToolResults) == 0 {
			continue
		}
		results := make([]types.ToolResult, len(bounded[i].ToolResults))
		copy(results, bounded[i].ToolResults)
		for j := range results {
			if results[j].Content == evictedToolResultNotice {
				continue
			}
			total -= len(results[j].Content)
			total += len(evictedToolResultNotice)
			results[j].Content = evictedToolResultNotice
			evicted++
		}
		// Both views. Assigning the field alone would leave a block-built turn
		// sending the payload this eviction just accounted for as gone.
		bounded[i] = bounded[i].WithToolResults(results)
	}

	// Every older result is gone and the transcript is still over: the newest
	// batch alone exceeds the ceiling. Clamp it head+tail rather than blanking
	// it — a tool result's tail carries the error or the last hunk.
	clamped := 0
	if total > maxToolLoopHistoryBytes && newest >= 0 {
		results := make([]types.ToolResult, len(bounded[newest].ToolResults))
		copy(results, bounded[newest].ToolResults)

		// Budget the newest batch against what the rest of the transcript
		// already costs, then reserve each result's marker inside its share.
		// Reserving after the split is what keeps the marker from pushing the
		// transcript back over the ceiling it was added to respect.
		newestBytes := 0
		for _, r := range results {
			newestBytes += len(r.Content)
		}
		share := (maxToolLoopHistoryBytes - (total - newestBytes)) / max(1, len(results))
		share -= toolResultMarkerBudget
		if share < minClampedToolResultBytes {
			share = minClampedToolResultBytes
		}

		for j := range results {
			if len(results[j].Content) <= share {
				continue
			}
			total -= len(results[j].Content)
			results[j].Content = types.ClampText(results[j].Content, share, "tool result")
			total += len(results[j].Content)
			clamped++
		}
		bounded[newest] = bounded[newest].WithToolResults(results)
	}

	if evicted > 0 || clamped > 0 {
		logging.Get(logging.CategorySession).Warn(
			"Tool-loop transcript exceeded %d bytes; evicted %d stale tool result(s) and clamped %d current one(s)",
			maxToolLoopHistoryBytes, evicted, clamped)
	}
	return bounded
}

// toolLoopHistoryBytes totals the text a transcript will put on the wire.
func toolLoopHistoryBytes(history []types.Message) int {
	total := 0
	for _, m := range history {
		total += len(m.Text)
		for _, tc := range m.ToolCalls {
			total += len(tc.Name)
		}
		for _, tr := range m.ToolResults {
			total += len(tr.Content)
		}
	}
	return total
}

// maxHistoryTurnChars caps a single stored conversation turn.
//
// The turn count was capped at 50 but each turn's Content never was, and both
// ends of a turn are unbounded input: the user side is whatever was typed or
// piped (`nerd run "$(cat build.log)"`), the assistant side is whatever the
// model returned. One oversized turn used to do two things, both bad.
//
// It poisoned the replay window: priorTurnMessages drops whole messages
// oldest-first until the 24000-char budget is met, so a single 200 KB turn
// evicted EVERY prior turn and the model started the next turn with no memory
// at all — a context loss that looked like amnesia rather than truncation.
// And perception replays the last five turns into every classification call,
// so the same blob was re-sent on every turn until it aged out of a 50-slot
// window.
//
// 8000 chars is a third of DefaultHistoryCharBudget: three capped turns still
// fit the replay window, so the cap bounds a pathological turn without
// shrinking a normal conversation.
const maxHistoryTurnChars = 8000

// maxHistoryThoughtChars caps a stored reasoning summary. Thinking models emit
// these at arbitrary length and nothing downstream depends on their full text.
const maxHistoryThoughtChars = 2000

// appendToHistory adds a turn to conversation history, bounding the turn's text
// so one oversized turn cannot evict the rest of the window.
func (e *Executor) appendToHistory(turn perception.ConversationTurn) {
	turn.Content = types.ClampText(turn.Content, maxHistoryTurnChars, "conversation turn")
	turn.ThoughtSummary = types.ClampHead(turn.ThoughtSummary, maxHistoryThoughtChars, "thought summary")

	e.mu.Lock()
	defer e.mu.Unlock()

	e.conversationHistory = append(e.conversationHistory, turn)

	// Limit history size
	maxHistory := 50
	if len(e.conversationHistory) > maxHistory {
		e.conversationHistory = e.conversationHistory[len(e.conversationHistory)-maxHistory:]
	}
}

// ClearHistory clears the conversation history.
func (e *Executor) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.conversationHistory = make([]perception.ConversationTurn, 0)
}

// GetHistory returns a copy of the conversation history.
func (e *Executor) GetHistory() []perception.ConversationTurn {
	e.mu.RLock()
	defer e.mu.RUnlock()

	history := make([]perception.ConversationTurn, len(e.conversationHistory))
	copy(history, e.conversationHistory)
	return history
}

// priorTurnMessages converts the most recent conversation turns into LLM
// messages for the generating model. Perception keeps the full history; this
// is the bounded window generation is allowed to see.
//
// Bounds: HistoryTurnWindow messages (0 disables, negative treated as
// disabled) and HistoryCharBudget total characters (non-positive falls back
// to DefaultHistoryCharBudget). The window keeps the most recent messages;
// the character cap then drops the oldest first, two at a time, so a trimmed
// window never splits a user/assistant pair. Turns with empty Content are
// skipped. Role mapping is "user" -> "user", anything else -> "assistant".
//
// Eviction is announced, not silent. Whole turns leaving the window is the
// same event as a tool result being clamped — the model is given less than
// there was — and it used to leave nothing behind but a debug line the model
// never sees. A model whose first six turns were dropped answers "as I said
// earlier" questions from a transcript that no longer contains what it said,
// and has no way to tell that from a conversation that started here. The
// surviving oldest message now opens with the pipeline's marker naming how
// many messages and characters left, and the evicted messages themselves are
// retained on the executor (recoverHistoryEviction) so what was dropped can
// be brought back rather than reconstructed.
//
// The notice rides inside that message rather than arriving as a message of
// its own, which would put two user turns in a row and break the alternation
// strict providers require. When the oldest survivor is an assistant turn the
// notice therefore sits in assistant text, and the marker's bracketed prefix
// is what keeps it from reading as something the assistant said — that
// distinctive prefix is the reason the convention has one.
func (e *Executor) priorTurnMessages() []types.Message {
	cfg := e.configSnapshot()
	window := cfg.HistoryTurnWindow
	if window <= 0 {
		return nil
	}
	budget := cfg.HistoryCharBudget
	if budget <= 0 {
		budget = DefaultHistoryCharBudget
	}

	turns := e.GetHistory()
	if len(turns) == 0 {
		return nil
	}
	msgs := make([]types.Message, 0, len(turns))
	for _, turn := range turns {
		if turn.Content == "" {
			continue
		}
		role := "assistant"
		if turn.Role == "user" {
			role = "user"
		}
		msgs = append(msgs, types.Message{Role: role, Text: turn.Content})
	}
	if len(msgs) == 0 {
		return nil
	}
	eligible := len(msgs)
	var evicted []types.Message
	if len(msgs) > window {
		evicted = append(evicted, msgs[:len(msgs)-window]...)
		msgs = msgs[len(msgs)-window:]
	}
	total := historyMessageChars(msgs)
	for total > budget && len(msgs) > 0 {
		drop := min(2, len(msgs))
		total -= historyMessageChars(msgs[:drop])
		evicted = append(evicted, msgs[:drop]...)
		msgs = msgs[drop:]
	}
	e.recordHistoryEviction(evicted)
	if len(msgs) == 0 {
		// Nothing survived the budget, so there is no surviving message to
		// carry the marker. The window is empty and the caller renders no
		// history at all, which is honest on its own: the model is not shown
		// a partial transcript it could mistake for the whole one.
		return nil
	}
	if len(evicted) > 0 {
		notice := types.DroppedNotice(len(evicted), eligible,
			fmt.Sprintf("older conversation messages (%d chars) evicted from this window; the session still holds them",
				historyMessageChars(evicted)), "")
		msgs[0].Text = notice + "\n\n" + msgs[0].Text
	}
	return msgs
}

// recordHistoryEviction stores the messages priorTurnMessages dropped so the
// window's contents are recoverable rather than merely gone. It replaces the
// record each call because each call recomputes the window from the whole
// history: what is evicted now is what is evicted, not the union of every
// pass.
func (e *Executor) recordHistoryEviction(evicted []types.Message) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(evicted) == 0 {
		e.evictedHistory = nil
		return
	}
	e.evictedHistory = append([]types.Message(nil), evicted...)
}

// recoverHistoryEviction returns the messages the last window computation
// evicted, oldest first. Empty means nothing was dropped.
func (e *Executor) recoverHistoryEviction() []types.Message {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]types.Message(nil), e.evictedHistory...)
}

// historyMessageChars totals the text carried by prior messages.
func historyMessageChars(msgs []types.Message) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Text)
	}
	return total
}

// renderHistoryTranscript degrades prior messages to plain text for clients
// without a native history channel (no ToolResultsProvider, no tools, or the
// Piggyback envelope path whose contract is unchanged).
func renderHistoryTranscript(prior []types.Message, currentInput string) string {
	var sb strings.Builder
	sb.WriteString("Conversation so far:\n")
	for _, m := range prior {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Text)
		sb.WriteString("\n")
	}
	sb.WriteString("\nCurrent request:\n")
	sb.WriteString(currentInput)
	return sb.String()
}

// persistTurn stores the session turn for cross-session continuity and records
// the turn's cost denominator for tokens-per-verified-work.
// Best-effort: failures are logged but do not interrupt execution.
func (e *Executor) persistTurn(ctx context.Context, input string, intent perception.Intent, result *ExecutionResult, telemetry turnTelemetry) {
	if result == nil {
		return
	}
	e.mu.RLock()
	persister := e.sessionPersister
	sid := e.sessionID
	historyLen := len(e.conversationHistory)
	e.mu.RUnlock()

	// Determine session ID
	sessionID := sid
	if sessionID == "" {
		sessionID = "default"
	}

	// Turn number based on conversation history length (each turn = user + assistant = 2 entries)
	turnNumber := historyLen / 2

	// Cost denominator: usage delta across this turn plus the kernel verdict.
	promptTokens, completionTokens := telemetry.usageBefore.delta(snapshotTurnUsage(ctx, sessionID))
	outcome := e.resolveTurnOutcome(result)
	e.assertTurnCost(turnCost{
		sessionID:        sessionID,
		turnNumber:       turnNumber,
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
		toolCalls:        result.ToolCallsExecuted,
		outcome:          outcome,
	})

	// Report the turn to the learning sink with the same verdict turn_cost
	// records. This runs before the persister check on purpose: a delegated
	// task clone has no persister (see CloneForTask) and returning early would
	// have skipped exactly the executions worth learning from.
	e.recordContextFeedback(ContextFeedbackRecord{
		SessionID:  sessionID,
		TurnNumber: turnNumber,
		IntentVerb: intent.Verb,
		Verified:   outcome == types.MangleAtom("/done"),
	}, telemetry)

	provider, model := e.servingIdentity(intent.Verb)
	e.recordTurn(TurnRecord{
		SessionID:        sessionID,
		TurnNumber:       turnNumber,
		IntentVerb:       intent.Verb,
		Task:             input,
		Response:         result.Response,
		Outcome:          outcome,
		Err:              result.Error,
		AtomIDs:          turnAtomIDs(telemetry),
		Duration:         result.Duration,
		ToolCalls:        result.ToolCallsExecuted,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Provider:         provider,
		Model:            model,
	})

	if persister == nil {
		return
	}

	// Serialize intent for storage
	intentJSON, err := json.Marshal(intent)
	if err != nil {
		logging.Get(logging.CategorySession).Debug("Failed to marshal intent for persistence: %v", err)
		intentJSON = []byte("{}")
	}

	atomsJSON := compilationAtomsJSON(telemetry.compileResult)

	// Store asynchronously to avoid blocking the response
	go func() {
		if storeErr := persister.StoreSessionTurn(
			sessionID,
			turnNumber,
			input,
			string(intentJSON),
			result.Response,
			atomsJSON,
		); storeErr != nil {
			logging.Get(logging.CategorySession).Warn("Failed to persist session turn %d: %v", turnNumber, storeErr)
		} else {
			logging.SessionDebug("Persisted session turn %d for session %s", turnNumber, sessionID)
		}
	}()
}

// processPiggybackControlPacket performs best-effort parsing of the LLM response
// as a Piggyback Protocol envelope. When the LLM emits structured JSON with a
// control_packet, this method extracts and processes:
//   - Self-correction signals (logged + asserted to kernel)
//   - Memory operations (logged for future Cold Storage wiring)
//   - Mangle updates (asserted to kernel via existing processMangleUpdatesFromEnvelope)
//   - Context feedback (logged for spreading activation tuning)
//
// Returns the surface response (user-facing text). If parsing fails, returns
// the original raw text unchanged — this is best-effort, never fatal.
func (e *Executor) processPiggybackControlPacket(rawText string) string {
	// Best-effort parse — don't fail if the response isn't Piggyback-formatted
	processed := articulation.ProcessLLMResponseAllowPlain(rawText)
	if processed.Control == nil {
		// No control packet found — return raw surface as-is
		return processed.Surface
	}

	logging.Session("Piggyback control packet detected (method=%s, confidence=%.2f)",
		processed.ParseMethod, processed.Confidence)

	// Build envelope for helper functions
	envelope := articulation.PiggybackEnvelope{
		Surface: processed.Surface,
		Control: *processed.Control,
	}

	// --- Self-Correction ---
	if articulation.HasSelfCorrection(envelope) {
		hypothesis := envelope.Control.SelfCorrection.Hypothesis
		logging.Session("Piggyback self-correction triggered: %s", hypothesis)

		// Assert self-correction fact to kernel for autopoiesis tracking
		if e.kernel != nil {
			if err := e.kernel.Assert(types.Fact{
				Predicate: "self_correction",
				Args:      []any{hypothesis, time.Now().Unix()},
			}); err != nil {
				logging.Get(logging.CategorySession).Warn("Failed to assert self_correction fact: %v", err)
			}
		}
	}

	// --- Memory Operations ---
	if articulation.HasMemoryOperations(envelope) {
		memOps := envelope.Control.MemoryOperations
		logging.Session("Piggyback memory operations: %d total", len(memOps))

		// Log by operation type for visibility
		for _, opType := range []string{"promote_to_long_term", "forget", "store_vector", "note"} {
			ops := articulation.GetMemoryOperationsByType(envelope, opType)
			if len(ops) > 0 {
				for _, op := range ops {
					logging.SessionDebug("Piggyback memory op: %s key=%s value=%.100s", op.Op, op.Key, op.Value)
				}
			}
		}

		// Assert memory operation facts for future Cold Storage integration
		if e.kernel != nil {
			for _, op := range memOps {
				if err := e.kernel.Assert(types.Fact{
					Predicate: "memory_operation",
					Args:      []any{op.Op, op.Key, op.Value},
				}); err != nil {
					logging.Get(logging.CategorySession).Warn("Failed to assert memory_operation fact: %v", err)
				}
			}
		}
	}

	// --- Mangle Updates ---
	if len(envelope.Control.MangleUpdates) > 0 {
		logging.Session("Piggyback mangle_updates: %d atoms", len(envelope.Control.MangleUpdates))
		e.processMangleUpdatesFromEnvelope(&envelope)
	}

	// --- Context Feedback ---
	// The model has just told us which of the context we assembled earned its
	// tokens. This used to be logged and dropped on every path but the chat
	// TUI, which is the one path that was already keeping it. Stash it for
	// persistTurn, which has the turn number and the compiled manifest the
	// rating has to be filed against.
	if envelope.Control.ContextFeedback != nil {
		fb := envelope.Control.ContextFeedback
		logging.Session("Piggyback context feedback: usefulness=%.2f, helpful=%d, noise=%d",
			fb.OverallUsefulness, len(fb.HelpfulFacts), len(fb.NoiseFacts))
		if fb.MissingContext != "" {
			logging.SessionDebug("Piggyback missing context: %s", fb.MissingContext)
		}
		e.stashContextFeedback(fb)
	}

	// --- Intent Classification ---
	ic := envelope.Control.IntentClassification
	if ic.Category != "" || ic.Verb != "" {
		logging.SessionDebug("Piggyback intent: category=%s verb=%s target=%s confidence=%.2f",
			ic.Category, ic.Verb, ic.Target, ic.Confidence)
	}

	// Return only the surface response (control data has been routed to kernel)
	return processed.Surface
}

// turnSeq numbers turn verdicts across the process, so two executors sharing a
// kernel never mint the same turn.
var turnSeq atomic.Uint64

// newTurnAtom mints the key every fact of one turn verdict carries. The kernel
// is shared -- CloneForTask hands the same one to every delegated task, and a
// campaign runs tasks side by side -- so the verb cannot be a turn's identity:
// two concurrent /fix turns read each other's gates under it (external audit
// F1, 2026-09-19). The process id keeps two processes' turns apart in anything
// that outlives one of them.
func newTurnAtom() types.MangleAtom {
	return types.MangleAtom(fmt.Sprintf("/turn_%d_%d", os.Getpid(), turnSeq.Add(1)))
}

// turnAtom is this turn's key in the kernel, minted the first time a forcing
// round or the closure asks for it.
func (r *ExecutionResult) turnAtom() types.MangleAtom {
	if r.turn == "" {
		r.turn = newTurnAtom()
	}
	return r.turn
}

// assertTurnWrites asserts turn_written for each path the turn has written
// and not yet asserted, with its lower-case extension: the corpus decides from
// the extension what evidence the write owes (turn_owes_gate). A turn only
// adds writes, so a fact asserted before a forcing round is still true at the
// closure.
func (e *Executor) assertTurnWrites(turn types.MangleAtom, result *ExecutionResult) {
	if result.writtenAsserted == nil {
		result.writtenAsserted = make(map[string]bool, len(result.WrittenPaths))
	}
	for _, path := range result.WrittenPaths {
		if result.writtenAsserted[path] {
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if e.assertTurnFact(types.Fact{Predicate: "turn_written", Args: []any{turn, types.MangleString(path), types.MangleString(ext)}}) {
			result.writtenAsserted[path] = true
		}
	}
}

// assertTurnFact asserts one fact of this turn's verdict and records it for
// cleanupTurnFacts, which retracts exactly what was recorded.
func (e *Executor) assertTurnFact(fact types.Fact) bool {
	if err := e.kernel.Assert(fact); err != nil {
		logging.Get(logging.CategorySession).Warn("failed to assert %s%v: %v", fact.Predicate, fact.Args, err)
		return false
	}
	e.mu.Lock()
	e.turnFacts = append(e.turnFacts, fact)
	e.mu.Unlock()
	logging.Get(logging.CategorySession).Debug("asserted %s%v", fact.Predicate, fact.Args)
	return true
}

// assertTurnEvidence records this turn's observable outcome (Go measures) as
// facts keyed by the turn, so the policy corpus can derive the verdict (Mangle
// decides): one turn_evidence fact, the gates, the coverage debt, the
// acceptance witness and the files the turn created. cleanupTurnFacts retracts
// them on every checkHollowSuccess path, so a later turn never sees them, and
// they are keyed by turn, so a concurrent one never does either.
func (e *Executor) assertTurnEvidence(turn types.MangleAtom, verb string, result *ExecutionResult) {
	if e.kernel == nil || result == nil {
		return
	}
	// The mechanical gates are evidence about the workspace, and the policy
	// corpus already asks for them by name. Assert them before turn_evidence
	// so !turn_build_red(Turn) can exclude turn_executed on the same pass.
	e.recordBuildState(turn, result)
	if result.Acceptance != nil && result.Acceptance.Status == "verified" {
		e.assertTurnFact(types.Fact{Predicate: "turn_acceptance", Args: []any{turn, result.Acceptance.ContractID, result.Acceptance.After}})
	}
	claimedOutput := types.MangleAtom("/false")
	if responsePresentsTestRunnerOutput(result.Response) {
		claimedOutput = types.MangleAtom("/true")
	}
	dreamMode := types.MangleAtom("/false")
	if e.sessionContext != nil && e.sessionContext.DreamMode {
		dreamMode = types.MangleAtom("/true")
	}
	// A test run by the executor's own post-edit gate is execution, not a
	// claim: the model may quote that output in its answer. Only a run that
	// actually ran counts; a skipped gate produced nothing to quote.
	testRuns := result.TestRunCalls

	if result.TestCheck.Ran {
		testRuns++
	}
	e.assertTurnFact(types.Fact{Predicate: "turn_evidence", Args: []any{
		turn,
		types.MangleAtom(verb),
		result.SuccessfulToolCalls,
		result.SuccessfulWriteTools,
		testRuns,
		claimedOutput,
		dreamMode,
	}})
	// The files this turn created, from the executor's own record: the
	// new-source obligation (turn_missing_test) joins only these, so a file
	// another turn created -- or one the world scanner found -- cannot raise
	// or discharge it.
	e.mu.RLock()
	sources := append([]string(nil), e.turnCreatedSources...)
	tests := append([][2]string(nil), e.turnCreatedTests...)
	e.mu.RUnlock()
	seen := make(map[string]bool, len(sources)+len(tests))
	for _, file := range sources {
		if !seen[file] {
			seen[file] = true
			e.assertTurnFact(types.Fact{Predicate: "turn_created_source", Args: []any{turn, types.MangleString(file)}})
		}
	}
	for _, pair := range tests {
		if key := pair[0] + "\x00" + pair[1]; !seen[key] {
			seen[key] = true
			e.assertTurnFact(types.Fact{Predicate: "turn_created_test", Args: []any{turn, types.MangleString(pair[0]), types.MangleString(pair[1])}})
		}
	}
	e.assertTurnWrites(turn, result)
}

// recordBuildState asserts this turn's mechanical gate verdicts as the facts
// the policy corpus reads: turn_gate/3 for this turn, build_state/1 and
// test_state/1 for the session.
//
// Until this existed the field below it tracked was written by nothing —
// perTurnBuildStateFacts named a recordBuildState that was not in the tree, the
// only build_state assertions in the repo were in a test, and so the
// !build_state(/failing) conjunct of turn_executed
// (internal/core/defaults/policy/coder_safety.mg) excluded nothing in
// production: a turn whose build failed could still be turn_executed, and with
// an acceptance contract, turn_done.
//
// Only an affirmative verdict is asserted. A skipped, canceled or
// indeterminate gate proves nothing about the workspace, and asserting
// /passing for it would be the same guess this seam exists to delete —
// while asserting /failing would fail turns on a missing Go toolchain.
//
// Two facts per gate. build_state/1 and test_state/1 are the session-global
// workspace state the rest of the corpus reads (commit_gate.mg, tdd_loop.mg,
// context_compilation.mg) and other producers also write. turn_gate/3 is the
// same measurement keyed by this turn, and it is the only gate evidence the
// turn verdict (coder_safety.mg turn_verified / turn_executed /
// turn_build_failed) reads: a global left behind by an earlier turn's tool
// call is not this turn's evidence (REVIEW-wave1 F2), and neither is a
// concurrent turn's gate. Both are retracted with the turn's other evidence.
func (e *Executor) recordBuildState(turn types.MangleAtom, result *ExecutionResult) {
	if e.kernel == nil || result == nil {
		return
	}
	record := func(global string, gate types.MangleAtom, verdict VerifyOutcome) {
		var state types.MangleAtom
		switch verdict {
		case VerifyPassed:
			state = types.MangleAtom("/passing")
		case VerifyFailed:
			state = types.MangleAtom("/failing")
		default:
			return
		}
		if global != "" {
			e.assertTurnFact(types.Fact{Predicate: global, Args: []any{state}})
		}
		e.assertTurnFact(types.Fact{Predicate: "turn_gate", Args: []any{turn, gate, state}})
	}
	record("build_state", types.MangleAtom("/build"), result.BuildCheck.Verdict())
	record("test_state", types.MangleAtom("/test"), result.TestCheck.Verdict())
	// Vet has no session-global: turn_gate is its only record.
	record("", types.MangleAtom("/vet"), result.VetCheck.Verdict())
	// The test run after the last write has no session-global either.
	record("", types.MangleAtom("/test_run"), result.testRunVerdict())
	// Coverage debt rides with the gates: asserted here, retracted with them.
	// The corpus withholds turn_verified while any holds and names it as
	// turn_missing_evidence(Turn, /tests_not_written) or
	// (Turn, /changed_code_unexecuted).
	for _, path := range result.UntestedPaths {
		e.assertTurnFact(types.Fact{Predicate: "turn_untested", Args: []any{turn, path}})
	}
	for _, path := range uncoveredPaths(result) {
		e.assertTurnFact(types.Fact{Predicate: "turn_uncovered", Args: []any{turn, path}})
	}
}

// testRunVerdict is the /test_run gate's verdict: passed when the last test
// run since the turn's last write exited 0, failed when it exited otherwise,
// skipped -- no verdict -- when none ran since it.
func (r *ExecutionResult) testRunVerdict() VerifyOutcome {
	switch {
	case r.TestRunSinceLastWrite == nil:
		return VerifySkipped
	case r.TestRunSinceLastWrite.ExitCode == 0:
		return VerifyPassed
	default:
		return VerifyFailed
	}
}

// uncoveredPaths names each file holding blocks of this turn's changed code
// that no test executes, once, as the turn wrote it: the profile's
// import-qualified path is matched to the written path it ends with, and a
// block with no written match keeps its profile path.
func uncoveredPaths(result *ExecutionResult) []string {
	if result == nil || len(result.UncoveredBlocks) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var paths []string
	for _, b := range result.UncoveredBlocks {
		path := NormalizeCoverPath(b.File)
		for _, written := range result.WrittenPaths {
			if w := NormalizeCoverPath(written); w != "" && strings.HasSuffix(path, w) {
				path = w
				break
			}
		}
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

// turnRows returns the rows of a turn-keyed predicate that belong to this
// turn. Every verdict relation carries the turn as its first argument, and a
// row with another turn's key is another execution's verdict.
func (e *Executor) turnRows(predicate string, turn types.MangleAtom) ([]types.Fact, error) {
	facts, err := e.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	var own []types.Fact
	for _, f := range facts {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == string(turn) {
			own = append(own, f)
		}
	}
	return own, nil
}

// consumeHollowSuccessVerdict is the Go consumer of the policy-derived
// hollow_success verdict. hollow_success carries the failure reason; a
// present fact for this turn fails it with the matching hollow-success error.
// Query failures degrade to the imperative fallbacks below so MockKernel unit
// tests and degraded kernels still gate.
func (e *Executor) consumeHollowSuccessVerdict(turn types.MangleAtom, verb string, result *ExecutionResult) error {
	if e.kernel == nil {
		logging.Get(logging.CategorySession).Debug("checkHollowSuccess: nil kernel, using Go fallback checks (verb %s)", verb)
		return nil
	}
	if hollowFacts, qerr := e.turnRows("hollow_success", turn); qerr != nil {
		logging.Get(logging.CategorySession).Debug("checkHollowSuccess: hollow_success query failed: %v", qerr)
	} else if len(hollowFacts) > 0 {
		reasons := make([]string, 0, len(hollowFacts))
		for _, hf := range hollowFacts {
			r := ""
			if len(hf.Args) > 1 {
				r = types.ExtractString(hf.Args[1])
			}
			reasons = append(reasons, r)
		}
		sort.Strings(reasons)
		reason := reasons[0]
		switch reason {
		case "requires side effects but no tool call succeeded":
			attempted := 0
			if result != nil {
				attempted = result.ToolCallsExecuted
			}
			return newHollowSuccessError(
				"intent %s requires side effects but no tool calls completed successfully (attempted=%d)",
				verb, attempted,
			)
		case "write-oriented intent completed without a recognized write-mutation tool":
			calls := 0
			if result != nil {
				calls = result.ToolCallsExecuted
			}
			return newHollowSuccessError(
				"write-oriented intent %s completed without a recognized write-mutation tool (tool_calls=%d)",
				verb, calls,
			)
		case "response presents test-runner output but no test-execution tool ran":
			return newHollowSuccessError("response presents test-runner output but no test-execution tool ran this turn (verb %s)", verb)
		case "new source was created without a test file":
			// The reason derives from turn_missing_test for this turn, so the
			// file it names is this turn's creation and nothing older.
			var files []string
			if missing, merr := e.turnRows("turn_missing_test", turn); merr == nil {
				for _, f := range missing {
					if len(f.Args) > 1 {
						files = append(files, types.ExtractString(f.Args[1]))
					}
				}
			}
			if len(files) == 0 {
				return newHollowSuccessError("turn created Go source without a test file (verb %s)", verb)
			}
			sort.Strings(files)
			return newHollowSuccessError("turn created Go source %s without a test file (verb %s)", files[0], verb)
		default:
			return newHollowSuccessError("policy blocked hollow completion for intent %s (reason %s)", verb, reason)
		}
	}
	// Fallback for kernels without the turn_evidence rules (unit-test mocks):
	// imperative checks mirror the policy verdict so MockKernel turns still
	// gate. Policy decides when available; this only runs when no
	// hollow_success fact was derived for the turn.
	if result != nil {
		if e.intentRequiresToolCall(verb) || e.writeOrientedIntent(verb) {
			if result.SuccessfulToolCalls == 0 {
				return newHollowSuccessError(
					"intent %s requires side effects but no tool calls completed successfully (attempted=%d)",
					verb, result.ToolCallsExecuted,
				)
			}
		}
		if e.writeOrientedIntent(verb) && result.SuccessfulWriteTools == 0 {
			return newHollowSuccessError(
				"write-oriented intent %s completed without a recognized write-mutation tool (tool_calls=%d)",
				verb, result.ToolCallsExecuted,
			)
		}
	}
	return nil
}

// turnVerdict is what the kernel says about the turn whose evidence is still
// asserted. It is read once, in consumeTurnDoneSignal, and every surface that
// wants to know how the turn went reads the atom that read produced —
// result.TurnOutcome — rather than asking the kernel again or inspecting the
// checks itself.
type turnVerdict struct {
	// Answered is false when there was no kernel to ask (MockKernel unit
	// tests, degraded runtime). A consumer must not read "no verdict" as
	// "not done": it means nothing was asked.
	Answered bool

	// Done is turn_done — executed AND verified.
	Done bool

	// BuildFailed is turn_build_failed: build_state(/failing) this turn.
	BuildFailed bool

	// Missing are the turn_missing_evidence atoms (/build_not_green,
	// /tests_not_green). Empty for a verified turn and for a turn that
	// changed nothing.
	Missing []string
}

// consumeTurnDoneSignal is THE read of the turn's verdict from the kernel.
//
// It used to query turn_done, compare the row count to one, and write two Debug
// lines — the single completion signal had no consumer at all, and could not
// have had one: turn_done needed turn_acceptance, which only `nerd fix
// --acceptance` ever produced, so on every chat turn, campaign task and
// observer run the query was answered "no" by construction.
//
// Now the corpus derives verification from the mechanical gates
// (turn_verified, coder_safety.mg) and this is where that lands. It must be
// called while the per-turn facts are still asserted — cleanup retracts them
// and the derivation goes with them.
func (e *Executor) consumeTurnDoneSignal(turn types.MangleAtom, verb string) turnVerdict {
	var v turnVerdict
	if e.kernel == nil {
		logging.Get(logging.CategorySession).Debug("turn verdict: no kernel to ask for verb %s", verb)
		return v
	}
	v.Answered = true

	if doneFacts, derr := e.turnRows("turn_done", turn); derr != nil {
		logging.Get(logging.CategorySession).Debug("turn verdict: turn_done query failed: %v", derr)
		v.Answered = false
	} else {
		v.Done = len(doneFacts) > 0
		if len(doneFacts) > 1 {
			logging.Get(logging.CategorySession).Debug("turn verdict: turn_done count=%d for turn %s (expected at most one)", len(doneFacts), turn)
		}
	}

	if failFacts, ferr := e.turnRows("turn_build_failed", turn); ferr != nil {
		logging.Get(logging.CategorySession).Debug("turn verdict: turn_build_failed query failed: %v", ferr)
	} else {
		v.BuildFailed = len(failFacts) > 0
	}

	if missingFacts, merr := e.turnRows("turn_missing_evidence", turn); merr != nil {
		logging.Get(logging.CategorySession).Debug("turn verdict: turn_missing_evidence query failed: %v", merr)
	} else {
		seen := make(map[string]struct{}, len(missingFacts))
		for _, f := range missingFacts {
			if len(f.Args) < 2 {
				continue
			}
			atom := types.ExtractString(f.Args[1])
			if atom == "" {
				continue
			}
			if _, dup := seen[atom]; dup {
				continue
			}
			seen[atom] = struct{}{}
			v.Missing = append(v.Missing, atom)
		}
		sort.Strings(v.Missing)
	}

	logging.Get(logging.CategorySession).Debug("turn verdict for %s (%s): done=%t build_failed=%t missing=%v", verb, turn, v.Done, v.BuildFailed, v.Missing)
	return v
}

// missingEvidenceSentence renders one turn_missing_evidence atom as the clause
// a human reads. The corpus decides WHICH atoms hold; this only spells them.
func missingEvidenceSentence(atom string) string {
	switch atom {
	case "/build_not_green":
		return "the build was not verified green"
	case "/tests_not_green":
		return "the tests were not verified green"
	case "/tests_not_written":
		return "production code was written with no test beside it"
	case "/changed_code_unexecuted":
		return "code this turn changed is executed by no test"
	case "/vet_not_clean":
		return "go vet reports problems this turn introduced"
	case "/test_run_not_green":
		return "no test run passed after this turn's last write"
	default:
		return strings.TrimPrefix(atom, "/")
	}
}

// DescribeMissingEvidence joins the derived reasons into the phrase that
// follows "Unverified: ". Empty when nothing is missing.
func DescribeMissingEvidence(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	parts := make([]string, 0, len(missing))
	for _, atom := range missing {
		parts = append(parts, missingEvidenceSentence(atom))
	}
	return strings.Join(parts, "; ")
}
