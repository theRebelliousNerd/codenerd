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

// docsSource stands in for web search: one section per subject, counted.
type docsSource struct{ calls *int }

func (docsSource) Name() string { return "web" }

func (d docsSource) Acquire(_ context.Context, s knowledge.Subject) ([]knowledge.Atom, error) {
	*d.calls++
	return []knowledge.Atom{{
		Concept:    "web/" + s.Key(),
		Content:    "Sourced guidance on " + s.Topic + ": calibrate against the reference table first.",
		Confidence: 0.8,
		Source:     "web/probe",
	}}, nil
}

const widgetExpertPromptsYAML = `- id: "widgetexpert/identity"
  category: "identity"
  subcategory: "widgetexpert"
  is_mandatory: true
  description: "Identity for WidgetExpert"
  content: |
    You are WidgetExpert.
`

// The background refresh is scheduled on the real boot path and does what
// policy/knowledge_refresh.mg derives over the agent's KB: a gap a
// consultation found first, then the topics it never learned, within the
// budget; nothing again while everything is fresh; a stale acquisition
// again; and an old acquired row a well-consulted agent never used, pruned.
func TestKnowledgeRefreshDoesWhatThePolicyDerives(t *testing.T) {
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
	topics := []string{"widget calibration", "widget testing", "widget storage", "widget shipping", "widget repair", "widget audit"}
	writeAgentsJSON(t, ws, []map[string]any{{
		"name": "WidgetExpert", "type": "persistent", "status": "ready",
		"domains": []string{}, "topics": topics,
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	refresher := cortex.knowledgeRefresher
	if refresher == nil {
		t.Fatal("boot did not schedule the knowledge refresh")
	}
	if schedule, err := refreshSchedule(cortex.Kernel); err != nil || schedule.Delay <= 0 || schedule.Interval <= 0 {
		t.Fatalf("knowledge_refresh_schedule = %+v, %v", schedule, err)
	}
	calls := 0
	refresher.acquirer = knowledge.Acquirer{Sources: []knowledge.Source{docsSource{calls: &calls}}}

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
		if err := knowledge.RecordMiss(ctx, db.GetDB(), "how do I recalibrate a bent widget", time.Now()); err != nil {
			t.Fatal(err)
		}
	})

	report, err := refresher.run(ctx)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if len(report.Refreshed) != 4 || !strings.Contains(report.Refreshed[0], "how do I recalibrate a bent widget (gap)") {
		t.Fatalf("first run refreshed %v, want the gap first and four subjects in all (the budget)", report.Refreshed)
	}
	report, err = refresher.run(ctx)
	if err != nil || len(report.Refreshed) != 3 {
		t.Fatalf("second run refreshed %v (err %v), want the three topics the budget left", report.Refreshed, err)
	}
	report, err = refresher.run(ctx)
	if err != nil || len(report.Refreshed) != 0 || calls != 7 {
		t.Fatalf("a run over fresh knowledge refreshed %v (err %v, %d calls in all)", report.Refreshed, err, calls)
	}

	// A web answer past its age is acquired again.
	withKB(func(db *store.LocalStore) {
		old := time.Now().AddDate(0, 0, -40).UTC().Format(time.RFC3339)
		if _, err := db.GetDB().Exec(`UPDATE knowledge_acquisitions SET attempted_at = ? WHERE subject = 'widget calibration'`, old); err != nil {
			t.Fatal(err)
		}
	})
	report, err = refresher.run(ctx)
	if err != nil || len(report.Refreshed) != 1 || !strings.Contains(report.Refreshed[0], "widget calibration (stale)") {
		t.Fatalf("stale run refreshed %v (err %v)", report.Refreshed, err)
	}

	// An old acquired row nobody retrieved, in an agent consulted often.
	withKB(func(db *store.LocalStore) {
		if err := db.StoreKnowledgeAtom("web/forgotten", "An answer no consultation ever used.", 0.8, "web/probe"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.GetDB().Exec(`UPDATE knowledge_atoms SET created_at = datetime('now', '-90 days') WHERE concept = 'web/forgotten'`); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 20; i++ {
			if err := knowledge.RecordRetrieval(ctx, db.GetDB(), []string{"web/widget testing"}, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
	})
	report, err = refresher.run(ctx)
	if err != nil || report.Pruned != 1 {
		t.Fatalf("prune run pruned %d (err %v), want the forgotten row", report.Pruned, err)
	}
	withKB(func(db *store.LocalStore) {
		if rows, _ := db.GetKnowledgeAtomsByPrefix("web/forgotten"); len(rows) != 0 {
			t.Fatalf("the pruned row is still there: %+v", rows)
		}
		if rows, _ := db.GetKnowledgeAtomsByPrefix("web/widget testing"); len(rows) != 1 {
			t.Fatalf("a used row was pruned: %+v", rows)
		}
	})
}
