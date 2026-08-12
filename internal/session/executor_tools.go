package session

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// runToolLoop drives the LLM ↔ tools cycle. It performs the initial generation
// and, when the model requests tools, executes them and feeds the results back
// via ToolResultsProvider for as many turns as needed (up to MaxToolIterations).
//
// Returns the final LLM response, a slice of tool error messages encountered
// across all iterations, and any fatal error.
//
// Behavior degrades gracefully:
//   - If the client implements types.ToolResultsProvider: native multi-turn loop.
//   - Otherwise: one round of tool execution then return (the pre-fix behavior).
//     Tool failures are still recorded for the caller to surface.
//   - Piggyback Protocol clients keep using their structured-output path — the
//     loop currently runs for one iteration on that path because the
//     Piggyback envelope is its own contract; extending it is future work.
func (e *Executor) runToolLoop(
	ctx context.Context,
	systemPrompt, userInput string,
	cfg *config.EffectiveAgentRuntimeConfig,
	compilationCtx *prompt.CompilationContext,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	// Resolve the turn's model once. Everything below — the initial
	// generation, the no-tool retry, and every tool-result follow-up — shares
	// one conversation history, so they must all hit the same client.
	client := e.llmForVerb(result.Intent.Verb)

	llmResponse, err := e.generateResponse(ctx, client, systemPrompt, userInput, cfg)
	if err != nil {
		return nil, nil, err
	}
	if llmResponse == nil {
		return nil, nil, errors.New("initial LLM generation returned a nil response")
	}
	e.promotePiggybackToolRequests(llmResponse)

	// No tools requested.
	//
	// Previously this was an unconditional early-return: the model could
	// reply with planning-only text ("I am creating the file...") and the
	// loop would exit with that text as the final answer — the orchestrator
	// would then mark the step "Complete" while no work had actually
	// happened (.nerd/logs/2026-05-28 shows the symptom: a /create intent
	// that produces zero side-effects and reports success).
	//
	// Mitigation, neuro-symbolic: ask the kernel whether the current
	// intent's verb requires a tool_call (Mangle rule
	// intent_requires_tool_call/1, derived from action_mapping/2 +
	// side_effecting_action/1). If yes, recompile the prompt with the
	// world_state "no_tool_call_retry" activation flag so the JIT injects
	// the system/tool_nudge/no_tool_call_retry atom (which references the
	// runtime's actually-allowed tools via {{available_tools}}, not a
	// hardcoded Go string), then reissue once.
	if len(llmResponse.ToolCalls) == 0 {
		if e.intentRequiresToolCall(result.Intent.Verb) {
			logging.Get(logging.CategorySession).Warn(
				"runToolLoop: intent_requires_tool_call(%q) derived true but model returned no tool_calls; recompiling prompt with no-tool-retry nudge atom",
				result.Intent.Verb,
			)
			retried, retryErr := e.retryWithNoToolNudge(ctx, client, userInput, cfg, compilationCtx)
			if retryErr == nil && retried != nil {
				// The retry ran under the more informative JIT prompt. Preserve it
				// even if the model still chose a prose-only conclusion; returning
				// the original planning text discards the entire retry.
				llmResponse = retried
				e.promotePiggybackToolRequests(llmResponse)
				if len(retried.ToolCalls) == 0 {
					return llmResponse, nil, nil
				}
			} else {
				if retryErr != nil {
					logging.Get(logging.CategorySession).Warn("runToolLoop: no-tool-retry path failed: %v", retryErr)
				}
				return llmResponse, nil, nil
			}
		} else {
			return llmResponse, nil, nil
		}
	}

	// Piggyback Protocol path: execute tools but don't continue the loop here.
	// (The Piggyback envelope carries tool_requests in structured output; a
	// proper loop for that path would require re-invoking with a synthesized
	// envelope. Out of scope for this fix.)
	if ptp, ok := client.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		toolErrs := e.executeToolBatchPiggyback(ctx, llmResponse.ToolCalls, cfg, result)
		verified, verifyErrs, verifyErr := e.verifyCompletedToolTurn(
			ctx, nil, systemPrompt, nil, llmResponse, e.buildToolDefinitions(cfg), cfg, result)
		toolErrs = append(toolErrs, verifyErrs...)
		return verified, toolErrs, verifyErr
	}

	// Native multi-turn tool calling required for correct semantics on
	// Anthropic/OpenAI-style providers.
	trp, supportsLoop := client.(types.ToolResultsProvider)
	toolDefs := e.buildToolDefinitions(cfg)
	executorCfg := e.configSnapshot()

	// Seed the history with the initial user turn and the assistant's
	// first response (which contains the tool_use blocks).
	history := []types.Message{
		{Role: "user", Text: userInput},
		{Role: "assistant", Text: llmResponse.Text, ToolCalls: llmResponse.ToolCalls},
	}

	budget := newToolBudgetController(executorCfg)
	finalizationCutoff, finalizationReserve, hasFinalizationCutoff :=
		toolExplorationCutoff(ctx, executorCfg.FinalAnswerReserve)
	// A client that cannot consume tool results has no final follow-up phase to
	// reserve; it retains the existing one-batch graceful-degradation behavior.
	hasFinalizationCutoff = hasFinalizationCutoff && supportsLoop

	var toolErrs []string
	currentResponse := llmResponse
	verifyTerminal := func(response *types.LLMToolResponse) (*types.LLMToolResponse, error) {
		verified, verifyErrs, verifyErr := e.verifyCompletedToolTurn(
			ctx, trp, systemPrompt, history, response, toolDefs, cfg, result)
		toolErrs = append(toolErrs, verifyErrs...)
		return verified, verifyErr
	}

	for iter := 0; iter < budget.iterationLimit; iter++ {
		if ctx.Err() != nil {
			return currentResponse, toolErrs, ctx.Err()
		}

		// A deadline is a second budget, independent of MaxToolIterations. Keep
		// its tail for a conclusion instead of starting another open-ended
		// provider call that can only end in context deadline exceeded.
		if hasFinalizationCutoff && !time.Now().Before(finalizationCutoff) {
			final, finalErrs, finalErr := e.forceDeadlineFinalAnswer(
				ctx, trp, systemPrompt, &history, currentResponse, cfg, result,
				false, finalizationReserve)
			toolErrs = append(toolErrs, finalErrs...)
			if finalErr != nil {
				return currentResponse, toolErrs, finalErr
			}
			verified, verifyErr := verifyTerminal(final)
			return verified, toolErrs, verifyErr
		}

		explorationCtx := ctx
		cancelExploration := func() {}
		if hasFinalizationCutoff {
			explorationCtx, cancelExploration = context.WithDeadline(ctx, finalizationCutoff)
		}

		// Execute all tool calls from this turn and collect tool_result blocks.
		toolResults, batchErrs := e.executeToolBatch(explorationCtx, currentResponse.ToolCalls, cfg, result)
		toolErrs = append(toolErrs, batchErrs...)
		budget.observe(currentResponse.ToolCalls, toolResults)
		toolResults = appendToolBudgetNudge(toolResults, budget.nudge(
			iter+1,
			result.ToolCallsExecuted,
			e.writeOrientedIntent(result.Intent.Verb),
			hasToolDefinition(toolDefs, "apply_edits"),
		))
		if ctx.Err() != nil {
			cancelExploration()
			return currentResponse, toolErrs, ctx.Err()
		}

		// If the client can't accept tool results back, we're done after the
		// first execution pass. The model will not see the results this turn.
		if !supportsLoop {
			cancelExploration()
			logging.Get(logging.CategorySession).Warn(
				"LLM client does not implement ToolResultsProvider; tool results not fed back to model. Provider=%T", client)
			verified, verifyErr := verifyTerminal(currentResponse)
			return verified, toolErrs, verifyErr
		}

		// Append the user tool_result turn to history and re-invoke.
		history = append(history, types.Message{
			Role:        "user",
			ToolResults: toolResults,
		})

		// A tool can itself reach the exploration cutoff. Its result (including
		// any cancellation error) is already paired in history, so do not run
		// it again on the forced-final path.
		if hasFinalizationCutoff && explorationCtx.Err() != nil && ctx.Err() == nil {
			cancelExploration()
			final, finalErrs, finalErr := e.forceDeadlineFinalAnswer(
				ctx, trp, systemPrompt, &history, currentResponse, cfg, result,
				true, finalizationReserve)
			toolErrs = append(toolErrs, finalErrs...)
			if finalErr != nil {
				return currentResponse, toolErrs, finalErr
			}
			verified, verifyErr := verifyTerminal(final)
			return verified, toolErrs, verifyErr
		}

		nextResp, err := trp.CompleteWithToolResults(explorationCtx, systemPrompt, history, toolDefs)
		plannedFinalization := hasFinalizationCutoff && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
		cancelExploration()
		if plannedFinalization {
			final, finalErrs, finalErr := e.forceDeadlineFinalAnswer(
				ctx, trp, systemPrompt, &history, currentResponse, cfg, result,
				true, finalizationReserve)
			toolErrs = append(toolErrs, finalErrs...)
			if finalErr != nil {
				return currentResponse, toolErrs, finalErr
			}
			verified, verifyErr := verifyTerminal(final)
			return verified, toolErrs, verifyErr
		}
		if err != nil {
			return currentResponse, toolErrs, describeToolLoopFailure(ctx, iter, len(toolResults), err)
		}
		if nextResp == nil {
			return currentResponse, toolErrs, errors.New("tool-result follow-up returned a nil response")
		}
		e.promotePiggybackToolRequests(nextResp)
		currentResponse = nextResp

		// Append the next assistant turn to history (whether or not it has more tool calls).
		history = append(history, types.Message{
			Role:      "assistant",
			Text:      nextResp.Text,
			ToolCalls: nextResp.ToolCalls,
		})

		if len(nextResp.ToolCalls) == 0 {
			verified, verifyErr := verifyTerminal(currentResponse)
			return verified, toolErrs, verifyErr
		}

		// The model still has executable work at the current boundary. The
		// orchestrator may extend only when the trace since the prior boundary
		// contains intent-appropriate material progress and no deterministic
		// repeat cycle or write-task read-only stall.
		if iter+1 >= budget.iterationLimit {
			decision := budget.maybeExtend(e.writeOrientedIntent(result.Intent.Verb))
			if decision.Granted {
				logging.Get(logging.CategorySession).Warn(
					"Adaptive tool budget extended by %d rounds to %d after %d executed tool call(s): %s",
					decision.AddedRounds, decision.NewLimit, result.ToolCallsExecuted, decision.Reason)
			} else {
				logging.Get(logging.CategorySession).Warn(
					"Adaptive tool budget refused extension at %d rounds after %d executed tool call(s): %s",
					budget.iterationLimit, result.ToolCallsExecuted, decision.Reason)
			}
		}
	}

	// Iteration budget exhausted. currentResponse still holds UNEXECUTED tool
	// calls and, on every provider observed, no assistant text at all — a model
	// that is still calling tools has not written its answer yet.
	//
	// Returning it here is how `nerd review internal/types/mangle_scale.go`
	// printed "📋 Result:" followed by nothing after 16 successful tool calls and
	// 2m42s of work, and exited 0. The exploration was fine; the harness simply
	// walked away before asking for the conclusion.
	//
	// So spend one more call to collect it: execute the pending batch (keeping
	// the tool_use/tool_result pairing every provider requires), then re-invoke
	// with the EXPLORATION tools removed. Unable to read anything more, the
	// model must work with what it has.
	//
	// Write tools survive that cut for a write-oriented intent. Stripping every
	// tool made `nerd create <doc>` structurally impossible to complete: it
	// spent 35 calls researching, hit the ceiling, and the tool-free final call
	// could only describe the file it had been asked to write — which the
	// hollow-success guard then correctly failed. "You have explored enough,
	// now do the thing" is the instruction that turn needed; "you have explored
	// enough, now describe the thing" is not.
	logging.Get(logging.CategorySession).Warn(
		"Tool iteration budget reached: %d rounds (base %d, hard %d, extensions %d/%d); forcing a final answer from %d executed tool call(s)",
		budget.iterationLimit, budget.baseLimit, budget.hardLimit, budget.extensions,
		budget.maxExtensions, result.ToolCallsExecuted)

	final, finalErrs, finalErr := e.forceFinalAnswer(ctx, trp, systemPrompt, &history, currentResponse, cfg, result)
	toolErrs = append(toolErrs, finalErrs...)
	if finalErr != nil {
		logging.Get(logging.CategorySession).Error(
			"Forced final answer failed after exhausting tool iterations: %v", finalErr)
		return currentResponse, toolErrs, fmt.Errorf("tool iteration budget exhausted (%d iterations, %d tool calls executed): forced final answer failed: %w", budget.iterationLimit, result.ToolCallsExecuted, finalErr)
	}
	verified, verifyErr := verifyTerminal(final)
	return verified, toolErrs, verifyErr
}

