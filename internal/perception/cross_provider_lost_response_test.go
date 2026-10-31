package perception

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

const crossLossChatReply = `{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":1000,"total_tokens":2000}}`
const crossLossResponsesReply = `{"id":"r1","status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"answer","annotations":[]}]}],"usage":{"input_tokens":1000,"output_tokens":1000,"total_tokens":2000}}`

func TestCrossProviderLostResponse_PreservesUnknownThroughBroker(t *testing.T) {
	fastRetries(t)
	for _, surface := range []string{"chat", "meta_responses", "grounded_search"} {
		for _, kind := range []string{"clean", "recover", "cancel", "exhausted"} {
			t.Run(surface+"/"+kind, func(t *testing.T) {
				ctx, tracker := trackedContext(t)
				outer, inner := &anthropicAttemptObserver{}, &anthropicAttemptObserver{}
				ctx = usage.WithObserver(usage.WithObserver(ctx, outer), inner)
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				// Reuse the real HTTP fixture: it consumes the inference body
				// before closing without headers, regardless of provider.
				server, seen := anthropicLostServer(t, func(n int) (string, bool) {
					reply := crossLossResponsesReply
					if surface == "chat" {
						reply = crossLossChatReply
					}
					return reply, kind == "exhausted" || (kind != "clean" && n == 1)
				}, func() {
					if kind == "cancel" {
						cancel()
					}
				})
				provider, model := ProviderMeta, "cross-loss-fixture-contributor"
				if surface == "chat" {
					provider, model = ProviderDashScope, "cross-loss-fixture"
				}
				cfg := DefaultOpenAICompatConfig(provider, "fixture-key")
				cfg.Model, cfg.BaseURL = model, server.URL
				cfg.EnableWebSearch = surface == "meta_responses"
				raw, err := NewOpenAICompatClient(cfg)
				if err != nil {
					t.Fatal(err)
				}
				meter := broker.NewMeter(broker.MeterConfig{})
				bc := meter.ConfigFor(broker.ProviderCreds{Provider: string(provider), Model: model})
				receipts := broker.NewRingSink(4)
				bc.Sink = receipts
				client, err := broker.Wrap(raw, bc)
				if err != nil {
					t.Fatal(err)
				}
				var text string
				operation := usageOpChat
				if surface == "grounded_search" {
					operation = usageOpSearch
					var result *types.GroundedWebSearchResult
					result, err = client.(types.GroundedWebSearcher).GroundedWebSearch(ctx, "q")
					if result != nil {
						text = result.Text
					}
				} else {
					text, err = client.CompleteWithSystem(ctx, "sys", "q")
				}
				wantRequests, reports, tokens, status := 1, 1, 1000, broker.UsageReported
				switch kind {
				case "recover":
					wantRequests, status = 2, broker.UsagePartial
				case "cancel":
					reports, tokens, status = 0, 0, broker.UsageUnreported
				case "exhausted":
					wantRequests, reports, tokens, status = llmMaxRetries()+1, 0, 0, broker.UsageUnreported
				}
				if kind == "clean" || kind == "recover" {
					if err != nil || text != "answer" {
						t.Fatalf("completion changed: text=%q err=%v", text, err)
					}
				} else if err == nil {
					t.Fatal("expected terminal error")
				}
				if kind == "cancel" && !errors.Is(err, context.Canceled) {
					t.Errorf("cancellation identity: %v", err)
				}
				if kind == "exhausted" && !strings.Contains(err.Error(), "attempts") && !strings.Contains(err.Error(), "max retries exceeded") {
					t.Errorf("retry bound error: %v", err)
				}
				if seen.Load() != int32(wantRequests) {
					t.Errorf("endpoint received %d requests, want %d", seen.Load(), wantRequests)
				}
				for _, o := range []*anthropicAttemptObserver{outer, inner} {
					o.mu.Lock()
					observed, unknown := append([]usage.Report(nil), o.reports...), o.unknown
					o.mu.Unlock()
					if len(observed) != reports || unknown != (kind != "clean") {
						t.Errorf("reports=%d unknown=%v want %d/%v", len(observed), unknown, reports, kind != "clean")
					}
					for _, report := range observed {
						if report.Model != model || report.Provider != string(provider) || report.Operation != operation || report.Usage.InputTokens != tokens || report.Usage.OutputTokens != tokens {
							t.Errorf("usage attribution/counts changed: %+v", report)
						}
					}
				}
				if got := tracker.Stats().TotalProject.Total; got != int64(2*tokens) {
					t.Errorf("known tokens=%d want=%d", got, 2*tokens)
				}
				got := receipts.Receipts()
				if len(got) != 1 || got[0].UsageStatus != status || got[0].Actual.InputTokens != int64(tokens) || got[0].Actual.OutputTokens != int64(tokens) || got[0].UsageConflict != "" {
					t.Errorf("receipt uncertainty/counts: %+v", got)
				}
			})
		}
	}
}
