package research

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/tools"
	"codenerd/internal/types"

	gohtml "golang.org/x/net/html"
)

// =============================================================================
// Mock LLM Client for Grounding / Thinking tests
// =============================================================================

// mockLLMClient is a minimal LLMClient for tests.
type mockLLMClient struct {
	completeResponse    string
	completeErr         error
	sysCompleteResponse string
	sysCompleteErr      error
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return m.completeResponse, m.completeErr
}

func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.sysCompleteResponse != "" || m.sysCompleteErr != nil {
		return m.sysCompleteResponse, m.sysCompleteErr
	}
	return m.completeResponse, m.completeErr
}

func (m *mockLLMClient) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error)
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m *mockLLMClient) CompleteWithTools(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: m.completeResponse}, nil
}

// mockGroundingClient implements LLMClient + GroundingController.
type mockGroundingClient struct {
	mockLLMClient
	googleSearchEnabled bool
	urlContextEnabled   bool
	urlContextURLs      []string
	lastGroundingSrc    []string
}

func (m *mockGroundingClient) SetEnableWebSearch(enable bool)    { m.googleSearchEnabled = enable }
func (m *mockGroundingClient) SetEnableURLContext(enable bool)   { m.urlContextEnabled = enable }
func (m *mockGroundingClient) SetURLContextURLs(urls []string)   { m.urlContextURLs = urls }
func (m *mockGroundingClient) IsWebSearchEnabled() bool          { return m.googleSearchEnabled }
func (m *mockGroundingClient) IsURLContextEnabled() bool         { return m.urlContextEnabled }
func (m *mockGroundingClient) GetLastGroundingSources() []string { return m.lastGroundingSrc }

// mockThinkingClient implements LLMClient + ThinkingProvider + ThoughtSignatureProvider.
type mockThinkingClient struct {
	mockLLMClient
	thinkingEnabled  bool
	thinkingLevel    string
	thoughtSummary   string
	thinkingTokens   int
	thoughtSignature string
}

func (m *mockThinkingClient) IsThinkingEnabled() bool         { return m.thinkingEnabled }
func (m *mockThinkingClient) GetThinkingLevel() string        { return m.thinkingLevel }
func (m *mockThinkingClient) GetLastThoughtSummary() string   { return m.thoughtSummary }
func (m *mockThinkingClient) GetLastThinkingTokens() int      { return m.thinkingTokens }
func (m *mockThinkingClient) GetLastThoughtSignature() string { return m.thoughtSignature }

// =============================================================================
// CONTEXT7: whole documents, loud max_docs bound (limits cleanup 2026-09-29)
// =============================================================================

func TestParseLlmsTxt_WhenDocExceedsOldCut_ShouldReturnWhole(t *testing.T) {
	// The old code cut every doc at 8000 chars with "[...truncated...]".
	// Docs now come back whole; the ledger sizes them for the window.
	withBoundResearchRuntime(t)
	big := strings.Repeat("0123456789abcdef", 800) // 12800 chars
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, big)
	}))
	defer ts.Close()

	docs, err := parseLlmsTxt(context.Background(), "o", "r",
		"[big]("+ts.URL+"/big.md)", 10)
	if err != nil {
		t.Fatalf("parseLlmsTxt: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs len = %d, want 1", len(docs))
	}
	if !strings.Contains(docs[0], big) {
		t.Errorf("doc cut: got %d chars of %d", len(docs[0]), len(big))
	}
	if strings.Contains(docs[0], "truncated") {
		t.Errorf("doc must not carry a truncation marker: %.80q...", docs[0])
	}
}

func TestParseLlmsTxt_WhenMaxDocsCapped_ShouldNameRemaining(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, strings.Repeat("doc content padding ", 5))
	}))
	defer ts.Close()

	docs, err := parseLlmsTxt(context.Background(), "o", "r",
		fmt.Sprintf("[a](%s/a)\n[b](%s/b)\n[c](%s/c)", ts.URL, ts.URL, ts.URL),
		1)
	if err != nil {
		t.Fatalf("parseLlmsTxt: %v", err)
	}
	joined := strings.Join(docs, "\n")
	if !strings.Contains(joined, "2 more linked docs not fetched") {
		t.Errorf("capped fetch must name the remaining docs, got: %q", joined)
	}
	if !strings.Contains(joined, "max_docs") {
		t.Errorf("capped fetch must name the raising argument, got: %q", joined)
	}
}

func TestFetchURL_WhenBodyExceedsGuard_ShouldMarkNotSilentlyCut(t *testing.T) {
	withBoundResearchRuntime(t)
	payload := strings.Repeat("z", fetchURLGuardBytes+100)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, payload)
	}))
	defer ts.Close()

	got, err := fetchURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchURL: %v", err)
	}
	if !strings.Contains(got, "fetch guard") {
		t.Errorf("guard trip must be marked in the content, got %d chars", len(got))
	}
	if len(got) > fetchURLGuardBytes+300 {
		t.Errorf("guard trip must still bound memory, got %d chars", len(got))
	}
}

// =============================================================================
// CONTEXT7: executeContext7 error paths
// =============================================================================

func TestExecuteContext7_WhenEmptyTopic_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeContext7(context.Background(), map[string]any{
		"topic": "",
	})
	if err == nil {
		t.Error("expected error for empty topic")
	}
	if !strings.Contains(err.Error(), "topic is required") {
		t.Errorf("expected 'topic is required', got %q", err.Error())
	}
}

func TestExecuteContext7_WhenNoTopicArg_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeContext7(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing topic")
	}
}

func TestExecuteContext7_WhenNoDocsFound_ShouldReturnHelpfulMessage(t *testing.T) {
	pinLocalGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	result, err := executeContext7(context.Background(), map[string]any{
		"topic": "nonexistent-lib-xyz",
		"repo":  "owner/repo",
	})
	if err == nil {
		t.Fatalf("expected non-nil error for no docs, got result: %s", result)
	}
	if result != "" {
		t.Errorf("expected empty result on no-docs error, got: %s", result)
	}
	// Case-insensitive: this asserts the condition, not the capitalization.
	if !strings.Contains(strings.ToLower(err.Error()), "no llm-optimized documentation found") {
		t.Errorf("expected a no-documentation-found error, got: %v", err)
	}
}

