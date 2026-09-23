package session

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"

	jitconfig "codenerd/internal/jit/config"
)

// repairRetainedOutputCap caps each OLDER attempt's retained output in later
// prompts. The latest failing output always goes back whole (a repair prompt
// built from truncated errors repairs whatever sorted first); only history
// older than one round is capped, so context cannot grow without bound.
const repairRetainedOutputCap = 2000

// RepairCost is the episode cost ledger: every attempt, model call, tool
// call, backtrack, and token the repair consumed.
type RepairCost struct {
	Attempts   int
	LLMCalls   int
	ToolCalls  int
	Backtracks int
	TokensIn   int
	TokensOut  int
}

func (c RepairCost) String() string {
	return fmt.Sprintf("attempts=%d llm_calls=%d tool_calls=%d backtracks=%d tokens_in=%d tokens_out=%d",
		c.Attempts, c.LLMCalls, c.ToolCalls, c.Backtracks, c.TokensIn, c.TokensOut)
}

// RepairToolRun records one tool invocation inside an attempt with its exit
// status.
type RepairToolRun struct {
	Tool string
	OK   bool
}

// RepairAttempt records one read→diagnose→edit→verify iteration. Index is the
// ordering key: wall clocks differ across runners, so observed_started orders
// attempts only within a record — Index orders them everywhere.
type RepairAttempt struct {
	Index     int
	Started   time.Time
	Wrote     bool
	LLMCalls  int
	ToolRuns  []RepairToolRun
	TokensIn  int
	TokensOut int
	Verdict   VerifyOutcome
	// SeedExcerpt is the failing output that seeded the NEXT attempt,
	// truncated for the record (the prompt itself carried it whole).
	SeedExcerpt string
}

// RepairRecord is the persistent episode record: outcome, cost, per-attempt
// evidence, edited files, and actionable follow-ups when the loop gives up.
type RepairRecord struct {
	Kind           string // "build" or "tests"
	Passed         bool
	InitialFailure string // the failing output that opened the episode
	Cost           RepairCost
	Attempts       []RepairAttempt
	EditedFiles    []string
	Followups      []string
}

// repairEpisodeContext is what a repair episode runs under -- its model calls,
// tools and re-verification: the turn's own deadline while one is still ahead
// (the user's constraint), cancelled with the turn, and nothing of the
// harness's own. A parent deadline already past does not apply, mirroring
// runVerificationCommand, so verification at the deadline edge still gets its
// repair (observed live: the repair round used to die instantly on the turn's
// expired context); a call then stays bounded by its client's HTTP timeout and
// the episode by its attempts.
func repairEpisodeContext(parent context.Context) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(parent)
	var ctx context.Context
	var cancel context.CancelFunc
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline) > 0 {
		ctx, cancel = context.WithDeadline(detached, deadline)
	} else {
		ctx, cancel = context.WithCancel(detached)
	}
	if parent.Err() == context.Canceled {
		// Explicit cancel kills the episode before it starts: no attempts.
		cancel()
		return ctx, func() {}
	}
	stop := context.AfterFunc(parent, func() {
		if parent.Err() == context.Canceled {
			cancel()
		}
	})
	return ctx, func() {
		stop()
		cancel()
	}
}

// repairFailure is what a recheck reports when it did not pass: the output
// that seeds the next attempt, and whether that output is this round's own
// subject or a test suite the round's edits broke on the way.
//
// Both are needed to write the prompt, and the verdict alone cannot tell them
// apart -- either way it is VerifyFailed. Ladder run R1-16 (2026-09-19) is
// what a round that cannot tell looks like: the model's coverage test found a
// real bug in the helper the turn had just written, the suite went red, and
// every remaining round opened "The tests pass, but no test executes these
// lines of code you changed:" above the FAIL trace, asked for more tests, and
// forbade the production fix the failure needed. The model weakened its own
// assertion to get out, and the turn ended /unverified.
type repairFailure struct {
	Output     string
	TestsBroke bool
}

