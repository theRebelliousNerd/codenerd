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

// One run derived both a stale refresh of a subject and a prune of its old,
// never-retrieved row; the refresh installed the replacement under the same
// concept, and the prune then deleted by concept -- the replacement with it.
// The prune is now bound to the rows the run measured, so the replacement
// survives.
func TestKnowledgeRefreshDoesNotPruneTheReplacementItAcquired(t *testing.T) {
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
		"name": "WidgetExpert", "type": "persistent", "status": "ready",
		"domains": []string{}, "topics": []string{"widget calibration"},
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
	if _, err := refresher.run(ctx); err != nil || calls != 1 {
		t.Fatalf("first run: %v (%d acquisitions)", err, calls)
	}

	const concept = "web/widget calibration"
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
	withKB(func(db *store.LocalStore) {
		// The acquisition is stale, its row old and never retrieved, and the
		// agent is consulted often enough that the silence counts.
		old := time.Now().AddDate(0, 0, -100).UTC().Format(time.RFC3339)
		if _, err := db.GetDB().Exec(`UPDATE knowledge_acquisitions SET attempted_at = ?`, old); err != nil {
			t.Fatal(err)
		}
		if _, err := db.GetDB().Exec(`UPDATE knowledge_atoms SET created_at = datetime('now', '-90 days'), content = 'the old answer' WHERE concept = ?`, concept); err != nil {
			t.Fatal(err)
		}
		if err := db.StoreKnowledgeAtom("web/widget storage", "Store widgets flat.", 0.8, "web/probe"); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 20; i++ {
			if err := knowledge.RecordRetrieval(ctx, db.GetDB(), []string{"web/widget storage"}, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
	})

	report, err := refresher.run(ctx)
	if err != nil || calls != 2 || len(report.Refreshed) != 1 || !strings.Contains(report.Refreshed[0], "(stale)") {
		t.Fatalf("second run: %v (err %v, %d acquisitions)", report.Refreshed, err, calls)
	}
	withKB(func(db *store.LocalStore) {
		rows, err := db.GetKnowledgeAtomsByPrefix(concept)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || !strings.Contains(rows[0].Content, "calibrate against the reference table") {
			t.Fatalf("the refreshed replacement was pruned: %+v", rows)
		}
	})
}
