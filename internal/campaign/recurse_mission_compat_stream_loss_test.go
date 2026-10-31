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

func TestMissionAdmissionCompatStreamingLostResponse(t *testing.T) {
	const model = "mission-compat-stream-loss-fixture"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	for _, kind := range []string{"clean", "recover", "known_over_budget"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			kernel, err := core.NewRealKernelWithWorkspace(root)
			if err != nil {
				t.Fatal(err)
			}
			limits := config.DefaultMissionConfig()
			limits.MaxSpendMicroUSD = 3000
			mission := &RecurseMission{Text: "Preserve unknown streaming setup spend", Limits: limits,
				QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
				Deliberator:       &deliberation.Service{Workspace: root, Session: "stream-loss", Kernel: kernel}}
			ctx, cancel, err := mission.begin(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			var seen atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || len(body) == 0 {
					t.Errorf("inference body not received: bytes=%d err=%v", len(body), err)
				}
				var request struct {
					Stream  bool `json:"stream"`
					Options struct {
						IncludeUsage bool `json:"include_usage"`
					} `json:"stream_options"`
				}
				if err := json.Unmarshal(body, &request); err != nil || !request.Stream || !request.Options.IncludeUsage || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
					t.Errorf("stream route/request changed: %s %+v err=%v", r.URL.Path, request, err)
				}
				if seen.Add(1) == 1 && kind == "recover" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					_ = conn.Close() // complete inference body received; no headers returned
					return
				}
				tokens := 1000
				if kind == "known_over_budget" {
					tokens = 2000
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"}}]}\n\n")
				_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
				_, _ = fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n", tokens, tokens, 2*tokens)
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
			}))
			defer server.Close()
			client, err := perception.NewClientFromConfig(&perception.ProviderConfig{Provider: perception.ProviderDashScope, APIKey: "fixture-key", Model: model, BaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			_, _, before := mission.meter.snapshot()
			content, done := client.CompleteWithStreaming(ctx, "system", "request", false)
			text, terminal := types.CollectStream(context.Background(), content, done, nil)
			requests := int32(1)
			if kind == "recover" {
				requests = 2
			}
			if terminal.Err != nil || terminal.Outcome() != types.StreamComplete || text != "answer" || seen.Load() != requests {
				t.Fatalf("stream behavior changed: %q / %+v / %d requests", text, terminal, seen.Load())
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
					t.Fatalf("clean stream refused: %v", err)
				}
			} else if err != errMissionStopped || mission.State.Status != wantStatus {
				t.Fatalf("unaccounted stream admitted: err=%v status=%s want=%s", err, mission.State.Status, wantStatus)
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
