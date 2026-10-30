package chat

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/session"
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

// Advancing an approved plan must not replace an unverified step's reason
// with the successor's summary, even when the successor is verified.
func TestDreamPlan_ClosingVerdictPreservesEarlierUnverifiedReason(t *testing.T) {
	cases := []struct {
		name    string
		outcome continuationOutcome
		summary string
	}{
		{name: "verified successor", outcome: continuationCompleted, summary: "The retry summary was verified."},
		{name: "unverified successor", outcome: continuationUnverified, summary: "The summary acceptance check did not run."},
		{name: "failed successor", outcome: continuationFailed, summary: "The summary check failed: provider unavailable."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := planLedgerModel(t, config.WorkModeYolo)
			m.workspace = t.TempDir()
			m.sessionID = "plan-unverified-reason"
			m.textarea.SetValue("do it")
			next, _ := m.handleSubmit()
			m = next.(Model)
			const reason = "Acceptance checks did not run: the retry integration fixture is unavailable."
			next, _ = m.Update(continuationDoneMsg{
				ran:       Subtask{ID: "s1", Description: "review the retry policy", ShardType: "reviewer"},
				stepCount: 1, outcome: continuationUnverified, summary: reason,
			})
			m = next.(Model)
			if got := planStatus(t, m, "s2"); got != core.SubtaskStatusRunning {
				t.Fatalf("the successor did not advance: status=%q", got)
			}
			next, cmd := m.Update(continuationDoneMsg{
				ran:       Subtask{ID: "s2", Description: "summarize the retry policy", ShardType: "researcher"},
				stepCount: 2, outcome: tc.outcome, summary: tc.summary,
			})
			m = next.(Model)
			banner := lastAssistant(m)
			if !strings.Contains(banner, reason) {
				t.Errorf("closing verdict lost the earlier unverified reason: %q", banner)
			}
			if !strings.Contains(banner, tc.summary) {
				t.Errorf("closing verdict lost the successor's outcome: %q", banner)
			}
			if strings.Contains(banner, "✅") {
				t.Errorf("an unverified plan must not claim verified completion: %q", banner)
			}
			saved := false
			var run func(tea.Cmd)
			run = func(c tea.Cmd) {
				if c == nil {
					return
				}
				switch msg := c().(type) {
				case tea.BatchMsg:
					for _, child := range msg {
						run(child)
					}
				case sessionStatePersistedMsg:
					if msg.err != nil {
						t.Fatalf("save closing verdict: %v", msg.err)
					}
					saved = true
				default:
					t.Fatalf("unexpected terminal command result %T", msg)
				}
			}
			run(cmd)
			if !saved {
				t.Fatal("closing verdict did not schedule a session save")
			}
			history, err := session.LoadSessionHistory(m.workspace, m.sessionID)
			if err != nil {
				t.Fatalf("load closing verdict before another user message: %v", err)
			}
			if len(history.Messages) == 0 || history.Messages[len(history.Messages)-1].Content != banner {
				t.Fatalf("saved history lost the closing verdict or its reason: %+v", history.Messages)
			}

			// A later failed request must not inherit this plan's missing evidence.
			next, _ = m.Update(continuationDoneMsg{stepCount: 1, outcome: continuationFailed, summary: "A separate request failed: provider unavailable."})
			m = next.(Model)
			if banner := lastAssistant(m); !strings.HasPrefix(banner, "❌") || strings.Contains(banner, reason) {
				t.Errorf("ending the plan retained a reason that leaked into a later failure: %q", banner)
			}

			// A later verified request must not inherit the plan's weaker verdict.
			next, _ = m.Update(continuationDoneMsg{stepCount: 1, outcome: continuationCompleted, summary: "A separate request was verified."})
			m = next.(Model)
			if banner := lastAssistant(m); !strings.HasPrefix(banner, "✅") || strings.Contains(banner, reason) {
				t.Errorf("the ended plan leaked its verification gap into a new request: %q", banner)
			}
		})
	}
}

