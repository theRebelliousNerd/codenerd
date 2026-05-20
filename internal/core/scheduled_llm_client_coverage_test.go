package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

type mockFullClient struct {
	model             string
	shardID           string
	shardType         string
	shardCategory     string
	sessionID         string
	taskContext       string
	semaphoreDisabled bool

	completeFunc              func(ctx context.Context, prompt string) (string, error)
	completeWithSystemFunc    func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	completeWithSchemaFunc    func(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
	completeWithToolsFunc     func(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
	completeWithStreamingFunc func(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error)
}

func (m *mockFullClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "mocked complete", nil
}

func (m *mockFullClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.completeWithSystemFunc != nil {
		return m.completeWithSystemFunc(ctx, systemPrompt, userPrompt)
	}
	return "mocked complete with system", nil
}

func (m *mockFullClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	if m.completeWithSchemaFunc != nil {
		return m.completeWithSchemaFunc(ctx, systemPrompt, userPrompt, jsonSchema)
	}
	return "mocked complete with schema", nil
}

func (m *mockFullClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if m.completeWithToolsFunc != nil {
		return m.completeWithToolsFunc(ctx, systemPrompt, userPrompt, tools)
	}
	return &types.LLMToolResponse{Text: "mocked tool response"}, nil
}

func (m *mockFullClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	if m.completeWithStreamingFunc != nil {
		return m.completeWithStreamingFunc(ctx, systemPrompt, userPrompt, enableThinking)
	}
	c := make(chan string, 1)
	c <- "mocked stream chunk"
	close(c)
	e := make(chan error, 1)
	close(e)
	return c, e
}

func (m *mockFullClient) SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string) {
	m.shardID = shardID
	m.shardType = shardType
	m.shardCategory = shardCategory
	m.sessionID = sessionID
	m.taskContext = taskContext
}

func (m *mockFullClient) ClearShardContext() {
	m.shardID = ""
	m.shardType = ""
	m.shardCategory = ""
	m.sessionID = ""
	m.taskContext = ""
}

func (m *mockFullClient) DisableSemaphore() {
	m.semaphoreDisabled = true
}

func (m *mockFullClient) SetModel(model string) {
	m.model = model
}

func (m *mockFullClient) GetModel() string {
	return m.model
}

func (m *mockFullClient) SchemaCapable() bool {
	return true
}

func (m *mockFullClient) IsThinkingEnabled() bool {
	return true
}

func (m *mockFullClient) GetThinkingLevel() string {
	return "high"
}

func (m *mockFullClient) GetLastThoughtSummary() string {
	return "thought summary"
}

func (m *mockFullClient) GetLastThinkingTokens() int {
	return 42
}

func (m *mockFullClient) GetLastThoughtSignature() string {
	return "thought signature"
}

func (m *mockFullClient) GetLastGroundingSources() []string {
	return []string{"source1"}
}

func (m *mockFullClient) IsGoogleSearchEnabled() bool {
	return true
}

func (m *mockFullClient) IsURLContextEnabled() bool {
	return true
}

func (m *mockFullClient) CreateCachedContent(ctx context.Context, files []string, ttl int) (string, error) {
	return "cache1", nil
}

func (m *mockFullClient) GetCachedContent(ctx context.Context, cacheName string) (interface{}, error) {
	return "cached_data", nil
}

func (m *mockFullClient) DeleteCachedContent(ctx context.Context, cacheName string) error {
	return nil
}

func (m *mockFullClient) ListCachedContent(ctx context.Context) ([]string, error) {
	return []string{"cache1"}, nil
}

func (m *mockFullClient) SetCachedContent(name string) {
}

func (m *mockFullClient) UploadFile(ctx context.Context, path string, mimeType string) (string, error) {
	return "file1", nil
}

func (m *mockFullClient) DeleteFile(ctx context.Context, fileID string) error {
	return nil
}

func (m *mockFullClient) ListFiles(ctx context.Context) ([]string, error) {
	return []string{"file1"}, nil
}

func (m *mockFullClient) GetFile(ctx context.Context, fileID string) (interface{}, error) {
	return "file_data", nil
}

// Minimal client for testing fallbacks
type mockMinimalClient struct {
	model string
}

func (m *mockMinimalClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "minimal complete", nil
}

func (m *mockMinimalClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "minimal complete with system", nil
}

func (m *mockMinimalClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return nil, nil
}

func (m *mockMinimalClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "minimal tools"}, nil
}

