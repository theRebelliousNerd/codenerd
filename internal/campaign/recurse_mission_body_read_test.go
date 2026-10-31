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

func TestMissionAdmissionStopsAfterAcceptedBodyReadRetry(t *testing.T) {
	const model = "mission-body-read-fixture"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	for _, truncated := range []bool{false, true} {
		name := "clean"
		if truncated {
			name = "truncated_then_success"
		}
		t.Run(name, func(t *testing.T) {
			m, ctx := missionPolicyFixture(t)
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 && truncated {
					// Returning fewer bytes than Content-Length closes this HTTP/1
					// connection and makes the client's body read fail after 200.
					w.Header().Set("Content-Length", "4096")
					w.Header().Set("Retry-After", "0")
					_, _ = w.Write([]byte(`{"choices":`))
					return
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":1000}}`))
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
			answer, err := client.CompleteWithTools(ctx, "system", "request", nil)
			wantRequests := int32(1)
			if truncated {
				wantRequests = 2
			}
			if err != nil || answer == nil || answer.Text != "answer" || requests.Load() != wantRequests {
				t.Fatalf("retry behavior changed: answer=%+v err=%v requests=%d", answer, err, requests.Load())
			}
			m.meter.phaseEnded(before)
			spend, unknown, reports := m.meter.snapshot()
			if spend != 2000 || reports != 1 || unknown != truncated {
				t.Errorf("spend=%d reports=%d unknown=%v; want 2000/1/%v", spend, reports, unknown, truncated)
			}
			_, err = m.admit()
			if truncated {
				if err != errMissionStopped || m.State.Status != "/unknown_spend" {
					t.Fatalf("unaccounted accepted attempt admitted: err=%v status=%s", err, m.State.Status)
				}
			} else if err != nil {
				t.Fatalf("clean response refused: %v", err)
			}
			if err := m.end(ctx, err); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			var saved MissionState
			if err := json.Unmarshal(data, &saved); err != nil || saved.UnknownSpend != truncated || saved.SpendMicroUSD != 2000 || saved.UsageReports != 1 || saved.Pending != nil {
				t.Fatalf("durable accounting lost: %+v err=%v", saved, err)
			}
		})
	}
}
