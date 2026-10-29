package session

import (
	"context"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/tools"
)

// An executor given its own modular registry offers, classifies and keeps
// only that registry's tools: nothing about its tool set is read from
// tools.Global(). That is what lets a test run an executor over its own tools
// without swapping the process-wide registry, and so run beside other tests.
func TestExecutor_OwnModularRegistryIsTheOnlyToolSource(t *testing.T) {
	t.Parallel()
	const own = "own_registry_probe_tool"
	if tools.Global().Has(own) {
		t.Fatalf("%s is in the global registry; the test needs a name only its own registry holds", own)
	}
	reg := tools.NewRegistry()
	if err := reg.Register(&tools.Tool{
		Name: own, Effect: tools.EffectExternal, Category: tools.CategoryGeneral, Description: "probe",
		Execute: func(context.Context, map[string]any) (string, error) { return "ok", nil },
	}); err != nil {
		t.Fatal(err)
	}

	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	e.SetModularTools(reg)

	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{own, "read_file"}}
	defs := e.buildToolDefinitions(cfg)
	if len(defs) != 1 || defs[0].Name != own {
		t.Fatalf("offered %+v, want exactly %s: a tool outside the executor's registry was offered, or its own was not", defs, own)
	}
	if got := e.effectAtom(own); got != "/external" {
		t.Errorf("effect of %s = %s, want /external read from the executor's registry", own, got)
	}
	if !e.remoteObservation(own) {
		t.Errorf("%s is external in the executor's registry but was not classified remote", own)
	}
	if got := e.toolUnavailability(own, false); got != "" {
		t.Errorf("%s reported unavailable (%q) on an executor whose registry holds it", own, got)
	}

	if clone := e.CloneForTask(); clone.modularTools() != reg {
		t.Error("a task clone lost the executor's registry and fell back to the global one")
	}
	if (&Executor{}).modularTools() != tools.Global() {
		t.Error("an executor given no registry does not use tools.Global()")
	}
}
