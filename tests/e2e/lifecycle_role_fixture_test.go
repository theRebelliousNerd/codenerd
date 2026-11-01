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
	appconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

// Lifecycle doubles keep their original nil execution kernel and inert worker.
// Their separate role compiler uses shipped atoms and production selection;
// this wires the audit rather than granting effects or task acceptance.
func wireLifecycleRoles(t *testing.T, executor *session.Executor, spawner *session.Spawner) types.RolePromptCompiler {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetWorkspace(t.TempDir())
	compiler, err := prompt.NewJITPromptCompiler(prompt.WithKernel(system.NewKernelAdapter(kernel)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if spawner != nil {
			owned := spawner.ListActive()
			spawner.StopAll()
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			for _, agent := range owned {
				select {
				case <-agent.Done():
				case <-deadline.C:
					t.Error("fixture-owned worker did not stop before role compiler cleanup")
					return
				}
			}
		}
		if err := compiler.Close(); err != nil {
			t.Errorf("close lifecycle role compiler: %v", err)
		}
	})
	roles, err := prompt.NewRoleCompiler(compiler, appconfig.DefaultJITConfig())
	if err != nil {
		t.Fatal(err)
	}
	if executor != nil {
		executor.SetRolePrompts(roles)
	}
	if spawner != nil {
		spawner.SetRolePrompts(roles)
	}
	return roles
}

func TestLifecycleFixtureInstalledAudit(t *testing.T) {
	env := setupTALEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "lifecycle-owned-marker"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := env.executor.WaitForResult(ctx, id)
	if err != nil || !strings.Contains(result, "lifecycle-owned-marker") {
		t.Fatalf("fixture did not deliver its worker result: %q %v", result, err)
	}
	if work, all := atomic.LoadInt64(&env.llm.workCount), atomic.LoadInt64(&env.llm.callCount); work != 1 || all != 2 {
		t.Fatalf("want one worker plus one audit, got work=%d all=%d: %s", work, all, env.llm.callHeads())
	}
	cached, done, err := env.executor.GetResult(id)
	if err != nil || !done || cached != result || atomic.LoadInt64(&env.llm.callCount) != 2 {
		t.Fatalf("cached completion was lost or reran the model: %q %v %v", cached, done, err)
	}
	if _, ok := env.spawner.Get(id); ok || len(env.spawner.ListActive()) != 0 {
		t.Fatal("completed fixture left an owned agent registered")
	}
}

func TestLifecycleWorkerCountingUsesPhase(t *testing.T) {
	model := &talMockLLMClient{}
	// Dummy strings deliberately disagree with the phase. They are test data,
	// not replacement role instructions sent by a production executor.
	if _, err := model.CompleteWithSystem(context.Background(), "You read an agent's final report -- worker test data", "worker marker"); err != nil {
		t.Fatal(err)
	}
	if all, work := atomic.LoadInt64(&model.callCount), atomic.LoadInt64(&model.workCount); all != 1 || work != 1 {
		t.Fatalf("worker wording was mistaken for audit ownership: all=%d work=%d", all, work)
	}
	if _, err := model.CompleteWithSystem(broker.WithPhase(context.Background(), broker.PhaseAdmissionAudit), "compiled role wrapper", "audit marker"); err != nil {
		t.Fatal(err)
	}
	if all, work := atomic.LoadInt64(&model.callCount), atomic.LoadInt64(&model.workCount); all != 2 || work != 1 {
		t.Fatalf("prompt wording changed call ownership: all=%d work=%d", all, work)
	}
}

type lifecycleAuditModel struct {
	*talMockLLMClient
	captureMu sync.Mutex
	system    string
	input     string
	audits    int
	answer    string
}

func (m *lifecycleAuditModel) CompleteWithSystem(ctx context.Context, system, input string) (string, error) {
	answer, err := m.talMockLLMClient.CompleteWithSystem(ctx, system, input)
	if err != nil || broker.PhaseFromContext(ctx) != broker.PhaseAdmissionAudit {
		return answer, err
	}
	m.captureMu.Lock()
	defer m.captureMu.Unlock()
	m.system, m.input = system, input
	m.audits++
	return m.answer, nil
}

