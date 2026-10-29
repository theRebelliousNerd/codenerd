package campaign

import "testing"

// Sweep finding F6: the orchestrator's handlers named the verb a task's turn
// runs, and every file, refactor, shard-spawn and generic task ran as /fix --
// a new .md deliverable as "/fix create file:...". The verb is the kernel's
// task_delegation: the task type's persona through the one persona table,
// except that a new document is created.
func TestTaskVerb_IsTheKernels(t *testing.T) {
	o := &Orchestrator{kernel: newAssertTestKernel(t), campaign: &Campaign{ID: "/campaign_verbs"}}
	for _, tc := range []struct {
		task Task
		want string
	}{
		{Task{ID: "t_doc", Type: TaskTypeFileCreate, WriteSet: []string{"docs/features/README.md"}}, "/create"},
		{Task{ID: "t_go", Type: TaskTypeFileCreate, WriteSet: []string{"internal/x/x.go"}}, "/fix"},
		{Task{ID: "t_mod_doc", Type: TaskTypeFileModify, WriteSet: []string{"README.md"}}, "/fix"},
		{Task{ID: "t_test", Type: TaskTypeTestWrite, WriteSet: []string{"internal/x/x_test.go"}}, "/test"},
		{Task{ID: "t_research", Type: TaskTypeResearch}, "/research"},
		{Task{ID: "t_verify", Type: TaskTypeVerify}, "/review"},
		{Task{ID: "t_refactor", Type: TaskTypeRefactor}, "/fix"},
		{Task{ID: "t_spawn", Type: TaskTypeShardSpawn}, "/fix"},
		{Task{ID: "t_untyped", Type: ""}, "/fix"},
	} {
		task := tc.task
		task.PhaseID, task.Description, task.Status = "p1", "do "+task.ID, TaskInProgress
		got, err := o.taskVerb(&task)
		if err != nil {
			t.Fatalf("taskVerb(%s): %v", task.ID, err)
		}
		if got != tc.want {
			t.Errorf("taskVerb(%s, %s %v) = %s, want %s", task.ID, task.Type, task.WriteSet, got, tc.want)
		}
	}
}

// No kernel, no verb: a handler does not pick one of its own.
func TestTaskVerb_NoKernelIsAnError(t *testing.T) {
	o := &Orchestrator{}
	if got, err := o.taskVerb(&Task{ID: "t1", Type: TaskTypeFileCreate}); err == nil {
		t.Fatalf("taskVerb with no kernel = %q, want an error", got)
	}
}
