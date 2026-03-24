package session

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

func TestExecutor_Process_SimpleInput(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "Hello user"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "Hello user", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/greet", Category: "/chat"}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
	)

	// Execute
	result, err := executor.Process(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify
	if result.Response != "Hello user" {
		t.Errorf("Expected response 'Hello user', got '%s'", result.Response)
	}
	if result.Intent.Verb != "/greet" {
		t.Errorf("Expected intent '/greet', got '%s'", result.Intent.Verb)
	}
	if result.ToolCallsExecuted != 0 {
		t.Errorf("Expected 0 tool calls, got %d", result.ToolCallsExecuted)
	}
}

func TestExecutor_Process_NilJITCompilerUsesBaselinePrompt(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if sys != "You are an AI assistant helping with software development." {
				t.Fatalf("expected baseline prompt, got %q", sys)
			}
			return &types.LLMToolResponse{Text: "baseline ok"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			if sys != "You are an AI assistant helping with software development." {
				t.Fatalf("expected baseline prompt, got %q", sys)
			}
			return "baseline ok", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/greet", Category: "/chat"}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		nil,
		&MockConfigFactory{},
		mockTransducer,
	)

	result, err := executor.Process(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result.Response != "baseline ok" {
		t.Fatalf("expected baseline response, got %q", result.Response)
	}
}

func TestExecutor_Process_ToolExecution(t *testing.T) {
	// Register mock tool
	tool := &tools.Tool{
		Name:        "readFile",
		Description: "Reads a file",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {Type: "string"},
			},
		},
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "file content", nil
		},
	}
	tools.Global().Register(tool)

	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				Text: "I'll read that file",
				ToolCalls: []types.ToolCall{
					{
						ID:   "call_1",
						Name: "readFile",
						Input: map[string]interface{}{
							"path": "/test/file.txt",
						},
					},
				},
			}, nil
		},
	}

	mockVS := &MockVirtualStore{
		ReadFileFunc: func(path string) ([]string, error) {
			if path == "/test/file.txt" {
				return []string{"file content"}, nil
			}
			return nil, errors.New("file not found")
		},
	}

	// Kernel with permission for readFile
	mockKernel := &MockKernel{}
	mockKernel.Assert(types.Fact{
		Predicate: "permitted",
		Args:      []interface{}{MangleAtom("/readFile"), "/test/file.txt", `{"path":"/test/file.txt"}`},
	})
	// Need to assert user_intent for safety check logic usually, but here we just asserted permitted directly.
	// Wait, checkSafety logic queries kernel.

	// Mock ConfigFactory to return allowed tools
	mockConfig := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
			return &config.AgentConfig{
				Tools: config.ToolSet{
					AllowedTools: []string{"readFile"},
				},
			}, nil
		},
	}

	executor := NewExecutor(
		mockKernel,
		mockVS,
		mockLLM,
		&MockJITCompiler{},
		mockConfig,
		&MockTransducer{},
	)
	executor.config.EnableSafetyGate = true // Ensure gate is on

	// Execute
	result, err := executor.Process(context.Background(), "Read /test/file.txt")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify
	if result.ToolCallsExecuted != 1 {
		t.Errorf("Expected 1 tool call, got %d", result.ToolCallsExecuted)
	}

	// Check history to ensure tool result was added (indirectly)
	// We can't easily check internal history without GetHistory()
	// executor.GetHistory() -> []ConversationTurn
}

func TestExecutor_Process_SafetyGate(t *testing.T) {
	// Register mock tool
	toolExecuted := false
	tool := &tools.Tool{
		Name:        "deleteFile",
		Description: "Deletes a file",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {Type: "string"},
			},
		},
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			toolExecuted = true
			return "deleted", nil
		},
	}
	tools.Global().Register(tool)

	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				ToolCalls: []types.ToolCall{
					{
						ID:   "call_unsafe",
						Name: "deleteFile", // Not permitted
						Input: map[string]interface{}{
							"path": "/important.txt",
						},
					},
				},
			}, nil
		},
	}

	mockKernel := &MockKernel{} // Empty kernel = no permissions

	mockConfig := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
			return &config.AgentConfig{
				Tools: config.ToolSet{
					AllowedTools: []string{"deleteFile"},
				},
			}, nil
		},
	}

	executor := NewExecutor(
		mockKernel,
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		mockConfig,
		&MockTransducer{},
	)
	executor.config.EnableSafetyGate = true

	// Execute
	result, err := executor.Process(context.Background(), "Delete everything")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify
	// ToolCallsExecuted tracks attempts, so it will be 1
	if result.ToolCallsExecuted != 1 {
		t.Errorf("Expected 1 tool call attempt, got %d", result.ToolCallsExecuted)
	}

	// CRITICAL: Verify tool was NOT executed
	if toolExecuted {
		t.Error("Safety gate failed: Tool was executed!")
	}

	// Check that pending_action was asserted
	foundPending := false
	for _, f := range mockKernel.asserts {
		if f.Predicate == "pending_action" {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Error("Expected pending_action assertion")
	}
}

