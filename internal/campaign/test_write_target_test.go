package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestWriteShardTaskTarget(t *testing.T) {
	newOrchestrator := func(t *testing.T) *Orchestrator {
		t.Helper()
		return &Orchestrator{workspace: t.TempDir()}
	}

	t.Run("directory WriteSet resolves to package label", func(t *testing.T) {
		o := newOrchestrator(t)
		if err := os.MkdirAll(filepath.Join(o.workspace, "pkg"), 0o755); err != nil {
			t.Fatalf("mkdir pkg: %v", err)
		}
		task := &Task{WriteSet: []string{"pkg"}}
		got, err := o.testWriteShardTask(task, o.resolveFileTaskTargetPath(task))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "generate_tests package:pkg ") {
			t.Fatalf("got %q, want prefix %q", got, "generate_tests package:pkg ")
		}
	})

	t.Run("artifact file resolves to file label", func(t *testing.T) {
		o := newOrchestrator(t)
		task := &Task{Artifacts: []TaskArtifact{{Path: "pkg/a.go"}}}
		got, err := o.testWriteShardTask(task, o.resolveFileTaskTargetPath(task))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "generate_tests file:pkg/a.go ") {
			t.Fatalf("got %q, want prefix %q", got, "generate_tests file:pkg/a.go ")
		}
	})

	t.Run("empty target has no dangling label", func(t *testing.T) {
		o := newOrchestrator(t)
		task := &Task{Description: "cover edge cases"}
		got, err := o.testWriteShardTask(task, o.resolveFileTaskTargetPath(task))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "generate_tests ") {
			t.Fatalf("got %q, want prefix %q", got, "generate_tests ")
		}
		if strings.Contains(got, "file:") || strings.Contains(got, "package:") {
			t.Fatalf("got %q, want no file: or package: label", got)
		}
	})
}
