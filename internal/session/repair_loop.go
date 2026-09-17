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

// DefaultRepairMaxAttempts bounds one repair episode. Three iterations give a
// model room to misdiagnose once and still recover; beyond that the loop is
// burning budget going nowhere and the turn must fail loudly instead.
const DefaultRepairMaxAttempts = 3

// DefaultRepairWallClock bounds one repair episode in wall time. Five minutes
// covers several real go build/test cycles plus model latency without letting
// an unattended run stall: repair that has not converged by then will not.
const DefaultRepairWallClock = 5 * time.Minute

// repairRoundsPerAttempt bounds one attempt's read-diagnose-edit cycle: each
// attempt is allowed multiple model calls so it can read, then edit, before
// the loop rechecks. Without it a single-call attempt that spends its only
// model call reading ends with no edit and the loop cannot converge.
const repairRoundsPerAttempt = 6

// repairRetainedOutputCap caps each OLDER attempt's retained output in later
// prompts. The latest failing output always goes back whole (a repair prompt
// built from truncated errors repairs whatever sorted first); only history
// older than one round is capped, so context cannot grow without bound.
const repairRetainedOutputCap = 2000

// RepairBudget is the hard budget for one repair episode: bounded
// read→diagnose→edit→verify iterations under attempt and wall-clock ceilings.
type RepairBudget struct {
	MaxAttempts int
	WallClock   time.Duration
}

// repairBudgetFor resolves the episode budget from executor config with
// defaults. Non-positive values fall back to defaults: repair is always
// bounded; there is no way to configure an unbounded loop.
func (e *Executor) repairBudgetFor() RepairBudget {
	cfg := e.configSnapshot()
	b := RepairBudget{MaxAttempts: DefaultRepairMaxAttempts, WallClock: DefaultRepairWallClock}
	if cfg.RepairMaxAttempts > 0 {
		b.MaxAttempts = cfg.RepairMaxAttempts
	}
	if cfg.RepairWallClock > 0 {
		b.WallClock = cfg.RepairWallClock
	}
	return b
}

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

// repairEpisodeContext derives the episode clock. It mirrors
// runVerificationCommand: an already-expired parent deadline does not apply
// (the episode keeps its own budget, so verification at the deadline edge
// still gets its repair — observed live, the repair round used to die
// instantly on the turn's expired context), a live parent deadline is
// respected via the earlier of the two, and an explicit parent cancel kills
// the episode immediately.
func repairEpisodeContext(parent context.Context, wallClock time.Duration) (context.Context, context.CancelFunc) {
	if parent.Err() == context.Canceled {
		// Explicit cancel kills the episode before it starts: no attempts.
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		return canceled, func() {}
	}
	// A pre-expired parent deadline does not apply: the episode keeps its
	// own budget (mirroring runVerificationCommand).
	timeout := wallClock
	if deadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		timeout = wallClock
	}
	epCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	stop := context.AfterFunc(parent, func() {
		if parent.Err() == context.Canceled {
			cancel()
		}
	})
	return epCtx, func() {
		stop()
		cancel()
	}
}

// repairSpec describes one gate's repair episode: how to prompt from failing
// output, how to recheck, and what follow-ups to record on give-up.
type repairSpec struct {
	kind string
	// brokenPhrase is the gate's contract phrase for failure errors
	// ("edits broke the build"). The loop's errors keep the wording the
	// single-round repair used so existing consumers keep matching; the
	// new cost and attempt evidence rides after it.
	brokenPhrase string
	promptFor    func(failingOutput string) string
	// recheck re-verifies after an attempt. It reports passed, the failing
	// output when failed (which seeds the next attempt), and the raw verdict
	// for cancel/indeterminate handling.
	recheck func(ctx context.Context) (passed bool, failingOutput string, verdict VerifyOutcome)
	// followups builds actionable next commands for the give-up record.
	followups func() []string
}