func TestExecuteContext7_WhenMaxDocsLimitsResults_ShouldRespectLimit(t *testing.T) {
	longContent := strings.Repeat("x", 60) // > 50 chars
	pinLocalGitHubFiles(t, map[string]string{
		"/owner/repo/main/llms.txt":  "- docs/a.md: A\n- docs/b.md: B\n- docs/c.md: C\n",
		"/owner/repo/main/docs/a.md": longContent,
		"/owner/repo/main/docs/b.md": longContent,
		"/owner/repo/main/docs/c.md": longContent,
	})

	result, err := executeContext7(context.Background(), map[string]any{
		"topic":    "test",
		"repo":     "owner/repo",
		"max_docs": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should contain at most 1 doc separator (one doc block)
	docCount := strings.Count(result, "## Source:")
	if docCount > 1 {
		t.Errorf("expected at most 1 doc block in output, got %d", docCount)
	}
}

// =============================================================================
// CONTEXT7: parseLlmsTxt edge cases
// =============================================================================

func TestParseLlmsTxt_WhenCommentAndBlankLines_ShouldSkipThem(t *testing.T) {
	longContent := strings.Repeat("documentation content ", 10)
	pinLocalGitHubFiles(t, map[string]string{
		"/owner/repo/main/docs/real.md": longContent,
	})

	content := "# Comment line\n\n> Blockquote\n- docs/real.md: Real Doc\n"
	results, err := parseLlmsTxt(context.Background(), "owner", "repo", content, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestParseLlmsTxt_WhenMarkdownLink_ShouldExtractURL(t *testing.T) {
	longContent := strings.Repeat("documentation content ", 10)
	pinLocalGitHubFiles(t, map[string]string{
		"/owner/repo/main/docs/guide.md": longContent,
	})

	content := "[Getting Started](docs/guide.md)\n"
	results, err := parseLlmsTxt(context.Background(), "owner", "repo", content, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result from markdown link, got %d", len(results))
	}
}

func TestParseLlmsTxt_WhenAbsoluteURL_ShouldNotPrefixGitHub(t *testing.T) {
	withBoundResearchRuntime(t)
	longContent := strings.Repeat("external documentation ", 10)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, longContent)
	}))
	defer ts.Close()

	content := ts.URL + "/docs.md\n"
	results, err := parseLlmsTxt(context.Background(), "owner", "repo", content, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result from absolute URL, got %d", len(results))
	}
	if len(results) > 0 && !strings.Contains(results[0], ts.URL+"/docs.md") {
		t.Errorf("expected source URL in result, got %s", results[0])
	}
}

func TestParseLlmsTxt_WhenMaxDocsReached_ShouldStop(t *testing.T) {
	longContent := strings.Repeat("y", 60)
	pinLocalGitHubFiles(t, map[string]string{
		"/o/r/main/d1.md": longContent,
		"/o/r/main/d2.md": longContent,
		"/o/r/main/d3.md": longContent,
	})

	content := "d1.md\nd2.md\nd3.md\n"
	results, err := parseLlmsTxt(context.Background(), "o", "r", content, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 docs fetched; the third entry is the loud cap notice, not a doc.
	if len(results) != 3 {
		t.Fatalf("expected 2 docs plus the cap notice, got %d", len(results))
	}
	if !strings.Contains(results[2], "1 more linked docs not fetched") {
		t.Errorf("third entry must name the skipped doc, got %q", results[2])
	}
}

// =============================================================================
// CONTEXT7: fetchCommonDocs
// =============================================================================

func TestFetchCommonDocs_WhenREADMEExists_ShouldReturnIt(t *testing.T) {
	readmeContent := strings.Repeat("readme content ", 20) // > 100 chars
	pinLocalGitHubFiles(t, map[string]string{
		"/owner/repo/main/README.md": readmeContent,
	})

	results, err := fetchCommonDocs(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result from fetchCommonDocs")
	}
	if len(results) > 0 && !strings.Contains(results[0], "readme content") {
		t.Errorf("expected readme content, got: %s", results[0])
	}
}

func TestFetchCommonDocs_WhenNothingFound_ShouldReturnEmpty(t *testing.T) {
	pinLocalGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	results, err := fetchCommonDocs(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFetchCommonDocs_WhenMaxDocsReached_ShouldStop(t *testing.T) {
	longContent := strings.Repeat("z", 120) // > 100 chars
	pinLocalGitHubFiles(t, map[string]string{
		"/owner/repo/main/README.md":               longContent,
		"/owner/repo/main/docs/README.md":          longContent,
		"/owner/repo/main/documentation/README.md": longContent,
	})

	results, err := fetchCommonDocs(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 1 {
		t.Errorf("expected at most 1 result with maxDocs=1, got %d", len(results))
	}
}

// =============================================================================
// CONTEXT7: fetchURL
// =============================================================================

func TestFetchURL_WhenSuccessful_ShouldReturnBody(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "codeNERD") {
			t.Errorf("expected codeNERD user-agent, got %q", ua)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "response body")
	}))
	defer ts.Close()

	content, err := fetchURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "response body" {
		t.Errorf("expected 'response body', got %q", content)
	}
}

// A Context7 API key is not a credential for the raw documentation URLs
// this reader loads, so the browser read must not attach it.
func TestFetchURL_DoesNotSendAuthorization(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("page read sent Authorization %q", auth)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	content, err := fetchURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "ok" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestFetchURL_WhenNon200_ShouldReturnError(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	_, err := fetchURL(context.Background(), ts.URL)
	if err == nil {
		t.Error("expected error for 403")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("expected HTTP 403 error, got: %v", err)
	}
}

func TestFetchURL_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	withBoundResearchRuntime(t)
	_, err := fetchURL(context.Background(), "://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestFetchURL_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // delay to trigger context cancel
		fmt.Fprint(w, "too late")
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err := fetchURL(ctx, ts.URL)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

// =============================================================================
// CONTEXT7: inferRepo extended
// =============================================================================

func TestInferRepo_WhenCaseInsensitive_ShouldMatchLowercase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		topic    string
		expected string
	}{
		{"React", "facebook/react"},
		{"REACT", "facebook/react"},
		{"Vue", "vuejs/vue"},
		{"Cobra", "spf13/cobra"},
		{"gin", "gin-gonic/gin"},
		{"nextjs", "vercel/next.js"},
		{"bubbletea", "charmbracelet/bubbletea"},
		{"sqlite-vec", "asg017/sqlite-vec"},
		{"testify", "stretchr/testify"},
		{"zap", "uber-go/zap"},
	}
	for _, tt := range tests {
		got := inferRepo(tt.topic)
		if got != tt.expected {
			t.Errorf("inferRepo(%q) = %q, want %q", tt.topic, got, tt.expected)
		}
	}
}

func TestInferRepo_WhenSlashPresent_ShouldReturnAsIs(t *testing.T) {
	t.Parallel()
	got := inferRepo("custom/repo-name")
	if got != "custom/repo-name" {
		t.Errorf("inferRepo(\"custom/repo-name\") = %q, want \"custom/repo-name\"", got)
	}
}

func TestInferRepo_WhenUnknownNoSlash_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	got := inferRepo("totally-unknown-lib-xyzzy")
	if got != "" {
		t.Errorf("inferRepo for unknown = %q, want empty", got)
	}
}

// =============================================================================
// CACHE: hashKey
// =============================================================================

