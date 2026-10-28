package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func fetchBlock(t *testing.T, page string) types.ContentBlock {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type":        "web_fetch_tool_result",
		"tool_use_id": "srvtoolu_1",
		"content": map[string]any{
			"type": "web_fetch_result", "url": "https://example.com/a",
			"source": map[string]any{"type": "text", "media_type": "text/plain", "data": page},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return types.ServerToolBlock(raw)
}

func groundedHistory(t *testing.T, page string) []types.Message {
	t.Helper()
	return []types.Message{
		{Role: "user", Text: "read the page"},
		types.NewAssistantMessage(fetchBlock(t, page), types.TextBlock("done")),
	}
}

// Two histories that differ only in the text inside one web_fetch_tool_result
// are replayed to the endpoint at very different sizes, so every measure of
// the request must differ with them.
func TestMeasureCountsReplayedServerToolBlocks(t *testing.T) {
	small := measure(&Request{Messages: groundedHistory(t, "tiny")})
	large := measure(&Request{Messages: groundedHistory(t, strings.Repeat("page ", 4000))})
	if large.History-small.History < 19000 {
		t.Fatalf("a 20k-char fetched page added only %d chars to the measure; a replayed server block is billed and was not counted", large.History-small.History)
	}
}

// The counting endpoint does not take server-tool content. Its count of the
// supported part must not be presented as the request's: the omitted content
// is added as a declared estimate, and the result is not exact.
func TestAnthropicCounterDeclaresServerContentAsEstimate(t *testing.T) {
	srv := newCountServer(t, 1000, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))
	count := func(page string) Count {
		t.Helper()
		c, err := counter.Count(context.Background(), &Request{Model: "claude-opus-5", Messages: groundedHistory(t, page)})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		return c
	}

	small, large := count("tiny"), count(strings.Repeat("page ", 4000))
	if body, _ := srv.lastBody.Load().([]byte); strings.Contains(string(body), "web_fetch") || strings.Contains(string(body), "page page") {
		t.Fatalf("server-tool content was sent to the counting endpoint: %s", body)
	}
	if large.Tokens <= small.Tokens+4000 {
		t.Fatalf("growing a replayed fetch result from 4 to 20000 chars must grow the count: small=%d large=%d", small.Tokens, large.Tokens)
	}
	for name, c := range map[string]Count{"small": small, "large": large} {
		if c.Confidence == ConfidenceExact {
			t.Errorf("%s: a count that estimates server content must not be exact", name)
		}
		if c.EstimatedTokens <= 0 || c.EstimatedTokens >= c.Tokens {
			t.Errorf("%s: estimated share %d of %d tokens is not declared", name, c.EstimatedTokens, c.Tokens)
		}
		if !strings.Contains(c.Source, "server content") {
			t.Errorf("%s: source %q does not name the estimated content", name, c.Source)
		}
		if c.Segments.Total() != c.Tokens {
			t.Errorf("%s: segments %d do not sum to %d", name, c.Segments.Total(), c.Tokens)
		}
	}
}

// With the endpoint failing, the fallback estimate still carries the content.
func TestAnthropicCounterFallbackKeepsServerContent(t *testing.T) {
	srv := newCountServer(t, 0, http.StatusInternalServerError)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))
	small, err := counter.Count(context.Background(), &Request{Model: "claude-opus-5", Messages: groundedHistory(t, "tiny")})
	if err != nil {
		t.Fatal(err)
	}
	large, err := counter.Count(context.Background(), &Request{Model: "claude-opus-5", Messages: groundedHistory(t, strings.Repeat("page ", 4000))})
	if err != nil {
		t.Fatal(err)
	}
	if large.Tokens <= small.Tokens+4000 || large.Confidence == ConfidenceExact {
		t.Fatalf("fallback lost the server content: small=%+v large=%+v", small, large)
	}
}

// A request with no server-tool content is counted exactly as before.
func TestAnthropicCounterWithoutServerContentStaysExact(t *testing.T) {
	srv := newCountServer(t, 777, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))
	c, err := counter.Count(context.Background(), &Request{Model: "claude-opus-5", Messages: []types.Message{{Role: "user", Text: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Tokens != 777 || c.Confidence != ConfidenceExact || c.EstimatedTokens != 0 {
		t.Fatalf("plain request must stay a pure provider count: %+v", c)
	}
}
