package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/knowledge"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// A miss recorded for a question stood after a later compile delivered the
// KB's rows for the same question, and the refresh researched it as a gap.
// A recovery newer than the latest miss now retires it (the row stays as
// history), and a genuine miss after the recovery reopens it.
func TestKnowledgeRefreshRetiresARecoveredMissAndReopensANewerOne(t *testing.T) {
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
	writeAgentsJSON(t, ws, []map[string]any{{
		"name": "WidgetExpert", "type": "persistent", "status": "ready", "domains": []string{},
	}})
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

	const question = "how do I recalibrate a bent widget"
	kbPath := types.AgentKnowledgeDBPath(filepath.Join(ws, ".nerd"), "widgetexpert")
	withKB := func(do func(db *store.LocalStore)) {
		t.Helper()
		db, err := store.NewLocalStore(kbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		do(db)
	}
	now := time.Now()
	withKB(func(db *store.LocalStore) {
		// A row the KB holds makes the agent one the refresh measures.
		if err := db.StoreKnowledgeAtom("web/widget basics", "Widgets bend under load.", 0.8, "web/probe"); err != nil {
			t.Fatal(err)
		}
		if err := knowledge.RecordMiss(ctx, db.GetDB(), question, now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := knowledge.RecordRecovery(ctx, db.GetDB(), question, now); err != nil {
			t.Fatal(err)
		}
	})
	report, err := refresher.run(ctx)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if calls != 0 || len(report.Refreshed) != 0 {
		t.Fatalf("a recovered miss was researched as a gap: %v (%d acquisition(s))", report.Refreshed, calls)
	}

	withKB(func(db *store.LocalStore) {
		if err := knowledge.RecordMiss(ctx, db.GetDB(), question, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	})
	report, err = refresher.run(ctx)
	if err != nil || len(report.Refreshed) != 1 || !strings.Contains(report.Refreshed[0], question+" (gap)") {
		t.Fatalf("a miss newer than the recovery did not reopen the gap: %v (err %v)", report.Refreshed, err)
	}
}
