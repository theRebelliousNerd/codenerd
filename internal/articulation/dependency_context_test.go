package articulation

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// DependencyContext was written by two functions in cmd/nerd/chat and read by
// none. The producers, the field and their 10/30 caps all existed; the render
// did not, so "1-hop dependencies for target file(s)" — the field's own
// comment — never reached a prompt.
//
// That is the mirror of the defect on the other end of the same chain:
// ActiveFiles, which feeds those producers, had two consumers and no writer.
// Fixing one without the other just moves which half is dead.
func TestDependenciesOfFilesInFocusReachThePrompt(t *testing.T) {
	pa := &PromptAssembler{}
	got := pa.buildSessionContext(&PromptContext{SessionCtx: &types.SessionContext{
		DependencyContext: []string{
			"internal/session/executor.go imports codenerd/internal/types",
			"internal/session/executor.go imported by internal/system/factory.go",
		},
	}})

	if !strings.Contains(got, "DEPENDENCIES OF FILES IN FOCUS") {
		t.Fatalf("no dependency section in the assembled context:\n%s", got)
	}
	for _, want := range []string{
		"imports codenerd/internal/types",
		"imported by internal/system/factory.go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the dependency section lost %q:\n%s", want, got)
		}
	}
}

// The section is bounded, and it has to be: this is the one place on this
// branch where prompt CONTENT grows, and an unbounded list of edges from a
// session that touched fifty files would crowd out the sections that were
// already earning their tokens.
func TestTheDependencySectionIsBounded(t *testing.T) {
	deps := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		deps = append(deps, "a.go imports b")
	}
	pa := &PromptAssembler{}
	got := pa.buildSessionContext(&PromptContext{SessionCtx: &types.SessionContext{
		DependencyContext: deps,
	}})

	if n := strings.Count(got, "a.go imports b"); n > 15 {
		t.Errorf("the dependency section rendered %d entries, cap is 15", n)
	}
	// And it must SAY it elided, or a model reading fifteen edges will believe
	// those are all of them.
	if !strings.Contains(got, "and 25 more") {
		t.Errorf("the section was cut with no notice that anything is missing:\n%s", got)
	}
}

// Nothing in focus means no section at all — not an empty heading. A heading
// with nothing under it reads as "this file has no dependencies", which is a
// claim, and the wrong one.
func TestNoDependenciesMeansNoSection(t *testing.T) {
	pa := &PromptAssembler{}
	got := pa.buildSessionContext(&PromptContext{SessionCtx: &types.SessionContext{
		GitBranch: "main",
	}})
	if strings.Contains(got, "DEPENDENCIES OF FILES IN FOCUS") {
		t.Errorf("empty dependency list still produced a heading:\n%s", got)
	}
}
