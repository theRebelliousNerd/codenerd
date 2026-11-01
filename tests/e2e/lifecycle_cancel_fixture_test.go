//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

func talWaitForModelEntry(t *testing.T, model *talMockLLMClient, timeout time.Duration) {
	t.Helper()
	talWaitForModelCalls(t, model, 1, timeout)
}

func talWaitForModelCalls(t *testing.T, model *talMockLLMClient, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for atomic.LoadInt64(&model.workCount) < want {
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("blocking worker never entered the model")
		}
	}
}

func talWaitForRetainedTerminal(t *testing.T, agent *session.SubAgent, want session.SubAgentState, timeout time.Duration) {
	t.Helper()
	if agent == nil {
		t.Fatal("missing retained live agent handle")
	}
	select {
	case <-agent.Done():
	case <-time.After(timeout):
		t.Fatalf("retained agent %s did not reach terminal state %s within %v", agent.GetID(), want, timeout)
	}
	if got := agent.GetState(); got != want {
		t.Fatalf("retained agent %s is %s, want %s", agent.GetID(), got, want)
	}
}

// Observe autonomous settlement before a result API can itself settle the task.
func cancelWaitForRemoval(t *testing.T, spawner *session.Spawner, id string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if _, ok := spawner.Get(id); !ok {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("terminal agent %s was not automatically removed", id)
		}
	}
}

func talRequireCachedCancellation(t *testing.T, env *talEnv, id string) {
	t.Helper()
	cancelWaitForRemoval(t, env.spawner, id)
	output, done, err := env.executor.GetResult(id)
	if !done || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was not cached after automatic removal: done=%v err=%v", done, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	replay, replayErr := env.executor.WaitForResult(ctx, id)
	if replay != output || !errors.Is(replayErr, context.Canceled) {
		t.Fatalf("cached cancellation changed on replay: output=%q replay=%q err=%v", output, replay, replayErr)
	}
	if all, work := atomic.LoadInt64(&env.llm.callCount), atomic.LoadInt64(&env.llm.workCount); all != 1 || work != 1 {
		t.Fatalf("canceled worker dispatched more model work: all=%d work=%d", all, work)
	}
}

type cancelModelEntry struct {
	ctx   context.Context
	input string
}

type cancelModelExit struct {
	input  string
	err    error
	ctxErr error
}

type cancelProbeModel struct {
	*talMockLLMClient
	entered chan cancelModelEntry
	exited  chan cancelModelExit
	failure error
	once    sync.Once
}

func (m *cancelProbeModel) release() { m.once.Do(func() { close(m.blockCh) }) }

func (m *cancelProbeModel) CompleteWithSystem(ctx context.Context, system, input string) (string, error) {
	if broker.PhaseFromContext(ctx) == broker.PhaseAdmissionAudit {
		return m.talMockLLMClient.CompleteWithSystem(ctx, system, input)
	}
	select {
	case m.entered <- cancelModelEntry{ctx: ctx, input: input}:
	default:
		return "", errors.New("cancellation fixture model script exhausted")
	}
	output, err := m.talMockLLMClient.CompleteWithSystem(ctx, system, input)
	if err == nil && m.failure != nil {
		output, err = "", m.failure
	}
	select {
	case m.exited <- cancelModelExit{input: input, err: err, ctxErr: ctx.Err()}:
	default:
		return "", errors.New("cancellation fixture exit witness exhausted")
	}
	return output, err
}

func (m *cancelProbeModel) CompleteWithTools(ctx context.Context, system, input string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	select {
	case m.entered <- cancelModelEntry{ctx: ctx, input: input}:
	default:
		return nil, errors.New("cancellation fixture model script exhausted")
	}
	output, err := m.talMockLLMClient.CompleteWithTools(ctx, system, input, tools)
	if err == nil && m.failure != nil {
		output, err = nil, m.failure
	}
	select {
	case m.exited <- cancelModelExit{input: input, err: err, ctxErr: ctx.Err()}:
	default:
		return nil, errors.New("cancellation fixture exit witness exhausted")
	}
	return output, err
}

type cancelProbe struct {
	jit     *session.JITExecutor
	spawner *session.Spawner
	model   *cancelProbeModel
}

