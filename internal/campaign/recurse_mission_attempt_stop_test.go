package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"
)

func attemptStopMission(t *testing.T) (*RecurseMission, context.Context) {
	t.Helper()
	root := t.TempDir()
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	m := &RecurseMission{Text: "Stop paid attempts on uncertainty", Limits: config.DefaultMissionConfig(),
		QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
		Deliberator:       &deliberation.Service{Workspace: root, Session: "attempt-stop", Kernel: kernel}}
	ctx, cancel, err := m.begin(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	return m, ctx
}

// Every failed fixture consumes the entire request before losing its response.
// This establishes ambiguous spend, not that a real provider billed it.
func TestMissionUnknownStopsProviderAttempts(t *testing.T) {
	for _, surface := range []string{"chat", "meta_responses", "grounded_search", "anthropic", "openai", "compat_stream", "openai_stream", "typesafe"} {
		for _, pooled := range []bool{false, true} {
			if pooled && surface == "typesafe" {
				continue
			} // TypeSafe has no credential-pool adapter.
			for _, mode := range []string{"clean", "lost", "ordinary_lost"} {
				lost := mode != "clean"
				t.Run(fmt.Sprintf("%s/pool=%v/%s", surface, pooled, mode), func(t *testing.T) {
					m, ctx := attemptStopMission(t)
					if mode == "ordinary_lost" {
						ctx = t.Context()
					}
					model := "attempt-stop-" + surface
					if surface == "meta_responses" || surface == "grounded_search" {
						model += "-contributor"
					}
					usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
					var requests atomic.Int32
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						if err != nil || len(body) == 0 {
							t.Errorf("request not consumed: bytes=%d error=%v", len(body), err)
						}
						if strings.HasSuffix(r.URL.Path, "/count_tokens") {
							_, _ = io.WriteString(w, `{"input_tokens":4}`)
							return
						}
						if requests.Add(1) == 1 && lost {
							conn, _, err := w.(http.Hijacker).Hijack()
							if err != nil {
								t.Errorf("hijack: %v", err)
								return
							}
							_ = conn.Close()
							return
						}
						switch surface {
						case "anthropic":
							_, _ = io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":{"input_tokens":1000,"output_tokens":1000}}`)
						case "typesafe":
							_, _ = fmt.Fprintf(w, `{"model":%q,"usage":{"input_tokens":1000,"output_tokens":1000},"answers":{"ready":{"type":"noul","noul":0.9}}}`, model)
						case "compat_stream":
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":1000}}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
						case "openai_stream":
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r1\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1000,\"output_tokens\":1000}}}\n\n")
						case "chat":
							_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":1000}}`)
						default:
							_, _ = io.WriteString(w, `{"id":"r1","status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"answer","annotations":[]}]}],"usage":{"input_tokens":1000,"output_tokens":1000}}`)
						}
					}))
					defer srv.Close()
					var call func(context.Context) error
					if surface == "typesafe" {
						client, err := typesafe.NewClient(typesafe.Options{Model: model, APIKey: "fixture-key", BaseURL: srv.URL})
						if err != nil {
							t.Fatal(err)
						}
						call = func(ctx context.Context) error {
							_, err := client.Ask(ctx, "synthetic", map[string]typesafe.Question{"ready": typesafe.Noul{Instructions: "Synthetic?"}})
							return err
						}
					} else {
						provider := perception.ProviderDashScope
						switch surface {
						case "anthropic":
							provider = perception.ProviderAnthropic
						case "openai", "openai_stream":
							provider = perception.ProviderOpenAI
						case "meta_responses", "grounded_search":
							provider = perception.ProviderMeta
						}
						enabled := surface == "meta_responses"
						providerConfig := &perception.ProviderConfig{Provider: provider, APIKey: "fixture-key", Model: model, BaseURL: srv.URL, Meta: &config.MetaProviderConfig{EnableWebSearch: &enabled}}
						if pooled {
							providerConfig.APIKeys = []config.ResolvedProviderAPIKey{{Name: "alpha", Key: "synthetic-alpha-" + t.Name()}, {Name: "beta", Key: "synthetic-beta-" + t.Name()}}
						}
						client, err := perception.NewClientFromConfig(providerConfig)
						if err != nil {
							t.Fatal(err)
						}
						call = func(ctx context.Context) error {
							if surface == "grounded_search" {
								_, err := client.(types.GroundedWebSearcher).GroundedWebSearch(ctx, "synthetic")
								return err
							}
							if strings.HasSuffix(surface, "_stream") {
								chunks, done := client.CompleteWithStreaming(ctx, "system", "synthetic", false)
								for range chunks {
								}
								result := <-done
								return result.Err
							}
							_, err := client.CompleteWithSystem(ctx, "system", "synthetic")
							return err
						}
					}
					err := call(ctx)
					if mode == "ordinary_lost" {
						// TypeSafe deliberately does not retry a lost HTTP response;
						// its next ordinary call remains allowed. Other adapters retry.
						if surface == "typesafe" {
							if err == nil || requests.Load() != 1 {
								t.Fatalf("ordinary TypeSafe lost-response behavior changed: %v/%d", err, requests.Load())
							}
							err = call(ctx)
						}
						if err != nil || requests.Load() != 2 || ctx.Err() != nil {
							t.Fatalf("ordinary retry behavior changed: %v/%d", err, requests.Load())
						}
						return
					}
					if lost {
						if err == nil || ctx.Err() == nil || requests.Load() != 1 {
							t.Fatalf("uncertain attempt retried or not canceled: err=%v ctx=%v requests=%d", err, ctx.Err(), requests.Load())
						}
						// Detaching cancellation must not erase the monotone admission state.
						second, cancel := context.WithTimeout(context.WithoutCancel(ctx), 200*time.Millisecond)
						defer cancel()
						if err := call(second); err == nil || requests.Load() != 1 {
							t.Fatalf("closed scope dispatched again: err=%v requests=%d", err, requests.Load())
						}
						_, unknown, _ := m.meter.snapshot()
						if !unknown {
							t.Fatal("missing bill erased")
						}
						if err := m.end(ctx, err); err == nil {
							t.Fatal("uncertain completion reported success")
						}
						var saved MissionState
						data, err := os.ReadFile(m.path)
						if err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal(data, &saved); err != nil || !saved.UnknownSpend || saved.Status != "/unknown_spend" {
							t.Fatalf("durable stop=%+v err=%v", saved, err)
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if err := call(ctx); err != nil || requests.Load() != 2 || ctx.Err() != nil {
							t.Fatalf("clean work refused: err=%v ctx=%v requests=%d", err, ctx.Err(), requests.Load())
						}
						spend, unknown, reports := m.meter.snapshot()
						if spend != 4000 || reports != 2 || unknown {
							t.Fatalf("known counts changed: %d/%v/%d", spend, unknown, reports)
						}
					}
				})
			}
		}
	}
}

