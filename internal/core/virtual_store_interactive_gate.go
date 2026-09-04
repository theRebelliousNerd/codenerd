package core

import (
	"context"
	"time"

	"codenerd/internal/logging"
)

// This file wires the VirtualStore executive (Dreamer destructive-action gate
// and the post-action validator registry) onto the *interactive* session
// executor path (internal/session/executor.go), which executes modular tools
// directly via tools.Global() and therefore bypasses RouteAction.
//
// We deliberately do NOT route interactive tool calls through RouteAction:
// the executor has already run the tool via the modular registry, so
// RouteAction's own executeAction switch would double-execute (or diverge).
// Instead we expose two focused, idempotent capability methods that the
// executor invokes around its existing executeToolCall:
//
//   - PreflightDestructiveToolCall: PRE-execution Dreamer simulation gate.
//   - ValidateInteractiveToolResult: POST-execution validator pass + fact
//     assertion to the kernel.
//
// Ordering is load-bearing: the Dreamer prevents unsafe writes, so it MUST run
// before the tool executes; validators verify side effects actually landed, so
// they MUST run after. The executor calls these at two distinct seams.

// interactiveToolActionType maps a modular tool name (as registered by
// internal/tools/{core,shell,codedom}.RegisterAll) to the ActionType the
// Dreamer's isDestructiveAction switch and the ValidatorRegistry dispatch on.
//
// This is an EXPLICIT map, not name==ActionType identity, because:
//   - isDestructiveAction(ActionType) returns false for unknown types, which
//     would silently disable the Dreamer gate.
//   - ValidatorRegistry.getValidatorsForType(ActionType) selects the right
//     validator (FileWriteValidator, etc.) by ActionType; a mismatch yields a
//     "skipped" (no-op) validation.
//
// Tools absent from this map are treated as non-destructive and unvalidated
// (the same posture as before this wiring existed) — see the ok checks below.
var interactiveToolActionType = map[string]ActionType{
	// core filesystem tools (internal/tools/core/file_ops.go)
	"read_file":   ActionReadFile,
	"write_file":  ActionWriteFile,
	"edit_file":   ActionEditFile,
	"delete_file": ActionDeleteFile,
	// shell execution tools (internal/tools/shell/execute.go)
	"run_command": ActionRunCommand,
	"bash":        ActionBash,
	"run_build":   ActionRunBuild,
	// codedom line-edit tools (internal/tools/codedom/lines.go)
	"edit_lines":   ActionEditLines,
	"insert_lines": ActionInsertLines,
	"delete_lines": ActionDeleteLines,
	// codedom element edits share the line tools' destructive posture and
	// already have CodeDOM/syntax validators and a constitution case wired.
	"edit_element": ActionEditElement,
	// NOTE: "apply_edits" is a known unmapped write tool with no ActionType in VirtualStore routing.
}

// actionTypeForToolName resolves a modular tool name to its ActionType.
// The bool is false when the tool is unmapped (treat as non-gated/unvalidated).
func actionTypeForToolName(toolName string) (ActionType, bool) {
	at, ok := interactiveToolActionType[toolName]
	return at, ok
}

// buildInteractiveActionRequest constructs the ActionRequest that the Dreamer
// and validators expect from an interactive tool call. Target extraction
// mirrors the executor's extractTarget heuristic (path/filename/.../target).
func buildInteractiveActionRequest(actionID string, at ActionType, args map[string]any) ActionRequest {
	if args == nil {
		args = map[string]any{}
	}
	target := extractActionTarget(args)
	// Copy args into Payload so validators that inspect e.g. "content" or
	// "old_string" see the same data the tool received.
	payload := make(map[string]any, len(args))
	for k, v := range args {
		payload[k] = v
	}
	return ActionRequest{
		ActionID: actionID,
		Type:     at,
		Target:   target,
		Payload:  payload,
	}
}

// extractActionTarget mirrors session/executor.go:extractTarget so the Target
// passed to the Dreamer/validators matches what the tool acted on.
func extractActionTarget(args map[string]any) string {
	for _, key := range []string{"path", "filename", "filepath", "file", "url", "target", "query"} {
		if val, ok := args[key]; ok {
			if s := extractStringArg(val); s != "" {
				return s
			}
		}
	}
	return "unknown"
}

