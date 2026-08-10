package research

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// groundedSearcherMock reproduces GroundedWebSearcher with Supports.

type groundedSearcherMock struct {
	supports bool
	handler  func(ctx context.Context, query string) (*types.GroundedWebSearchResult, error)
	lastQ    string
	calls    int
}

func (m *groundedSearcherMock) SupportsGroundedWebSearch() bool { return m.supports }
func (m *groundedSearcherMock) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	m.lastQ = query
	m.calls++
	if m.handler != nil {
		return m.handler(ctx, query)
	}
	return &types.GroundedWebSearchResult{
		Text:      "mock grounded answer",
		Citations: []types.GroundedCitation{{URL: "https://example.com", Title: "Example", StartIndex: 0, EndIndex: 5}},
		Usage:     types.GroundedUsage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30},
	}, nil
}

var _ types.GroundedWebSearcher = (*groundedSearcherMock)(nil)

func TestGroundedWebSearchTool_Definition(t *testing.T) {
	t.Parallel()
	m := &groundedSearcherMock{supports: true}
	tool := GroundedWebSearchTool(m)
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "grounded_web_search" {
		t.Errorf("name = %q", tool.Name)
	}
	if tool.Execute == nil {
		t.Fatal("execute nil")
	}
	if tool.Category != tools.CategoryResearch {
		t.Errorf("category = %q", tool.Category)
	}
	if len(tool.Schema.Required) != 1 || tool.Schema.Required[0] != "query" {
		t.Errorf("required = %v", tool.Schema.Required)
	}
	if _, ok := tool.Schema.Properties["query"]; !ok {
		t.Fatal("query property missing")
	}
	if tool.Schema.Properties["query"].Type != "string" {
		t.Errorf("query type = %q", tool.Schema.Properties["query"].Type)
	}
}

func TestGroundedWebSearchTool_ExactQueryForwarding(t *testing.T) {
	t.Parallel()
	want := "exact query 42 with  spaces "
	m := &groundedSearcherMock{supports: true, handler: func(_ context.Context, q string) (*types.GroundedWebSearchResult, error) {
		// Must be exact query (not trimmed or mutated) forwarded from args["query"]
		if q != want {
			t.Errorf("forwarded query = %q, want %q", q, want)
		}
		return &types.GroundedWebSearchResult{Text: "ok", Citations: nil, Usage: types.GroundedUsage{}}, nil
	}}
	tool := GroundedWebSearchTool(m)
	res, err := tool.Execute(context.Background(), map[string]any{"query": want})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if m.lastQ != want {
		t.Errorf("lastQ = %q, want %q", m.lastQ, want)
	}
	if m.calls != 1 {
		t.Errorf("calls = %d", m.calls)
	}
	// Result must be valid JSON
	var out map[string]any
	if err := json.Unmarshal([]byte(res), &out); err != nil {
		t.Fatalf("result not JSON: %v, raw=%s", err, res)
	}
}

