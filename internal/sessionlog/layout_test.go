package sessionlog

import (
	"path/filepath"
	"testing"
)

func TestValidateIDRefusesEscapes(t *testing.T) {
	for _, id := range []string{"", " ", ".", "..", "../x", "a/b", `a\b`, "x/../y"} {
		if ValidateID(id) == nil {
			t.Errorf("ValidateID(%q) must fail", id)
		}
	}
	for _, id := range []string{"session-1793208939530632285", "sess_123", "a.b"} {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) = %v", id, err)
		}
	}
}

func TestHistoryPathIsInsideTheSessionFolder(t *testing.T) {
	got, err := HistoryPath("/w", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/w", ".nerd", "sessions", "session-1", "session.json"); got != want {
		t.Fatalf("HistoryPath = %q, want %q", got, want)
	}
}

func TestAgentFolder(t *testing.T) {
	cases := []struct{ role, id, want string }{
		{"coder", "", "coder"},
		{"coder", "ab12", "coder-ab12"},
		{"coder", "coder-ab12", "coder-ab12"},
		{"campaign-phase", "phase 1/x", "campaign-phase-phase-1-x"},
		{"Perception Firewall", "", "perception-firewall"},
	}
	for _, c := range cases {
		if got := AgentFolder(c.role, c.id); got != c.want {
			t.Errorf("AgentFolder(%q,%q) = %q, want %q", c.role, c.id, got, c.want)
		}
	}
	if got := SafeSegment("../.."); got == "" || got == ".." {
		t.Errorf("SafeSegment must not return a traversal, got %q", got)
	}
}
