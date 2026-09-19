package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
)

// External audit F2 (2026-09-19): spawnTask took the string route
// (TaskExecutor.Execute), which returns the shard's prose and drops the typed
// verdict beside it, so a turn the kernel judged /unverified -- tests not
// green, production code with no test -- returned a nil error and completed
// the task. The campaign reads the verdict now: anything but /done fails the
// attempt, named by what the turn left missing.
func TestSpawnTask_OnlyADoneTurnCompletesTheAttempt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ret     observation.Return
		wantErr []string
	}{
		{
			name:    "an unverified turn fails, named by its missing evidence",
			ret:     observation.Return{Output: "Wrote 1 file(s).", Outcome: "/unverified", Missing: []string{"/tests_not_green", "/tests_not_written"}},
			wantErr: []string{"ended /unverified", "the tests were not verified green", "production code was written with no test beside it"},
		},
		{
			name:    "a turn with no verdict fails",
			ret:     observation.Return{Output: "done"},
			wantErr: []string{"returned no verdict"},
		},
		{
			name: "a done turn completes",
			ret:  observation.Return{Output: "done", Outcome: "/done"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &Orchestrator{taskExecutor: &MockTaskExecutor{
				ExecuteObservedFunc: func(context.Context, session.TaskRequest) (observation.Return, error) { return tc.ret, nil },
			}}
			out, err := o.spawnTask(context.Background(), nil, "/fix", "create internal/a/a.go")
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("spawnTask = %v, want success for a /done turn", err)
				}
				if out != tc.ret.Output {
					t.Fatalf("output = %q, want %q", out, tc.ret.Output)
				}
				return
			}
			if !errors.Is(err, ErrTaskNotDone) {
				t.Fatalf("spawnTask error = %v, want ErrTaskNotDone", err)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say %q", err, want)
				}
			}
		})
	}
}

// A string-only executor cannot run a campaign: its /unverified turns would
// read as success.
func TestSpawnTask_AnExecutorWithoutAVerdictIsRefused(t *testing.T) {
	o := &Orchestrator{taskExecutor: proseOnlyExecutor{}}
	if _, err := o.spawnTask(context.Background(), nil, "/fix", "x"); err == nil || !strings.Contains(err.Error(), "no observed result") {
		t.Fatalf("spawnTask = %v, want a refusal naming the missing observed result", err)
	}
	_, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       &MockKernel{},
		LLMClient:    &MockLLMClient{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		TaskExecutor: proseOnlyExecutor{},
	})
	if err == nil || !strings.Contains(err.Error(), "observed result") {
		t.Fatalf("NewOrchestrator = %v, want the prose-only executor refused", err)
	}
}

// Ladder C3: a retry re-spawned the same request without the failure that
// stopped it. The next attempt's input carries the last failed attempt's
// error, read from the live campaign.
func TestSpawnTask_ARetryCarriesWhyThePreviousAttemptFailed(t *testing.T) {
	const reason = "coder shard failed for /file_create task t1 on internal/a/a.go: the /fix turn ended /unverified: production code was written with no test beside it"
	c := &Campaign{ID: "/c", Phases: []Phase{{ID: "/p", Tasks: []Task{
		{ID: "t1", Attempts: []TaskAttempt{{Number: 1, Outcome: "/failure", Error: reason}}},
		{ID: "t2"},
	}}}}
	var got []string
	o := &Orchestrator{campaign: c, taskExecutor: &MockTaskExecutor{
		ExecuteObservedFunc: func(_ context.Context, req session.TaskRequest) (observation.Return, error) {
			got = append(got, req.Task)
			return observation.Return{Outcome: "/done"}, nil
		},
	}}
	// The caller's pointer is a copy: the attempt is read from the campaign.
	retried := Task{ID: "t1"}
	if _, err := o.spawnTask(context.Background(), &retried, "/fix", "create internal/a/a.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := o.spawnTask(context.Background(), &c.Phases[0].Tasks[1], "/fix", "create internal/b/b.go"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got[0], "create internal/a/a.go") || !strings.Contains(got[0], reason) {
		t.Fatalf("retry input = %q, want the request followed by the previous failure", got[0])
	}
	if got[1] != "create internal/b/b.go" {
		t.Fatalf("first-attempt input = %q, want the request unchanged", got[1])
	}
}

type proseOnlyExecutor struct{ session.TaskExecutor }
