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
	"fmt"
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

// MangleAtom wraps a string as a Mangle name constant (avoids core import).
type MangleAtom string

func (m MangleAtom) String() string { return string(m) }

// sessionContextKey is the context key for passing SessionContext.
type sessionContextKeyType struct{}

var sessionContextKey = sessionContextKeyType{}

// WithSessionContext returns a context with the SessionContext attached.
// This enables passing session context through the executor loop without
// relying on stateful Executor fields (thread-safe).
func WithSessionContext(ctx context.Context, sessionCtx *types.SessionContext) context.Context {
	return context.WithValue(ctx, sessionContextKey, sessionCtx)
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
	jitCompiler   *prompt.JITPromptCompiler
	configFactory *prompt.ConfigFactory

	// Perception
	transducer perception.Transducer

	// Tool registries (dual-registry Piggyback++ architecture)
	// ouroborosRegistry holds Ouroboros-generated compiled binary tools
	// Modular tools from tools.Global() are accessed directly
	ouroborosRegistry *core.ToolRegistry

	// Context management
	conversationHistory []perception.ConversationTurn
	sessionContext      *types.SessionContext

	// Configuration
	config ExecutorConfig
}

// ExecutorConfig holds configuration for the executor.
type ExecutorConfig struct {
	// MaxToolCalls limits tool calls per turn to prevent runaway execution.
	MaxToolCalls int

	// ToolTimeout is the maximum time for a single tool execution.
	ToolTimeout time.Duration

	// EnableSafetyGate enables constitutional safety checks.
	EnableSafetyGate bool
}

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxToolCalls:     50,
		ToolTimeout:      5 * time.Minute,
		EnableSafetyGate: true,
	}
}