// repairSpec describes one gate's repair episode: how to prompt from a
// failure, how to recheck, and what follow-ups to record on give-up.
type repairSpec struct {
	kind string
	// brokenPhrase is the gate's contract phrase for failure errors
	// ("edits broke the build"). The loop's errors keep the wording the
	// single-round repair used so existing consumers keep matching; the
	// new cost and attempt evidence rides after it.
	brokenPhrase string
	// promptFor writes the round's prompt from output that shows the round's
	// own subject.
	promptFor func(failingOutput string) string
	// brokeTestsPrompt writes it instead when the recheck reports a suite this
	// round's edits broke on the way. A round that leaves it nil gets
	// brokeTestsPrompt's shared wording: no round states its own subject over
	// a red run.
	brokeTestsPrompt func(testOutput string) string
	// recheck re-verifies after an attempt. It reports passed, the failure
	// when failed (which seeds the next attempt), and the raw verdict for
	// cancel/indeterminate handling.
	recheck func(ctx context.Context) (passed bool, failure repairFailure, verdict VerifyOutcome)
	// followups builds actionable next commands for the give-up record.
	followups func() []string
}

// prompt is the only way a round's prompt is written, so that the choice is
// made from what the recheck reported rather than from each round remembering
// to ask.
func (s repairSpec) prompt(f repairFailure) string {
	if !f.TestsBroke {
		return s.promptFor(f.Output)
	}
	if s.brokeTestsPrompt != nil {
		return s.brokeTestsPrompt(f.Output)
	}
	return brokeTestsRepairPrompt(f.Output)
}

// brokeTestsRepairPrompt is what a round says when its own edits left the
// suite red. The failure is the suite, not the round's subject, so the round
// says nothing about its subject until the suite is green again. Rounds whose
// subject changes which side is at fault -- vet keeps the behaviour its tests
// pin, pinning suspects the test it just wrote -- say so themselves.
func brokeTestsRepairPrompt(testOutput string) string {
	return "The tests fail after your last edit:\n\n```\n" + testOutput + "\n```\n\n" +
		"This is not what this round asked for; it is what your edit did on the way. Get the suite " +
		"green before anything else, then the round resumes. Fix whichever side is wrong -- the code " +
		"or the test you just wrote -- and say which it was. Do not delete or weaken a test that " +
		"existed before this turn."
}

