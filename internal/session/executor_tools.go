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
	llmResponse, err := e.generateResponse(ctx, systemPrompt, userInput, cfg)
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
			retried, retryErr := e.retryWithNoToolNudge(ctx, userInput, cfg, compilationCtx)
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
	if ptp, ok := e.llmClient.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		toolErrs := e.executeToolBatchPiggyback(ctx, llmResponse.ToolCalls, cfg, result)
		return llmResponse, toolErrs, nil
	}

	// Native multi-turn tool calling required for correct semantics on
	// Anthropic/OpenAI-style providers.
	trp, supportsLoop := e.llmClient.(types.ToolResultsProvider)
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
		toolResults := make([]types.ToolResult, 0, len(currentResponse.ToolCalls))
		for _, call := range currentResponse.ToolCalls {
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
					return currentResponse, toolErrs, ctx.Err()
				}
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

		// If the client can't accept tool results back, we're done after the
		// first execution pass. The model will not see the results this turn.
		if !supportsLoop {
			logging.Get(logging.CategorySession).Warn(
				"LLM client does not implement ToolResultsProvider; tool results not fed back to model. Provider=%T", e.llmClient)
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

	logging.Get(logging.CategorySession).Warn("Max tool iterations reached: %d", maxIter)
	return currentResponse, toolErrs, nil
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
func isWriteMutationTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "write_file", "edit_file", "delete_file", "apply_patch", "str_replace",
		"create_file", "replace_in_file", "multi_edit":
		return true
	default:
		return false
	}
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

	return e.generateResponse(ctx, compileResult.Prompt, userInput, cfg)
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