func newCancelProbe(t *testing.T) *cancelProbe {
	t.Helper()
	model := &cancelProbeModel{talMockLLMClient: &talMockLLMClient{blockCh: make(chan struct{})}, entered: make(chan cancelModelEntry, 4), exited: make(chan cancelModelExit, 4)}
	t.Cleanup(model.release)
	vstore, compiler, factory, transducer := &talMockVirtualStore{}, &talMockJITCompiler{}, &talMockConfigFactory{}, &talMockTransducer{intent: "/fix"}
	executor := session.NewExecutor(nil, vstore, model, compiler, factory, transducer)
	executor.SetConfig(session.DefaultExecutorConfig())
	spawner := session.NewSpawner(nil, vstore, model, compiler, factory, transducer, session.DefaultSpawnerConfig())
	wireLifecycleRoles(t, executor, spawner)
	return &cancelProbe{jit: session.NewJITExecutor(executor, spawner, transducer), spawner: spawner, model: model}
}

func (p *cancelProbe) start(t *testing.T, ctx context.Context, marker string) (string, *session.SubAgent, context.Context) {
	t.Helper()
	wantCalls := atomic.LoadInt64(&p.model.workCount) + 1
	id, err := p.jit.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: marker})
	if err != nil {
		t.Fatal(err)
	}
	var entry cancelModelEntry
	select {
	case entry = <-p.model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("worker never entered the controlled model")
	}
	if !strings.Contains(entry.input, marker) || entry.ctx.Err() != nil {
		t.Fatalf("wrong worker admission: input=%q err=%v", entry.input, entry.ctx.Err())
	}
	talWaitForModelCalls(t, p.model.talMockLLMClient, wantCalls, 5*time.Second)
	agent, ok := p.spawner.Get(id)
	if !ok || agent.GetState() != session.SubAgentStateRunning {
		t.Fatal("admitted blocked worker is not live")
	}
	if _, done, err := p.jit.GetResult(id); done || err != nil {
		t.Fatalf("blocked worker already has a terminal result: done=%v err=%v", done, err)
	}
	return id, agent, entry.ctx
}

func (p *cancelProbe) exit(t *testing.T, marker string) cancelModelExit {
	t.Helper()
	select {
	case exit := <-p.model.exited:
		if !strings.Contains(exit.input, marker) {
			t.Fatalf("received another worker's exit: %q", exit.input)
		}
		return exit
	case <-time.After(5 * time.Second):
		t.Fatal("controlled model never acknowledged completion")
		return cancelModelExit{}
	}
}

func TestCancellationObservedOwner(t *testing.T) {
	for _, mode := range []string{"waiter_cancel", "parent_cancel", "normal_completion", "execution_error"} {
		t.Run(mode, func(t *testing.T) {
			probe := newCancelProbe(t)
			parent, cancelParent := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelParent()
			if mode == "execution_error" {
				probe.model.failure = errors.New("controlled worker execution failure")
			}
			id, agent, worker := probe.start(t, parent, "owned-cancel-marker")
			var waitErr error
			if mode == "waiter_cancel" {
				waiter, cancelWaiter := context.WithCancel(parent)
				cancelWaiter()
				_, waitErr = probe.jit.WaitForResult(waiter, id)
				if !errors.Is(waitErr, context.Canceled) || parent.Err() != nil {
					t.Fatalf("waiter cancellation lost its ownership: err=%v parent=%v", waitErr, parent.Err())
				}
			} else if mode == "parent_cancel" {
				cancelParent()
			} else {
				probe.model.release()
			}
			exit := probe.exit(t, "owned-cancel-marker")
			canceled := mode == "waiter_cancel" || mode == "parent_cancel"
			wantState := session.SubAgentStateCompleted
			if canceled || mode == "execution_error" {
				wantState = session.SubAgentStateFailed
			}
			talWaitForRetainedTerminal(t, agent, wantState, 5*time.Second)
			cancelWaitForRemoval(t, probe.spawner, id)
			output, done, err := probe.jit.GetResult(id)
			if !done {
				t.Fatal("autonomously settled worker has no cached terminal result")
			}
			if canceled {
				if !errors.Is(worker.Err(), context.Canceled) || !errors.Is(exit.ctxErr, context.Canceled) || !errors.Is(exit.err, context.Canceled) || !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation was only reported, not observed by the model: worker=%v exit=%+v cached=%v", worker.Err(), exit, err)
				}
				select {
				case <-probe.model.blockCh:
					t.Fatal("test released the model instead of canceling it")
				default:
				}
			} else if exit.ctxErr != nil || parent.Err() != nil {
				t.Fatalf("ordinary completion was canceled: exit=%+v parent=%v", exit, parent.Err())
			} else if mode == "execution_error" {
				if exit.err != probe.model.failure || err == nil || !strings.Contains(err.Error(), probe.model.failure.Error()) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("execution failure was confused with cancellation: exit=%+v cached=%v", exit, err)
				}
			} else if err != nil || exit.err != nil || !strings.Contains(output, "owned-cancel-marker") {
				t.Fatalf("normal completion lost its result: output=%q err=%v exit=%+v", output, err, exit)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			replay, replayErr := probe.jit.WaitForResult(ctx, id)
			if replay != output || (replayErr == nil) != (err == nil) || (err != nil && replayErr.Error() != err.Error()) {
				t.Fatalf("cached result changed on replay: first=%q %v replay=%q %v", output, err, replay, replayErr)
			}
			wantCalls := int64(1)
			if mode == "normal_completion" {
				wantCalls++ // The unchanged final-report audit still runs.
			}
			if all, work := atomic.LoadInt64(&probe.model.callCount), atomic.LoadInt64(&probe.model.workCount); all != wantCalls || work != 1 || len(probe.spawner.ListActive()) != 0 {
				t.Fatalf("result replay dispatched work or leaked an agent: all=%d work=%d active=%d", all, work, len(probe.spawner.ListActive()))
			}
		})
	}
}