type lifecycleFailedRoles struct{}

func (lifecycleFailedRoles) CompileRolePrompt(context.Context, types.RolePromptRequest) (string, error) {
	return "", types.ErrRolePromptUnavailable
}

func TestLifecycleRoleModelBoundary(t *testing.T) {
	for _, mode := range []string{"compiled", "missing", "failed", "unfinished"} {
		t.Run(mode, func(t *testing.T) {
			model := &lifecycleAuditModel{talMockLLMClient: &talMockLLMClient{}, answer: "COMPLETE"}
			executor := session.NewExecutor(nil, &talMockVirtualStore{}, model, &talMockJITCompiler{}, &talMockConfigFactory{}, &talMockTransducer{intent: "/fix"})
			executor.SetConfig(session.DefaultExecutorConfig())
			wireLifecycleRoles(t, executor, nil)
			if mode == "missing" {
				executor.SetRolePrompts(nil)
			} else if mode == "failed" {
				executor.SetRolePrompts(lifecycleFailedRoles{})
			} else if mode == "unfinished" {
				model.answer = "INCOMPLETE"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := executor.Process(ctx, "role-boundary-task-marker")
			if result == nil {
				t.Fatalf("missing measured turn result: %v", err)
			}
			if mode == "missing" || mode == "failed" {
				if !errors.Is(errors.Join(err, result.Error), types.ErrRolePromptUnavailable) || model.audits != 0 {
					t.Fatalf("unavailable role was silently bypassed: err=%v resultErr=%v audits=%d", err, result.Error, model.audits)
				}
				return
			}
			if err != nil || result.Error != nil || model.audits != 1 {
				t.Fatalf("configured role did not reach model: err=%v resultErr=%v audits=%d", err, result.Error, model.audits)
			}
			corpus, err := prompt.LoadEmbeddedCorpus()
			if err != nil {
				t.Fatal(err)
			}
			atom, ok := corpus.Get("system/final_report_auditor/contract")
			if !ok || !strings.Contains(model.system, strings.TrimSpace(atom.Content)) || !strings.Contains(model.input, "Task:\nrole-boundary-task-marker") || !strings.Contains(model.input, result.Response) {
				t.Fatal("model did not receive shipped role atoms and this turn's actual task/report")
			}
			if result.SelfReportedIncomplete != (mode == "unfinished") {
				t.Fatalf("audit answer was lost: incomplete=%v mode=%s", result.SelfReportedIncomplete, mode)
			}
			if result.WriteToolEffects != 0 || result.ToolCallsExecuted != 0 {
				t.Fatal("inert lifecycle fixture unexpectedly acquired tool effects")
			}
		})
	}
}

// Retain the live handle before cancellation. The terminal result belongs to
// the JITExecutor cache; the spawner must not retain a zombie just for a poll.
func TestLifecycleRoleCancellationOwnership(t *testing.T) {
	env := setupTALEnvironment(t)
	env.llm.blockCh = make(chan struct{})
	defer close(env.llm.blockCh)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "blocked-owned-marker"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt64(&env.llm.workCount) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("worker never entered the blocking model")
		}
		time.Sleep(time.Millisecond)
	}
	agent, ok := env.spawner.Get(id)
	if !ok {
		t.Fatal("blocked live worker was already missing")
	}
	cancel()
	select {
	case <-agent.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("owner cancellation did not stop the worker")
	}
	_, done, err := env.executor.GetResult(id)
	if !done || !errors.Is(err, context.Canceled) {
		t.Fatalf("terminal cancellation was not cached: done=%v err=%v", done, err)
	}
	if _, ok := env.spawner.Get(id); ok || len(env.spawner.ListActive()) != 0 {
		t.Fatal("canceled worker remained registered or active")
	}
	if all, work := atomic.LoadInt64(&env.llm.callCount), atomic.LoadInt64(&env.llm.workCount); all != 1 || work != 1 {
		t.Fatalf("canceled worker dispatched later audit work: all=%d work=%d", all, work)
	}
}
