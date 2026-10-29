package projectdoc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPaths(t *testing.T) {
	critical := []string{"internal/core", "cmd/nerd/", "*.mg"}
	for p, want := range map[string]string{
		"internal/core/kernel.go":                  "internal/core",
		"internal/core":                            "internal/core",
		`C:\repo\Internal\Core\kernel.go`:          "internal/core",
		"/home/u/repo/internal/core/sub/x.go":      "internal/core",
		"cmd/nerd/main.go":                         "cmd/nerd/",
		"internal/core/defaults/policy/recurse.mg": "internal/core",
		"internal/world/rules.mg":                  "*.mg",
		"internal/corex/a.go":                      "",
		"internal/session/executor.go":             "",
		"docs/internal/core-notes.md":              "",
	} {
		got, ok := MatchPaths(critical, p)
		if got != want || ok != (want != "") {
			t.Errorf("MatchPaths(%q) = %q, %v; want %q", p, got, ok, want)
		}
	}
}

func TestCriticalPaths(t *testing.T) {
	ws := t.TempDir()
	if got, err := CriticalPaths(ws); err != nil || got != nil {
		t.Fatalf("no nerd.md declares nothing: %v, %v", got, err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(ws, "nerd.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("---\nschema: nerd/v1\ncritical:\n  - internal/core\n  - \"*.mg\"\n---\n")
	got, err := CriticalPaths(ws)
	if err != nil || len(got) != 2 || got[1] != "*.mg" {
		t.Fatalf("CriticalPaths = %v, %v", got, err)
	}
	write("---\nschema: nerd/v1\ncritical: [\n---\n")
	if _, err := CriticalPaths(ws); err == nil {
		t.Fatal("a nerd.md that does not parse is an error, not an empty safety list")
	}
}
