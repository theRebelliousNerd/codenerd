package perception

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

type baseMockLLMClient struct{}

func (m *baseMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (m *baseMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return `{"action_type":"/mutation","domain":"test","confidence":1.0,"understanding":{"action_type":"/mutation"}}`, nil
}

func (m *baseMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}

func (m *baseMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return nil, nil
}

type mockThinkingProvider struct {
	baseMockLLMClient
	thinkingEnabled bool
	thinkingLevel   string
	thoughtSummary  string
	thinkingTokens  int
	thoughtSig      string
	groundingSrc    []string
	googleSearch    bool
	urlContext      bool
}

func (m *mockThinkingProvider) IsThinkingEnabled() bool           { return m.thinkingEnabled }
func (m *mockThinkingProvider) GetThinkingLevel() string          { return m.thinkingLevel }
func (m *mockThinkingProvider) GetLastThoughtSummary() string     { return m.thoughtSummary }
func (m *mockThinkingProvider) GetLastThinkingTokens() int        { return m.thinkingTokens }
func (m *mockThinkingProvider) GetLastThoughtSignature() string   { return m.thoughtSig }
func (m *mockThinkingProvider) GetLastGroundingSources() []string { return m.groundingSrc }
func (m *mockThinkingProvider) IsGoogleSearchEnabled() bool       { return m.googleSearch }
func (m *mockThinkingProvider) IsURLContextEnabled() bool         { return m.urlContext }

func TestTracingLLMClient_PassThroughInterfaces(t *testing.T) {
	mockClient := &mockThinkingProvider{
		thinkingEnabled: true,
		thinkingLevel:   "high",
		thoughtSummary:  "a summary",
		thinkingTokens:  42,
		thoughtSig:      "sig",
		groundingSrc:    []string{"src1", "src2"},
		googleSearch:    true,
		urlContext:      true,
	}

	tc := NewTracingLLMClient(mockClient, nil)

	if !tc.IsThinkingEnabled() {
		t.Errorf("Expected IsThinkingEnabled to be true")
	}
	if tc.GetThinkingLevel() != "high" {
		t.Errorf("Expected GetThinkingLevel to be high, got %s", tc.GetThinkingLevel())
	}
	if tc.GetLastThoughtSummary() != "a summary" {
		t.Errorf("Expected GetLastThoughtSummary to be 'a summary', got %s", tc.GetLastThoughtSummary())
	}
	if tc.GetLastThinkingTokens() != 42 {
		t.Errorf("Expected GetLastThinkingTokens to be 42, got %d", tc.GetLastThinkingTokens())
	}
	if tc.GetLastThoughtSignature() != "sig" {
		t.Errorf("Expected GetLastThoughtSignature to be 'sig', got %s", tc.GetLastThoughtSignature())
	}
	sources := tc.GetLastGroundingSources()
	if len(sources) != 2 || sources[0] != "src1" || sources[1] != "src2" {
		t.Errorf("Expected GetLastGroundingSources to be [src1, src2], got %v", sources)
	}
	if !tc.IsGoogleSearchEnabled() {
		t.Errorf("Expected IsGoogleSearchEnabled to be true")
	}
	if !tc.IsURLContextEnabled() {
		t.Errorf("Expected IsURLContextEnabled to be true")
	}
}

type mockCacheAndFileProvider struct {
	baseMockLLMClient
	cacheCalled bool
	fileCalled  bool
}

func (m *mockCacheAndFileProvider) CreateCachedContent(ctx context.Context, files []string, ttl int) (string, error) {
	m.cacheCalled = true
	return "cache1", nil
}
func (m *mockCacheAndFileProvider) GetCachedContent(ctx context.Context, cacheName string) (interface{}, error) {
	m.cacheCalled = true
	return "cache_data", nil
}
func (m *mockCacheAndFileProvider) DeleteCachedContent(ctx context.Context, cacheName string) error {
	m.cacheCalled = true
	return nil
}
func (m *mockCacheAndFileProvider) ListCachedContent(ctx context.Context) ([]string, error) {
	m.cacheCalled = true
	return []string{"cache1"}, nil
}
func (m *mockCacheAndFileProvider) SetCachedContent(name string) {
	m.cacheCalled = true
}

func (m *mockCacheAndFileProvider) UploadFile(ctx context.Context, path string, mimeType string) (string, error) {
	m.fileCalled = true
	return "file1", nil
}
func (m *mockCacheAndFileProvider) DeleteFile(ctx context.Context, fileID string) error {
	m.fileCalled = true
	return nil
}
func (m *mockCacheAndFileProvider) ListFiles(ctx context.Context) ([]string, error) {
	m.fileCalled = true
	return []string{"file1"}, nil
}
func (m *mockCacheAndFileProvider) GetFile(ctx context.Context, fileID string) (interface{}, error) {
	m.fileCalled = true
	return "file_data", nil
}

func TestTracingLLMClient_CacheAndFileInterfaces(t *testing.T) {
	mockClient := &mockCacheAndFileProvider{}
	tc := NewTracingLLMClient(mockClient, nil)
	ctx := context.Background()

	cacheName, err := tc.CreateCachedContent(ctx, []string{"f1"}, 3600)
	if err != nil || cacheName != "cache1" || !mockClient.cacheCalled {
		t.Errorf("CreateCachedContent failed")
	}
	mockClient.cacheCalled = false

	_, err = tc.GetCachedContent(ctx, "c1")
	if err != nil || !mockClient.cacheCalled {
		t.Errorf("GetCachedContent failed")
	}
	mockClient.cacheCalled = false

	err = tc.DeleteCachedContent(ctx, "c1")
	if err != nil || !mockClient.cacheCalled {
		t.Errorf("DeleteCachedContent failed")
	}
	mockClient.cacheCalled = false

	_, err = tc.ListCachedContent(ctx)
	if err != nil || !mockClient.cacheCalled {
		t.Errorf("ListCachedContent failed")
	}
	mockClient.cacheCalled = false

	tc.SetCachedContent("c1")
	if !mockClient.cacheCalled {
		t.Errorf("SetCachedContent failed")
	}

	fileName, err := tc.UploadFile(ctx, "p1", "text/plain")
	if err != nil || fileName != "file1" || !mockClient.fileCalled {
		t.Errorf("UploadFile failed")
	}
	mockClient.fileCalled = false

	err = tc.DeleteFile(ctx, "f1")
	if err != nil || !mockClient.fileCalled {
		t.Errorf("DeleteFile failed")
	}
	mockClient.fileCalled = false

	_, err = tc.ListFiles(ctx)
	if err != nil || !mockClient.fileCalled {
		t.Errorf("ListFiles failed")
	}
	mockClient.fileCalled = false

	_, err = tc.GetFile(ctx, "f1")
	if err != nil || !mockClient.fileCalled {
		t.Errorf("GetFile failed")
	}
}

func TestTracingLLMClient_MissingInterfaces(t *testing.T) {
	// A mock that doesn't implement any of the extra interfaces
	mockClient := &baseMockLLMClient{}
	tc := NewTracingLLMClient(mockClient, nil)
	ctx := context.Background()

	if tc.IsThinkingEnabled() != false {
		t.Errorf("Expected IsThinkingEnabled to be false")
	}
	if tc.GetThinkingLevel() != "" {
		t.Errorf("Expected GetThinkingLevel to be empty")
	}
	if tc.GetLastThoughtSummary() != "" {
		t.Errorf("Expected GetLastThoughtSummary to be empty")
	}
	if tc.GetLastThinkingTokens() != 0 {
		t.Errorf("Expected GetLastThinkingTokens to be 0")
	}
	if tc.GetLastThoughtSignature() != "" {
		t.Errorf("Expected GetLastThoughtSignature to be empty")
	}
	if tc.GetLastGroundingSources() != nil {
		t.Errorf("Expected GetLastGroundingSources to be nil")
	}
	if tc.IsGoogleSearchEnabled() != false {
		t.Errorf("Expected IsGoogleSearchEnabled to be false")
	}
	if tc.IsURLContextEnabled() != false {
		t.Errorf("Expected IsURLContextEnabled to be false")
	}

	_, err := tc.CreateCachedContent(ctx, []string{}, 0)
	if err == nil {
		t.Errorf("Expected error from CreateCachedContent")
	}
	_, err = tc.GetCachedContent(ctx, "")
	if err == nil {
		t.Errorf("Expected error from GetCachedContent")
	}
	err = tc.DeleteCachedContent(ctx, "")
	if err == nil {
		t.Errorf("Expected error from DeleteCachedContent")
	}
	_, err = tc.ListCachedContent(ctx)
	if err == nil {
		t.Errorf("Expected error from ListCachedContent")
	}

	_, err = tc.UploadFile(ctx, "", "")
	if err == nil {
		t.Errorf("Expected error from UploadFile")
	}
	err = tc.DeleteFile(ctx, "")
	if err == nil {
		t.Errorf("Expected error from DeleteFile")
	}
	_, err = tc.ListFiles(ctx)
	if err == nil {
		t.Errorf("Expected error from ListFiles")
	}
	_, err = tc.GetFile(ctx, "")
	if err == nil {
		t.Errorf("Expected error from GetFile")
	}
}

type mockSchemaProvider struct {
	baseMockLLMClient
	schemaCalled bool
}

func (m *mockSchemaProvider) SchemaCapable() bool { return true }
func (m *mockSchemaProvider) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt string, schema string) (string, error) {
	m.schemaCalled = true
	return "schema", nil
}

