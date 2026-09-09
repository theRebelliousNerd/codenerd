// This file wires codeNERD's learning loop into the Cortex, which is the boot
// every execution path shares.
//
// Before it existed, System Prompt Learning was assembled in
// cmd/nerd/chat/session_shared_boot.go and nowhere else. That put every part
// of the loop behind a human sitting in the TUI: `nerd campaign`, `nerd
// instruction`, `nerd spawn`, `nerd swebench` and every delegated shard task
// booted through GetOrBootCortex, which built no evolver, recorded no
// executions, and therefore learned nothing. The agent improved only while
// being watched, which is precisely backwards — the autonomous runs are where
// it does the most work and makes the most mistakes.
//
// Owning the evolver here fixes that for every caller at once, and it has to
// be here rather than in both places: the feedback collector and strategy
// store are SQLite files under .nerd/, so two evolvers in one process would be
// two writers on one database.
package system

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	ctxlearn "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/features"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
)

// recordIDSequence disambiguates execution records minted within one clock
// tick.
//
// Process-wide and monotonic, because that is the scope the collision lives
// in: records from different processes are already partitioned by the session
// id minted at boot, and records from one process can be minted concurrently
// by delegated tasks that share a session id and a turn number.
var recordIDSequence atomic.Uint64

// mintRecordIDs builds the identifiers for one execution record.
//
// Session and turn alone do not identify a record: delegated tasks run on
// executor clones that inherit the parent's session id, so several tasks
// legitimately carry the same (session, turn) pair.
//
// What disambiguates them is the counter, NOT the clock. A wall-clock
// nanosecond looks unique and is not — Windows' timer granularity is coarse
// enough that consecutive calls return the same value, which CI caught as two
// records minted in one tick colliding. A colliding TaskID means one delegated
// task's outcome overwrites another's in the feedback database: silently, and
// in the direction that loses failures.
//
// The timestamp stays in the ID because it makes a record greppable and orders
// a listing. It is simply not what makes it unique, which is why `at` is a
// parameter: a test can pass the same instant twice and see that the IDs still
// differ. Sampling a loop cannot show that — on Linux the clock always
// advances, so the bug is invisible there no matter how many iterations run.
func mintRecordIDs(sessionID string, turnNumber int, shardType string, at time.Time) (taskID, shardID string) {
	seq := recordIDSequence.Add(1)
	nano := at.UnixNano()
	taskID = fmt.Sprintf("turn-%s-%d-%d-%d", sessionID, turnNumber, nano, seq)
	shardID = fmt.Sprintf("%s-%d-%d", strings.TrimPrefix(shardType, "/"), nano, seq)
	return taskID, shardID
}

// evolutionCycleTimeout bounds one automatic cycle. The cycle sends up to 20
// records to the LLM-as-Judge at five-way concurrency, so it is minutes, not
// seconds — but it must still be bounded, because it runs on the maintenance
// goroutine that Cortex.Close waits on.
const evolutionCycleTimeout = 2 * time.Minute

