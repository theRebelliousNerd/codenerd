package autopoiesis

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/learnjudge"
	"codenerd/internal/types"
)

// tool_value_test.go: Jev's tool-value observations become kernel evidence
// (tool_value.go, policy/jev_learning.mg tool_value_evidence).

func toolValueOrchestrator(t *testing.T, tools []types.ToolInfo) *Orchestrator {
	t.Helper()
	o := toolNeedOrchestrator(t)
	mock := &MockToolSynthesizer{ListToolsFunc: func() []types.ToolInfo { return tools }}
	o.ouroboros = mock
	return o
}

var valueTools = []types.ToolInfo{
	{Name: "json_check", Description: "Validates that a file is well-formed JSON."},
	{Name: "json_lint", Description: "Checks a JSON file parses and reports the first syntax error."},
	{Name: "line_count", Description: "Counts the lines in a file."},
}

func TestToolValue_PerCallTruthTable(t *testing.T) {
	o := toolValueOrchestrator(t, nil)
	o.profiles = NewProfileStore(t.TempDir())
	c, _ := toolNeedJev(t, nil, 0)
	o.SetJev(c, &internalconfig.TypeSafeConfig{})
	now := time.Now()
	call := func(tool, id string, p float64) {
		o.recordCallHit(&ExecutionFeedback{ToolName: tool, ExecutionID: id, Timestamp: now}, p)
	}
	call("lands", "1", 0.9)
	call("lands", "2", 0.5) // unsettled counts as neither
	call("misses", "1", 0.1)
	call("mixed", "1", 0.9)
	call("mixed", "2", 0.1)
	call("mostly", "1", 0.9)
	call("mostly", "2", 0.9)
	call("mostly", "3", 0.1)
	got, err := readToolValueEvidence(o.kernel)
	if err != nil {
		t.Fatal(err)
	}
	// Hits weigh against misses: a tie names neither side, the larger side
	// names the evidence.
	want := []ToolValueEvidence{
		{Tool: "lands", Evidence: []string{"/hits_mark"}},
		{Tool: "misses", Evidence: []string{"/misses_mark"}},
		{Tool: "mostly", Evidence: []string{"/hits_mark"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("evidence = %+v, want %+v", got, want)
	}
}

func TestJudgeToolValue_DuplicateAndHarness(t *testing.T) {
	o := toolValueOrchestrator(t, valueTools)
	o.profiles = NewProfileStore(t.TempDir())
	c, calls := toolNeedJev(t, map[string]float64{learnjudge.QAnyToolExists: 0.9, learnjudge.HarnessCoversName: 0.85}, 0)
	o.SetJev(c, &internalconfig.TypeSafeConfig{})
	got, err := o.JudgeToolValue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != int64(2*len(valueTools)) {
		t.Fatalf("calls = %d, want one harness and one duplicate check per tool", calls.Load())
	}
	if len(got) != len(valueTools) {
		t.Fatalf("evidence = %+v", got)
	}
	for _, e := range got {
		if !reflect.DeepEqual(e.Evidence, []string{"/duplicate", "/harness_covers"}) {
			t.Fatalf("%s evidence = %v", e.Tool, e.Evidence)
		}
	}
	// The verdicts outlive the process: a fresh kernel on the same ledger
	// derives the same evidence without asking Jev again.
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	reboot := &Orchestrator{profiles: NewProfileStore(o.profiles.storePath), toolGen: NewToolGenerator(nil, t.TempDir())}
	reboot.SetKernel(k)
	again, err := readToolValueEvidence(k)
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("evidence after a restart = %+v (err %v), want %+v", again, err, got)
	}
}

// No answer is no evidence: a failing Jev, the switch off and no Jev derive
// nothing, and never a "no".
func TestJudgeToolValue_NoAnswerNoEvidence(t *testing.T) {
	off := false
	failing, _ := toolNeedJev(t, nil, http.StatusServiceUnavailable)
	quiet, quietCalls := toolNeedJev(t, map[string]float64{learnjudge.HarnessCoversName: 0.99}, 0)
	for _, tc := range []struct {
		name  string
		setup func(o *Orchestrator)
	}{
		{"failing", func(o *Orchestrator) { o.SetJev(failing, &internalconfig.TypeSafeConfig{}) }},
		{"switch off", func(o *Orchestrator) { o.SetJev(quiet, &internalconfig.TypeSafeConfig{ToolChecks: &off}) }},
		{"no jev", func(o *Orchestrator) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := toolValueOrchestrator(t, valueTools)
			tc.setup(o)
			got, err := o.JudgeToolValue(context.Background())
			if err != nil || len(got) != 0 {
				t.Fatalf("evidence = %+v err %v, want none", got, err)
			}
		})
	}
	if quietCalls.Load() != 0 {
		t.Fatal("tool_checks off still asked Jev")
	}
}
