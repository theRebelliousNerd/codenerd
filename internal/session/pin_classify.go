package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encoding/json"

	internalbuild "codenerd/internal/build"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// Not pinnable.
//
// A change the turn's tests do not notice taken out is charged by the pin
// gate, and a decision held at a constant by the /survivors round. Some can
// never be pinned -- a branch only a lost race takes, a change that only makes
// the code faster -- and chasing them spent repair rounds and the turn's time.
// Two measurements go to the kernel: Jev's reading of whether taking the
// change out would change anything a test can observe and force
// (turn_pin_effect_reading), and, for the changes the kernel asks about
// (turn_pin_suite_wanted), whether the package's WHOLE suite passes with it
// taken out (turn_pin_suite_equivalent). turn_rounds.mg derives
// turn_pin_not_pinnable from both; the gate and the round drop those and the
// turn's report names them with the evidence.

// PinEffectJudge reads whether taking a change out is observable: the
// probability (0-100) that some test-forcible behaviour differs.
type PinEffectJudge interface {
	ReadPinEffect(ctx context.Context, label, withChange, takenOut string) (int64, error)
}

// SetPinEffectJudge installs the reading (typesafe.pin_equivalence). Nil
// classifies nothing.
func (e *Executor) SetPinEffectJudge(j PinEffectJudge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pinJudge = j
}

func (e *Executor) currentPinJudge() PinEffectJudge {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pinJudge
}

// classifyUnpinnable measures the evidence for the pin gate's unpinned
// changes and surviving decisions, asks the kernel which are not pinnable,
// and takes those out of what the gate and the survivors round ask for.
func (e *Executor) classifyUnpinnable(ctx context.Context, workspace string, result *ExecutionResult) {
	judge := e.currentPinJudge()
	if judge == nil || e.kernel == nil || result == nil {
		return
	}
	units := append(append([]pinUnit(nil), result.pinUnpinned...), result.pinSurvived...)
	if len(units) == 0 {
		return
	}
	turn := result.turnAtom()
	workspace = goWorkspace(workspace)
	byLabel := map[string]pinUnit{}
	readings := map[string]int64{}
	for _, u := range units {
		cur, err := os.ReadFile(diskPath(workspace, u.path))
		if err != nil {
			continue
		}
		with, without := changedRegion(string(cur), u.content)
		p, err := judge.ReadPinEffect(ctx, u.label(), with, without)
		if err != nil {
			logging.Get(logging.CategorySession).Info("pinning gate: no effect reading for %s: %v", u.label(), err)
			continue
		}
		if e.assertTurnFact(types.Fact{Predicate: "turn_pin_effect_reading", Args: []any{turn, u.label(), p}}) {
			byLabel[u.label()] = u
			readings[u.label()] = p
		}
	}
	if len(byLabel) == 0 {
		return
	}
	wanted, err := e.kernel.Query("turn_pin_suite_wanted")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("pinning gate: turn_pin_suite_wanted: %v", err)
	}
	for _, label := range turnLabels(wanted, turn) {
		u, ok := byLabel[label]
		if !ok {
			continue
		}
		if equivalent, why := pinSuiteEquivalent(ctx, workspace, u); equivalent {
			e.assertTurnFact(types.Fact{Predicate: "turn_pin_suite_equivalent", Args: []any{turn, label}})
		} else {
			logging.Get(logging.CategorySession).Info("pinning gate: %s is noticed by the package's suite: %s", label, why)
		}
	}
	notPinnable := map[string]bool{}
	classified, err := e.kernel.Query("turn_pin_not_pinnable")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("pinning gate: turn_pin_not_pinnable: %v", err)
	}
	for _, label := range turnLabels(classified, turn) {
		if _, ok := byLabel[label]; ok && !notPinnable[label] {
			notPinnable[label] = true
			result.PinNotPinnable = append(result.PinNotPinnable, fmt.Sprintf(
				"%s: not pinnable -- the package's whole suite passes with it taken out, and Jev reads no effect a test can force (%d%%)",
				label, readings[label]))
		}
	}
	if len(notPinnable) == 0 {
		return
	}
	logging.Get(logging.CategorySession).Info("pinning gate: %d change(s) classified not pinnable:\n%s",
		len(notPinnable), strings.Join(result.PinNotPinnable, "\n"))
	result.pinUnpinned = dropUnits(result.pinUnpinned, notPinnable)
	result.pinSurvived = dropUnits(result.pinSurvived, notPinnable)
	result.PinAdvisory = dropLabels(result.PinAdvisory, notPinnable)
	if result.PinCheck.Verdict() == VerifyFailed {
		if len(result.pinUnpinned) == 0 {
			result.PinCheck.OK = true
			result.PinCheck.Outcome = VerifyPassed
			result.PinCheck.Output = ""
			result.PinCheck.Reason = "every change the turn's tests do not notice is not pinnable: " + strings.Join(result.PinNotPinnable, "; ")
		} else {
			result.PinCheck.Output = dropListingLines(result.PinCheck.Output, notPinnable)
			result.PinCheck.Reason = fmt.Sprintf("%d change(s) pinned by no test this turn wrote; not pinnable: %s",
				len(result.pinUnpinned), strings.Join(result.PinNotPinnable, "; "))
		}
	}
}

