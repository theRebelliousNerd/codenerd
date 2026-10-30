package perception

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Stream conformance: every supported streaming adapter, fed recorded-shape
// SSE bodies from a fake HTTP server, must publish provisional chunks and then
// exactly one terminal result whose outcome follows the provider's own
// terminal evidence. The same scenario table runs against every provider, so
// the contract is symmetric by construction rather than by review.

// streamScenario names a provider-neutral fixture.
type streamScenario string

const (
	scComplete      streamScenario = "complete"        // deltas, normal finish, usage
	scEOF           streamScenario = "eof_no_terminal" // deltas, then a clean EOF
	scLength        streamScenario = "length"          // deltas, length finish
	scMalformed     streamScenario = "malformed"       // a malformed event between deltas
	scMultiline     streamScenario = "multiline"       // a delta framed over two data lines
	scFilter        streamScenario = "filter"          // provider safety stop
	scToolFinish    streamScenario = "tool_finish"     // a tool-call finish on a text stream
	scNoUsageTail   streamScenario = "no_usage_tail"   // normal finish, usage block never arrives
	scHeartbeat     streamScenario = "heartbeat"       // comments and event: lines between deltas
	scProviderError streamScenario = "provider_error"  // an error event mid-stream
)

// streamFixture renders one provider's wire format.
type streamFixture struct {
	name string
	// start is the events before the first delta.
	start []string
	// delta is one visible-text event's data.
	delta func(text string) string
	// multiline is a delta's data split across two data lines.
	multiline func(text string) string
	// finish is the events that end generation for a finish kind
	// ("stop", "length", "filter", "tool"), with or without the usage block.
	finish func(kind string, withUsage bool) []string
	// providerError is one error event's data.
	providerError string
	// stream builds the client against baseURL and starts a stream.
	stream func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult)
	// usage is what the fixture's usage block normalizes to.
	usage types.UsageMetadata
	// filterOutcome is the class its filter finish maps to.
	filterOutcome types.StreamOutcome
}

func chatDelta(text string) string {
	return fmt.Sprintf(`{"choices":[{"index":0,"delta":{"content":%q}}]}`, text)
}

func chatFinish(kind string, withUsage bool) []string {
	reason := map[string]string{"stop": "stop", "length": "length", "filter": "content_filter", "tool": "tool_calls"}[kind]
	out := []string{fmt.Sprintf(`{"choices":[{"index":0,"delta":{},"finish_reason":%q}]}`, reason)}
	if withUsage {
		out = append(out, `{"choices":[],"usage":{"prompt_tokens":20,"completion_tokens":9,"total_tokens":29,"prompt_tokens_details":{"cached_tokens":12},"completion_tokens_details":{"reasoning_tokens":4}}}`)
	}
	return append(out, "[DONE]")
}

var chatUsageWant = types.UsageMetadata{
	InputTokens: 20, OutputTokens: 9, TotalTokens: 29, CachedContentTokens: 12, ThinkingTokens: 4,
	Known: types.UsageTokens | types.UsageCacheRead | types.UsageReasoning,
}

