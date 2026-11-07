package campaign

import (
	"context"
	"testing"
)

type blockingRequirementLLM struct {
	MockLLMClient
	during func()
}

// campaign-intake-pause-ownership #1-#3: a Pause or Resume that arrives while
// a requirement is being integrated belongs to the user; the intake must not
// undo it when it finishes.
func TestHandleNewRequirement_LeavesAUsersPauseOrResumeStanding(t *testing.T) {
	cases := []struct {
		name       string
		startPause bool
		during     func(o *Orchestrator)
		wantPaused bool
	}{
		{"user pauses during intake stays paused", false, func(o *Orchestrator) { o.Pause() }, true},
		{"user resumes during intake stays resumed", true, func(o *Orchestrator) { o.Resume() }, false},
		{"untouched active campaign resumes", false, func(*Orchestrator) {}, false},
		{"untouched paused campaign stays paused", true, func(*Orchestrator) {}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orch := newSnapshotTestOrchestrator()
			orch.workspace = t.TempDir()
			orch.nerdDir = orch.workspace
			if tc.startPause {
				orch.Pause()
			}
			orch.replanner = NewReplanner(&MockKernel{}, &MockLLMClient{
				CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
					tc.during(orch)
					return `{"tasks": [], "summary": "ok"}`, nil
				},
			}, "")
			orch.replanner.SetPromptProvider(testPromptProvider{})
			_ = orch.HandleNewRequirement(context.Background(), "add a thing")
			orch.mu.RLock()
			paused, ch := orch.isPaused, orch.pauseCh
			orch.mu.RUnlock()
			if paused != tc.wantPaused {
				t.Fatalf("paused = %v, want %v", paused, tc.wantPaused)
			}
			// A paused campaign must hold an open channel the scheduler can
			// wait on; a resumed one a closed channel.
			select {
			case <-ch:
				if paused {
					t.Fatal("paused with a closed pause channel: the scheduler would not wait")
				}
			default:
				if !paused {
					t.Fatal("resumed with an open pause channel: the scheduler would wait forever")
				}
			}
			// A later Resume must not panic on a double close.
			orch.Resume()
		})
	}
}
