package autopoiesis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/learnjudge"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// tool_need_test.go: whether a request is a capability gap is the kernel's
// decision over Jev's judgments (tool_need.go, policy/jev_learning.mg); the
// capability pattern counts only when Jev gave none, and a failed attempt
// alone never does.

// toolNeedJev answers nouls from answers (0.5 when unlisted) and every Choice
// with its first option; status fails every call.
func toolNeedJev(t *testing.T, answers map[string]float64, status int) (*typesafe.Client, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		var req struct {
			Questions map[string]struct {
				Type     string         `json:"type"`
				Criteria map[string]any `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		out := map[string]any{}
		for name, q := range req.Questions {
			if q.Type == "choice" {
				probs := map[string]float64{}
				pick := ""
				for opt := range q.Criteria {
					if pick == "" || opt < pick {
						pick = opt
					}
					probs[opt] = 0.01
				}
				probs[pick] = 0.9
				out[name] = map[string]any{"type": "choice", "choice": pick, "confidence": 0.9, "probabilities": probs}
				continue
			}
			p, ok := answers[name]
			if !ok {
				p = 0.5
			}
			out[name] = map[string]any{"type": "noul", "noul": p}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-test", "answers": out, "usage": map[string]any{"input_tokens": 10, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-test", BaseURL: srv.URL, APIKey: "k", Timeout: 2e9, Retry: &typesafe.RetryPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls
}

func toolNeedOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{kernel: k, toolGen: NewToolGenerator(nil, t.TempDir())}
	return o
}

func TestToolGap_KernelTruthTable(t *testing.T) {
	off := false
	const patterned = "I need a tool to validate JSON"
	const plain = "convert every .avro file under data/ to CSV"
	cases := []struct {
		name      string
		input     string
		failed    string
		answers   map[string]float64
		status    int
		sec       *internalconfig.TypeSafeConfig
		noJev     bool
		wantGap   bool
		wantSrc   string
		wantCalls int64
	}{
		{name: "explicit ask", input: plain, answers: map[string]float64{learnjudge.AsksForTool: 0.9, learnjudge.NeedsMissingOp: 0.2}, wantGap: true, wantSrc: "jev", wantCalls: 1},
		{name: "failure for a missing operation", input: plain, failed: "no tool can read Avro", answers: map[string]float64{learnjudge.AsksForTool: 0.1, learnjudge.NeedsMissingOp: 0.85, learnjudge.FailedForMissingTool: 0.9}, wantGap: true, wantSrc: "jev", wantCalls: 1},
		{name: "failure for another reason", input: plain, failed: "context deadline exceeded", answers: map[string]float64{learnjudge.AsksForTool: 0.1, learnjudge.NeedsMissingOp: 0.85, learnjudge.FailedForMissingTool: 0.05}, wantCalls: 1},
		{name: "missing operation without a failure", input: plain, answers: map[string]float64{learnjudge.AsksForTool: 0.1, learnjudge.NeedsMissingOp: 0.9}, wantCalls: 1},
		{name: "jev no outranks the pattern", input: patterned, answers: map[string]float64{learnjudge.AsksForTool: 0.2, learnjudge.NeedsMissingOp: 0.1}, wantCalls: 1},
		{name: "jev failing: the pattern decides", input: patterned, status: http.StatusServiceUnavailable, wantGap: true, wantSrc: "pattern", wantCalls: 1},
		{name: "jev failing and no pattern", input: plain, failed: "boom", status: http.StatusServiceUnavailable, wantCalls: 1},
		{name: "switch off: the pattern decides", input: patterned, answers: map[string]float64{learnjudge.AsksForTool: 0.01}, sec: &internalconfig.TypeSafeConfig{ToolChecks: &off}, wantGap: true, wantSrc: "pattern"},
		{name: "no jev: a failed attempt alone is no gap", input: "try again", failed: "Previous tool execution failed", noJev: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := toolNeedOrchestrator(t)
			var calls *atomic.Int64
			if !tc.noJev {
				var c *typesafe.Client
				c, calls = toolNeedJev(t, tc.answers, tc.status)
				sec := tc.sec
				if sec == nil {
					sec = &internalconfig.TypeSafeConfig{}
				}
				o.SetJev(c, sec)
			}
			got := o.decideToolGap(context.Background(), tc.input, tc.failed)
			if got.gap != tc.wantGap || (tc.wantGap && got.source != tc.wantSrc) {
				t.Fatalf("decision = %+v, want gap %v from %q", got, tc.wantGap, tc.wantSrc)
			}
			if calls != nil && calls.Load() != tc.wantCalls {
				t.Fatalf("jev calls = %d, want %d", calls.Load(), tc.wantCalls)
			}
			// The evidence leaves with the decision.
			for _, pred := range []string{toolNeedJudgmentPred, toolNeedPatternPred} {
				if rows, _ := o.kernel.Query(pred); len(rows) != 0 {
					t.Fatalf("%s left in the kernel: %v", pred, rows)
				}
			}
		})
	}
}

// DetectToolNeed asks the LLM for a spec only after the kernel derives a gap.
func TestDetectToolNeed_LLMWritesTheSpecOnlyForAGap(t *testing.T) {
	var llmCalls atomic.Int64
	client := &MockLLMClient{CompleteWithSystemFunc: func(context.Context, string, string) (string, error) {
		llmCalls.Add(1)
		return `{"tool_name": "avro_to_csv", "purpose": "convert Avro to CSV", "input_type": "string", "output_type": "string", "priority": 0.8, "confidence": 0.8, "reasoning": "no Avro reader"}`, nil
	}}
	o := toolNeedOrchestrator(t)
	o.toolGen = NewToolGenerator(client, t.TempDir())
	o.toolGen.SetRolePrompts(fixedRolePrompts{})
	o.SetKernel(o.kernel)
	c, _ := toolNeedJev(t, map[string]float64{learnjudge.AsksForTool: 0.05, learnjudge.NeedsMissingOp: 0.3}, 0)
	o.SetJev(c, &internalconfig.TypeSafeConfig{})
	if need, err := o.toolGen.DetectToolNeed(context.Background(), "fix the nil pointer in executor.go", "test failed: nil dereference"); err != nil || need != nil || llmCalls.Load() != 0 {
		t.Fatalf("need %+v err %v llm calls %d, want none", need, err, llmCalls.Load())
	}
	c2, _ := toolNeedJev(t, map[string]float64{learnjudge.AsksForTool: 0.95}, 0)
	o.SetJev(c2, &internalconfig.TypeSafeConfig{})
	need, err := o.toolGen.DetectToolNeed(context.Background(), "build me a tool that converts Avro files to CSV", "")
	if err != nil || need == nil || need.Name != "avro_to_csv" || llmCalls.Load() != 1 {
		t.Fatalf("need %+v err %v llm calls %d", need, err, llmCalls.Load())
	}
}

// newRecordingRealKernel is a real kernel; tests read what was asserted
// through Query.
func newRecordingRealKernel(t *testing.T) *recordingKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	return &recordingKernel{RealKernel: k}
}

// recordingKernel keeps every fact asserted one at a time, beside the real
// kernel's behavior.
type recordingKernel struct {
	*core.RealKernel
	AssertedFacts []types.Fact
}

func (k *recordingKernel) Assert(f types.Fact) error {
	k.AssertedFacts = append(k.AssertedFacts, f)
	return k.RealKernel.Assert(f)
}