// verifyCompletedToolTurn is the transport-independent post-edit gate. Native
// and Piggyback calls differ in how tool results return to the model, but both
// must compile and test durable Go edits before the turn can report success.
// A Piggyback client has no native repair channel, so hard-gate failures remain
// failures with the compiler/test output instead of being silently skipped.
func (e *Executor) verifyCompletedToolTurn(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	current *types.LLMToolResponse,
	toolDefs []types.ToolDefinition,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if current == nil {
		return nil, nil, errors.New("cannot verify a nil completed response")
	}

	var toolErrs []string
	repaired, repairErrs, repairErr := e.verifyAndRepairBuild(
		ctx, trp, systemPrompt, history, current, toolDefs, cfg, result)
	toolErrs = append(toolErrs, repairErrs...)
	if repairErr != nil {
		return current, toolErrs, repairErr
	}
	if repaired != nil {
		current = repaired
	}

	// Compile first: test output wrapped around compiler errors is a worse
	// repair signal than the compiler's direct output.
	tested, testErrs, testErr := e.verifyAndRepairTests(
		ctx, trp, systemPrompt, history, toolDefs, cfg, result)
	toolErrs = append(toolErrs, testErrs...)
	if testErr != nil {
		return current, toolErrs, testErr
	}
	if tested != nil {
		current = tested
	}

	// The critic is advisory; only its resulting edits can fail the turn through
	// the mechanical rechecks inside verifyAndUpliftWithCritic.
	uplifted, upliftErrs, upliftErr := e.verifyAndUpliftWithCritic(
		ctx, trp, systemPrompt, history, toolDefs, cfg, result)
	toolErrs = append(toolErrs, upliftErrs...)
	if uplifted != nil {
		current = uplifted
	}
	if upliftErr != nil {
		return current, toolErrs, upliftErr
	}
	return current, toolErrs, nil
}

// toolExplorationCutoff divides a deadline-bound turn into exploration and
// finalization phases. Long turns reserve five minutes by default; short turns
// reserve half of whatever remains so both phases retain a usable budget.
func toolExplorationCutoff(ctx context.Context, configuredReserve time.Duration) (time.Time, time.Duration, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Time{}, 0, false
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		return deadline, 0, true
	}
	if configuredReserve <= 0 {
		configuredReserve = defaultFinalAnswerReserve
	}
	reserve := configuredReserve
	if half := remaining / 2; reserve > half {
		reserve = half
	}
	return deadline.Add(-reserve), reserve, true
}

// forceDeadlineFinalAnswer transitions from exploration to conclusion while
// preserving provider tool-use/tool-result pairing. If a pending batch did not
// run before the cutoff, it is paired with explicit cancellation results; the
// model gets an honest gap instead of an invalid conversation or a duplicated
// side effect.
func (e *Executor) forceDeadlineFinalAnswer(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history *[]types.Message,
	pending *types.LLMToolResponse,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
	pendingResultsRecorded bool,
	reserve time.Duration,
) (*types.LLMToolResponse, []string, error) {
	if history == nil {
		empty := []types.Message{}
		history = &empty
	}
	if !pendingResultsRecorded && pending != nil && len(pending.ToolCalls) > 0 {
		results := make([]types.ToolResult, 0, len(pending.ToolCalls))
		for _, call := range pending.ToolCalls {
			results = append(results, types.ToolResult{
				ToolUseID: call.ID,
				Content:   "not executed: the turn entered its reserved final-answer window",
				IsError:   true,
			})
		}
		*history = append(*history, types.Message{Role: "user", ToolResults: results})
	}

	withoutPendingCalls := &types.LLMToolResponse{}
	if pending != nil {
		copy := *pending
		copy.ToolCalls = nil
		withoutPendingCalls = &copy
	}

	logging.Get(logging.CategorySession).Warn(
		"Tool exploration reached its deadline boundary; reserving %s for a forced final answer after %d executed tool call(s)",
		reserve.Round(time.Second), result.ToolCallsExecuted)
	final, toolErrs, err := e.forceFinalAnswer(
		ctx, trp, systemPrompt, history, withoutPendingCalls, cfg, result)
	if err != nil {
		return pending, toolErrs, fmt.Errorf(
			"deadline-aware forced final answer failed with %s reserved: %w", reserve.Round(time.Second), err)
	}
	return final, toolErrs, nil
}

// Nudges appended as the final user turn when the loop runs out of iterations.
// Both are explicit that partial evidence is acceptable — the alternative the
// model would otherwise pick is another exploration call, which is exactly what
// it can no longer make.
const (
	readOnlyBudgetExhaustedNudge = "Your tool budget for this turn is exhausted; no further tools are available. " +
		"Write your final answer now using only the evidence you have already gathered. " +
		"State explicitly what you could not verify rather than requesting more tools."

	writeBudgetExhaustedNudge = "Your exploration budget for this turn is exhausted. Only write tools remain: " +
		"you cannot read, search, or list anything further. Produce the deliverable NOW with the write tool, " +
		"using only the evidence you have already gathered. Note any uncertainty inside the artifact itself " +
		"rather than deferring the write — describing what you would have written does not count as doing it."
)

// forceFinalAnswer executes any still-pending tool calls and then re-invokes the
// model with exploration tools removed.
//
// For a read-oriented intent no tools survive, so the model must produce prose.
// For a write-oriented intent the write tools survive, so it can still land the
// artifact it was asked for; anything else makes a large task structurally
// impossible to finish rather than merely truncated.
func (e *Executor) forceFinalAnswer(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history *[]types.Message,
	pending *types.LLMToolResponse,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if trp == nil {
		return pending, nil, errors.New("client does not support tool-result follow-up")
	}
	if pending == nil {
		return nil, nil, errors.New("cannot force a final answer from a nil pending response")
	}
	if history == nil {
		empty := []types.Message{}
		history = &empty
	}

	var toolErrs []string
	if len(pending.ToolCalls) > 0 {
		toolResults, errs := e.executeToolBatch(ctx, pending.ToolCalls, cfg, result)
		toolErrs = append(toolErrs, errs...)
		*history = append(*history, types.Message{Role: "user", ToolResults: toolResults})
	}

	// Only intents whose terminal contract specifically requires a durable file
	// mutation retain write tools. intentRequiresToolCall is broader: /commit,
	// /test, and /research also require real tools, but offering write_file as
	// their only final capability would not let them complete correctly.
	needsWrite := e.writeOrientedIntent(result.Intent.Verb) && result.SuccessfulWriteTools == 0

	nudge := readOnlyBudgetExhaustedNudge
	var finalTools []types.ToolDefinition
	if needsWrite {
		nudge = writeBudgetExhaustedNudge
		finalTools = writeOnlyToolDefinitions(e.buildToolDefinitions(cfg))
		logging.Get(logging.CategorySession).Warn(
			"Retaining %d write tool(s) for the final call: %s requires a side effect and none has landed yet",
			len(finalTools), result.Intent.Verb)
	}

	*history = append(*history, types.Message{Role: "user", Text: nudge})

	final, err := trp.CompleteWithToolResults(ctx, systemPrompt, *history, finalTools)
	if err != nil {
		return pending, toolErrs, fmt.Errorf("final completion failed: %w", err)
	}
	if final == nil {
		return pending, toolErrs, errors.New("final completion returned nothing")
	}
	e.promotePiggybackToolRequests(final)

	// Preserve original tool calls for history before clearing. The returned
	// response must clear ToolCalls so callers do not replay them, but the
	// conversation history must retain a balanced tool_use/tool_result pair
	// for every call so that subsequent verification/repair calls are valid
	// with strict providers (e.g. Meta: Missing tool response for tool_call_id).
	originalFinalCalls := append([]types.ToolCall(nil), final.ToolCalls...)
	*history = append(*history, types.Message{Role: "assistant", Text: final.Text, ToolCalls: originalFinalCalls})

	offered := make(map[string]struct{}, len(finalTools))
	for _, definition := range finalTools {
		offered[definition.Name] = struct{}{}
	}
	permittedFinalCalls := make([]types.ToolCall, 0, len(final.ToolCalls))
	for _, call := range final.ToolCalls {
		if _, ok := offered[call.Name]; !ok {
			logging.Get(logging.CategorySession).Warn(
				"Forced final answer requested unoffered tool %s; refusing execution", call.Name)
			toolErrs = append(toolErrs, fmt.Sprintf(
				"%s: not offered during forced finalization", call.Name))
			continue
		}
		permittedFinalCalls = append(permittedFinalCalls, call)
	}

	hadPermittedFinalCalls := len(permittedFinalCalls) > 0
	var permittedResults []types.ToolResult
	if hadPermittedFinalCalls {
		var errs []string
		permittedResults, errs = e.executeToolBatch(ctx, permittedFinalCalls, cfg, result)
		toolErrs = append(toolErrs, errs...)
	}
	// Build balanced ToolResults for every final call in original order:
	// real executeToolBatch result for permitted writes, synthetic IsError
	// for refused/unoffered calls.
	if len(originalFinalCalls) > 0 {
		permittedMap := make(map[string]types.ToolResult, len(permittedResults))
		for _, r := range permittedResults {
			permittedMap[r.ToolUseID] = r
		}
		finalResults := make([]types.ToolResult, 0, len(originalFinalCalls))
		for _, call := range originalFinalCalls {
			if r, ok := permittedMap[call.ID]; ok {
				finalResults = append(finalResults, r)
			} else {
				// Refused/unoffered synthetic result.
				finalResults = append(finalResults, types.ToolResult{
					ToolUseID: call.ID,
					Content:   fmt.Sprintf("%s: not offered during forced finalization", call.Name),
					IsError:   true,
				})
			}
		}
		*history = append(*history, types.Message{Role: "user", ToolResults: finalResults})
	}
	// Offered calls have run; unoffered calls were refused. Neither is pending
	// work for a caller to replay.
	final.ToolCalls = nil

	if strings.TrimSpace(final.Text) == "" && !hadPermittedFinalCalls {
		return pending, toolErrs, errors.New("final completion returned neither text nor a tool call")
	}
	return final, toolErrs, nil
}

