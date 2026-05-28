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
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
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
}

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxToolCalls:      50,
		MaxToolIterations: 8,
		ToolTimeout:       5 * time.Minute,
		EnableSafetyGate:  true,
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

// SetSessionContext sets the session context for dream mode and shared state.
func (e *Executor) SetSessionContext(ctx *types.SessionContext) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionContext = ctx
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
	start := time.Now()
	logging.Session("Processing input: %d chars", len(input))

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

	// 1. OBSERVE: Transducer converts NL → Intent
	intent, err := e.observe(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("observation failed: %w", err)
	}
	result.Intent = intent

	// Assert intent to kernel for Mangle policy evaluation
	if e.kernel != nil {
		if assertErr := e.kernel.Assert(types.Fact{
			Predicate: "user_intent",
			Args: []any{
				types.MangleAtom("/current_intent"),
				types.MangleAtom(intent.Category),
				types.MangleAtom(intent.Verb),
				intent.Target,
				intent.Constraint,
			},
		}); assertErr != nil {
			logging.Get(logging.CategorySession).Warn("Failed to assert intent: %v", assertErr)
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
		compileResult, err = e.jitCompiler.Compile(ctx, compilationCtx)
		if err != nil {
			logging.Get(logging.CategorySession).Warn("JIT compilation failed, using baseline: %v", err)
			// Fall back to baseline prompt if JIT fails
			compileResult = &prompt.CompilationResult{
				Prompt: "You are an AI assistant helping with software development.",
			}
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

	// 5+6. LLM ↔ tools loop. The model may request tools, we execute them, then
	// feed the results back as a new turn — repeated until the model returns a
	// final answer with no tool calls, or we hit the iteration cap.
	llmResponse, toolErrs, err := e.runToolLoop(ctx, compileResult.Prompt, input, EffectiveAgentRuntimeConfig, result)
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
	cc := &prompt.CompilationContext{
		IntentVerb:      intent.Verb,
		IntentTarget:    intent.Target,
		OperationalMode: "/active",
		TokenBudget:     8192, // Default token budget for prompt compilation
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
func (e *Executor) generateResponse(ctx context.Context, systemPrompt, userInput string, cfg *config.EffectiveAgentRuntimeConfig) (*types.LLMToolResponse, error) {
	// Check if client should use Piggyback for tools (e.g., Gemini with grounding enabled)
	if ptp, ok := e.llmClient.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		return e.generateResponseWithPiggybackTools(ctx, systemPrompt, userInput, cfg)
	}

	// Convert EffectiveAgentRuntimeConfig tool names to ToolDefinition structs
	toolDefs := e.buildToolDefinitions(cfg)

	// If we have tools, use native function calling; otherwise fall back to simple completion
	if len(toolDefs) > 0 {
		logging.Session("Calling LLM with %d tools via CompleteWithTools", len(toolDefs))
		return e.llmClient.CompleteWithTools(ctx, systemPrompt, userInput, toolDefs)
	}
	logging.Session("No tools configured, using CompleteWithSystem")

	// No tools configured - use simple completion
	text, err := e.llmClient.CompleteWithSystem(ctx, systemPrompt, userInput)
	if err != nil {
		return nil, err
	}
	return &types.LLMToolResponse{
		Text:       text,
		StopReason: "end_turn",
	}, nil
}

// generateResponseWithPiggybackTools uses structured output for tool invocation.
// This enables tool use to coexist with Gemini's built-in grounding tools
// (Google Search, URL Context) which cannot be combined with native function calling.
func (e *Executor) generateResponseWithPiggybackTools(ctx context.Context, systemPrompt, userInput string, cfg *config.EffectiveAgentRuntimeConfig) (*types.LLMToolResponse, error) {
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
	text, err := e.llmClient.CompleteWithSystem(ctx, systemPrompt, userInput)
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
	if cfg != nil && len(cfg.AllowedTools) > 0 {
		catalog.WriteString("### Built-in Tools\n\n")
		for _, toolName := range cfg.AllowedTools {
			tool := modularRegistry.Get(toolName)
			if tool == nil {
				continue
			}
			catalog.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))
			// Add parameter hints if schema exists
			if len(tool.Schema.Required) > 0 {
				catalog.WriteString(fmt.Sprintf("  Required: %s\n", strings.Join(tool.Schema.Required, ", ")))
			}
			toolCount++
		}
		catalog.WriteString("\n")
	}

	// 2. Add Ouroboros-generated tools
	e.mu.RLock()
	ouroborosReg := e.ouroborosRegistry
	e.mu.RUnlock()

	if ouroborosReg != nil {
		ouroborosTools := ouroborosReg.ListTools()
		if len(ouroborosTools) > 0 {
			catalog.WriteString("### Generated Tools (Ouroboros)\n\n")
			for _, tool := range ouroborosTools {
				catalog.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))
				if len(tool.Capabilities) > 0 {
					catalog.WriteString(fmt.Sprintf("  Capabilities: %s\n", strings.Join(tool.Capabilities, ", ")))
				}
				toolCount++
			}
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
		toolCount, len(cfg.AllowedTools), toolCount-len(cfg.AllowedTools))

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

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// runToolLoop drives the LLM ↔ tools cycle. It performs the initial generation
// and, when the model requests tools, executes them and feeds the results back
// via ToolResultsProvider for as many turns as needed (up to MaxToolIterations).
//
// Returns the final LLM response, a slice of tool error messages encountered
// across all iterations, and any fatal error.
//
// Behavior degrades gracefully:
//   - If the client implements types.ToolResultsProvider: native multi-turn loop.
//   - Otherwise: one round of tool execution then return (the pre-fix behavior).
//     Tool failures are still recorded for the caller to surface.
//   - Piggyback Protocol clients keep using their structured-output path — the
//     loop currently runs for one iteration on that path because the
//     Piggyback envelope is its own contract; extending it is future work.
func (e *Executor) runToolLoop(
	ctx context.Context,
	systemPrompt, userInput string,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	llmResponse, err := e.generateResponse(ctx, systemPrompt, userInput, cfg)
	if err != nil {
		return nil, nil, err
	}

	// No tools requested: done.
	if len(llmResponse.ToolCalls) == 0 {
		return llmResponse, nil, nil
	}

	// Piggyback Protocol path: execute tools but don't continue the loop here.
	// (The Piggyback envelope carries tool_requests in structured output; a
	// proper loop for that path would require re-invoking with a synthesized
	// envelope. Out of scope for this fix.)
	if ptp, ok := e.llmClient.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		toolErrs := e.executeToolBatchPiggyback(ctx, llmResponse.ToolCalls, cfg, result)
		return llmResponse, toolErrs, nil
	}

	// Native multi-turn tool calling required for correct semantics on
	// Anthropic/OpenAI-style providers.
	trp, supportsLoop := e.llmClient.(types.ToolResultsProvider)
	toolDefs := e.buildToolDefinitions(cfg)

	// Seed the history with the initial user turn and the assistant's
	// first response (which contains the tool_use blocks).
	history := []types.Message{
		{Role: "user", Text: userInput},
		{Role: "assistant", Text: llmResponse.Text, ToolCalls: llmResponse.ToolCalls},
	}

	maxIter := e.config.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 8
	}

	var toolErrs []string
	currentResponse := llmResponse

	for iter := 0; iter < maxIter; iter++ {
		if ctx.Err() != nil {
			return currentResponse, toolErrs, ctx.Err()
		}

		// Execute all tool calls from this turn and collect tool_result blocks.
		toolResults := make([]types.ToolResult, 0, len(currentResponse.ToolCalls))
		for _, call := range currentResponse.ToolCalls {
			if result.ToolCallsExecuted >= e.config.MaxToolCalls {
				logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", e.config.MaxToolCalls)
				toolResults = append(toolResults, types.ToolResult{
					ToolUseID: call.ID,
					Content:   "tool call budget exceeded for this turn",
					IsError:   true,
				})
				toolErrs = append(toolErrs, fmt.Sprintf("%s: budget exceeded", call.Name))
				continue
			}

			toolCall := ToolCall{
				ID:   call.ID,
				Name: call.Name,
				Args: call.Input,
			}
			out, execErr := e.executeToolCall(ctx, toolCall, cfg)
			result.ToolCallsExecuted++

			if execErr != nil {
				logging.Get(logging.CategorySession).Error("Tool call %s failed: %v", call.Name, execErr)
				if ctx.Err() != nil {
					return currentResponse, toolErrs, ctx.Err()
				}
				toolErrs = append(toolErrs, fmt.Sprintf("%s: %v", call.Name, execErr))
				toolResults = append(toolResults, types.ToolResult{
					ToolUseID: call.ID,
					Content:   execErr.Error(),
					IsError:   true,
				})
				continue
			}

			logging.SessionDebug("Tool %s executed successfully: %d chars result", call.Name, len(out))
			toolResults = append(toolResults, types.ToolResult{
				ToolUseID: call.ID,
				Content:   truncateToolResult(out),
				IsError:   false,
			})
		}

		// If the client can't accept tool results back, we're done after the
		// first execution pass. The model will not see the results this turn.
		if !supportsLoop {
			logging.Get(logging.CategorySession).Warn(
				"LLM client does not implement ToolResultsProvider; tool results not fed back to model. Provider=%T", e.llmClient)
			return currentResponse, toolErrs, nil
		}

		// Append the user tool_result turn to history and re-invoke.
		history = append(history, types.Message{
			Role:        "user",
			ToolResults: toolResults,
		})

		nextResp, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, toolDefs)
		if err != nil {
			return currentResponse, toolErrs, fmt.Errorf("tool-result follow-up failed: %w", err)
		}
		currentResponse = nextResp

		// Append the next assistant turn to history (whether or not it has more tool calls).
		history = append(history, types.Message{
			Role:      "assistant",
			Text:      nextResp.Text,
			ToolCalls: nextResp.ToolCalls,
		})

		if len(nextResp.ToolCalls) == 0 {
			// Model returned a final answer.
			return currentResponse, toolErrs, nil
		}
	}

	logging.Get(logging.CategorySession).Warn("Max tool iterations reached: %d", maxIter)
	return currentResponse, toolErrs, nil
}

