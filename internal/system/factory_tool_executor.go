package system

import (
	"context"
	"fmt"
	"sync"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// orchestratorToolExecutor is the VirtualStore's tool executor: it runs a
// generated tool through the autopoiesis Orchestrator's evaluate-and-profile
// path so every execution feeds the tool's learning record, and triggers an
// asynchronous refinement when the orchestrator says the tool needs one.
// Moved from cmd/nerd/chat (ToolExecutorAdapter) so every boot path — not
// only the TUI — executes tools with the feedback loop; the factory used to
// hand the VirtualStore the bare OuroborosLoop, which executes without it.
type orchestratorToolExecutor struct {
	orchestrator *autopoiesis.Orchestrator
	// lifetime is cancelled when the autopoiesis context ends (Cortex.Close).
	// Refinement is not a child of the tool-call context: ExecuteTool returns
	// the tool output immediately, and that context ends with the call.
	lifetime context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	active   sync.WaitGroup
	calls    map[string]*generatedLiveCall
}

type generatedLiveCall struct {
	mu          sync.Mutex
	fingerprint string
	done        chan struct{}
	receipt     types.GeneratedToolReceipt
}

func newOrchestratorToolExecutor(orch *autopoiesis.Orchestrator, lifetime context.Context) *orchestratorToolExecutor {
	if lifetime == nil {
		lifetime = context.Background()
	}
	owned, cancel := context.WithCancel(lifetime)
	return &orchestratorToolExecutor{orchestrator: orch, lifetime: owned, cancel: cancel, calls: make(map[string]*generatedLiveCall)}
}

func (a *orchestratorToolExecutor) Close() error {
	a.mu.Lock()
	a.closed = true
	a.cancel()
	a.mu.Unlock()
	a.active.Wait()
	return nil
}

func (a *orchestratorToolExecutor) GeneratedToolIdentity(name string) (types.GeneratedToolIdentity, error) {
	if a == nil {
		return types.GeneratedToolIdentity{}, fmt.Errorf("generated factory adapter disconnected")
	}
	return a.orchestrator.GeneratedToolIdentity(name)
}

func (a *orchestratorToolExecutor) ExecuteGenerated(ctx context.Context, request types.GeneratedToolRequest, validate types.GeneratedToolValidator) (receipt types.GeneratedToolReceipt, err error) {
	receipt = types.GeneratedToolReceipt{Request: request, ExitCode: -1}
	if a == nil || ctx == nil {
		receipt.BackendError = fmt.Errorf("generated factory adapter disconnected")
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	if validationErr := request.Validate(); validationErr != nil {
		receipt.BackendError = validationErr
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	if validate == nil || a.orchestrator == nil {
		receipt.BackendError = fmt.Errorf("generated validation/feedback route disconnected")
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		receipt.BackendError = ctxErr
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	a.mu.Lock()
	if a.closed || a.lifetime.Err() != nil {
		a.mu.Unlock()
		receipt.BackendError = fmt.Errorf("generated admission closed")
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	if prior := a.calls[request.Key()]; prior != nil {
		completed := false
		select {
		case <-prior.done:
			completed = true
		default:
		}
		a.mu.Unlock()
		if prior.fingerprint != request.Fingerprint() {
			receipt.BackendError = fmt.Errorf("conflicting live generated call identity")
			return receipt, &types.GeneratedExecutionError{Receipt: receipt}
		}
		select {
		case <-prior.done:
			prior.mu.Lock()
			receipt = prior.receipt
			prior.mu.Unlock()
			receipt.Replayed = true
			if completed && receipt.ProcessStarted && receipt.FeedbackError != nil {
				receipt, err = a.RetryGeneratedFeedback(ctx, request)
				receipt.Replayed = true
				return receipt, err
			}
			if receipt.Err() != nil {
				return receipt, &types.GeneratedExecutionError{Receipt: receipt}
			}
			return receipt, nil
		case <-ctx.Done():
			receipt.SharedCallPending = true
			receipt.BackendError = ctx.Err()
			return receipt, &types.GeneratedExecutionError{Receipt: receipt}
		case <-a.lifetime.Done():
			receipt.SharedCallPending = true
			receipt.BackendError = a.lifetime.Err()
			return receipt, &types.GeneratedExecutionError{Receipt: receipt}
		}
	}
	call := &generatedLiveCall{fingerprint: request.Fingerprint(), done: make(chan struct{})}
	a.calls[request.Key()] = call
	a.active.Add(1)
	a.mu.Unlock()
	workCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(a.lifetime, cancel)
	defer func() {
		stop()
		cancel()
		if recovered := recover(); recovered != nil {
			receipt.FeedbackError = fmt.Errorf("generated route panic: %v", recovered)
		}
		call.mu.Lock()
		call.receipt = receipt
		call.mu.Unlock()
		close(call.done)
		a.active.Done()
		if receipt.Err() != nil {
			err = &types.GeneratedExecutionError{Receipt: receipt}
		}
	}()
	receipt = a.orchestrator.ExecuteGeneratedBackend(workCtx, request)
	if !receipt.ProcessStarted {
		return receipt, nil
	}
	receipt.ValidationCompleted = true
	receipt.ValidationError = containGeneratedValidation(workCtx, request, receipt, validate)
	receipt.ValidationPassed = receipt.ValidationError == nil
	// Cancellation of the effect still owes a synchronous terminal learning.
	// This cleanup stays on the admitted stack and is joined by Close.
	receipt.Feedback, receipt.FeedbackError = a.recordGenerated(context.WithoutCancel(workCtx), receipt)
	if receipt.Feedback.Durable && receipt.FeedbackError == nil {
		a.startRefinement(request.Tool.Name)
	}
	return receipt, nil
}

func containGeneratedValidation(ctx context.Context, request types.GeneratedToolRequest, receipt types.GeneratedToolReceipt, validate types.GeneratedToolValidator) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("generated validator panic: %v", recovered)
		}
	}()
	return validate(ctx, request, receipt)
}

func (a *orchestratorToolExecutor) recordGenerated(ctx context.Context, receipt types.GeneratedToolReceipt) (ack types.GeneratedLearningAck, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("generated recorder panic: %v", recovered)
		}
	}()
	return a.orchestrator.RecordGeneratedExecution(ctx, receipt)
}