type mockStreamingProvider struct {
	baseMockLLMClient
	streamingCalled bool
}

func (m *mockStreamingProvider) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	m.streamingCalled = true
	return nil, nil
}

type mockModelProvider struct {
	baseMockLLMClient
	model string
}

func (m *mockModelProvider) SetModel(model string) { m.model = model }
func (m *mockModelProvider) GetModel() string { return m.model }

func TestTracingLLMClient_MiscInterfaces(t *testing.T) {
	ctx := context.Background()

	schemaMock := &mockSchemaProvider{}
	tcSchema := NewTracingLLMClient(schemaMock, nil)

	if !tcSchema.SchemaCapable() {
		t.Errorf("expected SchemaCapable to be true")
	}
	res, err := tcSchema.CompleteWithSchema(ctx, "sys", "user", "")
	if err != nil || res != "schema" || !schemaMock.schemaCalled {
		t.Errorf("CompleteWithSchema failed")
	}

	streamMock := &mockStreamingProvider{}
	tcStream := NewTracingLLMClient(streamMock, nil)
	_, _ = tcStream.CompleteWithStreaming(ctx, "sys", "user", true)
	if !streamMock.streamingCalled {
		t.Errorf("CompleteWithStreaming failed")
	}

	modelMock := &mockModelProvider{}
	tcModel := NewTracingLLMClient(modelMock, nil)
	tcModel.SetModel("my_model")
	if tcModel.GetModel() != "my_model" {
		t.Errorf("GetModel failed")
	}

	// DisableSemaphore
	tcSchema.DisableSemaphore()

	// Missing interface fallbacks
	tcMissing := NewTracingLLMClient(&baseMockLLMClient{}, nil)
	if tcMissing.SchemaCapable() {
		t.Errorf("SchemaCapable fallback failed")
	}
	_, err = tcMissing.CompleteWithSchema(ctx, "", "", "")
	if err == nil {
		t.Errorf("CompleteWithSchema fallback failed")
	}
	// Note: CompleteWithStreaming fallback uses base method which does nothing, but doesn't return error
	// SetModel / GetModel fallbacks do nothing
	tcMissing.SetModel("test")
	if tcMissing.GetModel() != "" {
		t.Errorf("GetModel fallback failed")
	}
}
