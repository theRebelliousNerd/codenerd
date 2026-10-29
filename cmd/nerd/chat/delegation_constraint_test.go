package chat

import (
	"strings"
	"testing"

	"codenerd/internal/perception"
)

// A follow-on request carries prior shard evidence into the task. The user's
// current limit ("keep the public API") must ride along with it, not be
// replaced by it: each prior-result branch used to return its own string and
// drop the constraint the base task had.
func TestFormatShardTaskWithContext_EveryPriorResultBranchKeepsTheConstraint(t *testing.T) {
	const constraint = "preserve the public API"
	findings := []map[string]any{{"file": "a.go", "severity": "medium", "message": "rename"}}
	reviewer := &ShardResult{ShardType: "reviewer", Findings: findings}
	tester := &ShardResult{ShardType: "tester", RawOutput: "--- FAIL: TestX"}
	coder := &ShardResult{ShardType: "coder", RawOutput: "edited a.go"}

	cases := []struct {
		name  string
		verb  string
		prior *ShardResult
		want  string
	}{
		{"fix after review", "/fix", reviewer, "findings:["},
		{"fix after test failure", "/fix", tester, "test_errors:["},
		{"refactor after review", "/refactor", reviewer, "suggestions:["},
		{"test after fix", "/test", coder, "after_fix context:["},
		{"debug after test", "/debug", tester, "context:["},
	}
	var m Model
	for _, tc := range cases {
		task := m.formatShardTaskWithContext(tc.verb, "a.go", constraint, t.TempDir(), tc.prior)
		if !strings.Contains(task, tc.want) {
			t.Fatalf("%s: the prior evidence is not in the task: %q", tc.name, task)
		}
		if !strings.HasSuffix(task, "with constraint: "+constraint) {
			t.Errorf("%s: the current constraint was replaced by prior evidence: %q", tc.name, task)
		}
	}
}

// No constraint, no clause: the unconstrained task is unchanged.
func TestFormatShardTaskWithContext_NoConstraintAddsNoClause(t *testing.T) {
	var m Model
	task := m.formatShardTaskWithContext("/debug", "a.go", "none", t.TempDir(), &ShardResult{ShardType: "tester", RawOutput: "x"})
	if strings.Contains(task, "constraint") {
		t.Errorf("an empty constraint added a clause: %q", task)
	}
}

// Work routed from an intent hands the executor its target and constraint as
// fields, as the verified lane does; "none" is no constraint.
func TestIntentTaskRequest_CarriesTargetAndConstraint(t *testing.T) {
	req := intentTaskRequest("coder", "fix the parser", perception.Intent{Target: "internal/parse.go", Constraint: "keep the API"}, "fix the parser, it drops the last token")
	if req.IntentVerb != "coder" || req.Task != "fix the parser" || req.Target != "internal/parse.go" || req.Constraint != "keep the API" || req.Request != "fix the parser, it drops the last token" {
		t.Fatalf("request = %+v", req)
	}
	if req := intentTaskRequest("coder", "x", perception.Intent{Constraint: "none"}, ""); req.Constraint != "" {
		t.Fatalf(`"none" became constraint %q`, req.Constraint)
	}
}

// A verb pair chain takes the verbs of the pair the intent's verb starts.
func TestAnchorVerbPairToIntent_TakesTheIntentsPair(t *testing.T) {
	pattern := &MultiStepPattern{Decomposer: "verb_pair_chain", VerbPairs: []verbPair{{"/create", "/test"}, {"/fix", "/test"}}}
	steps := anchorVerbPairToIntent([]TaskStep{{Verb: "/create"}, {Verb: "/test"}}, pattern, perception.Intent{Verb: "/fix", Target: "auth.go"}, "")
	if steps[0].Verb != "/fix" || steps[1].Verb != "/test" || steps[0].Target != "auth.go" {
		t.Fatalf("steps = %+v", steps)
	}
	three := []TaskStep{{Verb: "/a"}, {Verb: "/b"}, {Verb: "/c"}}
	if got := anchorVerbPairToIntent(three, pattern, perception.Intent{Verb: "/fix", Target: "auth.go"}, ""); got[0].Target != "" {
		t.Fatalf("a three-step chain was anchored as a pair: %+v", got)
	}
}
