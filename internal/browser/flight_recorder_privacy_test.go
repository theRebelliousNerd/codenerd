package browser

import (
	"os"
	"strings"
	"testing"

	browsersecurity "codenerd/internal/browser/security"
)

// Owner-only privacy is re-verified on every append, not only at creation.
// The evidence file is replaced by one created the ordinary way (0644 on Unix;
// an inherited, unprotected DACL on Windows), which is what a loosened or
// older-build trace looks like, and the next append must restore the policy.
func TestFlightRecorderReprotectsLoosenedEvidenceBeforeAppend(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	manager := NewSessionManagerWithSink(cfg, nil)
	if !manager.EvidenceEnabled() {
		t.Fatal("expected workspace flight recorder")
	}
	if _, err := manager.RecordEvidence("session-p", "tool", map[string]any{"step": 1}); err != nil {
		t.Fatalf("first RecordEvidence: %v", err)
	}
	path := manager.recorder.sessionPath("session-p")
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove evidence: %v", err)
	}
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatalf("recreate loosened evidence: %v", err)
	}
	if private, err := browsersecurity.IsPrivatePath(path, false); err != nil || private {
		t.Fatalf("precondition: recreated evidence should not be private, got private=%v err=%v", private, err)
	}

	if _, err := manager.RecordEvidence("session-p", "tool", map[string]any{"step": 2}); err != nil {
		t.Fatalf("second RecordEvidence: %v", err)
	}
	private, err := browsersecurity.IsPrivatePath(path, false)
	if err != nil || !private {
		t.Fatalf("appended evidence left without owner-only policy: private=%v err=%v", private, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if got := strings.Count(string(content), "\n"); got != 2 {
		t.Fatalf("expected both events in the trace, got %d lines: %s", got, content)
	}
}
