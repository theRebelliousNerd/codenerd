package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// scriptedDispatch answers each dispatch with the next scripted return and
// records the task text it was sent.
type scriptedDispatch struct {
	returns []observation.Return
	errs    []error
	tasks   []string
}

func (s *scriptedDispatch) dispatch(_ context.Context, req TaskRequest) (observation.Return, error) {
	i := len(s.tasks)
	s.tasks = append(s.tasks, req.Task)
	if i >= len(s.returns) {
		return observation.Return{}, errors.New("scripted dispatch: out of returns")
	}
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	return s.returns[i], err
}

func unverified(output string, changed []string, missing ...string) observation.Return {
	return observation.Return{Output: output, Outcome: "/unverified", Missing: missing, Changed: changed}
}

// createVerdictExecutor is a test executor over the shipped kernel: a turn the
// kernel can judge ends /done or names what it lacks, where a MockKernel gives
// no verdict at all and every turn reads /unverified.
func createVerdictExecutor(t *testing.T) *Executor {
	t.Helper()
	e := createTestExecutor(t)
	e.kernel = realKernel(t)
	return e
}

func loopKernel(t *testing.T) types.Kernel {
	t.Helper()
	k := realKernel(t)
	if err := config.EnsureParams(k, (&config.TypeSafeConfig{}).Params()); err != nil {
		t.Fatal(err)
	}
	return k
}

// The kernel's truth table for the last dispatch (policy/task_verdict.mg).
func TestTaskVerdictPolicy_TruthTable(t *testing.T) {
	type dispatch struct {
		outcome string
		changed int64
		gaps    []string
		blocked int64 // -1: no reading
	}
	for _, tc := range []struct {
		name string
		rows []dispatch
		want string
	}{
		{"done", []dispatch{{"/done", 1, nil, -1}}, "/verified"},
		{"first unverified with a gap goes out again", []dispatch{{"/unverified", 0, []string{"/tests_not_green"}, -1}}, "/redispatch"},
		{"second wrote", []dispatch{{"/unverified", 0, []string{"/tests_not_green"}, -1}, {"/unverified", 2, []string{"/tests_not_green"}, -1}}, "/redispatch"},
		{"second cleared a gap without a write", []dispatch{{"/unverified", 0, []string{"/tests_not_green", "/vet_not_clean"}, -1}, {"/unverified", 0, []string{"/tests_not_green"}, -1}}, "/redispatch"},
		{"second wrote nothing and cleared nothing", []dispatch{{"/unverified", 1, []string{"/tests_not_green"}, -1}, {"/unverified", 0, []string{"/tests_not_green"}, -1}}, "/stalled"},
		{"a new gap is not a cleared one", []dispatch{{"/unverified", 1, []string{"/tests_not_green"}, -1}, {"/unverified", 0, []string{"/tests_not_green", "/vet_not_clean"}, -1}}, "/stalled"},
		{"a named blocker stops the first", []dispatch{{"/unverified", 0, []string{"/self_reported_incomplete"}, 97}}, "/blocked"},
		{"a blocker reading under the bar does not", []dispatch{{"/unverified", 0, []string{"/self_reported_incomplete"}, 34}}, "/redispatch"},
		{"the blocker of the last dispatch decides", []dispatch{{"/unverified", 0, []string{"/tests_not_green"}, 98}, {"/unverified", 1, []string{"/tests_not_green"}, 3}}, "/redispatch"},
		{"no verdict", []dispatch{{"/no_verdict", 0, nil, -1}}, "/unjudged"},
		{"unverified naming nothing", []dispatch{{"/unverified", 3, nil, -1}}, "/unjudged"},
		{"done after a stall", []dispatch{{"/unverified", 1, []string{"/tests_not_green"}, -1}, {"/done", 0, nil, -1}}, "/verified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := loopKernel(t)
			loop := types.MangleAtom("/task_loop_test")
			var facts []types.Fact
			for i, d := range tc.rows {
				n := int64(i + 1)
				facts = append(facts, types.Fact{Predicate: "task_dispatch", Args: []any{loop, n, types.MangleAtom(d.outcome), d.changed}})
				for _, g := range d.gaps {
					facts = append(facts, types.Fact{Predicate: "task_dispatch_gap", Args: []any{loop, n, types.MangleAtom(g)}})
				}
				if d.blocked >= 0 {
					facts = append(facts, types.Fact{Predicate: "task_dispatch_blocked", Args: []any{loop, n, d.blocked}})
				}
			}
			if err := k.AssertBatch(facts); err != nil {
				t.Fatal(err)
			}
			got, err := loopVerdict(k, loop)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("task_loop_verdict = %s, want %s", got, tc.want)
			}
		})
	}
}