func TestScheduledLLMCall_AllMethods(t *testing.T) {
	mc := &mockFullClient{model: "gemini-3.5-flash"}
	sc := NewScheduledLLMCall("shard-test", mc)

	ctx := context.Background()

	// 1. Complete
	res, err := sc.Complete(ctx, "hello")
	if err != nil || res != "mocked complete" {
		t.Errorf("Complete failed: %v, %v", res, err)
	}

	// 2. CompleteWithSystem
	res, err = sc.CompleteWithSystem(ctx, "sys", "user")
	if err != nil || res != "mocked complete with system" {
		t.Errorf("CompleteWithSystem failed: %v, %v", res, err)
	}

	// 3. CompleteWithSchema
	res, err = sc.CompleteWithSchema(ctx, "sys", "user", "{}")
	if err != nil || res != "mocked complete with schema" {
		t.Errorf("CompleteWithSchema failed: %v, %v", res, err)
	}

	// 4. CompleteWithTools
	tRes, err := sc.CompleteWithTools(ctx, "sys", "user", nil)
	if err != nil || tRes.Text != "mocked tool response" {
		t.Errorf("CompleteWithTools failed: %+v, %v", tRes, err)
	}

	// 5. Tracing context pass-through
	sc.SetShardContext("s1", "t1", "c1", "sess1", "tc1")
	if mc.shardID != "s1" || mc.taskContext != "tc1" {
		t.Errorf("SetShardContext failed to propagate: %+v", mc)
	}
	sc.ClearShardContext()
	if mc.shardID != "" {
		t.Errorf("ClearShardContext failed to propagate: %+v", mc)
	}

	// 6. Thinking and metadata pass-through
	if !sc.SchemaCapable() {
		t.Error("SchemaCapable should be true")
	}
	if !sc.IsThinkingEnabled() {
		t.Error("IsThinkingEnabled should be true")
	}
	if sc.GetThinkingLevel() != "high" {
		t.Errorf("GetThinkingLevel got: %s", sc.GetThinkingLevel())
	}
	if sc.GetLastThoughtSummary() != "thought summary" {
		t.Errorf("GetLastThoughtSummary got: %s", sc.GetLastThoughtSummary())
	}
	if sc.GetLastThinkingTokens() != 42 {
		t.Errorf("GetLastThinkingTokens got: %d", sc.GetLastThinkingTokens())
	}
	if sc.GetLastThoughtSignature() != "thought signature" {
		t.Errorf("GetLastThoughtSignature got: %s", sc.GetLastThoughtSignature())
	}
	sources := sc.GetLastGroundingSources()
	if len(sources) != 1 || sources[0] != "source1" {
		t.Errorf("GetLastGroundingSources got: %v", sources)
	}
	if !sc.IsGoogleSearchEnabled() {
		t.Error("IsGoogleSearchEnabled should be true")
	}
	if !sc.IsURLContextEnabled() {
		t.Error("IsURLContextEnabled should be true")
	}

	// 7. Cache pass-through
	cName, err := sc.CreateCachedContent(ctx, nil, 100)
	if err != nil || cName != "cache1" {
		t.Errorf("CreateCachedContent failed: %v, %v", cName, err)
	}
	cData, err := sc.GetCachedContent(ctx, "cache1")
	if err != nil || cData != "cached_data" {
		t.Errorf("GetCachedContent failed: %v, %v", cData, err)
	}
	err = sc.DeleteCachedContent(ctx, "cache1")
	if err != nil {
		t.Errorf("DeleteCachedContent failed: %v", err)
	}
	cList, err := sc.ListCachedContent(ctx)
	if err != nil || len(cList) != 1 || cList[0] != "cache1" {
		t.Errorf("ListCachedContent failed: %v, %v", cList, err)
	}
	sc.SetCachedContent("cache-set")

	// 8. File pass-through
	fileID, err := sc.UploadFile(ctx, "path", "mime")
	if err != nil || fileID != "file1" {
		t.Errorf("UploadFile failed: %v, %v", fileID, err)
	}
	err = sc.DeleteFile(ctx, "file1")
	if err != nil {
		t.Errorf("DeleteFile failed: %v", err)
	}
	fList, err := sc.ListFiles(ctx)
	if err != nil || len(fList) != 1 || fList[0] != "file1" {
		t.Errorf("ListFiles failed: %v, %v", fList, err)
	}
	fData, err := sc.GetFile(ctx, "file1")
	if err != nil || fData != "file_data" {
		t.Errorf("GetFile failed: %v, %v", fData, err)
	}

	// 9. Model and Semaphore
	sc.SetModel("new-model")
	if sc.GetModel() != "new-model" {
		t.Errorf("Model set/get failed: %s", sc.GetModel())
	}
	if !mc.semaphoreDisabled {
		t.Error("Semaphore should be disabled on construction")
	}
}