// writeOnlyToolDefinitions keeps the write-mutation tools and drops the rest.
//
// The exploration tools have to go: a model given read_file back will use it,
// and the budget was exhausted precisely because reading is the cheap, endless
// option.
func writeOnlyToolDefinitions(defs []types.ToolDefinition) []types.ToolDefinition {
	writes := make([]types.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if isWriteMutationTool(def.Name) {
			writes = append(writes, def)
		}
	}
	return writes
}

// executeToolBatch runs one turn's worth of tool calls and returns the
// tool_result blocks plus any error strings. Extracted from runToolLoop so the
// forced-final-answer path produces byte-identical tool_result framing; a
// hand-rolled second copy would drift on budget handling or ID pairing.
func (e *Executor) executeToolBatch(
	ctx context.Context,
	calls []types.ToolCall,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) ([]types.ToolResult, []string) {
	toolResults := make([]types.ToolResult, 0, len(calls))
	var toolErrs []string
	maxToolCalls := effectiveMaxToolCalls(e.configSnapshot().MaxToolCalls)

	for _, call := range calls {
		if err := ctx.Err(); err != nil {
			for _, skipped := range calls[len(toolResults):] {
				toolResults = append(toolResults, types.ToolResult{
					ToolUseID: skipped.ID,
					Content:   "not executed: tool batch context ended: " + err.Error(),
					IsError:   true,
				})
				toolErrs = append(toolErrs, fmt.Sprintf("%s: %v", skipped.Name, err))
			}
			break
		}
		if result.ToolCallsExecuted >= maxToolCalls {
			logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", maxToolCalls)
			toolResults = append(toolResults, types.ToolResult{
				ToolUseID: call.ID,
				Content:   "tool call budget exceeded for this turn",
				IsError:   true,
			})
			toolErrs = append(toolErrs, fmt.Sprintf("%s: budget exceeded", call.Name))
			continue
		}

		out, execErr := e.executeAndRecordToolCall(ctx, call, cfg, result)

		if execErr != nil {
			logging.Get(logging.CategorySession).Error("Tool call %s failed: %v", call.Name, execErr)
			toolErrs = append(toolErrs, fmt.Sprintf("%s: %v", call.Name, execErr))
			toolResults = append(toolResults, types.ToolResult{
				ToolUseID: call.ID,
				Content:   execErr.Error(),
				IsError:   true,
			})
			continue
		}

		logging.SessionDebug("Tool %s executed successfully: %d chars result", call.Name, len(out))
		toolResults = append(toolResults, types.ToolResult{
			ToolUseID: call.ID,
			Content:   truncateToolResult(out),
			IsError:   false,
		})
	}

	return toolResults, toolErrs
}

// executeAndRecordToolCall is the single accounting path shared by native and
// Piggyback batches. The two transports frame results differently, but a call
// attempt, successful effect, write count, and written path must never drift.
func (e *Executor) executeAndRecordToolCall(
	ctx context.Context,
	call types.ToolCall,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (string, error) {
	out, err := e.executeToolCall(ctx, ToolCall{ID: call.ID, Name: call.Name, Args: call.Input}, cfg)
	result.ToolCallsExecuted++
	if err != nil {
		return "", err
	}

	result.SuccessfulToolCalls++
	if isWriteMutationTool(call.Name) {
		result.SuccessfulWriteTools++
		if err := recordWrittenPaths(result, call.Input, e.workspaceForVerification()); err != nil {
			logging.Get(logging.CategorySession).Warn(
				"successful write %s returned invalid target metadata: %v", call.Name, err)
		}
	}
	if isTestExecutionTool(call.Name, call.Input) {
		result.SuccessfulTestTools++
	}
	return out, nil
}

func recordWrittenPaths(result *ExecutionResult, args map[string]any, workspace string) error {
	paths, err := projectdoc.TargetPaths(args)
	if err != nil {
		return err
	}
	for _, path := range paths {
		normalized := canonicalizeWrittenPath(path, workspace)
		if normalized != "" && !slices.Contains(result.WrittenPaths, normalized) {
			result.WrittenPaths = append(result.WrittenPaths, normalized)
		}
	}
	return nil
}

// intentRequiresToolCall asks the Mangle kernel whether the supplied intent
// verb requires a real tool_call to make progress. The decision logic lives
// entirely in the policy corpus (delegation.mg → intent_requires_tool_call/1
// derived from action_mapping/2 + side_effecting_action/1) — this Go helper
// just queries it. When the kernel is unavailable or the query fails, we
// conservatively return false so we never block a final answer on missing
// policy.
func (e *Executor) intentRequiresToolCall(verb string) bool {
	verb = strings.TrimSpace(verb)
	validVerb := validMangleVerb(verb)
	if e.kernel == nil || !validVerb {
		if verb != "" && !validVerb {
			logging.Get(logging.CategorySession).Warn(
				"intentRequiresToolCall rejected malformed verb %q", verb)
		}
		return false
	}
	// Mangle atom constants are lowercase and slash-prefixed. The intent verb
	// arrives here as "/create", "/document", etc. — exactly the form the
	// policy expects.
	q := fmt.Sprintf("intent_requires_tool_call(%s)", verb)
	facts, err := e.kernel.Query(q)
	if err != nil {
		logging.Get(logging.CategorySession).Debug(
			"intentRequiresToolCall: kernel query %q failed: %v (defaulting to false)", q, err,
		)
		return false
	}
	return len(facts) > 0
}

// validMangleVerb admits only the atom shape emitted by perception and used by
// the policy corpus. Query helpers interpolate this value into Mangle source,
// so accepting whitespace, delimiters, or a second slash would turn malformed
// user state into a query-language fragment.
func validMangleVerb(verb string) bool {
	if len(verb) < 2 || verb[0] != '/' {
		return false
	}
	for i := 1; i < len(verb); i++ {
		c := verb[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

var writeOrientedIntentFallback = map[string]struct{}{
	"/create": {}, "/fix": {}, "/refactor": {}, "/write": {},
	"/delete": {}, "/implement": {}, "/scaffold": {}, "/optimize": {},
	"/document": {},
}

// isWriteOrientedIntent is the degraded-kernel fallback for the authoritative
// write_oriented_intent/1 policy predicate. Keep it query-free so a write turn
// still retains write tools while the kernel is unavailable; a real-kernel test
// pins this set byte-for-byte to the policy facts.
func isWriteOrientedIntent(verb string) bool {
	_, ok := writeOrientedIntentFallback[strings.TrimSpace(verb)]
	return ok
}

func (e *Executor) writeOrientedIntent(verb string) bool {
	verb = strings.TrimSpace(verb)
	if e.kernel != nil && validMangleVerb(verb) {
		facts, err := e.kernel.Query(fmt.Sprintf("write_oriented_intent(%s)", verb))
		if err == nil && len(facts) > 0 {
			return true
		}
		if err != nil {
			logging.Get(logging.CategorySession).Warn(
				"write_oriented_intent query failed for %s; using pinned fallback: %v", verb, err)
		}
	}
	// The pinned set is a conservative minimum, not an override of positive
	// kernel policy. It preserves fail-safe hollow-success behavior if a test,
	// partially initialized kernel, or older policy snapshot returns no facts.
	return isWriteOrientedIntent(verb)
}

// isWriteMutationTool reports tools that land durable file/workspace changes.
//
// This list drives two things at once, so a missing entry fails in two ways:
// checkHollowSuccess reports a successful edit as a failure, and
// projectForbidsWrite lets a nerd.md-protected path be written by a tool it
// does not recognize. The second is a hole in a safety gate, not a cosmetic bug.
//
// It must therefore cover every durable-write ActionType in
// internal/core/virtual_store_types.go. It previously did not: edit_lines,
// insert_lines and delete_lines (virtual_store_types.go:57-59, exposed as tools
// at prompt/config_factory.go:195-197 and routed RequiresSafe at
// shards/system/router.go:967-969) were all absent, while five names that are
// not registered anywhere — apply_patch, str_replace, create_file,
// replace_in_file, multi_edit — were present. The list had been written from
// generic LLM tool vocabulary rather than from this codebase's registry.
// Observed live: codeNERD landed a correct one-line insert via insert_lines and
// the run failed with "write-oriented intent /fix completed without
// write_file/edit_file (tool_calls=16)".
//
// TestIsWriteMutationTool_CoversEveryDurableWriteAction pins it to the registry.
func isWriteMutationTool(name string) bool {
	// Single source of truth: projectdoc.IsWriteMutationTool. Kept as a thin
	// wrapper so this package's tests and call sites are unchanged, and so the
	// session gate and the registry-level guard cannot drift apart.
	return projectdoc.IsWriteMutationTool(name)
}

func isTestExecutionTool(name string, args map[string]any) bool {
	return projectdoc.IsTestExecutionTool(name, args)
}

// SetProjectDoc attaches the workspace's parsed nerd.md.
//
// Only the prose rendering is held here. Write protection is enforced by
// querying the kernel (see projectForbidsWrite), so a subagent that never
// receives this pointer is still governed by the same rules — a safety gate
// that depends on a field being wired at every construction site is a gate that
// is off wherever someone forgot.
func (e *Executor) SetProjectDoc(doc *projectdoc.Document) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.projectDoc = doc
}

// withProjectInstructions appends nerd.md's rendered instructions to a compiled
// system prompt.
//
// The frontmatter is restated in prose even though it is already in the kernel:
// the model cannot read the fact store, and learning that a path is protected
// by being denied mid-edit costs a whole turn.
func (e *Executor) withProjectInstructions(systemPrompt string) string {
	e.mu.RLock()
	doc := e.projectDoc
	e.mu.RUnlock()

	section := doc.PromptSection()
	if section == "" {
		return systemPrompt
	}
	logging.Session("Injected %s instructions into system prompt (%d chars)", doc.Path, len(section))
	return systemPrompt + "\n\n" + section
}

// FileContextProvider is the narrow interface for per-file holographic context.
//
// Declared in package session so nothing needs to import internal/world and no
// import cycle is possible. Mirrors HolographicProvider.PromptSection.
type FileContextProvider interface {
	PromptSection(ctx context.Context, filePath string) string
}

// SetFileContextProvider attaches the holographic per-file context provider.
func (e *Executor) SetFileContextProvider(p FileContextProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fileContext = p
}

// withFileContext appends holographic per-file context to a compiled system
// prompt. Mirrors withProjectInstructions: returns systemPrompt unchanged when
// the provider is nil, the target is empty, or the rendered section is empty.
func (e *Executor) withFileContext(ctx context.Context, systemPrompt, target string) string {
	if strings.TrimSpace(target) == "" {
		return systemPrompt
	}
	e.mu.RLock()
	p := e.fileContext
	e.mu.RUnlock()
	if p == nil {
		return systemPrompt
	}
	section := p.PromptSection(ctx, target)
	if strings.TrimSpace(section) == "" {
		return systemPrompt
	}
	logging.Session("Injected holographic context for %s into system prompt (%d chars)", target, len(section))
	return systemPrompt + "\n\n" + section
}

// projectDocPathArgs are the argument names a write-mutation tool may use to
// name its target. Tools disagree ("path", "file_path", "file", "filename"), so
// the gate checks all of them rather than trusting one convention — a
// write-protection rule that only fires for tools using the arg name we guessed
// is a gate with holes in it.
var projectDocPathArgs = projectdoc.PathArgs

func canonicalizeWrittenPath(target, workspace string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	clean := filepath.Clean(target)
	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean)
	}
	if workspace != "" {
		ws := strings.TrimSpace(workspace)
		if ws != "" {
			if absWS, err := filepath.Abs(ws); err == nil {
				absWS = filepath.Clean(absWS)
				if rel, err := filepath.Rel(absWS, clean); err == nil {
					if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
						return filepath.ToSlash(rel)
					}
				}
			}
		}
	}
	return filepath.ToSlash(clean)
}

