package chat

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// planLedgerModel is a kernel-backed model holding a stored two-step dream
// plan, with an executor that records every request it is sent.
func planLedgerModel(t *testing.T, mode config.WorkMode) (Model, *requestRecordingExecutor) {
	t.Helper()
	m, _ := journeyModel(t, mode)
	exec := &requestRecordingExecutor{}
	m.taskExecutor = exec
	m.dreamPlanManager = core.NewDreamPlanManager(m.kernel)
	plan := core.NewDreamPlan("plan-ledger", "what if the retry policy were documented?")
	plan.AddSubtask(core.DreamSubtask{ID: "s1", Order: 0, ShardType: "reviewer", Description: "review the retry policy", Status: core.SubtaskStatusPending})
	plan.AddSubtask(core.DreamSubtask{ID: "s2", Order: 1, ShardType: "researcher", Description: "summarize the retry policy", Status: core.SubtaskStatusPending})
	m.dreamPlanManager.StorePlan(plan)
	return m, exec
}

// runDispatch runs the step a dispatch command carries (the batch's last
// command; the first is the spinner) and returns the step's message.
func runDispatch(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no step was dispatched")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		return batch[len(batch)-1]()
	}
	return msg
}

func sentTasks(exec *requestRecordingExecutor) []string {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	var out []string
	for _, r := range exec.requests {
		out = append(out, r.Task)
	}
	return out
}

func planStatus(t *testing.T, m Model, id string) core.SubtaskStatus {
	t.Helper()
	plan := m.dreamPlanManager.GetCurrentPlan()
	if plan == nil {
		hist := m.dreamPlanManager.GetHistory()
		if len(hist) == 0 {
			t.Fatal("the plan is neither current nor archived")
		}
		plan = hist[len(hist)-1]
	}
	for _, s := range plan.Subtasks {
		if s.ID == id {
			return s.Status
		}
	}
	t.Fatalf("plan has no step %s", id)
	return ""
}

// An approved plan whose first step left no obligation behind ended the
// chain, and the done handler cleared the queue: the second approved step
// never ran and the banner counted one step. The plan now owns its steps
// until each settles, so the quiet first step continues to the second.
func TestDreamPlan_AQuietFirstStepContinuesTheApprovedPlan(t *testing.T) {
	m, exec := planLedgerModel(t, config.WorkModeYolo)
	m.textarea.SetValue("do it")
	next, cmd := m.handleSubmit()
	m = next.(Model)

	next, cmd = m.Update(runDispatch(t, cmd))
	m = next.(Model)
	if got := planStatus(t, m, "s1"); got != core.SubtaskStatusCompleted {
		t.Fatalf("the first step settled as %q, want completed", got)
	}
	next, _ = m.Update(runDispatch(t, cmd))
	m = next.(Model)

	if tasks := sentTasks(exec); len(tasks) != 2 || !strings.Contains(tasks[0], "review") || !strings.Contains(tasks[1], "summarize") {
		t.Fatalf("dispatched %q, want the two approved steps in order", tasks)
	}
	if banner := lastAssistant(m); !strings.Contains(banner, "2 step(s)") {
		t.Fatalf("the closing banner does not count the whole plan: %q", banner)
	}
	if got := planStatus(t, m, "s2"); got != core.SubtaskStatusCompleted {
		t.Fatalf("the second step settled as %q, want completed", got)
	}
	if m.dreamPlanManager.GetCurrentPlan() != nil {
		t.Fatal("the finished plan was not archived")
	}
}

// A pause appended its follow-up behind the plan's already-dispatched first
// step, so Enter re-ran that step instead of the one the pause showed. Enter
// now runs exactly the offered step, and the plan's next step follows it.
func TestDreamPlan_EnterRunsTheOfferedStep(t *testing.T) {
	m, exec := planLedgerModel(t, config.WorkModeExplicit)
	m.textarea.SetValue("do it")
	next, cmd := m.handleSubmit()
	m = next.(Model)
	if cmd == nil {
		t.Fatal("approval dispatched nothing")
	}
	if got := planStatus(t, m, "s1"); got != core.SubtaskStatusRunning {
		t.Fatalf("the dispatched step is %q, want running", got)
	}

	// The first step's result derives one follow-up.
	follow := continueMsg{subtaskID: "fix-1", description: "fix the findings in the retry policy", shardType: "coder", ran: Subtask{ID: "s1", Description: "review the retry policy", ShardType: "reviewer"}}
	next, _ = m.Update(follow)
	m = next.(Model)
	if m.offeredSubtask == nil || m.offeredSubtask.ID != "fix-1" {
		t.Fatalf("the pause offers %+v, want the follow-up", m.offeredSubtask)
	}

	m.textarea.SetValue("")
	next, cmd = m.handleSubmit()
	m = next.(Model)
	next, cmd = m.Update(cmd())
	m = next.(Model)
	stepMsg := runDispatch(t, cmd)
	if tasks := sentTasks(exec); len(tasks) != 1 || !strings.Contains(tasks[0], "fix the findings") {
		t.Fatalf("Enter dispatched %q, want the offered follow-up", tasks)
	}

	// The follow-up's chain ends: the plan's next step is offered.
	next, _ = m.Update(stepMsg)
	m = next.(Model)
	if m.offeredSubtask == nil || m.offeredSubtask.ID != "s2" {
		t.Fatalf("after the follow-up the pause offers %+v, want the plan's second step", m.offeredSubtask)
	}
	if got := planStatus(t, m, "s2"); got != core.SubtaskStatusPending {
		t.Fatalf("the offered plan step is %q, want pending until it runs", got)
	}
}
