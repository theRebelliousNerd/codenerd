package mcpctl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// facadeSize is the number of verbs this package registers. It is asserted as a
// constant on purpose: the entire economic argument for the control plane is
// that this number does not move when a server with two hundred tools connects.
const facadeSize = 5

func TestRegisterAll_ShouldRegisterAFixedFacade(t *testing.T) {
	t.Parallel()

	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if got := reg.Count(); got != facadeSize {
		t.Errorf("registered %d tools, want %d; the facade must be fixed-size", got, facadeSize)
	}

	for _, name := range []string{"mcp_map", "mcp_probe", "mcp_call", "mcp_expand", "mcp_context"} {
		tool := reg.Get(name)
		if tool == nil {
			t.Errorf("%s is not registered", name)
			continue
		}
		// A tool with no valid effect declaration fails closed everywhere else
		// in this codebase; catching it here names the tool instead.
		if _, err := tool.DeclaredEffect(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if tool.Description == "" {
			t.Errorf("%s has no description; the description is where the disclosure ladder is taught", name)
		}
	}
}

func TestRegisterAll_ShouldBeIdempotent(t *testing.T) {
	t.Parallel()

	// HydrateModularTools registers into two registries and may run more than
	// once per process; a second pass must not error.
	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("first RegisterAll: %v", err)
	}
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("second RegisterAll: %v", err)
	}
	if got := reg.Count(); got != facadeSize {
		t.Errorf("count after re-register = %d, want %d", got, facadeSize)
	}
}

func TestTools_ShouldDeclareTheViewLadderConsistently(t *testing.T) {
	t.Parallel()

	// Every progressive surface in this codebase offers the same three views
	// and defaults to compact. An agent that learned the ladder on the browser
	// tools must not have to learn a second one here.
	for _, tool := range []*tools.Tool{MapTool(), ProbeTool(), CallTool(), ExpandTool(), ContextTool()} {
		view, ok := tool.Schema.Properties["view"]
		if !ok {
			t.Errorf("%s has no view property", tool.Name)
			continue
		}
		if view.Default != "compact" {
			t.Errorf("%s view default = %v, want compact", tool.Name, view.Default)
		}
		if len(view.Enum) != 3 {
			t.Errorf("%s view enum = %v, want summary/compact/full", tool.Name, view.Enum)
		}
	}
}

func TestExecute_WhenNoControlPlane_ShouldReportHonestlyNotPanic(t *testing.T) {
	// Not parallel: it mutates the package-level binding.
	SetControlPlane(nil)

	for name, execute := range map[string]func(context.Context, map[string]any) (string, error){
		"mcp_map":     executeMap,
		"mcp_probe":   executeProbe,
		"mcp_context": executeContext,
	} {
		out, err := execute(context.Background(), map[string]any{})
		if err != nil {
			t.Errorf("%s returned an error rather than an envelope: %v", name, err)
			continue
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Errorf("%s output is not JSON: %v", name, err)
			continue
		}
		if envelope["success"] != false {
			t.Errorf("%s reported success with no control plane bound", name)
		}
		if summary, _ := envelope["summary"].(string); !strings.Contains(summary, "MCP servers") {
			t.Errorf("%s summary = %q; want it to say plainly that none are configured", name, summary)
		}
	}
}

func TestExecute_ShouldRejectAnUnknownViewRatherThanDowngrade(t *testing.T) {
	SetControlPlane(nil)

	if _, err := executeMap(context.Background(), map[string]any{"view": "detailed"}); err == nil {
		t.Error("an unknown view was accepted")
	}
}

func TestExecuteProbe_ShouldRejectUnknownFacetAndRisk(t *testing.T) {
	SetControlPlane(nil)

	// These are enums in the schema, but a provider may not enforce enums, so
	// the tool has to. An unvalidated facet would silently match nothing and
	// read as "this server has no such tools".
	if _, err := executeProbe(context.Background(), map[string]any{"facet": "frobnicate"}); err == nil {
		t.Error("an unknown facet was accepted")
	}
	if _, err := executeProbe(context.Background(), map[string]any{"max_risk": "spicy"}); err == nil {
		t.Error("an unknown max_risk was accepted")
	}
}

func TestExecuteCall_ShouldRequireTool(t *testing.T) {
	SetControlPlane(nil)

	if _, err := executeCall(context.Background(), map[string]any{}); err == nil {
		t.Error("mcp_call without a tool was accepted")
	}
}

func TestMapArg_ShouldAcceptAnyJSONObjectShape(t *testing.T) {
	t.Parallel()

	// Providers hand nested arguments over in different Go shapes; rejecting a
	// well-formed call over the shape it arrived in leaks an implementation
	// detail into the agent's experience.
	direct, err := mapArg(map[string]any{"a": 1})
	if err != nil || direct["a"] != 1 {
		t.Errorf("direct map = %v, %v", direct, err)
	}
	if empty, err := mapArg(nil); err != nil || len(empty) != 0 {
		t.Errorf("nil args = %v, %v; want an empty map", empty, err)
	}
	roundTripped, err := mapArg(json.RawMessage(`{"b":2}`))
	if err != nil {
		t.Fatalf("raw JSON args: %v", err)
	}
	if got, _ := roundTripped["b"].(float64); got != 2 {
		t.Errorf("round-tripped args = %v", roundTripped)
	}
	if _, err := mapArg("not an object"); err == nil {
		t.Error("a non-object was accepted as args")
	}
}
