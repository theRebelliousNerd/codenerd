package prompt_evolution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/prompt"
)

// countingLLM answers generation calls with one uniquely named atom and
// counts them. A non-nil gate blocks generation calls until it is closed.
type countingLLM struct {
	generations atomic.Int64
	gate        chan struct{}
	entered     chan struct{}
}

func (c *countingLLM) Complete(ctx context.Context, p string) (string, error) { return "", nil }

func (c *countingLLM) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	if strings.Contains(system, "expert evaluator for an AI coding agent") {
		return `{"verdict":"FAIL","explanation":"failed","category":"LOGIC_ERROR"}`, nil
	}
	n := c.generations.Add(1)
	if c.entered != nil {
		select {
		case c.entered <- struct{}{}:
		default:
		}
	}
	if c.gate != nil {
		select {
		case <-c.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return fmt.Sprintf("```yaml\n- id: test/evolved/gen-%d\n  category: methodology\n  content: Generated guidance %d\n```", n, n), nil
}

func newLifecycleEvolver(t *testing.T, dir string, llm LLMClient, tweak func(*EvolverConfig)) *PromptEvolver {
	t.Helper()
	cfg := DefaultEvolverConfig()
	cfg.MinFailuresForEvolution = 1
	cfg.EnableStrategies = false
	if tweak != nil {
		tweak(cfg)
	}
	pe, err := NewPromptEvolver(dir, llm, cfg, newTestJudgeCompiler(t), nerdconfig.DefaultJITConfig())
	if err != nil {
		t.Fatalf("NewPromptEvolver: %v", err)
	}
	return pe
}

func failRecord(id, problem string, at time.Time) *ExecutionRecord {
	return &ExecutionRecord{
		TaskID: id, SessionID: "s", ShardType: "/coder", TaskRequest: "fix it",
		ProblemType: problem, Timestamp: at,
		ExecutionResult: ExecutionResult{Success: false},
		Verdict:         &JudgeVerdict{Verdict: "FAIL", Explanation: "broke " + id, TaskID: id},
	}
}

func storeCandidate(t *testing.T, pe *PromptEvolver, id, content string) *GeneratedAtom {
	t.Helper()
	ga := &GeneratedAtom{
		Atom:      &prompt.PromptAtom{ID: id, Category: prompt.CategoryMethodology, Content: content},
		Source:    "failure_analysis",
		CreatedAt: time.Now(),
	}
	pe.mu.Lock()
	err := pe.storeEvolvedAtom(ga)
	pe.mu.Unlock()
	if err != nil {
		t.Fatalf("storeEvolvedAtom: %v", err)
	}
	return ga
}

// GAP-PE-04: a replayed event adds nothing; a different payload under the
// same identity is refused.
func TestRecordExecution_ReplayIsIdempotentAndConflictIsRefused(t *testing.T) {
	pe := newLifecycleEvolver(t, t.TempDir(), &countingLLM{}, nil)
	defer pe.Close()
	ga := storeCandidate(t, pe, "test/evolved/replay", "verify outputs")

	rec := &ExecutionRecord{
		TaskID: "evt-1", SessionID: "s", ShardType: "/coder", Timestamp: time.Unix(1700000000, 0),
		AtomIDs: []string{ga.Atom.ID}, Verdict: &JudgeVerdict{Verdict: "PASS"},
		ExecutionResult: ExecutionResult{Success: true},
	}
	for range 3 {
		replay := *rec
		if err := pe.RecordExecution(&replay); err != nil {
			t.Fatalf("RecordExecution: %v", err)
		}
	}
	if recorded, _ := pe.feedbackCollector.GetStats(); recorded != 1 {
		t.Errorf("recorded = %d after three identical arrivals, want 1", recorded)
	}
	if ga.UsageCount != 1 || ga.SuccessCount != 1 {
		t.Errorf("counts = %d/%d, want 1/1", ga.SuccessCount, ga.UsageCount)
	}

	conflict := *rec
	conflict.TaskRequest = "a different turn wearing the same id"
	if err := pe.RecordExecution(&conflict); !errors.Is(err, ErrConflictingEvent) {
		t.Fatalf("conflicting payload: err = %v, want ErrConflictingEvent", err)
	}
}

// GAP-PE-04: a revised verdict supersedes its old contribution, and the
// failure counter moves by the difference.
func TestApplyVerdict_RevisionSupersedesInsteadOfDoubleCounting(t *testing.T) {
	pe := newLifecycleEvolver(t, t.TempDir(), &countingLLM{}, nil)
	defer pe.Close()
	ga := storeCandidate(t, pe, "test/evolved/revise", "check edge cases")

	rec := failRecord("evt-rev", "debugging", time.Unix(1700000000, 0))
	rec.AtomIDs = []string{ga.Atom.ID}
	if err := pe.RecordExecution(rec); err != nil {
		t.Fatal(err)
	}
	if ga.UsageCount != 1 || ga.SuccessCount != 0 {
		t.Fatalf("after FAIL: %d/%d", ga.SuccessCount, ga.UsageCount)
	}
	if _, failures := pe.feedbackCollector.GetStats(); failures != 1 {
		t.Fatalf("failures = %d, want 1", failures)
	}

	pass := &JudgeVerdict{Verdict: "PASS", TaskID: "evt-rev"}
	for range 2 { // the second application is a replay of the revision
		if err := pe.applyVerdict(rec, pass); err != nil {
			t.Fatal(err)
		}
	}
	if ga.UsageCount != 1 || ga.SuccessCount != 1 {
		t.Errorf("after revision to PASS: %d/%d, want 1/1", ga.SuccessCount, ga.UsageCount)
	}
	if _, failures := pe.feedbackCollector.GetStats(); failures != 0 {
		t.Errorf("failures = %d after FAIL->PASS, want 0", failures)
	}
}

// GAP-PE-02: unchanged evidence cannot repurchase generation, across cycles
// and across a restart.
func TestRunEvolutionCycle_UnchangedEvidenceIsNotRepurchased(t *testing.T) {
	dir := t.TempDir()
	llm := &countingLLM{}
	pe := newLifecycleEvolver(t, dir, llm, nil)
	if err := pe.RecordExecution(failRecord("f-1", "debugging", time.Unix(1700000000, 0))); err != nil {
		t.Fatal(err)
	}
	r1, err := pe.RunEvolutionCycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r1.GenerationCalls != 1 || r1.AtomsGenerated != 1 || r1.EvidenceConsumed != 1 {
		t.Fatalf("first cycle = %+v", r1)
	}
	r2, err := pe.RunEvolutionCycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r2.GenerationCalls != 0 || llm.generations.Load() != 1 {
		t.Fatalf("second cycle bought generation again from consumed evidence: %+v", r2)
	}
	if err := pe.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := newLifecycleEvolver(t, dir, llm, nil)
	defer restarted.Close()
	r3, err := restarted.RunEvolutionCycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r3.GenerationCalls != 0 || llm.generations.Load() != 1 {
		t.Fatalf("restart bought generation again: %+v", r3)
	}

	// New evidence is new: one more failure qualifies its group again.
	if err := restarted.RecordExecution(failRecord("f-2", "debugging", time.Unix(1700000100, 0))); err != nil {
		t.Fatal(err)
	}
	r4, _ := restarted.RunEvolutionCycle(context.Background())
	if r4.GenerationCalls != 1 || r4.EvidenceConsumed != 1 {
		t.Fatalf("new evidence did not buy exactly one generation: %+v", r4)
	}
}

// GAP-PE-02: the per-cycle budget spans groups, and the oldest evidence is
// served first; the rest is deferred, not dropped.
func TestRunEvolutionCycle_GlobalBudgetOldestFirst(t *testing.T) {
	llm := &countingLLM{}
	pe := newLifecycleEvolver(t, t.TempDir(), llm, func(c *EvolverConfig) {
		c.MaxCycleGenerationCalls = 1
		c.MaxCycleAtoms = 1
	})
	defer pe.Close()
	base := time.Unix(1700000000, 0)
	if err := pe.RecordExecution(failRecord("new", "testing", base.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := pe.RecordExecution(failRecord("old", "debugging", base)); err != nil {
		t.Fatal(err)
	}
	if err := pe.RecordExecution(failRecord("mid", "refactoring", base.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}

	r, err := pe.RunEvolutionCycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.GenerationCalls != 1 || r.GroupsDeferred != 2 {
		t.Fatalf("budget not global: %+v", r)
	}
	remaining, _ := pe.feedbackCollector.GetFailuresByProblemType(1, pe.config.AtomPinScope)
	for key := range remaining {
		if key.ProblemType == "debugging" {
			t.Fatal("the oldest group was not served first")
		}
	}
	if len(remaining) != 2 {
		t.Fatalf("deferred groups = %d, want 2 left unconsumed", len(remaining))
	}
}

// GAP-PE-02: claims are fenced; a stale owner cannot publish.
func TestEvidenceLedger_ClaimFencing(t *testing.T) {
	pe := newLifecycleEvolver(t, t.TempDir(), &countingLLM{}, nil)
	defer pe.Close()
	l := pe.ledger
	now := time.Unix(1700000000, 0)
	ev := []EvidenceRef{{TaskID: "a", VerdictVersion: "v"}}
	key := generationClaimKey("g", atomGeneratorVersion, ev)

	if ok, err := l.Claim(key, "g", atomGeneratorVersion, 1, "owner-1", time.Minute, now); err != nil || !ok {
		t.Fatalf("first claim: %v %v", ok, err)
	}
	if ok, _ := l.Claim(key, "g", atomGeneratorVersion, 1, "owner-2", time.Minute, now.Add(30*time.Second)); ok {
		t.Fatal("a live lease was taken over")
	}
	if ok, _ := l.Claim(key, "g", atomGeneratorVersion, 1, "owner-2", time.Minute, now.Add(2*time.Minute)); !ok {
		t.Fatal("an expired lease could not be taken over")
	}
	if err := l.Complete(key, "owner-1", ev, nil, now); !errors.Is(err, ErrClaimNotOwned) {
		t.Fatalf("stale owner completed: %v", err)
	}
	if err := l.Complete(key, "owner-2", ev, nil, now); err != nil {
		t.Fatal(err)
	}
	if ok, _ := l.Claim(key, "g", atomGeneratorVersion, 1, "owner-3", time.Minute, now.Add(time.Hour)); ok {
		t.Fatal("a completed claim was re-acquired")
	}
	if generationClaimKey("g", "atomgen-v2", ev) == key {
		t.Fatal("a new generator version must be a new claim (deliberate reanalysis)")
	}
}

// GAP-PE-03: no model call is made under the evolver lock, so recording
// proceeds while a generation is in flight.
func TestRunEvolutionCycle_RecordingIsNotBlockedByModelCall(t *testing.T) {
	llm := &countingLLM{gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	pe := newLifecycleEvolver(t, t.TempDir(), llm, nil)
	defer pe.Close()
	if err := pe.RecordExecution(failRecord("blocking", "debugging", time.Unix(1700000000, 0))); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = pe.RunEvolutionCycle(context.Background())
	}()
	<-llm.entered

	recorded := make(chan error, 1)
	go func() {
		recorded <- pe.RecordExecution(failRecord("during", "testing", time.Unix(1700000001, 0)))
	}()
	select {
	case err := <-recorded:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RecordExecution blocked behind an in-flight generation call")
	}
	close(llm.gate)
	<-done
}

// GAP-PE-03: the queue is bounded with explicit outcomes, Close drains
// accepted records exactly once, and nothing is admitted afterwards.
func TestRecordQueue_BoundedDrainAndClosedAdmission(t *testing.T) {
	pe := newLifecycleEvolver(t, t.TempDir(), &countingLLM{}, func(c *EvolverConfig) {
		c.RecordQueueCapacity = 2
	})
	ga := storeCandidate(t, pe, "test/evolved/queued", "queued guidance")

	// Hold the evolver lock so the worker parks inside its first record.
	pe.mu.Lock()
	var admissions []RecordAdmission
	for i := range 6 {
		rec := &ExecutionRecord{
			TaskID: fmt.Sprintf("q-%d", i), SessionID: "s", ShardType: "/coder",
			Timestamp: time.Unix(1700000000, int64(i)), AtomIDs: []string{ga.Atom.ID},
			Verdict: &JudgeVerdict{Verdict: "PASS"}, ExecutionResult: ExecutionResult{Success: true},
		}
		admissions = append(admissions, pe.SubmitExecution(rec, false))
		time.Sleep(20 * time.Millisecond) // let the worker take the first one
	}
	pe.mu.Unlock()

	accepted, full := 0, 0
	for _, a := range admissions {
		switch a {
		case RecordAccepted:
			accepted++
		case RecordRejectedFull:
			full++
		}
	}
	if full == 0 || accepted+full != 6 {
		t.Fatalf("admissions = %v: a bounded queue must reject when full", admissions)
	}

	if err := pe.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	stats := pe.recorder.stats()
	if stats.Persisted != int64(accepted) || stats.Abandoned != 0 {
		t.Fatalf("drain: persisted %d of %d accepted (stats %+v)", stats.Persisted, accepted, stats)
	}
	if a := pe.SubmitExecution(&ExecutionRecord{TaskID: "late"}, false); a != RecordRejectedClosed {
		t.Errorf("submit after Close = %s, want rejected_closed", a)
	}
	if err := pe.RecordExecution(&ExecutionRecord{TaskID: "late"}); !errors.Is(err, ErrEvolverClosed) {
		t.Errorf("RecordExecution after Close = %v, want ErrEvolverClosed", err)
	}

	// Each acknowledged event is in the reopened store exactly once.
	reopened, err := NewFeedbackCollector(pe.nerdDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if recorded, _ := reopened.GetStats(); recorded != accepted {
		t.Errorf("reopened store holds %d records, want %d", recorded, accepted)
	}
}

// GAP-PE-03: a drain that misses its deadline is an incomplete shutdown,
// reported with what it abandoned, and the stores still close only after
// the worker returns.
func TestRecordQueue_DrainTimeoutIsReported(t *testing.T) {
	pe := newLifecycleEvolver(t, t.TempDir(), &countingLLM{}, func(c *EvolverConfig) {
		c.RecordQueueCapacity = 8
		c.RecordDrainTimeout = 50 * time.Millisecond
	})
	ga := storeCandidate(t, pe, "test/evolved/drain", "drain guidance")
	pe.mu.Lock()
	for i := range 4 {
		pe.SubmitExecution(&ExecutionRecord{
			TaskID: fmt.Sprintf("d-%d", i), SessionID: "s", ShardType: "/coder",
			Timestamp: time.Unix(1700000000, int64(i)), AtomIDs: []string{ga.Atom.ID},
		}, false)
	}
	closed := make(chan error, 1)
	go func() { closed <- pe.Close() }()
	time.Sleep(200 * time.Millisecond)
	pe.mu.Unlock()
	err := <-closed
	if err == nil || !strings.Contains(err.Error(), "incomplete shutdown") {
		t.Fatalf("Close = %v, want an incomplete-shutdown error", err)
	}
	if pe.recorder.stats().Abandoned == 0 {
		t.Error("no abandoned records counted")
	}
}

// GAP-PE-01: a trial pins one candidate version, its exposures are counted
// from recorded turns, and promotion or rejection ends it.
func TestTrials_EnvelopeExposureAndEnd(t *testing.T) {
	pe := newLifecycleEvolver(t, t.TempDir(), &countingLLM{}, func(c *EvolverConfig) {
		c.TrialMaxExposures = 2
	})
	defer pe.Close()
	ga := storeCandidate(t, pe, "test/evolved/trial", "trial guidance")

	var mu sync.Mutex
	var last []TrialEnvelope
	pe.SetTrialFactSink(func(live []TrialEnvelope) {
		mu.Lock()
		last = live
		mu.Unlock()
	})
	snapshot := func() []TrialEnvelope {
		mu.Lock()
		defer mu.Unlock()
		return append([]TrialEnvelope(nil), last...)
	}

	trial, err := pe.ApproveTrial(ga.Atom.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if trial.Version != prompt.EvolvedCandidateHash(ga.Atom) || trial.MaxExposures != 2 {
		t.Fatalf("envelope = %+v", trial)
	}
	if got := snapshot(); len(got) != 1 || got[0].Exposures != 0 {
		t.Fatalf("sink after approval = %+v", got)
	}

	for i := range 2 {
		if err := pe.RecordExecution(&ExecutionRecord{
			TaskID: fmt.Sprintf("t-%d", i), SessionID: "s", ShardType: "/coder",
			Timestamp: time.Unix(1700000000, int64(i)), AtomIDs: []string{ga.Atom.ID},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Two exposures exhaust a budget of two: the envelope is no longer live.
	if got := snapshot(); len(got) != 0 {
		t.Fatalf("exhausted trial still published: %+v", got)
	}

	if _, err := pe.ApproveTrial(ga.Atom.ID, time.Hour, 10); err != nil {
		t.Fatal(err)
	}
	if got := snapshot(); len(got) != 1 || got[0].Exposures != 2 {
		t.Fatalf("re-approved trial = %+v", got)
	}
	if err := pe.PromoteAtom(ga.Atom.ID); err != nil {
		t.Fatal(err)
	}
	if got := snapshot(); len(got) != 0 {
		t.Fatalf("promotion did not end the trial: %+v", got)
	}
	if _, err := pe.ApproveTrial(ga.Atom.ID, time.Hour, 1); err == nil {
		t.Error("a promoted atom was put on trial")
	}
}

// GAP-PE-04 (cost): judge and generation calls land in the disjoint spend
// ledger by category.
func TestSpendLedger_RecordsAdaptationCalls(t *testing.T) {
	llm := &countingLLM{}
	pe := newLifecycleEvolver(t, t.TempDir(), llm, nil)
	defer pe.Close()
	unjudged := failRecord("u-1", "debugging", time.Unix(1700000000, 0))
	unjudged.Verdict = nil
	if err := pe.RecordExecution(unjudged); err != nil {
		t.Fatal(err)
	}
	if _, err := pe.RunEvolutionCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	byCategory := map[SpendCategory]SpendSummary{}
	for _, s := range pe.GetStats().Spend {
		byCategory[s.Category] = s
	}
	if byCategory[SpendJudge].Calls != 1 {
		t.Errorf("judge spend = %+v", byCategory[SpendJudge])
	}
	if byCategory[SpendGenerate].Calls != 1 {
		t.Errorf("generate spend = %+v", byCategory[SpendGenerate])
	}
}
