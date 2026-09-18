// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains the Continuation Protocol for multi-step task execution.
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// CONTINUATION PROTOCOL - Multi-Step Task Execution
// =============================================================================

// checkContinuation checks if there are pending subtasks after shard execution.
// Returns the next continueMsg if more work is needed, nil otherwise.
// This is called from processInput after each shard completes.
func (m *Model) checkContinuation(shardType, task string, ret observation.Return) *continueMsg {
	// Inject result facts to enable continuation derivation
	m.injectShardResultFacts(shardType, task, ret, nil)

	if m.kernel == nil {
		return nil
	}

	// Query for continuation signal
	shouldContinue, _ := m.kernel.Query("should_auto_continue")
	pending, _ := m.kernel.Query("has_pending_subtask")

	if len(shouldContinue) > 0 && len(pending) > 0 {
		// Extract first pending subtask
		subtask := pending[0]
		if len(subtask.Args) >= 3 {
			nextID, _ := subtask.Args[0].(string)
			nextDesc, _ := subtask.Args[1].(string)
			nextShard := types.StripAtomPrefix(types.ExtractString(subtask.Args[2]))

			// Check if this is a mutation operation
			isMutation := isMutationOperation(nextShard)
			return &continueMsg{
				subtaskID:   nextID,
				description: nextDesc,
				shardType:   nextShard,
				isMutation:  isMutation,
			}
		}
	}

	return nil
}

// executeSubtask executes a single subtask and checks for continuation.
// Returns either a continueMsg (more work) or continuationDoneMsg (complete).
func (m Model) executeSubtask(subtaskID, description, shardType string) tea.Cmd {
	return func() tea.Msg {
		// Use per-shard execution timeout from config when available.
		timeout := config.GetLLMTimeouts().ShardExecutionTimeout
		if m.Config != nil {
			profile := m.Config.GetShardProfile(shardType)
			if profile.MaxExecutionTimeSec > 0 {
				timeout = time.Duration(profile.MaxExecutionTimeSec) * time.Second
			}
		}

		// Log start of subtask execution
		logging.Get(logging.CategoryRouting).Info("Executing subtask %s (%s) with shard %s", subtaskID, description, shardType)

		// Parent on the session's shutdown context so Ctrl+X cancels the
		// in-flight shard call, not just the next step's pre-flight check.
		// processInput does the same; a bare Background here made
		// continuation steps uncancellable once started.
		parent := m.shutdownCtx
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()

		if m.isInterrupted {
			return continuationDoneMsg{
				stepCount: m.continuationStep,
				summary:   "Stopped by user (Ctrl+X)",
				outcome:   continuationInterrupted,
			}
		}

		// Build session context for shard execution
		sessionCtx := m.buildSessionContext(ctx)

		// Execute the shard with queue backpressure (user-initiated = high priority)
		ret, err := m.spawnTaskWithContext(ctx, shardType, description, sessionCtx, types.PriorityHigh)

		// Inject result facts into kernel
		m.injectShardResultFacts(shardType, description, ret, err)

		payload := &ShardResultPayload{
			ShardType: shardType,
			Task:      description,
			Result:    ret.Output,
			Facts:     nil,
		}

		if err != nil {
			return continuationDoneMsg{
				stepCount:            m.continuationStep,
				summary:              fmt.Sprintf("Step %d failed: %v", m.continuationStep, err),
				completedShardResult: payload,
				outcome:              continuationFailed,
			}
		}

		// Check for more pending subtasks from kernel
		if m.kernel != nil {
			// Query for continuation signal
			shouldContinue, _ := m.kernel.Query("should_auto_continue")
			pending, _ := m.kernel.Query("has_pending_subtask")

			if len(shouldContinue) > 0 && len(pending) > 0 {
				// Extract first pending subtask
				subtask := pending[0]
				if len(subtask.Args) >= 3 {
					nextID, _ := subtask.Args[0].(string)
					nextDesc, _ := subtask.Args[1].(string)
					nextShard := types.StripAtomPrefix(types.ExtractString(subtask.Args[2]))

					// Check if this is a mutation operation
					isMutation := isMutationOperation(nextShard)

					// Update total steps if we discover more (send to Update thread)
					newTotal := m.continuationTotal
					if m.continuationStep >= m.continuationTotal {
						newTotal = m.continuationStep + 1
					}

					return continueMsg{
						subtaskID:            nextID,
						description:          nextDesc,
						shardType:            nextShard,
						isMutation:           isMutation,
						totalSteps:           newTotal,
						completedShardResult: payload,
					}
				}
			}
		}

		// No further continuation was derived. That says nothing about whether
		// THIS step succeeded, which is why the summary is derived from the
		// step's own verdict and evidence rather than from having run out of
		// work.
		return continuationDoneMsg{
			stepCount:            m.continuationStep,
			summary:              continuationSummary(m.continuationStep, ret),
			completedShardResult: payload,
			outcome:              continuationOutcomeFor(ret, nil),
		}
	}
}