func TestMissionConcurrentUnknownPreservesKnownUsageAndIsolation(t *testing.T) {
	m, ctx := attemptStopMission(t)
	other, otherCtx := attemptStopMission(t)
	const model = "attempt-stop-concurrent-counts"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	report := usage.Report{Model: model, Provider: "meta", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 1000, OutputTokens: 1000, Known: types.UsageTokens}}
	usage.RecordFromContext(ctx, report)
	var workers sync.WaitGroup
	start := make(chan struct{})
	for range 16 {
		workers.Go(func() { <-start; usage.RecordFromContext(ctx, report) })
		workers.Go(func() { <-start; usage.NoteUnknownUsage(ctx) })
	}
	close(start)
	workers.Wait()
	// Counts returned by already admitted work remain evidence after closure.
	report.Provider = "anthropic"
	usage.RecordFromContext(ctx, report)
	spend, unknown, reports := m.meter.snapshot()
	if spend != 36000 || !unknown || reports != 18 {
		t.Fatalf("known reports lost: %d/%v/%d", spend, unknown, reports)
	}
	if ctx.Err() == nil || usage.BeforeAttempt(context.WithoutCancel(ctx)) == nil {
		t.Fatal("scope reopened")
	}
	usage.RecordFromContext(otherCtx, report)
	spend, unknown, reports = other.meter.snapshot()
	if otherCtx.Err() != nil || usage.BeforeAttempt(otherCtx) != nil || spend != 2000 || unknown || reports != 1 {
		t.Fatalf("other mission affected: %v/%d/%v/%d", otherCtx.Err(), spend, unknown, reports)
	}
}

func TestMissionTypeSafeUnknownStopsRetry(t *testing.T) {
	for _, kind := range []string{"retryable", "unpriced"} {
		t.Run(kind, func(t *testing.T) {
			m, ctx := attemptStopMission(t)
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if requests.Add(1) == 1 && kind == "retryable" {
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, `{"error":{"type":"rate_limit_error","message":"fixture"}}`)
					return
				}
				_, _ = io.WriteString(w, `{"model":"unpriced-attempt-fixture","usage":{"input_tokens":1000,"output_tokens":1000},"answers":{"ready":{"type":"noul","noul":0.9}}}`)
			}))
			defer srv.Close()
			client, err := typesafe.NewClient(typesafe.Options{Model: "jev-latest", BaseURL: srv.URL, APIKey: "fixture-key", Retry: &typesafe.RetryPolicy{Max: 1, Base: time.Millisecond, Cap: time.Millisecond, MaxWait: time.Second}})
			if err != nil {
				t.Fatal(err)
			}
			_, firstErr := client.Ask(ctx, "synthetic", map[string]typesafe.Question{"ready": typesafe.Noul{Instructions: "Synthetic?"}})
			if kind == "retryable" && firstErr == nil {
				t.Fatal("uncertain status retried")
			}
			_, err = client.Ask(context.WithoutCancel(ctx), "synthetic", map[string]typesafe.Question{"ready": typesafe.Noul{Instructions: "Synthetic?"}})
			if err == nil || ctx.Err() == nil || requests.Load() != 1 {
				t.Fatalf("unpriced/unknown work continued: err=%v ctx=%v requests=%d", err, ctx.Err(), requests.Load())
			}
			_, unknown, _ := m.meter.snapshot()
			if !unknown {
				t.Fatal("unknown bill erased")
			}
		})
	}
}
