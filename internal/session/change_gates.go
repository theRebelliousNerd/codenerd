package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	internalbuild "codenerd/internal/build"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The two gates in this file turn evidence the harness already measured into
// evidence the verdict reads, and give the model the round to act on it first.
// Observed 2026-09-19 (ladder run R1-2): a nerd fix turn wrote 27 blocks of Go
// that no test executes and an unreachable statement go vet reports. The
// session log named the blocks, gopls named the statement to an advisory critic
// that timed out, and the turn was recorded /done. The agent is not asked for
// the tests that harden the change; the harness makes it write them.

// vetFinding matches one positioned go vet diagnostic: path:line:col: message.
var vetFinding = regexp.MustCompile(`^(.+\.go):\d+:\d+: (.*)$`)

// vetDiagnostic is one go vet finding: the line vet printed, and its identity
// -- the file it names and what it says, without the position, so an edit
// above a finding moves its line and not what it is.
type vetDiagnostic struct {
	line string
	key  string
}

// vetDiagnostics parses vet's positioned findings. resolve maps the path vet
// printed to the workspace-relative file it stands for.
func vetDiagnostics(output string, resolve func(string) string) []vetDiagnostic {
	var out []vetDiagnostic
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		m := vetFinding.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, vetDiagnostic{line: line, key: resolve(m[1]) + "\x00" + m[2]})
	}
	return out
}

// workspaceFile resolves a path vet printed -- relative to the workspace it
// ran in, backslashed on Windows -- to the workspace-relative slash path.
func workspaceFile(workspace, printed string) string {
	abs := printed
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspace, abs)
	}
	if rel, err := filepath.Rel(workspace, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filepath.Clean(abs))
}

// newVetFindings is what the turn introduced: each finding vet reports now,
// less as many of the same finding as it reported before the turn.
func newVetFindings(now, before []vetDiagnostic) []string {
	seen := make(map[string]int, len(before))
	for _, d := range before {
		seen[d.key]++
	}
	var own []string
	for _, d := range now {
		if seen[d.key] > 0 {
			seen[d.key]--
			continue
		}
		own = append(own, d.line)
	}
	return own
}

// verifyVet runs go vet over the untagged packages the turn wrote and charges
// the turn with the findings it introduced, wherever vet reports them. Tag-gated
// packages are vetted by the test gate with their tags already (gateTests).
//
// A finding's position is not its cause: a lock added to a struct in one file
// is reported where another file copies the struct. External audit F5
// (2026-09-19): this gate kept only findings in files the turn wrote, so that
// case -- state.go written, "passes lock by value" in use.go -- was a pass.
// The turn's findings are now the difference from the same packages vetted as
// they were before the turn (vetBaseline); a finding that was already there is
// the workspace's, in whichever file. Without a baseline every finding in the
// packages is charged: attribution the gate cannot make is not a pass. Nor is
// a vet run that failed without naming a finding.
func verifyVet(ctx context.Context, workspace string, written []string, preWrite map[string]PreImage) BuildVerification {
	start := time.Now()
	runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(written))
	if len(runnable) == 0 {
		return BuildVerification{Outcome: VerifySkipped, Reason: "no untagged Go package written", Duration: time.Since(start)}
	}
	command := append([]string{"go", "vet"}, runnable...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout, command[0], command[1:], verifyBuildRunner)
	switch outcome {
	case VerifyPassed:
		return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Duration: time.Since(start)}
	case VerifyFailed:
		now := vetDiagnostics(string(out), func(p string) string { return workspaceFile(workspace, p) })
		if len(now) == 0 {
			return BuildVerification{Ran: true, Output: strings.TrimSpace(string(out)), Outcome: VerifyIndeterminate, Command: command,
				Reason: "go vet failed and named no finding to judge", Duration: time.Since(start)}
		}
		var own []string
		before, ok, why := vetBaseline(ctx, workspace, runnable, preWrite)
		if ok {
			own = newVetFindings(now, before)
		} else {
			for _, d := range now {
				own = append(own, d.line)
			}
			reason = "no pre-turn vet to compare with (" + why + "): every finding in the packages the turn wrote is charged to it"
		}
		if len(own) == 0 {
			logging.Get(logging.CategorySession).Warn(
				"go vet reports only findings that were there before this turn:\n%s", strings.TrimSpace(string(out)))
			return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Reason: "every finding predates the turn", Duration: time.Since(start)}
		}
		return BuildVerification{Ran: true, Output: strings.Join(own, "\n"), Outcome: VerifyFailed, Command: command, Reason: reason, Duration: time.Since(start)}
	default:
		return BuildVerification{Ran: len(out) > 0, Output: strings.TrimSpace(string(out)), Outcome: outcome, Command: command, Reason: reason, Duration: time.Since(start)}
	}
}