// injectShardResultFacts injects shard execution results into the kernel.
// This enables the continuation protocol to detect follow-up work.
//
// Every fact here is derived from what the runtime OBSERVED — the kernel's
// turn verdict, the gate outcomes, the write set, the structured findings —
// and none of it from the result text. Until 2026-09-18 this function read the
// prose: "TODO" or "FIXME" anywhere in the output made the step /incomplete, a
// coder result that happened not to contain the word "test" became
// /code_generated and always owed a test, and the word "issue" in a reviewer's
// output raised a review follow-up. A coder that wrote the string "TODO" into
// a comment declared its own turn incomplete; one that wrote a full test suite
// and said so as "spec" owed another. Those three substring tests are deleted.
func (m *Model) injectShardResultFacts(shardType, task string, ret observation.Return, err error) {
	if m.kernel == nil {
		return
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
	status := shardResultStatus(ret, err)

	// Assert shard_result fact
	resultFact := core.Fact{
		Predicate: "shard_result",
		Args: []any{
			taskID,
			status,
			"/" + shardType,
			task,
			truncateSummary(ret.Output, 200),
		},
	}
	_ = m.kernel.Assert(resultFact)

	// A test obligation is owed by the evidence, not by the vocabulary of the
	// answer: Go source changed and either nothing ran the tests or the
	// executor named files it wrote with no test alongside them.
	if pendingTestOwed(ret) {
		testFact := core.Fact{
			Predicate: "pending_test",
			Args: []any{
				fmt.Sprintf("test_%d", time.Now().UnixNano()),
				fmt.Sprintf("Write tests for: %s", truncateSummary(task, 100)),
			},
		}
		_ = m.kernel.Assert(testFact)
	}

	// A review obligation is owed by structured findings — the reviewer's own
	// findings, or the critic's on a write turn — not by the substring "issue".
	if len(ret.Findings) > 0 {
		reviewFact := core.Fact{
			Predicate: "pending_review",
			Args: []any{
				fmt.Sprintf("review_%d", time.Now().UnixNano()),
				fmt.Sprintf("Fix %d finding(s) reported in: %s", len(ret.Findings), truncateSummary(task, 100)),
			},
		}
		_ = m.kernel.Assert(reviewFact)
	}
}

// shardResultStatus derives the Status argument of shard_result/5 from the
// producer's verdict and evidence.
//
// An absent verdict is deliberately NOT completion. A producer that recorded
// no outcome observed nothing about whether the work holds, and the whole
// point of this seam is that "nobody checked" and "it worked" stop sharing a
// status atom.
func shardResultStatus(ret observation.Return, err error) string {
	if err != nil {
		return "/failed"
	}
	switch ret.Outcome {
	case "/failed":
		return "/failed"
	case "/hollow":
		// Nothing the intent required actually happened, so the same shard
		// still owes the work: /incomplete is what the continuation policy
		// routes back to the same shard.
		return "/incomplete"
	case "/done":
		return "/complete"
	}
	if changedGoSource(ret.Changed) {
		return "/code_generated"
	}
	return "/unverified"
}

// pendingTestOwed reports whether this run leaves a test obligation behind.
func pendingTestOwed(ret observation.Return) bool {
	// The executor names the files it wrote with no test file alongside them.
	// That list IS the obligation; nothing further needs deriving.
	if len(ret.Untested) > 0 {
		return true
	}
	if !changedGoSource(ret.Changed) {
		return false
	}
	// Go source changed and no test run was observed. A run that happened and
	// reported a verdict — pass or fail — has been tested; a failing suite is
	// a different problem from an untested one.
	return ret.Tests == nil || !ret.Tests.Ran
}

// changedGoSource reports whether the observed write set touched Go source
// that is not itself a test. A turn that only wrote markdown owes no tests.
func changedGoSource(changed []string) bool {
	for _, p := range changed {
		lower := strings.ToLower(strings.TrimSpace(p))
		if strings.HasSuffix(lower, ".go") && !strings.HasSuffix(lower, "_test.go") {
			return true
		}
	}
	return false
}

// continuationOutcomeFor maps a step's verdict onto how the continuation is
// rendered. /unverified and /hollow get their own outcome so neither can be
// printed under the checkmark that means "this is done".
func continuationOutcomeFor(ret observation.Return, err error) continuationOutcome {
	if err != nil {
		return continuationFailed
	}
	switch ret.Outcome {
	case "/done":
		return continuationCompleted
	case "/failed":
		return continuationFailed
	}
	// /hollow, /unverified, and an absent verdict all mean the same thing to
	// the reader: the work was not shown to hold.
	return continuationUnverified
}

// continuationSummary states the derived verdict for a finished continuation
// and names the evidence behind it.
//
// It replaces `fmt.Sprintf("Completed %d steps successfully.", n)`, which was
// a function of the kernel deriving no NEXT step — not of this step's result.
// Live (chat session 2026-09-17 21:24): a coder step reported "Wrote 1
// file(s) ... Evidence: checks_passed. Requested behavior remains unverified
// (no acceptance contract)." and the two lines under it read "All 1 steps
// complete." and "Completed 1 steps successfully."
func continuationSummary(steps int, ret observation.Return) string {
	var headline string
	switch ret.Outcome {
	case "/done":
		headline = fmt.Sprintf("Completed %d step(s); the requested behavior was verified.", steps)
	case "/failed":
		headline = fmt.Sprintf("Ran %d step(s); the last one failed.", steps)
	case "/hollow":
		headline = fmt.Sprintf("Ran %d step(s), but the work the request needed was never performed.", steps)
	default:
		headline = fmt.Sprintf("Ran %d step(s); the work is not verified.", steps)
	}
	if ev := continuationEvidence(ret); ev != "" {
		return headline + " " + ev
	}
	return headline
}

// continuationEvidence renders what was actually observed, in the order a
// reader needs it: what changed, what the gates said, and what was left
// unproven.
func continuationEvidence(ret observation.Return) string {
	var parts []string
	if n := len(ret.Changed); n > 0 {
		parts = append(parts, fmt.Sprintf("changed %d file(s)", n))
	}
	parts = append(parts, verificationPhrase("build", ret.Build))
	parts = append(parts, verificationPhrase("tests", ret.Tests))
	switch {
	case ret.Acceptance != nil && ret.Acceptance.Status == "verified":
		parts = append(parts, "requested behavior verified")
	case ret.Acceptance != nil:
		parts = append(parts, "requested behavior not verified")
	case len(ret.Changed) > 0:
		parts = append(parts, "requested behavior not verified (no acceptance contract)")
	}
	if n := len(ret.Untested); n > 0 {
		parts = append(parts, fmt.Sprintf("%d written file(s) with no test alongside", n))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Evidence: " + strings.Join(parts, "; ") + "."
}

// verificationPhrase names one gate's verdict. A gate that did not run says so
// — silence would read as a pass, which is the confusion this seam exists to
// remove.
func verificationPhrase(kind string, v *observation.Verification) string {
	if v == nil {
		return kind + " did not run"
	}
	switch v.Outcome {
	case "passed":
		return kind + " passed"
	case "failed":
		return kind + " failed"
	case "skipped":
		return kind + " skipped"
	case "":
		if v.Ran {
			return kind + " ran without a verdict"
		}
		return kind + " did not run"
	default:
		return kind + " " + v.Outcome
	}
}

// isMutationOperation returns true if the shard type performs mutations.
// Used by Breakpoint mode to pause before write/run operations.
func isMutationOperation(shardType string) bool {
	mutationShards := map[string]bool{
		"coder":            true,  // Writes code
		"tool_generator":   true,  // Creates tools
		"tester":           false, // Just runs tests (read-only)
		"reviewer":         false, // Just analyzes (read-only)
		"researcher":       false, // Just gathers info (read-only)
		"debugger":         true,  // May fix bugs
		"security_auditor": false, // Just analyzes
	}
	if isMutation, exists := mutationShards[shardType]; exists {
		return isMutation
	}
	// Default: assume mutation if unknown
	return true
}

// truncateSummary truncates a string for fact storage
func truncateSummary(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
