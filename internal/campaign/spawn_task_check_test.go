package campaign

import (
	"context"
	"testing"

	"codenerd/internal/session"
	"codenerd/internal/tools"
)

// The orchestrator hands the campaign's acceptance command to each task's
// turn on the context, so the turn can run the same judge the campaign runs
// after it (campaign 7b853890: two fix attempts, each re-run costing a whole
// acceptance round).
func TestSpawnTask_SetsCampaignCheckWhenDeclared(t *testing.T) {
	var got tools.CampaignCheck
	var ok bool
	o := &Orchestrator{
		workspace: t.TempDir(),
		campaign: &Campaign{
			ID:         "/campaign_check_test",
			Acceptance: &Acceptance{Command: []string{"go", "version"}},
		},
		taskExecutor: &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				got, ok = tools.CampaignCheckFrom(ctx)
				return "done", nil
			},
		},
	}
	task := &Task{ID: "/task_check_0", Description: "Fix the docs", Type: TaskTypeFileModify}
	if _, err := o.spawnTask(context.Background(), task, "/fix", task.Description); err != nil {
		t.Fatalf("spawnTask: %v", err)
	}
	if !ok {
		t.Fatal("spawnTask set no campaign check although the campaign declares one")
	}
	if got.CampaignID != "/campaign_check_test" || got.TaskID != "/task_check_0" {
		t.Fatalf("check = %+v, want the campaign and task IDs", got)
	}
	if len(got.Argv) != 2 || got.Argv[0] != "go" || got.Argv[1] != "version" {
		t.Fatalf("argv = %v, want the declared command", got.Argv)
	}
}

func TestSpawnTask_SetsNoCampaignCheckWithoutACommand(t *testing.T) {
	for name, acceptance := range map[string]*Acceptance{
		"no acceptance": nil,
		"empty command": {Command: nil},
		"blank binary":  {Command: []string{""}},
	} {
		t.Run(name, func(t *testing.T) {
			var ok bool
			o := &Orchestrator{
				workspace: t.TempDir(),
				campaign:  &Campaign{ID: "/campaign_no_check", Acceptance: acceptance},
				taskExecutor: &MockTaskExecutor{
					ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
						_, ok = tools.CampaignCheckFrom(ctx)
						return "done", nil
					},
				},
			}
			task := &Task{ID: "/task_no_check_0", Description: "Fix the docs", Type: TaskTypeFileModify}
			if _, err := o.spawnTask(context.Background(), task, "/fix", task.Description); err != nil {
				t.Fatalf("spawnTask: %v", err)
			}
			if ok {
				t.Fatal("spawnTask set a campaign check although the campaign declares no command")
			}
		})
	}
}