// vetBaseline vets the packages as they were before the turn: each file the
// turn wrote is overlaid with its preimage, and one it created is absent. Vet
// prints a finding inside an overlaid file under the overlay's temporary
// path, which is mapped back to the file it stands for, so the findings are
// keyed as the post-turn run's are. ok is false, with the reason, when there
// is no baseline to compare with.
func vetBaseline(ctx context.Context, workspace string, packages []string, preWrite map[string]PreImage) (findings []vetDiagnostic, ok bool, why string) {
	if len(preWrite) == 0 {
		return nil, false, "no record of the written files before the turn"
	}
	tmpDir, overlayPath, replace, err := buildTestOverlay(workspace, preWrite)
	if err != nil {
		return nil, false, err.Error()
	}
	defer os.RemoveAll(tmpDir)
	standsFor := make(map[string]string, len(replace))
	for abs, tmp := range replace {
		if tmp != "" {
			standsFor[overlayKey(tmp)] = abs
		}
	}
	resolve := func(printed string) string {
		abs := printed
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, abs)
		}
		if orig, found := standsFor[overlayKey(abs)]; found {
			return workspaceFile(workspace, orig)
		}
		return workspaceFile(workspace, printed)
	}
	args := append([]string{"vet", "-overlay", overlayPath}, packages...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout, "go", args, verifyBuildRunner)
	switch outcome {
	case VerifyPassed:
		return nil, true, ""
	case VerifyFailed:
		if findings = vetDiagnostics(string(out), resolve); len(findings) == 0 {
			return nil, false, "the pre-turn vet failed and named no finding"
		}
		return findings, true, ""
	default:
		return nil, false, fmt.Sprintf("the pre-turn vet did not finish: %s", reason)
	}
}

// overlayKey compares paths the way the file system does here: Windows
// paths are case-insensitive, and the overlay's own names never differ only
// by case.
func overlayKey(p string) string {
	return strings.ToLower(filepath.Clean(p))
}

func vetRepairPrompt(findings string) string {
	return "go vet reports these problems in the packages you changed, and they were not there before this turn:\n\n```\n" + findings + "\n```\n\n" +
		"A finding can be reported in a file you did not edit when your edit caused it -- a lock added to a struct is " +
		"reported where the struct is copied. Fix each one at its cause, then stop; go vet will be run again. Do not " +
		"silence a finding with a directive or by moving the code out of the file: the finding describes a defect " +
		"(unreachable code, a wrong format verb, a copied lock), and the fix is to remove the defect."
}

