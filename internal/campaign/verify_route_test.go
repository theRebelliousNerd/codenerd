package campaign

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
)

// verifyRouteCampaign is one phase: the given writing tasks, completed, and a
// pending /verify task after them.
func verifyRouteCampaign(verifyDesc string, writes ...Task) *Campaign {
	tasks := append([]Task(nil), writes...)
	for i := range tasks {
		tasks[i].PhaseID = "/phase_vr"
		tasks[i].Status = TaskCompleted
		tasks[i].Priority = PriorityNormal
		tasks[i].Order = i
	}
	tasks = append(tasks, Task{
		ID: "/task_vr_verify", PhaseID: "/phase_vr", Type: TaskTypeVerify,
		Status: TaskPending, Priority: PriorityNormal, Order: len(tasks), Description: verifyDesc,
	})
	return &Campaign{
		ID: "/campaign_vr", Type: CampaignTypeFeature, Title: "verify route", Status: StatusActive,
		Phases: []Phase{{
			ID: "/phase_vr", CampaignID: "/campaign_vr", Name: "docs", Status: PhaseInProgress, Tasks: tasks,
		}},
	}
}

func TestPolicy_AVerifyTaskIsABuildOnlyWhereItsPhaseWroteCode(t *testing.T) {
	doc := Task{ID: "/task_vr_doc", Type: TaskTypeFileCreate, WriteSet: []string{"docs/x.md"}}
	code := Task{ID: "/task_vr_code", Type: TaskTypeFileModify, WriteSet: []string{"internal/x/x.go"}}
	dir := Task{ID: "/task_vr_dir", Type: TaskTypeFileModify, WriteSet: []string{"internal/mangle"}}
	for _, tc := range []struct {
		name   string
		writes []Task
		want   string
	}{
		{"a phase that wrote a document", []Task{doc}, "/review"},
		{"a phase that wrote Go", []Task{code}, "/build"},
		{"a phase that wrote both", []Task{doc, code}, "/build"},
		{"a phase whose writes name no kind of file", []Task{dir}, "/review"},
		{"a phase with no declared writes", nil, "/review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := verifyRouteCampaign("Verify the work", tc.writes...)
			o := &Orchestrator{kernel: realKernelFor(t, c, true), campaign: c}
			got, err := o.derivedFor("verify_task_route", "/task_vr_verify")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, []string{tc.want}) {
				t.Fatalf("verify_task_route = %v, want [%s]", got, tc.want)
			}
		})
	}
}

// Campaign 7b853890 task_1_4, "Verify shipped drafts use Go code only no
// fabricated identifiers no bare filenames", matched none of the 25 keywords
// the Go router looked for, so a Markdown phase was "verified" by `go build
// ./...` in 14 seconds. Its phase wrote documents: it is a review.
func TestExecuteVerifyTask_AMarkdownPhaseIsReviewedNeverBuilt(t *testing.T) {
	c := verifyRouteCampaign("Verify x.md declares status",
		Task{ID: "/task_vr_doc", Type: TaskTypeFileCreate, WriteSet: []string{"docs/x.md"}})

	var mu sync.Mutex
	var commands []string
	exec := &mockExecutor{executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		mu.Lock()
		commands = append(commands, cmd.Binary+" "+strings.Join(cmd.Arguments, " "))
		mu.Unlock()
		return &tactile.ExecutionResult{Success: true, ExitCode: 0}, nil
	}}
	var intents []string
	te := &MockTaskExecutor{ExecuteObservedFunc: func(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
		mu.Lock()
		intents = append(intents, req.IntentVerb)
		mu.Unlock()
		return observation.Return{Output: "x.md declares its status in the header table.", Outcome: "/done"}, nil
	}}
	o := &Orchestrator{
		kernel: realKernelFor(t, c, true), campaign: c, workspace: t.TempDir(),
		executor: exec, taskExecutor: te, policy: testPolicy(nil),
	}

	task := &c.Phases[0].Tasks[1]
	if _, err := o.executeVerifyTask(context.Background(), task); err != nil {
		t.Fatalf("executeVerifyTask: %v", err)
	}
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, "go build") {
			t.Fatalf("a /verify task in a phase that wrote only Markdown ran %q", cmd)
		}
	}
	// The review runs as the reviewer: the kernel's task_delegation for a
	// /verify task (task_type_persona(/verify, /reviewer)).
	if !slices.Contains(intents, "/review") {
		t.Fatalf("the review path did not run: spawned intents %v", intents)
	}
}

// Where the phase wrote Go the build is the evidence, and only exit 0 passes:
// the executor reports a failed command as a non-zero exit, not an error.
func TestExecuteVerifyTask_ACodePhaseIsBuiltAndAFailedBuildFails(t *testing.T) {
	c := verifyRouteCampaign("Verify the change",
		Task{ID: "/task_vr_code", Type: TaskTypeFileModify, WriteSet: []string{"internal/x/x.go"}})
	exit := 0
	var built bool
	exec := &mockExecutor{executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		if cmd.Binary == "go" && len(cmd.Arguments) > 0 && cmd.Arguments[0] == "build" {
			built = true
		}
		return &tactile.ExecutionResult{Success: true, ExitCode: exit, Stdout: "x.go:1: undefined: y"}, nil
	}}
	o := &Orchestrator{
		kernel: realKernelFor(t, c, true), campaign: c, workspace: t.TempDir(),
		executor: exec, policy: testPolicy(nil),
	}
	task := &c.Phases[0].Tasks[1]

	if _, err := o.executeVerifyTask(context.Background(), task); err != nil || !built {
		t.Fatalf("a green build: err = %v, built = %t; want nil and a build", err, built)
	}
	exit = 2
	if _, err := o.executeVerifyTask(context.Background(), task); err == nil {
		t.Fatal("a build that exited 2 verified the task")
	}
}

// A kernel that derives no route is an error, never a build by default.
func TestExecuteVerifyTask_NoDerivedRouteIsAnError(t *testing.T) {
	c := verifyRouteCampaign("Verify the work")
	o := &Orchestrator{kernel: &MockKernel{}, campaign: c, workspace: t.TempDir(), executor: &mockExecutor{}, policy: testPolicy(nil)}
	if _, err := o.executeVerifyTask(context.Background(), &c.Phases[0].Tasks[0]); err == nil || !strings.Contains(err.Error(), "verify_task_route") {
		t.Fatalf("err = %v, want one naming the missing verify_task_route", err)
	}
}

var _ core.Kernel = (*MockKernel)(nil)
