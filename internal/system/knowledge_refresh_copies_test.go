package system

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"codenerd/internal/knowledge"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// A KB that holds one imported paragraph once per mirror path, plus rows
// imported from a worktree checkout that is gone, is reconciled by the
// production knowledge refresh: one row per content survives (the earliest
// whose source still stands), rows from a source that is gone are retired,
// and rows unique to a standing source are untouched.
func TestKnowledgeRefreshFoldsMirrorCopiesAndRetiresGoneSources(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "WidgetExpert")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(widgetExpertPromptsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".claude/agents/widget.md", ".grok/agents/widget.md", ".codex/agents/widget.md"} {
		full := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("Widgets are calibrated daily.\n\nOnly this mirror says this."), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeAgentsJSON(t, ws, []map[string]any{{
		"name": "WidgetExpert", "type": "persistent", "status": "ready",
		"domains": []string{}, "topics": []string{"widget calibration"},
	}})

	kbPath := types.AgentKnowledgeDBPath(filepath.Join(ws, ".nerd"), "widgetexpert")
	shared := "Widgets are calibrated daily."
	para := func(tool, path string, n int) string {
		return "ecosystem paragraph " + string(rune('0'+n)) + "/2 tool=" + tool + " path=" + path + " bytes=29"
	}
	seed := []struct{ concept, content string }{
		// A worktree checkout of the tree, imported first and since removed.
		{para("claude", ".claude/worktrees/feature-x/.claude/agents/widget.md", 1), shared},
		{para("claude", ".claude/worktrees/feature-x/.claude/agents/widget.md", 2), "A note only the worktree had."},
		{para("claude", ".claude/agents/widget.md", 1), shared},
		{para("grok", ".grok/agents/widget.md", 1), shared},
		{para("codex", ".codex/agents/widget.md", 1), shared},
		{para("codex", ".codex/agents/widget.md", 2), "Only this mirror says this."},
	}
	db, err := store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range seed {
		// Legacy rows: written before the import recorded a source.
		if err := db.StoreKnowledgeAtom(row.concept, row.content, 0.95, ""); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	refresher := cortex.knowledgeRefresher
	calls := 0
	refresher.acquirer = knowledge.Acquirer{Sources: []knowledge.Source{docsSource{calls: &calls}}}
	report, err := refresher.run(ctx)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if report.Pruned != 4 {
		t.Errorf("pruned %d rows, want 4 (two worktree rows, two mirror copies)", report.Pruned)
	}

	db, err = store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	atoms, err := db.GetKnowledgeAtomsByPrefix("ecosystem paragraph ")
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, a := range atoms {
		kept = append(kept, a.Concept)
	}
	sort.Strings(kept)
	want := []string{para("claude", ".claude/agents/widget.md", 1), para("codex", ".codex/agents/widget.md", 2)}
	sort.Strings(want)
	if len(kept) != len(want) || kept[0] != want[0] || kept[1] != want[1] {
		t.Fatalf("kept %q, want %q", kept, want)
	}
}