// verifyAndRepairVet runs the vet gate after the tests pass and, when the
// turn's own files have findings, gives the model repair rounds with them.
// A round that does not converge leaves the finding to the verdict
// (turn_missing_evidence /vet_not_clean); it does not fail the turn.
func (e *Executor) verifyAndRepairVet(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) {
		return nil, nil, nil
	}
	if !e.configSnapshot().VerifyBuildAfterEdits {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	result.VetCheck = verifyVet(ctx, workspace, result.WrittenPaths, result.PreWriteContents)
	if result.VetCheck.Verdict() != VerifyFailed || trp == nil {
		return nil, nil, nil
	}
	logging.Get(logging.CategorySession).Warn("go vet rejects this turn's files; giving the model repair rounds:\n%s", result.VetCheck.Output)
	snap, green, vetBefore := snapshotTurnFiles(workspace, result), result.TestCheck, result.VetCheck
	testsBroke := false
	spec := repairSpec{
		kind:         "vet",
		brokenPhrase: "go vet reports problems this turn introduced",
		promptFor: func(seed string) string {
			if testsBroke {
				return vetBrokeTestsPrompt(seed)
			}
			return vetRepairPrompt(seed)
		},
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			testsBroke = false
			v := verifyVet(epCtx, workspace, result.WrittenPaths, result.PreWriteContents)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				result.VetCheck = v
			}
			if v.Verdict() != VerifyPassed {
				return false, v.Output, v.Verdict()
			}
			// A vet repair that breaks the tests has repaired nothing: the
			// round keeps the suite as green as it found it.
			tv, _ := gateTests(epCtx, workspace, result, false)
			if tv.Verdict() == VerifyPassed || tv.Verdict() == VerifyFailed {
				tv.Repair = result.TestCheck.Repair
				result.TestCheck = tv
			}
			if tv.Verdict() != VerifyPassed {
				testsBroke = tv.Verdict() == VerifyFailed
				return false, tv.Output, tv.Verdict()
			}
			return true, "", VerifyPassed
		},
		followups: func() []string {
			return []string{"go vet " + strings.Join(packagesForPaths(result.WrittenPaths), " ")}
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, result.VetCheck.Output, spec)
	if undoRedRound("vet", workspace, result, snap, green, err) {
		result.VetCheck = vetBefore
		repaired = nil
	}
	result.VetCheck.Repair = rec
	return repaired, repairErrs, settleForcingRepair(err)
}

// vetBrokeTestsPrompt is the vet round's next attempt when the last one made
// go vet clean and broke the tests.
func vetBrokeTestsPrompt(testOutput string) string {
	return "go vet is clean now, but the tests fail after your vet repair:\n\n```\n" + testOutput + "\n```\n\n" +
		"A vet repair keeps the behaviour the tests pin. Fix the code so that go vet and the tests " +
		"both pass; do not change or remove a test to make it pass."
}

// narrowToChangedLines keeps the uncovered blocks that overlap lines the turn
// changed. The profile is file-level, so without this a one-line edit in a
// large file reports every uncovered block of the file as code the turn wrote
// (observed 2026-09-11: 59 blocks for one line); a file with no pre-write
// snapshot, which the turn created, keeps all its blocks.
func narrowToChangedLines(workspace string, result *ExecutionResult, uncovered []UncoveredBlock) []UncoveredBlock {
	if len(uncovered) == 0 || len(result.PreWriteContents) == 0 {
		return uncovered
	}
	changed := make(map[string][]LineRange, len(result.PreWriteContents))
	for path, before := range result.PreWriteContents {
		data, readErr := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
		if readErr != nil {
			continue
		}
		// An absent or unknown preimage has no lines: the whole file is the
		// turn's, the widest reading and the safe one for a coverage gate.
		changed[path] = changedLines(before.Content, string(data))
	}
	return blocksInChangedLines(uncovered, changed)
}

// uncoveredList names every block, one per line, by the path the turn wrote.
// summarizeUncovered caps its list for the log; a repair built from a capped
// list repairs the blocks that happened to sort first.
func uncoveredList(blocks []UncoveredBlock, written []string) string {
	var b strings.Builder
	for _, block := range blocks {
		path := NormalizeCoverPath(block.File)
		for _, w := range written {
			if nw := NormalizeCoverPath(w); nw != "" && strings.HasSuffix(path, nw) {
				path = nw
				break
			}
		}
		fmt.Fprintf(&b, "%s:%d-%d (%d statement(s))\n", path, block.StartLine, block.EndLine, block.NumStmts)
	}
	return strings.TrimRight(b.String(), "\n")
}