func TestHashKey_WhenSameInputs_ShouldReturnSameHash(t *testing.T) {
	t.Parallel()
	h1 := hashKey("a", "b", "c")
	h2 := hashKey("a", "b", "c")
	if h1 != h2 {
		t.Errorf("hashKey determinism: %q != %q", h1, h2)
	}
}

func TestHashKey_WhenDifferentInputs_ShouldReturnDifferentHash(t *testing.T) {
	t.Parallel()
	h1 := hashKey("a", "b")
	h2 := hashKey("c", "d")
	if h1 == h2 {
		t.Errorf("hashKey collision: %q == %q", h1, h2)
	}
}

func TestHashKey_WhenEmpty_ShouldReturnNonEmpty(t *testing.T) {
	t.Parallel()
	h := hashKey()
	if len(h) != 16 {
		t.Errorf("expected 16 char hash, got %d chars: %q", len(h), h)
	}
}

func TestHashKey_WhenSinglePart_ShouldReturnFixedLength(t *testing.T) {
	t.Parallel()
	h := hashKey("only-one-part")
	if len(h) != 16 {
		t.Errorf("expected 16 char hash, got %d chars: %q", len(h), h)
	}
}

// =============================================================================
// CACHE: Expiration
// =============================================================================

func TestResearchCache_WhenExpired_ShouldReturnNotFound(t *testing.T) {
	t.Parallel()
	cache := NewResearchCache(100, 1*time.Millisecond) // Tiny TTL
	cache.Set("expire-key", "expire-val", "test")
	time.Sleep(5 * time.Millisecond) // Wait for expiration

	_, found := cache.Get("expire-key")
	if found {
		t.Error("expected cache miss for expired entry")
	}
}

func TestResearchCache_WhenOverwrite_ShouldUpdateValue(t *testing.T) {
	t.Parallel()
	cache := NewResearchCache(100, time.Hour)
	cache.Set("key", "value1", "src1")
	cache.Set("key", "value2", "src2")

	entry, found := cache.Get("key")
	if !found {
		t.Fatal("expected to find key after overwrite")
	}
	if entry.Value != "value2" {
		t.Errorf("expected 'value2', got %q", entry.Value)
	}
	if entry.Source != "src2" {
		t.Errorf("expected source 'src2', got %q", entry.Source)
	}
}

// =============================================================================
// CACHE: executeCacheGet with cache miss message
// =============================================================================

func TestExecuteCacheGet_WhenCacheMiss_ShouldReturnCacheMissError(t *testing.T) {
	_, err := executeCacheGet(context.Background(), map[string]any{
		"key": "nonexistent-cache-key-unique-12345",
	})
	if err == nil {
		t.Error("expected error for cache miss")
	}
	if !strings.Contains(err.Error(), "cache miss") {
		t.Errorf("expected 'cache miss' in error, got: %v", err)
	}
}

// =============================================================================
// CACHE: executeCacheSet with source
// =============================================================================

func TestExecuteCacheSet_WhenValidWithSource_ShouldReturnSuccess(t *testing.T) {
	result, err := executeCacheSet(context.Background(), map[string]any{
		"key":    "coverage-test-key",
		"value":  "coverage-test-value",
		"source": "coverage-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Cached") {
		t.Errorf("expected 'Cached' in result, got %q", result)
	}
	if !strings.Contains(result, "coverage-test-key") {
		t.Errorf("expected key in result, got %q", result)
	}
}

func TestExecuteCacheSet_WhenNoSource_ShouldDefaultToUnknown(t *testing.T) {
	result, err := executeCacheSet(context.Background(), map[string]any{
		"key":   "no-source-key",
		"value": "no-source-value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Cached") {
		t.Errorf("expected success, got %q", result)
	}
}

// =============================================================================
// CACHE: executeCacheStats with populated cache
// =============================================================================

func TestExecuteCacheStats_WhenPopulated_ShouldShowSources(t *testing.T) {
	// Pre-populate via set
	executeCacheSet(context.Background(), map[string]any{ //nolint:errcheck
		"key":    "stats-key-1",
		"value":  "stats-val-1",
		"source": "context7",
	})

	result, err := executeCacheStats(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Cache Statistics") {
		t.Errorf("expected 'Cache Statistics' header, got %q", result)
	}
	if !strings.Contains(result, "By source:") {
		t.Errorf("expected 'By source:' section, got %q", result)
	}
}

// =============================================================================
// GROUNDING: GroundingHelper with non-grounding client
// =============================================================================

func TestNewGroundingHelper_WhenBasicClient_ShouldNotBeGrounding(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{completeResponse: "hello"}
	helper := NewGroundingHelper(client)

	if helper.IsGemini() {
		t.Error("expected IsGemini=false for basic client")
	}
	if helper.IsGroundingAvailable() {
		t.Error("expected IsGroundingAvailable=false for basic client")
	}
	if helper.IsWebSearchEnabled() {
		t.Error("expected IsWebSearchEnabled=false for basic client")
	}
	if helper.IsURLContextEnabled() {
		t.Error("expected IsURLContextEnabled=false for basic client")
	}
}

func TestNewGroundingHelper_WhenGroundingClient_ShouldSupportGrounding(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient: mockLLMClient{completeResponse: "grounded answer"},
	}
	helper := NewGroundingHelper(client)

	if !helper.IsGemini() {
		t.Error("expected IsGemini=true")
	}
	if !helper.IsGroundingAvailable() {
		t.Error("expected IsGroundingAvailable=true")
	}
}

func TestGroundingHelper_EnableDisableGoogleSearch(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{}
	helper := NewGroundingHelper(client)

	helper.EnableGoogleSearch()
	if !client.googleSearchEnabled {
		t.Error("expected Google Search to be enabled")
	}
	if !helper.IsWebSearchEnabled() {
		t.Error("expected IsWebSearchEnabled=true")
	}

	helper.DisableGoogleSearch()
	if client.googleSearchEnabled {
		t.Error("expected Google Search to be disabled")
	}
}

func TestGroundingHelper_EnableDisableURLContext(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{}
	helper := NewGroundingHelper(client)

	urls := []string{"https://doc1.example.com", "https://doc2.example.com"}
	helper.EnableURLContext(urls)
	if !client.urlContextEnabled {
		t.Error("expected URL Context to be enabled")
	}
	if len(client.urlContextURLs) != 2 {
		t.Errorf("expected 2 URLs, got %d", len(client.urlContextURLs))
	}

	helper.DisableURLContext()
	if client.urlContextEnabled {
		t.Error("expected URL Context to be disabled")
	}
}

func TestGroundingHelper_EnableURLContext_WhenOver20URLs_ShouldTruncate(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{}
	helper := NewGroundingHelper(client)

	urls := make([]string, 25)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://doc%d.example.com", i)
	}
	helper.EnableURLContext(urls)
	if len(client.urlContextURLs) != 20 {
		t.Errorf("expected 20 URLs after truncation, got %d", len(client.urlContextURLs))
	}
}