func TestScheduledLLMCall_Fallbacks(t *testing.T) {
	minClient := &mockMinimalClient{model: "minimal"}
	sc := NewScheduledLLMCall("shard-minimal", minClient)

	ctx := context.Background()

	// 1. CompleteWithSchema (not supported fallback)
	_, err := sc.CompleteWithSchema(ctx, "sys", "user", "{}")
	if !errors.Is(err, ErrSchemaNotSupported) {
		t.Errorf("expected ErrSchemaNotSupported, got: %v", err)
	}

	// 2. SchemaCapable check
	if sc.SchemaCapable() {
		t.Error("SchemaCapable should be false for minimal client")
	}

	// 3. CompleteWithStreaming (not supported fallback via nil client)
	sc.Scheduler.RegisterShard("shard-nil", "tester")
	scNil := &ScheduledLLMCall{
		Scheduler: sc.Scheduler,
		ShardID:   "shard-nil",
		Client:    nil,
	}
	_, errChan := scNil.CompleteWithStreaming(ctx, "sys", "user", false)
	err = <-errChan
	if !errors.Is(err, ErrStreamingNotSupported) {
		t.Errorf("expected ErrStreamingNotSupported, got: %v", err)
	}

	// 4. Cache functions fallbacks
	_, err = sc.CreateCachedContent(ctx, nil, 100)
	if err == nil || !strings.Contains(err.Error(), "does not implement CacheProvider") {
		t.Errorf("expected CacheProvider support error, got: %v", err)
	}
	_, err = sc.GetCachedContent(ctx, "cache")
	if err == nil || !strings.Contains(err.Error(), "does not implement CacheProvider") {
		t.Errorf("expected CacheProvider support error, got: %v", err)
	}
	err = sc.DeleteCachedContent(ctx, "cache")
	if err == nil || !strings.Contains(err.Error(), "does not implement CacheProvider") {
		t.Errorf("expected CacheProvider support error, got: %v", err)
	}
	_, err = sc.ListCachedContent(ctx)
	if err == nil || !strings.Contains(err.Error(), "does not implement CacheProvider") {
		t.Errorf("expected CacheProvider support error, got: %v", err)
	}

	// 5. File functions fallbacks
	_, err = sc.UploadFile(ctx, "path", "mime")
	if err == nil || !strings.Contains(err.Error(), "does not implement FileProvider") {
		t.Errorf("expected FileProvider support error, got: %v", err)
	}
	err = sc.DeleteFile(ctx, "file1")
	if err == nil || !strings.Contains(err.Error(), "does not implement FileProvider") {
		t.Errorf("expected FileProvider support error, got: %v", err)
	}
	_, err = sc.ListFiles(ctx)
	if err == nil || !strings.Contains(err.Error(), "does not implement FileProvider") {
		t.Errorf("expected FileProvider support error, got: %v", err)
	}
	_, err = sc.GetFile(ctx, "file1")
	if err == nil || !strings.Contains(err.Error(), "does not implement FileProvider") {
		t.Errorf("expected FileProvider support error, got: %v", err)
	}

	// 6. Thinking and grounding fallbacks
	if sc.IsThinkingEnabled() {
		t.Error("IsThinkingEnabled should be false")
	}
	if sc.GetThinkingLevel() != "" {
		t.Error("GetThinkingLevel should be empty")
	}
	if sc.GetLastThoughtSummary() != "" {
		t.Error("GetLastThoughtSummary should be empty")
	}
	if sc.GetLastThinkingTokens() != 0 {
		t.Error("GetLastThinkingTokens should be 0")
	}
	if sc.GetLastThoughtSignature() != "" {
		t.Error("GetLastThoughtSignature should be empty")
	}
	if sc.GetLastGroundingSources() != nil {
		t.Error("GetLastGroundingSources should be nil")
	}
	if sc.IsGoogleSearchEnabled() {
		t.Error("IsGoogleSearchEnabled should be false")
	}
	if sc.IsURLContextEnabled() {
		t.Error("IsURLContextEnabled should be false")
	}
}

