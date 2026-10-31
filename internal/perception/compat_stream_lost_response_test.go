package perception

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

const compatLossContent = "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"}}]}\n\n"
const compatLossUsage = "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":1000,\"total_tokens\":2000,\"prompt_tokens_details\":{\"cached_tokens\":400}}}\n\n"
const compatLossFinish = "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"

func TestCompatStreamingLostResponse_PreservesUnknownThroughBroker(t *testing.T) {
	fastRetries(t)
	for _, kind := range []string{"clean", "recover", "cancel", "exhausted", "http400", "midstream_eof"} {
		t.Run(kind, func(t *testing.T) {
			ctx, tracker := trackedContext(t)
			outer, inner := &anthropicAttemptObserver{}, &anthropicAttemptObserver{}
			ctx = usage.WithObserver(usage.WithObserver(ctx, outer), inner)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			var seen atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || len(body) == 0 {
					t.Errorf("request not received: bytes=%d err=%v", len(body), err)
				}
				var request struct {
					Stream  bool `json:"stream"`
					Options struct {
						IncludeUsage bool `json:"include_usage"`
					} `json:"stream_options"`
				}
				if err := json.Unmarshal(body, &request); err != nil || !request.Stream || !request.Options.IncludeUsage {
					t.Errorf("stream usage request changed: %+v err=%v", request, err)
				}
				n := seen.Add(1)
				if kind == "exhausted" || ((kind == "recover" || kind == "cancel") && n == 1) {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					if kind == "cancel" {
						cancel()
					}
					_ = conn.Close() // body consumed, no response headers sent
					return
				}
				if kind == "http400" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, `{"error":{"message":"fixture rejection"}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, compatLossContent)
				if kind != "midstream_eof" {
					_, _ = io.WriteString(w, compatLossFinish)
				}
				_, _ = io.WriteString(w, compatLossUsage)
				if kind != "midstream_eof" {
					_, _ = io.WriteString(w, "data: [DONE]\n\n")
				}
			}))
			defer server.Close()
			raw := newTestCompatClient(t, ProviderDashScope, server.URL)
			meter := broker.NewMeter(broker.MeterConfig{})
			cfg := meter.ConfigFor(broker.ProviderCreds{Provider: "dashscope", Model: raw.GetModel()})
			receipts := broker.NewRingSink(4)
			cfg.Sink = receipts
			client, err := broker.Wrap(raw, cfg)
			if err != nil {
				t.Fatal(err)
			}
			content, done := client.CompleteWithStreaming(ctx, "sys", "q", false)
			// Drain to terminal settlement even when request ctx is canceled.
			text, terminal := types.CollectStream(context.Background(), content, done, nil)
			requests, reports, tokens, cached, status := 1, 1, 1000, 400, broker.UsageReported
			switch kind {
			case "recover":
				requests, status = 2, broker.UsagePartial
			case "cancel", "http400":
				reports, tokens, cached, status = 0, 0, 0, broker.UsageUnreported
			case "exhausted":
				requests, reports, tokens, cached, status = llmMaxRetries()+1, 0, 0, 0, broker.UsageUnreported
			case "midstream_eof":
				status = broker.UsagePartial
			}
			if kind == "clean" || kind == "recover" {
				if terminal.Err != nil || terminal.Outcome() != types.StreamComplete || text != "answer" {
					t.Fatalf("stream completion changed: %q / %+v", text, terminal)
				}
			} else if terminal.Err == nil {
				t.Fatal("expected terminal error")
			}
			if kind == "cancel" && !errors.Is(terminal.Err, context.Canceled) {
				t.Errorf("cancellation identity: %v", terminal.Err)
			}
			if kind == "http400" && !strings.Contains(terminal.Err.Error(), "400") {
				t.Errorf("HTTP error identity: %v", terminal.Err)
			}
			if kind == "exhausted" && !strings.Contains(terminal.Err.Error(), "max retries exceeded") {
				t.Errorf("retry bound error: %v", terminal.Err)
			}
			if kind == "midstream_eof" && (terminal.Outcome() != types.StreamIncomplete || !errors.Is(terminal.Err, types.ErrStreamIncomplete) || text != "answer") {
				t.Errorf("midstream failure changed: %q / %+v", text, terminal)
			}
			if terminal.Usage.InputTokens != tokens || terminal.Usage.OutputTokens != tokens || terminal.Usage.CachedContentTokens != cached {
				t.Errorf("terminal known counts changed: %+v", terminal.Usage)
			}
			if seen.Load() != int32(requests) {
				t.Errorf("endpoint received %d requests, want %d", seen.Load(), requests)
			}
			for _, observer := range []*anthropicAttemptObserver{outer, inner} {
				observer.mu.Lock()
				observed, unknown := append([]usage.Report(nil), observer.reports...), observer.unknown
				observer.mu.Unlock()
				if len(observed) != reports || unknown != (kind != "clean") {
					t.Errorf("reports=%d unknown=%v want %d/%v", len(observed), unknown, reports, kind != "clean")
				}
				for _, r := range observed {
					if r.Provider != "dashscope" || r.Operation != usageOpChat || r.Usage.InputTokens != tokens || r.Usage.OutputTokens != tokens || r.Usage.CachedContentTokens != cached {
						t.Errorf("known report changed: %+v", r)
					}
				}
			}
			if got := tracker.Stats().TotalProject.Total; got != int64(2*tokens) {
				t.Errorf("known tracked tokens=%d want=%d", got, 2*tokens)
			}
			got := receipts.Receipts()
			if len(got) != 1 || got[0].UsageStatus != status || got[0].Actual.InputTokens != int64(tokens) || got[0].Actual.OutputTokens != int64(tokens) || got[0].Actual.CachedTokens != int64(cached) || got[0].UsageConflict != "" {
				t.Errorf("broker uncertainty/counts: %+v", got)
			}
		})
	}
}
