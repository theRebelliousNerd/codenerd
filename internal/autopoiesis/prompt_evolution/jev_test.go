package prompt_evolution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// jev_test.go: the task verdict, problem type and evolution trigger read Jev's
// observations through truth tables, and each falls back when Jev gives none.

// fakeJev answers nouls from nouls (0.5 when unlisted) and choices from
// choices; err fails every call.
type fakeJev struct {
	mu      sync.Mutex
	nouls   map[string]float64
	choices map[string]string
	err     error
	calls   atomic.Int32
}

func (f *fakeJev) Ask(_ context.Context, _ any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	answers := map[string]any{}
	for name, q := range qs {
		if c, ok := q.(typesafe.Choice); ok {
			pick := f.choices[name]
			probs := map[string]float64{}
			for opt := range c.Criteria {
				if pick == "" {
					pick = opt
				}
				probs[opt] = 0.01
			}
			probs[pick] = 0.9
			answers[name] = map[string]any{"type": "choice", "choice": pick, "confidence": 0.9, "probabilities": probs}
			continue
		}
		p, ok := f.nouls[name]
		if !ok {
			p = 0.5
		}
		answers[name] = map[string]any{"type": "noul", "noul": p}
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	var resp typesafe.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func testJevUse(f *fakeJev, sec nerdconfig.TypeSafeConfig) *jevUse {
	return newJevUse(f, &sec)
}

func judged(did, early, fits float64, kind string, files ...float64) *learnjudge.TaskJudgments {
	return &learnjudge.TaskJudgments{DidWhatWasAsked: did, StoppedEarly: early, CategoryFits: fits, Category: kind, FileNeeded: files, Model: "jev-test"}
}

func TestVerdictFromJudgments_TruthTable(t *testing.T) {
	u := testJevUse(&fakeJev{}, nerdconfig.TypeSafeConfig{})
	ok := &ExecutionRecord{TaskID: "t", ExecutionResult: ExecutionResult{Success: true}}
	failedRun := &ExecutionRecord{TaskID: "t", ExecutionResult: ExecutionResult{Success: false, BuildErrors: []string{"hollow"}}}
	files := []string{"stats.go", "README.md"}
	cases := []struct {
		name     string
		exec     *ExecutionRecord
		j        *learnjudge.TaskJudgments
		changed  []string
		verdict  string // "" is unsettled
		category ErrorCategory
	}{
		{"clean pass", ok, judged(0.95, 0.05, 0.1, "logic_error", 0.9, 0.8), files, "PASS", CategoryCorrect},
		{"not done", ok, judged(0.1, 0.1, 0.9, "instruction_miss"), nil, "FAIL", CategoryInstructionMiss},
		{"stopped early", ok, judged(0.6, 0.85, 0.9, "context_miss"), nil, "FAIL", CategoryContextMiss},
		{"stray file", ok, judged(0.9, 0.05, 0.9, "instruction_miss", 0.9, 0.1), files, "FAIL", CategoryInstructionMiss},
		{"kernel failed though Jev reads it done", failedRun, judged(0.9, 0.05, 0.85, "hallucination"), nil, "FAIL", CategoryHallucination},
		{"kernel failed, no agent mistake", failedRun, judged(0.2, 0.2, 0.1, "logic_error"), nil, "FAIL", CategoryNoAgentMistake},
		{"did unsettled", ok, judged(0.5, 0.1, 0.5, "logic_error"), nil, "", ""},
		{"file unsettled", ok, judged(0.9, 0.1, 0.5, "logic_error", 0.5), files[:1], "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, settled := verdictFromJudgments(tc.exec, tc.j, u, tc.changed)
			if tc.verdict == "" {
				if settled {
					t.Fatalf("settled %+v, want unsettled", v)
				}
				return
			}
			if !settled || v.Verdict != tc.verdict || v.Category != tc.category {
				t.Fatalf("verdict = %+v (settled %v), want %s %s", v, settled, tc.verdict, tc.category)
			}
			if !strings.HasPrefix(v.EvaluatedBy, "jev:") || v.Explanation == "" {
				t.Fatalf("verdict not attributed or explained: %+v", v)
			}
		})
	}
}

// judgeCountingLLM answers every judge call with one fixed verdict and counts them.
type judgeCountingLLM struct{ calls atomic.Int32 }

func (c *judgeCountingLLM) Complete(context.Context, string) (string, error) { return "", nil }
func (c *judgeCountingLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	c.calls.Add(1)
	return `{"verdict":"FAIL","explanation":"llm says","category":"EDGE_CASE"}`, nil
}

type jevStubCompiler struct{}

func (jevStubCompiler) Compile(context.Context, *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "judge"}, nil
}

