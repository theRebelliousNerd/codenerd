package chat

import (
	"context"
	"testing"

	"codenerd/internal/core"
)

// The prompt corpus gates 326 of its 918 atom entries on a language -- 113
// Mangle, 72 Go, and every TDD, debugging and refactoring methodology file.
// matchSelector fails closed on purpose: an atom that declares a constraint the
// context has no value for does not match, so a Go atom never leaks into a
// Python session.
//
// Fail-closed is right, and it is also why the missing half of the wire cost so
// much. CompilationContext.Language was set in exactly one place in the whole
// repository -- the `nerd init` scan -- and the interactive turn built its
// context from SessionContext.ExtraContext, which nothing ever gave a language
// to. So a third of the corpus could not be selected in a normal turn, and the
// symptom is not an error: it is a prompt that is quietly missing the Go advice
// while the agent works on Go.
//
// The workspace already knew. The world scan derives a project_language fact
// and asserts it into the kernel as a whole-snapshot property. Nothing
// downstream had ever read it back.
func TestSessionContextCarriesTheProjectLanguage(t *testing.T) {
	m, _ := SetupLiveModel(t)
	if err := m.kernel.Assert(core.Fact{
		Predicate: "project_language",
		Args:      []any{core.MangleAtom("/go")},
	}); err != nil {
		t.Fatalf("assert project_language: %v", err)
	}

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}

	got := sessionCtx.ExtraContext["language"]
	if got != "/go" {
		t.Errorf("ExtraContext[\"language\"] = %q, want \"/go\".\n"+
			"Without it CompilationContext.Language stays empty and matchSelector "+
			"excludes every atom that declares a language -- a third of the corpus, "+
			"including the Go and Mangle methodology this session would be using.", got)
	}
}

// A kernel with no scan yet must not invent a language. Guessing one is worse
// than having none: it would select another language's atoms and spend budget
// on advice for the wrong ecosystem, and unlike the empty case nothing about
// the prompt would look wrong.
func TestSessionContextOmitsLanguageWhenTheKernelHasNone(t *testing.T) {
	m, _ := SetupLiveModel(t)
	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}
	if got, ok := sessionCtx.ExtraContext["language"]; ok && got != "" {
		t.Errorf("ExtraContext[\"language\"] = %q with no project_language fact asserted; "+
			"an invented language selects the wrong ecosystem's atoms", got)
	}
}