// executeToolBatchPiggyback handles the single-turn Piggyback path. Tools are
// executed and any errors collected, but results are not fed back to the LLM
// here (that would require a Piggyback-specific envelope follow-up).
func (e *Executor) executeToolBatchPiggyback(
	ctx context.Context,
	calls []types.ToolCall,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) []string {
	var toolErrs []string
	for _, call := range calls {
		if result.ToolCallsExecuted >= e.config.MaxToolCalls {
			logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", e.config.MaxToolCalls)
			break
		}
		toolCall := ToolCall{
			ID:   call.ID,
			Name: call.Name,
			Args: call.Input,
		}
		out, execErr := e.executeToolCall(ctx, toolCall, cfg)
		result.ToolCallsExecuted++
		if execErr != nil {
			logging.Get(logging.CategorySession).Error("Tool call %s failed: %v", call.Name, execErr)
			if ctx.Err() != nil {
				return toolErrs
			}
			toolErrs = append(toolErrs, fmt.Sprintf("%s: %v", call.Name, execErr))
			continue
		}
		logging.SessionDebug("Tool %s executed successfully: %d chars result", call.Name, len(out))
	}
	return toolErrs
}

// truncateToolResult caps tool output before feeding it back to the model.
// Massive output (greps, file dumps) wastes context budget for diminishing
// returns; 16 KB is enough for typical agent decisions.
func truncateToolResult(s string) string {
	const limit = 16 * 1024
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n...[truncated]"
}