// NewExecutor creates a new executor with the given dependencies.
func NewExecutor(
	kernel types.Kernel,
	virtualStore types.VirtualStore,
	llmClient types.LLMClient,
	jitCompiler *prompt.JITPromptCompiler,
	configFactory *prompt.ConfigFactory,
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

// SetOuroborosRegistry sets the Ouroboros tool registry for generated tools.
// This enables Piggyback++ to include Ouroboros-generated tools in the catalog.
func (e *Executor) SetOuroborosRegistry(registry *core.ToolRegistry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ouroborosRegistry = registry
	logging.Session("Ouroboros registry configured with %d tools", len(registry.ListTools()))
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
			Args: []interface{}{
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
	compileResult, err := e.jitCompiler.Compile(ctx, compilationCtx)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("JIT compilation failed, using baseline: %v", err)
		// Fall back to baseline prompt if JIT fails
		compileResult = &prompt.CompilationResult{
			Prompt: "You are an AI assistant helping with software development.",
		}
	}

	// 4. JIT: Compile config (tools, policies)
	agentConfig, err := e.compileConfig(ctx, compileResult, intent)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("Config compilation failed: %v", err)
		// Continue with empty config - LLM can still respond
		agentConfig = &config.AgentConfig{}
	} else {
		logging.Session("Config compiled: %d tools allowed", len(agentConfig.Tools.AllowedTools))
	}

	// 5. LLM: Generate response with tool calling
	llmResponse, err := e.generateResponse(ctx, compileResult.Prompt, input, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// 6. Execute tool calls (if any)
	for i, call := range llmResponse.ToolCalls {
		if i >= e.config.MaxToolCalls {
			logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", e.config.MaxToolCalls)
			break
		}

		toolCall := ToolCall{
			ID:   call.ID,
			Name: call.Name,
			Args: call.Input,
		}

		toolResult, err := e.executeToolCall(ctx, toolCall, agentConfig)
		if err != nil {
			logging.Get(logging.CategorySession).Error("Tool call %s failed: %v", call.Name, err)
			// Continue with other tool calls
		} else {
			logging.SessionDebug("Tool %s executed successfully: %d chars result", call.Name, len(toolResult))
		}
		result.ToolCallsExecuted++
	}

	// 7. Articulate response
	result.Response = llmResponse.Text
	result.Duration = time.Since(start)

	// Update conversation history
	e.appendToHistory("user", input)
	e.appendToHistory("assistant", result.Response)

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
	if sCtx, ok := ctx.Value(sessionContextKey).(*types.SessionContext); ok {
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

// compileConfig creates an AgentConfig from the compilation result and intent.
func (e *Executor) compileConfig(ctx context.Context, result *prompt.CompilationResult, intent perception.Intent) (*config.AgentConfig, error) {
	if e.configFactory == nil {
		return &config.AgentConfig{}, nil
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
func (e *Executor) generateResponse(ctx context.Context, systemPrompt, userInput string, cfg *config.AgentConfig) (*types.LLMToolResponse, error) {
	// Check if client should use Piggyback for tools (e.g., Gemini with grounding enabled)
	if ptp, ok := e.llmClient.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		return e.generateResponseWithPiggybackTools(ctx, systemPrompt, userInput, cfg)
	}

	// Convert AgentConfig tool names to ToolDefinition structs
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
func (e *Executor) generateResponseWithPiggybackTools(ctx context.Context, systemPrompt, userInput string, cfg *config.AgentConfig) (*types.LLMToolResponse, error) {
	// Build tool catalog for injection into system prompt
	toolCatalog := e.buildToolCatalogForPiggyback(cfg)
	if toolCatalog != "" {
		systemPrompt = systemPrompt + "\n\n" + toolCatalog
		logging.Session("Injected tool catalog into system prompt for Piggyback++ (%d chars)", len(toolCatalog))
	}

	// Use CompleteWithSystem (supports grounding + structured output)
	// The Piggyback envelope will contain tool_requests
	logging.Session("Using Piggyback++ for tool invocation (grounding-compatible mode)")
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
func (e *Executor) buildToolCatalogForPiggyback(cfg *config.AgentConfig) string {
	var catalog strings.Builder
	catalog.WriteString("\n## Available Tools\n\n")
	catalog.WriteString("Request tools via `tool_requests` in control_packet:\n")
	catalog.WriteString("```json\n")
	catalog.WriteString("\"tool_requests\": [{\n")
	catalog.WriteString("  \"id\": \"req_1\",\n")
	catalog.WriteString("  \"tool_name\": \"<tool_name>\",\n")
	catalog.WriteString("  \"tool_args\": { ... },\n")
	catalog.WriteString("  \"purpose\": \"why this tool is needed\"\n")
	catalog.WriteString("}]\n")
	catalog.WriteString("```\n\n")

	toolCount := 0

	// 1. Add modular tools from tools.Global()
	modularRegistry := tools.Global()
	if cfg != nil && len(cfg.Tools.AllowedTools) > 0 {
		catalog.WriteString("### Built-in Tools\n\n")
		for _, toolName := range cfg.Tools.AllowedTools {
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
		toolCount, len(cfg.Tools.AllowedTools), toolCount-len(cfg.Tools.AllowedTools))

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
			"missing_tool_for": {},
			"observation":      {},
			"task_status":      {},
			"task_completed":   {},
			"diagnostic":       {},
			"failing_test":     {},
			"test_state":       {},
			"review_finding":   {},
			"modified":         {},
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
			intent := fmt.Sprintf("%v", fact.Args[0])
			capability := fmt.Sprintf("%v", fact.Args[1])
			logging.Session("Detected missing_tool_for: intent=%s capability=%s", intent, capability)
		}
	}
}

// parseMangleArgs parses comma-separated Mangle arguments.
// Handles quoted strings and atom constants.
func (e *Executor) parseMangleArgs(argsStr string) []interface{} {
	var args []interface{}
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
func (e *Executor) parseMangleArg(arg string) interface{} {
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

// buildToolDefinitions converts tool names from AgentConfig to ToolDefinition structs.
func (e *Executor) buildToolDefinitions(cfg *config.AgentConfig) []types.ToolDefinition {
	if cfg == nil || len(cfg.Tools.AllowedTools) == 0 {
		logging.SessionDebug("buildToolDefinitions: no tools configured (cfg=%v)", cfg != nil)
		return nil
	}
	logging.Session("buildToolDefinitions: building %d tool definitions", len(cfg.Tools.AllowedTools))

	registry := tools.Global()
	var defs []types.ToolDefinition

	for _, toolName := range cfg.Tools.AllowedTools {
		tool := registry.Get(toolName)
		if tool == nil {
			logging.SessionDebug("Tool %s not found in registry", toolName)
			continue
		}

		// Build input schema from tool's schema
		inputSchema := make(map[string]interface{})
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

	logging.Session("Built %d tool definitions from %d allowed tools", len(defs), len(cfg.Tools.AllowedTools))
	return defs
}

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]interface{}
}

// executeToolCall routes a tool call through the appropriate registry with safety checks.
// It checks both registries in order:
// 1. Modular tools (tools.Global()) - Go function handlers
// 2. Ouroboros tools (core.ToolRegistry) - compiled binary tools
func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.AgentConfig) (string, error) {
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
func (e *Executor) isToolAllowed(toolName string, cfg *config.AgentConfig) bool {
	if cfg == nil || len(cfg.Tools.AllowedTools) == 0 {
		return true // No restrictions
	}

	for _, allowed := range cfg.Tools.AllowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// checkSafety verifies a tool call against the Constitutional Gate.
func (e *Executor) checkSafety(call ToolCall) bool {
	if e.kernel == nil {
		return true // No kernel, no safety check
	}

	// 1. Prepare Mangle terms
	// Action names must be Mangle atoms (start with /)
	actionName := call.Name
	if !strings.HasPrefix(actionName, "/") {
		actionName = "/" + actionName
	}
	actionAtom := types.MangleAtom(actionName)

	// Extract target and serialize payload
	target := e.extractTarget(call.Args)
	payloadBytes, err := json.Marshal(call.Args)
	if err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: cannot marshal args: %v", err)
		return false
	}
	payload := string(payloadBytes)
	timestamp := time.Now().Unix()

	// 2. Assert pending_action
	// Decl pending_action(ActionID, ActionType, Target, Payload, Timestamp)
	pendingFact := types.Fact{
		Predicate: "pending_action",
		Args: []interface{}{
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
		factAction := fmt.Sprintf("%v", f.Args[0])
		if factAction != string(actionAtom) {
			continue
		}

		// Check Target
		factTarget := fmt.Sprintf("%v", f.Args[1])
		if factTarget != target {
			continue
		}

		// Check Payload
		factPayload := fmt.Sprintf("%v", f.Args[2])
		if factPayload != payload {
			continue
		}

		// Match found!
		return true
	}

	logging.Get(logging.CategorySession).Warn("Safety check denied action: %s (target: %s)", actionName, target)
	return false
}

// extractTarget attempts to identify the primary target of a tool call.
func (e *Executor) extractTarget(args map[string]interface{}) string {
	// Common keys for targets
	candidates := []string{"path", "filename", "filepath", "file", "url", "target", "query"}
	for _, key := range candidates {
		if val, ok := args[key]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return "unknown"
}


// appendToHistory adds a turn to conversation history.
func (e *Executor) appendToHistory(role, content string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.conversationHistory = append(e.conversationHistory, perception.ConversationTurn{
		Role:    role,
		Content: content,
	})

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
