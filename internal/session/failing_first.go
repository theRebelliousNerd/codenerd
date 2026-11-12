package session

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/logging"
)

// A test repair's recheck runs the tests that failed first.
//
// The recheck used to run the whole gate after every repair edit: the turn's
// own packages and the importers the change reaches. Live 2026-11-12, a
// self-repair fix turn on internal/core selected 6089 of 8080 importer tests
// in 30 packages, more than ten minutes a round, three rounds -- while the one
// failing test, which the turn itself had written, failed the same way in
// each. Nothing the full gate measures matters until the tests that failed
// pass: the round goes back to the model as soon as they still fail, and the
// full gate runs once they pass. A failure that predates the turn, or that did
// not reproduce, is not one the repair has to clear, so it is not rerun first:
// rerunning it would keep the full gate from ever running.

// lastFailingRun is the run the suite failed on (suiteExit's): the importer
// run when it failed, else the own run; nil when neither failed.
func lastFailingRun(result *ExecutionResult) *TestVerification {
	if result == nil || result.FinalTestCheck != nil {
		return nil
	}
	if result.ImporterCheck.Verdict() == VerifyFailed {
		return &result.ImporterCheck
	}
	if result.TestCheck.Verdict() == VerifyFailed {
		return &result.TestCheck
	}
	return nil
}

// turnFailures is every failing test of v that is the turn's to clear: not
// failing before the turn, and reproduced.
func turnFailures(v TestVerification) []TestKey {
	skip := map[TestKey]bool{}
	for _, k := range v.BaselineFailures {
		skip[k] = true
	}
	for _, k := range v.Unreproduced {
		skip[k] = true
	}
	var out []TestKey
	for _, k := range failedTestKeys(v.Result) {
		if !skip[k] {
			out = append(out, k)
		}
	}
	return out
}

// failingFirstRuns is one `go test -run` per package over the top-level tests
// keys name (a failing subtest reruns its parent).
func failingFirstRuns(keys []TestKey) []testRun {
	byPkg := map[string]map[string]bool{}
	for _, k := range keys {
		top, _, _ := strings.Cut(k.Name, "/")
		if k.Pkg == "" || top == "" {
			continue
		}
		if byPkg[k.Pkg] == nil {
			byPkg[k.Pkg] = map[string]bool{}
		}
		byPkg[k.Pkg][top] = true
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	runs := make([]testRun, 0, len(pkgs))
	for _, p := range pkgs {
		names := make([]string, 0, len(byPkg[p]))
		for n := range byPkg[p] {
			names = append(names, regexp.QuoteMeta(n))
		}
		sort.Strings(names)
		runs = append(runs, testRun{Packages: []string{p}, Args: []string{"-run", "^(" + strings.Join(names, "|") + ")$"}})
	}
	return runs
}

// recheckFailingFirst reruns the turn's failing tests. It reports whether
// they still fail, in which case it has put that run in the failing run's
// place on result, carrying its baseline so the gate attributes as before.
// A rerun that passed, or did not finish, reports false: the full gate runs.
func recheckFailingFirst(ctx context.Context, workspace string, result *ExecutionResult) (stillFailing bool, canceled bool) {
	prev := lastFailingRun(result)
	if prev == nil {
		return false, false
	}
	runs := failingFirstRuns(turnFailures(*prev))
	if len(runs) == 0 {
		return false, false
	}
	focus := verifyTestRuns(ctx, workspace, "the tests that failed, before the full gate", runs...)
	switch focus.Verdict() {
	case VerifyCanceled:
		return false, true
	case VerifyFailed:
		focus.BaselineRan, focus.BaselineFailures = prev.BaselineRan, prev.BaselineFailures
		focus.PreExistingFailures = prev.PreExistingFailures
		*prev = focus
		logging.Get(logging.CategorySession).Info(
			"repair recheck: the tests that failed still fail; the full gate waits until they pass")
		return true, false
	default:
		return false, false
	}
}
