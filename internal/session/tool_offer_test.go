package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

func generatedOfferRegistry(t *testing.T, names ...string) *core.ToolRegistry {
	t.Helper()
	registry := core.NewToolRegistry(t.TempDir())
	for _, name := range names {
		if err := registry.RegisterToolWithInfo(&core.Tool{Name: name, Command: "go", Description: "Counts things in the workspace.", Capabilities: []string{"/count"}}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	return registry
}

// T1: a granted generated tool was offered by the single-shot Piggyback
// renderer but vanished from the working loop's Piggyback request, because
// that path intersected the allowlist with the native (modular-only)
// definitions before rendering. The model had to guess the tool existed.
func TestWorkingPiggybackRequestOffersGrantedGeneratedTool(t *testing.T) {
	name := fmt.Sprintf("gen_offer_%d", capabilityTestToolCounter.Add(1))
	ungranted := fmt.Sprintf("gen_offer_denied_%d", capabilityTestToolCounter.Add(1))
	client := newEnvelopeOnlyClient(func(int, string) string { return `{"surface_response":"ok"}` })
	e := piggybackTestExecutor(t, client)
	e.SetOuroborosRegistry(generatedOfferRegistry(t, name, ungranted))
	cfg := validCapabilityTestConfig(name)

	channel := &piggybackChannel{e: e, client: client, cfg: cfg}
	if _, err := e.completeWithWorkingContext(context.Background(), channel, "system", []types.Message{{Role: "user", Text: "count"}}, e.buildToolDefinitions(cfg)); err != nil {
		t.Fatalf("working request: %v", err)
	}
	if len(client.systems) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.systems))
	}
	sys := client.systems[0]
	if !strings.Contains(sys, "**"+name+"**") || !strings.Contains(sys, "Capabilities: /count") {
		t.Fatalf("granted generated tool not offered on the working Piggyback request:\n%s", sys)
	}
	if !strings.Contains(sys, "declares no typed schema") {
		t.Fatalf("generated tool's untyped contract not stated:\n%s", sys)
	}
	if strings.Contains(sys, ungranted) {
		t.Fatalf("an ungranted generated tool was offered:\n%s", sys)
	}
	// The native offer stays modular-only: a provider needs a typed schema.
	for _, def := range e.buildToolDefinitions(cfg) {
		if def.Name == name {
			t.Fatalf("generated tool offered natively with an invented schema")
		}
	}
}

// T2: the Piggyback catalog printed only required argument names, so
// optional controls and argument types never reached a text-channel model.
func TestPiggybackCatalogCarriesTheWholeArgumentSchema(t *testing.T) {
	registerProductionTools(t)
	e := &Executor{}
	catalog := e.buildToolCatalogForPiggyback(validCapabilityTestConfig("list_files", "get_element"))
	for _, want := range []string{
		"- recursive (boolean, optional)",
		"- include_hidden (boolean, optional)",
		"- path (string, required)",
		"- refs (array of string, optional)",
		"- full (boolean, optional)",
	} {
		if !strings.Contains(catalog, want) {
			t.Errorf("catalog missing %q:\n%s", want, catalog)
		}
	}
}

// T3: a grant with no backend, or a generated tool on a native channel, was
// in AvailableTools and so claimed callable to the JIT; it is now a typed
// disposition and the callable list is what the request can carry.
func TestResolveAvailableToolsSeparatesGrantsTheRequestCannotCarry(t *testing.T) {
	registerProductionTools(t)
	gen := fmt.Sprintf("gen_avail_%d", capabilityTestToolCounter.Add(1))
	missing := fmt.Sprintf("granted_without_backend_%d", capabilityTestToolCounter.Add(1))
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"read_file", gen, missing}}

	resolve := func(client types.LLMClient) *prompt.CompilationContext {
		e := &Executor{llmClient: client, EffectiveAgentRuntimeConfig: cfg}
		e.SetOuroborosRegistry(generatedOfferRegistry(t, gen))
		cc := &prompt.CompilationContext{}
		e.resolveAvailableTools(context.Background(), cc, perception.Intent{Verb: "/explain"})
		return cc
	}

	native := resolve(&MockLLMClient{})
	if strings.Join(native.AvailableTools, ",") != "read_file" {
		t.Fatalf("native available = %v, want only read_file", native.AvailableTools)
	}
	if !containsSubstring(native.UnavailableTools, gen+": generated tool with no typed argument schema") ||
		!containsSubstring(native.UnavailableTools, missing+": no backend is registered") {
		t.Fatalf("native dispositions = %v", native.UnavailableTools)
	}

	text := resolve(newEnvelopeOnlyClient(func(int, string) string { return "" }))
	if strings.Join(text.AvailableTools, ",") != "read_file,"+gen {
		t.Fatalf("text-channel available = %v, want read_file and the generated tool", text.AvailableTools)
	}
	if len(text.UnavailableTools) != 1 || !strings.HasPrefix(text.UnavailableTools[0], missing+":") {
		t.Fatalf("text-channel dispositions = %v", text.UnavailableTools)
	}
}
