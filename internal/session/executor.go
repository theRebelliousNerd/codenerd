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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/projectdoc"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// JITCompiler compiles prompt atoms for the current context.
type JITCompiler interface {
	Compile(ctx context.Context, compilationCtx *prompt.CompilationContext) (*prompt.CompilationResult, error)
}

// ConfigFactory creates EffectiveAgentRuntimeConfig from compilation results.
type ConfigFactory interface {
	Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error)
}

// SessionPersister stores session turn data for cross-session continuity.
type SessionPersister interface {
	StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error
	StoreCompressedState(sessionID string, turnNumber int, stateJSON string, ratio float64) error
}

// InteractiveExecutiveGate is the optional capability a VirtualStore can expose
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

	// Session persistence
	sessionPersister SessionPersister
	sessionID        string

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
	fileContext FileContextProvider
}

// ExecutorConfig holds configuration for the executor.
type ExecutorConfig struct {
	// MaxToolCalls limits tool calls per turn to prevent runaway execution.
	MaxToolCalls int

	// MaxToolIterations limits the number of LLM → tools → LLM loop iterations
	// in a single Process() call. Without this cap, a model that keeps requesting
	// tools could spin forever. 0 falls back to the default.
	MaxToolIterations int

	// ToolTimeout is the maximum time for a single tool execution.
	ToolTimeout time.Duration

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
}