// initLearningLoop constructs the prompt evolver and connects it to both ends
// of the loop: the session executor writes turn outcomes in, and the JIT
// compiler reads evolved atoms back out.
//
// It never fails the boot. A workspace where .nerd/ is read-only, or where the
// evolution databases are corrupt, should still run codeNERD — it just will
// not learn, and says so once at Warn.
func initLearningLoop(bctx *bootContext) error {
	if bctx == nil || bctx.jitCompiler == nil {
		// No compiler means no atoms to credit and nowhere to serve an
		// evolved atom from; an evolver here would only write records nothing
		// could ever read back.
		return nil
	}

	judge := bctx.judgeClient()
	if judge == nil {
		// A boot with no LLM client is a degraded one — Cortex.WorkerLLM hands
		// out missingLLMClient, so every turn fails on "no LLM configured".
		// Recording those is not learning, it is filling the corpus with one
		// error, and the judge would nil-panic on the maintenance goroutine
		// the first time a cycle ran.
		logging.Get(logging.CategoryBoot).Info(
			"Learning loop skipped: no LLM client configured for this boot")
		return nil
	}

	nerdDir := filepath.Join(bctx.workspace, ".nerd")
	evolver, err := pe.NewPromptEvolver(nerdDir, judge, pe.DefaultEvolverConfig())
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn(
			"Prompt evolution unavailable in %s: %v; this session will not learn from its turns", nerdDir, err)
		return nil
	}
	bctx.promptEvolver = evolver

	// Promotion is a change to the agent's own instructions, so it is recorded
	// as a kernel fact rather than left implicit in a file on disk.
	if kernel := bctx.kernel; kernel != nil {
		evolver.SetOnAtomPromoted(func(atomID string, promotedAt time.Time) {
			if err := kernel.Assert(core.Fact{
				Predicate: "prompt_evolved",
				Args:      []any{atomID, promotedAt.Unix()},
			}); err != nil {
				logging.Get(logging.CategoryBoot).Warn(
					"Failed to assert prompt_evolved for %s: %v", atomID, err)
			}
		})
	}

	// The read-back half of the loop. Without these two registrations the
	// evolver writes atoms to .nerd/prompts/evolved/promoted/ and strategies
	// to .nerd/prompts/strategies.db that the compiler never serves — which
	// looks like learning and is not.
	bctx.jitCompiler.RegisterEvolvedAtomManager(prompt.NewEvolvedAtomManager(nerdDir))
	if provider := evolver.NewStrategyAtomProviderFor(); provider != nil {
		bctx.jitCompiler.RegisterStrategyProvider(provider)
	}

	if bctx.sessionExecutor != nil {
		bctx.sessionExecutor.SetTurnRecorder(newTurnEvolutionRecorder(evolver))
	}

	initContextFeedback(bctx)

	logging.Get(logging.CategoryBoot).Info(
		"Learning loop wired: recording=on automatic-evolution=%t (CODENERD_PROMPT_EVOLUTION)",
		features.IsPromptEvolutionEnabled())
	return nil
}

// judgeClient picks the LLM that grades past work.
//
// The worker tier, not the planner: judging is a short, structured
// classification over text that is already written, and the cycle issues one
// call per record. Routing that volume to the expensive reasoning tier would
// make the learning loop cost more than the work it is learning from.
//
// Returns nil when the boot has no LLM at all, which the caller treats as
// "this boot does not learn" rather than constructing an evolver whose judge
// would nil-panic the first time a cycle ran.
func (bctx *bootContext) judgeClient() pe.LLMClient {
	if bctx.shardLLMClient != nil {
		return bctx.shardLLMClient
	}
	if bctx.llmClient == nil {
		return nil
	}
	return bctx.llmClient
}

// turnEvolutionRecorder translates a finished session turn into the execution
// record the SPL loop consumes.
//
// The translation is where the kernel's verdict earns its keep. The chat path
// this replaces set Success to `err == nil`, which grades the agent on its own
// report: a turn that announced it had fixed the bug and written the tests,
// having done neither, was recorded as a success and its prompt atoms were
// credited for it. codeNERD already detects exactly that turn — the
// hollow-success gate ends it at /hollow — so the recorder reads the kernel
// instead of the return value.
type turnEvolutionRecorder struct {
	evolver *pe.PromptEvolver
}

func newTurnEvolutionRecorder(evolver *pe.PromptEvolver) *turnEvolutionRecorder {
	return &turnEvolutionRecorder{evolver: evolver}
}

// RecordTurn implements session.TurnRecorder. It returns immediately; the
// SQLite write happens on its own goroutine because this is called at the end
// of every turn, on the turn's goroutine.
func (r *turnEvolutionRecorder) RecordTurn(rec session.TurnRecord) {
	if r == nil || r.evolver == nil {
		return
	}
	record, ok := executionRecordFor(rec)
	if !ok {
		return
	}
	go func() {
		if err := r.evolver.RecordExecution(record); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Debug(
				"Failed to record turn %d of session %s for evolution: %v",
				rec.TurnNumber, rec.SessionID, err)
		}
		// Attribute the outcome to any learned strategy that was in this
		// turn's prompt. Without this write-back every strategy keeps a
		// success rate of zero forever and SelectStrategies' ranking is
		// arbitrary — the playbook never learns which of its plays work.
		r.evolver.RecordStrategyOutcome(record.TaskID, rec.AtomIDs, rec.Verified())
	}()
}