func TestGroundingHelper_SetURLContextURLs_WhenNilController_ShouldNoop(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{}
	helper := NewGroundingHelper(client)
	// Should not panic
	helper.SetURLContextURLs([]string{"https://example.com"})
}

func TestGroundingHelper_SetURLContextURLs_WhenOver20_ShouldTruncate(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{}
	helper := NewGroundingHelper(client)

	urls := make([]string, 25)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://url%d.com", i)
	}
	helper.SetURLContextURLs(urls)
	if len(client.urlContextURLs) != 20 {
		t.Errorf("expected 20 URLs, got %d", len(client.urlContextURLs))
	}
}

func TestGroundingHelper_EnableGoogleSearch_WhenNilController_ShouldNoop(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{}
	helper := NewGroundingHelper(client)
	// Should not panic
	helper.EnableGoogleSearch()
	helper.DisableGoogleSearch()
}

func TestGroundingHelper_EnableURLContext_WhenNilController_ShouldNoop(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{}
	helper := NewGroundingHelper(client)
	helper.EnableURLContext([]string{"https://example.com"})
	helper.DisableURLContext()
}

// =============================================================================
// GROUNDING: CaptureGroundingSources
// =============================================================================

func TestGroundingHelper_CaptureGroundingSources_WhenGroundingClient(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		lastGroundingSrc: []string{"https://source1.com", "https://source2.com"},
	}
	helper := NewGroundingHelper(client)

	sources := helper.CaptureGroundingSources()
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}

	lastSources := helper.GetLastGroundingSources()
	if len(lastSources) != 2 {
		t.Errorf("expected 2 last sources, got %d", len(lastSources))
	}
}

func TestGroundingHelper_CaptureGroundingSources_WhenBasicClient_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{}
	helper := NewGroundingHelper(client)

	sources := helper.CaptureGroundingSources()
	if sources != nil {
		t.Errorf("expected nil sources for basic client, got %v", sources)
	}
}

// =============================================================================
// GROUNDING: CompleteWithGrounding
// =============================================================================

func TestGroundingHelper_CompleteWithGrounding_WhenSuccess_ShouldReturnResponseAndSources(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient:    mockLLMClient{completeResponse: "grounded answer"},
		lastGroundingSrc: []string{"https://source.com"},
	}
	helper := NewGroundingHelper(client)

	response, sources, err := helper.CompleteWithGrounding(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "grounded answer" {
		t.Errorf("unexpected response: %q", response)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestGroundingHelper_CompleteWithGrounding_WhenError_ShouldReturnError(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient: mockLLMClient{completeErr: fmt.Errorf("LLM down")},
	}
	helper := NewGroundingHelper(client)

	_, _, err := helper.CompleteWithGrounding(context.Background(), "test")
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "LLM down") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// GROUNDING: CompleteWithSystemAndGrounding
// =============================================================================

func TestGroundingHelper_CompleteWithSystemAndGrounding_WhenSuccess(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient:    mockLLMClient{sysCompleteResponse: "sys grounded"},
		lastGroundingSrc: []string{"https://source.com"},
	}
	helper := NewGroundingHelper(client)

	response, sources, err := helper.CompleteWithSystemAndGrounding(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "sys grounded" {
		t.Errorf("unexpected response: %q", response)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestGroundingHelper_CompleteWithSystemAndGrounding_WhenError(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient: mockLLMClient{sysCompleteResponse: "", sysCompleteErr: fmt.Errorf("sys error")},
	}
	helper := NewGroundingHelper(client)

	_, _, err := helper.CompleteWithSystemAndGrounding(context.Background(), "sys", "user")
	if err == nil {
		t.Error("expected error")
	}
}

// =============================================================================
// GROUNDING: GroundedResearch
// =============================================================================

func TestGroundingHelper_GroundedResearch_WhenSuccess(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient:    mockLLMClient{completeResponse: "researched answer"},
		lastGroundingSrc: []string{"https://research.com"},
	}
	helper := NewGroundingHelper(client)

	result, err := helper.GroundedResearch(context.Background(), "test query", []string{"https://doc.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Query != "test query" {
		t.Errorf("expected query 'test query', got %q", result.Query)
	}
	if result.Response != "researched answer" {
		t.Errorf("expected response 'researched answer', got %q", result.Response)
	}
	if len(result.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(result.Sources))
	}
	if len(result.DocURLs) != 1 {
		t.Errorf("expected 1 doc URL, got %d", len(result.DocURLs))
	}
}

func TestGroundingHelper_GroundedResearch_WhenNoDocURLs(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient: mockLLMClient{completeResponse: "no docs answer"},
	}
	helper := NewGroundingHelper(client)

	result, err := helper.GroundedResearch(context.Background(), "topic", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response != "no docs answer" {
		t.Errorf("unexpected response: %q", result.Response)
	}
}

func TestGroundingHelper_GroundedResearch_WhenError_ShouldReturnWrappedError(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		mockLLMClient: mockLLMClient{completeErr: fmt.Errorf("network error")},
	}
	helper := NewGroundingHelper(client)

	_, err := helper.GroundedResearch(context.Background(), "query", nil)
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "grounded research failed") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

// =============================================================================
// GROUNDING: GetStats
// =============================================================================

func TestGroundingHelper_GetStats(t *testing.T) {
	t.Parallel()
	client := &mockGroundingClient{
		lastGroundingSrc: []string{"src1"},
	}
	helper := NewGroundingHelper(client)
	helper.CaptureGroundingSources()

	stats := helper.GetStats()
	if !stats.IsGemini {
		t.Error("expected IsGemini=true")
	}
	if stats.TotalSearches != 1 {
		t.Errorf("expected 1 total search, got %d", stats.TotalSearches)
	}
	if stats.LastSourcesCount != 1 {
		t.Errorf("expected 1 last source, got %d", stats.LastSourcesCount)
	}
}

// =============================================================================
// GROUNDING: truncateQuery
// =============================================================================

func TestTruncateQuery_WhenShort_ShouldReturnOriginal(t *testing.T) {
	t.Parallel()
	got := truncateQuery("short query")
	if got != "short query" {
		t.Errorf("expected original, got %q", got)
	}
}

func TestTruncateQuery_WhenLong_ShouldTruncateWith3Dots(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 100)
	got := truncateQuery(long)
	if len(got) != 50 {
		t.Errorf("expected 50 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected '...' suffix, got %q", got)
	}
}

func TestTruncateQuery_WhenExactly50_ShouldReturnOriginal(t *testing.T) {
	t.Parallel()
	exact := strings.Repeat("a", 50)
	got := truncateQuery(exact)
	if got != exact {
		t.Errorf("expected original 50-char string")
	}
}

// =============================================================================
// GROUNDING: GetDocURLsForTech
// =============================================================================