// projectDocTargetPath extracts the target path from a tool call's arguments.
func projectDocTargetPath(args map[string]any) string {
	return projectdoc.TargetPath(args)
}

func projectDocTargetLabel(args map[string]any) string {
	paths, err := projectdoc.TargetPaths(args)
	if err != nil || len(paths) == 0 {
		return "unknown"
	}
	return strings.Join(paths, ", ")
}

// pendingEditContentKeys are argument names that may carry the file content
// for a write-mutation tool. Different tools use different conventions
// ("content", "new_content", "text", etc.), so we check all known variants.
var pendingEditContentKeys = []string{"content", "new_content", "newContent", "text", "body", "data", "patch", "edits"}

// pendingEditContent extracts the content payload from a tool call's arguments.
// Returns "" when no content key is present — e.g. delete_file/delete_lines
// which intentionally carry no body.
func pendingEditContent(args map[string]any) string {
	for _, key := range pendingEditContentKeys {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok {
				return boundedPendingEditContent(s)
			}
			// Non-string payloads (e.g. JSON objects) — stringify for the fact.
			if raw != nil {
				if b, err := json.Marshal(raw); err == nil {
					return boundedPendingEditContent(string(b))
				}
				return boundedPendingEditContent(fmt.Sprintf("%v", raw))
			}
		}
	}
	return ""
}

const maxPendingEditContentBytes = 16 * 1024

// boundedPendingEditContent prevents a single file body from being copied into
// the kernel's fact store. Current policy binds only FilePath and treats Content
// as anonymous; retaining a digest preserves identity for diagnostics without
// retaining an unbounded source blob.
func boundedPendingEditContent(content string) string {
	if len(content) <= maxPendingEditContentBytes {
		return content
	}
	digest := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x bytes:%d", digest, len(content))
}

// assertPendingEdits asserts pending_edit(FilePath, Content) for every target
// in a write-mutation tool call. Paths and content are derived via the
// existing path/content extraction helpers, and the classification reuses
// isWriteMutationTool so the predicate stays in sync with the write-mutation
// registry (see isWriteMutationTool godoc). The assertion is best-effort: kernel
// absence or assertion errors are logged and do not block the tool execution.
// It returns every asserted fact so the caller can defer matching retractions.
// A pending_edit that is asserted and
// never retracted is worse than one never asserted: the fact means "an edit is
// in flight", so a stale one makes the 26 rules that read it reason about work
// that finished long ago, and the facts accumulate without bound against the
// kernel's fact ceiling.
func (e *Executor) assertPendingEdits(call ToolCall) []types.Fact {
	if !isWriteMutationTool(call.Name) {
		return nil
	}
	if e.kernel == nil {
		return nil
	}
	paths, err := projectdoc.TargetPaths(call.Args)
	if err != nil || len(paths) == 0 {
		logging.Get(logging.CategorySession).Warn(
			"Refusing to assert pending_edit for %s without valid target paths: %v", call.Name, err)
		return nil
	}
	content := pendingEditContent(call.Args)
	facts := make([]types.Fact, 0, len(paths))
	for _, filePath := range paths {
		fact := types.Fact{Predicate: "pending_edit", Args: []any{filePath, content}}
		if err := e.kernel.Assert(fact); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to assert pending_edit for %s (%s): %v", call.Name, filePath, err)
			continue
		}
		facts = append(facts, fact)
	}
	return facts
}

// assertPendingEdit preserves the single-target test and caller seam.
func (e *Executor) assertPendingEdit(call ToolCall) (types.Fact, bool) {
	facts := e.assertPendingEdits(call)
	if len(facts) == 0 {
		return types.Fact{}, false
	}
	return facts[0], true
}

// retractPendingEdit removes the in-flight marker asserted by
// assertPendingEdit. Callers defer it so it runs on every exit path — success,
// tool error, gate refusal, timeout, or panic. Any path that can leave the fact
// behind reintroduces the stale-fact problem the retraction exists to prevent.
func (e *Executor) retractPendingEdit(fact types.Fact) {
	if e.kernel == nil {
		return
	}
	if err := e.kernel.RetractFact(fact); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to retract pending_edit for %v: %v", fact.Args, err)
	}
}

