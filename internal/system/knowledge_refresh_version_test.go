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

// A dependency upgrade makes the knowledge acquired at the old version stale
// (item #13): each run measures what go.mod pins now, and the policy
// researches the module again because no manifest pins the version it was
// learned at.
func TestKnowledgeRefreshResearchesAnUpgradedDependency(t *testing.T) {
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
	writeGoMod := func(version string) {
		t.Helper()
		mod := "module example.com/widgets\n\ngo 1.24\n\nrequire github.com/spf13/cobra " + version + "\n"
		if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte(mod), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeGoMod("v1.8.0")
	writeAgentsJSON(t, ws, []map[string]any{{
		"name": "WidgetExpert", "type": "persistent", "status": "ready", "domains": []string{"cobra"},
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
	calls := 0
	refresher.acquirer = knowledge.Acquirer{Sources: []knowledge.Source{docsSource{calls: &calls}}}

	report, err := refresher.run(ctx)
	if err != nil || len(report.Refreshed) != 1 || !strings.Contains(report.Refreshed[0], "github.com/spf13/cobra (unlearned)") {
		t.Fatalf("first run refreshed %v (err %v)", report.Refreshed, err)
	}
	if report, err = refresher.run(ctx); err != nil || len(report.Refreshed) != 0 {
		t.Fatalf("an unchanged pin refreshed %v (err %v)", report.Refreshed, err)
	}

	writeGoMod("v1.10.2")
	report, err = refresher.run(ctx)
	if err != nil || len(report.Refreshed) != 1 || !strings.Contains(report.Refreshed[0], "github.com/spf13/cobra (version_changed)") {
		t.Fatalf("after the upgrade the run refreshed %v (err %v)", report.Refreshed, err)
	}
	db, err := store.NewLocalStore(types.AgentKnowledgeDBPath(filepath.Join(ws, ".nerd"), "widgetexpert"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	outcomes, err := knowledge.Outcomes(ctx, db.GetDB())
	if err != nil || len(outcomes) != 1 || outcomes[0].Version != "v1.10.2" {
		t.Fatalf("outcomes after the upgrade = %+v (err %v), want the knowledge recorded at v1.10.2", outcomes, err)
	}
}