func TestCancellationUnrelatedOwnerIsolation(t *testing.T) {
	probe := newCancelProbe(t)
	parent, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	firstID, first, _ := probe.start(t, parent, "cancel-first-marker")
	secondID, second, secondCtx := probe.start(t, parent, "retain-second-marker")
	waiter, stopWaiter := context.WithCancel(parent)
	stopWaiter()
	if _, err := probe.jit.WaitForResult(waiter, firstID); !errors.Is(err, context.Canceled) {
		t.Fatalf("first waiter did not cancel its worker: %v", err)
	}
	exit := probe.exit(t, "cancel-first-marker")
	if !errors.Is(exit.ctxErr, context.Canceled) || !errors.Is(exit.err, context.Canceled) {
		t.Fatalf("first model did not acknowledge cancellation: %+v", exit)
	}
	talWaitForRetainedTerminal(t, first, session.SubAgentStateFailed, 5*time.Second)
	cancelWaitForRemoval(t, probe.spawner, firstID)
	if secondCtx.Err() != nil || parent.Err() != nil || second.GetState() != session.SubAgentStateRunning {
		t.Fatalf("first cancellation stopped another owner: second=%v parent=%v state=%v", secondCtx.Err(), parent.Err(), second.GetState())
	}
	if got, ok := probe.spawner.Get(secondID); !ok || got != second {
		t.Fatal("unrelated live owner was removed or replaced")
	}
	if _, done, err := probe.jit.GetResult(secondID); done || err != nil {
		t.Fatalf("unrelated blocked owner acquired a terminal result: done=%v err=%v", done, err)
	}
	if _, done, err := probe.jit.GetResult(firstID); !done || !errors.Is(err, context.Canceled) {
		t.Fatalf("first cancellation was not independently cached: done=%v err=%v", done, err)
	}
	if all, work := atomic.LoadInt64(&probe.model.callCount), atomic.LoadInt64(&probe.model.workCount); all != 2 || work != 2 {
		t.Fatalf("canceled first owner started later work: all=%d work=%d", all, work)
	}
	probe.model.release()
	if exit := probe.exit(t, "retain-second-marker"); exit.err != nil || exit.ctxErr != nil {
		t.Fatalf("unrelated worker failed after normal release: %+v", exit)
	}
	talWaitForRetainedTerminal(t, second, session.SubAgentStateCompleted, 5*time.Second)
	cancelWaitForRemoval(t, probe.spawner, secondID)
	output, err := probe.jit.WaitForResult(parent, secondID)
	if err != nil || !strings.Contains(output, "retain-second-marker") || strings.Contains(output, "cancel-first-marker") {
		t.Fatalf("unrelated worker lost its own result: %q %v", output, err)
	}
	if all, work := atomic.LoadInt64(&probe.model.callCount), atomic.LoadInt64(&probe.model.workCount); all != 3 || work != 2 || len(probe.spawner.ListActive()) != 0 {
		t.Fatalf("owner isolation leaked work: all=%d work=%d active=%d", all, work, len(probe.spawner.ListActive()))
	}
}