func TestGroundedWebSearchTool_StructuredOutput(t *testing.T) {
	t.Parallel()
	m := &groundedSearcherMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return &types.GroundedWebSearchResult{
			Text: "grounded answer text",
			Citations: []types.GroundedCitation{
				{URL: "https://example.com/a", Title: "A", StartIndex: 0, EndIndex: 3},
				{URL: "https://example.com/b", Title: "B", StartIndex: 4, EndIndex: 7},
			},
			Usage: types.GroundedUsage{InputTokens: 11, OutputTokens: 22, TotalTokens: 33},
		}, nil
	}}
	tool := GroundedWebSearchTool(m)
	res, err := tool.Execute(context.Background(), map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var parsed struct {
		Text      string                   `json:"text"`
		Citations []types.GroundedCitation `json:"citations"`
		Usage     types.GroundedUsage      `json:"usage"`
		Extra     map[string]any           `json:"-"`
	}
	if err := json.Unmarshal([]byte(res), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Text != "grounded answer text" {
		t.Errorf("text = %q", parsed.Text)
	}
	if len(parsed.Citations) != 2 {
		t.Fatalf("citations len %d", len(parsed.Citations))
	}
	if parsed.Citations[0].URL != "https://example.com/a" {
		t.Errorf("citation[0] = %+v", parsed.Citations[0])
	}
	if parsed.Usage.TotalTokens != 33 {
		t.Errorf("usage = %+v", parsed.Usage)
	}
	// Must not contain config/credentials keys
	raw := map[string]any{}
	_ = json.Unmarshal([]byte(res), &raw)
	for _, forbidden := range []string{"api_key", "apikey", "config", "credentials", "secret", "authorization", "reasoning", "thought"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("output must not contain %q", forbidden)
		}
		// Also check inside text not leaking reasoning marker (text content check is provider-level, but ensure JSON keys)
		if strings.Contains(strings.ToLower(res), forbidden) && forbidden == "reasoning" {
			// Only fail if reasoning appears as a key, not inside legitimate text.
			// Since text is "grounded answer text" it shouldn't contain reasoning.
			t.Errorf("output contains forbidden token %q in JSON: %s", forbidden, res)
		}
	}
}

func TestGroundedWebSearchTool_BoundedOutput(t *testing.T) {
	t.Parallel()
	longText := strings.Repeat("x", 25000)
	manyCitations := make([]types.GroundedCitation, 100)
	for i := range manyCitations {
		manyCitations[i] = types.GroundedCitation{URL: "https://example.com/" + string(rune('a'+i%26)), Title: "t", StartIndex: i, EndIndex: i + 1}
	}
	m := &groundedSearcherMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return &types.GroundedWebSearchResult{Text: longText, Citations: manyCitations, Usage: types.GroundedUsage{TotalTokens: 1}}, nil
	}}
	tool := GroundedWebSearchTool(m)
	res, err := tool.Execute(context.Background(), map[string]any{"query": "q"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var parsed struct {
		Text      string                   `json:"text"`
		Citations []types.GroundedCitation `json:"citations"`
	}
	if err := json.Unmarshal([]byte(res), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Text) > maxGroundedWebSearchOutputChars+len(groundedWebSearchTruncateSuffix) {
		t.Errorf("text not bounded, len %d", len(parsed.Text))
	}
	if !strings.Contains(parsed.Text, "[...truncated...]") {
		t.Error("expected truncation suffix")
	}
	if len(parsed.Citations) > maxGroundedWebSearchCitations {
		t.Errorf("citations not bounded, len %d", len(parsed.Citations))
	}
}

func TestGroundedWebSearchTool_NilAndUnsupportedPaths(t *testing.T) {
	t.Parallel()
	// Nil searcher captured at tool creation
	nilTool := GroundedWebSearchTool(nil)
	_, err := nilTool.Execute(context.Background(), map[string]any{"query": "hello"})
	if err == nil || !strings.Contains(err.Error(), "searcher is nil") {
		t.Fatalf("nil searcher err = %v", err)
	}
	// Unsupported
	m := &groundedSearcherMock{supports: false}
	tool := GroundedWebSearchTool(m)
	_, err = tool.Execute(context.Background(), map[string]any{"query": "hello"})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unsupported err = %v", err)
	}
}

func TestGroundedWebSearchTool_RequiredQuery(t *testing.T) {
	t.Parallel()
	m := &groundedSearcherMock{supports: true}
	tool := GroundedWebSearchTool(m)
	for _, tc := range []map[string]any{
		{},
		{"query": ""},
		{"query": "   "},
	} {
		_, err := tool.Execute(context.Background(), tc)
		if err == nil {
			t.Errorf("args %v: expected error for blank/missing query", tc)
		}
	}
	// Oversized query
	oversized := strings.Repeat("a", 10001)
	_, err := tool.Execute(context.Background(), map[string]any{"query": oversized})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized err = %v", err)
	}
}