// projectForbidsWrite asks the kernel whether nerd.md protects this call's
// target path.
//
// The kernel is the authority, not a cached Go struct: nerd.md facts are
// asserted at boot like any other EDB, so policy, /query, and this gate all see
// exactly the same rules. A parallel in-memory copy would be one refactor away
// from disagreeing with what the kernel actually holds.
//
// Only write-mutation tools are gated. Reading a protected file is fine and
// often necessary — the point is to stop the agent editing it.
func (e *Executor) projectForbidsWrite(call ToolCall) (string, bool) {
	if !isWriteMutationTool(call.Name) {
		return "", false
	}
	paths, err := projectdoc.TargetPaths(call.Args)
	if err != nil {
		reason := fmt.Sprintf("invalid write targets: %v", err)
		logging.Get(logging.CategorySession).Warn(
			"nerd.md BLOCKED %s due to invalid targets: %v", call.Name, err)
		logging.Audit().SafetyCheck("nerd.md_write_guard", false, reason)
		return reason, true
	}
	if len(paths) == 0 {
		return "write target is missing or uses an unrecognized path argument", true
	}
	if e.kernel == nil {
		return "nerd.md write protection authority is unavailable", true
	}

	// Matching lives in projectdoc.ForbiddenByKernel so this gate and the
	// VirtualStore's cannot drift apart. They used to be one gate and one hole:
	// shards route writes through the VirtualStore, which checked nothing.
	// Evaluate every target so one protected nested path blocks the entire call.
	// Fail closed on parser error (above) or kernel evaluation error.
	for _, target := range paths {
		reason, forbidden, kerr := projectdoc.ForbiddenByKernel(e.kernel, target)
		if kerr != nil {
			// Fail closed. nerd.md's machine-readable protection is an executive
			// constraint, and a kernel failure means the executor cannot prove that
			// this write is allowed. Reads remain available for diagnosis.
			reason := fmt.Sprintf("write protection could not be evaluated: %v", kerr)
			logging.Get(logging.CategorySession).Warn(
				"nerd.md BLOCKED %s because protection could not be evaluated: %v", target, kerr)
			logging.Audit().SafetyCheck("nerd.md_write_guard", false, reason)
			return reason, true
		}
		if forbidden {
			return reason, true
		}
	}
	return "", false
}
// writesPlaceholderTestFile is the mechanical guard against hollow success.
//
// Prose is a request the model complies with most of the time, while a fact
// checked before the tool runs is one no amount of model conviction gets past.
// A placeholder test is the purest form of hollow success — it makes
// `go test` green while testing nothing.
//
// It mirrors projectForbidsWrite: consulted at the single chokepoint in
// executeToolCall, after the nerd.md guard and before the Dreamer preflight,
// so both guards live at one site rather than at call sites.
func (e *Executor) writesPlaceholderTestFile(call ToolCall) (string, bool) {
	if !isWriteMutationTool(call.Name) {
		return "", false
	}
	paths, err := projectdoc.TargetPaths(call.Args)
	if err != nil {
		return "", false
	}
	var testPaths []string
	for _, p := range paths {
		if strings.HasSuffix(filepath.Base(p), "_test.go") {
			testPaths = append(testPaths, p)
		}
	}
	if len(testPaths) == 0 {
		return "", false
	}
	content, present := placeholderTestContent(call.Args)
	if !present {
		return "", false
	}
	funcs := findPlaceholderTestFuncs(content)
	if len(funcs) == 0 {
		return "", false
	}
	allVacuous := true
	var firstVacuous string
	for _, fn := range funcs {
		if !isVacuousTestBody(fn.body) {
			allVacuous = false
			break
		}
		if firstVacuous == "" {
			firstVacuous = fn.name
		}
	}
	if !allVacuous {
		return "", false
	}
	reason := fmt.Sprintf("placeholder test %s in %s is vacuous (empty or only t.Skip/t.SkipNow/t.Skipf); write a test that fails before the fix and passes after", firstVacuous, testPaths[0])
	return reason, true
}
// modularityGuard enforces the modularity standard at the single write choke point.
//
// Semantics: block only violations the write INTRODUCES:
//   - If the target file does not exist yet, any violation in the proposed content blocks.
//   - If the target already exists, block only violations present in the proposed content
//     that were not already present for the same function+rule in the current file.
// Compare is by function name and rule, not counting, so shrinking one violation
// while adding another still blocks.
//
// Applies only to Go source the tool is writing in full: skip when not .go,
// skip when content does not parse, do not skip _test.go. When no kernel is
// available the guard allows the write and logs at Debug; it is a standard,
// not a safety interlock.
func (e *Executor) modularityGuard(call ToolCall) (string, bool) {
	if !isWriteMutationTool(call.Name) {
		return "", false
	}
	paths, err := projectdoc.TargetPaths(call.Args)
	if err != nil || len(paths) == 0 {
		return "", false
	}
	var goPaths []string
	for _, p := range paths {
		if strings.HasSuffix(strings.ToLower(p), ".go") {
			goPaths = append(goPaths, p)
		}
	}
	if len(goPaths) == 0 {
		return "", false
	}
	if e.kernel == nil {
		logging.Get(logging.CategorySession).Debug("modularity guard skipped: kernel is nil for %s on %s", call.Name, strings.Join(goPaths, ", "))
		return "", false
	}
	proposedContent := rawModularityContent(call.Args)
	if strings.TrimSpace(proposedContent) == "" {
		return "", false
	}
	for _, goPath := range goPaths {
		proposedViolations, err := evaluateModularity(e.kernel, goPath, proposedContent)
		if err != nil {
			logging.Get(logging.CategorySession).Warn("modularity guard evaluation failed for proposed %s: %v", goPath, err)
			continue
		}
		if len(proposedViolations) == 0 {
			continue
		}
		// Resolve on-disk path for existing-file comparison.
		fsPath := e.resolveModularityFilePath(goPath)
		existingBytes, readErr := os.ReadFile(fsPath)
		if readErr != nil && fsPath != goPath {
			if altBytes, altErr := os.ReadFile(goPath); altErr == nil {
				existingBytes = altBytes
				readErr = nil
			}
		}
		if readErr != nil {
			// New file: any violation blocks.
			reason := proposedViolations[0]
			return reason, true
		}
		existingContent := string(existingBytes)
		existingViolations, err := evaluateModularity(e.kernel, goPath, existingContent)
		if err != nil {
			logging.Get(logging.CategorySession).Warn("modularity guard evaluation failed for existing %s: %v", goPath, err)
			// Fail open on existing evaluation failure? Treat as no existing violations
			// so proposed violations block — conservative for correctness.
			reason := proposedViolations[0]
			return reason, true
		}
		existingSet := make(map[string]struct{}, len(existingViolations))
		for _, v := range existingViolations {
			existingSet[modularityViolationKey(v)] = struct{}{}
		}
		for _, v := range proposedViolations {
			key := modularityViolationKey(v)
			if _, ok := existingSet[key]; !ok {
				return v, true
			}
		}
		// All proposed violations were already present for this file — allow.
	}
	return "", false
}

// rawModularityContent extracts the full file content payload without the
// 16KB truncation applied for kernel facts. A truncated hash does not parse as
// Go and would make the guard silently allow large violating files.
func rawModularityContent(args map[string]any) string {
	for _, key := range pendingEditContentKeys {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok {
				return s
			}
			if raw != nil {
				if b, err := json.Marshal(raw); err == nil {
					return string(b)
				}
				return fmt.Sprintf("%v", raw)
			}
		}
	}
	return ""
}

// modularityViolationKey extracts the function+rule identity from the
// evaluateModularity violation string "Func: predicate in file".
func modularityViolationKey(v string) string {
	colon := strings.Index(v, ":")
	if colon == -1 {
		return strings.TrimSpace(v)
	}
	funcPart := strings.TrimSpace(v[:colon])
	rest := strings.TrimSpace(v[colon+1:])
	if idx := strings.Index(rest, " in "); idx != -1 {
		pred := strings.TrimSpace(rest[:idx])
		return funcPart + ":" + pred
	}
	return funcPart + ":" + strings.TrimSpace(rest)
}

// resolveModularityFilePath maps a tool target path to a filesystem path for
// the pre-existing-file comparison. Absolute targets are used directly;
// relative targets are joined with the verification workspace when available.
func (e *Executor) resolveModularityFilePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if ws := e.workspaceForVerification(); ws != "" {
		return filepath.Join(ws, p)
	}
	return p
}


// placeholderTestContent extracts the content payload from a tool call's
// arguments using the same key set as pendingEditContent. It reports whether
// a content argument was present at all, so a pure delete/rename with no body
// is not misclassified as an empty placeholder.
func placeholderTestContent(args map[string]any) (string, bool) {
	for _, key := range pendingEditContentKeys {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok {
				return s, true
			}
			if raw != nil {
				if b, err := json.Marshal(raw); err == nil {
					return string(b), true
				}
				return fmt.Sprintf("%v", raw), true
			}
			return "", true
		}
	}
	return "", false
}

type placeholderTestFunc struct {
	name string
	body string
}

var placeholderTestFuncRe = regexp.MustCompile(`func\s+(Test\w*)\s*\(\s*t\s+\*testing\.T\s*\)`)

func findPlaceholderTestFuncs(content string) []placeholderTestFunc {
	matches := placeholderTestFuncRe.FindAllStringSubmatchIndex(content, -1)
	var out []placeholderTestFunc
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		nameStart, nameEnd := m[2], m[3]
		if nameStart < 0 || nameEnd < 0 || nameStart >= len(content) || nameEnd > len(content) {
			continue
		}
		name := content[nameStart:nameEnd]
		matchEnd := m[1]
		if matchEnd < 0 || matchEnd >= len(content) {
			continue
		}
		braceIdx := strings.Index(content[matchEnd:], "{")
		if braceIdx == -1 {
			continue
		}
		openIdx := matchEnd + braceIdx
		closeIdx := findPlaceholderMatchingBrace(content, openIdx)
		if closeIdx == -1 {
			continue
		}
		body := ""
		if openIdx+1 <= closeIdx {
			body = content[openIdx+1 : closeIdx]
		}
		out = append(out, placeholderTestFunc{name: name, body: body})
	}
	return out
}

