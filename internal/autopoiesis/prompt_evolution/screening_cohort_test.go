package prompt_evolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"
)

type cohortAsker struct {
	probability float64
	err         error
	states      []map[string]any
	report      bool
}

func (a *cohortAsker) Ask(ctx context.Context, state any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	if _, ok := qs[learnjudge.SharesOneCause]; !ok {
		return nil, fmt.Errorf("unexpected question: %v", qs)
	}
	a.states = append(a.states, state.(map[string]any))
	if a.err != nil {
		return nil, a.err
	}
	if a.report {
		usage.RecordFromContext(ctx, usage.Report{Provider: typesafe.Provider, Model: "screen-fixture-version", Operation: learnjudge.OpSharedCause,
			Usage: types.UsageMetadata{InputTokens: 17, OutputTokens: 2, TotalTokens: 19, Known: types.UsageTokens}})
	}
	wire, _ := json.Marshal(map[string]any{"model": "screen-fixture-version", "usage": map[string]any{"input_tokens": 17, "output_tokens": 2},
		"answers": map[string]any{learnjudge.SharesOneCause: map[string]any{"type": "noul", "noul": a.probability}}})
	var resp typesafe.Response
	if err := json.Unmarshal(wire, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type cohortLLM struct {
	countingLLM
	prompts []string
	fail    bool
}

func (c *cohortLLM) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	c.prompts = append(c.prompts, user)
	if c.fail {
		return "", errors.New("generation fixture unavailable")
	}
	return c.countingLLM.CompleteWithSystem(ctx, system, user)
}

func installCohortAsker(pe *PromptEvolver, a *cohortAsker) {
	off := false
	pe.SetJev(a, &nerdconfig.TypeSafeConfig{TaskVerdicts: &off, ProblemTypes: &off})
}
func recordCohortFailure(t *testing.T, pe *PromptEvolver, id, problem string, at time.Time) {
	t.Helper()
	r := failRecord(id, problem, at)
	r.TaskRequest = "request-marker-" + id
	r.Verdict.Category = CategoryLogicError
	r.Verdict.Explanation = "explanation-marker-" + id
	r.Verdict.ImprovementRule = "rule-marker-" + id
	if err := pe.RecordExecution(r); err != nil {
		t.Fatal(err)
	}
}
func cycleForCohort(t *testing.T, pe *PromptEvolver, ctx context.Context) *EvolutionResult {
	t.Helper()
	r, err := pe.RunEvolutionCycle(ctx)
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("cycle=%+v err=%v", r, err)
	}
	return r
}

