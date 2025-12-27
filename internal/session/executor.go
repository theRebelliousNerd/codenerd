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
	}

	// 5. LLM: Generate response
	response, err := e.generateResponse(ctx, compileResult.Prompt, input, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// 6. Execute tool calls (if any)
	toolCalls := e.extractToolCalls(response)
	for i, call := range toolCalls {
		if i >= e.config.MaxToolCalls {
			logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", e.config.MaxToolCalls)
			break
		}

		if err := e.executeToolCall(ctx, call, agentConfig); err != nil {
			logging.Get(logging.CategorySession).Error("Tool call failed: %v", err)
			// Continue with other tool calls
		}
		result.ToolCallsExecuted++
	}

	// 7. Articulate response
	result.Response = e.extractTextResponse(response)
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

// generateResponse calls the LLM with the compiled prompt and input.
func (e *Executor) generateResponse(ctx context.Context, systemPrompt, userInput string, cfg *config.AgentConfig) (string, error) {
	// TODO: Pass tools to LLM for tool calling
	// For now, use simple completion
	return e.llmClient.CompleteWithSystem(ctx, systemPrompt, userInput)
}

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	Name   string
	Args   map[string]interface{}
	RawArg string
}

// extractToolCalls parses tool calls from LLM response.
// TODO: Implement proper tool call parsing based on LLM response format.
func (e *Executor) extractToolCalls(response string) []ToolCall {
	// Placeholder - will be implemented when we add tool calling to LLM
	return nil
}

// executeToolCall routes a tool call through VirtualStore with safety checks.
func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.AgentConfig) error {
	// Check if tool is allowed by config
	if !e.isToolAllowed(call.Name, cfg) {
		return fmt.Errorf("tool %s not allowed by config", call.Name)
	}

	// Safety check via Constitutional Gate
	if e.config.EnableSafetyGate {
		if !e.checkSafety(call) {
			return fmt.Errorf("tool call blocked by safety gate: %s", call.Name)
		}
	}

	// Execute via VirtualStore
	// TODO: Implement tool execution through VirtualStore
	logging.SessionDebug("Would execute tool: %s", call.Name)

	return nil
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

// extractTextResponse extracts the text portion of the LLM response.
func (e *Executor) extractTextResponse(response string) string {
	// For now, return the full response
	// TODO: Parse out tool calls and return just text
	return response
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