func TestGroundedWebSearchTool_ProviderErrors(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("meta grounded search: request failed with status 429: code=rate_limited type=throttle")
	m := &groundedSearcherMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return nil, wantErr
	}}
	tool := GroundedWebSearchTool(m)
	_, err := tool.Execute(context.Background(), map[string]any{"query": "hello"})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if !errors.Is(err, wantErr) && err.Error() != wantErr.Error() {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	// Nil result
	m2 := &groundedSearcherMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return nil, nil
	}}
	tool2 := GroundedWebSearchTool(m2)
	_, err = tool2.Execute(context.Background(), map[string]any{"query": "hello"})
	if err == nil {
		t.Fatal("nil result must error")
	}
}

func TestRegisterGroundedWebSearchIfSupported(t *testing.T) {
	t.Parallel()
	// Nil searcher -> skip
	reg := tools.NewRegistry()
	ok, err := RegisterGroundedWebSearchIfSupported(reg, nil)
	if err != nil {
		t.Fatalf("nil searcher err = %v", err)
	}
	if ok {
		t.Error("nil searcher should not register")
	}
	if reg.Has("grounded_web_search") {
		t.Error("registry should not have tool")
	}
	// Unsupported -> skip
	unsupported := &groundedSearcherMock{supports: false}
	ok, err = RegisterGroundedWebSearchIfSupported(reg, unsupported)
	if err != nil {
		t.Fatalf("unsupported err = %v", err)
	}
	if ok {
		t.Error("unsupported should not register")
	}
	if reg.Has("grounded_web_search") {
		t.Error("registry should not have tool after unsupported")
	}
	// Supported -> register
	supported := &groundedSearcherMock{supports: true}
	ok, err = RegisterGroundedWebSearchIfSupported(reg, supported)
	if err != nil {
		t.Fatalf("supported err = %v", err)
	}
	if !ok {
		t.Error("supported should have registered")
	}
	if !reg.Has("grounded_web_search") {
		t.Fatal("registry should have grounded_web_search")
	}
	// Idempotent: second registration skips
	ok, err = RegisterGroundedWebSearchIfSupported(reg, supported)
	if err != nil {
		t.Fatalf("second err = %v", err)
	}
	if ok {
		t.Error("second should not re-register")
	}
	// Tool works via registry after conditional registration
	res, err := reg.Execute(context.Background(), "grounded_web_search", map[string]any{"query": "hello world"})
	if err != nil {
		t.Fatalf("registry execute: %v", err)
	}
	if res.Result == "" {
		t.Error("expected non-empty result")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Result), &parsed); err != nil {
		t.Fatalf("invalid json: %v, raw=%s", err, res.Result)
	}
	if _, ok := parsed["text"]; !ok {
		t.Error("missing text in output")
	}
	// Existing RegisterAll callers unchanged: RegisterAll does not add grounded_web_search without searcher
	reg2 := tools.NewRegistry()
	if err := RegisterAll(reg2); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if reg2.Has("grounded_web_search") {
		t.Error("RegisterAll must not add grounded_web_search without searcher")
	}
	// Nil registry -> error
	_, err = RegisterGroundedWebSearchIfSupported(nil, supported)
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
	// Alias
	reg3 := tools.NewRegistry()
	ok, err = RegisterGroundedWebSearch(reg3, supported)
	if err != nil || !ok {
		t.Fatalf("alias failed ok=%v err=%v", ok, err)
	}
}

func TestGroundedWebSearchTool_NilContext(t *testing.T) {
	t.Parallel()
	m := &groundedSearcherMock{supports: true}
	tool := GroundedWebSearchTool(m)
	// Pass nil ctx via direct ExecuteFunc call with nil context
	// tools.Execute will substitute Background for nil context, but our ExecuteFunc itself guards.
	// Here we call the closure directly: the handler receives a non-nil context after guard.
	res, err := tool.Execute(nil, map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("nil ctx: %v", err)
	}
	if res == "" {
		t.Error("expected result for nil ctx")
	}
}