// Before this change the 15-record fixture screens the oldest 12, renders
// only the newest 3 explanations, and retires all 15. These assertions use
// the real recorder, collector, policy, generator, candidate store and ledger.
func TestRunEvolutionCycle_ScreenedCohortIsGeneratedAndRetired(t *testing.T) {
	for _, n := range []int{3, 12, 13, 14, 15} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			llm := &cohortLLM{}
			pe := newLifecycleEvolver(t, t.TempDir(), llm, func(c *EvolverConfig) { c.MinFailuresForEvolution = 3 })
			defer pe.Close()
			a := &cohortAsker{probability: .95}
			installCohortAsker(pe, a)
			for i := 0; i < n; i++ {
				recordCohortFailure(t, pe, fmt.Sprintf("f%02d", i), "debugging", time.Unix(1700000000+int64(i), 0))
			}
			r := cycleForCohort(t, pe, context.Background())
			want := min(n, pe.config.EvidenceCohortSize)
			if r.ScreeningCalls != 1 || r.GenerationCalls != 1 || r.EvidenceConsumed != want || r.EvidenceDeferred != n-want {
				t.Fatalf("purchase=%+v", r)
			}
			if len(a.states) != 1 || len(llm.prompts) != 1 {
				t.Fatalf("screens=%d generations=%d", len(a.states), len(llm.prompts))
			}
			items := a.states[0]["failures"].([]map[string]any)
			if len(items) != want {
				t.Fatalf("screened %d, want %d", len(items), want)
			}
			selected := map[string]bool{}
			for _, item := range items {
				id := item["task_id"].(string)
				selected[id] = true
				for _, field := range []string{"task", "what_went_wrong", "suggested_rule"} {
					if !strings.Contains(llm.prompts[0], item[field].(string)) {
						t.Fatalf("screened %s of %s absent from generation", field, id)
					}
				}
			}
			if !selected["f00"] || !selected[fmt.Sprintf("f%02d", n-1)] {
				t.Fatalf("both ends must be reachable: %v", selected)
			}
			for id, ga := range pe.evolvedAtoms {
				if ga.EvidenceCohortID == "" || ga.ScreenObservationID == "" || len(ga.SourceIDs) != want {
					t.Fatalf("candidate %s provenance=%+v", id, ga)
				}
				for _, source := range ga.SourceIDs {
					if !selected[source] {
						t.Fatalf("unselected source %s retired", source)
					}
				}
				var state string
				if err := pe.ledger.db.QueryRow(`SELECT c.state_json FROM evolution_cohorts c JOIN generation_cohorts g ON g.cohort_id=c.cohort_id WHERE g.screen_key=?`, ga.ScreenObservationID).Scan(&state); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(llm.prompts[0], state) {
					t.Fatal("durable cohort differs from generator projection")
				}
			}
			remaining, err := pe.feedbackCollector.GetFailuresByProblemType(1, pe.config.AtomPinScope)
			if err != nil {
				t.Fatal(err)
			}
			left := 0
			for _, records := range remaining {
				for _, record := range records {
					left++
					if selected[record.TaskID] {
						t.Fatal("selected evidence remains eligible")
					}
				}
			}
			if left != n-want {
				t.Fatalf("unselected backlog=%d, want %d", left, n-want)
			}
		})
	}
}

// Previously all three screens were bought in each cycle although generation
// allowance was one. Cached negatives/unsettled results must cost no calls,
// and must not consume their evidence. New cohorts invalidate exact reuse.
func TestRunEvolutionCycle_ScreenBudgetCacheAndNewEvidence(t *testing.T) {
	for _, probability := range []float64{.1, .5} {
		t.Run(fmt.Sprint(probability), func(t *testing.T) {
			llm := &cohortLLM{}
			pe := newLifecycleEvolver(t, t.TempDir(), llm, func(c *EvolverConfig) { c.MaxCycleScreeningCalls = 1; c.MaxCycleGenerationCalls = 1 })
			defer pe.Close()
			a := &cohortAsker{probability: probability}
			installCohortAsker(pe, a)
			for i, problem := range []string{"debugging", "testing", "refactoring"} {
				recordCohortFailure(t, pe, problem, problem, time.Unix(1700000000+int64(i), 0))
			}
			for i := 0; i < 3; i++ {
				r := cycleForCohort(t, pe, context.Background())
				if r.ScreeningCalls != 1 || r.ScreeningReused != i || r.GenerationCalls != 0 || r.EvidenceConsumed != 0 || r.GroupsDeferred != 2-i {
					t.Fatalf("cycle %d: %+v", i, r)
				}
			}
			r := cycleForCohort(t, pe, context.Background())
			if r.ScreeningCalls != 0 || r.ScreeningReused != 3 || len(a.states) != 3 {
				t.Fatalf("unchanged backlog repurchased: %+v", r)
			}
			recordCohortFailure(t, pe, "new", "debugging", time.Unix(1700000100, 0))
			r = cycleForCohort(t, pe, context.Background())
			if r.ScreeningCalls != 1 || len(a.states) != 4 {
				t.Fatalf("new evidence not screened: %+v", r)
			}
			items := a.states[3]["failures"].([]map[string]any)
			if items[0]["task_id"] != "new" {
				t.Fatalf("newest failure not exposed: %v", items)
			}
			// An explicit Jev reattachment is a conservative model-version
			// boundary: moving aliases cannot reuse a previous attachment.
			installCohortAsker(pe, a)
			r = cycleForCohort(t, pe, context.Background())
			if r.ScreeningCalls != 1 || r.ScreeningReused != 0 {
				t.Fatalf("model attachment reused old observation: %+v", r)
			}
		})
	}
}