func extractStringArg(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// PreflightDestructiveToolCall runs the Dreamer speculative safety gate for an
// interactive tool call BEFORE it executes. It returns a non-nil error if the
// action is unsafe and must be blocked.
//
// Fail-CLOSED policy: every mapped destructive tool requires a usable Dreamer.
// Permission and speculative safety are independent gates; an allow decision
// from checkSafety must never compensate for a missing simulation engine.
func (v *VirtualStore) PreflightDestructiveToolCall(ctx context.Context, actionID, toolName string, args map[string]any) error {
	at, ok := actionTypeForToolName(toolName)
	if !ok || !isDestructiveAction(at) {
		return nil // non-destructive or unmapped: nothing to simulate
	}

	req := buildInteractiveActionRequest(actionID, at, args)
	dreamer := v.getDreamer()
	if dreamer == nil {
		reason := "dreamer unavailable for destructive interactive tool"
		logging.Get(logging.CategoryVirtualStore).Error(
			"Dreamer unavailable; BLOCKED interactive tool: %s on %s", toolName, req.Target)
		v.injectFact(newSecurityViolationFact(req, reason))
		return &InteractiveGateError{Reason: reason}
	}

	dreamResult := dreamer.SimulateAction(ctx, req)
	if dreamResult.Unsafe {
		logging.Get(logging.CategoryVirtualStore).Warn(
			"Dreamer BLOCKED interactive tool: %s on %s (reason: %s)",
			toolName, req.Target, dreamResult.Reason)
		v.injectFact(Fact{
			Predicate: "dream_blocked_action",
			Args:      []any{dreamResult.ActionID, string(at), req.Target, dreamResult.Reason},
		})
		v.injectFact(newSecurityViolationFact(req, "dreamer: "+dreamResult.Reason))
		return &InteractiveGateError{Reason: "dreamer safety gate: " + dreamResult.Reason}
	}
	logging.VirtualStoreDebug("Dreamer approved interactive tool: %s on %s", toolName, req.Target)
	return nil
}

// ValidateInteractiveToolResult runs the post-action validator registry for an
// interactive tool call AFTER it has executed, asserting validation facts to
// the kernel (so policy — e.g. task_complete/1 — can reason over them).
//
// It returns a non-nil error only when a validator fails with high confidence
// (>= 0.8), mirroring RouteAction's threshold (virtual_store.go:1296). A nil
// return means "verified or no opinion" — the caller should treat a non-nil
// error as "the tool reported success but the side effect did not actually
// land," and surface that to the model.
//
// success reflects whether the tool itself reported success; validators only
// run on success (a tool that already errored needs no side-effect check).
func (v *VirtualStore) ValidateInteractiveToolResult(ctx context.Context, actionID, toolName string, args map[string]any, output string, success bool) error {
	if !success || v.validators == nil {
		return nil
	}
	at, ok := actionTypeForToolName(toolName)
	if !ok {
		return nil // unmapped tool: no validators to dispatch
	}

	req := buildInteractiveActionRequest(actionID, at, args)
	req = v.requestForValidation(req)
	res := ActionResult{
		Success:  true,
		Output:   output,
		Metadata: map[string]any{"completed_at": time.Now().Unix()},
	}

	validations := v.validators.Validate(ctx, req, res)
	// Assert validation facts to the kernel (reuses the same path RouteAction
	// uses, so Step 2's task_complete/1 has facts to reason over).
	v.processValidationResults(req, res, validations)

	if !ValidateAll(validations) {
		if failure := FirstFailure(validations); failure != nil && failure.Confidence >= 0.8 {
			logging.Get(logging.CategoryVirtualStore).Warn(
				"Post-action validation failed (interactive): %s on %s - %s (confidence=%.2f)",
				toolName, req.Target, failure.Error, failure.Confidence)
			return &InteractiveGateError{Reason: "validation failed: " + failure.Error}
		}
	}
	return nil
}

// InteractiveGateError marks an error originating from the interactive
// executive gate so callers can distinguish a policy/validation block from a
// tool execution error.
type InteractiveGateError struct {
	Reason string
}

func (e *InteractiveGateError) Error() string { return e.Reason }
