package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/build"
	"codenerd/internal/logging"
)

// attributeTestFailures compares the post-edit verification result against the
// pre-turn state. Failures that also occur without this turn's edits are
// pre-existing and must not be charged to the turn.
func attributeTestFailures(ctx context.Context, workspace string, packages []string, writtenPaths []string, preWrite map[string]string, head TestVerification) TestVerification {
	if head.Outcome != VerifyFailed {
		return head
	}
	headFailed := topLevelFailedTests(head.Output)
	if len(headFailed) == 0 || len(packages) == 0 || len(preWrite) == 0 {
		return head
	}
	for _, p := range writtenPaths {
		key := canonicalizeWrittenPath(p, workspace)
		if key == "" {
			key = p
		}
		if _, ok := preWrite[key]; !ok {
			if _, ok := preWrite[p]; !ok {
				logging.SessionDebug("test gate: skipping baseline attribution: missing pre-write snapshot for %q", p)
				return head
			}
		}
	}
	if ctx.Err() != nil {
		return head
	}
	tmpDir, overlayPath, ok := buildTestOverlay(workspace, preWrite)
	if !ok {
		return head
	}
	defer os.RemoveAll(tmpDir)
	out, ok := runBaselineTests(ctx, workspace, overlayPath, baselineRunRegex(headFailed), packages)
	if !ok {
		return head
	}
	preExisting := partitionPreExisting(headFailed, out)
	if len(preExisting) == 0 {
		return head
	}
	if len(preExisting) == len(headFailed) {
		return markAllPreExisting(head, preExisting)
	}
	return markSomePreExisting(head, preExisting)
}

func markAllPreExisting(head TestVerification, preExisting []string) TestVerification {
	logging.Get(logging.CategorySession).Warn("test gate: %d failure(s) also fail before this turn's edits (pre-existing), not charged to the turn: %s", len(preExisting), strings.Join(preExisting, ", "))
	passed := head
	passed.Outcome = VerifyPassed
	passed.OK = true
	passed.Ran = true
	passed.PreExistingFailures = preExisting
	return passed
}

func markSomePreExisting(head TestVerification, preExisting []string) TestVerification {
	head.PreExistingFailures = preExisting
	head.Output = "Pre-existing failures (also fail without this turn's edits; not yours to fix): " + strings.Join(preExisting, ", ") + "\n" + head.Output
	return head
}

func partitionPreExisting(headFailed []string, baselineOut string) []string {
	baselineSet := make(map[string]bool, len(headFailed))
	for _, n := range topLevelFailedTests(baselineOut) {
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

func runBaselineTests(ctx context.Context, workspace, overlayPath, runArg string, packages []string) (string, bool) {
	args := []string{"test", "-overlay", overlayPath, "-count=1", "-run", runArg}
	args = append(args, packages...)
	out, outcome, _ := runVerificationCommand(ctx, workspace, build.GetBuildEnv(nil, workspace), testVerifyTimeout, "go", args, verifyTestRunner)
	switch outcome {
	case VerifyPassed, VerifyFailed:
		return string(out), true
	default:
		return "", false
	}
}

func buildTestOverlay(workspace string, preWrite map[string]string) (string, string, bool) {
	tmpDir, err := os.MkdirTemp("", "test-baseline-*")
	if err != nil {
		return "", "", false
	}
	replace := writeOverlayFiles(tmpDir, workspace, preWrite)
	if replace == nil {
		os.RemoveAll(tmpDir)
		return "", "", false
	}
	overlayBytes, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", false
	}
	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlayBytes, 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", false
	}
	return tmpDir, overlayPath, true
}

func writeOverlayFiles(tmpDir, workspace string, preWrite map[string]string) map[string]string {
	replace := make(map[string]string, len(preWrite))
	idx := 0
	for key, content := range preWrite {
		abs := filepath.Join(workspace, filepath.FromSlash(key))
		if content == "" {
			replace[abs] = ""
			continue
		}
		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("overlay-%d.go", idx))
		idx++
		if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
			return nil
		}
		replace[abs] = tmpFile
	}
	return replace
}

// topLevelFailedTests extracts deduplicated, sorted top-level test names.
func topLevelFailedTests(output string) []string {
	set := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		name, ok := parseFailLine(line)
		if !ok {
			continue
		}
		set[name] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func parseFailLine(line string) (string, bool) {
	idx := strings.Index(line, "--- FAIL: ")
	if idx < 0 {
		return "", false
	}
	rest := strings.TrimSpace(line[idx+len("--- FAIL: "):])
	if rest == "" {
		return "", false
	}
	name := strings.Fields(rest)[0]
	if i := strings.Index(name, "/"); i >= 0 {
		name = name[:i]
	}
	if name == "" {
		return "", false
	}
	return name, true
}