func findPlaceholderMatchingBrace(s string, openIdx int) int {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '{' {
		return -1
	}
	depth := 1
	i := openIdx + 1
	inSingle := false
	inDouble := false
	inRaw := false
	inLineComment := false
	inBlockComment := false
	for i < len(s) {
		if inLineComment {
			if s[i] == '\n' {
				inLineComment = false
			}
			i++
			continue
		}
		if inBlockComment {
			if i+1 < len(s) && s[i] == '*' && s[i+1] == '/' {
				inBlockComment = false
				i += 2
				continue
			}
			i++
			continue
		}
		if inSingle {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if s[i] == '\'' {
				inSingle = false
			}
			i++
			continue
		}
		if inDouble {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if s[i] == '"' {
				inDouble = false
			}
			i++
			continue
		}
		if inRaw {
			if s[i] == '`' {
				inRaw = false
			}
			i++
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			inLineComment = true
			i += 2
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			inBlockComment = true
			i += 2
			continue
		}
		if s[i] == '\'' {
			inSingle = true
			i++
			continue
		}
		if s[i] == '"' {
			inDouble = true
			i++
			continue
		}
		if s[i] == '`' {
			inRaw = true
			i++
			continue
		}
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

func stripPlaceholderComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSingle := false
	inDouble := false
	inRaw := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < len(s); {
		if inLineComment {
			if s[i] == '\n' {
				inLineComment = false
				b.WriteByte(s[i])
				i++
				continue
			}
			i++
			continue
		}
		if inBlockComment {
			if i+1 < len(s) && s[i] == '*' && s[i+1] == '/' {
				inBlockComment = false
				i += 2
				b.WriteByte(' ')
				continue
			}
			i++
			continue
		}
		if inSingle {
			b.WriteByte(s[i])
			if s[i] == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
				i++
				continue
			}
			if s[i] == '\'' {
				inSingle = false
			}
			i++
			continue
		}
		if inDouble {
			b.WriteByte(s[i])
			if s[i] == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
				i++
				continue
			}
			if s[i] == '"' {
				inDouble = false
			}
			i++
			continue
		}
		if inRaw {
			b.WriteByte(s[i])
			if s[i] == '`' {
				inRaw = false
			}
			i++
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			inLineComment = true
			i += 2
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			inBlockComment = true
			i += 2
			continue
		}
		if s[i] == '\'' {
			inSingle = true
			b.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '"' {
			inDouble = true
			b.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '`' {
			inRaw = true
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isVacuousTestBody(body string) bool {
	cleaned := stripPlaceholderComments(body)
	trimmed := strings.TrimSpace(cleaned)
	if trimmed == "" {
		return true
	}
	normalized := strings.ReplaceAll(trimmed, ";", "\n")
	lines := strings.Split(normalized, "\n")
	skipRe := regexp.MustCompile(`^\s*t\.Skip(?:Now|f)?\s*\(.*\)\s*$`)
	for _, line := range lines {
		cur := strings.TrimSpace(line)
		if cur == "" {
			continue
		}
		if skipRe.MatchString(cur) {
			continue
		}
		return false
	}
	return true
}


// checkShellEffect denies shell invocations whose effects are mutating,
// ambiguous, or missing. Command text is only evidence about effect; it cannot
// authorize itself. Read-only and explicit verification commands remain usable
// while immutable task-baseline authorization is implemented.
func (e *Executor) checkShellEffect(call ToolCall) error {
	kind, _, err := projectdoc.ValidateShellToolInvocation(call.Name, call.Args)
	if err == nil {
		return nil
	}

	reason := fmt.Sprintf("%s effect=%s: %v", call.Name, kind, err)
	actionName := strings.TrimSpace(call.Name)
	if !strings.HasPrefix(actionName, "/") {
		actionName = "/" + actionName
	}
	if validMangleActionAtom(actionName) {
		e.assertSecurityViolation(types.MangleAtom(actionName), reason)
	} else {
		logging.Audit().SafetyCheck("shell_effect_gate", false, reason)
	}
	logging.Get(logging.CategorySession).Warn("shell-effect gate BLOCKED %s effect=%s", call.Name, kind)
	return err
}

// describeToolLoopFailure explains a mid-loop failure in terms an operator can
// act on.
//
// A bare "tool-result follow-up failed: context deadline exceeded" is
// unactionable: it names neither the budget that expired, nor how much work was
// already done, nor the flag that controls it. Observed live on
// `nerd analyze internal/projectdoc` with an 8-minute budget — indistinguishable
// from a broken shard, and the same shape already fixed for `nerd tool
// generate` (describeStageTimeout) and `nerd dream` (dreamSummary). The
// recurrence is the point: this is the third command where a timeout looked
// like a defect.
func describeToolLoopFailure(ctx context.Context, iteration, toolsThisRound int, err error) error {
	if err == nil {
		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("tool-result follow-up failed: %w", err)
	}

	// The total budget is not recoverable here: only the deadline is in the
	// context, and by the time this fires the remainder is negative. Saying
	// "the operation budget ran out" is what can honestly be claimed; the
	// caller knows the number it passed to --timeout.
	overrun := ""
	if deadline, ok := ctx.Deadline(); ok {
		if over := time.Since(deadline).Round(time.Second); over > 0 {
			overrun = fmt.Sprintf(" (overran by %s)", over)
		}
	}

	return fmt.Errorf(
		"tool-result follow-up failed after %d tool iteration(s), %d tool call(s) in the final round: %w — "+
			"the work was progressing, not stuck; the operation budget ran out%s. Raise it with --timeout",
		iteration+1, toolsThisRound, err, overrun)
}

// hollowSuccessPrefix is the stable error marker for hollow-completion failures.
const hollowSuccessPrefix = "hollow success blocked:"

type hollowSuccessError struct {
	detail string
}

func (e *hollowSuccessError) Error() string {
	return hollowSuccessPrefix + " " + e.detail
}

func newHollowSuccessError(format string, args ...any) error {
	return &hollowSuccessError{detail: fmt.Sprintf(format, args...)}
}

// isHollowSuccessError reports whether err is a hollow-completion failure.
func isHollowSuccessError(err error) bool {
	var hollow *hollowSuccessError
	return errors.As(err, &hollow)
}

// checkHollowSuccess fails when a side-effect-requiring intent finished
// without performing the required work. Prevents one-shot CLI paths
// (nerd create / fix / spawn) from printing success Result after planning-only
// prose.
//
// Dream mode is exempt: speculative subagents must not be forced to mutate.
func (e *Executor) checkHollowSuccess(result *ExecutionResult) error {
	defer e.cleanupPerTurnCoverageFacts()
	if result == nil {
		return nil
	}
	if e.sessionContext != nil && e.sessionContext.DreamMode {
		return nil
	}
	verb := strings.TrimSpace(result.Intent.Verb)
	if verb == "" {
		return nil
	}

	requiresTools := e.intentRequiresToolCall(verb) || e.writeOrientedIntent(verb)
	if !requiresTools {
		return nil
	}

	if result.SuccessfulToolCalls == 0 {
		return newHollowSuccessError(
			"intent %s requires side effects but no tool calls completed successfully (attempted=%d)",
			verb, result.ToolCallsExecuted,
		)
	}

	// Write-oriented work that only ran read/search tools still claims success
	// in live matrices (prose "Created backend/main.go" with no write_file).
	if e.writeOrientedIntent(verb) && result.SuccessfulWriteTools == 0 {
		return newHollowSuccessError(
			"write-oriented intent %s completed without a recognized write-mutation tool (tool_calls=%d)",
			verb, result.ToolCallsExecuted,
		)
	}
	if e.kernel == nil {
		logging.Get(logging.CategorySession).Debug("checkHollowSuccess: nil kernel, skipping missing_test_for check")
		return nil
	}
	facts, err := e.kernel.Query("missing_test_for")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("checkHollowSuccess: missing_test_for query failed: %v", err)
		return nil
	}
	if len(facts) == 0 {
		return nil
	}
	e.mu.RLock()
	createdSet := make(map[string]struct{}, len(e.perTurnCreatedSourceFacts))
	for _, f := range e.perTurnCreatedSourceFacts {
		if len(f.Args) > 0 {
			createdSet[types.ExtractString(f.Args[0])] = struct{}{}
		}
	}
	e.mu.RUnlock()
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		file := types.ExtractString(f.Args[0])
		if _, ok := createdSet[file]; ok {
			return newHollowSuccessError("%s was created without a test file; a test file is required (create %s)", file, strings.TrimSuffix(file, ".go")+"_test.go")
		}
	}
	return nil
}

func (e *Executor) recordGoFileCreations(preExist map[string]bool, canonicalToPhys map[string]string) {
	if e.kernel == nil || len(preExist) == 0 {
		return
	}
	for canonical, existedBefore := range preExist {
		if existedBefore {
			continue
		}
		phys, ok := canonicalToPhys[canonical]
		if !ok {
			continue
		}
		if _, err := os.Stat(phys); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			logging.Get(logging.CategorySession).Debug("recordGoFileCreations: post-stat error for %s: %v", phys, err)
			continue
		}
		if strings.HasSuffix(canonical, "_test.go") {
			sourceCanonical := strings.TrimSuffix(canonical, "_test.go") + ".go"
			fact := types.Fact{Predicate: "test_file_for", Args: []any{types.MangleString(canonical), types.MangleString(sourceCanonical)}}
			if err := e.kernel.Assert(fact); err != nil {
				logging.Get(logging.CategorySession).Warn("failed to assert test_file_for for %s: %v", canonical, err)
				continue
			}
			e.mu.Lock()
			e.perTurnTestFileForFacts = append(e.perTurnTestFileForFacts, fact)
			e.mu.Unlock()
			logging.Get(logging.CategorySession).Debug("asserted test_file_for(%q, %q)", canonical, sourceCanonical)
		} else {
			fact := types.Fact{Predicate: "created_source", Args: []any{types.MangleString(canonical)}}
			if err := e.kernel.Assert(fact); err != nil {
				logging.Get(logging.CategorySession).Warn("failed to assert created_source for %s: %v", canonical, err)
				continue
			}
			e.mu.Lock()
			e.perTurnCreatedSourceFacts = append(e.perTurnCreatedSourceFacts, fact)
			e.mu.Unlock()
			logging.Get(logging.CategorySession).Debug("asserted created_source(%q)", canonical)
		}
	}
}

func (e *Executor) cleanupPerTurnCoverageFacts() {
	if e.kernel == nil {
		e.mu.Lock()
		e.perTurnCreatedSourceFacts = nil
		e.perTurnTestFileForFacts = nil
		e.mu.Unlock()
		logging.Get(logging.CategorySession).Debug("cleanupPerTurnCoverageFacts: nil kernel, cleared local state")
		return
	}
	e.mu.Lock()
	created := append([]types.Fact(nil), e.perTurnCreatedSourceFacts...)
	testFacts := append([]types.Fact(nil), e.perTurnTestFileForFacts...)
	e.perTurnCreatedSourceFacts = nil
	e.perTurnTestFileForFacts = nil
	e.mu.Unlock()
	for _, f := range created {
		if err := e.kernel.RetractFact(f); err != nil {
			logging.Get(logging.CategorySession).Debug("cleanupPerTurnCoverageFacts: failed to retract created_source %v: %v", f.Args, err)
		}
	}
	for _, f := range testFacts {
		if err := e.kernel.RetractFact(f); err != nil {
			logging.Get(logging.CategorySession).Debug("cleanupPerTurnCoverageFacts: failed to retract test_file_for %v: %v", f.Args, err)
		}
	}
	// Direct kernel asserts (verify_created2 helpers) are not tracked in perTurn lists.
	// Retract any remaining coverage facts so the next turn starts clean and
	// stale facts do not leak forever.
	if len(created) == 0 && len(testFacts) == 0 {
		if facts, err := e.kernel.Query("created_source"); err == nil {
			for _, f := range facts {
				if err := e.kernel.RetractFact(f); err != nil {
					logging.Get(logging.CategorySession).Debug("cleanupPerTurnCoverageFacts: failed to retract direct created_source %v: %v", f.Args, err)
				}
			}
		}
		if facts, err := e.kernel.Query("test_file_for"); err == nil {
			for _, f := range facts {
				if err := e.kernel.RetractFact(f); err != nil {
					logging.Get(logging.CategorySession).Debug("cleanupPerTurnCoverageFacts: failed to retract direct test_file_for %v: %v", f.Args, err)
				}
			}
		}
	}
}

// retryWithNoToolNudge recompiles the prompt with the world_state
// "no_tool_call_retry" raised (and the actually-allowed tool list threaded
// through) and reissues the LLM turn exactly once.
//
// The decision about what the retry says lives in the prompt-atom corpus
// (system/tool_nudge/no_tool_call_retry.yaml), not in Go. The atom uses
// {{available_tools}} to render the runtime's permitted-tool surface so the
// model sees the real allowed set, not a hardcoded triple of
// write_file/edit_file/run_command. Go's responsibility is limited to:
//   - cloning the original CompilationContext,
//   - setting the activation flag + AvailableTools slice,
//   - asking the JIT compiler to recompile,
//   - reissuing the LLM call.
func (e *Executor) retryWithNoToolNudge(
	ctx context.Context,
	client types.LLMClient,
	userInput string,
	cfg *config.EffectiveAgentRuntimeConfig,
	compilationCtx *prompt.CompilationContext,
) (*types.LLMToolResponse, error) {
	if e.jitCompiler == nil || compilationCtx == nil {
		return nil, errors.New("no-tool-retry path requires JIT compiler and compilation context")
	}

	retryCtx := compilationCtx.Clone()
	retryCtx.PreviousAttemptNoToolCall = true
	if cfg != nil && len(cfg.AllowedTools) > 0 {
		retryCtx.AvailableTools = slices.Clone(cfg.AllowedTools)
	}

	compileResult, err := e.jitCompiler.Compile(ctx, retryCtx)
	if err != nil {
		return nil, fmt.Errorf("no-tool-retry: JIT recompile failed: %w", err)
	}
	if compileResult == nil || strings.TrimSpace(compileResult.Prompt) == "" {
		return nil, errors.New("no-tool-retry: JIT recompile produced empty prompt")
	}

	return e.generateResponse(ctx, client, compileResult.Prompt, userInput, cfg)
}

// executeToolBatchPiggyback handles the single-turn Piggyback path. Tools are
// executed and any errors collected, but results are not fed back to the LLM
// here (that would require a Piggyback-specific envelope follow-up).
func (e *Executor) executeToolBatchPiggyback(
	ctx context.Context,
	calls []types.ToolCall,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) []string {
	// Piggyback does not feed results back to the provider, but execution,
	// cancellation, budget handling, and accounting must still be identical to
	// the native path. Discard only the transport-specific result frames.
	_, toolErrs := e.executeToolBatch(ctx, calls, cfg, result)
	return toolErrs
}

// truncateToolResult caps tool output before feeding it back to the model.
// Massive output (greps, file dumps) wastes context budget for diminishing
// returns; 16 KB is enough for typical agent decisions.
func truncateToolResult(s string) string {
	const limit = 16 * 1024
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "\n...[truncated]"
}

func effectiveMaxToolCalls(configured int) int {
	if configured <= 0 {
		return defaultMaxToolCalls
	}
	return configured
}

func effectiveToolTimeout(configured time.Duration) time.Duration {
	if configured <= 0 {
		return defaultToolTimeout
	}
	return configured
}

// executeToolCall routes a tool call through the appropriate registry with safety checks.
// It checks both registries in order:
// 1. Modular tools (tools.Global()) - Go function handlers
// 2. Ouroboros tools (core.ToolRegistry) - compiled binary tools
func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.EffectiveAgentRuntimeConfig) (string, error) {
	executorCfg := e.configSnapshot()
	// The effective JIT allowlist is authoritative for every execution backend.
	// Registry membership only proves that a handler exists; it does not grant
	// the current agent the capability to invoke that handler.
	if !e.isToolAllowed(call.Name, cfg) {
		return "", fmt.Errorf("tool %s not allowed by effective JIT config", call.Name)
	}

	// Safety check via Constitutional Gate
	if executorCfg.EnableSafetyGate {
		if ok, reason := e.checkSafetyWithGate(call, true); !ok {
			// DENY PATH ONLY: ask the kernel whether this action atom
			// requires explicit human permission. Hot path (allowed) is
			// unaffected.
			actionLabel := call.Name
			if strings.TrimSpace(actionLabel) == "" {
				actionLabel = "unknown"
			}
			target := e.extractTarget(call.Args)
			actionAtom := actionLabel
			if !strings.HasPrefix(actionAtom, "/") {
				actionAtom = "/" + actionAtom
			}
			requiresPermission := false
			if validMangleActionAtom(actionAtom) && e.kernel != nil {
				grounded := fmt.Sprintf("requires_permission(%s)", actionAtom)
				if facts, err := e.kernel.Query(grounded); err == nil && len(facts) > 0 {
					requiresPermission = true
				} else {
					if facts2, err2 := e.kernel.Query("requires_permission"); err2 == nil {
						for _, f := range facts2 {
							if len(f.Args) == 0 {
								continue
							}
							if types.ExtractString(f.Args[0]) == actionAtom {
								requiresPermission = true
								break
							}
						}
					}
				}
			}
			if requiresPermission {
				return "", fmt.Errorf("tool call blocked by safety gate: action %s target %s denied (%s); this action requires human approval and cannot be self-authorized — do not retry the same call, it will fail identically; ask the user to perform it or accomplish the goal another way", actionLabel, target, reason)
			}
			return "", fmt.Errorf("tool call blocked by safety gate: action %s target %s denied (%s); constitution derived no permitted(...) fact for that action and target — identical retry will fail", actionLabel, target, reason)
		}
	}

	// The constitutional gate classifies run_command as generally available;
	// effect scope is a separate executive decision. This gate closes the path
	// that previously let shell cleanup bypass write-mutation accounting.
	if err := e.checkShellEffect(call); err != nil {
		return "", err
	}

	// Project write protection declared in nerd.md.
	//
	// This is the line that makes nerd.md's frontmatter different in kind from
	// CLAUDE.md. A "never touch config.json" written in prose is a request the
	// model complies with most of the time; a project_forbidden_path fact is
	// checked here, before the tool runs, and no amount of model conviction
	// gets past it.
	//
	// It sits after checkSafety and before the Dreamer preflight on purpose:
	// constitutional rules outrank project rules, and there is no reason to
	// simulate the consequences of an action that is already denied.
	if err := e.runWriteGuards(call); err != nil {
		return "", err
	}

	// Capture pre-existence for .go creation tracking.
	// For write-mutation tools targeting .go paths, note whether each file
	// existed before the call so a successful creation can be asserted.
	var preGoExistence map[string]bool
	var canonicalToPhys map[string]string
	if e.kernel != nil && isWriteMutationTool(call.Name) {
		if paths, err := projectdoc.TargetPaths(call.Args); err == nil && len(paths) > 0 {
			workspace := e.workspaceForVerification()
			for _, p := range paths {
				trimmed := strings.TrimSpace(p)
				if trimmed == "" {
					continue
				}
				if !strings.HasSuffix(strings.ToLower(trimmed), ".go") {
					continue
				}
				canonical := canonicalizeWrittenPath(trimmed, workspace)
				if canonical == "" {
					canonical = filepath.ToSlash(filepath.Clean(trimmed))
				}
				phys := trimmed
				if !filepath.IsAbs(trimmed) && workspace != "" {
					phys = filepath.Join(workspace, trimmed)
				}
				existed := false
				if _, err := os.Stat(phys); err == nil {
					existed = true
				} else if err != nil && !os.IsNotExist(err) {
					logging.Get(logging.CategorySession).Debug("creation tracking: stat error for %s: %v", phys, err)
					existed = true
				}
				if preGoExistence == nil {
					preGoExistence = make(map[string]bool)
					canonicalToPhys = make(map[string]string)
				}
				preGoExistence[canonical] = existed
				canonicalToPhys[canonical] = phys
			}
		}
	}

	// PRE-execution executive gate: run the Dreamer destructive-action
	// simulation before the tool mutates anything. This brings the VirtualStore
	// safety gate (otherwise reachable only via RouteAction) onto the
	// interactive coding path. Skipped gracefully when the store doesn't
	// implement InteractiveExecutiveGate.
	if gate, ok := e.virtualStore.(InteractiveExecutiveGate); ok && gate != nil {
		if blockErr := gate.PreflightDestructiveToolCall(ctx, call.ID, call.Name, call.Args); blockErr != nil {
			logging.Get(logging.CategorySession).Warn("Interactive executive gate BLOCKED tool %s: %v", call.Name, blockErr)
			return "", fmt.Errorf("tool call blocked by executive gate: %w", blockErr)
		}
	}

	// Assert pending_edit(FilePath, Content) immediately before any write-mutation
	// tool execution, and retract it on every exit path. Reuses
	// isWriteMutationTool so the classification stays consistent with the
	// hollow-success and projectForbidsWrite gates.
	//
	// The deferred retraction is the load-bearing half: pending_edit means "an
	// edit is in flight right now", and this function returns from many places
	// (modular registry error, Ouroboros path, validator refusal, timeout). A
	// fact left behind by any one of them would make every rule that reads it
	// reason about an edit that already finished.
	if pendingFacts := e.assertPendingEdits(call); len(pendingFacts) > 0 {
		defer func() {
			for _, fact := range pendingFacts {
				e.retractPendingEdit(fact)
			}
		}()
	}

	// Apply timeout to tool execution
	toolCtx, cancel := context.WithTimeout(ctx, effectiveToolTimeout(executorCfg.ToolTimeout))
	defer cancel()

	// Route to appropriate registry
	// 1. Try modular tool registry first (Go function handlers)
	modularRegistry := tools.Global()
	if modularRegistry.Has(call.Name) {
		logging.Session("Executing modular tool: %s with %d args", call.Name, len(call.Args))
		result, err := modularRegistry.Execute(toolCtx, call.Name, call.Args)
		if err != nil {
			return "", fmt.Errorf("modular tool execution failed: %w", err)
		}
		if result.Error != nil {
			return "", fmt.Errorf("modular tool returned error: %w", result.Error)
		}

		// POST-execution validation: verify the side effect actually landed
		// (file written, build passed, etc.) and assert validation facts to the
		// kernel so policy (e.g. task_complete/1) can reason over them. A
		// high-confidence validator failure is surfaced as an error so the model
		// sees the work did not take and can retry. Skipped gracefully when the
		// store doesn't implement InteractiveExecutiveGate.
		if gate, ok := e.virtualStore.(InteractiveExecutiveGate); ok && gate != nil {
			if valErr := gate.ValidateInteractiveToolResult(toolCtx, call.ID, call.Name, call.Args, result.Result, true); valErr != nil {
				return "", fmt.Errorf("post-action validation failed: %w", valErr)
			}
		}
		// Track creations for the turn: only on successful call that created a
		// file that did not previously exist. Non-test .go → created_source,
		// _test.go → test_file_for pairing exactly as the world scanner does.
		e.recordGoFileCreations(preGoExistence, canonicalToPhys)
		return result.Result, nil
	}

	// 2. Try Ouroboros registry (compiled binary tools)
	e.mu.RLock()
	ouroborosReg := e.ouroborosRegistry
	e.mu.RUnlock()

	if ouroborosReg != nil {
		if _, exists := ouroborosReg.GetTool(call.Name); exists {
			logging.Session("Executing Ouroboros tool: %s with %d args", call.Name, len(call.Args))
			// Convert args map to JSON string for binary execution
			argsJSON, err := json.Marshal(call.Args)
			if err != nil {
				return "", fmt.Errorf("failed to marshal Ouroboros tool args: %w", err)
			}
			result, err := ouroborosReg.ExecuteRegisteredTool(toolCtx, call.Name, []string{string(argsJSON)})
			if err != nil {
				return "", fmt.Errorf("Ouroboros tool execution failed: %w", err)
			}
			e.recordGoFileCreations(preGoExistence, canonicalToPhys)
			return result, nil
		}
	}

	return "", fmt.Errorf("tool %s not found in any registry", call.Name)
}

