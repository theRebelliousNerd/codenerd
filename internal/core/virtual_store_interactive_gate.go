package core

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
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
// Other registered tools use their explicit effect declaration. Unknown
// effects are rejected, and executable/external effects take the generic gate.
var interactiveToolActionType = map[string]ActionType{
	// core filesystem tools (internal/tools/core/file_ops.go)
	"read_file":   ActionReadFile,
	"write_file":  ActionWriteFile,
	"edit_file":   ActionEditFile,
	"delete_file": ActionDeleteFile,
	// shell execution tools (internal/tools/shell/execute.go)
	"run_command":        ActionRunCommand,
	"bash":               ActionBash,
	"run_build":          ActionRunBuild,
	"run_tests":          ActionRunTests,
	"run_impacted_tests": ActionRunTests,
	"git_operation":      ActionGitOperation,
	// codedom line-edit tools (internal/tools/codedom/lines.go)
	"edit_lines":   ActionEditLines,
	"insert_lines": ActionInsertLines,
	"delete_lines": ActionDeleteLines,
	// codedom element edits share the line tools' destructive posture and
	// already have CodeDOM/syntax validators and a constitution case wired.
	"edit_element": ActionEditElement,
	// apply_edits is the multi-file transactional editor. It has no dedicated
	// ActionType in VirtualStore routing, but it mutates files exactly like
	// edit_file, so it takes the same Dreamer preflight and post-write
	// validators. It was unmapped until 2026-09-04, which skipped both.
	"apply_edits": ActionEditFile,
}

// actionTypeForToolName resolves a modular tool name to its ActionType.
// The bool is false when the tool lacks an effect declaration (fail closed).
func actionTypeForToolName(toolName string) (ActionType, bool) {
	at, ok := interactiveToolActionType[toolName]
	if ok {
		return at, true
	}
	effect, err := tools.LookupEffect(toolName)
	if err != nil {
		return "", false
	}
	if effect == tools.EffectRead {
		return ActionReadFile, true
	}
	return ActionExecTool, true
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
	for _, key := range []string{"path", "file_path", "filename", "filepath", "file", "command", "url", "target", "query"} {
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
	if ctx == nil {
		return &InteractiveGateError{Reason: "nil context"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	at, ok := actionTypeForToolName(toolName)
	if !ok && v != nil {
		if registry := v.GetToolRegistry(); registry != nil {
			if _, exists := registry.GetTool(toolName); exists {
				at, ok = ActionExecTool, true
			}
		}
	}
	if !ok {
		return &InteractiveGateError{Reason: "missing effect declaration for tool " + toolName}
	}
	if !isDestructiveAction(at) {
		return nil
	}
	if v == nil {
		return &InteractiveGateError{Reason: "dreamer unavailable: VirtualStore is nil"}
	}
	if toolName == "apply_edits" {
		paths, err := projectdoc.TargetPaths(args)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return &InteractiveGateError{Reason: "apply_edits has no targets"}
		}
		for i, path := range paths {
			payload := make(map[string]any, len(args)+1)
			for k, val := range args {
				payload[k] = val
			}
			payload["path"] = path
			if err := v.PreflightDestructiveToolCall(ctx, fmt.Sprintf("%s:%d", actionID, i), "edit_file", payload); err != nil {
				return err
			}
		}
		return nil
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
	if v == nil {
		return &InteractiveGateError{Reason: "validator store unavailable"}
	}
	if !success || v.validators == nil {
		return nil
	}
	if toolName == "apply_edits" {
		paths, err := projectdoc.TargetPaths(args)
		if err != nil || len(paths) == 0 {
			return &InteractiveGateError{Reason: "invalid apply_edits validation targets"}
		}
		for i, path := range paths {
			// apply_edits verifies staged replacements internally. Apply the
			// post-write existence/syntax validators to every resulting file.
			if err := v.ValidateInteractiveToolResult(ctx, fmt.Sprintf("%s:%d", actionID, i), "edit_file", map[string]any{"path": path}, output, true); err != nil {
				return err
			}
		}
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
