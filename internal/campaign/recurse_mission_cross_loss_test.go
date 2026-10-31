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
	"sync/atomic"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

func TestMissionAdmissionCrossProviderLostResponse(t *testing.T) {
	for _, surface := range []string{"chat", "meta_responses", "grounded_search"} {
		for _, kind := range []string{"clean", "recover", "known_over_budget"} {
			t.Run(surface+"/"+kind, func(t *testing.T) {
				model := "mission-cross-loss-" + surface
				provider := perception.ProviderDashScope
				if surface != "chat" {
					provider = perception.ProviderMeta
					model += "-contributor"
				}
				usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
				root := t.TempDir()
				kernel, err := core.NewRealKernelWithWorkspace(root)
				if err != nil {
					t.Fatal(err)
				}
				limits := config.DefaultMissionConfig()
				limits.MaxSpendMicroUSD = 3000
				mission := &RecurseMission{Text: "Retain unknown transport spend", Limits: limits,
					QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
					Deliberator:       &deliberation.Service{Workspace: root, Session: "cross-loss", Kernel: kernel}}
				ctx, cancel, err := mission.begin(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil || len(body) == 0 {
						t.Errorf("request not consumed: bytes=%d err=%v", len(body), err)
					}
					wantPath := "/responses"
					if surface == "chat" {
						wantPath = "/chat/completions"
					}
					if !strings.HasSuffix(r.URL.Path, wantPath) {
						t.Errorf("route=%s want suffix %s", r.URL.Path, wantPath)
					}
					if surface != "chat" {
						var request struct {
							Tools []struct {
								Type string `json:"type"`
							} `json:"tools"`
						}
						if err := json.Unmarshal(body, &request); err != nil {
							t.Errorf("request JSON: %v", err)
						}
						search := false
						for _, tool := range request.Tools {
							search = search || tool.Type == "web_search"
						}
						if !search {
							t.Error("grounded request omitted web_search")
						}
					}
					if requests.Add(1) == 1 && kind == "recover" {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Errorf("hijack: %v", err)
							return
						}
						_ = conn.Close()
						return
					}
					tokens := 1000
					if kind == "known_over_budget" {
						tokens = 2000
					}
					if surface == "chat" {
						_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":%d,"completion_tokens":%d}}`, tokens, tokens)
					} else {
						_, _ = fmt.Fprintf(w, `{"id":"r1","status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"answer","annotations":[]}]}],"usage":{"input_tokens":%d,"output_tokens":%d}}`, tokens, tokens)
					}
				}))
				defer server.Close()
				enabled := surface == "meta_responses"
				client, err := perception.NewClientFromConfig(&perception.ProviderConfig{Provider: provider, APIKey: "fixture-key", Model: model, BaseURL: server.URL, Meta: &config.MetaProviderConfig{EnableWebSearch: &enabled}})
				if err != nil {
					t.Fatal(err)
				}
				_, _, before := mission.meter.snapshot()
				var answer string
				if surface == "grounded_search" {
					var result *types.GroundedWebSearchResult
					result, err = client.(types.GroundedWebSearcher).GroundedWebSearch(ctx, "q")
					if result != nil {
						answer = result.Text
					}
				} else {
					answer, err = client.CompleteWithSystem(ctx, "system", "request")
				}
				wantRequests := int32(1)
				if kind == "recover" {
					wantRequests = 2
				}
				if err != nil || answer != "answer" || requests.Load() != wantRequests {
					t.Fatalf("completion=%q err=%v requests=%d", answer, err, requests.Load())
				}
				mission.meter.phaseEnded(before)
				spend, unknown, reports := mission.meter.snapshot()
				wantSpend := int64(2000)
				if kind == "known_over_budget" {
					wantSpend = 4000
				}
				if spend != wantSpend || unknown != (kind == "recover") || reports != 1 {
					t.Errorf("meter=%d/%v/%d want %d/%v/1", spend, unknown, reports, wantSpend, kind == "recover")
				}
				_, err = mission.admit()
				wantStatus := "/unknown_spend"
				if kind == "known_over_budget" {
					wantStatus = "/spend"
				}
				if kind == "clean" {
					if err != nil {
						t.Fatalf("clean response refused: %v", err)
					}
				} else if err != errMissionStopped || mission.State.Status != wantStatus {
					t.Fatalf("unaccounted work admitted: err=%v status=%s want=%s", err, mission.State.Status, wantStatus)
				}
				if err := mission.end(ctx, err); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(mission.path)
				if err != nil {
					t.Fatal(err)
				}
				var saved MissionState
				if err := json.Unmarshal(data, &saved); err != nil || saved.SpendMicroUSD != wantSpend || saved.UnknownSpend != (kind == "recover") || saved.UsageReports != 1 || saved.Cycles != 0 || saved.Pending != nil {
					t.Fatalf("durable accounting=%+v err=%v", saved, err)
				}
				if kind != "clean" && saved.Status != wantStatus {
					t.Errorf("durable stop=%s want=%s", saved.Status, wantStatus)
				}
			})
		}
	}
}
