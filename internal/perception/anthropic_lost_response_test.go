package perception

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// The endpoint fully consumes each inference request before deciding to lose
// its response. This distinguishes an ambiguous exchange from a connect error.
func anthropicLostServer(t *testing.T, reply func(int) (string, bool), onDrop func()) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	seen := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			t.Errorf("endpoint did not receive the request body: bytes=%d err=%v", len(body), err)
		}
		if strings.HasSuffix(r.URL.Path, "/count_tokens") {
			_, _ = io.WriteString(w, `{"input_tokens":4}`)
			return
		}
		text, drop := reply(int(seen.Add(1)))
		if drop {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			if onDrop != nil {
				onDrop()
			}
			_ = conn.Close() // no headers or usage reach the client
			return
		}
		_, _ = io.WriteString(w, text)
	}))
	t.Cleanup(server.Close)
	return server, seen
}

func TestAnthropicLostResponse_PreservesUnknownThroughBroker(t *testing.T) {
	fastRetries(t)
	for _, surface := range []string{"chat", "tools", "stream"} {
		for _, kind := range []string{"clean", "recover", "cancel", "exhausted"} {
			t.Run(surface+"/"+kind, func(t *testing.T) {
				ctx, tracker := trackedContext(t)
				outer, inner := &anthropicAttemptObserver{}, &anthropicAttemptObserver{}
				ctx = usage.WithObserver(usage.WithObserver(ctx, outer), inner)
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				onDrop := func() {
					if kind == "cancel" {
						cancel()
					}
				}
				server, seen := anthropicLostServer(t, func(n int) (string, bool) {
					drop := kind == "exhausted" || (kind != "clean" && n == 1)
					if surface == "stream" {
						return anthropicSSE("end_turn"), drop
					}
					return anthropicChatOK, drop
				}, onDrop)
				raw := anthropicTestClient(server.URL)
				meter := broker.NewMeter(broker.MeterConfig{})
				cfg := meter.ConfigFor(broker.ProviderCreds{Provider: "anthropic", Model: raw.GetModel()})
				receipts := broker.NewRingSink(4)
				cfg.Sink = receipts
				client, err := broker.Wrap(raw, cfg)
				if err != nil {
					t.Fatal(err)
				}
				var text string
				switch surface {
				case "chat":
					text, err = client.CompleteWithSystem(ctx, "sys", "q")
				case "tools":
					var response *types.LLMToolResponse
					response, err = client.CompleteWithTools(ctx, "sys", "q", nil)
					if response != nil {
						text = response.Text
					}
				case "stream":
					content, done := client.CompleteWithStreaming(ctx, "sys", "q", false)
					var terminal types.StreamResult
					// Wait for settlement even when the request context is canceled.
					text, terminal = types.CollectStream(context.Background(), content, done, nil)
					err = terminal.Err
				}
				wantRequests, reports, in, out, status := 1, 1, 3, 7, broker.UsageReported
				if surface == "stream" {
					in, out = 16, 5
				}
				switch kind {
				case "recover":
					wantRequests, status = 2, broker.UsagePartial
				case "cancel":
					reports, in, out, status = 0, 0, 0, broker.UsageUnreported
				case "exhausted":
					wantRequests, reports, in, out, status = llmMaxRetries()+1, 0, 0, 0, broker.UsageUnreported
				}
				if kind == "clean" || kind == "recover" {
					if err != nil || text != "hello" {
						t.Fatalf("successful retry changed: text=%q err=%v", text, err)
					}
				} else if err == nil {
					t.Fatal("expected terminal error")
				}
				if kind == "cancel" && !errors.Is(err, context.Canceled) {
					t.Errorf("cancellation identity changed: %v", err)
				}
				if kind == "exhausted" && !strings.Contains(err.Error(), "max retries exceeded") {
					t.Errorf("retry bound identity changed: %v", err)
				}
				if seen.Load() != int32(wantRequests) {
					t.Errorf("endpoint received %d requests, want %d", seen.Load(), wantRequests)
				}
				for _, o := range []*anthropicAttemptObserver{outer, inner} {
					assertAnthropicAttempts(t, o, reports, in, out, kind != "clean", usageOpChat)
				}
				if got := tracker.Stats().TotalProject.Total; got != int64(in+out) {
					t.Errorf("known tokens=%d, want %d", got, in+out)
				}
				got := receipts.Receipts()
				if len(got) != 1 || got[0].UsageStatus != status || got[0].Actual.InputTokens != int64(in) || got[0].Actual.OutputTokens != int64(out) || got[0].UsageConflict != "" {
					t.Errorf("receipt lost uncertainty or known counts: %+v", got)
				}
			})
		}
	}
}

func TestAnthropicLostResponse_KeepsCountedContinuationParts(t *testing.T) {
	fastRetries(t)
	server, seen := anthropicLostServer(t, func(n int) (string, bool) {
		if n == 1 {
			return pausedFetchReply(1000), false
		}
		return continuationDone, n == 2
	}, nil)
	raw := newContinuationClient(t, server.URL, 0, config.AnthropicProviderConfig{WebFetch: true})
	meter := broker.NewMeter(broker.MeterConfig{})
	cfg := meter.ConfigFor(broker.ProviderCreds{Provider: "anthropic", Model: raw.GetModel()})
	receipts := broker.NewRingSink(4)
	cfg.Sink = receipts
	client, err := broker.Wrap(raw, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, tracker := trackedContext(t)
	observer := &anthropicAttemptObserver{}
	ctx = usage.WithObserver(ctx, observer)
	response, err := client.CompleteWithTools(ctx, "sys", "q", nil)
	if err != nil || response == nil || response.Text != "answer" || response.Usage.InputTokens != 40 || response.Usage.OutputTokens != 12 || seen.Load() != 3 {
		t.Fatalf("continuation behavior changed: %+v / %v / %d requests", response, err, seen.Load())
	}
	assertAnthropicAttempts(t, observer, 2, 40, 12, true, usageOpChat)
	if got := tracker.Stats().TotalProject.Total; got != 52 {
		t.Errorf("known tokens=%d, want 52", got)
	}
	got := receipts.Receipts()
	if len(got) != 1 || got[0].UsageStatus != broker.UsagePartial || got[0].Actual.InputTokens != 40 || got[0].Actual.OutputTokens != 12 {
		t.Errorf("continuation receipt: %+v", got)
	}
}
