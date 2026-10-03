package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"codenerd/internal/types"
)

// ExecuteGeneratedToolCall preserves the session's authorization envelope.
// It deliberately never calls RouteAction or the raw registry executor.
func (v *VirtualStore) ExecuteGeneratedToolCall(ctx context.Context, request types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
	receipt := types.GeneratedToolReceipt{Request: request, ExitCode: -1}
	refuse := func(err error) (types.GeneratedToolReceipt, error) {
		receipt.BackendError = err
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	if v == nil || ctx == nil {
		return refuse(fmt.Errorf("generated bridge unavailable"))
	}
	if err := ctx.Err(); err != nil {
		return refuse(err)
	}
	if err := request.Validate(); err != nil {
		return refuse(err)
	}
	v.mu.RLock()
	kernel, registry, executor := v.kernel, v.toolRegistry, v.toolExecutor
	v.mu.RUnlock()
	backend, ok := executor.(types.GeneratedToolBackend)
	if !ok || backend == nil || kernel == nil || registry == nil {
		return refuse(fmt.Errorf("mandatory generated execution bridge disconnected"))
	}
	identity, err := registry.GeneratedToolIdentity(request.Tool.Name)
	if err != nil || identity != request.Tool {
		return refuse(fmt.Errorf("generated registration identity mismatch: %v", err))
	}
	if err := secureValidateCommand(identity.BinaryPath); err != nil {
		return refuse(err)
	}
	if err := secureValidateArgs([]string{request.CanonicalArgs}); err != nil {
		return refuse(err)
	}
	if err := verifyGeneratedAuthorization(kernel, request); err != nil {
		return refuse(err)
	}
	args, err := request.Args()
	if err != nil {
		return refuse(err)
	}
	if err := v.ensureGeneratedValidator(request.Tool.Name); err != nil {
		return refuse(err)
	}
	if err := v.preflightGenerated(ctx, request, args); err != nil {
		return refuse(err)
	}
	receipt, err = backend.ExecuteGenerated(ctx, request, v.validateGeneratedReceipt)
	registry.recordGeneratedReceipt(receipt)
	return receipt, err
}

func verifyGeneratedAuthorization(kernel Kernel, request types.GeneratedToolRequest) error {
	pending, err := kernel.Query("pending_action")
	if err != nil {
		return fmt.Errorf("generated authorization query: %w", err)
	}
	matched := false
	for _, fact := range pending {
		if len(fact.Args) == 5 && types.ExtractString(fact.Args[0]) == request.AuthorizationID &&
			(types.ExtractString(fact.Args[1]) != string(request.Action) || types.ExtractString(fact.Args[2]) != request.Target || types.ExtractString(fact.Args[3]) != request.CanonicalArgs) {
			return fmt.Errorf("conflicting pending generated authorization identity")
		}
		if len(fact.Args) == 5 && types.ExtractString(fact.Args[0]) == request.AuthorizationID &&
			types.ExtractString(fact.Args[1]) == string(request.Action) && types.ExtractString(fact.Args[2]) == request.Target &&
			types.ExtractString(fact.Args[3]) == request.CanonicalArgs {
			matched = true
		}
	}
	if !matched {
		return fmt.Errorf("exact pending generated authorization unavailable")
	}
	query := fmt.Sprintf("permitted(%s, %s, %s)", request.Action, strconv.Quote(request.Target), strconv.Quote(request.CanonicalArgs))
	permitted, err := kernel.Query(query)
	if err != nil {
		return fmt.Errorf("generated permission query: %w", err)
	}
	for _, fact := range permitted {
		if len(fact.Args) == 3 && types.ExtractString(fact.Args[0]) == string(request.Action) &&
			types.ExtractString(fact.Args[1]) == request.Target && types.ExtractString(fact.Args[2]) == request.CanonicalArgs {
			return nil
		}
	}
	return fmt.Errorf("exact generated permission unavailable")
}

func (v *VirtualStore) RetryGeneratedToolFeedback(ctx context.Context, request types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
	if v == nil || ctx == nil {
		return types.GeneratedToolReceipt{}, fmt.Errorf("generated feedback bridge disconnected")
	}
	if err := ctx.Err(); err != nil {
		return types.GeneratedToolReceipt{}, err
	}
	if err := request.Validate(); err != nil {
		return types.GeneratedToolReceipt{}, err
	}
	v.mu.RLock()
	executor, kernel, registry := v.toolExecutor, v.kernel, v.toolRegistry
	v.mu.RUnlock()
	if kernel == nil || registry == nil {
		return types.GeneratedToolReceipt{}, fmt.Errorf("feedback authorization unavailable")
	}
	identity, err := registry.GeneratedToolIdentity(request.Tool.Name)
	if err != nil || identity != request.Tool {
		return types.GeneratedToolReceipt{}, fmt.Errorf("feedback registration identity mismatch")
	}
	if err := verifyGeneratedAuthorization(kernel, request); err != nil {
		return types.GeneratedToolReceipt{}, err
	}
	retry, ok := executor.(types.GeneratedFeedbackRetrier)
	if !ok {
		return types.GeneratedToolReceipt{}, fmt.Errorf("generated feedback retry disconnected")
	}
	return retry.RetryGeneratedFeedback(ctx, request)
}

// The named action remains named in the Dreamer. Its declared arbitrary-code
// effect contributes the same execution projections as other executable tools.
func (v *VirtualStore) preflightGenerated(ctx context.Context, request types.GeneratedToolRequest, args map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dreamer := v.getDreamer()
	if dreamer == nil {
		return fmt.Errorf("generated Dreamer unavailable")
	}
	dreamer.mu.RLock()
	kernel, ready := dreamer.kernel, dreamer.criticalPathsReady
	dreamer.mu.RUnlock()
	if kernel == nil || !ready {
		return fmt.Errorf("generated Dreamer cannot simulate without kernel and critical paths")
	}
	if len(request.Target) > 4096 {
		return fmt.Errorf("generated target exceeds Dreamer bound")
	}
	action := ActionRequest{ActionID: request.AuthorizationID, Type: ActionType(strings.TrimPrefix(string(request.Action), "/")), Target: request.Target, Payload: args}
	projected := dreamer.projectEffects(kernel, request.AuthorizationID, action)
	projected = append(projected, Fact{Predicate: "projected_fact", Args: []any{request.AuthorizationID, MangleAtom("/exec_cmd"), request.Target}})
	if isDangerousCommand(request.Target) {
		projected = append(projected, Fact{Predicate: "projected_fact", Args: []any{request.AuthorizationID, MangleAtom("/exec_danger"), request.Target}})
	}
	if unsafe, reason := dreamer.evaluateProjection(kernel, request.AuthorizationID, projected); unsafe {
		return fmt.Errorf("generated Dreamer refusal: %s", reason)
	}
	return ctx.Err()
}

func (v *VirtualStore) validateGeneratedReceipt(ctx context.Context, request types.GeneratedToolRequest, receipt types.GeneratedToolReceipt) error {
	args, err := request.Args()
	if err != nil {
		return err
	}
	return v.validateNamedGeneratedResult(ctx, request, args, receipt)
}