func coverageRepairPrompt(blocks string) string {
	return "The tests pass, but no test executes these lines of code you changed:\n\n```\n" + blocks + "\n```\n\n" +
		"Write or extend tests that run each of them and assert what it does -- the behaviour the " +
		"change was made for, and the error and edge branches it added. Put them in the package's " +
		"_test.go files. Do not change the production code to make these lines unreachable or to " +
		"remove them; they are the change. The tests will be run again with coverage."
}

// verifyAndRepairCoverage runs after the tests pass. When code the turn
// changed is executed by no test, the model gets repair rounds to write the
// tests that run it; what is still unexecuted afterwards is left to the
// verdict (turn_missing_evidence /changed_code_unexecuted), not failed.
func (e *Executor) verifyAndRepairCoverage(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || len(result.UncoveredBlocks) == 0 || result.TestCheck.Verdict() != VerifyPassed {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	if result.TestCheck.Repair != nil {
		// A test repair ran after the blocks were measured and may have
		// changed them: measure again before asking for anything.
		v, uncovered := gateTests(ctx, workspace, result, true)
		if v.Verdict() != VerifyPassed {
			return nil, nil, nil
		}
		result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
		if len(result.UncoveredBlocks) == 0 {
			return nil, nil, nil
		}
	}
	if trp == nil {
		return nil, nil, nil
	}
	logging.Get(logging.CategorySession).Warn(
		"Code this turn changed is executed by no test (%d block(s)); giving the model repair rounds to write the tests", len(result.UncoveredBlocks))
	// The state a red give-up is undone to: the round's start, then each
	// attempt that left the suite green, so a good attempt survives a bad one.
	snap, green, greenBlocks := snapshotTurnFiles(workspace, result), result.TestCheck, result.UncoveredBlocks
	spec := repairSpec{
		kind:         "coverage",
		brokenPhrase: "code this turn changed is executed by no test",
		promptFor:    coverageRepairPrompt,
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			v, uncovered := gateTests(epCtx, workspace, result, true)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				// The test gate's own repair record stays with it; this
				// round's record is its own.
				v.Repair = result.TestCheck.Repair
				result.TestCheck = v
			}
			if v.Verdict() != VerifyPassed {
				// The new tests broke something: the failure is what the next
				// round works from; a round that ends here is undone below.
				return false, v.Output, v.Verdict()
			}
			result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
			snap, green, greenBlocks = snapshotTurnFiles(workspace, result), result.TestCheck, result.UncoveredBlocks
			if len(result.UncoveredBlocks) == 0 {
				return true, "", VerifyPassed
			}
			return false, uncoveredList(result.UncoveredBlocks, result.WrittenPaths), VerifyFailed
		},
		followups: func() []string {
			runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
			return repairFollowups(workspace, runnable, result, "coverage")
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, uncoveredList(result.UncoveredBlocks, result.WrittenPaths), spec)
	if undoRedRound("coverage", workspace, result, snap, green, err) {
		result.UncoveredBlocks = greenBlocks
		repaired = nil
	}
	if rec != nil && !rec.Passed {
		logging.Get(logging.CategorySession).Warn(
			"Coverage repair left %d block(s) executed by no test; the verdict names them", len(result.UncoveredBlocks))
	}
	return repaired, repairErrs, settleForcingRepair(err)
}

// removedTestsRepairPrompt asks for the tests the turn deleted back. The seed
// is either the tests still missing, with their source, or the failing run of
// the restored tests.
func removedTestsRepairPrompt(seed string) string {
	return "This turn deleted tests that existed before it, and no test of the same name exists in the " +
		"same package now. A test is a contract the system already had: put each one back in the " +
		"file it came from. If your change altered the behaviour a test pins on purpose, keep the test " +
		"and change its assertions to the new behaviour -- never delete it. The tests are run again " +
		"afterwards.\n\n" + seed
}

