package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codenerd/internal/build"
	"codenerd/internal/logging"
	"codenerd/internal/testfacts"
)

// attributeTestFailures compares the post-edit verification result against the
// pre-turn state. Failures that also occur without this turn's edits are
// pre-existing and must not be charged to the turn.
func attributeTestFailures(ctx context.Context, workspace string, packages []string, writtenPaths []string, preWrite map[string]PreImage, head TestVerification) TestVerification {
	if head.Outcome != VerifyFailed {
		return head
	}
	workspace = goWorkspace(workspace)
	// Full sanitized names, subtests included. The -run below is still the
	// parents: Go splits a -run regexp on '/', so "TestX/case_one" would
	// not select that subtest.
	headFailed := failedTestKeys(head.Result)
	if len(headFailed) == 0 {
		logging.SessionDebug("test gate: no baseline attribution: no failed test names in the head run's Result")
		return head
	}
	if len(packages) == 0 {
		logging.SessionDebug("test gate: no baseline attribution: no packages")
		return head
	}
	if len(preWrite) == 0 {
		logging.SessionDebug("test gate: no baseline attribution: no pre-write snapshots (%d written paths)", len(writtenPaths))
		return head
	}
	for _, p := range writtenPaths {
		key := canonicalizeWrittenPath(p, workspace)
		if key == "" {
			key = p
		}
		if _, ok := preWrite[key]; !ok {
			if _, ok := preWrite[p]; !ok {
				logging.SessionDebug("test gate: no baseline attribution: missing pre-write snapshot for %q", p)
				return head
			}
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		logging.SessionDebug("test gate: no baseline attribution: context canceled: %v", ctx.Err())
		return head
	}
	tmpDir, overlayPath, _, err := buildTestOverlay(workspace, preWrite)
	if err != nil {
		logging.SessionDebug("test gate: no baseline attribution: overlay could not be built: %v", err)
		return head
	}
	defer os.RemoveAll(tmpDir)
	baseline, baselineOutcome := runSelectedTests(ctx, workspace, overlayPath, baselineRunRegex(failedTopLevels(head.Result)), packages)
	if baselineOutcome != VerifyPassed && baselineOutcome != VerifyFailed {
		logging.SessionDebug("test gate: no baseline attribution: baseline run inconclusive (outcome %s)", baselineOutcome)
		return head
	}
	// The full baseline set, not the intersection. A name the head did not
	// fail does not become a failure; the /test rule charges a head name
	// only when it is absent here.
	head.BaselineRan = true
	head.BaselineFailures = failedTestKeys(baseline)
	preExisting := partitionPreExisting(headFailed, baseline)
	head = noteUnreproduced(head, rerunNewFailures(ctx, workspace, packages, headFailed, preExisting))
	if len(preExisting) == 0 {
		logging.SessionDebug("test gate: no baseline attribution: every failure is new to this turn (%d failed)", len(headFailed))
		return head
	}
	return notePreExisting(head, preExisting, len(preExisting) == len(headFailed))
}

// rerunNewFailures runs the failures new to this turn once more on the
// edited tree and returns those that ran and passed. A test that fails once
// and passes on the same code is flaky; charging it sent a campaign coder
// through three repair rounds for a timing test its one-file edit could not
// reach (2026-11-07). A rerun that does not finish proves nothing: none.
func rerunNewFailures(ctx context.Context, workspace string, packages []string, headFailed, preExisting []TestKey) []TestKey {
	old := make(map[TestKey]bool, len(preExisting))
	for _, k := range preExisting {
		old[k] = true
	}
	var fresh []TestKey
	tops := map[string]bool{}
	var topNames []string
	for _, k := range headFailed {
		if old[k] {
			continue
		}
		fresh = append(fresh, k)
		top := strings.SplitN(k.Name, "/", 2)[0]
		if !tops[top] {
			tops[top] = true
			topNames = append(topNames, top)
		}
	}
	if len(fresh) == 0 || errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	rerun, outcome := runSelectedTests(ctx, workspace, "", baselineRunRegex(topNames), packages)
	if outcome != VerifyPassed && outcome != VerifyFailed {
		logging.SessionDebug("test gate: rerun of new failures inconclusive (outcome %s)", outcome)
		return nil
	}
	passed := passedTestKeys(rerun)
	var unreproduced []TestKey
	for _, k := range fresh {
		if passed[k] {
			unreproduced = append(unreproduced, k)
		}
	}
	return unreproduced
}

// noteUnreproduced records the failures that did not reproduce and names
// them for the model and the operator. It does not change the outcome.
func noteUnreproduced(head TestVerification, unreproduced []TestKey) TestVerification {
	if len(unreproduced) == 0 {
		return head
	}
	labels := make([]string, len(unreproduced))
	for i, k := range unreproduced {
		labels[i] = k.String()
	}
	logging.Get(logging.CategorySession).Warn("test gate: %d failure(s) passed when the edited tree ran them again (flaky), not charged to the turn: %s", len(labels), strings.Join(labels, ", "))
	head.Unreproduced = unreproduced
	head.Output = "Flaky (failed once, passed when run again on the same code; not yours to fix): " + strings.Join(labels, ", ") + "\n" + head.Output
	return head
}

// notePreExisting names the failures that also fail without this turn's
// edits. It does not change the outcome: the head run failed, and whether
// that failure is the turn's is the /test rule's.
func notePreExisting(head TestVerification, preExisting []TestKey, all bool) TestVerification {
	labels := make([]string, len(preExisting))
	for i, k := range preExisting {
		labels[i] = k.String()
	}
	if all {
		logging.Get(logging.CategorySession).Warn("test gate: %d failure(s) also fail before this turn's edits (pre-existing), not charged to the turn: %s", len(labels), strings.Join(labels, ", "))
	}
	head.PreExistingFailures = labels
	head.Output = "Pre-existing failures (also fail without this turn's edits; not yours to fix): " + strings.Join(labels, ", ") + "\n" + head.Output
	return head
}

// noFailureIsTheTurns reports whether every named failure is one the /test
// rule does not charge to the turn: it failed before the turn (the baseline),
// or it failed once and passed when the edited tree ran it again (flaky,
// Unreproduced) -- the same exemption turn_test_unreproduced gives in
// policy/coder_safety.mg and turnFailures applies. gateTests uses it to keep
// measuring importers, the uplift recheck to decide whether an uplift broke
// the tests. It does not set the outcome. An unnamed failure is the turn's,
// and a baseline that did not run attributes nothing.
//
// It used to exempt the baseline only. Live 2026-11-12 a flaky failure the
// rerun had cleared still read as the turn's here: the importer run was
// canceled ("the turn's own tests failed first") and an uplift round whose
// code was green was undone as having broken the tests.
func noFailureIsTheTurns(v TestVerification) bool {
	if !v.BaselineRan {
		return false
	}
	if len(failedTestKeys(v.Result)) == 0 {
		return false
	}
	return len(turnFailures(v)) == 0
}

func partitionPreExisting(headFailed []TestKey, baseline *testfacts.Result) []TestKey {
	baselineSet := make(map[TestKey]bool, len(headFailed))
	for _, k := range failedTestKeys(baseline) {
		baselineSet[k] = true
	}
	var preExisting []TestKey
	for _, k := range headFailed {
		if baselineSet[k] {
			preExisting = append(preExisting, k)
		}
	}
	return preExisting
}

func baselineRunRegex(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, n := range names {
		quoted = append(quoted, regexp.QuoteMeta(n))
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

// runSelectedTests runs the tests runArg selects in packages: through
// overlayPath (the pre-turn files) for the baseline, or on the tree as it
// stands when overlayPath is empty.
func runSelectedTests(ctx context.Context, workspace, overlayPath, runArg string, packages []string) (*testfacts.Result, VerifyOutcome) {
	args := []string{"test", "-json"}
	if overlayPath != "" {
		args = append(args, "-overlay", overlayPath)
	}
	args = append(args, "-count=1", "-run", runArg)
	args = append(args, packages...)
	out, outcome, _ := runVerificationCommand(ctx, workspace, build.GetBuildEnv(nil, workspace), testVerifyTimeout, "go", args, verifyTestRunner)
	switch outcome {
	case VerifyPassed, VerifyFailed:
		return parseTestJSON(workspace, out), outcome
	default:
		return nil, outcome
	}
}

// buildTestOverlay writes a go build overlay that puts each written file back
// as it was before the turn. It returns the overlay's directory (the caller
// removes it), the overlay file, and the replacements: each written file's
// absolute path mapped to the file standing in for it, "" for one the turn
// created.
func buildTestOverlay(workspace string, preWrite map[string]PreImage) (string, string, map[string]string, error) {
	tmpDir, err := os.MkdirTemp("", "test-baseline-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create baseline overlay temp dir: %w", err)
	}
	replace, err := writeOverlayFiles(tmpDir, workspace, preWrite)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", nil, err
	}
	overlayBytes, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", nil, fmt.Errorf("marshal baseline overlay: %w", err)
	}
	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlayBytes, 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", nil, fmt.Errorf("write baseline overlay %s: %w", overlayPath, err)
	}
	return tmpDir, overlayPath, replace, nil
}

// writeOverlayFiles maps each written path back to its preimage: absent
// before the turn stays absent in the baseline, anything that existed --
// empty included -- is its old bytes. A path whose preimage is unknown has no
// baseline, and the overlay is refused rather than guessed.
func writeOverlayFiles(tmpDir, workspace string, preWrite map[string]PreImage) (map[string]string, error) {
	replace := make(map[string]string, len(preWrite))
	idx := 0
	for key, pre := range preWrite {
		if !pre.Known() {
			return nil, fmt.Errorf("no baseline for %s: its preimage is unknown (%s)", key, pre.Unknown)
		}
		// The key go matches is the file as go spells it (go_paths.go): an
		// alias-spelled key is an overlay go ignores, and a baseline run
		// that silently measures the tree as it is now.
		abs := goOverlayKey(workspace, key)
		if !pre.Existed {
			replace[abs] = ""
			continue
		}
		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("overlay-%d.go", idx))
		idx++
		if err := os.WriteFile(tmpFile, []byte(pre.Content), 0644); err != nil {
			return nil, fmt.Errorf("write baseline overlay file %s: %w", tmpFile, err)
		}
		replace[abs] = tmpFile
	}
	return replace, nil
}
