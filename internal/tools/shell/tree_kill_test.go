package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/tools"
)

func TestBashTool_TimeoutKillsTheWholeTree(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "alive")

	ctx := tools.WithWorkspaceRoot(context.Background(), dir)

	tool := BashTool()

	script := `sh -c "exec sh -c 'sleep 4; echo alive > ` + filepath.ToSlash(marker) + `'"`

	start := time.Now()
	_, _ = tool.Execute(ctx, map[string]any{
		"script":          script,
		"timeout_seconds": 1,
		"working_dir":     dir,
	})
	elapsed := time.Since(start)
	if elapsed >= 4*time.Second {
		t.Fatalf("bash tool call took %v, want < 4s", elapsed)
	}

	time.Sleep(5 * time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("marker file %q exists: grandchild survived timeout", marker)
		}
		t.Fatalf("stat marker file: %v", err)
	}
	// The inner exec is the MSYS shape taskkill /T cannot reach.
	// Before this change on Windows the marker appears.
}