// executeToolCall routes a tool call through the appropriate registry with safety checks.
// It checks both registries in order:
// 1. Modular tools (tools.Global()) - Go function handlers
// 2. Ouroboros tools (core.ToolRegistry) - compiled binary tools
func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.EffectiveAgentRuntimeConfig) (string, error) {
	// Check if tool is allowed by config (for modular tools) or exists in Ouroboros
	if !e.isToolAllowed(call.Name, cfg) && !e.isOuroborosTool(call.Name) {
		return "", fmt.Errorf("tool %s not allowed by config and not in Ouroboros registry", call.Name)
	}

	// Safety check via Constitutional Gate
	if e.config.EnableSafetyGate {
		if !e.checkSafety(call) {
			return "", fmt.Errorf("tool call blocked by safety gate: %s", call.Name)
		}
	}

	// Apply timeout to tool execution
	toolCtx, cancel := context.WithTimeout(ctx, e.config.ToolTimeout)
	defer cancel()

	// Route to appropriate registry
	// 1. Try modular tool registry first (Go function handlers)
	modularRegistry := tools.Global()
	if modularRegistry.Has(call.Name) {
		logging.Session("Executing modular tool: %s with %d args", call.Name, len(call.Args))
		result, err := modularRegistry.Execute(toolCtx, call.Name, call.Args)
		if err != nil {
			return "", fmt.Errorf("modular tool execution failed: %w", err)
		}
		if result.Error != nil {
			return "", fmt.Errorf("modular tool returned error: %w", result.Error)
		}
		return result.Result, nil
	}

	// 2. Try Ouroboros registry (compiled binary tools)
	e.mu.RLock()
	ouroborosReg := e.ouroborosRegistry
	e.mu.RUnlock()

	if ouroborosReg != nil {
		if _, exists := ouroborosReg.GetTool(call.Name); exists {
			logging.Session("Executing Ouroboros tool: %s with %d args", call.Name, len(call.Args))
			// Convert args map to JSON string for binary execution
			argsJSON, err := json.Marshal(call.Args)
			if err != nil {
				return "", fmt.Errorf("failed to marshal Ouroboros tool args: %w", err)
			}
			result, err := ouroborosReg.ExecuteRegisteredTool(toolCtx, call.Name, []string{string(argsJSON)})
			if err != nil {
				return "", fmt.Errorf("Ouroboros tool execution failed: %w", err)
			}
			return result, nil
		}
	}

	return "", fmt.Errorf("tool %s not found in any registry", call.Name)
}