func TestGetDocURLsForTech_WhenKnownTech_ShouldReturnURLs(t *testing.T) {
	t.Parallel()
	urls := GetDocURLsForTech("go")
	if len(urls) == 0 {
		t.Error("expected Go doc URLs")
	}
	if urls[0] != "https://go.dev/doc/" {
		t.Errorf("unexpected first URL: %s", urls[0])
	}
}

func TestGetDocURLsForTech_WhenCaseInsensitive_ShouldMatch(t *testing.T) {
	t.Parallel()
	urls := GetDocURLsForTech("Python")
	if len(urls) == 0 {
		t.Error("expected Python doc URLs (case insensitive)")
	}
}

func TestGetDocURLsForTech_WhenUnknown_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	urls := GetDocURLsForTech("unknown-tech-xyz")
	if urls != nil {
		t.Errorf("expected nil for unknown tech, got %v", urls)
	}
}

// =============================================================================
// GROUNDING: GetDocURLsForTechs
// =============================================================================

func TestGetDocURLsForTechs_WhenMultiple_ShouldDedup(t *testing.T) {
	t.Parallel()
	urls := GetDocURLsForTechs([]string{"go", "go"})
	seen := make(map[string]bool)
	for _, u := range urls {
		if seen[u] {
			t.Errorf("duplicate URL: %s", u)
		}
		seen[u] = true
	}
}

func TestGetDocURLsForTechs_WhenCatalogRepeated_ReturnsEveryDistinctURL(t *testing.T) {
	t.Parallel()
	techs := make([]string, 0)
	want := make(map[string]struct{})
	for tech, docURLs := range CommonDocURLs {
		techs = append(techs, tech, tech)
		for _, u := range docURLs {
			want[u] = struct{}{}
		}
	}
	urls := GetDocURLsForTechs(techs)
	if len(urls) != len(want) {
		t.Fatalf("got %d URLs, want every distinct catalog URL (%d)", len(urls), len(want))
	}
	seen := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		if _, ok := want[u]; !ok {
			t.Errorf("unexpected URL %q", u)
		}
		if _, dup := seen[u]; dup {
			t.Errorf("duplicate URL %q", u)
		}
		seen[u] = struct{}{}
	}
}

func TestGetDocURLsForTechs_WhenEmpty_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	urls := GetDocURLsForTechs(nil)
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs, got %d", len(urls))
	}
}

// =============================================================================
// THINKING: ThinkingHelper with non-thinking client
// =============================================================================

func TestNewThinkingHelper_WhenBasicClient_ShouldNotBeThinking(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{completeResponse: "hello"}
	helper := NewThinkingHelper(client)

	if helper.IsThinkingAvailable() {
		t.Error("expected IsThinkingAvailable=false")
	}
	if helper.IsSignatureAvailable() {
		t.Error("expected IsSignatureAvailable=false")
	}
	if helper.GetThinkingLevel() != "" {
		t.Errorf("expected empty thinking level, got %q", helper.GetThinkingLevel())
	}
}

func TestNewThinkingHelper_WhenThinkingClient_ShouldBeThinking(t *testing.T) {
	t.Parallel()
	client := &mockThinkingClient{
		thinkingEnabled:  true,
		thinkingLevel:    "high",
		thoughtSummary:   "I analyzed the problem",
		thinkingTokens:   500,
		thoughtSignature: "sig-abc-123",
	}
	helper := NewThinkingHelper(client)

	if !helper.IsThinkingAvailable() {
		t.Error("expected IsThinkingAvailable=true")
	}
	if !helper.IsSignatureAvailable() {
		t.Error("expected IsSignatureAvailable=true")
	}
	if helper.GetThinkingLevel() != "high" {
		t.Errorf("expected 'high' thinking level, got %q", helper.GetThinkingLevel())
	}
}

// =============================================================================
// THINKING: CaptureThinkingMetadata
// =============================================================================

func TestThinkingHelper_CaptureThinkingMetadata_WhenThinkingClient(t *testing.T) {
	t.Parallel()
	client := &mockThinkingClient{
		thinkingEnabled:  true,
		thinkingLevel:    "medium",
		thoughtSummary:   "analyzed thoroughly",
		thinkingTokens:   250,
		thoughtSignature: "sig-xyz",
	}
	helper := NewThinkingHelper(client)

	metadata := helper.CaptureThinkingMetadata()
	if metadata.ThoughtSummary != "analyzed thoroughly" {
		t.Errorf("unexpected summary: %q", metadata.ThoughtSummary)
	}
	if metadata.ThinkingTokens != 250 {
		t.Errorf("expected 250 tokens, got %d", metadata.ThinkingTokens)
	}
	if metadata.ThoughtSignature != "sig-xyz" {
		t.Errorf("unexpected signature: %q", metadata.ThoughtSignature)
	}
	if metadata.ThinkingLevel != "medium" {
		t.Errorf("unexpected level: %q", metadata.ThinkingLevel)
	}

	// Verify internal state updated
	if helper.GetLastThoughtSummary() != "analyzed thoroughly" {
		t.Error("GetLastThoughtSummary mismatch")
	}
	if helper.GetLastThoughtSignature() != "sig-xyz" {
		t.Error("GetLastThoughtSignature mismatch")
	}
	if helper.GetLastThinkingTokens() != 250 {
		t.Error("GetLastThinkingTokens mismatch")
	}
}

func TestThinkingHelper_CaptureThinkingMetadata_WhenBasicClient_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{}
	helper := NewThinkingHelper(client)

	metadata := helper.CaptureThinkingMetadata()
	if metadata.ThoughtSummary != "" {
		t.Errorf("expected empty summary, got %q", metadata.ThoughtSummary)
	}
	if metadata.ThinkingTokens != 0 {
		t.Errorf("expected 0 tokens, got %d", metadata.ThinkingTokens)
	}
}

// =============================================================================
// THINKING: GetStats
// =============================================================================

func TestThinkingHelper_GetStats(t *testing.T) {
	t.Parallel()
	client := &mockThinkingClient{
		thinkingEnabled: true,
		thinkingLevel:   "low",
		thinkingTokens:  100,
	}
	helper := NewThinkingHelper(client)
	helper.CaptureThinkingMetadata()
	helper.CaptureThinkingMetadata() // capture twice

	stats := helper.GetStats()
	if !stats.IsEnabled {
		t.Error("expected IsEnabled=true")
	}
	if stats.TotalCaptures != 2 {
		t.Errorf("expected 2 captures, got %d", stats.TotalCaptures)
	}
	if stats.TotalThinkingTokens != 200 {
		t.Errorf("expected 200 total tokens, got %d", stats.TotalThinkingTokens)
	}
	if stats.ThinkingLevel != "low" {
		t.Errorf("expected level 'low', got %q", stats.ThinkingLevel)
	}
}

// =============================================================================
// THINKING: MultiTurnContext
// =============================================================================