// executionRecordFor maps one turn onto an ExecutionRecord, and decides
// whether the turn is worth recording at all.
//
// The decision is the cost control for the whole loop. RunEvolutionCycle sends
// every record that carries no verdict to the LLM-as-Judge, so "record
// everything" is not a neutral choice: a campaign is thousands of turns, and
// thousands of judge calls to learn what the kernel already knew would cost
// more than the campaign. The three cases split on who has the cheaper answer:
//
//   - /done — the kernel watched the evidence land (build ran, tests ran,
//     files were written). It is a confirmed pass, so the record carries a
//     pre-filled PASS verdict and never reaches the judge. The verdict still
//     credits every atom that was in the prompt, which is the signal
//     promotion is scored on.
//   - /hollow, /failed — the kernel knows it went wrong but not why, and "why"
//     is what an atom has to be generated from. These records carry no verdict
//     so the judge picks them up and explains them. Judge spend is therefore
//     bounded by the failure count, not the turn count.
//   - /unverified — read-only turns: a question answered, nothing to verify.
//     Recording them would either pollute the failure buckets with answered
//     questions or burn a judge call to conclude nothing happened. Dropped.
func executionRecordFor(rec session.TurnRecord) (*pe.ExecutionRecord, bool) {
	if !rec.Verified() && !rec.Failed() {
		return nil, false
	}

	now := time.Now()
	shardType := strings.TrimSpace(rec.IntentVerb)
	if shardType == "" {
		shardType = "/unknown"
	}

	taskID, shardID := mintRecordIDs(rec.SessionID, rec.TurnNumber, shardType, now)

	record := &pe.ExecutionRecord{
		TaskID:      taskID,
		SessionID:   rec.SessionID,
		Timestamp:   now,
		ShardID:     shardID,
		ShardType:   shardType,
		Provider:    rec.Provider,
		Model:       rec.Model,
		TaskRequest: rec.Task,
		Duration:    rec.Duration,
		AtomIDs:     rec.AtomIDs,
		ExecutionResult: pe.ExecutionResult{
			Success: rec.Verified(),
			Output:  rec.Response,
			Metadata: map[string]string{
				"kernel_outcome": string(rec.Outcome),
				"tool_calls":     fmt.Sprintf("%d", rec.ToolCalls),
				"prompt_tokens":  fmt.Sprintf("%d", rec.PromptTokens),
			},
		},
	}

	if rec.Verified() {
		record.Verdict = &pe.JudgeVerdict{
			Verdict:     "PASS",
			Explanation: "Kernel derived turn_done: the turn's claims were matched by recorded evidence.",
			Category:    pe.CategoryCorrect,
			Confidence:  1.0,
			TaskID:      taskID,
			ShardType:   shardType,
			AtomIDs:     rec.AtomIDs,
			Timestamp:   now,
			Provider:    rec.Provider,
			Model:       rec.Model,
			EvaluatedBy: "mangle-kernel",
		}
		return record, true
	}

	// A failure reaches the judge, so hand it what the kernel saw. The judge
	// prompt renders BuildErrors verbatim, and "claimed success with no
	// evidence" is a far better starting point for an atom than the raw error
	// text alone.
	record.ExecutionResult.BuildErrors = failureEvidence(rec)
	return record, true
}

// failureEvidence renders the kernel's reason for rejecting a turn, plus the
// turn's own error when it has one.
func failureEvidence(rec session.TurnRecord) []string {
	var out []string
	if rec.Outcome == "/hollow" {
		out = append(out, "Hollow success: the turn reported the work as done, but the kernel "+
			"found no evidence for the claim (no successful write, test run, or build for an "+
			"intent that requires one).")
	}
	if rec.Err != nil {
		out = append(out, rec.Err.Error())
	}
	if len(out) == 0 {
		out = append(out, fmt.Sprintf("Turn ended at %s with no recorded error.", rec.Outcome))
	}
	return out
}

