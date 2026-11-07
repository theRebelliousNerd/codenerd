package campaign

import (
	"context"
	"encoding/json"
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
	"codenerd/internal/usage"
)

func TestMissionAdmissionAnthropicLostResponse(t *testing.T) {
	t.Parallel()
	const model = "mission-anthropic-lost-response-fixture"
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
			mission := &RecurseMission{Text: "Preserve uncertainty after a lost response", Limits: limits,
				QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
				Deliberator:       &deliberation.Service{Workspace: root, Session: "lost-response", Kernel: kernel}}
			ctx, cancel, err := mission.begin(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			var requests, counts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || len(body) == 0 {
					t.Errorf("incomplete request: bytes=%d err=%v", len(body), err)
				}
				if strings.HasSuffix(r.URL.Path, "/count_tokens") {
					counts.Add(1)
					_, _ = io.WriteString(w, `{"input_tokens":4}`)
					return
				}
				if requests.Add(1) == 1 && kind == "recover" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					_ = conn.Close() // request received; no response headers sent
					return
				}
				counts := `{"input_tokens":1000,"output_tokens":1000}`
				if kind == "known_over_budget" {
					counts = `{"input_tokens":2000,"output_tokens":2000}`
				}
				_, _ = io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":`+counts+`}`)
			}))
			defer server.Close()
			// The production factory's ordinary single-key path, without a pool.
			client, err := perception.NewClientFromConfig(&perception.ProviderConfig{Provider: perception.ProviderAnthropic, APIKey: "fixture-key", Model: model, BaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			_, _, before := mission.meter.snapshot()
			answer, err := client.CompleteWithTools(ctx, "system", "request", nil)
			if requests.Load() != 1 || counts.Load() == 0 {
				t.Fatalf("dispatch/count calls=%d/%d", requests.Load(), counts.Load())
			}
			if kind == "recover" {
				if err == nil || ctx.Err() == nil || answer != nil {
					t.Fatalf("uncertain retry admitted: %+v/%v/%v", answer, err, ctx.Err())
				}
			} else if err != nil || answer == nil || answer.Text != "answer" {
				t.Fatalf("completion: %+v/%v", answer, err)
			}
			mission.meter.phaseEnded(before)
			spend, unknown, reports := mission.meter.snapshot()
			wantSpend, wantReports := int64(2000), 1
			if kind == "recover" {
				wantSpend, wantReports = 0, 0
			}
			if kind == "known_over_budget" {
				wantSpend = 4000
			}
			if spend != wantSpend || unknown != (kind == "recover") || reports != wantReports {
				t.Errorf("meter=%d/%v/%d want %d/%v/1", spend, unknown, reports, wantSpend, kind == "recover")
			}
			_, err = mission.admit()
			wantStatus := "/unknown_spend"
			if kind == "known_over_budget" {
				wantStatus = "/spend"
			}
			if kind == "clean" {
				if err != nil {
					t.Fatalf("clean call refused: %v", err)
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
			if err := json.Unmarshal(data, &saved); err != nil || saved.SpendMicroUSD != wantSpend || saved.UnknownSpend != (kind == "recover") || saved.UsageReports != wantReports || saved.Cycles != 0 || saved.Pending != nil {
				t.Fatalf("durable accounting: %+v err=%v", saved, err)
			}
			if kind != "clean" && saved.Status != wantStatus {
				t.Errorf("durable stop=%s want=%s", saved.Status, wantStatus)
			}
		})
	}
}
