package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDurablePauseSurvivesSnapshotSaveAndStopsRun(t *testing.T) {
	root := t.TempDir()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	o, err := NewOrchestrator(OrchestratorConfig{Workspace: root, Kernel: k, LLMClient: &MockLLMClient{}, TaskExecutor: &MockTaskExecutor{}, Executor: tactile.NewDirectExecutor(), VirtualStore: &core.VirtualStore{}, DisableTimeouts: true})
	if err != nil {
		t.Fatal(err)
	}
	o.campaign = &Campaign{ID: "/campaign_pause", Type: CampaignTypeCustom, Title: "Pause owner", Goal: "wait", Status: StatusActive, TotalTasks: 1, TotalPhases: 1, Phases: []Phase{{ID: "/phase_wait", Status: PhasePending, Tasks: []Task{{ID: "/task_wait", Type: TaskTypeResearch, Status: TaskPending}}}}}
	if err := RequestPause(root, o.campaign.ID); err != nil {
		t.Fatal(err)
	}
	o.mu.Lock()
	err = o.saveCampaign()
	o.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = o.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("owner did not observe pause: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".nerd", "campaigns", "campaign_pause.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Campaign
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Status != StatusPaused || o.isRunning {
		t.Fatalf("owner not joined and paused: %s", saved.Status)
	}
	if err := ClearPauseRequest(root, o.campaign.ID); err != nil {
		t.Fatal(err)
	}
	path, _ := pauseRequestPath(root, o.campaign.ID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("resume did not clear signal")
	}
}
func TestPauseRejectsEscapingIdentity(t *testing.T) {
	for _, id := range []string{"", "/../escape", "C:/outside", "//bad", ".."} {
		if RequestPause(t.TempDir(), id) == nil {
			t.Fatalf("accepted %q", id)
		}
	}
}
