package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// A search that stops at its cap must say so.
//
// The defect, measured 2026-09-20 on a live run: asked to find every
// forwarding stub under Docs/architecture -- 189 of them -- the model ran
// grep with max_results raised to 100, got exactly 100 matches and no notice,
// and could not tell a full page from an exact count. Its only move for the
// rest was the same search again; the working policy saw two identical rounds,
// derived working_stop(/repeated_cycle), and ended the task after three tool
// calls. The cap was reported as a count, and the loop followed from that.

func capTestWorkspace(t *testing.T, files map[string]string) context.Context {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return tools.WithWorkspaceRoot(context.Background(), dir)
}

func TestGrepAnnouncesItsCap(t *testing.T) {
	files := make(map[string]string, 12)
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("docs/stub%02d.md", i)] = "# Redirect\n\nSee elsewhere.\n"
	}
	ctx := capTestWorkspace(t, files)

	t.Run("capped result carries the marker and a way out", func(t *testing.T) {
		out, err := executeGrep(ctx, map[string]any{
			"pattern":     "^# Redirect",
			"max_results": 5,
		})
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if !types.IsClamped(out) {
			t.Errorf("a grep that stopped at its cap must be marked as truncated; got:\n%s", out)
		}
		if !strings.Contains(out, "max_results") {
			t.Errorf("the notice must name the way out, so the model can act on it; got:\n%s", out)
		}
		if strings.Count(out, "stub") < 5 {
			t.Errorf("the matches themselves must still be returned; got:\n%s", out)
		}
	})

	t.Run("an uncapped result is not marked", func(t *testing.T) {
		out, err := executeGrep(ctx, map[string]any{
			"pattern":     "^# Redirect",
			"max_results": 100,
		})
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if types.IsClamped(out) {
			t.Errorf("12 matches under a cap of 100 is a complete answer and must not be marked; got:\n%s", out)
		}
	})

	t.Run("no matches is not a cap", func(t *testing.T) {
		out, err := executeGrep(ctx, map[string]any{
			"pattern":     "^# NothingLikeThis",
			"max_results": 5,
		})
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if types.IsClamped(out) {
			t.Errorf("an empty result must not claim to be truncated; got:\n%s", out)
		}
	})
}

func TestGlobAnnouncesItsCap(t *testing.T) {
	files := make(map[string]string, 12)
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("docs/f%02d.md", i)] = "x"
	}
	ctx := capTestWorkspace(t, files)

	out, err := executeGlob(ctx, map[string]any{
		"pattern":     "**/*.md",
		"max_results": 4,
	})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !types.IsClamped(out) {
		t.Errorf("a glob that stopped at its cap must be marked as truncated; got:\n%s", out)
	}

	full, err := executeGlob(ctx, map[string]any{
		"pattern":     "**/*.md",
		"max_results": 50,
	})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if types.IsClamped(full) {
		t.Errorf("12 files under a cap of 50 is complete and must not be marked; got:\n%s", full)
	}
}

// The notice has to be distinguishable from "and N more", which needs a total
// the capped caller does not have.
func TestCapReachedNoticeShape(t *testing.T) {
	notice := types.CapReachedNotice(100, "matches", "Raise max_results.")
	if !types.IsClamped(notice) {
		t.Error("CapReachedNotice must carry the shared truncation marker")
	}
	if !strings.Contains(notice, "100") || !strings.Contains(notice, "matches") {
		t.Errorf("the notice must name the cap and the unit: %q", notice)
	}
	if !strings.Contains(notice, "Raise max_results.") {
		t.Errorf("the remedy must survive into the notice: %q", notice)
	}
	if got := types.CapReachedNotice(0, "matches", "x"); got != "" {
		t.Errorf("nothing shown is not a cap: %q", got)
	}
}