// A verified turn returns at once: no kernel, no second dispatch.
func TestRunUntilVerified_ADoneTurnIsReturnedAsIs(t *testing.T) {
	s := &scriptedDispatch{returns: []observation.Return{{Output: "done", Outcome: "/done"}}}
	ret, err := RunUntilVerified(context.Background(), nil, s.dispatch, TaskRequest{IntentVerb: "/fix", Task: "fix it"})
	if err != nil || ret.Output != "done" || len(s.tasks) != 1 {
		t.Fatalf("ret=%+v err=%v dispatches=%d", ret, err, len(s.tasks))
	}
}

// An unverified turn goes out again with what is missing and the report it
// ended with, until it is verified; the writes of every dispatch come back.
func TestRunUntilVerified_AnUnverifiedTurnIsSentBackWithTheGap(t *testing.T) {
	k := loopKernel(t)
	first := unverified("Wrote a.go. TestZero is left.", []string{"a.go"}, "/self_reported_incomplete", "/tests_not_written")
	first.Untested = []string{"a.go"}
	first.Writes = []observation.FileWrite{{Path: "/w/a.go", Before: observation.FileState{Known: true}, After: observation.FileState{Known: true, Exists: true, Content: "v1"}}}
	second := observation.Return{Output: "Added TestZero.", Outcome: "/done", Changed: []string{"a_test.go"},
		Writes: []observation.FileWrite{
			{Path: "/w/a.go", Before: observation.FileState{Known: true, Exists: true, Content: "v1"}, After: observation.FileState{Known: true, Exists: true, Content: "v2"}},
			{Path: "/w/a_test.go", After: observation.FileState{Known: true, Exists: true, Content: "t"}},
		}}
	s := &scriptedDispatch{returns: []observation.Return{first, second}}
	ret, err := RunUntilVerified(context.Background(), k, s.dispatch, TaskRequest{IntentVerb: "/fix", Task: "add the tests"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.tasks) != 2 {
		t.Fatalf("dispatched %d time(s), want 2", len(s.tasks))
	}
	if !strings.HasPrefix(s.tasks[0], "add the tests") || s.tasks[0] != "add the tests" {
		t.Fatalf("the first dispatch carries %q, want the task as given", s.tasks[0])
	}
	for _, want := range []string{"add the tests", "ENDED UNVERIFIED", "the turn's own report says part of the requested work was not done", "production code was written with no test beside it", "Wrote a.go. TestZero is left.", "a.go"} {
		if !strings.Contains(s.tasks[1], want) {
			t.Errorf("the second dispatch does not carry %q:\n%s", want, s.tasks[1])
		}
	}
	if !ret.Done() || ret.Output != "Added TestZero." {
		t.Fatalf("the loop's return is the verified dispatch's: %+v", ret)
	}
	if got := strings.Join(ret.Changed, " "); got != "a.go a_test.go" {
		t.Fatalf("changed = %q, want both dispatches' writes", got)
	}
	if len(ret.Writes) != 2 || ret.Writes[0].Path != "/w/a.go" || ret.Writes[0].Before.Exists || ret.Writes[0].After.Content != "v2" {
		t.Fatalf("a path keeps what it held before the first write and what the last left: %+v", ret.Writes)
	}
	if rows, _ := k.Query("task_dispatch"); len(rows) != 0 {
		t.Fatalf("the loop left %d task_dispatch rows in the kernel", len(rows))
	}
}

// A dispatch that wrote nothing and cleared nothing stops the loop, with the
// gap named; the task is not sent a third time.
func TestRunUntilVerified_NoProgressStops(t *testing.T) {
	k := loopKernel(t)
	s := &scriptedDispatch{returns: []observation.Return{
		unverified("one", []string{"a.go"}, "/tests_not_green"),
		unverified("two", nil, "/tests_not_green"),
		{Output: "three", Outcome: "/done"},
	}}
	ret, err := RunUntilVerified(context.Background(), k, s.dispatch, TaskRequest{IntentVerb: "/fix", Task: "t"})
	var nv *NotVerifiedError
	if !errors.As(err, &nv) || !errors.Is(err, ErrNotVerified) {
		t.Fatalf("err = %v, want a NotVerifiedError", err)
	}
	if nv.Verdict != "/stalled" || nv.Dispatches != 2 || len(s.tasks) != 2 {
		t.Fatalf("verdict=%s dispatches=%d sent=%d, want /stalled after two", nv.Verdict, nv.Dispatches, len(s.tasks))
	}
	if !strings.Contains(err.Error(), "the tests were not verified green") {
		t.Fatalf("the stop does not name the gap: %v", err)
	}
	if ret.Output != "two" || len(ret.Changed) != 1 {
		t.Fatalf("the stop returns the merged return: %+v", ret)
	}
}

// A report that names an obstacle stops the loop at once: the person is told
// what it is, the task is not sent again.
func TestRunUntilVerified_ANamedBlockerStops(t *testing.T) {
	k := loopKernel(t)
	blocked := unverified("I need the deploy key.", nil, "/self_reported_incomplete")
	blocked.Report = &observation.ReportReading{Unfinished: 96, Blocked: 97}
	s := &scriptedDispatch{returns: []observation.Return{blocked}}
	_, err := RunUntilVerified(context.Background(), k, s.dispatch, TaskRequest{IntentVerb: "/fix", Task: "t"})
	var nv *NotVerifiedError
	if !errors.As(err, &nv) || nv.Verdict != "/blocked" || len(s.tasks) != 1 {
		t.Fatalf("err=%v sent=%d, want /blocked after one", err, len(s.tasks))
	}
}

// An executor that takes no verdict is not success, and has nothing to send
// back.
func TestRunUntilVerified_NoVerdictIsNotDone(t *testing.T) {
	k := loopKernel(t)
	s := &scriptedDispatch{returns: []observation.Return{{Output: "prose"}}}
	_, err := RunUntilVerified(context.Background(), k, s.dispatch, TaskRequest{IntentVerb: "/fix", Task: "t"})
	var nv *NotVerifiedError
	if !errors.As(err, &nv) || nv.Verdict != "/unjudged" || !strings.Contains(err.Error(), "no verdict") {
		t.Fatalf("err = %v, want /unjudged naming the missing verdict", err)
	}
}

// A dispatch error is the dispatch's, not a verdict; the caller's deadline
// ends a loop that is still moving.
func TestRunUntilVerified_ErrorsAndDeadlines(t *testing.T) {
	k := loopKernel(t)
	boom := errors.New("boom")
	s := &scriptedDispatch{returns: []observation.Return{unverified("one", []string{"a.go"}, "/tests_not_green"), {}}, errs: []error{nil, boom}}
	ret, err := RunUntilVerified(context.Background(), k, s.dispatch, TaskRequest{IntentVerb: "/fix", Task: "t"})
	if !errors.Is(err, boom) || len(ret.Changed) != 1 {
		t.Fatalf("ret=%+v err=%v, want the dispatch error with the writes so far", ret, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	moving := func(context.Context, TaskRequest) (observation.Return, error) {
		cancel()
		return unverified("moving", []string{"a.go"}, "/tests_not_green"), nil
	}
	if _, err := RunUntilVerified(ctx, k, moving, TaskRequest{IntentVerb: "/fix", Task: "t"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the caller's cancellation", err)
	}
}
