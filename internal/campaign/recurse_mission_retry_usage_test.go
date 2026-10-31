package campaign

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/perception"
	"codenerd/internal/usage"
)

func TestMissionAdmissionAccountsForUsageInsideProviderRetries(t *testing.T) {
	const model = "mission-retry-usage-fixture"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	for _, missing := range []bool{false, true} {
		name := "reported_retry"
		if missing {
			name = "missing_retry"
		}
		t.Run(name, func(t *testing.T) {
			m, ctx := missionPolicyFixture(t)
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if calls.Add(1) == 1 {
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","reasoning_content":"reasoning only"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":1000}}`))
					return
				}
				reply := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "recovered answer"}, "finish_reason": "stop"}}}
				if !missing {
					reply["usage"] = map[string]int{"prompt_tokens": 500, "completion_tokens": 500}
				}
				_ = json.NewEncoder(w).Encode(reply)
			}))
			defer srv.Close()
			cfg := perception.DefaultOpenAICompatConfig(perception.ProviderDashScope, "fixture-key")
			cfg.BaseURL, cfg.Model = srv.URL, model
			raw, err := perception.NewOpenAICompatClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			meter := broker.NewMeter(broker.MeterConfig{})
			client, err := broker.Wrap(raw, meter.ConfigFor(broker.ProviderCreds{Provider: string(perception.ProviderDashScope), Model: model}))
			if err != nil {
				t.Fatal(err)
			}
			_, _, before := m.meter.snapshot()
			answer, err := client.CompleteWithSystem(ctx, "fixture system", "fixture request")
			if err != nil || answer != "recovered answer" || calls.Load() != 2 {
				t.Fatalf("answer=%q err=%v calls=%d", answer, err, calls.Load())
			}
			m.meter.phaseEnded(before)
			wantSpend, wantReports := int64(3000), 2
			if missing {
				wantSpend, wantReports = 2000, 1
			}
			spend, unknown, reports := m.meter.snapshot()
			if spend != wantSpend || reports != wantReports || unknown != missing {
				t.Errorf("spend=%d reports=%d unknown=%v; want %d/%d/%v", spend, reports, unknown, wantSpend, wantReports, missing)
			}
			_, err = m.admit()
			if missing {
				if err != errMissionStopped || m.State.Status != "/unknown_spend" || len(m.State.StopFacts) != 1 {
					t.Fatalf("unaccounted retry admitted: err=%v status=%s facts=%v", err, m.State.Status, m.State.StopFacts)
				}
			} else if err != nil {
				t.Fatalf("fully accounted retry refused: %v", err)
			}
			if err := m.end(ctx, err); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			var saved MissionState
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.UnknownSpend != missing || saved.SpendMicroUSD != wantSpend || saved.UsageReports != wantReports || saved.Cycles != 0 || saved.Pending != nil {
				t.Fatalf("wrong durable accounting: %+v", saved)
			}
		})
	}
}
