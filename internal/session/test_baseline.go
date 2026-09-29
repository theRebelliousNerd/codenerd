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
	headFailed := failedTestNames(head.Result)
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
	baseline, baselineOutcome := runBaselineTests(ctx, workspace, overlayPath, baselineRunRegex(failedTopLevels(head.Result)), packages)
	if baselineOutcome != VerifyPassed && baselineOutcome != VerifyFailed {
		logging.SessionDebug("test gate: no baseline attribution: baseline run inconclusive (outcome %s)", baselineOutcome)
		return head
	}
	// The full baseline set, not the intersection. A name the head did not
	// fail does not become a failure; the /test rule charges a head name
	// only when it is absent here.
	head.BaselineRan = true
	head.BaselineFailures = failedTestNames(baseline)
	preExisting := partitionPreExisting(headFailed, baseline)
	if len(preExisting) == 0 {
		logging.SessionDebug("test gate: no baseline attribution: every failure is new to this turn (%d failed)", len(headFailed))
		return head
	}
	return notePreExisting(head, preExisting, len(preExisting) == len(headFailed))
}

// notePreExisting names the failures that also fail without this turn's
// edits. It does not change the outcome: the head run failed, and whether
// that failure is the turn's is the /test rule's.
func notePreExisting(head TestVerification, preExisting []string, all bool) TestVerification {
	if all {
		logging.Get(logging.CategorySession).Warn("test gate: %d failure(s) also fail before this turn's edits (pre-existing), not charged to the turn: %s", len(preExisting), strings.Join(preExisting, ", "))
	}
	head.PreExistingFailures = preExisting
	head.Output = "Pre-existing failures (also fail without this turn's edits; not yours to fix): " + strings.Join(preExisting, ", ") + "\n" + head.Output
	return head
}

// failuresAllPredate reports whether every named failure also failed before
// the turn. gateTests uses it only to keep measuring importers, so a new
// importer failure is still recorded when the turn's own failures are
// entirely the baseline's. It does not set the outcome. An unnamed failure
// is the turn's, and a baseline that did not run attributes nothing.
func failuresAllPredate(v TestVerification) bool {
	if !v.BaselineRan {
		return false
	}
	names := failedTestNames(v.Result)
	if len(names) == 0 {
		return false
	}
	before := make(map[string]bool, len(v.BaselineFailures))
	for _, name := range v.BaselineFailures {
		before[name] = true
	}
	for _, name := range names {
		if !before[name] {
			return false
		}
	}
	return true
}

func partitionPreExisting(headFailed []string, baseline *testfacts.Result) []string {
	baselineSet := make(map[string]bool, len(headFailed))
	for _, n := range failedTestNames(baseline) {
		baselineSet[n] = true
	}
	var preExisting []string
	for _, n := range headFailed {
		if baselineSet[n] {
			preExisting = append(preExisting, n)
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

func runBaselineTests(ctx context.Context, workspace, overlayPath, runArg string, packages []string) (*testfacts.Result, VerifyOutcome) {
	args := []string{"test", "-json", "-overlay", overlayPath, "-count=1", "-run", runArg}
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
