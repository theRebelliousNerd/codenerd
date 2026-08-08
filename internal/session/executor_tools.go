package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
			if retryErr == nil && retried != nil && len(retried.ToolCalls) > 0 {
				llmResponse = retried
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
		return llmResponse, toolErrs, nil
	}

	// Native multi-turn tool calling required for correct semantics on
	// Anthropic/OpenAI-style providers.
	trp, supportsLoop := client.(types.ToolResultsProvider)
	toolDefs := e.buildToolDefinitions(cfg)

	// Seed the history with the initial user turn and the assistant's
	// first response (which contains the tool_use blocks).
	history := []types.Message{
		{Role: "user", Text: userInput},
		{Role: "assistant", Text: llmResponse.Text, ToolCalls: llmResponse.ToolCalls},
	}

	maxIter := e.config.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 8
	}

	var toolErrs []string
	currentResponse := llmResponse

	for iter := 0; iter < maxIter; iter++ {
		if ctx.Err() != nil {
			return currentResponse, toolErrs, ctx.Err()
		}

		// Execute all tool calls from this turn and collect tool_result blocks.
		toolResults, batchErrs := e.executeToolBatch(ctx, currentResponse.ToolCalls, cfg, result)
		toolErrs = append(toolErrs, batchErrs...)
		if ctx.Err() != nil {
			return currentResponse, toolErrs, ctx.Err()
		}

		// If the client can't accept tool results back, we're done after the
		// first execution pass. The model will not see the results this turn.
		if !supportsLoop {
			logging.Get(logging.CategorySession).Warn(
				"LLM client does not implement ToolResultsProvider; tool results not fed back to model. Provider=%T", client)
			return currentResponse, toolErrs, nil
		}

		// Append the user tool_result turn to history and re-invoke.
		history = append(history, types.Message{
			Role:        "user",
			ToolResults: toolResults,
		})

		nextResp, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, toolDefs)
		if err != nil {
			return currentResponse, toolErrs, fmt.Errorf("tool-result follow-up failed: %w", err)
		}
		currentResponse = nextResp

		// Append the next assistant turn to history (whether or not it has more tool calls).
		history = append(history, types.Message{
			Role:      "assistant",
			Text:      nextResp.Text,
			ToolCalls: nextResp.ToolCalls,
		})

		if len(nextResp.ToolCalls) == 0 {
			// Model returned a final answer.
			return currentResponse, toolErrs, nil
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
		"Max tool iterations reached: %d; forcing a final answer from %d executed tool call(s)",
		maxIter, result.ToolCallsExecuted)

	final, finalErrs, finalErr := e.forceFinalAnswer(ctx, trp, systemPrompt, history, currentResponse, cfg, result)
	toolErrs = append(toolErrs, finalErrs...)
	if finalErr != nil {
		logging.Get(logging.CategorySession).Error(
			"Forced final answer failed after exhausting tool iterations: %v", finalErr)
		return currentResponse, toolErrs, nil
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
	history []types.Message,
	pending *types.LLMToolResponse,
	cfg *config.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if trp == nil {
		return pending, nil, errors.New("client does not support tool-result follow-up")
	}

	var toolErrs []string
	if len(pending.ToolCalls) > 0 {
		toolResults, errs := e.executeToolBatch(ctx, pending.ToolCalls, cfg, result)
		toolErrs = append(toolErrs, errs...)
		history = append(history, types.Message{Role: "user", ToolResults: toolResults})
	}

	// The kernel decides which verbs need a real side effect
	// (intent_requires_tool_call/1 in delegation.mg), not a Go switch.
	needsWrite := e.intentRequiresToolCall(result.Intent.Verb) && result.SuccessfulWriteTools == 0

	nudge := readOnlyBudgetExhaustedNudge
	var finalTools []types.ToolDefinition
	if needsWrite {
		nudge = writeBudgetExhaustedNudge
		finalTools = writeOnlyToolDefinitions(e.buildToolDefinitions(cfg))
		logging.Get(logging.CategorySession).Warn(
			"Retaining %d write tool(s) for the final call: %s requires a side effect and none has landed yet",
			len(finalTools), result.Intent.Verb)
	}

	history = append(history, types.Message{Role: "user", Text: nudge})

	final, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, finalTools)
	if err != nil {
		return pending, toolErrs, fmt.Errorf("final completion failed: %w", err)
	}
	if final == nil {
		return pending, toolErrs, errors.New("final completion returned nothing")
	}

	// Execute whatever writes the model asked for. Handing back an unexecuted
	// write tool_call would reproduce the original bug one layer down: the
	// deliverable named but never produced.
	if len(final.ToolCalls) > 0 {
		_, errs := e.executeToolBatch(ctx, final.ToolCalls, cfg, result)
		toolErrs = append(toolErrs, errs...)
	}

	if strings.TrimSpace(final.Text) == "" && len(final.ToolCalls) == 0 {
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

	for _, call := range calls {
		if result.ToolCallsExecuted >= e.config.MaxToolCalls {
			logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", e.config.MaxToolCalls)
			toolResults = append(toolResults, types.ToolResult{
				ToolUseID: call.ID,
				Content:   "tool call budget exceeded for this turn",
				IsError:   true,
			})
			toolErrs = append(toolErrs, fmt.Sprintf("%s: budget exceeded", call.Name))
			continue
		}

		out, execErr := e.executeToolCall(ctx, ToolCall{ID: call.ID, Name: call.Name, Args: call.Input}, cfg)
		result.ToolCallsExecuted++

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

		if isWriteMutationTool(call.Name) {
			result.SuccessfulWriteTools++
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

// intentRequiresToolCall asks the Mangle kernel whether the supplied intent
// verb requires a real tool_call to make progress. The decision logic lives
// entirely in the policy corpus (delegation.mg → intent_requires_tool_call/1
// derived from action_mapping/2 + side_effecting_action/1) — this Go helper
// just queries it. When the kernel is unavailable or the query fails, we
// conservatively return false so we never block a final answer on missing
// policy.
func (e *Executor) intentRequiresToolCall(verb string) bool {
	if e.kernel == nil || verb == "" {
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

// isWriteOrientedIntent reports intents whose completion requires durable
// file mutations (write_file/edit_file/...). Pure analysis/query verbs are
// false so prose-only answers remain valid terminal responses.
func isWriteOrientedIntent(verb string) bool {
	switch strings.TrimSpace(verb) {
	case "/create", "/fix", "/refactor", "/write", "/delete", "/implement",
		"/scaffold", "/optimize", "/format", "/migrate", "/document", "/commit":
		return true
	default:
		return false
	}
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
	switch strings.ToLower(strings.TrimSpace(name)) {
	case // Registered VirtualStore write actions.
		"write_file", "edit_file", "delete_file",
		"edit_lines", "insert_lines", "delete_lines",
		"edit_element", "fs_write",
		// Defensive aliases: not registered here, but common names a model may
		// emit. Harmless to accept; keeps the gate closed if one is ever added.
		"apply_patch", "str_replace", "create_file", "replace_in_file", "multi_edit":
		return true
	default:
		return false
	}
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
var projectDocPathArgs = []string{"path", "file_path", "filepath", "file", "filename", "target", "dest", "destination"}

// projectDocTargetPath extracts the target path from a tool call's arguments.
func projectDocTargetPath(args map[string]any) string {
	for _, key := range projectDocPathArgs {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// pendingEditContentKeys are argument names that may carry the file content
// for a write-mutation tool. Different tools use different conventions
// ("content", "new_content", "text", etc.), so we check all known variants.
var pendingEditContentKeys = []string{"content", "new_content", "newContent", "text", "body", "data", "patch"}

// pendingEditContent extracts the content payload from a tool call's arguments.
// Returns "" when no content key is present — e.g. delete_file/delete_lines
// which intentionally carry no body.
func pendingEditContent(args map[string]any) string {
	for _, key := range pendingEditContentKeys {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok {
				return s
			}
			// Non-string payloads (e.g. JSON objects) — stringify for the fact.
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

// assertPendingEdit asserts pending_edit(FilePath, Content) for a write-mutation
// tool call. FilePath and Content are derived from the tool invocation via the
// existing path/content extraction helpers, and the classification reuses
// isWriteMutationTool so the predicate stays in sync with the write-mutation
// registry (see isWriteMutationTool godoc). The assertion is best-effort: kernel
// absence or assertion errors are logged and do not block the tool execution.
// It returns the asserted fact and true when an assertion was made, so the
// caller can defer the matching retraction. A pending_edit that is asserted and
// never retracted is worse than one never asserted: the fact means "an edit is
// in flight", so a stale one makes the 26 rules that read it reason about work
// that finished long ago, and the facts accumulate without bound against the
// kernel's fact ceiling.
func (e *Executor) assertPendingEdit(call ToolCall) (types.Fact, bool) {
	if !isWriteMutationTool(call.Name) {
		return types.Fact{}, false
	}
	if e.kernel == nil {
		return types.Fact{}, false
	}
	filePath := projectDocTargetPath(call.Args)
	content := pendingEditContent(call.Args)
	fact := types.Fact{
		Predicate: "pending_edit",
		Args:      []any{filePath, content},
	}
	if err := e.kernel.Assert(fact); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to assert pending_edit for %s (%s): %v", call.Name, filePath, err)
		return types.Fact{}, false
	}
	return fact, true
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
	target := projectDocTargetPath(call.Args)
	if target == "" {
		return "", false
	}
	if e.kernel == nil {
		return "", false
	}

	// Matching lives in projectdoc.ForbiddenByKernel so this gate and the
	// VirtualStore's cannot drift apart. They used to be one gate and one hole:
	// shards route writes through the VirtualStore, which checked nothing.
	reason, forbidden, err := projectdoc.ForbiddenByKernel(e.kernel, target)
	if err != nil {
		// Fail OPEN, loudly. A kernel query failure is not evidence that the
		// path is protected, and turning every transient query error into a
		// blocked write would make the agent unusable the moment the kernel
		// hiccups. The warning is what makes the degraded state visible.
		logging.Get(logging.CategorySession).Warn(
			"nerd.md write protection could not be evaluated for %s (%v); allowing the write", target, err)
		return "", false
	}
	return reason, forbidden
}

// hollowSuccessPrefix is the stable error marker for hollow-completion failures.
const hollowSuccessPrefix = "hollow success blocked:"

// isHollowSuccessError reports whether err is a hollow-completion failure.
func isHollowSuccessError(err error) bool {
	return err != nil && strings.Contains(err.Error(), hollowSuccessPrefix)
}

// checkHollowSuccess fails when a side-effect-requiring intent finished
// without performing the required work. Prevents one-shot CLI paths
// (nerd create / fix / spawn) from printing success Result after planning-only
// prose.
//
// Dream mode is exempt: speculative subagents must not be forced to mutate.
func (e *Executor) checkHollowSuccess(result *ExecutionResult) error {
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

	requiresTools := e.intentRequiresToolCall(verb) || isWriteOrientedIntent(verb)
	if !requiresTools {
		return nil
	}

	if result.ToolCallsExecuted == 0 {
		return fmt.Errorf(
			"%s intent %s requires side effects but no tool calls completed",
			hollowSuccessPrefix, verb,
		)
	}

	// Write-oriented work that only ran read/search tools still claims success
	// in live matrices (prose "Created backend/main.go" with no write_file).
	if isWriteOrientedIntent(verb) && result.SuccessfulWriteTools == 0 {
		return fmt.Errorf(
			"%s write-oriented intent %s completed without write_file/edit_file (tool_calls=%d)",
			hollowSuccessPrefix, verb, result.ToolCallsExecuted,
		)
	}
	return nil
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
	var toolErrs []string
	for _, call := range calls {
		if result.ToolCallsExecuted >= e.config.MaxToolCalls {
			logging.Get(logging.CategorySession).Warn("Max tool calls reached: %d", e.config.MaxToolCalls)
			break
		}
		toolCall := ToolCall{
			ID:   call.ID,
			Name: call.Name,
			Args: call.Input,
		}
		out, execErr := e.executeToolCall(ctx, toolCall, cfg)
		result.ToolCallsExecuted++
		if execErr != nil {
			logging.Get(logging.CategorySession).Error("Tool call %s failed: %v", call.Name, execErr)
			if ctx.Err() != nil {
				return toolErrs
			}
			toolErrs = append(toolErrs, fmt.Sprintf("%s: %v", call.Name, execErr))
			continue
		}
		if isWriteMutationTool(call.Name) {
			result.SuccessfulWriteTools++
		}
		logging.SessionDebug("Tool %s executed successfully: %d chars result", call.Name, len(out))
	}
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
	return s[:limit] + "\n...[truncated]"
}

// executeToolCall routes a tool call through the appropriate registry with safety checks.
// It checks both registries in order:
// 1. Modular tools (tools.Global()) - Go function handlers
// 2. Ouroboros tools (core.ToolRegistry) - compiled binary tools
func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.EffectiveAgentRuntimeConfig) (string, error) {
	// The effective JIT allowlist is authoritative for every execution backend.
	// Registry membership only proves that a handler exists; it does not grant
	// the current agent the capability to invoke that handler.
	if !e.isToolAllowed(call.Name, cfg) {
		return "", fmt.Errorf("tool %s not allowed by effective JIT config", call.Name)
	}

	// Safety check via Constitutional Gate
	if e.config.EnableSafetyGate {
		if !e.checkSafety(call) {
			return "", fmt.Errorf("tool call blocked by safety gate: %s", call.Name)
		}
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
	if reason, denied := e.projectForbidsWrite(call); denied {
		logging.Get(logging.CategorySession).Warn(
			"nerd.md BLOCKED %s on %s: %s", call.Name, projectDocTargetPath(call.Args), reason)
		return "", fmt.Errorf("blocked by nerd.md: %s is write-protected (%s)",
			projectDocTargetPath(call.Args), reason)
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
	if pendingFact, asserted := e.assertPendingEdit(call); asserted {
		defer e.retractPendingEdit(pendingFact)
	}

	// Apply timeout to tool execution
	toolCtx, cancel := context.WithTimeout(ctx, e.config.ToolTimeout)
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
	if e.kernel == nil {
		return
	}
	fact := types.Fact{
		Predicate: "security_violation",
		Args:      []any{actionAtom, reason, time.Now().Unix()},
	}
	_ = e.kernel.Assert(fact)
}

// maxPayloadBytes caps the JSON-serialized tool args we'll push into the
// Mangle kernel. Large blobs (file dumps, base64 images) bloat the fact store
// and the permitted/pending_action comparison would never match anyway.
const maxPayloadBytes = 100 * 1024 // 100 KB

// checkSafety verifies a tool call against the Constitutional Gate.
func (e *Executor) checkSafety(call ToolCall) bool {
	// Categorically reject empty tool names — they would assert "/" as the
	// action atom, which is meaningless and bypasses meaningful policy match.
	if strings.TrimSpace(call.Name) == "" {
		logging.Get(logging.CategorySession).Warn("Safety check denied: empty tool call name")
		if e.kernel != nil {
			e.assertSecurityViolation(types.MangleAtom("/unknown"), "empty tool call name")
		}
		return false
	}

	if e.kernel == nil {
		// If the safety gate is enabled, missing kernel must FAIL CLOSED.
		// Otherwise the agent effectively runs in "god mode" on kernel init failure.
		if e.config.EnableSafetyGate {
			logging.Get(logging.CategorySession).Error("Safety check failed closed: kernel is nil while EnableSafetyGate=true")
			return false
		}
		return true // Gate disabled: allow
	}

	// 1. Prepare Mangle terms
	// Action names must be Mangle atoms (start with /)
	actionName := call.Name
	if !strings.HasPrefix(actionName, "/") {
		actionName = "/" + actionName
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
		return false
	}
	// Reject oversized payloads outright. Truncating would silently break the
	// permitted-fact comparison (truncated payload != permitted payload), so
	// the safer contract is: refuse loudly.
	if len(payloadBytes) > maxPayloadBytes {
		logging.Get(logging.CategorySession).Error(
			"Safety check denied: payload too large for kernel (%d bytes > %d)",
			len(payloadBytes), maxPayloadBytes)
		e.assertSecurityViolation(actionAtom, fmt.Sprintf("payload too large: %d > %d", len(payloadBytes), maxPayloadBytes))
		return false
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
		return false
	}

	// Ensure cleanup of pending_action
	defer func() {
		if err := e.kernel.RetractFact(pendingFact); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to retract pending_action: %v", err)
		}
	}()

	// 3. Query permitted
	// permitted(Action, Target, Payload)
	// We query for all permitted facts and filter for matching this exact request.
	facts, err := e.kernel.Query("permitted")
	if err != nil {
		logging.Get(logging.CategorySession).Error("Safety check failed: query error: %v", err)
		e.assertSecurityViolation(actionAtom, "failed to query permitted facts")
		return false
	}

	wantAction := string(actionAtom)
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
		return true
	}

	logging.Get(logging.CategorySession).Warn("Safety check denied action: %s (target: %s)", actionName, target)
	e.assertSecurityViolation(actionAtom, fmt.Sprintf("action not permitted: target=%s", target))
	return false
}

// extractTarget attempts to identify the primary target of a tool call.
func (e *Executor) extractTarget(args map[string]any) string {
	// Common keys for targets (include glob/search patterns used by tools).
	candidates := []string{"path", "filename", "filepath", "file", "url", "target", "query", "pattern", "glob", "dir", "directory"}
	for _, key := range candidates {
		if val, ok := args[key]; ok {
			return types.ExtractString(val)
		}
	}
	return "unknown"
}