func TestRunEvolutionCycle_ScreenReceiptAndRetry(t *testing.T) {
	tracker, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer tracker.Close()
	llm := &cohortLLM{fail: true}
	pe := newLifecycleEvolver(t, t.TempDir(), llm, nil)
	defer pe.Close()
	a := &cohortAsker{probability: .95, report: true}
	installCohortAsker(pe, a)
	recordCohortFailure(t, pe, "receipt", "debugging", time.Unix(1700000000, 0))
	ctx := usage.NewContext(context.Background(), tracker)
	r, err := pe.RunEvolutionCycle(ctx)
	if err != nil || len(r.Errors) != 1 || r.ScreeningCalls != 1 || r.EvidenceConsumed != 0 {
		t.Fatalf("failed generation: %+v err=%v", r, err)
	}
	llm.fail = false
	r = cycleForCohort(t, pe, ctx)
	if r.ScreeningCalls != 0 || r.ScreeningReused != 1 || r.EvidenceConsumed != 1 {
		t.Fatalf("retry: %+v", r)
	}
	var receipts, complete int
	var reportsJSON, model string
	if err := pe.ledger.db.QueryRow(`SELECT COUNT(*),usage_complete,usage_json,model FROM screening_receipts`).Scan(&receipts, &complete, &reportsJSON, &model); err != nil {
		t.Fatal(err)
	}
	var reports []usage.Report
	if err := json.Unmarshal([]byte(reportsJSON), &reports); err != nil {
		t.Fatal(err)
	}
	if receipts != 1 || complete != 1 || model != "screen-fixture-version" || len(reports) != 1 || reports[0].Usage.TotalTokens != 19 {
		t.Fatalf("receipt count=%d complete=%d model=%s reports=%+v", receipts, complete, model, reports)
	}
	if tracker.Stats().TotalProject.Total != 19 {
		t.Fatalf("aggregate charged twice or lost usage: %+v", tracker.Stats().TotalProject)
	}
	spend, err := pe.ledger.SpendSummaries()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range spend {
		if s.Category == SpendScreen {
			found = true
			if s.Calls != 1 {
				t.Fatalf("screen spend=%+v", s)
			}
		}
	}
	if !found {
		t.Fatal("screening missing from adaptation spend")
	}
}

func TestRunEvolutionCycle_UnavailableScreenIsNotNegativeOrFree(t *testing.T) {
	llm := &cohortLLM{fail: true}
	pe := newLifecycleEvolver(t, t.TempDir(), llm, nil)
	defer pe.Close()
	a := &cohortAsker{err: errors.New("screen transport failed")}
	installCohortAsker(pe, a)
	recordCohortFailure(t, pe, "unavailable", "debugging", time.Unix(1700000000, 0))
	for i := 0; i < 2; i++ {
		r, err := pe.RunEvolutionCycle(context.Background())
		if err != nil || r.ScreeningUnavailable != 1 || r.GroupsWithoutSharedCause != 0 || r.GenerationCalls != 1 || r.EvidenceConsumed != 0 || r.ScreeningCalls != 1-i || r.ScreeningReused != i {
			t.Fatalf("unavailable fallback: %+v err=%v", r, err)
		}
	}
	var complete int
	var outcome string
	if err := pe.ledger.db.QueryRow(`SELECT usage_complete,outcome FROM screening_receipts`).Scan(&complete, &outcome); err != nil {
		t.Fatal(err)
	}
	if complete != 0 || outcome != "unavailable" || len(a.states) != 1 {
		t.Fatalf("complete=%d outcome=%s calls=%d", complete, outcome, len(a.states))
	}
}

