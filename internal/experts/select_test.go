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

func TestAgentKeyNormalizesIdentity(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "MangleExpert", "mangleexpert"},
		{"leading slash", "/MangleExpert", "mangleexpert"},
		{"surrounding spaces and slash", "  /GoExpert  ", "goexpert"},
		{"already key", "goexpert", "goexpert"},
		{"empty", "", ""},
		{"slash only", "/", ""},
		{"spaces only", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AgentKey(tc.input); got != tc.want {
				t.Errorf("AgentKey(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestAgentAtomCarriesSlashLowercased(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "MangleExpert", "/mangleexpert"},
		{"leading slash", "/GoExpert", "/goexpert"},
		{"spaces", "  BubbleTeaExpert ", "/bubbleteaexpert"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := types.ExtractString(AgentAtom(tc.input)); got != tc.want {
				t.Errorf("AgentAtom(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestValidNameAcceptsOnlyWellFormedConstants(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "/fix", true},
		{"nested", "/a/b", true},
		{"uppercase", "/A1", true},
		{"empty", "", false},
		{"no slash", "fix", false},
		{"slash only", "/", false},
		{"double slash", "//", false},
		{"empty segment", "/a//b", false},
		{"trailing slash", "/a/", false},
		{"space inside", "/a b", false},
		{"trailing space", "/fix ", false},
		{"leading space", " /fix", false},
		{"quote", "/a\"b", false},
		{"tab", "/a\tb", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validName(tc.input); got != tc.want {
				t.Errorf("validName(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestAppendUniqueDedupsAndIgnoresEmpty(t *testing.T) {
	if got := appendUnique(nil, ""); len(got) != 0 {
		t.Errorf("appendUnique(nil, empty) = %v, want empty", got)
	}
	if got := appendUnique(nil, "x"); !slices.Equal(got, []string{"x"}) {
		t.Errorf("appendUnique(nil, x) = %v, want [x]", got)
	}
	if got := appendUnique([]string{"a"}, "a"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("appendUnique([a], a) = %v, want [a]", got)
	}
	if got := appendUnique([]string{"a"}, "b"); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("appendUnique([a], b) = %v, want [a b]", got)
	}
	if got := appendUnique([]string{"a", "b"}, ""); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("appendUnique([a b], empty) = %v, want [a b]", got)
	}
}

func TestNumberConvertsKernelScalars(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  int64
	}{
		{"int64", int64(42), 42},
		{"int", int(7), 7},
		{"float truncates", float64(3.9), 3},
		{"string is zero", "42", 0},
		{"nil is zero", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := number(tc.input); got != tc.want {
				t.Errorf("number(%v) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestTaskFactsOwnEveryFactAndDedupsTargets(t *testing.T) {
	ws := t.TempDir()
	a := filepath.Join(ws, "a.go")
	b := filepath.Join(ws, "B.GO")
	task := Task{Verb: "/fix", Targets: []string{a, a, " " + a + " ", b}, Workspace: ws, Speaker: "GoExpert"}
	facts := taskFacts("expert_task_99", "/fix", task)
	for _, f := range facts {
		if len(f.Args) == 0 || types.ExtractString(f.Args[0]) != "expert_task_99" {
			t.Errorf("fact %s does not carry its task id: %+v", f.Predicate, f)
		}
		if f.Predicate == "file_extension" {
			t.Errorf("untagged file_extension escapes ownership: %+v", f)
		}
	}
	targets := 0
	extensions := 0
	speaker := false
	sawLowerExt := false
	for _, f := range facts {
		switch f.Predicate {
		case "expert_task_target":
			targets++
		case "expert_task_extension":
			extensions++
			if len(f.Args) >= 3 && types.ExtractString(f.Args[2]) == ".go" {
				sawLowerExt = true
			}
			if len(f.Args) >= 3 && types.ExtractString(f.Args[2]) == ".GO" {
				t.Errorf("extension not lower-cased: %+v", f)
			}
		case "expert_task_speaker":
			speaker = true
			if len(f.Args) < 2 || types.ExtractString(f.Args[1]) != "/goexpert" {
				t.Errorf("speaker atom = %+v, want /goexpert", f)
			}
		}
	}
	if targets != 2 || extensions != 2 {
		t.Errorf("targets = %d, extensions = %d; want 2 and 2 (deduped a.go, B.GO)", targets, extensions)
	}
	if !sawLowerExt {
		t.Errorf("no lower-cased .go extension among %v", facts)
	}
	if !speaker {
		t.Errorf("speaker fact missing among %v", facts)
	}
	if got := len(facts); got != 6 {
		t.Errorf("len(facts) = %d, want 6 (task + 2 targets + 2 extensions + speaker)", got)
	}

	empty := taskFacts("id2", "/fix", Task{Verb: "/fix", Targets: []string{"", ".", "   "}})
	if len(empty) != 1 {
		t.Errorf("empty targets yield %d facts, want 1 (only expert_task): %v", len(empty), empty)
	}
	noSpeaker := taskFacts("id3", "/fix", Task{Verb: "/fix", Targets: []string{"a.go"}, Speaker: "///"})
	for _, f := range noSpeaker {
		if f.Predicate == "expert_task_speaker" {
			t.Errorf("invalid speaker must not assert a fact: %+v", f)
		}
	}
}

func TestSelectRejectsInvalidVerbWithoutTouchingKernel(t *testing.T) {
	k := rosterKernel(t)
	cases := []struct {
		name string
		verb string
	}{
		{"empty", ""},
		{"bare word", "fix"},
		{"slash only", "/"},
		{"empty segment", "/a//b"},
		{"space inside", "/a b"},
		{"quote", "/a\"b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel, err := Select(k, Task{Verb: tc.verb, Targets: []string{"a.mg"}})
			if err != nil {
				t.Fatalf("Select(%q): %v", tc.verb, err)
			}
			if len(sel.Needs) != 0 {
				t.Errorf("Select(%q) needs = %+v, want none", tc.verb, sel.Needs)
			}
		})
	}
	for _, predicate := range []string{"expert_task", "expert_task_target", "expert_task_extension", "expert_task_speaker"} {
		rows, err := k.Query(predicate)
		if err != nil {
			t.Fatalf("query %s: %v", predicate, err)
		}
		if len(rows) != 0 {
			t.Errorf("%s holds %d row(s) after invalid verbs: %v", predicate, len(rows), rows)
		}
	}
}

func TestSelectTrimsSurroundingWhitespaceFromVerb(t *testing.T) {
	k := rosterKernel(t)
	sel, err := Select(k, Task{Verb: "  /review  ", Targets: []string{"internal/policy/rules.gl"}})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if !slices.Equal(sel.Agents(), []string{"mangleexpert"}) {
		t.Errorf("trimmed verb agents = %v, want [mangleexpert]", sel.Agents())
	}
}

func TestSelectionAgentsPreservesNeedOrder(t *testing.T) {
	sel := Selection{Needs: []Need{{Agent: "b"}, {Agent: "a"}}}
	if got := sel.Agents(); !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("Agents() = %v, want [b a] in Needs order", got)
	}
	if got := (Selection{}).Agents(); len(got) != 0 {
		t.Errorf("empty Agents() = %v, want empty", got)
	}
}

// An import of a language technology's host library from another language is
// that other language's code: a Go file building Mangle programs through
// mangle-go pushes the Mangle expert's knowledge and spawns no one, so no
// consultation precedes its step and no Mangle review follows its change.
// A framework import still routes its expert to execute.
func TestSelectAHostLibraryImportIsNotTheLanguagesSubject(t *testing.T) {
	k := rosterKernel(t)
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "technology_import_prefix", Args: []any{"codeberg.org/TauCeti/mangle-go", types.MangleAtom("/mangle")}},
		{Predicate: "dependency_link", Args: []any{"internal/core/kernel_query.go", "pkg:codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/ast"}},
	}); err != nil {
		t.Fatal(err)
	}
	sel, err := Select(k, Task{Verb: "/fix", Targets: []string{"internal/core/kernel_query.go"}, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(sel.Needs) != 1 {
		t.Fatalf("needs = %+v, want the Mangle expert's knowledge only", sel.Needs)
	}
	mg := sel.Needs[0]
	if experts := mg.Agent; AgentKey(experts) != "mangleexpert" || mg.Confidence != 40 || mg.Spawn != "" || mg.Critique {
		t.Errorf("a Go file importing mangle-go consults or reviews the Mangle expert: %+v", mg)
	}

	sel, err = Select(k, Task{Verb: "/fix", Targets: []string{"cmd/nerd/chat/view.go"}, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(sel.Needs) != 1 || sel.Needs[0].Confidence != 90 || sel.Needs[0].Spawn != SpawnLead {
		t.Errorf("a framework import no longer routes its expert to lead: %+v", sel.Needs)
	}
}