func TestScheduledLLMCall_RetryAndStreaming(t *testing.T) {
	mc := &mockFullClient{model: "gemini-3.5-flash"}
	sc := NewScheduledLLMCall("shard-retry", mc)

	ctx := context.Background()

	// Test streaming normal path
	contentChan, errChan := sc.CompleteWithStreaming(ctx, "sys", "user", false)
	chunk := <-contentChan
	if chunk != "mocked stream chunk" {
		t.Errorf("expected 'mocked stream chunk', got: %s", chunk)
	}
	err := <-errChan
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// Test retry success
	mc.completeWithSystemFunc = func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		return "retry success", nil
	}
	res, err := sc.CompleteWithRetry(ctx, "sys", "user", 3)
	if err != nil || res != "retry success" {
		t.Errorf("CompleteWithRetry failed: %v, %v", res, err)
	}

	// Test retry eventual success (failure then success)
	attempts := 0
	mc.completeWithSystemFunc = func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		attempts++
		if attempts < 2 {
			return "", errors.New("temporary error")
		}
		return "eventual success", nil
	}
	res, err = sc.CompleteWithRetry(ctx, "sys", "user", 3)
	if err != nil || res != "eventual success" {
		t.Errorf("CompleteWithRetry eventual success failed: %v, %v", res, err)
	}

	// Test retry all failed
	mc.completeWithSystemFunc = func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		return "", errors.New("persistent error")
	}
	_, err = sc.CompleteWithRetry(ctx, "sys", "user", 2)
	if err == nil || !strings.Contains(err.Error(), "all 3 attempts failed") {
		t.Errorf("expected persistent retry failure, got: %v", err)
	}
}

func TestScheduledLLMCall_SlotAcquireCancellation(t *testing.T) {
	// Create a scheduler with 0 slots so any acquisition blocks
	cfg := APISchedulerConfig{
		MaxConcurrentAPICalls: 0,
		SlotAcquireTimeout:    10 * time.Millisecond,
		EnableMetrics:         true,
	}
	scheduler := NewAPIScheduler(cfg)

	mc := &mockFullClient{model: "gemini-3.5-flash"}
	sc := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "shard-cancel",
		Client:    mc,
	}
	scheduler.RegisterShard(sc.ShardID, "tester")

	// Trigger a complete call with a timeout context that will expire
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := sc.Complete(ctx, "hello")
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded")) {
		t.Errorf("expected deadline exceeded error, got: %v", err)
	}
}
