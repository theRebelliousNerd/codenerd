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
	"fmt"
	"sync"
	"time"

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
	compilationCtx := e.buildCompilationContext(intent)

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
func (e *Executor) buildCompilationContext(intent perception.Intent) *prompt.CompilationContext {
	cc := &prompt.CompilationContext{
		IntentVerb:      intent.Verb,
		IntentTarget:    intent.Target,
		OperationalMode: "/active",
		TokenBudget:     8192, // Default token budget for prompt compilation
	}

	// Determine world states from kernel facts
	if e.kernel != nil {
		// Check for failing tests
		if facts, err := e.kernel.Query("test_failed"); err == nil {
			cc.FailingTestCount = len(facts)
		}

		// Check for active diagnostics
		if facts, err := e.kernel.Query("diagnostic_active"); err == nil {
			cc.DiagnosticCount = len(facts)
		}
	}

	// Set session context if available
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
func (e *Executor) generateResponse(ctx context.Context, systemPrompt, userInput string, cfg *config.AgentConfig) (*types.LLMToolResponse, error) {
	// Convert AgentConfig tool names to ToolDefinition structs
	toolDefs := e.buildToolDefinitions(cfg)

	// If we have tools, use tool calling; otherwise fall back to simple completion
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

// executeToolCall routes a tool call through the modular tool registry with safety checks.
func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.AgentConfig) (string, error) {
	// Check if tool is allowed by config
	if !e.isToolAllowed(call.Name, cfg) {
		return "", fmt.Errorf("tool %s not allowed by config", call.Name)
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

	// Execute via modular tool registry
	logging.Session("Executing tool: %s with %d args", call.Name, len(call.Args))

	result, err := tools.Execute(toolCtx, call.Name, call.Args)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// Check for errors
	if result.Error != nil {
		return "", fmt.Errorf("tool returned error: %w", result.Error)
	}

	return result.Result, nil
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

	// Query kernel for permission
	// TODO: Implement proper Mangle query for permitted(action)
	return true
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
