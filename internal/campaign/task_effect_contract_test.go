package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type witnessExecutor struct {
	mockTactileExecutor
	run func()
}

func (e *witnessExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	if e.run != nil {
		e.run()
	}
	return e.res, e.err
}
func TestTestRunRequiresHostWitness(t *testing.T) {
	for _, name := range []string{"passed", "infra_failure", "failed_with_PASS_prose", "changed_snapshot", "explicit_shard"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "sample.go")
			if err := os.WriteFile(path, []byte("package sample"), 0600); err != nil {
				t.Fatal(err)
			}
			k, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			ex := &witnessExecutor{mockTactileExecutor: mockTactileExecutor{res: &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "ok sample"}}, run: func() { calls++ }}
			switch name {
			case "infra_failure":
				ex.res.Success = false
				ex.res.Error = "could not start"
			case "failed_with_PASS_prose":
				ex.res.ExitCode = 1
				ex.res.Stdout = "PASS: described but not demonstrated"
			case "changed_snapshot":
				ex.run = func() {
					calls++
					if err := os.WriteFile(path, []byte("package changed"), 0600); err != nil {
						t.Error(err)
					}
				}
			}
			cfg := core.DefaultVirtualStoreConfig()
			cfg.WorkingDir = root
			vs := core.NewVirtualStoreWithConfig(ex, cfg)
			defer vs.Close()
			vs.SetKernel(k)
			vs.DisableBootGuard()
			o := &Orchestrator{campaign: &Campaign{ID: "/campaign_witness"}, workspace: root, kernel: k, virtualStore: vs, taskExecutor: &MockTaskExecutor{ExecuteFunc: func(context.Context, session.TaskRequest) (string, error) {
				t.Error("model prose used as test evidence")
				return "PASS", nil
			}}}
			task := &Task{ID: "/test_witness", Type: TaskTypeTestRun, TestWitness: &TestExecutionWitness{Snapshot: "stale"}}
			var result any
			if name == "explicit_shard" {
				task.Shard = "tester"
				result, err = o.executeTask(context.Background(), task)
			} else {
				result, err = o.executeTestRunTask(context.Background(), task)
			}
			want := name == "passed" || name == "explicit_shard"
			if (err == nil) != want {
				t.Fatalf("result=%v err=%v want pass=%v", result, err, want)
			}
			if calls != 1 {
				t.Fatalf("host executor calls=%d", calls)
			}
			if want && (task.TestWitness == nil || task.TestWitness.Snapshot == "stale" || task.TestWitness.Command != "go test -count=1 ./...") {
				t.Fatalf("missing current witness: %+v", task.TestWitness)
			}
			if !want && task.TestWitness != nil {
				t.Fatal("failure retained witness")
			}
		})
	}
}
func TestTaskEffectsCannotDowngrade(t *testing.T) {
	for _, task := range []Task{{Type: TaskTypeResearch, PlannedType: TaskTypeFileModify}, {Type: TaskTypeResearch, PlannedType: TaskTypeTestRun}, {Type: TaskTypeDocument, Artifacts: []TaskArtifact{{Path: " "}}, WriteSet: []string{""}}} {
		if validateTaskEffect(&task) == nil {
			t.Fatalf("accepted invalid obligation: %+v", task)
		}
	}
	p := &Phase{Tasks: []Task{{ID: "repro", Type: TaskTypeTestRun}, {ID: "edit", Type: TaskTypeFileModify}, {ID: "check", Type: TaskTypeTestRun}}}
	orderVerificationAfterEdits(p)
	orderVerificationAfterEdits(p)
	if len(p.Tasks[0].DependsOn) != 0 || len(p.Tasks[2].DependsOn) != 1 || p.Tasks[2].DependsOn[0] != "edit" {
		t.Fatalf("incorrect ordering: %+v", p.Tasks)
	}
}