func TestRunEvolutionCycle_CohortProjectionIsBoundedAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	llm := &cohortLLM{}
	pe := newLifecycleEvolver(t, dir, llm, nil)
	defer pe.Close()
	a := &cohortAsker{probability: .95}
	installCohortAsker(pe, a)
	r := failRecord("long", "debugging", time.Unix(1700000000, 0))
	r.TaskRequest = "head-request-" + strings.Repeat("x", 5000) + "-tail-request"
	r.Verdict.Explanation = "head-explanation-" + strings.Repeat("x", 5000) + "-tail-explanation"
	r.Verdict.ImprovementRule = "head-rule-" + strings.Repeat("x", 5000) + "-tail-rule"
	if err := pe.RecordExecution(r); err != nil {
		t.Fatal(err)
	}
	cycleForCohort(t, pe, context.Background())
	item := a.states[0]["failures"].([]map[string]any)[0]
	for _, field := range []string{"task", "what_went_wrong", "suggested_rule"} {
		if item[field+"_omitted_bytes"].(int) <= 0 {
			t.Fatalf("%s omission absent", field)
		}
		projected, _ := json.Marshal(item[field])
		if !strings.Contains(llm.prompts[0], string(projected)) {
			t.Fatalf("%s bounded projection drifted", field)
		}
	}
	var cohortID, screenID, atomID string
	for id, ga := range pe.evolvedAtoms {
		atomID = id
		cohortID = ga.EvidenceCohortID
		screenID = ga.ScreenObservationID
	}
	if err := pe.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := newLifecycleEvolver(t, dir, llm, nil)
	defer reopened.Close()
	ga := reopened.evolvedAtoms[atomID]
	if ga == nil || ga.EvidenceCohortID != cohortID || ga.ScreenObservationID != screenID {
		t.Fatalf("candidate cohort lost on restart: %+v", ga)
	}
	stored, err := reopened.feedbackCollector.GetFailuresByProblemType(1, reopened.config.AtomPinScope)
	if err != nil || len(stored) != 0 {
		t.Fatalf("retired evidence reused after restart: %+v %v", stored, err)
	}
}

func TestRunEvolutionCycle_CohortTieOrderAndVerdictRevision(t *testing.T) {
	llm := &cohortLLM{}
	pe := newLifecycleEvolver(t, t.TempDir(), llm, func(c *EvolverConfig) { c.EvidenceCohortSize = 3 })
	defer pe.Close()
	a := &cohortAsker{probability: .1}
	installCohortAsker(pe, a)
	for _, id := range []string{"d", "b", "a", "c"} {
		recordCohortFailure(t, pe, id, "debugging", time.Unix(1700000000, 0))
	}
	cycleForCohort(t, pe, context.Background())
	items := a.states[0]["failures"].([]map[string]any)
	for i, want := range []string{"a", "d", "b"} {
		if items[i]["task_id"] != want {
			t.Fatalf("unstable tie order: %v", items)
		}
	}
	r := cycleForCohort(t, pe, context.Background())
	if r.ScreeningReused != 1 {
		t.Fatal("tied cohort not reused")
	}
	// The same selected task, with a revised verdict version, is a new
	// observation identity even if the number of records is unchanged.
	v := &JudgeVerdict{TaskID: "a", Verdict: "FAIL", Category: CategoryEdgeCase, Explanation: "revised explanation"}
	if _, _, err := pe.feedbackCollector.UpdateVerdict("a", v); err != nil {
		t.Fatal(err)
	}
	r = cycleForCohort(t, pe, context.Background())
	if r.ScreeningCalls != 1 || r.ScreeningReused != 0 {
		t.Fatalf("revision reused stale judgment: %+v", r)
	}
}
