package session

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/gates"
	"codenerd/internal/tools"
)

// sequencedTestRun is a test process the tool layer recorded, with its place
// on the turn's clock. The kernel keeps only the seq and the exit.
type sequencedTestRun struct {
	Seq int
	Run tools.TestRun
}

// obligationOutcome is what the host measured for one obligation of the
// workspace's test plan; the policy (coder_safety.mg turn_ob_state) decides
// what each means for the /test_run gate.
type obligationOutcome string

const (
	// outcomePassed: the check ran, executed tests and passed.
	outcomePassed obligationOutcome = "/passed"
	// outcomeFailed: the check ran and its exit is not a pass.
	outcomeFailed obligationOutcome = "/failed"
	// outcomeUnavailable: the workspace declares the check and it cannot run
	// (its program is missing, or it would not start).
	outcomeUnavailable obligationOutcome = "/unavailable"
	// outcomeIndeterminate: the run was stopped before it gave a verdict.
	outcomeIndeterminate obligationOutcome = "/indeterminate"
	// outcomeNoTests: the check ran, failed nothing and collected no test.
	outcomeNoTests obligationOutcome = "/no_tests"
	// outcomeUncovered: a language this turn wrote has no declared test check.
	outcomeUncovered obligationOutcome = "/uncovered"
)

// plannedCheck is one thing the workspace's declared checks owe for this
// turn's writes: a gate over a node, or a written language no gate covers.
type plannedCheck struct {
	ID          string
	Gate        gates.Gate
	Node        string
	Argv        []string
	Unavailable string // why the check cannot run; "" when it can
	Language    string // set for an uncovered-language obligation
}

// obligationRun is the latest outcome of one planned check.
type obligationRun struct {
	ID      string
	Outcome obligationOutcome
	Argv    []string
	Exit    int
	Reason  string
	Reused  bool // an identical process the turn already ran satisfied it
}

// testPlan is the checks the workspace's own test gates owe for a turn's
// non-Go writes. Discovery availability does not shrink it: a declared check
// that cannot run is still owed.
type testPlan struct {
	checks []plannedCheck
}

// buildTestPlan plans the workspace's applicable test checks for the written
// languages and directories. It returns no plan when the workspace declares no
// applicable test check at all: the turn's gate is then the test process the
// model started, as before.
func buildTestPlan(set gates.Set, langs map[string]bool, dirs []string) testPlan {
	var plan testPlan
	covered := map[string]bool{}
	allCovered := false
	add := func(g gates.Gate, reason string) {
		nodes := []string{""}
		if g.Scope == gates.ScopeNode {
			nodes = dirs
		}
		for _, node := range nodes {
			label := node
			if label == "" {
				label = "."
			}
			argv := g.Argv
			if g.Scope == gates.ScopeNode {
				argv = g.ForNode(node)
			}
			plan.checks = append(plan.checks, plannedCheck{
				ID: g.ID + "@" + label, Gate: g, Node: node,
				Argv: slices.Clone(argv), Unavailable: reason,
			})
		}
		if g.Language == "" {
			allCovered = true
		}
		covered[g.Language] = true
	}
	applicable := func(g gates.Gate) bool {
		return g.Kind == gates.Test && g.Language != "go" && (g.Language == "" || langs[g.Language])
	}
	for _, g := range set.Gates {
		if applicable(g) {
			add(g, "")
		}
	}
	for _, u := range set.Unavailable {
		if applicable(u.Gate) {
			add(u.Gate, u.Reason)
		}
	}
	if len(plan.checks) == 0 || allCovered {
		return plan
	}
	// A written language with no declared check is not covered by another
	// language's pass; only a test process the turn started can speak for it.
	missing := make([]string, 0, len(langs))
	for lang := range langs {
		if !covered[lang] {
			missing = append(missing, lang)
		}
	}
	slices.Sort(missing)
	for _, lang := range missing {
		plan.checks = append(plan.checks, plannedCheck{ID: "uncovered:" + lang, Language: lang})
	}
	return plan
}

// classifyRun is the outcome of a gate run the host observed.
func classifyRun(ctx context.Context, g gates.Gate, res gates.Result) (obligationOutcome, string) {
	switch {
	case res.Unverified():
		if ctx.Err() != nil {
			return outcomeIndeterminate, res.Err.Error()
		}
		return outcomeUnavailable, res.Err.Error()
	case !res.Passed:
		return outcomeFailed, ""
	case g.NoTests(res.ExitCode):
		return outcomeNoTests, ""
	}
	return outcomePassed, ""
}

// classifyExit is classifyRun for a process the model's tool started.
func classifyExit(g gates.Gate, exit int) obligationOutcome {
	switch {
	case !g.Passed(exit):
		return outcomeFailed
	case g.NoTests(exit):
		return outcomeNoTests
	}
	return outcomePassed
}

// matchingReceipt is the latest test process the turn started after its last
// write whose command is exactly the planned check's. Matching a command is
// the reference rule for "this process satisfies that obligation"; a process
// that is merely a passing test is not one.
func matchingReceipt(result *ExecutionResult, argv []string) (sequencedTestRun, bool) {
	for i := len(result.testReceipts) - 1; i >= 0; i-- {
		r := result.testReceipts[i]
		if r.Seq > result.lastWriteSeq && slices.Equal(r.Run.Argv, argv) {
			return r, true
		}
	}
	return sequencedTestRun{}, false
}

// obligationOpen reports whether the model can do something about the
// obligation: a check that cannot run is not cleared by more test runs.
func obligationOpen(o obligationRun) bool {
	return o.Outcome != outcomePassed && o.Outcome != outcomeUnavailable && o.Outcome != outcomeIndeterminate
}

// planShortfall words what the plan still owes, from the host's own record.
func planShortfall(result *ExecutionResult) string {
	var parts []string
	for _, o := range result.testPlan {
		cmd := "`" + strings.Join(o.Argv, " ") + "`"
		switch o.Outcome {
		case outcomeFailed:
			parts = append(parts, fmt.Sprintf("the workspace's check %s failed (%s exited %d)", o.ID, cmd, o.Exit))
		case outcomeUnavailable:
			parts = append(parts, fmt.Sprintf("the workspace's check %s could not run (%s)", o.ID, o.Reason))
		case outcomeIndeterminate:
			parts = append(parts, fmt.Sprintf("the workspace's check %s was stopped before it gave a verdict", o.ID))
		case outcomeNoTests:
			parts = append(parts, fmt.Sprintf("the workspace's check %s ran %s and collected no test, so nothing it ran exercises the files written", o.ID, cmd))
		case outcomeUncovered:
			parts = append(parts, fmt.Sprintf("the workspace declares no test check for %s files, so only a passing test process you start after your last write can cover them", strings.TrimPrefix(o.ID, "uncovered:")))
		}
	}
	return strings.Join(parts, "; ")
}
