package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"
)

type typesafePriceTransport func(*http.Request) (*http.Response, error)

func (f typesafePriceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// The production TypeSafe client reports the answered version to the real
// mission observer. Every request is answered in memory, without network I/O.
func TestMissionTypeSafeVersionedPriceAdmission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, model, status string
		input               int
		unknown             bool
	}{
		{name: "below_budget", model: "jev-1.14.0", input: 1000},
		{name: "over_budget", model: "jev-1.14.0", input: 1000000, status: "/spend"},
		{name: "alias_identity", model: "jev-latest", input: 1000, status: "/unknown_spend", unknown: true},
		{name: "future_version", model: "jev-1.15.0", input: 1000, status: "/unknown_spend", unknown: true},
		{name: "undocumented_suffix", model: "jev-1.14.0-preview", input: 1000, status: "/unknown_spend", unknown: true},
		{name: "missing_response_identity", input: 1000, status: "/unknown_spend", unknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			kernel, err := core.NewRealKernelWithWorkspace(root)
			if err != nil {
				t.Fatal(err)
			}
			limits := config.DefaultMissionConfig()
			limits.MaxSpendMicroUSD = 3000
			m := &RecurseMission{Text: "Account for documented TypeSafe response versions", Limits: limits,
				QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
				Deliberator:       &deliberation.Service{Workspace: root, Session: "typesafe-price", Kernel: kernel}}
			ctx, cancel, err := m.begin(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			calls := 0
			client, err := typesafe.NewClient(typesafe.Options{Model: "jev-latest", APIKey: "synthetic-offline", BaseURL: "https://provider.invalid",
				HTTPClient: &http.Client{Transport: typesafePriceTransport(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.URL.String() != "https://provider.invalid/v1/systemone" {
						return nil, fmt.Errorf("unexpected synthetic request")
					}
					body := fmt.Sprintf(`{"model":%q,"usage":{"input_tokens":%d,"output_tokens":100},"answers":{"ready":{"type":"noul","noul":0.9}}}`, tc.model, tc.input)
					return &http.Response{StatusCode: 200, Header: make(http.Header), Request: req, Body: io.NopCloser(strings.NewReader(body))}, nil
				})}})
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Ask(ctx, "synthetic state", map[string]typesafe.Question{"ready": typesafe.Noul{Instructions: "Is this synthetic?"}})
			if err != nil || response.Usage.InputTokens != tc.input || response.Usage.OutputTokens != 100 {
				t.Fatalf("reported counts changed: response=%+v err=%v", response, err)
			}
			spend, unknown, reports := m.meter.snapshot()
			if calls != 1 || reports != 1 || unknown != tc.unknown {
				t.Fatalf("calls=%d reports=%d unknown=%v", calls, reports, unknown)
			}
			if !tc.unknown {
				want := int64(tc.input) * 42 / 1000
				if spend < want || spend > want+1 || len(m.meter.prices()) != 1 {
					t.Fatalf("input-only versioned spend=%d want=%d(+rounding) prices=%v", spend, want, m.meter.prices())
				}
				_, version, known := usage.LookupPriceVersion(tc.model)
				if !known || m.meter.prices()[version] != spend {
					t.Fatalf("price identity lost: %q %v", version, m.meter.prices())
				}
			}
			_, err = m.admit()
			if tc.status == "" {
				if err != nil {
					t.Fatalf("known below-budget usage refused: %v", err)
				}
				if err := m.save(); err != nil {
					t.Fatal(err)
				}
			} else if err != errMissionStopped || m.State.Status != tc.status {
				t.Fatalf("admission=%v status=%q want=%q", err, m.State.Status, tc.status)
			}
			data, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			var saved MissionState
			if err := json.Unmarshal(data, &saved); err != nil || saved.UnknownSpend != tc.unknown || saved.SpendMicroUSD != spend || saved.UsageReports != 1 {
				t.Fatalf("durable accounting changed: %v %+v", err, saved)
			}
		})
	}
}
