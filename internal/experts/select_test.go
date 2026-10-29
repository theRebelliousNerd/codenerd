package experts

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

func rosterKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{AgentAtom("MangleExpert")}},
		{Predicate: "shard_agent", Args: []any{AgentAtom("BubbleTeaExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{AgentAtom("MangleExpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
		{Predicate: "shard_agent_domain", Args: []any{AgentAtom("BubbleTeaExpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_prefix", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		{Predicate: "dependency_link", Args: []any{"cmd/nerd/chat/view.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	return k
}

// Select reads the kernel's decision for a task and leaves nothing behind: the
// absolute target is canonicalized to the identity world facts carry, the
// import match routes the Bubble Tea expert to execute, and the task's facts
// are gone afterwards.
func TestSelectReadsTheKernelsDecisionAndRetracts(t *testing.T) {
	k := rosterKernel(t)
	ws := t.TempDir()

	sel, err := Select(k, Task{
		Verb:      "/fix",
		Targets:   []string{filepath.Join(ws, "cmd/nerd/chat/view.go"), "internal/core/defaults/policy/shards.mg"},
		Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got := sel.Agents(); !slices.Equal(got, []string{"bubbleteaexpert", "mangleexpert"}) {
		t.Fatalf("needed agents = %v, want bubbleteaexpert (90) then mangleexpert (40)", got)
	}
	tea := sel.Needs[0]
	if tea.Confidence != 90 || !tea.Execute || !slices.Equal(tea.Files, []string{"cmd/nerd/chat/view.go"}) ||
		!slices.Contains(tea.Technologies, "bubbletea") {
		t.Errorf("bubbletea need = %+v", tea)
	}
	mg := sel.Needs[1]
	if !tea.Critique {
		t.Errorf("a /fix (advise, execute, critique) owes the spawned expert's critique: %+v", tea)
	}
	if tea.Spawn != SpawnLead {
		t.Errorf("an executor routed to execute is consulted to lead; spawn = %q", tea.Spawn)
	}
	// A language match pushes knowledge and spawns no one.
	if mg.Confidence != 40 || mg.Execute || mg.Advise || mg.Spawn != "" || !slices.Equal(mg.Technologies, []string{"mangle"}) {
		t.Errorf("mangle need = %+v", mg)
	}

	for _, predicate := range []string{"expert_task", "expert_task_target", "expert_task_extension", "file_extension", "expert_needed", "specialist_match"} {
		rows, err := k.Query(predicate)
		if err != nil {
			t.Fatalf("query %s: %v", predicate, err)
		}
		if len(rows) != 0 {
			t.Errorf("%s still holds %d row(s) after Select: %v", predicate, len(rows), rows)
		}
	}
}

// A verb whose answer reads no code needs no expert, and a task with no
// target asks nothing.
func TestSelectNeedsAVerbThatReadsCodeAndATarget(t *testing.T) {
	k := rosterKernel(t)
	if sel, err := Select(k, Task{Verb: "/greet", Targets: []string{"a.mg"}}); err != nil || len(sel.Needs) != 0 {
		t.Errorf("greet selection = %+v, %v; want none", sel, err)
	}
	if sel, err := Select(k, Task{Verb: "/fix"}); err != nil || len(sel.Needs) != 0 {
		t.Errorf("targetless selection = %+v, %v; want none", sel, err)
	}
	if sel, err := Select(nil, Task{Verb: "/fix", Targets: []string{"a.mg"}}); err != nil || len(sel.Needs) != 0 {
		t.Errorf("nil-kernel selection = %+v, %v; want none", sel, err)
	}
}

// The kernel derives a task's complexity from what it measures, not from its
// wording: a complex verb, more than three files, or a file the world model
// says defines more than sixty symbols makes it /high, and a /high task
// requires strategic advice while a strategic advisor is classified.
func TestSelectDerivesComplexityFromTheTasksShape(t *testing.T) {
	k := rosterKernel(t)
	var symbols []types.Fact
	for n := range 61 {
		symbols = append(symbols, types.Fact{Predicate: "symbol_graph", Args: []any{
			fmt.Sprintf("sym_%d", n), types.MangleAtom("/function"), types.MangleAtom("/public"), "internal/big/surface.go", "func f()"}})
	}
	if err := k.AssertBatch(symbols); err != nil {
		t.Fatalf("assert symbols: %v", err)
	}
	for _, tc := range []struct {
		name    string
		task    Task
		want    string
		advisor bool
	}{
		{"a complex verb", Task{Verb: "/refactor", Targets: []string{"a.mg"}}, "high", true},
		{"one small file", Task{Verb: "/fix", Targets: []string{"a.mg"}}, "normal", false},
		{"four files", Task{Verb: "/fix", Targets: []string{"a.go", "b.go", "c.go", "d.go"}}, "high", true},
		{"a large file", Task{Verb: "/fix", Targets: []string{"internal/big/surface.go"}}, "high", true},
		// The wording no longer decides: "security" in a path is not a verb.
		{"a word in the target", Task{Verb: "/fix", Targets: []string{"security_architecture.go"}}, "normal", false},
	} {
		sel, err := Select(k, tc.task)
		if err != nil {
			t.Fatalf("%s: Select: %v", tc.name, err)
		}
		if sel.Complexity != tc.want || sel.StrategicAdvisorRequired != tc.advisor {
			t.Errorf("%s: complexity %q, advisor %v; want %q, %v", tc.name, sel.Complexity, sel.StrategicAdvisorRequired, tc.want, tc.advisor)
		}
	}
	if rows, _ := k.Query("task_complexity"); len(rows) != 0 {
		t.Errorf("task_complexity left behind: %v", rows)
	}
}
