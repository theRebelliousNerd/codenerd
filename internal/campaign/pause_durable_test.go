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
	caller := t.TempDir()
	t.Chdir(caller)
	if err := os.WriteFile(filepath.Join(root, "marker.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := os.Stat(filepath.Join(caller, ".nerd", "cache", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("pathless campaign scanned the caller instead of its configured workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".nerd", "cache", "manifest.json")); err != nil {
		t.Fatalf("configured workspace was not scanned by risk intelligence: %v", err)
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
