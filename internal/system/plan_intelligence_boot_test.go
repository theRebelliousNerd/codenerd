package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/types"
)

// The campaign intelligence rules (policy/intelligence.mg) judged copies the
// decomposer never seeded and nothing read their conclusions. They now read the
// world model the scan keeps (imports, churn, line counts, test pairing) joined
// to the plan's task artifacts, and the planner consumes them: a plan that edits
// a volatile, widely imported, untested file is sent back through the
// refinement channel (validation_error), the context compiler is told which
// plan files are dangerous (context_relevant), and the user sees the plan's
// risk, blast radius and per-file warnings (campaign.ReadPlanIntelligence).
// This boots the real cortex on a real repository.
func TestPlanIntelligenceReachesThePlannerOnTheShardedKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(ws, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/app\n\ngo 1.26\n")
	write("core/core.go", "package core\n\nfunc Do() {}\n")
	for _, pkg := range []string{"a", "b", "c"} {
		write(pkg+"/"+pkg+".go", fmt.Sprintf("package %s\n\nimport \"example.com/app/core\"\n\nfunc Run() { core.Do() }\n", pkg))
	}
	write("quiet/quiet.go", "package quiet\n\nfunc Hum() {}\n")
	write("quiet/quiet_test.go", "package quiet\n\nimport \"testing\"\n\nfunc TestHum(t *testing.T) { Hum() }\n")

	runGit(t, ws, nil, "init", "-q")
	runGit(t, ws, nil, "config", "user.name", "Me Tester")
	runGit(t, ws, nil, "config", "user.email", "me@example.test")
	runGit(t, ws, nil, "add", "-A")
	runGit(t, ws, nil, "commit", "-q", "-m", "initial")
	for i := 0; i < 17; i++ { // core.go becomes volatile: 18 commits touch it
		write("core/core.go", fmt.Sprintf("package core\n\nfunc Do() { _ = %d }\n", i))
		runGit(t, ws, nil, "add", "core/core.go")
		runGit(t, ws, nil, "commit", "-q", "-m", fmt.Sprintf("tweak %d", i))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	artifact := func(task, path string) types.Fact {
		return types.Fact{Predicate: "task_artifact", Args: []any{task, types.MangleAtom("/source"), path, "h"}}
	}
	if err := cortex.Kernel.AssertBatch([]types.Fact{
		artifact("t_core", "core/core.go"),
		artifact("t_quiet", "quiet/quiet.go"),
		// What the decomposer seeds from the gatherer: the knowledge graph's
		// edges for the plan's files and the standing strategy.
		{Predicate: "intelligence_knowledge_link", Args: []any{"core/core.go", "documented_by", "docs/core.md", int64(80)}},
		{Predicate: "intelligence_knowledge_link", Args: []any{"quiet/quiet.go", "documented_by", "docs/weak.md", int64(20)}},
		{Predicate: "campaign_phase", Args: []any{"p1", "camp1", "Phase", int64(0), types.MangleAtom("/pending"), "{}"}},
		{Predicate: "campaign_task", Args: []any{"t_core", "p1", "edit core", types.MangleAtom("/pending"), types.MangleAtom("/file_modify")}},
		{Predicate: "intelligence_strategic_knowledge", Args: []any{"goal", types.MangleAtom("/vision"), "keep core stable", int64(100)}},
		{Predicate: "intelligence_strategic_knowledge", Args: []any{"architecture", types.MangleAtom("/pattern"), "layered", int64(40)}},
		{Predicate: "intelligence_strategic_knowledge", Args: []any{"reusable_pattern", types.MangleAtom("/pattern"), "table tests", int64(70)}},
	}); err != nil {
		t.Fatalf("assert plan: %v", err)
	}

	// The refinement channel: only the volatile, imported, untested file.
	errs, err := cortex.Kernel.Query("validation_error")
	if err != nil {
		t.Fatal(err)
	}
	var coreIssue, quietIssue bool
	for _, f := range errs {
		if len(f.Args) < 3 || types.ExtractString(f.Args[1]) != "/needs_tests_first" {
			continue
		}
		if types.ExtractString(f.Args[0]) == "t_core" && strings.Contains(types.ExtractString(f.Args[2]), "core/core.go") {
			coreIssue = true
		}
		if types.ExtractString(f.Args[0]) == "t_quiet" {
			quietIssue = true
		}
	}
	if !coreIssue {
		t.Errorf("a task editing a volatile, widely imported, untested file must come back for a test task: %v", errs)
	}
	if quietIssue {
		t.Errorf("a quiet, tested file must not: %v", errs)
	}

	// What the user is shown.
	pi := campaign.ReadPlanIntelligence(cortex.Kernel)
	if pi.Risk != "/high" || len(pi.RiskReasons) == 0 {
		t.Errorf("risk = %q %v, want /high from the high-churn core file", pi.Risk, pi.RiskReasons)
	}
	if pi.Complexity != "/low" {
		t.Errorf("complexity = %q, want /low: the plan touches core and quiet and reaches three importers", pi.Complexity)
	}
	related := map[string]bool{}
	for _, r := range pi.Related {
		related[r.Path+">"+r.Entity] = true
	}
	if !related["core/core.go>docs/core.md"] || related["quiet/quiet.go>docs/weak.md"] {
		t.Errorf("the strong link of a plan file is shown, the weak one is not: %+v", pi.Related)
	}
	strategy := map[string]bool{}
	for _, st := range pi.Strategy {
		strategy[st.Content] = true
	}
	if !strategy["keep core stable"] || !strategy["table tests"] || strategy["layered"] {
		t.Errorf("the vision and a pattern above the priority bar apply, a weak pattern does not: %+v", pi.Strategy)
	}
	var impact bool
	for _, a := range pi.Advisories {
		if a.Kind == "/high_impact" && a.Path == "core/core.go" {
			impact = true
		}
		if a.Path == "quiet/quiet.go" {
			t.Errorf("no advisory for the quiet file: %+v", a)
		}
	}
	if !impact {
		t.Errorf("core.go is imported by three files and must carry a high-impact advisory: %+v", pi.Advisories)
	}

	// What the context compiler includes.
	relevant, err := cortex.Kernel.Query("context_relevant")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(relevant, "core/core.go", "/p85") {
		t.Errorf("a dangerous plan file must be relevant context: %v", relevant)
	}
	if factWith(relevant, "quiet/quiet.go", "/p85") {
		t.Errorf("a quiet plan file is not dangerous: %v", relevant)
	}
}
