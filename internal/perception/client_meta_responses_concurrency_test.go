package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// metaEchoServer serves the Responses surface with per-conversation reasoning
// markers: each reply carries encrypted_content "marker-for-<firstUserText>",
// and every reasoning block the client replays is recorded under the first
// user text of the request that carried it. The first user text identifies the
// conversation because the client replays history verbatim.
type metaEchoServer struct {
	t        *testing.T
	mu       sync.Mutex
	replayed map[string][]string
	srv      *httptest.Server
}

func newMetaEchoServer(t *testing.T) *metaEchoServer {
	t.Helper()
	m := &metaEchoServer{t: t, replayed: map[string][]string{}}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

// metaEchoFirstUserText returns the first user message text in a Responses
// input array, which identifies the conversation that sent it.
func metaEchoFirstUserText(input []map[string]any) string {
	for _, item := range input {
		if typ, _ := item["type"].(string); typ == "reasoning" {
			continue
		}
		if role, _ := item["role"].(string); role != "user" {
			continue
		}
		if text := metaEchoItemText(item); text != "" {
			return text
		}
	}
	return ""
}

// metaEchoItemText extracts the text of a message item's first content block.
func metaEchoItemText(item map[string]any) string {
	content, ok := item["content"].([]any)
	if !ok {
		return ""
	}
	for _, b := range content {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if txt, _ := bm["text"].(string); txt != "" {
			return txt
		}
	}
	return ""
}

// metaEchoReasoningMarkers collects the encrypted_content of every reasoning
// replay item in a Responses input array.
func metaEchoReasoningMarkers(input []map[string]any) []string {
	var markers []string
	for _, item := range input {
		if typ, _ := item["type"].(string); typ != "reasoning" {
			continue
		}
		enc, _ := item["encrypted_content"].(string)
		if enc == "" {
			continue
		}
		markers = append(markers, enc)
	}
	return markers
}

// metaEchoReply builds a minimal completed Responses reply carrying one
// reasoning block with the given encrypted marker.
func metaEchoReply(marker string) string {
	return fmt.Sprintf(`{"id":"r1","status":"completed","model":"m","output":[`+
		`{"id":"rs1","type":"reasoning","summary":[],"encrypted_content":%q},`+
		`{"id":"m1","type":"message","role":"assistant","status":"completed",`+
		`"content":[{"type":"output_text","text":"ok"}]}]}`,
		marker)
}

func (m *metaEchoServer) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input []map[string]any `json:"input"`
	}
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &req)

	firstUser := metaEchoFirstUserText(req.Input)
	markers := metaEchoReasoningMarkers(req.Input)

	m.mu.Lock()
	m.replayed[firstUser] = append(m.replayed[firstUser], markers...)
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(metaEchoReply("marker-for-" + firstUser)))
}

func (m *metaEchoServer) replayedFor(conv string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.replayed[conv]...)
}

// Two conversations at the same history length, sharing one client and one
// system prompt, must each replay their own reasoning — never each other's.
// Regression test for the turn:<len(history)> key with no conversation
// identity, where the second prime overwrote the first and both replays
// carried the same block.
func TestMetaResponses_ConcurrentConversationsDoNotCrossReplay(t *testing.T) {
	echo := newMetaEchoServer(t)
	c := newTestCompatClient(t, ProviderMeta, echo.srv.URL)

	const systemPrompt = "shared-sys"
	bg := context.Background()
	histA1 := []types.Message{{Role: "user", Text: "alpha-root"}}
	histB1 := []types.Message{{Role: "user", Text: "beta-root"}}
	if _, err := c.CompleteWithToolResults(bg, systemPrompt, histA1, nil); err != nil {
		t.Fatalf("prime A: %v", err)
	}
	if _, err := c.CompleteWithToolResults(bg, systemPrompt, histB1, nil); err != nil {
		t.Fatalf("prime B: %v", err)
	}

	// Same history length (2), each with an assistant turn at index 1 so the
	// turn:1 slot replays. Only the conversation root tells them apart.
	histA2 := []types.Message{{Role: "user", Text: "alpha-root"}, {Role: "assistant", Text: "ack-a"}}
	histB2 := []types.Message{{Role: "user", Text: "beta-root"}, {Role: "assistant", Text: "ack-b"}}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, errs[0] = c.CompleteWithToolResults(bg, systemPrompt, histA2, nil) }()
	go func() { defer wg.Done(); _, errs[1] = c.CompleteWithToolResults(bg, systemPrompt, histB2, nil) }()
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
	}

	for conv, want := range map[string]string{
		"alpha-root": "marker-for-alpha-root",
		"beta-root":  "marker-for-beta-root",
	} {
		got := echo.replayedFor(conv)
		if len(got) != 1 || got[0] != want {
			t.Errorf("conversation %q replayed %q, want exactly [%q]", conv, got, want)
		}
	}
}