// verifyAndRepairRemovedTests runs after the other gates, so a suite made
// green by losing a contract is still caught. A turn that deleted tests which
// existed before it -- and exist nowhere in their package now -- gets repair
// rounds in which it is handed each one's source as the turn found it, and
// the restored tests are run again. A turn that still deletes them fails: a
// test is fixed by fixing the code, not by deleting it. Until 2026-09-19 the
// first deletion failed the turn with no round (ladder run R1-4: a whole-file
// rewrite dropped three tests that still passed against the new code, and
// fourteen minutes of otherwise green work were refused).
func (e *Executor) verifyAndRepairRemovedTests(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || result.SuccessfulWriteTools == 0 || len(result.PreWriteContents) == 0 {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	removed := removedTestFunctions(workspace, result.WrittenPaths, result.PreWriteContents)
	if len(removed) == 0 {
		return nil, nil, nil
	}
	if trp == nil {
		return nil, nil, removedTestsError(removed)
	}
	missing := func() string {
		return "Deleted by this turn, as they were before it:\n\n" + removedTestListing(removed, result.PreWriteContents)
	}
	logging.Get(logging.CategorySession).Warn(
		"This turn deleted %d test(s) that existed before it; giving the model repair rounds to restore them: %s",
		len(removed), strings.Join(removed, ", "))
	spec := repairSpec{
		kind:         "removed_tests",
		brokenPhrase: "the turn deleted tests that existed before it",
		promptFor:    removedTestsRepairPrompt,
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			removed = removedTestFunctions(workspace, result.WrittenPaths, result.PreWriteContents)
			if len(removed) > 0 {
				return false, missing(), VerifyFailed
			}
			v, _ := gateTests(epCtx, workspace, result, false)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				// The test gate's own repair record stays with it.
				v.Repair = result.TestCheck.Repair
				result.TestCheck = v
			}
			if v.Verdict() != VerifyPassed {
				return false, "The deleted tests are back, and the tests fail:\n\n```\n" + v.Output + "\n```", v.Verdict()
			}
			return true, "", VerifyPassed
		},
		followups: func() []string {
			if len(removed) > 0 {
				return []string{"restore " + strings.Join(removed, ", ")}
			}
			runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
			return repairFollowups(workspace, runnable, result, "tests")
		},
	}
	repaired, repairErrs, _, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, missing(), spec)
	if err != nil && errors.Is(err, ErrVerificationFailed) && len(removed) > 0 {
		return nil, repairErrs, removedTestsError(removed)
	}
	return repaired, repairErrs, err
}

// removedTestsError fails a turn that deleted tests and did not put them back.
func removedTestsError(removed []string) error {
	return fmt.Errorf("%w: turn removed test(s) without replacing them: %s; a failing test is fixed by fixing the code, not by deleting the test",
		ErrVerificationFailed, strings.Join(removed, ", "))
}

// turnFiles holds the turn's written files as a forcing round found them. A
// forcing round starts from a green suite and must not end the turn worse than
// it found it: one that gives up with the suite red is undone from this, and
// the debt it leaves is the obligation it did not discharge, which the verdict
// names. Ladder run R1-4b: a change that passed build and tests was failed
// because the coverage round's own test was wrong three times over.
type turnFiles struct {
	written []string
	// pre is each written file as the round found it. A file that could not
	// be read is an unknown preimage, not an absent one: the restore refuses
	// it rather than deleting a file it merely failed to read.
	pre map[string]PreImage
}

func snapshotTurnFiles(workspace string, result *ExecutionResult) turnFiles {
	snap := turnFiles{
		written: append([]string(nil), result.WrittenPaths...),
		pre:     map[string]PreImage{},
	}
	for _, p := range snap.written {
		snap.pre[p] = readPreImage(turnFilePath(workspace, p))
	}
	return snap
}