func TestThinkingHelper_NewMultiTurnContext(t *testing.T) {
	t.Parallel()
	client := &mockThinkingClient{
		thinkingEnabled:  true,
		thoughtSignature: "initial-sig",
	}
	helper := NewThinkingHelper(client)
	helper.CaptureThinkingMetadata()

	mtc := helper.NewMultiTurnContext()
	if mtc.TurnNumber != 1 {
		t.Errorf("expected turn 1, got %d", mtc.TurnNumber)
	}
	if mtc.ThoughtSignature != "initial-sig" {
		t.Errorf("unexpected signature: %q", mtc.ThoughtSignature)
	}
	if !mtc.HasSignature() {
		t.Error("expected HasSignature=true")
	}
}

func TestMultiTurnContext_UpdateFromResponse(t *testing.T) {
	t.Parallel()
	client := &mockThinkingClient{
		thinkingEnabled:  true,
		thoughtSignature: "sig-turn-2",
		thinkingTokens:   50,
	}
	helper := NewThinkingHelper(client)

	mtc := &MultiTurnContext{TurnNumber: 1}
	mtc.UpdateFromResponse(helper)

	if mtc.TurnNumber != 2 {
		t.Errorf("expected turn 2, got %d", mtc.TurnNumber)
	}
	if mtc.ThoughtSignature != "sig-turn-2" {
		t.Errorf("unexpected signature: %q", mtc.ThoughtSignature)
	}
}

func TestMultiTurnContext_HasSignature_WhenEmpty_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	mtc := &MultiTurnContext{}
	if mtc.HasSignature() {
		t.Error("expected HasSignature=false for empty signature")
	}
}

// =============================================================================
// WEB_FETCH: cleanMarkdown
// =============================================================================

func TestCleanMarkdown_WhenMultipleNewlines_ShouldCollapse(t *testing.T) {
	t.Parallel()
	input := "line1\n\n\n\n\nline2"
	result := cleanMarkdown(input)
	if strings.Contains(result, "\n\n\n") {
		t.Errorf("expected collapsed newlines, got %q", result)
	}
}

func TestCleanMarkdown_WhenMultipleSpaces_ShouldCollapse(t *testing.T) {
	t.Parallel()
	input := "word1    word2\ttab"
	result := cleanMarkdown(input)
	if strings.Contains(result, "    ") {
		t.Errorf("expected collapsed spaces, got %q", result)
	}
}

func TestCleanMarkdown_WhenWhitespaceLines_ShouldTrim(t *testing.T) {
	t.Parallel()
	input := "  line1  \n  line2  "
	result := cleanMarkdown(input)
	lines := strings.SplitSeq(result, "\n")
	for line := range lines {
		if line != strings.TrimSpace(line) {
			t.Errorf("line not trimmed: %q", line)
		}
	}
}

// =============================================================================
// WEB_FETCH: getAttr
// =============================================================================

func TestGetAttr_WhenFound_ShouldReturnValue(t *testing.T) {
	t.Parallel()
	html := `<a href="https://example.com" class="link">text</a>`
	doc, err := parseHTMLString(html)
	if err != nil {
		t.Fatal(err)
	}
	// Find the <a> node
	aNode := findElement(doc, "a")
	if aNode == nil {
		t.Fatal("expected to find <a> element")
	}
	val := getAttr(aNode, "href")
	if val != "https://example.com" {
		t.Errorf("expected href value, got %q", val)
	}
}

func TestGetAttr_WhenNotFound_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	html := `<a href="https://example.com">text</a>`
	doc, err := parseHTMLString(html)
	if err != nil {
		t.Fatal(err)
	}
	aNode := findElement(doc, "a")
	if aNode == nil {
		t.Fatal("expected to find <a> element")
	}
	val := getAttr(aNode, "nonexistent")
	if val != "" {
		t.Errorf("expected empty for missing attr, got %q", val)
	}
}

// =============================================================================
// WEB_FETCH: executeWebFetch edge cases
// =============================================================================

func TestExecuteWebFetch_WhenEmptyURL_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeWebFetch(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty url")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected 'url is required', got %v", err)
	}
}

func TestExecuteWebFetch_WhenMarkdownContentType_ShouldReturnAsIs(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Heading\n\nParagraph content")
	}))
	defer ts.Close()

	result, err := executeWebFetch(context.Background(), map[string]any{
		"url": ts.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "# Heading") {
		t.Errorf("expected markdown heading, got %q", result)
	}
}

func TestExecuteWebFetch_WhenNoMaxLength_ShouldReturnWhole(t *testing.T) {
	// Limits cleanup 2026-09-29: the old 50000 default cut is gone; an
	// unpaged fetch returns the content whole for the ledger to size.
	withBoundResearchRuntime(t)
	payload := strings.Repeat("y", 60000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, payload)
	}))
	defer ts.Close()

	result, err := executeWebFetch(context.Background(), map[string]any{
		"url": ts.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != payload {
		t.Errorf("unpaged fetch cut: got %d chars of %d", len(result), len(payload))
	}
}

func TestExecuteWebFetch_WhenMaxLengthSet_ShouldPageWithOffsetHint(t *testing.T) {
	withBoundResearchRuntime(t)
	payload := strings.Repeat("x", 1000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, payload)
	}))
	defer ts.Close()

	result, err := executeWebFetch(context.Background(), map[string]any{
		"url":        ts.URL,
		"max_length": 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "950 more chars, page with offset=50") {
		t.Errorf("paged fetch must name the remainder and offset, got %q", result)
	}
	// The second page reaches the rest; concatenated pages equal the whole.
	page2, err := executeWebFetch(context.Background(), map[string]any{
		"url": ts.URL, "max_length": 950, "offset": 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	page1, _, _ := strings.Cut(result, "\n\n[950 more")
	if page1+strings.SplitN(page2, "\n\n[end of content", 2)[0] != payload {
		t.Errorf("paged windows do not reassemble the whole content")
	}
}

func TestExecuteWebFetch_WhenBodyExceedsGuard_ShouldMarkNotSilentlyCut(t *testing.T) {
	withBoundResearchRuntime(t)
	payload := strings.Repeat("z", webFetchGuardBytes+100)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, payload)
	}))
	defer ts.Close()

	result, err := executeWebFetch(context.Background(), map[string]any{
		"url": ts.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "fetch guard") {
		t.Errorf("guard trip must be marked, got %d chars", len(result))
	}
}

func TestHtmlToMarkdown_WhenNestingPastGuard_ShouldMarkSkippedLevels(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < maxExtractDepth+10; i++ {
		sb.WriteString("<div>")
	}
	sb.WriteString("deep text")
	for i := 0; i < maxExtractDepth+10; i++ {
		sb.WriteString("</div>")
	}
	sb.WriteString("</body></html>")

	md, err := htmlToMarkdown(sb.String(), "http://base.com", true)
	if err != nil {
		t.Fatalf("htmlToMarkdown: %v", err)
	}
	if !strings.Contains(md, "parse guard") {
		t.Errorf("deep-nesting skip must be marked, got %q", md)
	}
}

