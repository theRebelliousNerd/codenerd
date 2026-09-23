package campaign

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/tactile"
)

func known(content string) observation.FileState {
	return observation.FileState{Known: true, Exists: true, Content: content}
}

var absent = observation.FileState{Known: true}

// Ladder C4: a failed attempt's writes are undone -- the ones outside its
// declared write set included -- when the path still holds what the attempt
// left, and only then.
func TestUndoAttemptWrites_PutsBackOnlyWhatTheAttemptStillOwns(t *testing.T) {
	dir := t.TempDir()
	path := func(name string) string { return filepath.Join(dir, name) }
	write := func(name, content string) {
		if err := os.WriteFile(path(name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("modified.go", "attempt, second turn")
	write("created.go", "new")
	write("sibling.go", "a sibling task's write")
	write("unread.go", "attempt")

	left, err := undoAttemptWrites([]observation.FileWrite{
		{Path: path("modified.go"), Before: known("original"), After: known("attempt, first turn")},
		{Path: path("created.go"), Before: absent, After: known("new")},
		{Path: path("sibling.go"), Before: known("original"), After: known("attempt")},
		{Path: path("unread.go"), Before: observation.FileState{}, After: known("attempt")},
		// A later turn of the same attempt wrote modified.go again: its After is
		// what the attempt left; the first turn's Before is what it found.
		{Path: path("modified.go"), Before: known("attempt, first turn"), After: known("attempt, second turn")},
	})
	if err != nil {
		t.Fatalf("undoAttemptWrites: %v", err)
	}

	read := func(name string) string {
		data, rerr := os.ReadFile(path(name))
		if rerr != nil {
			return "<" + rerr.Error() + ">"
		}
		return string(data)
	}
	if got := read("modified.go"); got != "original" {
		t.Errorf("modified.go = %q, want the content from before the attempt", got)
	}
	if _, serr := os.Stat(path("created.go")); !os.IsNotExist(serr) {
		t.Errorf("created.go survived the undo: the attempt created it")
	}
	if got := read("sibling.go"); got != "a sibling task's write" {
		t.Errorf("sibling.go = %q: a path changed since the attempt must be left alone", got)
	}
	if got := read("unread.go"); got != "attempt" {
		t.Errorf("unread.go = %q: a path with no known preimage must be left alone", got)
	}
	slices.Sort(left)
	if len(left) != 2 || !strings.Contains(left[0], "sibling.go (changed since") || !strings.Contains(left[1], "unread.go (what it held before") {
		t.Errorf("left = %v, want sibling.go and unread.go named with their reasons", left)
	}
}

// Through the scheduler: a create task whose first turn wrote its target and
// edited a file outside its write set to import it, and ended /unverified.
// The retry must find that file as it was -- not importing a file the
// rollback removed.
func TestRunPhase_AFailedAttemptsWriteOutsideItsWriteSetIsUndone(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	workspace := t.TempDir()
	const target = "internal/widget/widget.go"
	targetPath := filepath.Join(workspace, filepath.FromSlash(target))
	caller := filepath.Join(workspace, "cmd", "app", "main.go")
	const callerBefore = "package main\n\nfunc main() {}\n"
	const callerAfter = "package main\n\nimport _ \"example/internal/widget\"\n\nfunc main() {}\n"
	for _, dir := range []string{filepath.Dir(targetPath), filepath.Dir(caller)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(caller, []byte(callerBefore), 0o644); err != nil {
		t.Fatal(err)
	}

	var callerAtRetry string
	turns := &scriptedTurns{turn: func(n int, _ string) (observation.Return, error) {
		const widget = "package widget\n"
		if n == 2 {
			data, _ := os.ReadFile(caller)
			callerAtRetry = string(data)
		}
		if err := os.WriteFile(targetPath, []byte(widget), 0o644); err != nil {
			t.Error(err)
		}
		if n == 1 {
			if err := os.WriteFile(caller, []byte(callerAfter), 0o644); err != nil {
				t.Error(err)
			}
			return observation.Return{
				Output: "Wrote 2 file(s).", Outcome: "/unverified", Missing: []string{"/tests_not_written"},
				Writes: []observation.FileWrite{
					{Path: targetPath, Before: absent, After: known(widget)},
					{Path: caller, Before: known(callerBefore), After: known(callerAfter)},
				},
			}, nil
		}
		return observation.Return{Output: "done", Outcome: "/done"}, nil
	}}
	c := &Campaign{
		ID: "/campaign_undo", Type: CampaignTypeFeature, Title: "undo", Goal: "undo the attempt",
		Status: StatusActive, CreatedAt: time.Now().UTC(), TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_undo", CampaignID: "/campaign_undo", Name: "build", Status: PhaseInProgress,
			Category: "/implementation", EstimatedComplexity: "/low",
			Objectives: []PhaseObjective{{Type: ObjectiveCreate, Description: "the widget", VerificationMethod: VerifyNone}},
			Tasks: []Task{{
				ID: "/task_widget", PhaseID: "/phase_undo", Description: "create the widget",
				Status: TaskPending, Type: TaskTypeFileCreate, Priority: PriorityNormal,
				Artifacts: []TaskArtifact{{Type: "/source_file", Path: target}},
			}},
		}},
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace: workspace, Kernel: kernel, LLMClient: &MockLLMClient{}, TaskExecutor: turns,
		Executor: tactile.NewDirectExecutor(), VirtualStore: &core.VirtualStore{},
		Campaign: testCampaignConfig(fastRetries(3)),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.replanner = nil
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := orch.runPhase(ctx, &orch.campaign.Phases[0]); err != nil && ctx.Err() == nil {
		t.Logf("runPhase returned: %v", err)
	}

	if len(turns.calls()) != 2 {
		t.Fatalf("the coder ran %d time(s), want 2", len(turns.calls()))
	}
	if callerAtRetry != callerBefore {
		t.Fatalf("at the retry %s held the failed attempt's edit:\n%s\nwant it as it was before the attempt", caller, callerAtRetry)
	}
}