// restore puts the turn's files back as the round found them: a file the
// round changed gets its content back, a file the round wrote for the first
// time gets its pre-turn content -- an empty file stays an empty file -- and
// one the round created is removed. A file the round wrote whose earlier state
// is unrecorded or unknown is not guessed at: the restore refuses before
// touching anything. It returns the paths it changed; a path it could not put
// back stays in WrittenPaths, so the gates that follow still see that write.
func (snap turnFiles) restore(workspace string, result *ExecutionResult) ([]string, error) {
	type step struct {
		path string
		want PreImage
	}
	var plan []step
	for _, p := range result.WrittenPaths {
		want, recorded := snap.pre[p]
		if !recorded {
			want, recorded = result.PreWriteContents[p]
		}
		if !recorded {
			return nil, fmt.Errorf("%s was written by the round with no record of it before the turn", p)
		}
		if !want.Known() {
			return nil, fmt.Errorf("%s was written by the round and what it held before is unknown: %s", p, want.Unknown)
		}
		plan = append(plan, step{path: p, want: want})
	}
	var changed, unrestored []string
	var errs []error
	for _, s := range plan {
		path := turnFilePath(workspace, s.path)
		current, readErr := os.ReadFile(path)
		// Only "does not exist" is absent: a path that is there but cannot
		// be read as a file is still there, and must still be removed.
		absent := errors.Is(readErr, fs.ErrNotExist)
		switch {
		case !s.want.Existed && !absent:
			if err := os.Remove(path); err != nil {
				errs = append(errs, err)
				unrestored = append(unrestored, s.path)
				continue
			}
		case s.want.Existed && (readErr != nil || !bytes.Equal(current, []byte(s.want.Content))):
			if err := os.WriteFile(path, []byte(s.want.Content), 0o644); err != nil {
				errs = append(errs, err)
				unrestored = append(unrestored, s.path)
				continue
			}
		default:
			continue
		}
		changed = append(changed, s.path)
	}
	result.WrittenPaths = append([]string(nil), snap.written...)
	for _, p := range unrestored {
		if !slices.Contains(result.WrittenPaths, p) {
			result.WrittenPaths = append(result.WrittenPaths, p)
		}
	}
	return changed, errors.Join(errs...)
}

func turnFilePath(workspace, p string) string {
	if filepath.IsAbs(p) || workspace == "" {
		return filepath.FromSlash(p)
	}
	return filepath.Join(workspace, filepath.FromSlash(p))
}

// undoRedRound undoes a forcing round that gave up with the suite red and
// puts back the green test verdict the round started from. It reports whether
// it undid the round.
func undoRedRound(kind, workspace string, result *ExecutionResult, snap turnFiles, green TestVerification, err error) bool {
	if err == nil || !errors.Is(err, ErrVerificationFailed) || result.TestCheck.Verdict() != VerifyFailed {
		return false
	}
	failure := result.TestCheck.Output
	restored, restoreErr := snap.restore(workspace, result)
	if restoreErr != nil {
		logging.Get(logging.CategorySession).Warn("could not undo the %s round (the final check will judge the workspace): %v", kind, restoreErr)
		return false
	}
	result.TestCheck = green
	logging.Get(logging.CategorySession).Warn(
		"%s round gave up with the tests failing; restored %s as the round found them. Last failure:\n%s",
		kind, strings.Join(restored, ", "), failure)
	return true
}

// settleForcingRepair decides what a forcing round's error means for the
// turn. Running out of attempts leaves the debt on the result, where the
// kernel's verdict names it; a cancel is a cancel.
func settleForcingRepair(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, ErrVerificationFailed) {
		logging.Get(logging.CategorySession).Warn("forcing round did not converge; the verdict carries what is missing: %v", err)
		return nil
	}
	return err
}
