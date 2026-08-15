package system

import (
	"context"
	"path/filepath"
	"testing"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// MockTaskExecutor for testing Cortex routing
type MockTaskExecutor struct {
	LastIntent string
}

func (m *MockTaskExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	m.LastIntent = req.IntentVerb
	return "executed", nil
}

func (m *MockTaskExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	m.LastIntent = req.IntentVerb
	return "executed_with_context", nil
}

func (m *MockTaskExecutor) ExecuteAsync(ctx context.Context, req session.TaskRequest) (string, error) {
	return "task_id", nil
}

func (m *MockTaskExecutor) GetResult(taskID string) (string, bool, error) {
	return "", false, nil
}

func (m *MockTaskExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	return "result", nil
}

func TestNormalizeShardTypeName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"coder", "coder"},
		{"/coder", "coder"},
		{" system/scheduler ", "system/scheduler"},
	}

	for _, tc := range cases {
		if got := normalizeShardTypeName(tc.input); got != tc.expected {
			t.Errorf("normalizeShardTypeName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCortex_SpawnTask_Routing(t *testing.T) {
	mockExec := &MockTaskExecutor{}
	cortex := &Cortex{
		TaskExecutor: mockExec,
	}

	// Test routing to TaskExecutor
	res, err := cortex.SpawnTask(context.Background(), "/fix", "do something")
	if err != nil {
		t.Fatalf("SpawnTask failed: %v", err)
	}
	if res != "executed" {
		t.Errorf("Expected 'executed', got %s", res)
	}

	// Verify exact intent passing (no legacy translation)
	if mockExec.LastIntent != "/fix" {
		t.Errorf("TaskExecutor.Execute called with intent %q, want \"/fix\"", mockExec.LastIntent)
	}
}

func TestCortex_SpawnTaskWithContext_Routing(t *testing.T) {
	mockExec := &MockTaskExecutor{}
	cortex := &Cortex{
		TaskExecutor: mockExec,
	}

	// Test routing
	res, err := cortex.SpawnTaskWithContext(context.Background(), "/test", "test it", nil, types.PriorityHigh)
	if err != nil {
		t.Fatalf("SpawnTaskWithContext failed: %v", err)
	}
	if res != "executed_with_context" {
		t.Errorf("Expected 'executed_with_context', got %s", res)
	}
}

func TestCortex_SpawnTask_ImageRoutesToShardManagerNotTaskExecutor(t *testing.T) {
	// Nano Banana 2 isolation: image_generator must never hit TaskExecutor
	// (worker/Ollama path). Without ShardManager it fails closed.
	mockExec := &MockTaskExecutor{}
	cortex := &Cortex{TaskExecutor: mockExec}

	_, err := cortex.SpawnTask(context.Background(), "image_generator", "draw a square")
	if err == nil {
		t.Fatal("expected error when ShardManager missing for image spawn")
	}
	if mockExec.LastIntent != "" {
		t.Fatalf("TaskExecutor must not be called for image_generator, got intent %q", mockExec.LastIntent)
	}

	// With ShardManager present, spawn uses manager (image client path), not TaskExecutor.
	sm := coreshards.NewShardManager()
	image := &imageRouteStubLLM{name: "gemini-image"}
	worker := &imageRouteStubLLM{name: "worker"}
	sm.SetLLMClient(worker)
	sm.SetImageLLMClient(image)
	// Capture which client the spawned agent received.
	var gotClient types.LLMClient
	sm.RegisterShard("image_generator", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &imageCaptureAgent{
			Base:  NewImageCaptureBase(id, cfg),
			onSet: func(c types.LLMClient) { gotClient = c },
		}
	})
	// RegisterShard alone is enough for factory lookup; profile optional.
	cortex.ShardManager = sm
	res, err := cortex.SpawnTask(context.Background(), "image_generator", "minimal")
	if err != nil {
		t.Fatalf("SpawnTask image: %v", err)
	}
	if mockExec.LastIntent != "" {
		t.Fatalf("TaskExecutor must stay unused, intent=%q", mockExec.LastIntent)
	}
	if gotClient != types.LLMClient(image) {
		t.Fatalf("spawned image shard client = %v, want image client", gotClient)
	}
	if res == "" {
		t.Fatal("empty result from image spawn")
	}
}

// imageRouteStubLLM is a marker LLM for Cortex image routing tests.
type imageRouteStubLLM struct{ name string }

func (s *imageRouteStubLLM) Complete(context.Context, string) (string, error) { return s.name, nil }
func (s *imageRouteStubLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	return s.name, nil
}
func (s *imageRouteStubLLM) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- s.name
	close(ch)
	close(errCh)
	return ch, errCh
}
func (s *imageRouteStubLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: s.name}, nil
}

// imageCaptureAgent records SetLLMClient for routing assertions.
type imageCaptureAgent struct {
	Base  *coreshards.BaseShardAgent
	onSet func(types.LLMClient)
}

func NewImageCaptureBase(id string, cfg types.ShardConfig) *coreshards.BaseShardAgent {
	return coreshards.NewBaseShardAgent(id, cfg)
}

func (a *imageCaptureAgent) GetID() string                             { return a.Base.GetID() }
func (a *imageCaptureAgent) GetState() types.ShardState                { return a.Base.GetState() }
func (a *imageCaptureAgent) GetConfig() types.ShardConfig              { return a.Base.GetConfig() }
func (a *imageCaptureAgent) Stop() error                               { return a.Base.Stop() }
func (a *imageCaptureAgent) SetParentKernel(k types.Kernel)            { a.Base.SetParentKernel(k) }
func (a *imageCaptureAgent) SetSessionContext(c *types.SessionContext) { a.Base.SetSessionContext(c) }
func (a *imageCaptureAgent) SetLLMClient(c types.LLMClient) {
	if a.onSet != nil {
		a.onSet(c)
	}
	a.Base.SetLLMClient(c)
}
func (a *imageCaptureAgent) Execute(ctx context.Context, task string) (string, error) {
	return "image-ok", nil
}

func TestSessionVirtualStoreAdapter(t *testing.T) {
	// Adapter uses os package directly for ReadFile/WriteFile fallback
	// This tests the fallback logic in sessionVirtualStoreAdapter

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	adapter := &sessionVirtualStoreAdapter{vs: nil} // VS can be nil for ReadFile/WriteFile fallback

	// Test WriteFile
	content := []string{"line1", "line2"}
	if err := adapter.WriteFile(path, content); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test ReadFile
	readContent, err := adapter.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(readContent) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(readContent))
	}
	if readContent[0] != "line1" {
		t.Errorf("Expected line1, got %s", readContent[0])
	}
}