func TestExecutor_Process_SessionContext(t *testing.T) {
	// Setup
	var capturedContext *types.SessionContext

	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			if sCtx, ok := cc.SessionContext.(*types.SessionContext); ok {
				capturedContext = sCtx
			}
			return &prompt.CompilationResult{Prompt: "prompt"}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		mockJIT,
		&MockConfigFactory{},
		&MockTransducer{},
	)

	sessionCtx := &types.SessionContext{
		DreamMode: true,
	}

	// Execute with context
	ctx := types.WithSessionContext(context.Background(), sessionCtx)
	_, err := executor.Process(ctx, "Dream a little dream")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify
	if capturedContext == nil {
		t.Fatal("SessionContext was not captured")
	}
	if !capturedContext.DreamMode {
		t.Error("Expected DreamMode to be true in compilation context")
	}
}

// TODO: TEST_GAP: Null/Empty: Verify Process(ctx, "") handles empty input gracefully (no panic, sensible error/response).
// TODO: TEST_GAP: Null/Empty: Verify behavior when LLM returns empty ToolCall Name or ID.
// TODO: TEST_GAP: Null/Empty: Verify handling of ToolCall with nil Args (prevent nil pointer deref during json.Marshal).
// TODO: TEST_GAP: Null/Empty: Verify Process behaves correctly when ctx is nil (should it panic or handle gracefully?).
// TODO: TEST_GAP: Null/Empty: Verify behavior when ConfigFactory or JITCompiler are explicitly nil but called.

// TODO: TEST_GAP: Type Coercion: Verify behavior when LLM provides a tool argument string where a number/object is expected by the modular tool schema.
// TODO: TEST_GAP: Type Coercion: Verify parseMangleArg behavior with massive integers that overflow Go's int type but might fit in uint64.
// TODO: TEST_GAP: Type Coercion: Verify extractTarget handles unexpected argument types (e.g., target key exists but value is a boolean/int instead of string).

// TODO: TEST_GAP: User Request Extremes: Verify MaxToolCalls limit with a mock LLM returning > MaxToolCalls (e.g., 10,000 tool calls).
// TODO: TEST_GAP: User Request Extremes: Verify system behavior when input string is extremely large (e.g., 50MB string, testing memory and execution bounds).
// TODO: TEST_GAP: User Request Extremes: Verify tool timeout enforcement (mock tool sleeping > ToolTimeout).
// TODO: TEST_GAP: User Request Extremes: Verify processMangleUpdatesFromEnvelope handles an absurdly large number of mangle_updates gracefully (testing MaxUpdates limit enforcement).

// TODO: TEST_GAP: State Conflicts: Verify TOCTOU handling where a tool is allowed by config but removed from tools.Global() before execution.
// TODO: TEST_GAP: State Conflicts: Verify thread safety of SetOuroborosRegistry when called concurrently with Process/executeToolCall.
// TODO: TEST_GAP: State Conflicts: Verify conversationHistory append thread safety under high concurrent load (simultaneous Process calls).
// TODO: TEST_GAP: State Conflicts: Verify Process respects ctx.Done() and halts execution immediately (preventing resource leaks during context cancellation).
// TODO: TEST_GAP: State Conflicts: Verify panic recovery within executeToolCall (mock tool panicking, ensuring it does not crash the Executor loop).
// TODO: TEST_GAP: State Conflicts: Verify "Fail Closed" behavior if Kernel is nil but SafetyGate is enabled.

// TODO: TEST_GAP: Verify behavior when JITCompiler returns error (Fallback to baseline prompt?).
// TODO: TEST_GAP: Verify behavior when ConfigFactory returns error (Fallback to empty config?).
// TODO: TEST_GAP: Verify behavior when Transducer returns error.