// repairLoop runs read→diagnose→edit→verify attempts until the recheck
// passes, the policy gives up (repair_episode.mg: the user's attempt cap, or
// the same failure surviving two edits), or the episode is canceled. There
// is no wall clock: how long a model thinks is not evidence about whether it
// is converging (ladder run R1-4d: a 368 s call, cut with nothing returned by
// a 6.2-minute clock sized for faster models). The turn's own deadline, when
// the user set one, still applies.
//
// Each attempt starts from retained error context — the latest failing output
// plus compact summaries of prior attempts — appended to history, never
// rewritten: turns after the first continue from the failure, not from a
// cleaned-up story. Once an attempt reads without editing, every later one
// runs under the commit regime (reads closed; the policy's repair_closed).
//
// It returns the last model response (for the turn's answer), accumulated
// tool errors, the episode record (always non-nil when a repair ran), and an
// error only when the loop gave up on a still-failing workspace or was
// canceled. An indeterminate recheck retains the original failure and ends
// the episode without error — a timeout is not proof of recovery — exactly
// like the single-round code it replaces.
func (e *Executor) repairLoop(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history *[]types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
	seedOutput string,
	spec repairSpec,
) (*types.LLMToolResponse, []string, *RepairRecord, error) {
	rec := &RepairRecord{Kind: spec.kind, InitialFailure: seedOutput}
	episode := newRepairEpisodeAtom()
	epCtx, cancelEpisode := repairEpisodeContext(ctx)
	defer cancelEpisode()

	var last *types.LLMToolResponse
	var allErrs []string
	// The seeding failure is the round's own subject: the gate measured it
	// before any repair edit existed to break anything.
	failure := repairFailure{Output: seedOutput}
	useCommitRegime := false
	// Why the episode gave up, for its error: the policy's reason.
	gaveUp := ""

	for attempt := 1; gaveUp == ""; attempt++ {
		if err := epCtx.Err(); err != nil {
			rec.Followups = spec.followups()
			if err == context.Canceled {
				return nil, allErrs, rec, fmt.Errorf("repair canceled: %w", context.Canceled)
			}
			logging.Get(logging.CategorySession).Warn(
				"Repair episode (%s) reached the turn's deadline after %d attempts; cost=%s",
				spec.kind, attempt-1, rec.Cost.String())
			return nil, allErrs, rec, fmt.Errorf(
				"%w: %s and the turn's deadline passed after %d repair attempt(s) (cost=%s) and the workspace still fails. Follow-ups: %s. Last failure:\n%s",
				ErrVerificationFailed, spec.brokenPhrase, attempt-1, rec.Cost.String(),
				strings.Join(rec.Followups, "; "), failure.Output)
		}

		att := RepairAttempt{Index: attempt, Started: time.Now()}
		prompt := spec.prompt(failure)
		if summary := repairHistorySummary(rec.Attempts); summary != "" {
			prompt += "\n\nPrior repair attempts this episode (do not repeat what already failed):\n" + summary
		}
		if useCommitRegime {
			prompt = withRegimePrompt(prompt, commitRegime)
		}
		regimeNote := ""
		if useCommitRegime {
			regimeNote = " under the commit regime"
		}
		logging.Get(logging.CategorySession).Warn(
			"Repair attempt %d (%s)%s", attempt, spec.kind, regimeNote)

		repaired, llmCalls, allCalls, repairErrs, toolResults, wrote, err := e.repairRound(epCtx, trp, systemPrompt, history, toolDefs, cfg, result, prompt, useCommitRegime)
		allErrs = append(allErrs, repairErrs...)
		if err != nil {
			att.Verdict = VerifyIndeterminate
			att.SeedExcerpt = excerpt(failure.Output, repairRetainedOutputCap)
			rec.Attempts = append(rec.Attempts, att)
			rec.Cost.Attempts++
			rec.Cost.Backtracks++
			rec.Followups = spec.followups()
			return nil, allErrs, rec, fmt.Errorf(
				"%w: %s and the repair loop failed on attempt %d (%v) (cost=%s). Follow-ups: %s. Last failure:\n%s",
				ErrVerificationFailed, spec.brokenPhrase, attempt, err, rec.Cost.String(),
				strings.Join(rec.Followups, "; "), failure.Output)
		}
		last = repaired
		// One attempt may span several model calls (its read-diagnose-edit
		// rounds), so the cost counts every call, not just the last response.
		// The returned response's Usage already folds in all rounds' tokens.
		att.LLMCalls = llmCalls
		rec.Cost.LLMCalls += llmCalls
		if repaired != nil {
			rec.Cost.TokensIn += repaired.Usage.InputTokens
			rec.Cost.TokensOut += repaired.Usage.OutputTokens
			att.TokensIn = repaired.Usage.InputTokens
			att.TokensOut = repaired.Usage.OutputTokens
			att.ToolRuns = toolRunsFor(flattenRepairCalls(allCalls), toolResults)
			rec.Cost.ToolCalls += len(att.ToolRuns)
		}
		att.Wrote = wrote

		passed, rechecked, verdict := spec.recheck(epCtx)
		att.Verdict = verdict
		att.SeedExcerpt = excerpt(rechecked.Output, repairRetainedOutputCap)
		rec.Attempts = append(rec.Attempts, att)
		rec.Cost.Attempts++

		switch {
		case passed:
			rec.Passed = true
			rec.EditedFiles = editedFilesFor(result)
			logging.Get(logging.CategorySession).Info(
				"%s repair converged after %d attempt(s); cost=%s",
				spec.kind, attempt, rec.Cost.String())
			return repaired, allErrs, rec, nil
		case verdict == VerifyCanceled:
			rec.Followups = spec.followups()
			return last, allErrs, rec, fmt.Errorf("repair canceled during re-verification: %w", context.Canceled)
		case verdict == VerifyFailed:
			rec.Cost.Backtracks++
			failure = rechecked
			// What comes next is the policy's (repair_episode.mg): another
			// attempt, under which regime, or giving up.
			var closed bool
			gaveUp, closed = e.nextRepairMove(episode, attempt, wrote, rechecked.Output)
			useCommitRegime = useCommitRegime || closed
			if gaveUp == "" {
				logging.Get(logging.CategorySession).Warn(
					"Repair attempt %d (%s) still failing; backtracking to retained error context",
					attempt, spec.kind)
			}
		default: // VerifyIndeterminate, VerifySkipped
			// No verdict: the failure stands until an affirmative pass clears
			// it. The turn completes unverified and closeChangeEvidence gets
			// the final word — a timeout is not proof of recovery.
			logging.Get(logging.CategorySession).Warn(
				"Repair re-verification produced no verdict (%s); recovery NOT verified (cost=%s)",
				verdict, rec.Cost.String())
			return last, allErrs, rec, nil
		}
	}

	rec.EditedFiles = editedFilesFor(result)
	rec.Followups = spec.followups()
	logging.Get(logging.CategorySession).Warn(
		"Repair episode (%s) gave up after %d attempts (%s); cost=%s",
		spec.kind, rec.Cost.Attempts, gaveUp, rec.Cost.String())
	// Only the turn's own build and test repair ends here with nothing behind
	// it. The forcing rounds (vet, coverage, pinning, removed tests) start from
	// a green suite, hold their own snapshot, and put it back themselves.
	restoredNote := ""
	if spec.kind == "build" || spec.kind == "tests" {
		restoredNote = e.leaveBuildableTree(ctx, result)
	}
	if restoredNote != "" {
		restoredNote = " " + restoredNote
	}
	return nil, allErrs, rec, fmt.Errorf(
		"%w: %s and the repair loop did not converge after %d attempts: %s (cost=%s).%s Follow-ups: %s. Last failure:\n%s",
		ErrVerificationFailed, spec.brokenPhrase, rec.Cost.Attempts, gaveUp, rec.Cost.String(), restoredNote,
		strings.Join(rec.Followups, "; "), failure.Output)
}

