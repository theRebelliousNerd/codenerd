package tools_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// TestEffects_WhenToolRegistered_ShouldDeclareAResolvableEffect closes the gap
// between "a tool registered" and "a tool can run".
//
// executeToolCall calls tools.LookupEffect before it dispatches, and refuses
// the call when no effect resolves: the executive gate will not run what it
// cannot classify. That is correct and deliberately fail-closed. What is not
// correct is how quietly it fails. Registry.Register validates Name and
// Execute, never Effect, so a tool with no effect registers cleanly, appears
// in the catalog, is offered to the model, is accepted by the JIT allowlist —
// and then returns an error from a spot two thousand lines away, with the
// registered Execute closure never invoked.
//
// That is not hypothetical. Nine tools across three e2e files were registered
// without an effect and could never execute. The tests asserted on counters
// the tool bodies incremented, saw zero, and reported "execution dropped" and
// "race corruption" — plausible-sounding diagnoses of a bug that did not
// exist. One blocked forever on a channel its tool body was supposed to close
// and timed out the whole package, hiding every test after it. The registered
// half and the dispatching half were each individually correct.
//
// This test asserts the property directly, for every tool the production
// registrars install: registered implies runnable.
func TestEffects_WhenToolRegistered_ShouldDeclareAResolvableEffect(t *testing.T) {
	t.Parallel()

	reg := fullyHydratedRegistry(t)

	// A loop over zero tools passes. This test exists to catch a check that
	// silently stops running, so it must not be able to become one itself:
	// if a registrar is renamed out of fullyHydratedRegistry, fail here rather
	// than report green over an empty catalog. The floor is well under the
	// current count (45) so ordinary additions and removals do not touch it.
	const minExpectedTools = 30
	if got := len(reg.All()); got < minExpectedTools {
		t.Fatalf("hydrated registry holds %d tools, expected at least %d: "+
			"a registrar is missing from fullyHydratedRegistry and this test is checking nothing",
			got, minExpectedTools)
	}

	var undeclared []string
	for _, tool := range reg.All() {
		if _, err := tool.DeclaredEffect(); err != nil {
			undeclared = append(undeclared, tool.Name)
		}
	}
	sort.Strings(undeclared)

	if len(undeclared) > 0 {
		t.Errorf("%d registered tool(s) have no resolvable effect and can never execute: %v\n\n"+
			"Fix by either adding the name to BuiltinEffect's manifest in effects.go "+
			"(for a reviewed built-in) or setting Tool.Effect at the registration site.",
			len(undeclared), undeclared)
	}
}

// TestEffects_WhenEffectMissing_ShouldFailClosedNotOpen pins the direction of
// the failure. A tool with no declaration must be refused, never silently
// treated as read-only — "unknown" is the one classification that must not
// default to the safest-looking answer, because an undeclared tool is far more
// likely to be a new effectful one than a new inert one.
//
// The refusal now happens at Register, not only at dispatch. That is the whole
// point: the author of the tool finds out at the registration site, where the
// fix is obvious, instead of at a call site two thousand lines away where the
// only visible symptom is a side effect that never happened.
func TestEffects_WhenEffectMissing_ShouldFailClosedNotOpen(t *testing.T) {
	t.Parallel()

	undeclared := &tools.Tool{
		Name:    "conformance_tool_with_no_effect",
		Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil },
	}

	if _, err := undeclared.DeclaredEffect(); err == nil {
		t.Fatal("DeclaredEffect returned no error for a tool with no effect: it must fail closed")
	}

	reg := tools.NewRegistry()
	err := reg.Register(undeclared)
	if err == nil {
		t.Fatal("Register accepted a tool with no effect: it would be catalogued, offered to the model, " +
			"and refused at dispatch with the Execute closure never entered")
	}
	if !strings.Contains(err.Error(), "effect declaration") {
		t.Errorf("Register error should name the missing effect declaration, got: %v", err)
	}

	// And the tool must not be visible afterwards. A rejected registration that
	// still leaves the tool in the catalog would reintroduce the exact split
	// this guards: advertised to the model, unable to run.
	if reg.Get(undeclared.Name) != nil {
		t.Error("a rejected tool is still present in the registry")
	}
}
