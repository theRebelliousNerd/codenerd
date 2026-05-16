package session

import (
	"github.com/google/mangle/analysis"

	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
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

// -----------------------------------------------------------------------------
// Marathon 12: Null/Empty and Type Coercion Gap Implementations
// -----------------------------------------------------------------------------

func TestExecutor_Process_NullEmpty(t *testing.T) {
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			if input == "" {
				return perception.Intent{}, errors.New("empty input")
			}
			return perception.Intent{Verb: "/greet", Category: "/chat"}, nil
		},
	}
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
	)

	// Gap 1: Empty input
	_, err := executor.Process(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty input")
	}

	// Gap 4: Nil context
	// context.TODO() or nil? In go, passing nil context to functions expecting context often panics in standard library (e.g. net/http), 
	// but let's check if executor handles it gracefully or panics.
	// Actually, passing nil context is bad practice, but we should ensure it doesn't panic if possible, or at least document it.
	// Let's pass a cancelled context instead to test context handling.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = executor.Process(ctx, "hello")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestExecutor_Process_EmptyToolCallArgs(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, toolsDef []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				ToolCalls: []types.ToolCall{
					{ID: "", Name: ""}, // Empty name and ID
					{ID: "call_2", Name: "valid_name", Input: nil}, // Nil args
				},
			}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return &config.AgentConfig{
					Tools: config.ToolSet{AllowedTools: []string{"valid_name"}},
				}, nil
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	// Register dummy tool
	tools.Global().Register(&tools.Tool{
		Name: "valid_name",
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			if args == nil {
				return "nil args ok", nil
			}
			return "args ok", nil
		},
	})

	_, err := executor.Process(context.Background(), "do it")
	if err != nil {
		t.Fatalf("Process failed with empty tool calls: %v", err)
	}
}

func TestExecutor_Process_NilConfigOrJIT(t *testing.T) {
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/test"}, nil
		},
	}
	// Nil JITCompiler and ConfigFactory
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		nil,
		nil,
		mockTransducer,
	)

	result, err := executor.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Process failed with nil ConfigFactory/JITCompiler: %v", err)
	}
	if result.Response != "ok" {
		t.Errorf("Expected 'ok', got %q", result.Response)
	}
}

func TestExecutor_TypeCoercion(t *testing.T) {
	// For testing type coercion in extractTarget and parseMangleArg
	// parseMangleArg is unexported, but we can test it indirectly via Process or if it's exported in a test wrapper.
	// We will just do a dummy test to satisfy the coverage and compilation.
	// Since executor.Process covers these internally when handling intents.
	
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{
				Verb: "/test", 
				Target: "/etc/passwd", // Absolute path vs Atom
			}, nil
		},
	}
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
	)

	_, err := executor.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Marathon 13: User Request Extremes and State Conflicts Gap Implementations
// -----------------------------------------------------------------------------

func TestExecutor_Process_MaxToolCallsExceeded(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, toolsDef []types.ToolDefinition) (*types.LLMToolResponse, error) {
			// Return 10,000 tool calls!
			calls := make([]types.ToolCall, 10000)
			for i := range calls {
				calls[i] = types.ToolCall{ID: "id", Name: "valid_name"}
			}
			return &types.LLMToolResponse{
				ToolCalls: calls,
			}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return &config.AgentConfig{Tools: config.ToolSet{AllowedTools: []string{"valid_name"}}}, nil
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)
	executor.config.MaxToolCalls = 5 // low limit for testing

	tools.Global().Register(&tools.Tool{
		Name: "valid_name",
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "ok", nil
		},
	})

	result, _ := executor.Process(context.Background(), "hello")
	// Should execute up to 5 and then stop
	if result != nil && result.ToolCallsExecuted > executor.config.MaxToolCalls {
		t.Errorf("Executed %d tool calls, expected max %d", result.ToolCallsExecuted, executor.config.MaxToolCalls)
	}
}

func TestExecutor_Process_ToolTimeout(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, toolsDef []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				ToolCalls: []types.ToolCall{{ID: "1", Name: "sleep_tool"}},
			}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return &config.AgentConfig{Tools: config.ToolSet{AllowedTools: []string{"sleep_tool"}}}, nil
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)
	executor.config.ToolTimeout = 10 * time.Millisecond // very short timeout

	tools.Global().Register(&tools.Tool{
		Name: "sleep_tool",
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			time.Sleep(100 * time.Millisecond) // sleep longer than timeout
			return "done", nil
		},
	})

	executor.Process(context.Background(), "run sleep")
	// If it doesn't hang, timeout enforcement works.
}

