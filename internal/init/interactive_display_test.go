package init

import (
	"os"
	"path/filepath"
	"testing"
)

// captureToFile runs fn with a temp *os.File writer and returns what was written.
func captureToFile(t *testing.T, fn func(w *os.File)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	fn(f)
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	return string(data)
}

func TestDefaultInteractiveConfig_ShouldUseStdio(t *testing.T) {
	cfg := DefaultInteractiveConfig()
	if cfg.Reader == nil {
		t.Error("DefaultInteractiveConfig should provide a non-nil Reader")
	}
	if cfg.Writer != os.Stdout {
		t.Error("DefaultInteractiveConfig should write to os.Stdout")
	}
	if cfg.SkipConfirmation {
		t.Error("DefaultInteractiveConfig should not skip confirmation by default")
	}
}

func TestDisplayNumberedList_ShouldMarkSelected(t *testing.T) {
	agents := []DetectedAgent{
		{Name: "go-expert", Selected: true},
		{Name: "rust-expert", Selected: false},
	}
	out := captureToFile(t, func(w *os.File) { displayNumberedList(agents, w) })
	if !containsSub(out, "1. [x] go-expert") {
		t.Errorf("selected agent should be numbered and marked [x]:\n%s", out)
	}
	if !containsSub(out, "2. [ ] rust-expert") {
		t.Errorf("unselected agent should be marked [ ]:\n%s", out)
	}
}

func TestDisplayAgentList_ShouldTagRecommendedAndReason(t *testing.T) {
	agents := []DetectedAgent{
		{Name: "go-expert", Recommended: true, DetectedBy: "go.mod", Reason: "project is Go"},
		{Name: "extra", Recommended: false, Reason: "maybe useful"},
	}
	out := captureToFile(t, func(w *os.File) { displayAgentList(agents, w) })
	if want := "[x] go-expert (recommended - go.mod)"; !containsSub(out, want) {
		t.Errorf("expected recommended tag %q in:\n%s", want, out)
	}
	if !containsSub(out, "project is Go") {
		t.Errorf("expected the agent reason to be printed:\n%s", out)
	}
	if !containsSub(out, "[ ] extra (optional)") {
		t.Errorf("optional agent should be marked [ ] (optional):\n%s", out)
	}
}

// containsSub reports whether needle occurs in haystack (a strings.Contains
// stand-in kept local to avoid an extra import in this small display test).
func containsSub(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
