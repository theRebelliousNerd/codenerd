package campaign_test

import (
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/types"
)

// The evidence policy on the production kernel: the domain shards the factory
// boots, where a rule fires only if every predicate it joins lives on one
// shard. task_evidence joins what the orchestrator measured
// (task_artifact_on_disk, task_brief_names, task_output_path) with the
// campaign's own rows (campaign_task, task_dependency, task_order,
// phase_dependency), config_param and write_class; verify_report_hollow joins
// the same. Package campaign's own tests run on a single-store RealKernel,
// which cannot show a split join (and cannot import internal/system).
func TestTaskEvidence_OnTheProductionKernel(t *testing.T) {
	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	policy, err := config.DefaultCampaignConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if err := ck.LoadFacts(config.ParamFacts(policy.Params())); err != nil {
		t.Fatal(err)
	}
	c := &campaign.Campaign{
		ID: "/campaign_cortex",
		Phases: []campaign.Phase{
			{ID: "/phase_a", Order: 0, Status: campaign.PhaseCompleted, Tasks: []campaign.Task{
				{ID: "/task_a1", PhaseID: "/phase_a", Type: campaign.TaskTypeResearch, Status: campaign.TaskCompleted, Order: 0, Description: "Audit"},
				{ID: "/task_a2", PhaseID: "/phase_a", Type: campaign.TaskTypeFileCreate, Status: campaign.TaskCompleted, Order: 1, Description: "Create the principles doc"},
			}},
			{ID: "/phase_b", Order: 1, Status: campaign.PhaseCompleted,
				Dependencies: []campaign.PhaseDependency{{DependsOnPhaseID: "/phase_a", Type: campaign.DepHard}},
				Tasks: []campaign.Task{
					{ID: "/task_b1", PhaseID: "/phase_b", Type: campaign.TaskTypeResearch, Status: campaign.TaskCompleted, Order: 0, Description: "Map"},
				}},
			{ID: "/phase_c", Order: 2, Status: campaign.PhaseInProgress,
				Dependencies: []campaign.PhaseDependency{{DependsOnPhaseID: "/phase_b", Type: campaign.DepHard}},
				Tasks: []campaign.Task{
					{ID: "/task_c0", PhaseID: "/phase_c", Type: campaign.TaskTypeDocument, Status: campaign.TaskCompleted, Order: 0, Description: "Write the report"},
					{ID: "/task_c1", PhaseID: "/phase_c", Type: campaign.TaskTypeDocument, Status: campaign.TaskPending, Order: 1, DependsOn: []string{"/task_c0"}, Description: "Write using a2.md"},
					{ID: "/task_cv", PhaseID: "/phase_c", Type: campaign.TaskTypeVerify, Status: campaign.TaskPending, Order: 2, DependsOn: []string{"/task_c0"}, Description: "Verify the report"},
				}},
		},
	}
	if err := ck.LoadFacts(c.ToFacts()); err != nil {
		t.Fatal(err)
	}
	onDisk := func(task, path, artType, ext string, bytes int64) core.Fact {
		return core.Fact{Predicate: "task_artifact_on_disk", Args: []any{task, path, types.MangleAtom(artType), ext, bytes}}
	}
	measured := []core.Fact{
		onDisk("/task_a1", ".nerd/campaigns/x/artifacts/a1.md", "/doc", ".md", 900),
		onDisk("/task_a2", "Docs/spec/a2.md", "/source_file", ".md", 900),
		onDisk("/task_b1", ".nerd/campaigns/x/artifacts/b1.md", "/doc", ".md", 900),
		onDisk("/task_c0", "docs/report.md", "/doc", ".md", 300),
		{Predicate: "task_brief_names", Args: []any{"/task_c1", "Docs/spec/a2.md"}},
		// The preload's measurements (policy/campaign_preload.mg): the code
		// the task writes, a package its brief names, the code its dependency
		// wrote, and an element its brief names.
		{Predicate: "code_outline", Args: []any{"pkg/widget/widget.go", types.MangleAtom("/file"), int64(3)}},
		{Predicate: "task_output_path", Args: []any{"/task_c1", "pkg/widget/widget.go"}},
		{Predicate: "code_outline", Args: []any{"pkg/big", types.MangleAtom("/package"), int64(900)}},
		{Predicate: "task_brief_names", Args: []any{"/task_c1", "pkg/big"}},
		onDisk("/task_c0", "pkg/widget/util.go", "/source_file", ".go", 120),
		{Predicate: "code_outline", Args: []any{"pkg/widget/util.go", types.MangleAtom("/file"), int64(1)}},
		{Predicate: "task_brief_element", Args: []any{"/task_c1", "pkg/widget.Widget.Render", int64(4), "a416de3a8d2e"}},
		// A document the brief names that no task produced
		// (brief_document), and one a task declares.
		{Predicate: "task_brief_file", Args: []any{"/task_c1", "Docs/standard.md", ".md", int64(700)}},
		{Predicate: "task_brief_file", Args: []any{"/task_c1", "Docs/spec/a2.md", ".md", int64(900)}},
		onDisk("/task_cv", "Docs/draft.md", "/doc", ".md", 500),
		// A far artifact that cites the task's target (artifact_cites).
		onDisk("/task_a1", ".nerd/campaigns/x/artifacts/a3.md", "/doc", ".md", 400),
		{Predicate: "artifact_cites", Args: []any{".nerd/campaigns/x/artifacts/a3.md", "pkg/widget/widget.go"}},
		{Predicate: "task_brief_file", Args: []any{"/task_c1", "Docs/draft.md", ".md", int64(500)}},
	}
	if err := ck.LoadFacts(measured); err != nil {
		t.Fatal(err)
	}

	rows, err := ck.Query("task_evidence")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rows {
		if types.ExtractString(f.Args[0]) == "/task_c1" {
			got[types.ExtractString(f.Args[1])] = types.ExtractString(f.Args[2])
		}
	}
	want := map[string]string{
		"docs/report.md":                    "/inline",
		"Docs/spec/a2.md":                   "/inline",
		".nerd/campaigns/x/artifacts/b1.md": "/digest",
		".nerd/campaigns/x/artifacts/a1.md": "/handle",
		".nerd/campaigns/x/artifacts/a3.md": "/inline",
		"Docs/standard.md":                  "/inline",
	}
	for path, mode := range want {
		if got[path] != mode {
			t.Errorf("task_evidence(/task_c1, %s) = %q on the Cortex, want %q (all: %v)", path, got[path], mode, got)
		}
	}
	// A document a task still at work declares is not a brief document: the
	// negation holds only where declared_artifact can derive.
	if mode, ok := got["Docs/draft.md"]; ok {
		t.Errorf("an unfinished task's declared document was handed over as %s on the Cortex", mode)
	}

	hollow, err := ck.Query("verify_report_hollow")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range hollow {
		if types.ExtractString(f.Args[0]) == "/task_cv" && types.ExtractString(f.Args[1]) == "docs/report.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("verify_report_hollow did not derive for /task_cv's 300-byte report on the Cortex: %v", hollow)
	}

	preload, err := ck.Query("task_preload")
	if err != nil {
		t.Fatal(err)
	}
	forms := map[string]string{}
	for _, f := range preload {
		if types.ExtractString(f.Args[0]) == "/task_c1" {
			forms[types.ExtractString(f.Args[1])] = types.ExtractString(f.Args[2])
		}
	}
	for target, form := range map[string]string{
		"pkg/widget/widget.go":     "/outline", // its own target
		"pkg/big":                  "/count",   // named, over preload_outline_max_rows
		"pkg/widget/util.go":       "/outline", // its dependency wrote it
		"pkg/widget.Widget.Render": "/element", // named element
	} {
		if forms[target] != form {
			t.Errorf("task_preload(/task_c1, %s) = %q on the Cortex, want %q (all: %v)", target, forms[target], form, forms)
		}
	}
}
