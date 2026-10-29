package chat

import (
	"context"
	"strings"
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

// Frameworks are the language dimension's mirror image. matchSelector skips the
// framework check entirely when the context names none, so all 42
// framework-gated atoms stay eligible in every session regardless of project --
// and, more to the point, a project that IS built on bubbletea gets nothing
// favouring the bubbletea atoms over django's. The dimension contributes
// nothing in either direction until something fills it.
//
// `nerd init` writes project_framework into .nerd/profile.mg and chat loads that
// file at boot, so the fact was already there to be read.
func TestSessionContextCarriesProjectFrameworks(t *testing.T) {
	m, _ := SetupLiveModel(t)

	for _, fw := range []string{"/cobra", "/bubbletea"} {
		if err := m.kernel.Assert(core.Fact{
			Predicate: "project_framework",
			Args:      []any{core.MangleAtom(fw)},
		}); err != nil {
			t.Fatalf("assert project_framework(%s): %v", fw, err)
		}
	}

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}

	// Sorted, so the prompt -- and the compilation cache key built from it --
	// is the same on every run rather than following kernel iteration order.
	got := sessionCtx.ExtraContext["frameworks"]
	if got != "/bubbletea,/cobra" {
		t.Errorf("ExtraContext[\"frameworks\"] = %q, want \"/bubbletea,/cobra\"", got)
	}
}

// project_framework facts reach ExtraContext["frameworks"] as a comma-joined list
// with no empty element, because prompt_assembler splits that value on commas.
func TestProjectFrameworksReachTheSessionContext(t *testing.T) {
	m, _ := SetupLiveModel(t)

	if err := m.kernel.Assert(core.Fact{
		Predicate: "project_framework",
		Args:      []any{core.MangleAtom("/cobra")},
	}); err != nil {
		t.Fatalf("assert project_framework: %v", err)
	}

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}
	if got := sessionCtx.ExtraContext["frameworks"]; got != "/cobra" {
		t.Fatalf("frameworks = %q, want \"/cobra\"", got)
	}

	// prompt_assembler splits this value on commas, so a single value must not be wrapped in anything the split would not undo.
	if strings.Contains(sessionCtx.ExtraContext["frameworks"], ",,") {
		t.Error("frameworks value has an empty element; prompt_assembler would produce an empty tag")
	}
}
