package articulation

import (
	"fmt"
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

// Every edge is rendered. The old cap of 15 hid the rest behind "... and N
// more", and no tool lists this blackboard slice, so a model that stopped at
// fifteen edges would edit as if the hidden callers did not exist.
func TestTheDependencySectionRendersEveryEdge(t *testing.T) {
	deps := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		deps = append(deps, fmt.Sprintf("file-%02d.go imports pkg-%02d", i, i))
	}
	pa := &PromptAssembler{}
	got := pa.buildSessionContext(&PromptContext{SessionCtx: &types.SessionContext{
		DependencyContext: deps,
	}})

	for _, dep := range deps {
		if !strings.Contains(got, dep) {
			t.Errorf("dependency section lost %q", dep)
		}
	}
	if strings.Contains(got, "and 25 more") || strings.Contains(got, "... and") {
		t.Errorf("dependency section still hides edges:\n%s", got)
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
