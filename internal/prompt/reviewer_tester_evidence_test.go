package prompt

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// This is the shipped corpus through the real Mangle compiler and renderer,
// not a YAML-selector assertion or a model's claim that it verified behavior.
func TestReviewerTesterHostEvidenceCompiled(t *testing.T) {
	compiler, kernel := newEmbeddedCorpusCompiler(t)
	kernel.SetWorkspace(t.TempDir())
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	guidance, ok := corpus.Get("system/change_evidence")
	if !ok || !guidance.IsMandatory || len(guidance.RequiresTools) != 0 || !guidance.Importance.Scored() {
		t.Fatalf("missing scored, tool-agnostic mandatory host-evidence guidance: %+v", guidance)
	}
	for _, required := range []string{
		"passing checks, and verified behavior", "successful write is not acceptance",
		"host's current acceptance report", "timed-out, or stale checks remain unverified",
		"Preserve caller-supplied", "Never change or weaken them",
		"Later edits invalidate earlier results",
	} {
		if !strings.Contains(guidance.Content, required) {
			t.Fatalf("host-evidence contract lost %q", required)
		}
	}
	cases := []struct {
		name, shard, intent string
		tools               []string
		want                bool
	}{
		{"review", "reviewer", "/review", nil, true},
		{"test", "tester", "/test", nil, true},
		{"unrelated research", "researcher", "/research", nil, false},
		{"audit", "reviewer", "/audit", nil, true},
		{"verify", "tester", "/verify", nil, true},
		{"unrelated explanation", "coder", "/explain", nil, false},
		{"inspect", "reviewer", "/inspect", nil, true},
		{"analyze", "reviewer", "/analyze", nil, true},
		{"review check", "reviewer", "/check", nil, true},
		{"test check", "tester", "/check", nil, true},
		{"cover", "tester", "/cover", nil, true},
		{"validate", "tester", "/validate", nil, true},
		{"review with tools", "reviewer", "/review", []string{"read_file"}, true},
		{"test with tools", "tester", "/test", []string{"read_file", "run_tests"}, true},
		{"existing fix", "coder", "/fix", nil, true},
		{"existing create", "coder", "/create", nil, true},
		{"existing refactor", "coder", "/refactor", nil, true},
		{"existing implement", "coder", "/implement", nil, true},
		{"unrelated plan", "planner", "/plan", nil, false},
		{"review after negatives", "reviewer", "/review", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cc := NewCompilationContextWithBudget(32768)
			cc.ShardType, cc.ShardID, cc.IntentVerb = "/"+tc.shard, tc.shard, tc.intent
			cc.IntentTarget, cc.Language = "component.go", "/go"
			cc.OperationalMode = "/active"
			cc.AvailableTools = slices.Clone(tc.tools)
			before := slices.Clone(cc.AvailableTools)
			included, result := compiledAtoms(t, compiler, cc)
			atom, selected := included[guidance.ID]
			rendered := strings.Contains(result.Prompt, guidance.Content)
			if selected != tc.want || rendered != tc.want {
				t.Fatalf("%s/%s host-evidence guidance: selected=%t rendered=%t want=%t", tc.shard, tc.intent, selected, rendered, tc.want)
			}
			if selected && (atom.ContentHash != guidance.ContentHash || atom.Importance != guidance.Importance) {
				t.Fatal("compilation substituted or rescored the existing evidence contract")
			}
			// Reuse the existing role protocols: contract-derived normal,
			// boundary and error behavior for testing; reachability and an
			// explicit uncertain verdict for review. No new shard is needed.
			var behavior []string
			if tc.want && tc.shard == "tester" {
				behavior = []string{"PHASE 1: CONTRACT DISCOVERY", "POSTCONDITIONS", "PHASE 2: BOUNDARY ANALYSIS", "ERROR values", "NIL/NULL cases"}
			}
			if tc.want && tc.shard == "reviewer" {
				behavior = []string{"Why the issue is reachable", "What input would trigger the issue", "UNCERTAIN VERDICT"}
			}
			for _, required := range behavior {
				if !strings.Contains(result.Prompt, required) {
					t.Fatalf("existing %s behavior protocol omitted %q", tc.shard, required)
				}
			}
			if !reflect.DeepEqual(cc.AvailableTools, before) {
				t.Fatal("evidence guidance changed the effective capability envelope")
			}
		})
	}
}
