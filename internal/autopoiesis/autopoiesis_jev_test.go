package autopoiesis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/learnjudge"
	"codenerd/internal/typesafe"
)

// fakeJevServer answers every noul with the probability named for it (0.5
// otherwise), or fails every call with status when it is non-zero. It counts
// calls.
func fakeJevServer(t *testing.T, answers map[string]float64, status int) (*typesafe.Client, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"down"}`))
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		out := map[string]any{}
		for name := range req["questions"].(map[string]any) {
			p, ok := answers[name]
			if !ok {
				p = 0.5
			}
			out[name] = map[string]any{"type": "noul", "noul": p}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-test", "answers": out, "usage": map[string]any{"input_tokens": 100, "output_tokens": 4}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-test", BaseURL: srv.URL, APIKey: "k", Timeout: 2e9})
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls
}

// jevOrchestrator is an Orchestrator with the real kernel and policy and only
// the analyzers SuggestForRequest reads.
func jevOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	return &Orchestrator{
		complexity:  NewComplexityAnalyzer(nil),
		persistence: NewPersistenceAnalyzer(nil),
		evaluator:   NewQualityEvaluator(nil, nil),
		kernel:      k,
	}
}

func sectionOn() *internalconfig.TypeSafeConfig { return &internalconfig.TypeSafeConfig{} }

// The kernel's truth table for a campaign and a persistent agent, row by row,
// from Jev's judgments; the keyword heuristic's match on the same request does
// not count once Jev has judged it.
func TestSuggestForRequest_KernelTruthTable(t *testing.T) {
	// Each request matches the keyword heuristic for a campaign ("implement a
	// complete feature") so a Jev "no" visibly outranks it.
	const epic = "implement a complete feature for billing"
	cases := []struct {
		name         string
		answers      map[string]float64
		wantCampaign bool
		wantPersist  bool
	}{
		{"deliverables and stages", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.9, learnjudge.ScopeOrderedStages: 0.85, learnjudge.ScopeManyFiles: 0.1, learnjudge.ScopeContinuesAfter: 0.05}, true, false},
		{"deliverables and many files", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.8, learnjudge.ScopeOrderedStages: 0.2, learnjudge.ScopeManyFiles: 0.9, learnjudge.ScopeContinuesAfter: 0.05}, true, false},
		{"one result in stages across many files", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.2, learnjudge.ScopeOrderedStages: 0.88, learnjudge.ScopeManyFiles: 0.89, learnjudge.ScopeContinuesAfter: 0.05}, true, false},
		{"one change spread wide in one pass", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.04, learnjudge.ScopeOrderedStages: 0.36, learnjudge.ScopeManyFiles: 0.84, learnjudge.ScopeContinuesAfter: 0.04}, false, false},
		{"several small results", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.96, learnjudge.ScopeOrderedStages: 0.09, learnjudge.ScopeManyFiles: 0.11, learnjudge.ScopeContinuesAfter: 0.03}, false, false},
		{"deliverables unsettled", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.5, learnjudge.ScopeOrderedStages: 0.2, learnjudge.ScopeManyFiles: 0.9, learnjudge.ScopeContinuesAfter: 0.05}, false, false},
		{"deliverables alone", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.9, learnjudge.ScopeOrderedStages: 0.5, learnjudge.ScopeManyFiles: 0.2, learnjudge.ScopeContinuesAfter: 0.05}, false, false},
		{"continues after", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.1, learnjudge.ScopeOrderedStages: 0.1, learnjudge.ScopeManyFiles: 0.1, learnjudge.ScopeContinuesAfter: 0.92}, false, true},
		{"continues unsettled", map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.1, learnjudge.ScopeOrderedStages: 0.1, learnjudge.ScopeManyFiles: 0.1, learnjudge.ScopeContinuesAfter: 0.6}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := jevOrchestrator(t)
			c, calls := fakeJevServer(t, tc.answers, 0)
			o.SetJev(c, sectionOn())
			got := o.SuggestForRequest(context.Background(), epic, "")
			if calls.Load() != 1 {
				t.Fatalf("calls = %d, want one request", calls.Load())
			}
			if got.Campaign != tc.wantCampaign {
				t.Fatalf("campaign = %v (%s: %s), want %v", got.Campaign, got.CampaignSource, got.CampaignReason, tc.wantCampaign)
			}
			if got.Campaign && got.CampaignSource != "jev" {
				t.Fatalf("campaign source = %q, want jev", got.CampaignSource)
			}
			if (got.Persistent != nil) != tc.wantPersist {
				t.Fatalf("persistent = %+v, want %v", got.Persistent, tc.wantPersist)
			}
			if got.Persistent != nil && got.PersistentSource != "jev" {
				t.Fatalf("persistent source = %q, want jev", got.PersistentSource)
			}
		})
	}
}

// Jev failing, switched off, or unconfigured is no evidence, never a "no":
// the keyword heuristic decides, through the same kernel rule.
func TestSuggestForRequest_NoJudgmentFallsBackToTheHeuristic(t *testing.T) {
	off := false
	failing, failCalls := fakeJevServer(t, nil, http.StatusInternalServerError)
	unused, unusedCalls := fakeJevServer(t, map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.01}, 0)
	for _, tc := range []struct {
		name  string
		setup func(o *Orchestrator)
	}{
		{"failing", func(o *Orchestrator) { o.SetJev(failing, sectionOn()) }},
		{"switched off", func(o *Orchestrator) {
			o.SetJev(unused, &internalconfig.TypeSafeConfig{AutopoiesisChecks: &off})
		}},
		{"unconfigured", func(o *Orchestrator) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := jevOrchestrator(t)
			tc.setup(o)
			got := o.SuggestForRequest(context.Background(), "implement a complete feature for billing", "")
			if !got.Campaign || got.CampaignSource != "heuristic" || !strings.Contains(got.CampaignReason, "keyword heuristic") {
				t.Fatalf("campaign = %+v, want the heuristic's", got)
			}
			persist := o.SuggestForRequest(context.Background(), "notify me when the build breaks", "")
			if persist.Persistent == nil || persist.PersistentSource != "heuristic" {
				t.Fatalf("persistent = %+v, want the heuristic's", persist)
			}
			small := o.SuggestForRequest(context.Background(), "fix a typo in the readme", "")
			if small.Campaign || small.Persistent != nil {
				t.Fatalf("small request suggested %+v", small)
			}
		})
	}
	if failCalls.Load() == 0 {
		t.Fatal("the failing Jev was never asked")
	}
	if unusedCalls.Load() != 0 {
		t.Fatalf("autopoiesis_checks off still asked Jev %d time(s)", unusedCalls.Load())
	}
}

// Last request's judgments leave before the next one is judged: a request Jev
// fails on does not inherit the previous request's yes.
func TestSuggestForRequest_JudgmentsDoNotOutliveTheirRequest(t *testing.T) {
	o := jevOrchestrator(t)
	yes, _ := fakeJevServer(t, map[string]float64{learnjudge.ScopeSeveralDeliverables: 0.9, learnjudge.ScopeOrderedStages: 0.9}, 0)
	o.SetJev(yes, sectionOn())
	if !o.SuggestForRequest(context.Background(), "build the importer, then the exporter", "").Campaign {
		t.Fatal("first request: no campaign")
	}
	failing, _ := fakeJevServer(t, nil, http.StatusInternalServerError)
	o.SetJev(failing, sectionOn())
	if got := o.SuggestForRequest(context.Background(), "fix a typo in the readme", ""); got.Campaign {
		t.Fatalf("second request inherited a campaign: %+v", got)
	}
}

// The quality evaluator's truth table: Jev's yes is the issue, its no or
// unsettled reading is none even where the pattern would match, an exact rule
// runs regardless, and a failed Jev leaves the patterns in charge.
func TestOutputIssues_JevReplacesTheLanguagePatterns(t *testing.T) {
	feedback := &ExecutionFeedback{ToolName: "log_grep", Success: true,
		Output: "found 3 lines: ERROR failed to connect; error budget ok; timeout=30s"}
	issueTypes := func(issues []QualityIssue) map[IssueType]bool {
		out := map[IssueType]bool{}
		for _, i := range issues {
			out[i.Type] = true
		}
		return out
	}

	qe := NewQualityEvaluator(nil, nil)
	if !issueTypes(qe.outputIssues(context.Background(), feedback))[IssuePartialFailure] {
		t.Fatal("without Jev the error pattern must decide")
	}

	qe.SetOutputJudge(func(context.Context, *ExecutionFeedback) (OutputJudgments, error) {
		return OutputJudgments{Yes: 0.7, Probabilities: map[string]float64{
			learnjudge.OutputReportsError: 0.08, learnjudge.OutputReportsPartial: 0.5,
			learnjudge.OutputReportsMorePages: 0.9, learnjudge.OutputReportsRateLimit: 0.02,
		}}, nil
	})
	got := issueTypes(qe.outputIssues(context.Background(), feedback))
	if got[IssuePartialFailure] {
		t.Fatal("Jev read a result, not an error; the word 'error' must not make it one")
	}
	if got[IssueIncomplete] {
		t.Fatal("an unsettled partial-results reading is no issue")
	}
	if !got[IssuePagination] {
		t.Fatal("Jev's yes on more pages is the pagination issue")
	}

	empty := &ExecutionFeedback{ToolName: "lister", Success: true, Output: "[]"}
	if !issueTypes(qe.outputIssues(context.Background(), empty))[IssueIncomplete] {
		t.Fatal("the exact empty-output rule runs whatever Jev says")
	}

	qe.SetOutputJudge(func(context.Context, *ExecutionFeedback) (OutputJudgments, error) {
		return OutputJudgments{}, context.DeadlineExceeded
	})
	if !issueTypes(qe.outputIssues(context.Background(), feedback))[IssuePartialFailure] {
		t.Fatal("a failed Jev is no evidence: the error pattern must decide")
	}
}

// The orchestrator hands its evaluator Jev's output judge through the
// learnjudge questions, with the configured yes threshold.
func TestToolOutputJudge_AsksJevWithTheConfiguredThreshold(t *testing.T) {
	o := jevOrchestrator(t)
	c, calls := fakeJevServer(t, map[string]float64{learnjudge.OutputReportsRateLimit: 0.75}, 0)
	o.SetJev(c, &internalconfig.TypeSafeConfig{JudgmentYesPercent: 80, JudgmentNoPercent: 20})
	judge := o.toolOutputJudge()
	if judge == nil {
		t.Fatal("no judge with the checks on")
	}
	j, err := judge(context.Background(), &ExecutionFeedback{ToolName: "fetch", Output: "HTTP 429"})
	if err != nil || calls.Load() != 1 {
		t.Fatalf("judge: %v, calls %d", err, calls.Load())
	}
	if j.Yes != 0.8 || j.Probabilities[learnjudge.OutputReportsRateLimit] != 0.75 {
		t.Fatalf("judgments = %+v", j)
	}
	off := false
	o.SetJev(c, &internalconfig.TypeSafeConfig{AutopoiesisChecks: &off})
	if o.toolOutputJudge() != nil {
		t.Fatal("switched off, the evaluator must have no judge")
	}
}