// isToolAllowed checks if a tool is in the effective JIT allowlist. Missing or
// empty configs fail closed: an absent capability envelope must never mean
// unrestricted execution.
func (e *Executor) isToolAllowed(toolName string, cfg *config.EffectiveAgentRuntimeConfig) bool {
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		return false
	}

	return slices.Contains(cfg.AllowedTools, toolName)
}

// assertSecurityViolation asserts a security_violation fact into the kernel to provide
// context to the agent about why an action was blocked.
func (e *Executor) assertSecurityViolation(actionAtom types.MangleAtom, reason string) {
	// Record the denial durably before touching the kernel.
	//
	// The security_violation fact below is session-scoped: it dies with the
	// process, so nothing about a refusal survived a run. logging.Audit()
	// .SafetyCheck had no caller anywhere in the repo, so the safety_allow and
	// safety_block families were declared and never written. For a system whose
	// stated contract is "every action must derive permitted(...); default
	// deny", the verdicts were the one thing no one could audit afterwards.
	//
	// Deliberately before the nil-kernel guard: a denial that happens because
	// the kernel is missing is exactly the one worth having on disk.
	logging.Audit().SafetyCheck(string(actionAtom), false, reason)

	if e.kernel == nil {
		return
	}
	fact := types.Fact{
		Predicate: "security_violation",
		Args:      []any{actionAtom, reason, time.Now().Unix()},
	}
	if err := e.kernel.Assert(fact); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to assert security_violation: %v", err)
	}
}