func cascadeEvolver(t *testing.T, llm *judgeCountingLLM) *PromptEvolver {
	t.Helper()
	pe, err := NewPromptEvolver(t.TempDir(), llm, nil, jevStubCompiler{}, nerdconfig.JITConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pe.Close() })
	return pe
}

func TestEvaluateBatch_Cascade(t *testing.T) {
	recs := func() []*ExecutionRecord {
		return []*ExecutionRecord{{TaskID: "a", TaskRequest: "fix Max", ExecutionResult: ExecutionResult{Success: true}}}
	}
	t.Run("jev settles, no LLM call", func(t *testing.T) {
		llm := &judgeCountingLLM{}
		pe := cascadeEvolver(t, llm)
		pe.SetJev(&fakeJev{nouls: map[string]float64{learnjudge.DidWhatWasAsked: 0.95, learnjudge.StoppedEarly: 0.05}}, &nerdconfig.TypeSafeConfig{})
		v, err := pe.evaluateBatch(context.Background(), recs())
		if err != nil || v[0] == nil || v[0].Verdict != "PASS" || llm.calls.Load() != 0 {
			t.Fatalf("verdict %+v err %v llm calls %d", v[0], err, llm.calls.Load())
		}
	})
	t.Run("unsettled goes to the LLM judge", func(t *testing.T) {
		llm := &judgeCountingLLM{}
		pe := cascadeEvolver(t, llm)
		pe.SetJev(&fakeJev{nouls: map[string]float64{learnjudge.DidWhatWasAsked: 0.5, learnjudge.StoppedEarly: 0.05}}, &nerdconfig.TypeSafeConfig{})
		v, _ := pe.evaluateBatch(context.Background(), recs())
		if v[0] == nil || v[0].Category != CategoryEdgeCase || llm.calls.Load() != 1 {
			t.Fatalf("verdict %+v llm calls %d", v[0], llm.calls.Load())
		}
	})
	t.Run("failing jev is not a no: the LLM judge decides", func(t *testing.T) {
		llm := &judgeCountingLLM{}
		pe := cascadeEvolver(t, llm)
		pe.SetJev(&fakeJev{err: errors.New("503")}, &nerdconfig.TypeSafeConfig{})
		v, _ := pe.evaluateBatch(context.Background(), recs())
		if v[0] == nil || v[0].EvaluatedBy == "jev:jev-test" || llm.calls.Load() != 1 {
			t.Fatalf("verdict %+v llm calls %d", v[0], llm.calls.Load())
		}
	})
	t.Run("switch off asks no Jev", func(t *testing.T) {
		llm := &judgeCountingLLM{}
		pe := cascadeEvolver(t, llm)
		off := false
		f := &fakeJev{}
		pe.SetJev(f, &nerdconfig.TypeSafeConfig{TaskVerdicts: &off})
		_, _ = pe.evaluateBatch(context.Background(), recs())
		if f.calls.Load() != 0 || llm.calls.Load() != 1 {
			t.Fatalf("jev calls %d llm calls %d", f.calls.Load(), llm.calls.Load())
		}
	})
}