// Starting a new request clears the prior chain's evidence before a later
// terminal outcome can render it. No endContinuation call precedes this reset.
func TestContinuationInit_ClearsEarlierUnverifiedReason(t *testing.T) {
	cases := []struct {
		name    string
		outcome continuationOutcome
		summary string
		prefix  string
	}{
		{name: "unverified new request", outcome: continuationUnverified, summary: "The new request's acceptance checks did not run.", prefix: "⚠️"},
		{name: "failed new request", outcome: continuationFailed, summary: "The new request failed: provider unavailable.", prefix: "❌"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTestModel()
			defer m.shutdownCancel()
			const oldReason = "The earlier plan could not load its retry integration fixture."
			m.chainUnverified = true
			m.chainUnverifiedNote = "\n\n" + oldReason
			next, _ := m.Update(continuationInitMsg{
				completedSurface: "The new request's first step ran.",
				totalSteps:       2,
				next:             continueMsg{subtaskID: "new-request-follow-up", description: "check the new request", shardType: "reviewer"},
			})
			m = next.(Model)
			if m.chainUnverifiedNote != "" {
				t.Errorf("initializing a new request retained an earlier reason: %q", m.chainUnverifiedNote)
			}
			if m.chainUnverified {
				t.Error("initializing a new request retained the earlier unverified verdict")
			}
			next, _ = m.Update(continuationDoneMsg{stepCount: 2, outcome: tc.outcome, summary: tc.summary})
			m = next.(Model)
			banner := lastAssistant(m)
			if !strings.HasPrefix(banner, tc.prefix) || !strings.Contains(banner, tc.summary) {
				t.Errorf("new request lost its own terminal outcome: %q", banner)
			}
			if strings.Contains(banner, oldReason) {
				t.Errorf("initializing a new request leaked an earlier verification gap into its verdict: %q", banner)
			}
		})
	}
}

// A verified step's summary is not a verification gap, and an empty or
// whitespace-only unverified summary must not add blank notes to the verdict.
func TestDreamPlan_AdvanceDoesNotRetainVerifiedOrBlankSummaries(t *testing.T) {
	cases := []struct {
		name    string
		outcome continuationOutcome
		summary string
	}{
		{name: "verified step", outcome: continuationCompleted, summary: "The retry policy review was verified."},
		{name: "empty unverified summary", outcome: continuationUnverified, summary: ""},
		{name: "whitespace unverified summary", outcome: continuationUnverified, summary: " \t\n "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := planLedgerModel(t, config.WorkModeYolo)
			m.textarea.SetValue("do it")
			next, _ := m.handleSubmit()
			m = next.(Model)
			next, _ = m.Update(continuationDoneMsg{
				ran:       Subtask{ID: "s1", Description: "review the retry policy", ShardType: "reviewer"},
				stepCount: 1, outcome: tc.outcome, summary: tc.summary,
			})
			m = next.(Model)
			if got := planStatus(t, m, "s2"); got != core.SubtaskStatusRunning {
				t.Fatalf("the successor did not advance: status=%q", got)
			}
			if m.chainUnverifiedNote != "" {
				t.Errorf("advancing retained a summary that is not a verification reason: %q", m.chainUnverifiedNote)
			}
			next, _ = m.Update(continuationDoneMsg{
				ran:       Subtask{ID: "s2", Description: "summarize the retry policy", ShardType: "researcher"},
				stepCount: 2, outcome: continuationUnverified, summary: "Current acceptance did not run.",
			})
			m = next.(Model)
			const want = "⚠️ 2 step(s) ran; the work is not verified.\n\nCurrent acceptance did not run."
			if banner := lastAssistant(m); banner != want {
				t.Errorf("closing verdict included a verified or blank prior note: got %q, want %q", banner, want)
			}
		})
	}
}
