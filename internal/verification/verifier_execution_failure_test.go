package verification

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// observingExecutor is a worker that records the request it was given and
// fails with a return that names what it had already done.
type observingExecutor struct {
	*stubTaskExecutor
	reqs []session.TaskRequest
	ret  observation.Return
	err  error
}

func (o *observingExecutor) ExecuteObservedWithContext(_ context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	o.reqs = append(o.reqs, req)
	return o.ret, o.err
}

// A worker that fails after tool work has changed files. The delegation fails
// with the worker's own error, and the observation the worker produced rides
// on it: the caller can say what is on disk and what nobody checked.
func TestVerifyWithRetry_AWorkerFailureKeepsTheWorkersObservation(t *testing.T) {
	cause := errors.New("provider transport: connection reset")
	client, _ := judgeClient(judgeSays)
	exec := &observingExecutor{
		stubTaskExecutor: &stubTaskExecutor{},
		ret:              observation.Return{Output: "wrote a.go", Changed: []string{"a.go"}, Outcome: "/failed", Failure: cause.Error()},
		err:              cause,
	}
	v := newDelegationVerifier(t, client, nil, exec.stubTaskExecutor)
	v.SetTaskExecutor(exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("fix a.go", 3))

	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want the worker's error findable", err)
	}
	var failure *ExecutionError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %T, want the typed execution failure", err)
	}
	if failure.Attempt != 1 || len(failure.Return.Changed) != 1 || failure.Return.Changed[0] != "a.go" {
		t.Errorf("the failure lost the worker's observation: %+v", failure)
	}
	if verdict != nil {
		t.Errorf("a judge verdict exists for a worker that never finished: %+v", verdict)
	}
	// A failed execution is not retried: a blind retry would repeat effects.
	if len(exec.reqs) != 1 {
		t.Errorf("the worker ran %d times after failing, want 1", len(exec.reqs))
	}
	if err.Error() != "shard execution failed: "+cause.Error() {
		t.Errorf("the failure's wording changed: %q", err)
	}
}

// Target and constraint ride on the request as fields, not only inside the
// task text, so the worker's intent holds them.
func TestVerifyWithRetry_TheRequestCarriesTargetAndConstraint(t *testing.T) {
	client, _ := judgeClient(judgeSays)
	exec := &observingExecutor{stubTaskExecutor: &stubTaskExecutor{}, ret: observation.Return{Outcome: "/done"}}
	v := newDelegationVerifier(t, client, nil, exec.stubTaskExecutor)
	v.SetTaskExecutor(exec)

	d := delegation("fix a.go", 1)
	d.Target = "a.go"
	d.Constraint = "keep the exported API"
	_, _, _ = v.VerifyWithRetry(context.Background(), d)

	if len(exec.reqs) == 0 {
		t.Fatal("the worker never ran")
	}
	if got := exec.reqs[0]; got.Target != "a.go" || got.Constraint != "keep the exported API" {
		t.Errorf("request target/constraint = %q/%q", got.Target, got.Constraint)
	}
}
