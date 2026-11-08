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

func TestExecute_WhenNoControlPlane_ShouldCoverCallAndExpand(t *testing.T) {
	// The existing no-plane test pins only map/probe/context. Call and expand
	// take the same failure-envelope path (validate args, then look up the
	// plane), so a missing plane must report honestly rather than panic or
	// return a Go error for them as well.
	SetControlPlane(nil)

	for name, execute := range map[string]func(context.Context, map[string]any) (string, error){
		"mcp_call": func(ctx context.Context, args map[string]any) (string, error) {
			return executeCall(ctx, map[string]any{"tool": "srv/tool"})
		},
		"mcp_expand": func(ctx context.Context, _ map[string]any) (string, error) {
			return executeExpand(ctx, map[string]any{"handle": "h1"})
		},
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
		if _, ok := envelope["next_step"]; !ok {
			t.Errorf("%s envelope is missing next_step guidance", name)
		}
	}
}

func TestExecute_ShouldRejectUnknownViewOnEveryVerb(t *testing.T) {
	// Only executeMap's view rejection was pinned. Every executor validates
	// the view before touching the plane, so a misspelled view is wrong even
	// when no server is configured, on every verb.
	SetControlPlane(nil)

	cases := map[string]func(context.Context, map[string]any) (string, error){
		"mcp_map":   executeMap,
		"mcp_probe": executeProbe,
		"mcp_call": func(ctx context.Context, args map[string]any) (string, error) {
			return executeCall(ctx, map[string]any{"view": "detailed", "tool": "srv/tool"})
		},
		"mcp_expand": func(ctx context.Context, args map[string]any) (string, error) {
			return executeExpand(ctx, map[string]any{"view": "detailed", "handle": "h1"})
		},
		"mcp_context": executeContext,
	}
	for name, execute := range cases {
		if _, err := execute(context.Background(), map[string]any{"view": "detailed"}); err == nil {
			t.Errorf("%s accepted an unknown view", name)
		} else if !strings.Contains(err.Error(), "unsupported view") {
			t.Errorf("%s error = %q; want it to name the unsupported view", name, err.Error())
		}
	}
}

func TestExecuteExpand_ShouldRequireHandle(t *testing.T) {
	SetControlPlane(nil)

	if _, err := executeExpand(context.Background(), map[string]any{}); err == nil {
		t.Error("mcp_expand without a handle was accepted")
	}
	if _, err := executeExpand(context.Background(), map[string]any{"handle": "   "}); err == nil {
		t.Error("mcp_expand with a blank handle was accepted")
	}
}

func TestExecuteCall_ShouldRejectNonObjectArgs(t *testing.T) {
	// mapArg itself is pinned; this pins the executeCall wrapper, which must
	// prefix the failure with the verb so the next attempt is informed.
	SetControlPlane(nil)

	if _, err := executeCall(context.Background(), map[string]any{"tool": "srv/tool", "args": "not an object"}); err == nil {
		t.Error("mcp_call with non-object args was accepted")
	} else if !strings.Contains(err.Error(), "mcp_call") {
		t.Errorf("mcp_call args error = %q; want it attributed to mcp_call", err.Error())
	}
}

func TestArgHelpers_ShouldHonorFallbacks(t *testing.T) {
	t.Parallel()

	if got := stringArg(map[string]any{"s": "v"}, "s"); got != "v" {
		t.Errorf("stringArg = %q, want %q", got, "v")
	}
	if got := stringArg(map[string]any{}, "missing"); got != "" {
		t.Errorf("stringArg missing = %q, want empty", got)
	}
	if got := stringArg(map[string]any{"s": 42}, "s"); got != "" {
		t.Errorf("stringArg non-string = %q, want empty", got)
	}

	if got := boolArg(map[string]any{"b": true}, "b", false); got != true {
		t.Errorf("boolArg true = %v, want true", got)
	}
	if got := boolArg(map[string]any{}, "missing", true); got != true {
		t.Errorf("boolArg missing = %v, want fallback true", got)
	}
	if got := boolArg(map[string]any{"b": "yes"}, "b", false); got != false {
		t.Errorf("boolArg non-bool = %v, want fallback false", got)
	}
}