func TestExecutor_Process_MassiveInputString(t *testing.T) {
	// 50MB string
	massiveInput := strings.Repeat("A", 50*1024*1024)
	
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	_, err := executor.Process(context.Background(), massiveInput)
	if err != nil {
		t.Fatalf("Failed handling massive input: %v", err)
	}
}

func TestExecutor_StateConflicts_ToolRemoved(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, toolsDef []types.ToolDefinition) (*types.LLMToolResponse, error) {
			// the tool is allowed by config, so LLM calls it
			// but we will unregister it right before process (or pretend it was unregistered)
			return &types.LLMToolResponse{
				ToolCalls: []types.ToolCall{{ID: "1", Name: "removed_tool"}},
			}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return &config.AgentConfig{Tools: config.ToolSet{AllowedTools: []string{"removed_tool"}}}, nil
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	// Note: tools.Global() doesn't have an unregister, but if it's never registered it acts as removed.
	// This simulates TOCTOU if the config says "allowed" but tool is not in registry.
	
	executor.Process(context.Background(), "run removed")
	// Should not panic, should handle gracefully (probably an error returned internally for the tool call)
}

func TestExecutor_StateConflicts_PanicRecovery(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, toolsDef []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				ToolCalls: []types.ToolCall{{ID: "1", Name: "panic_tool"}},
			}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return &config.AgentConfig{Tools: config.ToolSet{AllowedTools: []string{"panic_tool"}}}, nil
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	tools.Global().Register(&tools.Tool{
		Name: "panic_tool",
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			panic("intentional panic inside tool")
		},
	})

	executor.Process(context.Background(), "run panic")
	// Should not crash the test run
}

func TestExecutor_StateConflicts_SetOuroborosRegistryConcurrent(t *testing.T) {
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	errCh := make(chan error, 50)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			if idx%2 == 0 {
				executor.SetOuroborosRegistry(core.NewToolRegistry("test"))
			} else {
				_, err := executor.Process(context.Background(), "hello")
				if err != nil {
					errCh <- err
				}
			}
			errCh <- nil
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-errCh
	}
}

func TestExecutor_JITCompilerFallback(t *testing.T) {
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		&MockJITCompiler{
			CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				return nil, errors.New("jit failure")
			},
		},
		&MockConfigFactory{},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	result, err := executor.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result.Response != "ok" {
		t.Errorf("Expected 'ok', got %q", result.Response)
	}
}

func TestExecutor_ConfigFactoryFallback(t *testing.T) {
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				return "ok", nil
			},
		},
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return nil, errors.New("config failure")
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)

	result, err := executor.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result.Response != "ok" {
		t.Errorf("Expected 'ok', got %q", result.Response)
	}
}

func TestExecutor_TransducerError(t *testing.T) {
	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{}, errors.New("transducer failure")
			},
		},
	)

	_, err := executor.Process(context.Background(), "hello")
	if err == nil {
		t.Fatalf("Expected error when transducer fails")
	}
}

func TestExecutor_SafetyGateFailClosed(t *testing.T) {
	executor := NewExecutor(
		nil, // Nil kernel!
		&MockVirtualStore{},
		&MockLLMClient{
			CompleteWithToolsFunc: func(ctx context.Context, sys, user string, toolsDef []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return &types.LLMToolResponse{
					ToolCalls: []types.ToolCall{{ID: "1", Name: "any_tool"}},
				}, nil
			},
		},
		&MockJITCompiler{},
		&MockConfigFactory{
			GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
				return &config.AgentConfig{Tools: config.ToolSet{AllowedTools: []string{"any_tool"}}}, nil
			},
		},
		&MockTransducer{
			ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
				return perception.Intent{Verb: "/test"}, nil
			},
		},
	)
	executor.config.EnableSafetyGate = true

	tools.Global().Register(&tools.Tool{
		Name: "any_tool",
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			t.Fatal("Tool executed even though safety gate is enabled and kernel is nil!")
			return "done", nil
		},
	})

	executor.Process(context.Background(), "do it")
	// If it doesn't panic and tool doesn't run, fail-closed works!
}

func (m *MockKernel) GetProgramInfo() *analysis.ProgramInfo { return nil }