// repairLoop runs bounded read→diagnose→edit→verify iterations until the
// recheck passes, the budget exhausts, or the episode is canceled.
//
// Each attempt starts from retained error context — the latest failing output
// plus compact summaries of prior attempts — appended to history, never
// rewritten: turns after the first continue from the failure, not from a
// cleaned-up story. An attempt that reads without editing escalates the next
// one to the commit regime (reads closed), preserving the old second-round
// behavior inside the loop.
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
	budget := e.repairBudgetFor()
	rec := &RepairRecord{Kind: spec.kind, InitialFailure: seedOutput}
	epCtx, cancelEpisode := repairEpisodeContext(ctx, budget.WallClock)
	defer cancelEpisode()

	var last *types.LLMToolResponse
	var allErrs []string
	seed := seedOutput
	useCommitRegime := false

	for attempt := 1; attempt <= budget.MaxAttempts; attempt++ {
		if err := epCtx.Err(); err != nil {
			rec.Followups = spec.followups()
			if epCtx.Err() == context.Canceled && ctx.Err() == context.Canceled {
				return nil, allErrs, rec, fmt.Errorf("repair canceled: %w", context.Canceled)
			}
			logging.Get(logging.CategorySession).Warn(
				"Repair episode (%s) exhausted its wall clock after %d attempts; cost=%s",
				spec.kind, attempt-1, rec.Cost.String())
			return nil, allErrs, rec, fmt.Errorf(
				"%w: %s and the repair loop exhausted its %s wall clock after %d attempts (cost=%s) and the workspace still fails. Follow-ups: %s. Last failure:\n%s",
				ErrVerificationFailed, spec.brokenPhrase, budget.WallClock, attempt-1, rec.Cost.String(),
				strings.Join(rec.Followups, "; "), seed)
		}

		att := RepairAttempt{Index: attempt, Started: time.Now()}
		prompt := spec.promptFor(seed)
		if summary := repairHistorySummary(rec.Attempts); summary != "" {
			prompt += "\n\nPrior repair attempts this episode (do not repeat what already failed):\n" + summary
		}
		if useCommitRegime {
			prompt += "\n\n" + workingRegimeText(commitRegime)
		}
		regimeNote := ""
		if useCommitRegime {
			regimeNote = " under the commit regime"
		}
		logging.Get(logging.CategorySession).Warn(
			"Repair attempt %d/%d (%s)%s", attempt, budget.MaxAttempts, spec.kind, regimeNote)

		repaired, llmCalls, allCalls, repairErrs, toolResults, wrote, err := e.repairRound(epCtx, trp, systemPrompt, history, toolDefs, cfg, result, prompt, useCommitRegime)
		allErrs = append(allErrs, repairErrs...)
		if err != nil {
			att.Verdict = VerifyIndeterminate
			att.SeedExcerpt = excerpt(seed, repairRetainedOutputCap)
			rec.Attempts = append(rec.Attempts, att)
			rec.Cost.Attempts++
			rec.Cost.Backtracks++
			rec.Followups = spec.followups()
			return nil, allErrs, rec, fmt.Errorf(
				"%w: %s and the repair loop failed on attempt %d (%v) (cost=%s). Follow-ups: %s. Last failure:\n%s",
				ErrVerificationFailed, spec.brokenPhrase, attempt, err, rec.Cost.String(),
				strings.Join(rec.Followups, "; "), seed)
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

		passed, failingOutput, verdict := spec.recheck(epCtx)
		att.Verdict = verdict
		att.SeedExcerpt = excerpt(failingOutput, repairRetainedOutputCap)
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
			seed = failingOutput
			if !wrote {
				useCommitRegime = true
			}
			logging.Get(logging.CategorySession).Warn(
				"Repair attempt %d (%s) still failing; backtracking to retained error context",
				attempt, spec.kind)
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
		"Repair episode (%s) gave up after %d attempts; cost=%s",
		spec.kind, budget.MaxAttempts, rec.Cost.String())
	return nil, allErrs, rec, fmt.Errorf(
		"%w: %s and the repair loop did not converge after %d attempts (cost=%s). Follow-ups: %s. Last failure:\n%s",
		ErrVerificationFailed, spec.brokenPhrase, budget.MaxAttempts, rec.Cost.String(),
		strings.Join(rec.Followups, "; "), seed)
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