// turnLabels reads the Change argument of the rows for turn.
func turnLabels(rows []types.Fact, turn types.MangleAtom) []string {
	var out []string
	for _, f := range rows {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == string(turn) {
			out = append(out, types.ExtractString(f.Args[1]))
		}
	}
	return out
}

// pinSuiteEquivalent runs the unit's package's whole test suite with the
// change taken out: equivalent when it passes.
func pinSuiteEquivalent(ctx context.Context, workspace string, u pinUnit) (bool, string) {
	pkgs := packagesForPaths([]string{u.path})
	if len(pkgs) == 0 {
		return false, "no package"
	}
	tmpDir, err := os.MkdirTemp("", "pin-suite-*")
	if err != nil {
		return false, err.Error()
	}
	defer os.RemoveAll(tmpDir)
	replace := map[string]string{goOverlayKey(workspace, u.path): ""}
	if !u.absent {
		stand := filepath.Join(tmpDir, "unit.go")
		if err := os.WriteFile(stand, []byte(u.content), 0o644); err != nil {
			return false, err.Error()
		}
		replace[goOverlayKey(workspace, u.path)] = stand
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		return false, err.Error()
	}
	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		return false, err.Error()
	}
	args := []string{"test", "-json", "-overlay", overlayPath, "-count=1", pkgs[0]}
	_, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), testVerifyTimeout, "go", args, verifyTestRunner)
	if outcome == VerifyPassed {
		return true, ""
	}
	return false, fmt.Sprintf("%s%s", outcome, suffixed(reason))
}

// changedRegion is the part of two versions of a file that differs, widened
// to whole lines: the code with the change and the code with it taken out.
func changedRegion(with, without string) (string, string) {
	a, b := strings.Split(with, "\n"), strings.Split(without, "\n")
	start := 0
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	return strings.Join(a[start:endA], "\n"), strings.Join(b[start:endB], "\n")
}

func dropUnits(units []pinUnit, drop map[string]bool) []pinUnit {
	var out []pinUnit
	for _, u := range units {
		if !drop[u.label()] {
			out = append(out, u)
		}
	}
	return out
}

func dropLabels(labels []string, drop map[string]bool) []string {
	var out []string
	for _, l := range labels {
		if !drop[l] {
			out = append(out, l)
		}
	}
	return out
}

func dropListingLines(listing string, drop map[string]bool) string {
	lines := strings.Split(listing, "\n")
	out := lines[:0]
	for _, l := range lines {
		if !drop[l] {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