// isOuroborosTool checks if a tool exists in the Ouroboros registry.
func (e *Executor) isOuroborosTool(toolName string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.ouroborosRegistry == nil {
		return false
	}
	_, exists := e.ouroborosRegistry.GetTool(toolName)
	return exists
}

// isToolAllowed checks if a tool is in the allowed list.
func (e *Executor) isToolAllowed(toolName string, cfg *config.EffectiveAgentRuntimeConfig) bool {
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		return true // No restrictions
	}

	return slices.Contains(cfg.AllowedTools, toolName)
}

// maxPayloadBytes caps the JSON-serialized tool args we'll push into the
// Mangle kernel. Large blobs (file dumps, base64 images) bloat the fact store
// and the permitted/pending_action comparison would never match anyway.
const maxPayloadBytes = 100 * 1024 // 100 KB

// checkSafety verifies a tool call against the Constitutional Gate.
func (e *Executor) checkSafety(call ToolCall) bool {
	// Categorically reject empty tool names — they would assert "/" as the
	// action atom, which is meaningless and bypasses meaningful policy match.
	if strings.TrimSpace(call.Name) == "" {
		logging.Get(logging.CategorySession).Warn("Safety check denied: empty tool call name")
		return false
	}

	if e.kernel == nil {
		// If the safety gate is enabled, missing kernel must FAIL CLOSED.
		// Otherwise the agent effectively runs in "god mode" on kernel init failure.
		if e.config.EnableSafetyGate {
			logging.Get(logging.CategorySession).Error("Safety check failed closed: kernel is nil while EnableSafetyGate=true")
			return false
		}
		return true // Gate disabled: allow
	}

	// 1. Prepare Mangle terms
	// Action names must be Mangle atoms (start with /)
	actionName := call.Name
	if !strings.HasPrefix(actionName, "/") {
		actionName = "/" + actionName
	}
	actionAtom := types.MangleAtom(actionName)

	// Normalize nil Args to empty map so json.Marshal produces "{}" instead
	// of "null" — the permitted facts written by policy use "{}" for no-arg
	// actions, so matching depends on this consistency.
	if call.Args == nil {
		call.Args = map[string]any{}
	}

	// Extract target and serialize payload
	target := e.extractTarget(call.Args)
	payloadBytes, err := json.Marshal(call.Args)
	if err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: cannot marshal args: %v", err)
		return false
	}
	// Reject oversized payloads outright. Truncating would silently break the
	// permitted-fact comparison (truncated payload != permitted payload), so
	// the safer contract is: refuse loudly.
	if len(payloadBytes) > maxPayloadBytes {
		logging.Get(logging.CategorySession).Error(
			"Safety check denied: payload too large for kernel (%d bytes > %d)",
			len(payloadBytes), maxPayloadBytes)
		return false
	}
	payload := string(payloadBytes)
	timestamp := time.Now().Unix()

	// 2. Assert pending_action
	// Decl pending_action(ActionID, ActionType, Target, Payload, Timestamp)
	pendingFact := types.Fact{
		Predicate: "pending_action",
		Args: []any{
			call.ID,
			actionAtom,
			target,
			payload,
			timestamp,
		},
	}

	if err := e.kernel.Assert(pendingFact); err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: assertion error: %v", err)
		return false
	}

	// Ensure cleanup of pending_action
	defer func() {
		if err := e.kernel.RetractFact(pendingFact); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to retract pending_action: %v", err)
		}
	}()

	// 3. Query permitted
	// permitted(Action, Target, Payload)
	// We query for all permitted facts and filter for matching this exact request.
	facts, err := e.kernel.Query("permitted")
	if err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: query error: %v", err)
		return false
	}

	for _, f := range facts {
		if len(f.Args) != 3 {
			continue
		}

		// Check Action (Handle both MangleAtom and string types)
		factAction := types.ExtractString(f.Args[0])
		if factAction != string(actionAtom) {
			continue
		}

		// Check Target
		factTarget := types.ExtractString(f.Args[1])
		if factTarget != target {
			continue
		}

		// Check Payload
		factPayload := types.ExtractString(f.Args[2])
		if factPayload != payload {
			continue
		}

		// Match found!
		return true
	}

	logging.Get(logging.CategorySession).Warn("Safety check denied action: %s (target: %s)", actionName, target)
	// TODO(improvement): Add more granular safety policies here. Currently it's a binary allow/deny.
	return false
}

// extractTarget attempts to identify the primary target of a tool call.
func (e *Executor) extractTarget(args map[string]any) string {
	// Common keys for targets
	candidates := []string{"path", "filename", "filepath", "file", "url", "target", "query"}
	for _, key := range candidates {
		if val, ok := args[key]; ok {
			return types.ExtractString(val)
		}
	}
	return "unknown"
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
