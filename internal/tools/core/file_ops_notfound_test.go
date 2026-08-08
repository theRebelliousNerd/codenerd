package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The defect these guard (F-TOOL-2, observed live): a model guessed
// internal/northstar/campaign_observer.go when the file is observer.go.
// read_file returned a bare "no such file or directory", the model had nothing
// to correct against, and after five guessed paths in one batch the entire
// shard run aborted. A wrong filename should cost one turn, not the task.

func TestNotFoundWithSuggestions_NamesSimilarFile(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"observer.go", "guardian.go", "store.go"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("package x\n"), 0644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// The exact live mistake: campaign_observer.go vs observer.go.
	err := notFoundWithSuggestions(filepath.Join(dir, "campaign_observer.go"), "internal/northstar/campaign_observer.go")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()

	if !strings.Contains(msg, "observer.go") {
		t.Errorf("error does not suggest the similar file that exists:\n%s", msg)
	}
	if !strings.Contains(msg, "internal/northstar/campaign_observer.go") {
		t.Errorf("error does not echo the path that was requested:\n%s", msg)
	}
	if !strings.Contains(msg, "do not guess") {
		t.Errorf("error does not steer away from another guess:\n%s", msg)
	}
}

func TestNotFoundWithSuggestions_ListsDirectoryWhenNothingSimilar(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"alpha.go", "beta.go"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("package x\n"), 0644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	err := notFoundWithSuggestions(filepath.Join(dir, "zzz_unrelated.go"), "pkg/zzz_unrelated.go")
	msg := err.Error()

	if !strings.Contains(msg, "alpha.go") || !strings.Contains(msg, "beta.go") {
		t.Errorf("error does not list what the directory actually contains:\n%s", msg)
	}
}

// A missing DIRECTORY is a different mistake from a wrong filename, and saying
// so stops the model retrying filenames inside a path that cannot exist.
func TestNotFoundWithSuggestions_DistinguishesMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no", "such", "dir")

	err := notFoundWithSuggestions(filepath.Join(dir, "file.go"), "no/such/dir/file.go")
	msg := err.Error()

	if !strings.Contains(msg, "does not exist either") {
		t.Errorf("error does not distinguish a missing directory:\n%s", msg)
	}
	if !strings.Contains(msg, "glob") && !strings.Contains(msg, "list_files") {
		t.Errorf("error does not point at a discovery tool:\n%s", msg)
	}
}

// A huge directory must not dump hundreds of names into the context window.
func TestNotFoundWithSuggestions_TruncatesLargeDirectories(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 120; i++ {
		n := filepath.Join(dir, string(rune('a'+i%26))+string(rune('a'+i/26))+"_file.go")
		if err := os.WriteFile(n, []byte("package x\n"), 0644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	err := notFoundWithSuggestions(filepath.Join(dir, "totallyunrelatedname.go"), "pkg/totallyunrelatedname.go")
	msg := err.Error()

	if !strings.Contains(msg, "...") {
		t.Errorf("large directory listing was not truncated:\n%s", msg)
	}
	if len(msg) > 4000 {
		t.Errorf("error message is %d chars; it should stay compact", len(msg))
	}
}