// repairHistorySummary compacts prior attempts for the next prompt: what was
// tried and what the recheck said. Full outputs stay in the transcript; the
// summary only needs enough to stop repeats.
func repairHistorySummary(attempts []RepairAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, a := range attempts {
		fmt.Fprintf(&sb, "- Attempt %d: wrote=%v verdict=%s", a.Index, a.Wrote, a.Verdict)
		if a.SeedExcerpt != "" {
			fmt.Fprintf(&sb, " failing output: %s", a.SeedExcerpt)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// toolRunsFor joins tool calls with their results by ID for per-tool exit
// statuses. A call with no matching result is recorded failed: a swallowed
// tool response is a defect, not a success.
func toolRunsFor(calls []types.ToolCall, results []types.ToolResult) []RepairToolRun {
	byID := make(map[string]types.ToolResult, len(results))
	for _, r := range results {
		byID[r.ToolUseID] = r
	}
	runs := make([]RepairToolRun, 0, len(calls))
	for _, c := range calls {
		name := c.Name
		if name == "" {
			name = c.ID
		}
		ok := false
		if r, found := byID[c.ID]; found {
			ok = !r.IsError
		}
		runs = append(runs, RepairToolRun{Tool: name, OK: ok})
	}
	return runs
}

// flattenRepairCalls concatenates each round's tool calls in order, so repair
// accounting sees every call the attempt made. Nil when there are no rounds.
func flattenRepairCalls(rounds [][]types.ToolCall) []types.ToolCall {
	var all []types.ToolCall
	for _, round := range rounds {
		all = append(all, round...)
	}
	return all
}

// editedFilesFor snapshots the turn's written paths for the record, sorted
// and deduplicated.
func editedFilesFor(result *ExecutionResult) []string {
	if result == nil {
		return nil
	}
	seen := make(map[string]bool, len(result.WrittenPaths))
	out := make([]string, 0, len(result.WrittenPaths))
	for _, p := range result.WrittenPaths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// inheritRepair decides which repair record a fresh re-verification verdict
// may carry forward from the check it replaces. Downstream re-checks (the
// critic's uplift re-verify, closeChangeEvidence) replace the whole check
// struct after the repair gates ran; without this the converged episode's
// cost and attempt evidence silently vanishes even though the repair is
// exactly why the workspace passes.
//
// Inheritance is verdict-gated, fail-closed: only a fresh PASS inherits,
// and only a record that itself passed. A failed, indeterminate, canceled,
// or skipped recheck drops the record — a "repair passed" attachment on
// anything but a green check would be a lie, and a stale green attachment
// on a fresh failure would be worse.
func inheritRepair(freshVerdict VerifyOutcome, prior *RepairRecord) *RepairRecord {
	if freshVerdict == VerifyPassed && prior != nil && prior.Passed {
		return prior
	}
	return nil
}

func excerpt(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…[truncated]"
}

// repairFollowups builds actionable next commands for the give-up record:
// the exact verification command to rerun plus the edited files to inspect.
func repairFollowups(workspace string, packages []string, result *ExecutionResult, kind string) []string {
	var out []string
	switch kind {
	case "tests":
		if len(packages) > 0 {
			// packagesForPaths form is already go-test-ready ("." or ./dir).
			out = append(out, fmt.Sprintf("go test %s (in %s)", strings.Join(packages, " "), workspace))
		} else {
			out = append(out, fmt.Sprintf("go test ./... (in %s)", workspace))
		}
	default:
		out = append(out, fmt.Sprintf("go build ./... (in %s)", workspace))
	}
	for _, f := range editedFilesFor(result) {
		out = append(out, "inspect "+f)
	}
	return out
}
