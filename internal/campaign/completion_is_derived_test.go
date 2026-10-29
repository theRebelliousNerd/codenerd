package campaign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// A campaign's completion is derived: campaign_phases_done and
// all_phase_tasks_complete (policy/campaign_core.mg), from the campaign's own
// rows. Until 2026-09-23 two in-memory scans decided it beside those rules
// (sweep finding F3), so the kernel and the orchestrator could disagree about
// whether a phase was done. No Go function in this package may decide it again.
func TestCompletionIsNotDecidedInGo(t *testing.T) {
	forbidden := map[string]string{
		"isPhaseComplete":    "all_phase_tasks_complete",
		"isCampaignComplete": "campaign_phases_done",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if rule, bad := forbidden[fn.Name.Name]; bad {
				t.Errorf("%s declares %s; completion is the kernel's (%s)", fset.Position(fn.Pos()), fn.Name.Name, rule)
			}
		}
	}
}

// A kernel that holds the campaign but not all of its rows is missing state:
// asked as it stands, it would call a campaign with a pending phase done (no
// phase row to be incomplete) and a phase with a pending task done (no task
// row to be incomplete). The orchestrator reloads the campaign's facts before
// it trusts either answer.
func TestCompletion_AKernelMissingRowsIsReloadedNotBelieved(t *testing.T) {
	c := &Campaign{ID: "/campaign_missing", Title: "missing", Status: StatusActive, Phases: []Phase{{
		ID: "/phase_missing", CampaignID: "/campaign_missing", Name: "p", Status: PhaseInProgress,
		Tasks: []Task{
			{ID: "/task_missing_0", PhaseID: "/phase_missing", Description: "t", Status: TaskCompleted, Type: TaskTypeFileModify},
			{ID: "/task_missing_1", PhaseID: "/phase_missing", Description: "t", Status: TaskPending, Type: TaskTypeFileModify},
		},
	}}}

	// Only the campaign row: the phase is unknown to the kernel.
	orch := completionOrchestrator(t, &Campaign{ID: c.ID, Title: c.Title, Status: StatusActive})
	orch.campaign = c
	if orch.campaignPhasesDone() {
		t.Fatal("a campaign with a pending phase the kernel did not hold was declared done")
	}

	// The phase and its completed task, but not its pending task.
	partial := *c
	partial.Phases = []Phase{c.Phases[0]}
	partial.Phases[0].Tasks = c.Phases[0].Tasks[:1]
	orch = completionOrchestrator(t, &partial)
	orch.campaign = c
	if orch.phaseTasksDone(&c.Phases[0]) {
		t.Fatal("a phase with a pending task the kernel did not hold was declared done")
	}
}
