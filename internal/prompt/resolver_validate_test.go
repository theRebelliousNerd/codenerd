package prompt

import (
	"context"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// ValidateDependencies fills every finding's own fields: the missing
// dependency, each disjoint cycle's path, and each conflicting pair once.
func TestValidateDependencies_ReportsCyclesAndConflicts(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"d"}},
		{ID: "d", DependsOn: []string{"c", "gone"}},
		{ID: "self", DependsOn: []string{"self"}},
		{ID: "terse", ConflictsWith: []string{"verbose", "absent"}},
		{ID: "verbose", ConflictsWith: []string{"terse"}},
	}
	var missing []string
	var cycles [][]string
	var conflicts [][2]string
	for _, e := range NewDependencyResolver().ValidateDependencies(atoms) {
		switch e.Type {
		case DependencyErrorMissing:
			missing = append(missing, e.AtomID+"->"+e.MissingDepID)
		case DependencyErrorCycle:
			if e.AtomID != e.CycleIDs[0] || e.CycleIDs[0] != e.CycleIDs[len(e.CycleIDs)-1] {
				t.Fatalf("cycle %v is not a closed path headed by %s", e.CycleIDs, e.AtomID)
			}
			if !strings.HasPrefix(e.Error(), "dependency cycle: ") {
				t.Fatalf("cycle renders as %q", e.Error())
			}
			cycles = append(cycles, e.CycleIDs)
		case DependencyErrorConflict:
			conflicts = append(conflicts, [2]string{e.AtomID, e.ConflictID})
			if e.Error() != "atom terse conflicts with verbose" {
				t.Fatalf("conflict renders as %q", e.Error())
			}
		}
	}
	if !slices.Equal(missing, []string{"d->gone"}) {
		t.Fatalf("missing = %v", missing)
	}
	if len(cycles) != 3 {
		t.Fatalf("want the a/b, c/d and self cycles, got %v", cycles)
	}
	for _, want := range []string{"a", "c", "self"} {
		if !slices.ContainsFunc(cycles, func(c []string) bool { return slices.Contains(c, want) }) {
			t.Fatalf("no cycle through %s in %v", want, cycles)
		}
	}
	if !slices.Equal(conflicts, [][2]string{{"terse", "verbose"}}) {
		t.Fatalf("conflicts = %v, want the pair once", conflicts)
	}
}

// A compile through the real kernel reports a dependency cycle among the
// selected atoms in its stats, where Resolve would otherwise break it at an
// arbitrary atom with only a log line.
func TestCompile_StatsNameTheSelectedSetsDependencyCycle(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	corpus := NewEmbeddedCorpus([]*PromptAtom{
		{ID: "identity/one", Category: CategoryIdentity, Content: "You are one.", IsMandatory: true, Importance: 100, DependsOn: []string{"identity/two"}},
		{ID: "identity/two", Category: CategoryIdentity, Content: "You are two.", IsMandatory: true, Importance: 90, DependsOn: []string{"identity/one"}},
	})
	compiler, err := NewJITPromptCompiler(WithKernel(&realKernelQuerier{k: k}), WithEmbeddedCorpus(corpus))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	result, err := compiler.Compile(context.Background(), NewCompilationContextWithBudget(16000))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	defects := result.Stats.DependencyDefects
	if len(defects) != 1 || !strings.Contains(defects[0], "identity/one") || !strings.Contains(defects[0], "identity/two") {
		t.Fatalf("stats dependency defects = %v, want the identity cycle", defects)
	}
	if got := result.Stats.ToLogFields()["dependency_defects"]; got == nil {
		t.Fatal("the defects do not reach the structured log fields")
	}
}

// The shipped corpus selects no cycle and no conflicting pair: a compile of
// the embedded atoms through the real kernel reports no defects.
func TestCompile_EmbeddedCorpusSelectsNoDependencyDefects(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := NewJITPromptCompiler(WithKernel(&realKernelQuerier{k: k}), WithEmbeddedCorpus(embedded))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	cc := NewCompilationContextWithBudget(192000)
	cc.ShardType = "/coder"
	cc.OperationalMode = "/active"
	cc.IntentVerb = "/fix"
	result, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if d := result.Stats.DependencyDefects; len(d) != 0 {
		t.Fatalf("the shipped corpus selected dependency defects: %v", d)
	}
}