// DefaultTokenBudget is the prompt-compilation budget used when no
// ExecutorConfig/SpawnerConfig override is set. 65,536 tokens is a
// safe sub-agent default that survives on Claude/Gemini/GPT context
// windows ≥128K and still leaves headroom for response + tool I/O.
const DefaultTokenBudget = 65536

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxToolCalls:      50,
		MaxToolIterations: 8,
		ToolTimeout:       5 * time.Minute,
		EnableSafetyGate:  true,
		TokenBudget:       DefaultTokenBudget,
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
	if e.kernel == nil {
		return false
	}

	// Intent verbs arrive as Mangle atoms already ("/review", "/campaign"), so
	// they need no quoting — but a verb that is not an atom would be a syntax
	// error in the query, so reject anything unexpected rather than build one.
	if !strings.HasPrefix(verb, "/") {
		logging.SessionDebug("intentRequiresReasoningModel: %q is not an atom, defaulting to false", verb)
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
	clone.plannerClient = e.plannerClient
	clone.projectDoc = e.projectDoc
	clone.fileContext = e.fileContext
	// Deliberately NOT copied: conversationHistory, sessionContext,
	// sessionPersister/sessionID (task runs must not be recorded as session
	// turns), EffectiveAgentRuntimeConfig (set per task by the caller).
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
func (e *Executor) SetOuroborosRegistry(registry *core.ToolRegistry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ouroborosRegistry = registry
	logging.Session("Ouroboros registry configured with %d tools", len(registry.ListTools()))
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

// ExecutionResult holds the result of processing user input.
type ExecutionResult struct {
	// Response is the text response to show the user.
	Response string

	// Intent is the parsed user intent.
	Intent perception.Intent

	// ToolCallsExecuted is the number of tool calls made.
	ToolCallsExecuted int

	// SuccessfulWriteTools counts write_file/edit_file (and peers) that
	// completed without error. Used to block hollow success on write-oriented
	// intents that only produced prose or non-mutating tool calls.
	SuccessfulWriteTools int

	// Duration is how long the execution took.
	Duration time.Duration

	// Error is set if execution failed.
	Error error
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
	systemPrompt := e.withProjectInstructions(compileResult.Prompt)
	systemPrompt = e.withFileContext(ctx, systemPrompt, intent.Target)

	// 5+6. LLM ↔ tools loop. The model may request tools, we execute them, then
	// feed the results back as a new turn — repeated until the model returns a
	// final answer with no tool calls, or we hit the iteration cap.
	llmResponse, toolErrs, err := e.runToolLoop(ctx, systemPrompt, input, EffectiveAgentRuntimeConfig, compilationCtx, result)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// 7. Articulate response — process Piggyback control packet (best-effort)
	result.Response = e.processPiggybackControlPacket(llmResponse.Text)
	result.Duration = time.Since(start)

	// Surface unrecovered tool failures as the execution error. A tool that
	// errored is considered "recovered" if the model ultimately produced a
	// final text response without re-requesting that tool. We can't know that
	// precisely, but as a conservative heuristic: if the final response is
	// empty AND tools errored, treat it as execution failure so the learning
	// trace doesn't record success.
	if len(toolErrs) > 0 && strings.TrimSpace(result.Response) == "" {
		result.Error = fmt.Errorf("tool execution failed: %s", strings.Join(toolErrs, "; "))
	}

	// Block hollow success: mutation intents that require real side effects
	// must not report completion when the model only returned planning prose
	// (or only non-mutating tools). Dream/shadow runs skip this gate.
	//
	// Hollow failures are hard errors (non-nil return) so CLI one-shots exit
	// non-zero. Other soft tool failures stay on result.Error with a nil
	// return for interactive chat compatibility; TaskExecutor still surfaces
	// result.Error for SpawnTask callers.
	if result.Error == nil {
		if hollowErr := e.checkHollowSuccess(result); hollowErr != nil {
			result.Error = hollowErr
		}
	}

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

	// Dispatch asynchronous learning. Success is only true if no tool errored
	// AND the executor produced a response.
	if perception.SharedTaxonomy != nil {
		trace := perception.ReasoningTrace{
			UserPrompt: input,
			Response:   result.Response,
			Success:    result.Error == nil && len(toolErrs) == 0,
		}
		perception.SharedTaxonomy.QueueForLearning([]perception.ReasoningTrace{trace})
	}

	// Persist session turn for cross-session continuity
	e.persistTurn(input, intent, result)

	logging.Session("Execution complete: %d tool calls, %v duration", result.ToolCallsExecuted, result.Duration)

	if result.Error != nil && isHollowSuccessError(result.Error) {
		return result, result.Error
	}
	return result, nil
}

// observe uses the transducer to convert natural language to intent.
func (e *Executor) observe(ctx context.Context, input string) (perception.Intent, error) {
	e.mu.RLock()
	history := e.conversationHistory
	e.mu.RUnlock()

	return e.transducer.ParseIntentWithContext(ctx, input, history)
}

// buildCompilationContext creates a CompilationContext from the current state.
func (e *Executor) buildCompilationContext(ctx context.Context, intent perception.Intent) *prompt.CompilationContext {
	budget := e.config.TokenBudget
	if budget <= 0 {
		budget = DefaultTokenBudget
	}
	cc := &prompt.CompilationContext{
		IntentVerb:      intent.Verb,
		IntentTarget:    intent.Target,
		OperationalMode: "/active",
		TokenBudget:     budget,
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

	return cc
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

	return e.configFactory.Generate(ctx, result, intentVerb)
}

// generateResponse calls the LLM with the compiled prompt and tools for tool calling.
// Uses Piggyback Protocol for tools when the client supports it (e.g., Gemini with grounding).
//
// client is the turn's resolved LLM (see llmForVerb) and is passed explicitly
// rather than read from the struct so that every call in one tool loop provably
// hits the same model.
func (e *Executor) generateResponse(ctx context.Context, client types.LLMClient, systemPrompt, userInput string, cfg *config.EffectiveAgentRuntimeConfig) (*types.LLMToolResponse, error) {
	// Check if client should use Piggyback for tools (e.g., Gemini with grounding enabled)
	if ptp, ok := client.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		return e.generateResponseWithPiggybackTools(ctx, client, systemPrompt, userInput, cfg)
	}

	// Convert EffectiveAgentRuntimeConfig tool names to ToolDefinition structs
	toolDefs := e.buildToolDefinitions(cfg)

	// If we have tools, use native function calling; otherwise fall back to simple completion
	if len(toolDefs) > 0 {
		logging.Session("Calling LLM with %d tools via CompleteWithTools", len(toolDefs))
		return client.CompleteWithTools(ctx, systemPrompt, userInput, toolDefs)
	}
	logging.Session("No tools configured, using CompleteWithSystem")

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
	text, err := client.CompleteWithSystem(ctx, systemPrompt, userInput)
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

	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"missing_tool_for":  {},
			"observation":       {},
			"task_status":       {},
			"task_completed":    {},
			"diagnostic":        {},
			"failing_test":      {},
			"test_state":        {},
			"review_finding":    {},
			"modified":          {},
			"modified_function": {},
		},
		MaxUpdates: 100,
	}

	facts, blocked := core.FilterMangleUpdates(e.kernel, envelope.Control.MangleUpdates, policy)
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

// appendToHistory adds a turn to conversation history.
func (e *Executor) appendToHistory(turn perception.ConversationTurn) {
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

// persistTurn stores the session turn for cross-session continuity.
// Best-effort: failures are logged but do not interrupt execution.
func (e *Executor) persistTurn(input string, intent perception.Intent, result *ExecutionResult) {
	e.mu.RLock()
	persister := e.sessionPersister
	sid := e.sessionID
	historyLen := len(e.conversationHistory)
	e.mu.RUnlock()

	if persister == nil {
		return
	}

	// Determine session ID
	sessionID := sid
	if sessionID == "" {
		sessionID = "default"
	}

	// Serialize intent for storage
	intentJSON, err := json.Marshal(intent)
	if err != nil {
		logging.Get(logging.CategorySession).Debug("Failed to marshal intent for persistence: %v", err)
		intentJSON = []byte("{}")
	}

	// Turn number based on conversation history length (each turn = user + assistant = 2 entries)
	turnNumber := historyLen / 2

	// Store asynchronously to avoid blocking the response
	go func() {
		if storeErr := persister.StoreSessionTurn(
			sessionID,
			turnNumber,
			input,
			string(intentJSON),
			result.Response,
			"", // atomsJSON — populated by piggyback integration
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
	if envelope.Control.ContextFeedback != nil {
		fb := envelope.Control.ContextFeedback
		logging.Session("Piggyback context feedback: usefulness=%.2f, helpful=%d, noise=%d",
			fb.OverallUsefulness, len(fb.HelpfulFacts), len(fb.NoiseFacts))
		if fb.MissingContext != "" {
			logging.SessionDebug("Piggyback missing context: %s", fb.MissingContext)
		}
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
