package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Nine call sites spelled campaign persistence as `_ = o.saveCampaign()`.
// saveCampaign logs its own cause, but the caller dropped the outcome, so
// nothing recorded WHICH checkpoint was lost and no operator-facing surface
// heard about it: the campaign kept running with completed phases and replans
// that existed only in memory, and a crash rolled it back to whichever snapshot
// happened to succeed.

func TestPersistCampaign_EmitsAnEventWhenTheSnapshotCannotBeWritten(t *testing.T) {
	dir := t.TempDir()

	// A regular file where the campaigns directory must go: MkdirAll fails, so
	// saveCampaign fails, on a path the orchestrator otherwise treats as fine.
	blocked := filepath.Join(dir, "campaigns")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	events := make(chan OrchestratorEvent, 4)
	o := &Orchestrator{
		workspace: dir,
		nerdDir:   dir,
		eventChan: events,
		campaign:  &Campaign{ID: "/campaign_persist_test"},
	}

	o.persistCampaign("phase completion")

	select {
	case ev := <-events:
		if ev.Type != EventSnapshotWriteFailed {
			t.Fatalf("expected %q, got %q", EventSnapshotWriteFailed, ev.Type)
		}
		if !strings.Contains(ev.Message, "phase completion") {
			t.Errorf("event should name the lost checkpoint, got: %q", ev.Message)
		}
	default:
		t.Fatal("a failed campaign snapshot produced no event; the operator is told nothing")
	}
}

func TestPersistCampaign_StaysSilentOnSuccess(t *testing.T) {
	dir := t.TempDir()
	events := make(chan OrchestratorEvent, 4)
	o := &Orchestrator{
		workspace: dir,
		nerdDir:   dir,
		eventChan: events,
		campaign:  &Campaign{ID: "/campaign_persist_ok"},
	}

	o.persistCampaign("autosave")

	select {
	case ev := <-events:
		t.Fatalf("a successful snapshot emitted %q", ev.Type)
	default:
	}
	if _, err := os.Stat(filepath.Join(dir, "campaigns", "/campaign_persist_ok.json")); err == nil {
		return // written under the id as-is
	}
}

// EventSnapshotWriteFailed must be in the closed set, or every UI drops it
// through a default branch and the fix above is decorative.
func TestSnapshotWriteFailedIsAKnownEventType(t *testing.T) {
	if !IsKnownOrchestratorEventType(EventSnapshotWriteFailed) {
		t.Fatal("EventSnapshotWriteFailed is not in orchestratorEventTypes")
	}
}