func TestFailure_ShouldRenderFailedEnvelope(t *testing.T) {
	t.Parallel()

	out, err := failure("boom", "do this")
	if err != nil {
		t.Fatalf("failure returned an error: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("failure output is not JSON: %v", err)
	}
	if envelope["success"] != false {
		t.Errorf("failure envelope success = %v, want false", envelope["success"])
	}
	if envelope["summary"] != "boom" {
		t.Errorf("failure envelope summary = %v, want boom", envelope["summary"])
	}
	if envelope["next_step"] != "do this" {
		t.Errorf("failure envelope next_step = %v, want the guidance", envelope["next_step"])
	}

	if _, err := marshalResult(make(chan int)); err == nil {
		t.Error("marshalResult of an unmarshalable value was accepted")
	} else if !strings.Contains(err.Error(), "marshal MCP control-plane result") {
		t.Errorf("marshalResult error = %q; want it wrapped with context", err.Error())
	}
}

func TestResolveView_ShouldDefaultAndNormalize(t *testing.T) {
	t.Parallel()

	view, err := resolveView(map[string]any{})
	if err != nil {
		t.Fatalf("empty view: %v", err)
	}
	if string(view) != "compact" {
		t.Errorf("empty view = %q, want compact", view)
	}
	view, err = resolveView(map[string]any{"view": "  FULL "})
	if err != nil {
		t.Fatalf("padded view: %v", err)
	}
	if string(view) != "full" {
		t.Errorf("padded view = %q, want full", view)
	}
	if _, err := resolveView(map[string]any{"view": "detailed"}); err == nil {
		t.Error("unknown view was accepted")
	}
}

func TestMapArg_ShouldRoundTripStructShapes(t *testing.T) {
	t.Parallel()

	got, err := mapArg(struct {
		Name string `json:"name"`
	}{Name: "srv"})
	if err != nil {
		t.Fatalf("struct args: %v", err)
	}
	if got["name"] != "srv" {
		t.Errorf("struct args = %v, want name=srv", got)
	}
}

func TestRegisterAll_ShouldSkipPreexistingTools(t *testing.T) {
	t.Parallel()

	reg := tools.NewRegistry()
	sentinel := MapTool()
	sentinel.Description = "sentinel"
	if err := reg.Register(sentinel); err != nil {
		t.Fatalf("pre-register mcp_map: %v", err)
	}
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if got := reg.Count(); got != facadeSize {
		t.Errorf("count = %d, want %d", got, facadeSize)
	}
	if got := reg.Get("mcp_map"); got == nil || got.Description != "sentinel" {
		t.Errorf("pre-existing mcp_map was overwritten")
	}
	if got := reg.Get("mcp_call"); got == nil {
		t.Error("mcp_call was not registered alongside the pre-existing tool")
	}
}

func TestTools_ShouldDeclareEffectsAndRequirements(t *testing.T) {
	t.Parallel()

	effects := map[string]tools.Effect{
		"mcp_map":     tools.EffectRead,
		"mcp_probe":   tools.EffectRead,
		"mcp_expand":  tools.EffectRead,
		"mcp_call":    tools.EffectExternal,
		"mcp_context": tools.EffectExternal,
	}
	constructors := map[string]func() *tools.Tool{
		"mcp_map":     MapTool,
		"mcp_probe":   ProbeTool,
		"mcp_call":    CallTool,
		"mcp_expand":  ExpandTool,
		"mcp_context": ContextTool,
	}
	for name, build := range constructors {
		tool := build()
		if tool.Name != name {
			t.Errorf("tool name = %q, want %q", tool.Name, name)
		}
		effect, err := tool.DeclaredEffect()
		if err != nil {
			t.Errorf("%s effect: %v", name, err)
			continue
		}
		if effect != effects[name] {
			t.Errorf("%s effect = %v, want %v", name, effect, effects[name])
		}
	}
	if got := CallTool().Schema.Required; len(got) != 1 || got[0] != "tool" {
		t.Errorf("mcp_call required = %v, want [tool]", got)
	}
	if got := ExpandTool().Schema.Required; len(got) != 1 || got[0] != "handle" {
		t.Errorf("mcp_expand required = %v, want [handle]", got)
	}
}
