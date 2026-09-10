package tools

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// The capability envelope is the registry's security boundary: it decides which
// tools may execute at all. It sat at 0% coverage.
//
// The rule that matters most is the one easiest to get backwards. Enforced with
// an empty list must deny EVERYTHING, not permit everything — an empty
// AllowedTools is what an unconfigured or failed session produces, and a
// registry that reads that as "no restrictions" hands the full catalog to
// exactly the caller that failed to establish an envelope.

func allowTool(t *testing.T, r *Registry, name string, ran *bool) {
	t.Helper()
	err := r.Register(&Tool{
		Name:     name,
		Effect:   EffectRead,
		Category: CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) {
			if ran != nil {
				*ran = true
			}
			return "executed", nil
		},
	})
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
}

func TestEnforcedEmptyAllowlistDeniesEverything(t *testing.T) {
	r := NewRegistry()
	var ran bool
	allowTool(t, r, "probe", &ran)

	r.SetAllowlist(&Allowlist{Enforced: true, Names: nil})

	if r.IsAllowed("probe") {
		t.Fatal("an enforced empty envelope permitted a tool; an unconfigured session " +
			"would receive the full catalog")
	}

	res, err := r.Execute(context.Background(), "probe", nil)
	if err == nil {
		t.Fatal("execution succeeded under an enforced empty envelope")
	}
	if !errors.Is(err, ErrToolNotAllowed) {
		t.Errorf("error = %v, want ErrToolNotAllowed so a caller can tell a refusal from a failure", err)
	}
	if ran {
		t.Error("the tool's Execute closure ran despite the refusal")
	}
	if res == nil || res.ToolName != "probe" {
		t.Errorf("result = %+v, want one naming the refused tool", res)
	}
}

func TestUnenforcedAllowlistDoesNotGate(t *testing.T) {
	r := NewRegistry()
	var ran bool
	allowTool(t, r, "probe", &ran)

	// Enforced=false is the explicit "no envelope" state, distinct from an
	// enforced empty one. Conflating them in either direction is a bug.
	r.SetAllowlist(&Allowlist{Enforced: false, Names: nil})

	if !r.IsAllowed("probe") {
		t.Error("an unenforced envelope denied a tool")
	}
	if r.AllowlistEnforced() {
		t.Error("AllowlistEnforced reported true for an unenforced envelope")
	}
	if _, err := r.Execute(context.Background(), "probe", nil); err != nil {
		t.Fatalf("Execute under no envelope: %v", err)
	}
	if !ran {
		t.Error("the tool did not run under an unenforced envelope")
	}
}

func TestNilAllowlistIsUnconstrained(t *testing.T) {
	r := NewRegistry()
	allowTool(t, r, "probe", nil)

	if !r.IsAllowed("probe") {
		t.Error("a registry with no envelope denied a tool")
	}
	if r.AllowlistEnforced() {
		t.Error("AllowlistEnforced reported true with no envelope installed")
	}

	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"probe"}})
	r.SetAllowlist(nil)
	// Removing an envelope must restore the unconstrained state rather than
	// leave the last one in force.
	if r.AllowlistEnforced() {
		t.Error("SetAllowlist(nil) did not remove the envelope")
	}
}

func TestAllowlistPermitsOnlyNamedTools(t *testing.T) {
	r := NewRegistry()
	var permittedRan, deniedRan bool
	allowTool(t, r, "permitted", &permittedRan)
	allowTool(t, r, "denied", &deniedRan)

	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"permitted"}})

	if _, err := r.Execute(context.Background(), "permitted", nil); err != nil {
		t.Fatalf("permitted tool: %v", err)
	}
	if !permittedRan {
		t.Error("the permitted tool did not run")
	}

	if _, err := r.Execute(context.Background(), "denied", nil); !errors.Is(err, ErrToolNotAllowed) {
		t.Errorf("denied tool error = %v, want ErrToolNotAllowed", err)
	}
	if deniedRan {
		t.Error("the denied tool ran")
	}
}

func TestAllowlistIsClonedOnInstall(t *testing.T) {
	r := NewRegistry()
	allowTool(t, r, "probe", nil)

	names := []string{"probe"}
	envelope := &Allowlist{Enforced: true, Names: names}
	r.SetAllowlist(envelope)

	// Mutating the caller's slice after installing must not widen the
	// envelope. A caller that reuses a buffer, or a config object that is
	// rebuilt in place, would otherwise silently change what may execute.
	names[0] = "something-else"
	envelope.Names = append(envelope.Names, "smuggled")

	if !r.IsAllowed("probe") {
		t.Error("mutating the caller's slice narrowed the installed envelope")
	}
	if r.IsAllowed("smuggled") {
		t.Error("appending to the caller's allowlist after install widened the envelope")
	}
}