func TestExecuteWebFetch_WhenIncludeLinksIsFalse_ShouldExcludeLinks(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><a href="http://example.com">Link Text</a></body></html>`)
	}))
	defer ts.Close()

	result, err := executeWebFetch(context.Background(), map[string]any{
		"url":           ts.URL,
		"include_links": false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "](http://example.com)") {
		t.Errorf("expected links to be excluded, got %q", result)
	}
	if !strings.Contains(result, "Link Text") {
		t.Errorf("expected text content, got %q", result)
	}
}

// =============================================================================
// WEB_FETCH: htmlToMarkdown with various elements
// =============================================================================

func TestHtmlToMarkdown_WhenScriptAndStyle_ShouldSkipThem(t *testing.T) {
	t.Parallel()
	htmlStr := `<html><body>
		<script>alert('bad')</script>
		<style>body{color:red}</style>
		<p>Visible content</p>
	</body></html>`
	md, err := htmlToMarkdown(htmlStr, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "alert") {
		t.Errorf("expected script content to be skipped, got: %q", md)
	}
	if strings.Contains(md, "color:red") {
		t.Errorf("expected style content to be skipped, got: %q", md)
	}
	if !strings.Contains(md, "Visible content") {
		t.Error("expected visible content")
	}
}

func TestHtmlToMarkdown_WhenCodeAndPre_ShouldFormatAsCode(t *testing.T) {
	t.Parallel()
	htmlStr := `<html><body><code>inline code</code> and <pre>block code</pre></body></html>`
	md, err := htmlToMarkdown(htmlStr, "", true)
	if err != nil {
		t.Fatal(err)
	}
	// The converter adds a trailing space inside the element before closing backtick
	if !strings.Contains(md, "`inline code") {
		t.Errorf("expected inline code markers, got %q", md)
	}
	if !strings.Contains(md, "```") {
		t.Errorf("expected code block, got %q", md)
	}
}

func TestHtmlToMarkdown_WhenBoldAndItalic_ShouldFormatMarkdown(t *testing.T) {
	t.Parallel()
	htmlStr := `<html><body><strong>bold</strong> and <em>italic</em></body></html>`
	md, err := htmlToMarkdown(htmlStr, "", true)
	if err != nil {
		t.Fatal(err)
	}
	// The converter wraps with ** and * but text nodes add trailing space
	if !strings.Contains(md, "**bold") {
		t.Errorf("expected bold markers, got %q", md)
	}
	if !strings.Contains(md, "*italic") {
		t.Errorf("expected italic markers, got %q", md)
	}
}

func TestHtmlToMarkdown_WhenImg_ShouldExtractAlt(t *testing.T) {
	t.Parallel()
	htmlStr := `<html><body><img alt="photo of cat" src="cat.jpg"/></body></html>`
	md, err := htmlToMarkdown(htmlStr, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "[Image: photo of cat]") {
		t.Errorf("expected image alt text, got %q", md)
	}
}

func TestHtmlToMarkdown_WhenHeadings_ShouldPrefixWithHash(t *testing.T) {
	t.Parallel()
	htmlStr := `<html><body>
		<h1>H1</h1><h2>H2</h2><h3>H3</h3>
		<h4>H4</h4><h5>H5</h5><h6>H6</h6>
	</body></html>`
	md, err := htmlToMarkdown(htmlStr, "", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"# H1", "## H2", "### H3", "#### H4", "##### H5", "###### H6"} {
		if !strings.Contains(md, h) {
			t.Errorf("expected %q in markdown, got %q", h, md)
		}
	}
}

func TestHtmlToMarkdown_WhenAnchorHashLink_ShouldSkipLink(t *testing.T) {
	t.Parallel()
	htmlStr := `<html><body><a href="#section">anchor</a></body></html>`
	md, err := htmlToMarkdown(htmlStr, "", true)
	if err != nil {
		t.Fatal(err)
	}
	// Hash links should be skipped - no markdown link syntax
	if strings.Contains(md, "](#section)") {
		t.Errorf("expected hash link to be skipped, got %q", md)
	}
}

// =============================================================================
// WEB_SEARCH: executeWebSearch edge cases
// =============================================================================

func TestExecuteWebSearch_WhenEmptyQuery_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeWebSearch(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Errorf("expected 'query is required', got %v", err)
	}
}