// responsesFixture renders the Responses event stream: typed events, the
// usage and finish carried on one terminal response event.
func responsesFixture(name string, stream func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult)) streamFixture {
	return streamFixture{
		name: name,
		start: []string{
			`{"type":"response.created","response":{"id":"resp_1","status":"in_progress","output":[]}}`,
			`{"type":"response.output_item.added","item":{"id":"msg_1","type":"message","role":"assistant","phase":"final_answer","content":[]}}`,
		},
		delta: func(text string) string {
			return fmt.Sprintf(`{"type":"response.output_text.delta","item_id":"msg_1","content_index":0,"delta":%q}`, text)
		},
		multiline: func(text string) string {
			return "{\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\n" + fmt.Sprintf(`"delta":%q}`, text)
		},
		finish: func(kind string, withUsage bool) []string {
			usage := ""
			if withUsage {
				usage = `,"usage":{"input_tokens":20,"output_tokens":9,"total_tokens":29,"input_tokens_details":{"cached_tokens":12,"cache_write_tokens":3},"output_tokens_details":{"reasoning_tokens":4}}`
			}
			msg := `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello world"}]}`
			switch kind {
			case "length":
				return []string{`{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[` + msg + `]` + usage + `}}`}
			case "filter":
				return []string{`{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"content_filter"},"output":[` + msg + `]` + usage + `}}`}
			case "tool":
				return []string{`{"type":"response.completed","response":{"status":"completed","output":[` + msg + `,{"type":"function_call","call_id":"c1","name":"read_file","arguments":"{}"}]` + usage + `}}`}
			default:
				return []string{`{"type":"response.completed","response":{"status":"completed","output":[` + msg + `]` + usage + `}}`}
			}
		},
		providerError: `{"type":"error","code":"server_is_overloaded","message":"overloaded"}`,
		stream:        stream,
		usage: types.UsageMetadata{
			InputTokens: 20, OutputTokens: 9, TotalTokens: 29, CachedContentTokens: 12, CacheWriteTokens: 3, ThinkingTokens: 4,
			Known: types.UsageTokens | types.UsageCacheRead | types.UsageCacheWrite | types.UsageReasoning,
		},
		filterOutcome: types.StreamFiltered,
	}
}

func chatFixture(name string, stream func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult)) streamFixture {
	return streamFixture{
		name:  name,
		delta: chatDelta,
		multiline: func(text string) string {
			return "{\"choices\":[{\"index\":0,\n" + fmt.Sprintf(`"delta":{"content":%q}}]}`, text)
		},
		finish:        chatFinish,
		providerError: `{"error":{"message":"overloaded","type":"server_error"}}`,
		stream:        stream,
		usage:         chatUsageWant,
		filterOutcome: types.StreamFiltered,
	}
}