func TestAllowlistRefusalIsAuditedAsARefusalNotAFailure(t *testing.T) {
	r := NewRegistry()
	allowTool(t, r, "probe", nil)
	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"other"}})

	res, err := r.Execute(context.Background(), "probe", nil)
	if err == nil {
		t.Fatal("execution succeeded")
	}
	// The message has to say what happened and how wide the envelope was, or
	// an operator debugging a denied tool cannot tell an empty envelope from a
	// misspelled name.
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error does not mention the allowlist: %v", err)
	}
	if res.DurationMs < 0 {
		t.Errorf("refusal duration = %d, want a non-negative measurement", res.DurationMs)
	}
}

func TestFilterByIntentRespectsTheEnvelope(t *testing.T) {
	r := NewRegistry()
	allowTool(t, r, "visible", nil)
	allowTool(t, r, "hidden", nil)

	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"visible"}})

	// Offering a tool the registry will refuse to run wastes a turn: the model
	// picks it, the call is denied, and the failure looks like a tool bug.
	for _, got := range r.FilterByIntent("/general") {
		if got.Name == "hidden" {
			t.Error("a tool outside the envelope was offered to the model")
		}
	}
}

func TestGlobalAllowlistSettersReachTheGlobalRegistry(t *testing.T) {
	// The global registry has no envelope getter, so the pre-state cannot be
	// captured and restored. It is unconstrained by default and nothing else
	// in this package installs one -- asserted here rather than assumed, so a
	// future test that does install one fails loudly instead of being
	// silently undone by this cleanup.
	if Global().AllowlistEnforced() {
		t.Fatal("the global registry already has an envelope; this test would destroy it")
	}
	t.Cleanup(func() { SetGlobalAllowlist(nil) })

	SetGlobalAllowlist(&Allowlist{Enforced: true, Names: []string{"only-this"}})
	if !Global().AllowlistEnforced() {
		t.Fatal("SetGlobalAllowlist did not reach the global registry")
	}
	if Global().IsAllowed("anything-else") {
		t.Error("the global envelope permits a tool outside it")
	}

	SetGlobalAllowlist(nil)
	if Global().AllowlistEnforced() {
		t.Error("clearing the global envelope left it in force")
	}
}

func TestWorkspaceRootRoundTrips(t *testing.T) {
	r := NewRegistry()
	if got := r.WorkspaceRoot(); got != "" {
		t.Errorf("WorkspaceRoot = %q on a fresh registry, want empty", got)
	}

	r.SetWorkspaceRoot("  /tmp/workspace  ")
	if got := r.WorkspaceRoot(); got != "/tmp/workspace" {
		t.Errorf("WorkspaceRoot = %q, want it trimmed", got)
	}

	// An empty root clears the boundary and restores the env/cwd fallback,
	// rather than pinning containment to the empty string.
	r.SetWorkspaceRoot("")
	if got := r.WorkspaceRoot(); got != "" {
		t.Errorf("WorkspaceRoot = %q after clearing, want empty", got)
	}
}

func TestAllowlistIsSafeUnderConcurrentInstallAndCheck(t *testing.T) {
	r := NewRegistry()
	allowTool(t, r, "probe", nil)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_ = r.IsAllowed("probe")
				_ = r.AllowlistEnforced()
			}
		}()
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 250; j++ {
				r.SetAllowlist(&Allowlist{Enforced: j%2 == 0, Names: []string{"probe"}})
			}
		}(i)
	}
	wg.Wait()
}

func TestLookupEffectFailsClosedForUnknownTools(t *testing.T) {
	// An effect that cannot be resolved must be an error, not a default: the
	// executive gate refuses what it cannot classify, and inventing a
	// permissive default here would route an unclassified tool straight past
	// it.
	if _, err := LookupEffect("a-tool-that-was-never-registered"); err == nil {
		t.Fatal("LookupEffect resolved an unknown tool")
	}

	name := "lookup-effect-probe"
	registerGlobalProbe(t, name)
	got, err := LookupEffect(name)
	if err != nil {
		t.Fatalf("LookupEffect on a registered tool: %v", err)
	}
	if got != EffectRead {
		t.Errorf("effect = %q, want %q", got, EffectRead)
	}
}

func registerGlobalProbe(t *testing.T, name string) {
	t.Helper()
	err := Global().Register(&Tool{
		Name:     name,
		Effect:   EffectRead,
		Category: CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) {
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
	t.Cleanup(func() { Global().Unregister(name) })
}
