package campaign

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/perception"
	"codenerd/internal/usage"
)

func TestMissionAdmissionAnthropicIncompleteAttempts(t *testing.T) {
	const model = "mission-anthropic-attempt-fixture"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	for _, kind := range []string{"clean", "missing", "over_budget"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			k, err := core.NewRealKernelWithWorkspace(root)
			if err != nil {
				t.Fatal(err)
			}
			limits := config.DefaultMissionConfig()
			limits.MaxSpendMicroUSD = 3000
			m := &RecurseMission{Text: "Bound the fixture spend", Limits: limits,
				QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
				Deliberator:       &deliberation.Service{Workspace: root, Session: "anthropic-accounting", Kernel: k}}
			ctx, cancel, err := m.begin(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := requests.Add(1)
				if n == 1 && kind != "clean" {
					counts := `{}`
					if kind == "over_budget" {
						counts = `{"input_tokens":1000,"output_tokens":1000}`
					}
					_, _ = w.Write([]byte(`{"type":"message","content":[],"stop_reason":"end_turn","usage":` + counts + `}`))
					return
				}
				_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":{"input_tokens":1000,"output_tokens":1000}}`))
			}))
			defer server.Close()
			raw := perception.NewAnthropicClientWithConfig(perception.AnthropicConfig{APIKey: "fixture-key", BaseURL: server.URL, Model: model})
			meter := broker.NewMeter(broker.MeterConfig{})
			client, err := broker.Wrap(raw, meter.ConfigFor(broker.ProviderCreds{Provider: "anthropic", Model: model}))
			if err != nil {
				t.Fatal(err)
			}
			_, _, before := m.meter.snapshot()
			answer, err := client.CompleteWithTools(ctx, "system", "request", nil)
			wantRequests := int32(2)
			if kind == "clean" || kind == "missing" {
				wantRequests = 1
			}
			if requests.Load() != wantRequests {
				t.Fatalf("requests=%d want=%d", requests.Load(), wantRequests)
			}
			if kind == "missing" {
				if err == nil || ctx.Err() == nil || answer != nil {
					t.Fatalf("unknown incomplete attempt retried: %+v/%v", answer, err)
				}
			} else if err != nil || answer == nil || answer.Text != "answer" {
				t.Fatalf("completion: %+v/%v", answer, err)
			}
			m.meter.phaseEnded(before)
			spend, unknown, reports := m.meter.snapshot()
			wantSpend, wantReports := int64(2000), 1
			if kind == "missing" {
				wantSpend, wantReports = 0, 0
			}
			if kind == "over_budget" {
				wantSpend, wantReports = 4000, 2
			}
			if spend != wantSpend || reports != wantReports || unknown != (kind == "missing") {
				t.Errorf("meter = %d/%d/%v want %d/%d/%v", spend, reports, unknown, wantSpend, wantReports, kind == "missing")
			}
			_, err = m.admit()
			wantStatus := "/unknown_spend"
			if kind == "over_budget" {
				wantStatus = "/spend"
			}
			if kind == "clean" {
				if err != nil {
					t.Fatalf("clean call refused: %v", err)
				}
			} else if err != errMissionStopped || m.State.Status != wantStatus {
				t.Fatalf("unbounded call admitted: err=%v status=%s want=%s", err, m.State.Status, wantStatus)
			}
			if err := m.end(ctx, err); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			var saved MissionState
			if err := json.Unmarshal(data, &saved); err != nil || saved.UnknownSpend != (kind == "missing") || saved.SpendMicroUSD != wantSpend || saved.UsageReports != wantReports || saved.Cycles != 0 || saved.Pending != nil {
				t.Fatalf("durable accounting: %+v / %v", saved, err)
			}
			if kind != "clean" && saved.Status != wantStatus {
				t.Errorf("durable stop=%s want=%s", saved.Status, wantStatus)
			}
		})
	}
}
