package system

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"
)

type screeningTransport func(*http.Request) (*http.Response, error)

func (f screeningTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The actual automatic-maintenance adapter starts with an untracked background
// context. A real TypeSafe client must deliver exactly one report to the
// Cortex tracker; before the fix that context had no accounting destination.
func TestAutomaticEvolutionScreeningOwnsUsageContext(t *testing.T) {
	t.Setenv("CODENERD_PROMPT_EVOLUTION", "1")
	tracker, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer tracker.Close()
	cfg := pe.DefaultEvolverConfig()
	cfg.MinFailuresForEvolution = 1
	cfg.EnableStrategies = false
	evolver, err := pe.NewPromptEvolver(t.TempDir(), stubJudgeLLM{}, cfg, nil, nerdconfig.DefaultJITConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer evolver.Close()
	calls := 0
	client, err := typesafe.NewClient(typesafe.Options{
		Model: "screen-fixture-alias", BaseURL: "https://screen.example.test", APIKey: "fixture", Retry: &typesafe.RetryPolicy{},
		HTTPClient: &http.Client{Transport: screeningTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			if _, ok := req.Context().Deadline(); !ok {
				t.Error("automatic call lacks owned cycle deadline")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"model":"screen-fixture-version","usage":{"input_tokens":17,"output_tokens":2},"answers":{"shares_one_cause":{"type":"noul","noul":0.1}}}`))}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	off := false
	evolver.SetJev(client, &nerdconfig.TypeSafeConfig{Model: client.Model(), TaskVerdicts: &off, ProblemTypes: &off})
	if err := evolver.RecordExecution(&pe.ExecutionRecord{
		TaskID: "maintenance-screen", SessionID: "maintenance", ShardType: "/coder", Provider: "fixture-provider", Model: "fixture-worker",
		TaskRequest: "repair a boundary", ProblemType: "debugging", Timestamp: time.Unix(1700000000, 0),
		Verdict: &pe.JudgeVerdict{TaskID: "maintenance-screen", Verdict: "FAIL", Category: pe.CategoryLogicError, Explanation: "missed a boundary"},
	}); err != nil {
		t.Fatal(err)
	}
	c := &Cortex{PromptEvolver: evolver, UsageTracker: tracker}
	c.runEvolutionCycle(context.Background())
	if calls != 1 || tracker.Stats().TotalProject.Total != 19 {
		t.Fatalf("screens=%d aggregate=%+v", calls, tracker.Stats().TotalProject)
	}
	stats := evolver.GetStats()
	found := false
	for _, s := range stats.Spend {
		if s.Category == pe.SpendScreen {
			found = true
			if s.Calls != 1 {
				t.Fatalf("screen spend=%+v", s)
			}
		}
	}
	if !found {
		t.Fatal("automatic screen missing from adaptation accounting")
	}
}