func streamFixtures() []streamFixture {
	opts := func(baseURL string) (string, time.Duration) { return baseURL, 5 * time.Second }
	return []streamFixture{
		responsesFixture("openai", func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
			u, _ := opts(baseURL)
			return newTestOpenAIClient(t, u, "gpt-6.1-sol").CompleteWithStreaming(ctx, "sys", "user", false)
		}),
		chatFixture("openrouter", func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
			u, to := opts(baseURL)
			return NewOpenRouterClientWithConfig(OpenRouterConfig{APIKey: "k", BaseURL: u, Model: "some/model", Timeout: to}).CompleteWithStreaming(ctx, "sys", "user", false)
		}),
		chatFixture("meta", func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
			u, to := opts(baseURL)
			c, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderMeta, APIKey: "k", BaseURL: u, Model: "muse-spark-1.3-contributor", Timeout: to})
			if err != nil {
				t.Fatalf("meta client: %v", err)
			}
			return c.CompleteWithStreaming(ctx, "sys", "user", false)
		}),
		chatFixture("dashscope", func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
			u, to := opts(baseURL)
			c, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderDashScope, APIKey: "k", BaseURL: u, Model: "qwen", Timeout: to})
			if err != nil {
				t.Fatalf("dashscope client: %v", err)
			}
			return c.CompleteWithStreaming(ctx, "sys", "user", false)
		}),
		chatFixture("zai", func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
			u, to := opts(baseURL)
			return NewZAIClientWithConfig(ZAIConfig{APIKey: "k", BaseURL: u, Model: "glm", Timeout: to}).CompleteWithStreaming(ctx, "sys", "user", false)
		}),
		{
			name:  "anthropic",
			start: []string{`{"type":"message_start","message":{"usage":{"input_tokens":5,"cache_read_input_tokens":12,"cache_creation_input_tokens":3,"output_tokens":1}}}`, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
			delta: func(text string) string {
				return fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, text)
			},
			multiline: func(text string) string {
				return "{\"type\":\"content_block_delta\",\"index\":0,\n" + fmt.Sprintf(`"delta":{"type":"text_delta","text":%q}}`, text)
			},
			finish: func(kind string, withUsage bool) []string {
				reason := map[string]string{"stop": "end_turn", "length": "max_tokens", "filter": "refusal", "tool": "tool_use"}[kind]
				usage := ""
				if withUsage {
					usage = `,"usage":{"output_tokens":9}`
				}
				return []string{fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q}%s}`, reason, usage), `{"type":"message_stop"}`}
			},
			providerError: `{"type":"error","error":{"type":"overloaded_error"}}`,
			stream: func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
				return NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", BaseURL: baseURL, Model: "claude-x", Timeout: 5 * time.Second}).CompleteWithStreaming(ctx, "sys", "user", false)
			},
			usage: types.UsageMetadata{
				InputTokens: 20, OutputTokens: 9, TotalTokens: 29, CachedContentTokens: 12, CacheWriteTokens: 3,
				Known: types.UsageTokens | types.UsageCacheRead | types.UsageCacheWrite,
			},
			filterOutcome: types.StreamRefused,
		},
		{
			name: "gemini",
			delta: func(text string) string {
				return fmt.Sprintf(`{"candidates":[{"content":{"parts":[{"text":%q}],"role":"model"}}]}`, text)
			},
			multiline: func(text string) string {
				return "{\"candidates\":[{\"content\":\n" + fmt.Sprintf(`{"parts":[{"text":%q}],"role":"model"}}]}`, text)
			},
			finish: func(kind string, withUsage bool) []string {
				reason := map[string]string{"stop": "STOP", "length": "MAX_TOKENS", "filter": "SAFETY", "tool": "MALFORMED_FUNCTION_CALL"}[kind]
				usage := ""
				if withUsage {
					usage = `,"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":5,"thoughtsTokenCount":4,"cachedContentTokenCount":12,"totalTokenCount":29}`
				}
				return []string{fmt.Sprintf(`{"candidates":[{"content":{"parts":[],"role":"model"},"finishReason":%q}]%s}`, reason, usage)}
			},
			providerError: `{"error":{"code":503,"message":"overloaded","status":"UNAVAILABLE"}}`,
			stream: func(t *testing.T, ctx context.Context, baseURL string) (<-chan string, <-chan types.StreamResult) {
				return NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", BaseURL: baseURL, Model: "gemini-x", Timeout: 5 * time.Second}).CompleteWithStreaming(ctx, "sys", "user", false)
			},
			usage: types.UsageMetadata{
				InputTokens: 20, OutputTokens: 9, TotalTokens: 29, CachedContentTokens: 12, ThinkingTokens: 4,
				Known: types.UsageTokens | types.UsageCacheRead | types.UsageReasoning,
			},
			filterOutcome: types.StreamFiltered,
		},
	}
}

// body renders a scenario as an SSE body for fixture f.
func (f streamFixture) body(sc streamScenario) string {
	var b strings.Builder
	event := func(data string) { fmt.Fprintf(&b, "data: %s\n\n", data) }
	for _, e := range f.start {
		event(e)
	}
	finish := func(kind string, withUsage bool) {
		for _, e := range f.finish(kind, withUsage) {
			event(e)
		}
	}
	switch sc {
	case scComplete:
		event(f.delta("hello "))
		event(f.delta("world"))
		finish("stop", true)
	case scEOF:
		event(f.delta("hello "))
		event(f.delta("world"))
	case scLength:
		event(f.delta("hello "))
		event(f.delta("world"))
		finish("length", true)
	case scMalformed:
		event(f.delta("hello "))
		event(`{"choices":[{"delta":{"content":"lost`)
		event(f.delta("world"))
		finish("stop", true)
	case scMultiline:
		event(f.delta("hello "))
		parts := strings.SplitN(f.multiline("world"), "\n", 2)
		fmt.Fprintf(&b, "data: %s\ndata: %s\n\n", parts[0], parts[1])
		finish("stop", true)
	case scFilter:
		event(f.delta("hello "))
		finish("filter", true)
	case scToolFinish:
		event(f.delta("hello "))
		finish("tool", true)
	case scNoUsageTail:
		event(f.delta("hello "))
		event(f.delta("world"))
		finish("stop", false)
	case scHeartbeat:
		b.WriteString(": keep-alive\n\n")
		event(f.delta("hello "))
		b.WriteString(": ping\nevent: message\n")
		event(f.delta("world"))
		finish("stop", true)
	case scProviderError:
		event(f.delta("hello "))
		event(f.providerError)
	}
	return b.String()
}

// recordingUsageObserver captures every usage report the adapter delivers.
type recordingUsageObserver struct {
	mu      sync.Mutex
	reports []usage.Report
}

func (o *recordingUsageObserver) Observed(r usage.Report) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reports = append(o.reports, r)
}

func runConformance(t *testing.T, f streamFixture, sc streamScenario) (string, types.StreamResult, []usage.Report) {
	t.Helper()
	body := f.body(sc)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	obs := &recordingUsageObserver{}
	ctx, cancel := context.WithTimeout(usage.WithObserver(context.Background(), obs), 10*time.Second)
	defer cancel()
	content, done := f.stream(t, ctx, srv.URL)
	text, res := types.CollectStream(ctx, content, done, nil)
	if _, extra := <-done; extra {
		t.Fatal("a second terminal result was published")
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	return text, res, append([]usage.Report(nil), obs.reports...)
}

func TestStreamConformance(t *testing.T) {
	cases := []struct {
		sc      streamScenario
		outcome func(f streamFixture) types.StreamOutcome
		// text is the visible content the consumer must have received.
		text string
		// partialUsage is whether the terminal usage must be marked partial.
		partialUsage bool
	}{
		{scComplete, fixed(types.StreamComplete), "hello world", false},
		{scEOF, fixed(types.StreamIncomplete), "hello world", true},
		{scLength, fixed(types.StreamTruncated), "hello world", false},
		{scMalformed, fixed(types.StreamProtocolFailed), "hello ", true},
		{scMultiline, fixed(types.StreamComplete), "hello world", false},
		{scFilter, func(f streamFixture) types.StreamOutcome { return f.filterOutcome }, "hello ", false},
		{scToolFinish, fixed(types.StreamUnsupported), "hello ", false},
		{scNoUsageTail, fixed(types.StreamComplete), "hello world", true},
		{scHeartbeat, fixed(types.StreamComplete), "hello world", false},
		{scProviderError, fixed(types.StreamFailed), "hello ", true},
	}
	for _, f := range streamFixtures() {
		for _, tc := range cases {
			t.Run(f.name+"/"+string(tc.sc), func(t *testing.T) {
				text, res, reports := runConformance(t, f, tc.sc)
				want := tc.outcome(f)
				if got := res.Outcome(); got != want {
					t.Fatalf("outcome = %s (%v), want %s", got, res.Err, want)
				}
				if (res.Err == nil) != (want == types.StreamComplete) {
					t.Fatalf("Err = %v for outcome %s: only a complete stream may have a nil error", res.Err, want)
				}
				if text != tc.text {
					t.Fatalf("visible text = %q, want %q", text, tc.text)
				}
				if res.Usage.Partial != tc.partialUsage {
					t.Fatalf("usage partial = %v, want %v (usage %+v)", res.Usage.Partial, tc.partialUsage, res.Usage)
				}
				if want == types.StreamTruncated {
					tr, ok := types.AsOutputTruncated(res.Err)
					if !ok || tr.Partial != "hello world" || !types.LengthStop(tr.Reason) {
						t.Fatalf("truncation report = %+v, want the shared typed error with the partial", tr)
					}
				}
				// Usage reaches the observer once per attempt, before the
				// terminal, as the same normalized report the terminal carries.
				if !tc.partialUsage {
					if len(reports) != 1 {
						t.Fatalf("observer saw %d reports, want exactly 1", len(reports))
					}
					if got := reports[0].Usage; got != f.usage || res.Usage != f.usage {
						t.Fatalf("usage observer %+v terminal %+v, want %+v", got, res.Usage, f.usage)
					}
				}
			})
		}
	}
}

func fixed(o types.StreamOutcome) func(streamFixture) types.StreamOutcome {
	return func(streamFixture) types.StreamOutcome { return o }
}

// A multiline data frame must decode exactly like its single-line equivalent.
func TestStreamConformance_MultilineFrameEqualsSingleLine(t *testing.T) {
	for _, f := range streamFixtures() {
		t.Run(f.name, func(t *testing.T) {
			single, resSingle, _ := runConformance(t, f, scComplete)
			multi, resMulti, _ := runConformance(t, f, scMultiline)
			if single != multi || resSingle.Outcome() != resMulti.Outcome() || resSingle.Usage != resMulti.Usage {
				t.Fatalf("single %q/%s/%+v != multiline %q/%s/%+v",
					single, resSingle.Outcome(), resSingle.Usage, multi, resMulti.Outcome(), resMulti.Usage)
			}
		})
	}
}

// Canceling while the adapter is blocked forwarding must end the stream with
// a canceled outcome, never a success, and release the producer.
func TestStreamConformance_CancelWhileBlocked(t *testing.T) {
	for _, f := range streamFixtures() {
		t.Run(f.name, func(t *testing.T) {
			release := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, e := range f.start {
					fmt.Fprintf(w, "data: %s\n\n", e)
				}
				fmt.Fprintf(w, "data: %s\n\n", f.delta("hello "))
				w.(http.Flusher).Flush()
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}))
			defer srv.Close()
			defer close(release)

			ctx, cancel := context.WithCancel(context.Background())
			content, done := f.stream(t, ctx, srv.URL)
			if chunk := <-content; chunk != "hello " {
				t.Fatalf("first chunk = %q", chunk)
			}
			cancel()
			waitCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			_, res := types.CollectStream(waitCtx, content, done, nil)
			if res.Outcome() != types.StreamCanceled || !errors.Is(res.Err, context.Canceled) {
				t.Fatalf("outcome = %s (%v), want canceled", res.Outcome(), res.Err)
			}
		})
	}
}

