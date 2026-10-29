package chat

import (
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/shards"
	"codenerd/internal/types"
)

// Chat delegation asks the kernel which shard agents a task needs and how each
// is routed. A Go matcher (CoreTechnologyPatterns) used to decide the first
// half from file-name and content substrings, on two chat paths only; the
// kernel decides both now, from the roster boot asserts and the task's files.

func assertExpertRoster(t *testing.T, m Model) {
	t.Helper()
	if err := m.kernel.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{experts.AgentAtom("BubbleTeaExpert")}},
		{Predicate: "shard_agent", Args: []any{experts.AgentAtom("MangleExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("BubbleTeaExpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("MangleExpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_root", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		{Predicate: "dependency_link", Args: []any{"cmd/nerd/chat/view.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
}

// A Bubble Tea import hands the task to the Bubble Tea expert (an executor in
// policy/shards.mg, matched above 80); a policy file only earns the Mangle
// expert an advisory seat (60); a /high task requires strategic advice; a
// /normal one does not.
func TestSpecialistPolicy_KernelDerivesWhoExecutesAndWhoAdvises(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	assertExpertRoster(t, m)
	files := []string{"cmd/nerd/chat/view.go", "internal/core/defaults/policy/shards.mg"}

	sel := m.selectShardExperts("/refactor", files, shards.TaskComplexityHigh)
	specs := specialistsFromSelection(sel)
	if len(specs) != 2 {
		t.Fatalf("specialists = %+v, want the Bubble Tea and Mangle experts", specs)
	}
	tea, mg := specs[0], specs[1]
	if tea.AgentName != "bubbleteaexpert" || !tea.ShouldExecute || tea.Score != 0.9 ||
		tea.Classification == nil || tea.Classification.ExecutionMode != shards.SpecialistModeExecutor {
		t.Errorf("Bubble Tea expert = %+v, want it executing at 0.9", tea)
	}
	if len(tea.Files) != 1 || tea.Files[0] != "cmd/nerd/chat/view.go" || tea.Reason != "domain: bubbletea" {
		t.Errorf("Bubble Tea expert files/reason = %v / %q", tea.Files, tea.Reason)
	}
	if mg.AgentName != "mangleexpert" || mg.ShouldExecute || mg.Score != 0.6 {
		t.Errorf("Mangle expert = %+v, want it advising at 0.6", mg)
	}
	if !sel.StrategicAdvisorRequired {
		t.Error("strategic_advisor_required was not derived for a /high task")
	}

	if sel := m.selectShardExperts("/refactor", files, shards.TaskComplexityNormal); sel.StrategicAdvisorRequired {
		t.Error("strategic_advisor_required derived for a /normal task")
	}
}

// Without a kernel there is no decision, so no specialist: the delegation
// takes the plain spawn rather than a Go guess.
func TestSpecialistPolicy_NoKernelNoSpecialist(t *testing.T) {
	m := NewTestModel() // kernel is nil
	if specs := specialistsFromSelection(m.selectShardExperts("/fix", []string{"a.mg"}, shards.TaskComplexityNormal)); len(specs) != 0 {
		t.Errorf("a kernel-less model chose specialists %+v", specs)
	}
}