// RetryGeneratedFeedback only retries publication of a joined, retained receipt.
// It can never admit another process, including after a successful reconnect.
func (a *orchestratorToolExecutor) RetryGeneratedFeedback(ctx context.Context, request types.GeneratedToolRequest) (receipt types.GeneratedToolReceipt, err error) {
	if err := request.Validate(); err != nil {
		return receipt, err
	}
	if err := ctx.Err(); err != nil {
		return receipt, err
	}
	a.mu.Lock()
	call := a.calls[request.Key()]
	if a.closed || a.lifetime.Err() != nil || call == nil || call.fingerprint != request.Fingerprint() {
		a.mu.Unlock()
		return receipt, fmt.Errorf("feedback retry has no matching live receipt")
	}
	select {
	case <-call.done:
	default:
		a.mu.Unlock()
		return receipt, fmt.Errorf("generated call still active")
	}
	a.active.Add(1)
	a.mu.Unlock()
	defer a.active.Done()
	call.mu.Lock()
	defer call.mu.Unlock()
	receipt = call.receipt
	if !receipt.ProcessStarted {
		return receipt, fmt.Errorf("refused process has no feedback")
	}
	if receipt.Feedback.Durable && receipt.FeedbackError == nil {
		if receipt.Err() != nil {
			return receipt, &types.GeneratedExecutionError{Receipt: receipt}
		}
		return receipt, nil
	}
	receipt.FeedbackError = nil
	receipt.Feedback, receipt.FeedbackError = a.recordGenerated(ctx, receipt)
	call.receipt = receipt
	if receipt.Err() != nil {
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	return receipt, nil
}

func (a *orchestratorToolExecutor) startRefinement(toolName string) {
	a.mu.Lock()
	if a.closed || a.lifetime.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.active.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.active.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				logging.Get(logging.CategoryAutopoiesis).Error("Tool refinement panic: %v", recovered)
			}
		}()
		if !a.orchestrator.ShouldRefineToolByKernel(toolName) {
			return
		}
		ctx, cancel := toolRefinementContext(a.lifetime)
		defer cancel()
		if _, err := a.orchestrator.RefineTool(ctx, toolName, ""); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Tool refinement failed: %v", err)
		}
	}()
}

// toolRefinementContext bounds one refinement completion. RefineTool is a
// single LLM call (legacy or JIT). The deadline is llm_timeouts.per_call_timeout.
// lifetime is the process shutdown context, so Close cancels an in-flight
// refinement instead of letting a detached Background run out a wall clock.
func toolRefinementContext(lifetime context.Context) (context.Context, context.CancelFunc) {
	if lifetime == nil {
		lifetime = context.Background()
	}
	timeout := config.GetLLMTimeouts().PerCallTimeout
	if timeout <= 0 {
		return context.WithCancel(lifetime)
	}
	return context.WithTimeout(lifetime, timeout)
}

// ExecuteTool runs a registered tool with the given input.
func (a *orchestratorToolExecutor) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	a.mu.Lock()
	if a.closed || a.lifetime.Err() != nil {
		a.mu.Unlock()
		return "", fmt.Errorf("generated admission closed")
	}
	a.active.Add(1)
	a.mu.Unlock()
	defer a.active.Done()
	workCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(a.lifetime, cancel)
	defer func() { stop(); cancel() }()
	output, _, err := a.orchestrator.ExecuteAndEvaluateWithProfile(workCtx, toolName, input)
	if err != nil {
		return output, err
	}

	// ExecuteAndEvaluateWithProfile already records execution feedback.
	// Only the refinement decision remains here.
	a.startRefinement(toolName)

	return output, nil
}

// ListTools returns all registered tools.
func (a *orchestratorToolExecutor) ListTools() []core.ToolInfo {
	autoTools := a.orchestrator.ListTools()
	coreTools := make([]core.ToolInfo, len(autoTools))
	for i, t := range autoTools {
		coreTools[i] = core.ToolInfo(t)
	}
	return coreTools
}

// GetTool returns info about a specific tool.
func (a *orchestratorToolExecutor) GetTool(name string) (*core.ToolInfo, bool) {
	for _, info := range a.ListTools() {
		if info.Name == name {
			copyInfo := info
			return &copyInfo, true
		}
	}
	return nil, false
}

var _ core.ToolExecutor = (*orchestratorToolExecutor)(nil)