// The production path end to end: a streamed Anthropic turn through the real
// adapter and the real broker wrapper delivers the same cache categories to
// usage.json and to the receipt, and the receipt exists by the time the
// consumer has observed the terminal.
func TestStreamConformance_BrokeredCacheDetailReachesTrackerAndReceipt(t *testing.T) {
	var f streamFixture
	for _, fx := range streamFixtures() {
		if fx.name == "anthropic" {
			f = fx
		}
	}
	body := f.body(scComplete)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	sink := &receiptCollector{}
	meter := broker.NewMeter(broker.MeterConfig{Window: 200000, OutputReserve: 8000, ExtraSink: sink})
	client, err := broker.Wrap(
		NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", BaseURL: srv.URL, Model: "claude-x", Timeout: 5 * time.Second}),
		meter.ConfigFor(broker.ProviderCreds{Provider: "anthropic", Model: "claude-x"}))
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	ctx, tracker := trackedContext(t)
	content, done := client.CompleteWithStreaming(ctx, "sys", "user", false)
	text, res := types.CollectStream(ctx, content, done, nil)
	if res.Err != nil || text != "hello world" {
		t.Fatalf("stream = %q, %v", text, res.Err)
	}

	r, ok := sink.last()
	if !ok {
		t.Fatal("no receipt by the time the terminal was observed")
	}
	if r.Actual.CachedTokens != 12 || r.Actual.CacheWriteTokens != 3 || r.UsageStatus != broker.UsageReported {
		t.Fatalf("receipt actual %+v status %q, want cache read 12, write 3, reported", r.Actual, r.UsageStatus)
	}
	tot := tracker.Stats().ByProvider["anthropic"]
	if tot.CacheRead != 12 || tot.CacheWrite != 3 || tot.Input != 20 {
		t.Fatalf("tracker %+v, want the same cache categories as the receipt", tot)
	}
}

type receiptCollector struct {
	mu       sync.Mutex
	receipts []broker.Receipt
}

func (c *receiptCollector) Record(r broker.Receipt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.receipts = append(c.receipts, r)
}

func (c *receiptCollector) last() (broker.Receipt, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.receipts) == 0 {
		return broker.Receipt{}, false
	}
	return c.receipts[len(c.receipts)-1], true
}