// The replay cache must stay bounded no matter how many distinct
// conversations share the client.
func TestMetaResponses_ReasoningCacheStaysBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(metaEchoReply("enc")))
	}))
	t.Cleanup(srv.Close)

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	bg := context.Background()
	for i := 0; i < 200; i++ {
		hist := []types.Message{{Role: "user", Text: fmt.Sprintf("root-%d", i)}}
		if _, err := c.CompleteWithToolResults(bg, fmt.Sprintf("sys-%d", i), hist, nil); err != nil {
			t.Fatalf("conversation %d: %v", i, err)
		}
	}

	c.reasoningMu.Lock()
	defer c.reasoningMu.Unlock()
	convs := map[string]struct{}{}
	for k := range c.reasoningCache {
		conv, ok := metaReasoningConvOf(k)
		if !ok {
			t.Fatalf("cache key %q carries no conversation discriminator", k)
		}
		convs[conv] = struct{}{}
	}
	if len(convs) > metaReasoningMaxConversations {
		t.Fatalf("cache holds %d conversations, want at most %d", len(convs), metaReasoningMaxConversations)
	}
	if max := metaReasoningMaxConversations * metaReasoningMaxTurnsPerConversation; len(c.reasoningCache) > max {
		t.Fatalf("cache holds %d entries, want at most %d", len(c.reasoningCache), max)
	}
}

// The discriminator prefers the task-scoped intent ID from the context when
// present (combined with the root hash), and falls back to the root hash of
// system prompt plus first user text when no session travels with the call.
func TestMetaConversationID_DiscriminatesConversations(t *testing.T) {
	hist := []types.Message{{Role: "user", Text: "same-root"}}
	base := metaConversationID(context.Background(), "sys", hist)
	if base == "" {
		t.Fatal("empty conversation ID")
	}
	if got := metaConversationID(context.Background(), "sys", hist); got != base {
		t.Fatalf("root discriminator not deterministic: %q vs %q", base, got)
	}
	if got := metaConversationID(context.Background(), "sys", []types.Message{{Role: "user", Text: "other-root"}}); got == base {
		t.Fatalf("different roots share discriminator %q", got)
	}
	if got := metaConversationID(context.Background(), "other-sys", hist); got == base {
		t.Fatalf("different system prompts share discriminator %q", got)
	}

	ctxA := types.WithSessionContext(context.Background(), &types.SessionContext{UserIntent: &types.StructuredIntent{ID: "intent-A"}})
	ctxB := types.WithSessionContext(context.Background(), &types.SessionContext{UserIntent: &types.StructuredIntent{ID: "intent-B"}})
	idA := metaConversationID(ctxA, "sys", hist)
	idB := metaConversationID(ctxB, "sys", hist)
	if idA == idB {
		t.Fatalf("different intent IDs share discriminator %q", idA)
	}
	if idA == base {
		t.Fatalf("intent-scoped ID %q ignores the session identity", idA)
	}
	if got := metaConversationID(ctxA, "sys", hist); got != idA {
		t.Fatalf("intent discriminator not stable across turns: %q vs %q", idA, got)
	}
}

// strategicRaceStubClient answers classification with a fixed envelope after a
// short delay, widening the window in which a writer can race the reader.
type strategicRaceStubClient struct{}

func (s *strategicRaceStubClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (s *strategicRaceStubClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	time.Sleep(2 * time.Millisecond)
	return `{"primary_intent":"explain","semantic_type":"definition","action_type":"explain","domain":"general","confidence":0.9,"surface_response":"ok"}`, nil
}

func (s *strategicRaceStubClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	close(ch)
	ech := make(chan error)
	close(ech)
	return ch, ech
}

func (s *strategicRaceStubClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok"}, nil
}

// SetStrategicContext runs live from session boot while in-flight
// Transduce/Understand calls read the field; hammering both must be
// race-clean under -race.
func TestUnderstandingTransducer_SetStrategicContextConcurrentWithTransduce(t *testing.T) {
	tr := NewUnderstandingTransducer(&strategicRaceStubClient{}).(*UnderstandingTransducer)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			tr.SetStrategicContext(strings.Repeat(fmt.Sprintf("strategic-%d ", i), 50))
		}(i)
		go func() {
			defer wg.Done()
			_, _ = tr.ParseIntentWithContext(ctx, "what does the auth module do?", nil)
		}()
	}
	wg.Wait()
}
