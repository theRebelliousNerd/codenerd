package campaign

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

func policyOrchestrator(t *testing.T, workspace string) *Orchestrator {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	return &Orchestrator{
		workspace:    workspace,
		kernel:       kernel,
		eventChan:    make(chan OrchestratorEvent, 16),
		contextPager: NewContextPager(kernel, nil, 100),
		campaign: &Campaign{
			ID:     "/campaign_policy",
			Status: StatusActive,
		},
	}
}

// How many upcoming tasks the pager prefetches was the constant 3. It is the
// policy's now: the pager's measured use against its budget crosses 80 and 95
// percent, and the answer shrinks to 1 and then nothing.
func TestPrefetchLimit_ShrinksAsTheContextBudgetFills(t *testing.T) {
	o := policyOrchestrator(t, t.TempDir())
	for _, tc := range []struct {
		used, want int
	}{{10, 3}, {79, 3}, {80, 1}, {94, 1}, {95, 0}, {100, 0}} {
		o.contextPager.usedTokens = tc.used
		if got := o.prefetchLimit(); got != tc.want {
			t.Errorf("%d of 100 tokens used: prefetch %d, want %d", tc.used, got, tc.want)
		}
	}
}

func TestAnnounceMilestones_EmitsEachOnceAndHalfwayIsArithmetic(t *testing.T) {
	o := policyOrchestrator(t, t.TempDir())
	drain := func() []string {
		var got []string
		for {
			select {
			case ev := <-o.eventChan:
				if ev.Type == EventMilestoneReached {
					got = append(got, ev.Message)
				}
			default:
				return got
			}
		}
	}

	o.campaign.TotalPhases, o.campaign.CompletedPhases = 4, 1
	o.announceMilestones()
	if got := drain(); len(got) != 0 {
		t.Fatalf("one of four phases is not halfway: %v", got)
	}

	o.campaign.CompletedPhases = 2
	o.announceMilestones()
	if got := drain(); len(got) != 1 || got[0] != "halfway" {
		t.Fatalf("two of four phases is halfway, once: %v", got)
	}
	o.announceMilestones()
	if got := drain(); len(got) != 0 {
		t.Fatalf("a recorded milestone is not announced again: %v", got)
	}
}

// A completed task whose files carry measured violations is owed one fix task;
// the fix records its target, which retires the debt, and a fix that falls
// short is not re-spawned.
func TestAppendQualityRemediation_OwesOneFixPerMeasuredTask(t *testing.T) {
	ws := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(ws, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/app\n\ngo 1.26\n")
	write("svc/svc.go", "package svc\n\n// TODO: finish this\nfunc Serve() {}\n")
	write("clean/clean.go", "package clean\n\nfunc Add(a, b int) int { return a + b }\n")
	write("clean/clean_test.go", "package clean\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { _ = Add(1, 2) }\n")

	o := policyOrchestrator(t, ws)
	o.campaign.Phases = []Phase{{
		ID: "p1", CampaignID: o.campaign.ID, Name: "build", Order: 1, Status: PhaseInProgress,
		Tasks: []Task{
			{ID: "t_dirty", PhaseID: "p1", Description: "write svc", Status: TaskCompleted, Type: TaskTypeFileCreate,
				WriteSet: []string{"svc/svc.go"}, Artifacts: []TaskArtifact{{Type: "/source_file", Path: "svc/svc.go"}}},
			{ID: "t_clean", PhaseID: "p1", Description: "write clean", Status: TaskCompleted, Type: TaskTypeFileCreate,
				WriteSet: []string{"clean/clean.go"}, Artifacts: []TaskArtifact{{Type: "/source_file", Path: "clean/clean.go"}}},
			{ID: "t_research", PhaseID: "p1", Description: "read", Status: TaskCompleted, Type: TaskTypeResearch,
				Artifacts: []TaskArtifact{{Type: "/source_file", Path: "svc/svc.go"}}},
		},
	}}
	o.campaign.TotalPhases, o.campaign.TotalTasks = 1, 3
	if err := syncCampaignFacts(o.kernel, nil, o.campaign, "test"); err != nil {
		t.Fatal(err)
	}

	n, err := o.appendQualityRemediation(context.Background(), "p1")
	if err != nil {
		t.Fatalf("appendQualityRemediation: %v", err)
	}
	if n != 1 {
		t.Fatalf("appended %d fix tasks, want 1 (only t_dirty carries violations; research tasks are not judged)", n)
	}
	phase := o.campaign.Phases[0]
	fix := phase.taskByID("t_dirty_fix")
	if fix == nil || fix.Status != TaskPending || len(fix.DependsOn) != 1 || fix.DependsOn[0] != "t_dirty" {
		t.Fatalf("t_dirty_fix = %+v, want a pending task depending on t_dirty", fix)
	}
	if len(fix.WriteSet) != 1 || fix.WriteSet[0] != "svc/svc.go" {
		t.Errorf("the fix writes where the task wrote: %v", fix.WriteSet)
	}
	if phase.taskByID("t_clean_fix") != nil {
		t.Error("a task whose files measure clean is owed nothing")
	}

	// The debt is retired by the recorded target; a second pass owes nothing.
	n, err = o.appendQualityRemediation(context.Background(), "p1")
	if err != nil || n != 0 {
		t.Fatalf("second pass appended %d (err %v), want 0: the fix records its target and is not judged itself", n, err)
	}
	select {
	case ev := <-o.eventChan:
		if ev.Type != EventQualityRemediation {
			t.Errorf("event = %v, want %v", ev.Type, EventQualityRemediation)
		}
	default:
		t.Error("the remediation must be announced")
	}
}
