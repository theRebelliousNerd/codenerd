package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCampaignDirectoryWalksUseRootMembership(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) string {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write(".nerd/config.json", `{"world":{"ignore_patterns":["pkg/ignored/"]}}`)
	kept := []string{"pkg/a.go", "pkg/.hidden/h.go", "pkg/vendor/v.go", "pkg/testdata/f.go"}
	for _, rel := range kept {
		write(rel, "original\n")
	}
	ignored := write("pkg/ignored/deep/skip.go", "ignored\n")
	o := newSnapshotTestOrchestrator()
	o.workspace = root
	task := &o.campaign.Phases[0].Tasks[0]
	task.Type = TaskTypeFileModify
	task.WriteSet = []string{"pkg"}
	briefing := o.writeSetBriefing(task)
	for _, rel := range kept {
		if !strings.Contains(briefing, rel) {
			t.Errorf("briefing missed member %s: %s", rel, briefing)
		}
	}
	if strings.Contains(briefing, "ignored") {
		t.Fatalf("briefing included ignored tree: %s", briefing)
	}
	snapshot, err := o.captureTaskExecutionSnapshot(task)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.fileMutations) != len(kept) {
		t.Fatalf("snapshot = %v, want only %v", snapshot.fileMutations, kept)
	}
	for _, rel := range kept {
		write(rel, "changed\n")
	}
	created := write("pkg/new.go", "new\n")
	ignoredCreated := write("pkg/ignored/new.go", "outside membership\n")
	if err := o.rollbackTaskExecutionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	for _, rel := range kept {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || string(got) != "original\n" {
			t.Errorf("rollback %s = %q, %v", rel, got, err)
		}
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("new member survived rollback: %v", err)
	}
	for path, want := range map[string]string{ignored: "ignored\n", ignoredCreated: "outside membership\n"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("excluded file %s changed: %q, %v", path, got, err)
		}
	}
}
