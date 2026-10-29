package chat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/orient"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

func TestRunScanAppliesOrientationWhenWorldIsUnchanged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	chatOrientationGit(t, root, "init", "--quiet")
	chatOrientationGit(t, root, "config", "user.name", "Chat Orientation Test")
	chatOrientationGit(t, root, "config", "user.email", "chat-orientation@example.invalid")
	chatOrientationWrite(t, root, "go.mod", "module example.com/orient-chat\n\ngo 1.24\n")
	chatOrientationWrite(t, root, "main.go", "package main\n\nfunc main() {}\n")
	chatOrientationWrite(t, root, "README.md", "# Fixture\n\nInitial documentation.\n")
	chatOrientationGit(t, root, "add", "go.mod", "main.go", "README.md")
	chatOrientationGit(t, root, "commit", "--quiet", "-m", "initial")

	snapshot, err := orient.Measure(context.Background(), root, config.OrientConfig{}.WithDefaults(), nil)
	if err != nil {
		t.Fatalf("measure initial orientation: %v", err)
	}
	orientEngine, err := snapshot.Engine(context.Background())
	if err != nil {
		t.Fatalf("build initial orientation engine: %v", err)
	}
	if err := snapshot.Save(root, orientEngine); err != nil {
		_ = orientEngine.Close()
		t.Fatalf("save initial orientation: %v", err)
	}
	if err := orientEngine.Close(); err != nil {
		t.Fatalf("close initial orientation engine: %v", err)
	}
	oldHead := chatOrientationGit(t, root, "rev-parse", "HEAD")

	// Boot the kernel against the workspace as production does: boot loads the
	// saved orientation projection, which declares oriented_head and carries
	// the measured HEAD.
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	initialHeads, err := kernel.Query("oriented_head")
	if err != nil || len(initialHeads) != 1 || types.ExtractString(initialHeads[0].Args[0]) != oldHead {
		t.Fatalf("initial oriented_head facts=%v err=%v; want fixture HEAD %q", initialHeads, err, oldHead)
	}

	model := NewTestModel()
	model.kernel = kernel
	model.scanner = world.NewScanner()
	model.workspace = root
	initial := model.runScan(false)()
	initialMsg, ok := initial.(scanCompleteMsg)
	if !ok || initialMsg.err != nil || initialMsg.fileCount == 0 {
		t.Fatalf("initial production runScan result = %#v; want a successful full scan", initial)
	}
	globalBefore, err := kernel.Query("project_language")
	if err != nil || len(globalBefore) != 1 {
		t.Fatalf("initial project_language facts=%v err=%v; want one world snapshot global", globalBefore, err)
	}

	chatOrientationGit(t, root, "commit", "--quiet", "--allow-empty", "-m", "advance orientation without source changes")
	newHead := chatOrientationGit(t, root, "rev-parse", "HEAD")
	if oldHead == newHead {
		t.Fatal("fixture commit did not advance HEAD")
	}
	refreshed := model.runScan(false)()
	refreshedMsg, ok := refreshed.(scanCompleteMsg)
	if !ok || refreshedMsg.err != nil || refreshedMsg.factCount != 0 {
		t.Fatalf("head-only production runScan result = %#v; want a successful world-Unchanged result", refreshed)
	}

	heads, err := kernel.Query("oriented_head")
	if err != nil || len(heads) != 1 || types.ExtractString(heads[0].Args[0]) != newHead {
		t.Fatalf("oriented_head facts=%v err=%v; want only refreshed HEAD %q", heads, err, newHead)
	}
	globalAfter, err := kernel.Query("project_language")
	if err != nil || len(globalAfter) != 1 || globalAfter[0].String() != globalBefore[0].String() {
		t.Fatalf("world global changed during unchanged runScan: before=%v after=%v err=%v", globalBefore, globalAfter, err)
	}
}

func chatOrientationWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chatOrientationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