// maxPayloadBytes caps the JSON-serialized tool args we'll push into the
// Mangle kernel. Large blobs (file dumps, base64 images) bloat the fact store
// and the permitted/pending_action comparison would never match anyway.
const maxPayloadBytes = 100 * 1024 // 100 KB

// checkSafety verifies a tool call against the Constitutional Gate.
func (e *Executor) checkSafety(call ToolCall) bool {
	ok, _ := e.checkSafetyWithGate(call, e.configSnapshot().EnableSafetyGate)
	return ok
}

func (e *Executor) checkSafetyWithGate(call ToolCall, safetyGateEnabled bool) (bool, string) {
	// Categorically reject empty tool names — they would assert "/" as the
	// action atom, which is meaningless and bypasses meaningful policy match.
	if strings.TrimSpace(call.Name) == "" {
		logging.Get(logging.CategorySession).Warn("Safety check denied: empty tool call name")
		e.assertSecurityViolation(types.MangleAtom("/unknown"), "empty tool call name")
		return false, "empty tool call name"
	}

	if e.kernel == nil {
		// If the safety gate is enabled, missing kernel must FAIL CLOSED.
		// Otherwise the agent effectively runs in "god mode" on kernel init failure.
		if safetyGateEnabled {
			logging.Get(logging.CategorySession).Error("Safety check failed closed: kernel is nil while EnableSafetyGate=true")
			logging.Audit().SafetyCheck(call.Name, false, "failed closed: kernel is nil while EnableSafetyGate=true")
			return false, "failed closed: kernel is nil while EnableSafetyGate=true"
		}
		// Running ungated is the single most important thing to find in an
		// unattended run's log after the fact.
		logging.Audit().SafetyCheck(call.Name, true, "safety gate disabled and kernel is nil")
		return true, "" // Gate disabled: allow
	}

	// 1. Prepare Mangle terms
	// Action names must be Mangle atoms (start with /)
	actionName := call.Name
	if !strings.HasPrefix(actionName, "/") {
		actionName = "/" + actionName
	}
	if !validMangleActionAtom(actionName) {
		logging.Get(logging.CategorySession).Warn(
			"Safety check denied malformed action name %q", call.Name)
		e.assertSecurityViolation(types.MangleAtom("/unknown"), "malformed tool action name")
		return false, "malformed tool action name"
	}
	actionAtom := types.MangleAtom(actionName)

	// Normalize nil Args to empty map so json.Marshal produces "{}" instead
	// of "null" — the permitted facts written by policy use "{}" for no-arg
	// actions, so matching depends on this consistency.
	if call.Args == nil {
		call.Args = map[string]any{}
	}

	// Extract target and serialize payload
	target := e.extractTarget(call.Args)
	payloadBytes, err := json.Marshal(call.Args)
	if err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: cannot marshal args: %v", err)
		e.assertSecurityViolation(actionAtom, "cannot marshal args")
		return false, "cannot marshal args"
	}
	// Reject oversized payloads outright. Truncating would silently break the
	// permitted-fact comparison (truncated payload != permitted payload), so
	// the safer contract is: refuse loudly.
	if len(payloadBytes) > maxPayloadBytes {
		logging.Get(logging.CategorySession).Error(
			"Safety check denied: payload too large for kernel (%d bytes > %d)",
			len(payloadBytes), maxPayloadBytes)
		e.assertSecurityViolation(actionAtom, fmt.Sprintf("payload too large: %d > %d", len(payloadBytes), maxPayloadBytes))
		return false, fmt.Sprintf("payload too large: %d > %d", len(payloadBytes), maxPayloadBytes)
	}
	payload := string(payloadBytes)
	timestamp := time.Now().Unix()

	// 2. Assert pending_action
	// Decl pending_action(ActionID, ActionType, Target, Payload, Timestamp)
	pendingFact := types.Fact{
		Predicate: "pending_action",
		Args: []any{
			call.ID,
			actionAtom,
			target,
			payload,
			timestamp,
		},
	}

	if err := e.kernel.Assert(pendingFact); err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: assertion error: %v", err)
		e.assertSecurityViolation(actionAtom, "failed to assert pending_action")
		return false, "failed to assert pending_action"
	}

	// Ensure cleanup of pending_action
	defer func() {
		if err := e.kernel.RetractFact(pendingFact); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to retract pending_action: %v", err)
		}
	}()

	// 3. Query permitted(Action, Target, Payload) using the kernel's grounded
	// pattern form. This avoids scanning the whole permitted relation on the
	// normal allowed path. The bare-predicate fallback preserves compatibility
	// with lightweight Kernel implementations that only support Query("name").
	wantAction := string(actionAtom)
	exactQuery := fmt.Sprintf("permitted(%s, %s, %s)",
		wantAction, strconv.Quote(target), strconv.Quote(payload))
	facts, exactErr := e.kernel.Query(exactQuery)
	if exactErr != nil || len(facts) == 0 {
		var fallbackErr error
		facts, fallbackErr = e.kernel.Query("permitted")
		if fallbackErr != nil {
			if exactErr != nil {
				fallbackErr = fmt.Errorf("exact query failed: %v; predicate query failed: %w", exactErr, fallbackErr)
			}
			logging.Get(logging.CategorySession).Error("Safety check failed: query error: %v", fallbackErr)
			e.assertSecurityViolation(actionAtom, "failed to query permitted facts")
			return false, "failed to query permitted facts"
		}
	}
	if exactErr != nil {
		logging.Get(logging.CategorySession).Debug(
			"Kernel does not support grounded permitted query; used predicate fallback: %v", exactErr)
	}
	for _, f := range facts {
		if len(f.Args) != 3 {
			continue
		}

		// Check Action (Handle both MangleAtom and string types)
		factAction := types.ExtractString(f.Args[0])
		if factAction != wantAction {
			continue
		}

		// Check Target
		factTarget := types.ExtractString(f.Args[1])
		if factTarget != target {
			continue
		}

		// Check Payload
		factPayload := types.ExtractString(f.Args[2])
		if factPayload != payload {
			continue
		}

		// Match found!
		logging.Audit().SafetyCheck(wantAction, true, "matched permitted("+wantAction+", "+target+", payload)")
		return true, ""
	}

	logging.Get(logging.CategorySession).Warn("Safety check denied action: %s (target: %s)", actionName, target)
	reason := fmt.Sprintf("action not permitted: target=%s", target)
	e.assertSecurityViolation(actionAtom, reason)
	return false, reason
}

// validMangleActionAtom is intentionally a little broader than
// validMangleVerb: legacy tool registries contain camelCase action names, while
// perceived intent verbs are normalized lowercase. Both forms remain safe to
// interpolate when restricted to one slash followed by ASCII alphanumerics or
// underscores.
func validMangleActionAtom(action string) bool {
	if len(action) < 2 || action[0] != '/' {
		return false
	}
	for i := 1; i < len(action); i++ {
		c := action[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') &&
			(c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// extractTarget attempts to identify the primary target of a tool call.
func (e *Executor) extractTarget(args map[string]any) string {
	// Path-bearing calls share the exact extraction contract used by nerd.md's
	// write gate. Search/network tools then add their non-path target keys.
	if target := projectdoc.TargetPath(args); target != "" {
		return target
	}
	for _, key := range []string{"url", "query", "pattern", "glob", "dir", "directory", "session_id", "browser_id", "target_id"} {
		if val, ok := args[key]; ok {
			if target := strings.TrimSpace(types.ExtractString(val)); target != "" {
				return target
			}
		}
	}
	return "unknown"
}
