package core

import (
	"context"
	"encoding/json"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// fakeGroundedSearcher is a test double for types.GroundedWebSearcher.
type fakeGroundedSearcher struct {
	supports bool
	handler  func(context.Context, string) (*types.GroundedWebSearchResult, error)
	lastQ    string
	calls    int
}

func (f *fakeGroundedSearcher) SupportsGroundedWebSearch() bool { return f.supports }

func (f *fakeGroundedSearcher) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	f.lastQ = query
	f.calls++
	if f.handler != nil {
		return f.handler(ctx, query)
	}
	return &types.GroundedWebSearchResult{
		Text:      "mock grounded answer",
		Citations: []types.GroundedCitation{{URL: "https://example.com", Title: "Example", StartIndex: 0, EndIndex: 5}},
		Usage:     types.GroundedUsage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30},
	}, nil
}

var _ types.GroundedWebSearcher = (*fakeGroundedSearcher)(nil)

func TestHydrateModularTools_SourceCompatibleWithoutGroundedSearch(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	if err := vs.HydrateModularTools(); err != nil {
		t.Fatalf("HydrateModularTools() error = %v", err)
	}
	local := vs.GetModularTools()
	if local == nil {
		t.Fatal("GetModularTools() = nil")
	}
	if local.Has("grounded_web_search") {
		t.Fatal("fresh local registry must not have grounded_web_search after HydrateModularTools() with no searcher")
	}
	// Intentionally no assertion on tools.Global() absence: global is process-shared.
}

func TestHydrateModularTools_UnsupportedSearcherSkipsLocalRegistration(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	fake := &fakeGroundedSearcher{supports: false}
	if err := vs.HydrateModularTools(fake); err != nil {
		t.Fatalf("HydrateModularTools(unsupported) error = %v", err)
	}
	local := vs.GetModularTools()
	if local == nil {
		t.Fatal("GetModularTools() = nil")
	}
	if local.Has("grounded_web_search") {
		t.Fatal("local registry must not have grounded_web_search when searcher SupportsGroundedWebSearch is false")
	}
	// No global absence assertion: global is process-shared and may have been populated by another test.
}

func TestHydrateModularTools_SupportedSearcherRegistersAndForwardsExactQuery(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	wantQuery := "exact query 42 with  spaces "
	var captured string
	fake := &fakeGroundedSearcher{
		supports: true,
		handler: func(_ context.Context, q string) (*types.GroundedWebSearchResult, error) {
			captured = q
			if q != wantQuery {
				t.Errorf("forwarded query = %q, want %q", q, wantQuery)
			}
			return &types.GroundedWebSearchResult{
				Text: "grounded answer text",
				Citations: []types.GroundedCitation{
					{URL: "https://example.com/a", Title: "A", StartIndex: 0, EndIndex: 3},
					{URL: "https://example.com/b", Title: "B", StartIndex: 4, EndIndex: 7},
				},
				Usage: types.GroundedUsage{InputTokens: 11, OutputTokens: 22, TotalTokens: 33},
			}, nil
		},
	}
	if err := vs.HydrateModularTools(fake); err != nil {
		t.Fatalf("HydrateModularTools(supported) error = %v", err)
	}
	local := vs.GetModularTools()
	if local == nil {
		t.Fatal("GetModularTools() = nil")
	}
	if !local.Has("grounded_web_search") {
		t.Fatal("local registry must have grounded_web_search after supported searcher")
	}
	// Positive global assertion is order-independent: once registered, global must contain it.
	if !tools.Global().Has("grounded_web_search") {
		t.Fatal("global registry must have grounded_web_search after supported HydrateModularTools")
	}
	res, err := local.Execute(context.Background(), "grounded_web_search", map[string]any{"query": wantQuery})
	if err != nil {
		t.Fatalf("local Execute grounded_web_search: %v", err)
	}
	if res == nil || res.Result == "" {
		t.Fatal("expected non-empty structured JSON result")
	}
	if captured != wantQuery {
		t.Errorf("handler captured query = %q, want %q", captured, wantQuery)
	}
	if fake.lastQ != wantQuery {
		t.Errorf("fake.lastQ = %q, want %q", fake.lastQ, wantQuery)
	}
	if fake.calls != 1 {
		t.Errorf("calls = %d, want 1", fake.calls)
	}
	var parsed struct {
		Text      string                   `json:"text"`
		Citations []types.GroundedCitation `json:"citations"`
		Usage     types.GroundedUsage      `json:"usage"`
	}
	if err := json.Unmarshal([]byte(res.Result), &parsed); err != nil {
		t.Fatalf("result not valid JSON: %v, raw=%s", err, res.Result)
	}
	if parsed.Text != "grounded answer text" {
		t.Errorf("text = %q, want %q", parsed.Text, "grounded answer text")
	}
	if len(parsed.Citations) != 2 {
		t.Fatalf("citations len = %d, want 2", len(parsed.Citations))
	}
	if parsed.Citations[0].URL != "https://example.com/a" {
		t.Errorf("citation[0] URL = %q", parsed.Citations[0].URL)
	}
	if parsed.Usage.TotalTokens != 33 || parsed.Usage.InputTokens != 11 || parsed.Usage.OutputTokens != 22 {
		t.Errorf("usage = %+v, want {11 22 33}", parsed.Usage)
	}
	// Ensure output is bounded structured JSON with only text/citations/usage keys.
	var raw map[string]any
	if err := json.Unmarshal([]byte(res.Result), &raw); err != nil {
		t.Fatalf("raw unmarshal: %v", err)
	}
	if _, ok := raw["text"]; !ok {
		t.Error("structured JSON missing text")
	}
	if _, ok := raw["citations"]; !ok {
		t.Error("structured JSON missing citations")
	}
	if _, ok := raw["usage"]; !ok {
		t.Error("structured JSON missing usage")
	}
	for _, forbidden := range []string{"api_key", "apikey", "config", "credentials", "secret", "authorization"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("output must not contain %q", forbidden)
		}
	}
}