func TestProblemClassifier_JevFirstPatternFallback(t *testing.T) {
	f := &fakeJev{nouls: map[string]float64{learnjudge.ProblemKindFits: 0.9}, choices: map[string]string{learnjudge.ProblemKind: "concurrency"}}
	pc := NewProblemClassifier()
	pc.setJev(testJevUse(f, nerdconfig.TypeSafeConfig{}))
	// "fix" would win on the patterns; Jev reads the race.
	if got, _ := pc.ClassifyWithContext(context.Background(), "fix the data race between the cache writer and the reader goroutine", "/coder"); got != ProblemConcurrency {
		t.Fatalf("type = %s, want Jev's concurrency", got)
	}
	_, _ = pc.ClassifyWithContext(context.Background(), "fix the data race between the cache writer and the reader goroutine", "/coder")
	if f.calls.Load() != 1 {
		t.Fatalf("Jev asked %d times for one task, want the cached answer", f.calls.Load())
	}

	none := &fakeJev{nouls: map[string]float64{learnjudge.ProblemKindFits: 0.05}, choices: map[string]string{learnjudge.ProblemKind: "debugging"}}
	pc.setJev(testJevUse(none, nerdconfig.TypeSafeConfig{}))
	if got, _ := pc.Classify("hello there, how are you"); got != ProblemOther {
		t.Fatalf("type = %s, want other when no kind fits", got)
	}

	pc.setJev(testJevUse(&fakeJev{err: errors.New("timeout")}, nerdconfig.TypeSafeConfig{}))
	if got, _ := pc.Classify("write unit tests for the parser"); got != ProblemTesting {
		t.Fatalf("type = %s, want the pattern classifier's testing on a failed call", got)
	}
	off := false
	silent := &fakeJev{}
	pc.setJev(testJevUse(silent, nerdconfig.TypeSafeConfig{ProblemTypes: &off}))
	if _, _ = pc.Classify("write unit tests for the parser"); silent.calls.Load() != 0 {
		t.Fatal("problem_types off still asked Jev")
	}
}

func TestEvolutionTrigger_TruthTable(t *testing.T) {
	fail := func(cat ErrorCategory) *ExecutionRecord {
		return &ExecutionRecord{TaskID: string(cat), TaskRequest: "x", Verdict: &JudgeVerdict{Verdict: "FAIL", Category: cat, Explanation: "e"}}
	}
	if got := agentMistakes([]*ExecutionRecord{fail(CategoryLogicError), fail(CategoryNoAgentMistake), fail(CategoryEdgeCase)}); len(got) != 2 {
		t.Fatalf("agentMistakes kept %d, want the two agent mistakes", len(got))
	}
	group := failureGroup{key: FailureGroupKey{ProblemType: "debugging", ShardType: "/coder"}}
	failures := []*ExecutionRecord{fail(CategoryLogicError), fail(CategoryLogicError), fail(CategoryLogicError)}
	off := false
	cases := []struct {
		name string
		jev  *fakeJev
		sec  nerdconfig.TypeSafeConfig
		want bool
	}{
		{"shared cause", &fakeJev{nouls: map[string]float64{learnjudge.SharesOneCause: 0.85}}, nerdconfig.TypeSafeConfig{}, true},
		{"no shared cause", &fakeJev{nouls: map[string]float64{learnjudge.SharesOneCause: 0.1}}, nerdconfig.TypeSafeConfig{}, false},
		{"unsettled", &fakeJev{nouls: map[string]float64{learnjudge.SharesOneCause: 0.5}}, nerdconfig.TypeSafeConfig{}, false},
		{"jev failing falls back to the count", &fakeJev{err: errors.New("503")}, nerdconfig.TypeSafeConfig{}, true},
		{"switch off falls back to the count", &fakeJev{nouls: map[string]float64{learnjudge.SharesOneCause: 0.1}}, nerdconfig.TypeSafeConfig{AtomChecks: &off}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pe := cascadeEvolver(t, &judgeCountingLLM{})
			pe.SetJev(tc.jev, &tc.sec)
			for i, f := range failures {
				f.TaskID = string(rune('a' + i))
				f.VerdictVersion = "v1"
			}
			group.failures = failures
			c, err := selectEvidenceCohort(context.Background(), group, *pe.config)
			if err != nil {
				t.Fatal(err)
			}
			_, got := pe.screenCohort(context.Background(), "truth-table", *pe.config, group, c, &cycleBudget{screeningCalls: pe.config.MaxCycleScreeningCalls}, &EvolutionResult{})
			if got != tc.want {
				t.Fatalf("warrants = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskRecordFor_ChangedFilesFromSuccessfulWrites(t *testing.T) {
	rec := taskRecordFor(&ExecutionRecord{AgentActions: []AgentAction{
		{Type: "read", Target: "a.go", Success: true},
		{Type: "edit", Target: "b.go", Success: true},
		{Type: "edit", Target: "b.go", Success: true},
		{Type: "write", Target: "c.go", Success: false},
	}})
	if len(rec.ChangedFiles) != 1 || rec.ChangedFiles[0] != "b.go" || len(rec.Actions) != 4 || !rec.Actions[3].Failed {
		t.Fatalf("record = %+v", rec)
	}
}