func TestExecuteWebSearch_WhenManyResults_ShouldReturnAllPastOldCap(t *testing.T) {
	// Limits cleanup 2026-09-29: the old default-10 plus hard-30 cap is
	// gone. An unpaged search returns every result the page holds.
	// The page is a local DuckDuckGo-shaped document: the parser runs on
	// the rendered DOM, and the default suite does not call the public site.
	withBoundResearchRuntime(t)
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := range 35 {
		sb.WriteString(fmt.Sprintf(`<div class="result results_links"><a class="result__a" href="https://ex%d.com">Title %d</a></div>`, i, i))
	}
	sb.WriteString("</body></html>")
	body := sb.String()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(ts.Close)
	prevURL := duckDuckGoSearchURL
	duckDuckGoSearchURL = ts.URL + "/html/?q=%s"
	t.Cleanup(func() { duckDuckGoSearchURL = prevURL })

	result, err := executeWebSearch(context.Background(), map[string]any{
		"query": "test search",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Found 35 results") {
		t.Errorf("unpaged search dropped results, got: %.200q...", result)
	}

	// An explicit max_results is the model's own choice and is honored.
	result, err = executeWebSearch(context.Background(), map[string]any{
		"query": "test search", "max_results": 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Found 2 results") {
		t.Errorf("explicit max_results not honored, got: %.200q...", result)
	}
}

// =============================================================================
// WEB_SEARCH: parseDuckDuckGoResults
// =============================================================================

func TestParseDuckDuckGoResults_WhenValidHTML_ShouldParseResults(t *testing.T) {
	t.Parallel()
	htmlContent := `<html><body>
		<div class="result results_links">
			<a class="result__a" href="https://example.com">Example Title</a>
			<a class="result__snippet">This is a snippet</a>
		</div>
	</body></html>`

	results, err := parseDuckDuckGoResults(htmlContent, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Example Title" {
		t.Errorf("unexpected title: %q", results[0].Title)
	}
	if results[0].URL != "https://example.com" {
		t.Errorf("unexpected URL: %q", results[0].URL)
	}
	if results[0].Snippet != "This is a snippet" {
		t.Errorf("unexpected snippet: %q", results[0].Snippet)
	}
}

func TestParseDuckDuckGoResults_WhenDDGRedirectURL_ShouldDecode(t *testing.T) {
	t.Parallel()
	htmlContent := `<html><body>
		<div class="result results_links">
			<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Freal-site.com%2Fpage&rut=abc">Title</a>
		</div>
	</body></html>`

	results, err := parseDuckDuckGoResults(htmlContent, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL != "https://real-site.com/page" {
		t.Errorf("expected decoded URL, got %q", results[0].URL)
	}
}

func TestParseDuckDuckGoResults_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	results, err := parseDuckDuckGoResults(`<html><body></body></html>`, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseDuckDuckGoResults_WhenMaxResultsReached_ShouldStop(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := range 5 {
		sb.WriteString(fmt.Sprintf(`<div class="result results_links"><a class="result__a" href="https://ex%d.com">Title %d</a></div>`, i, i))
	}
	sb.WriteString("</body></html>")

	results, err := parseDuckDuckGoResults(sb.String(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestParseDuckDuckGoResults_WhenUnlimited_ShouldReturnAll(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := range 5 {
		sb.WriteString(fmt.Sprintf(`<div class="result results_links"><a class="result__a" href="https://ex%d.com">Title %d</a></div>`, i, i))
	}
	sb.WriteString("</body></html>")

	results, err := parseDuckDuckGoResults(sb.String(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected all 5 results, got %d", len(results))
	}
}

// =============================================================================
// WEB_SEARCH: getAttrValue and getTextContent
// =============================================================================

func TestGetAttrValue_WhenFound_ShouldReturnValue(t *testing.T) {
	t.Parallel()
	htmlStr := `<a class="test-class" href="https://example.com">text</a>`
	doc, err := parseHTMLString(htmlStr)
	if err != nil {
		t.Fatal(err)
	}
	aNode := findElement(doc, "a")
	if aNode == nil {
		t.Fatal("expected to find <a>")
	}
	val := getAttrValue(aNode, "class")
	if val != "test-class" {
		t.Errorf("expected 'test-class', got %q", val)
	}
}

func TestGetAttrValue_WhenNotFound_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	htmlStr := `<a href="https://example.com">text</a>`
	doc, err := parseHTMLString(htmlStr)
	if err != nil {
		t.Fatal(err)
	}
	aNode := findElement(doc, "a")
	if aNode == nil {
		t.Fatal("expected to find <a>")
	}
	val := getAttrValue(aNode, "id")
	if val != "" {
		t.Errorf("expected empty for missing attr, got %q", val)
	}
}

func TestGetTextContent_WhenNestedElements_ShouldExtractAll(t *testing.T) {
	t.Parallel()
	htmlStr := `<div><span>Hello</span> <b>World</b></div>`
	doc, err := parseHTMLString(htmlStr)
	if err != nil {
		t.Fatal(err)
	}
	divNode := findElement(doc, "div")
	if divNode == nil {
		t.Fatal("expected to find <div>")
	}
	text := getTextContent(divNode)
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "World") {
		t.Errorf("expected 'Hello' and 'World', got %q", text)
	}
}

// =============================================================================
// BROWSER: Validation error paths
// =============================================================================

func TestExecuteBrowserNavigate_WhenEmptyURL_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeBrowserNavigate(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected 'url is required', got %v", err)
	}
}

func TestExecuteBrowserExtract_WhenEmptySessionID_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeBrowserExtract(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty session_id")
	}
	if !strings.Contains(err.Error(), "session_id is required") {
		t.Errorf("expected 'session_id is required', got %v", err)
	}
}

func TestExecuteBrowserScreenshot_WhenEmptySessionID_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeBrowserScreenshot(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty session_id")
	}
}

func TestExecuteBrowserClick_WhenMissingArgs_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeBrowserClick(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing session_id")
	}

	_, err = executeBrowserClick(context.Background(), map[string]any{
		"session_id": "test-session",
	})
	if err == nil {
		t.Error("expected error for missing selector")
	}
}

func TestExecuteBrowserType_WhenMissingArgs_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeBrowserType(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing session_id")
	}

	_, err = executeBrowserType(context.Background(), map[string]any{
		"session_id": "test-session",
	})
	if err == nil {
		t.Error("expected error for missing selector")
	}

	_, err = executeBrowserType(context.Background(), map[string]any{
		"session_id": "test-session",
		"selector":   "#input",
	})
	if err == nil {
		t.Error("expected error for missing text")
	}
}

func TestExecuteBrowserClose_WhenEmptySessionID_ShouldReturnError(t *testing.T) {
	t.Parallel()
	_, err := executeBrowserClose(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty session_id")
	}
}

func TestExecuteBrowserClose_WhenValidSessionID_ShouldCloseSession(t *testing.T) {
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, nil)
	t.Cleanup(func() { ClearBrowserManager(mgr) })

	result, err := executeBrowserClose(context.Background(), map[string]any{
		"session_id": "test-session-123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "test-session-123") {
		t.Errorf("expected session ID in result, got %q", result)
	}
	if !strings.Contains(result, "closed") {
		t.Errorf("expected 'closed', got %q", result)
	}
}

// =============================================================================
// REGISTER: RegisterAll
// =============================================================================

func TestRegisterAll_WhenValidRegistry_ShouldRegisterAllTools(t *testing.T) {
	t.Parallel()
	registry := tools.NewRegistry()
	err := RegisterAll(registry)
	if err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	expectedTools := []string{
		"context7_fetch",
		"web_search",
		"web_fetch",
		"browser_navigate",
		"browser_extract",
		"browser_screenshot",
		"browser_click",
		"browser_type",
		"browser_close",
		"browser_observe",
		"browser_act",
		"browser_mangle",
		"browser_wait",
		"browser_reason",
		"browser_audit",
		"browser_evidence",
		"browser_specs",
		"browser_test",
		"research_cache_get",
		"research_cache_set",
		"research_cache_clear",
		"research_cache_stats",
	}

	for _, name := range expectedTools {
		if !registry.Has(name) {
			t.Errorf("expected tool %q to be registered", name)
		}
	}

	if registry.Count() != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), registry.Count())
	}
}

func TestRegisterAll_WhenCalledTwice_ShouldNotReturnError(t *testing.T) {
	t.Parallel()
	registry := tools.NewRegistry()
	err := RegisterAll(registry)
	if err != nil {
		t.Fatalf("first RegisterAll failed: %v", err)
	}

	err = RegisterAll(registry)
	if err != nil {
		t.Errorf("expected no error when registering duplicate tools, got %v", err)
	}
}

// =============================================================================
// HELPERS for HTML parsing in tests
// =============================================================================

// parseHTMLString is a test helper to parse an HTML string into a node tree.
func parseHTMLString(htmlStr string) (*gohtml.Node, error) {
	doc, err := gohtml.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// findElement traverses the HTML tree to find the first element with the given tag.
func findElement(n *gohtml.Node, tag string) *gohtml.Node {
	if n.Type == gohtml.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}