// runEvolutionCycle advances the learning loop one step: grade what is
// ungraded, generate atoms from failure groups, and reload the compiler so a
// newly promoted atom is actually served.
//
// It returns without spending anything when the operator has not opted in.
// See features.IsPromptEvolutionEnabled for why the default is off.
func (c *Cortex) runEvolutionCycle(ctx context.Context) {
	if c == nil || c.PromptEvolver == nil {
		return
	}
	if !features.IsPromptEvolutionEnabled() {
		return
	}
	if !c.PromptEvolver.ShouldRunEvolution() {
		return
	}

	cycleCtx, cancel := context.WithTimeout(ctx, evolutionCycleTimeout)
	defer cancel()

	// The cycle runs on the maintenance goroutine, so an unrecovered panic in
	// it takes the whole process down mid-session. Background self-improvement
	// is never worth that.
	defer func() {
		if r := recover(); r != nil {
			logging.Get(logging.CategoryAutopoiesis).Error("Evolution cycle panicked: %v", r)
		}
	}()

	result, err := c.PromptEvolver.RunEvolutionCycle(cycleCtx)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Evolution cycle failed: %v", err)
		return
	}
	if result == nil {
		return
	}

	logging.Get(logging.CategoryAutopoiesis).Info(
		"Evolution cycle: %d failure groups, %d atoms generated, %d atoms promoted",
		result.GroupsProcessed, result.AtomsGenerated, result.AtomsPromoted)

	// Refresh even when this cycle generated nothing: a promotion made
	// elsewhere (the chat's /evolve-promote, another process on the same
	// workspace) is on disk and invisible to this compiler until it reloads.
	if c.JITCompiler != nil {
		if err := c.JITCompiler.RefreshEvolvedAtoms(); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn(
				"Failed to reload evolved atoms into the compiler: %v", err)
		}
	}
}

// initContextFeedback opens the context-usefulness store and points the
// session executor at it.
//
// The store is the third learning loop, and it was in the same shape as the
// other two: built in the chat TUI, and on every other path the model's rating
// of the context it had just been given was written to a log line and dropped.
// The ratings tune spreading activation (ActivationEngine.computeFeedbackScore)
// on a per-predicate, per-verb basis, so a headless campaign's ratings improve
// what the next session retrieves.
//
// A store that cannot be opened is not fatal: the session runs, it just does
// not learn what context was worth its tokens.
func initContextFeedback(bctx *bootContext) {
	if bctx == nil || bctx.sessionExecutor == nil {
		return
	}

	dbPath := filepath.Join(bctx.workspace, ".nerd", "context_feedback.db")
	store, err := ctxlearn.NewContextFeedbackStore(dbPath)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn(
			"Context feedback store unavailable at %s: %v; context ratings will be discarded", dbPath, err)
		return
	}
	bctx.contextFeedback = store
	bctx.sessionExecutor.SetContextFeedbackRecorder(&contextFeedbackRecorder{store: store})
}

// contextFeedbackRecorder adapts a session turn's rating onto the store.
type contextFeedbackRecorder struct {
	store *ctxlearn.ContextFeedbackStore
}

// RecordContextFeedback implements session.ContextFeedbackRecorder. The write
// is asynchronous because it happens at the end of every turn, on the turn's
// own goroutine.
func (r *contextFeedbackRecorder) RecordContextFeedback(rec session.ContextFeedbackRecord) {
	if r == nil || r.store == nil {
		return
	}
	go func() {
		if err := r.store.StoreFeedback(
			rec.TurnNumber,
			rec.ManifestHash,
			rec.OverallUsefulness,
			rec.IntentVerb,
			// The kernel's verdict, not the turn's return value: a rating from
			// a turn that only claimed to succeed should not be weighted as
			// evidence that the context was what made it succeed.
			rec.Verified,
			rec.HelpfulPredicates,
			rec.NoisePredicates,
		); err != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Failed to store context feedback for session %s turn %d: %v",
				rec.SessionID, rec.TurnNumber, err)
		}
		if rec.MissingContext != "" {
			// Not stored: the store keys on predicate names and this is free
			// text. Logged so the gap is at least visible to whoever tunes
			// retrieval, rather than silently discarded.
			logging.Get(logging.CategoryContext).Info(
				"Context gap reported by the model (session %s turn %d): %s",
				rec.SessionID, rec.TurnNumber, rec.MissingContext)
		}
	}()
}
