package autopoiesis

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/mangle"
)

// TestVersionFromBinding unit-tests the tool_version binding coercion (F-OURO-1).
func TestVersionFromBinding(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"string form (schema /string)", "3", 3},
		{"string with whitespace", " 4 ", 4},
		{"int form", 5, 5},
		{"int64 form (Mangle NumberType)", int64(6), 6},
		{"float64 form", float64(7), 7},
		{"nil/absent", nil, 0},
		{"garbage string", "abc", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionFromBinding(tc.in); got != tc.want {
				t.Errorf("versionFromBinding(%#v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestHotReload_IncrementsVersion is the integration regression for F-OURO-1.
// Two hot-reloads of the same tool must produce version 2. The old code failed
// two ways at once: it wrote the version as an int (silently violating the
// tool_version ... /string decl, so the write was discarded) and read it back
// via ?tool_version(Tool, V) with V unbound (violating the bound [/string,
// /string] mode, so the read returned nothing) — leaving the counter at 1. The
// fix writes a string and reads via QueryFacts (a direct EDB scan that bypasses
// the mode check). Mirrors production's engine setup (real schema,
// AutoEval=false).
func TestHotReload_IncrementsVersion(t *testing.T) {
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = false
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	stateContent, err := core.GetDefaultContent("schemas_state.mg")
	if err != nil {
		t.Fatalf("GetDefaultContent(schemas_state.mg): %v", err)
	}
	if err := engine.LoadSchemaString(stateContent); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	loop := &OuroborosLoop{engine: engine}
	loop.hotReload("mytool")
	loop.hotReload("mytool")

	// Read via the same mode-safe path the fix uses (a query with unbound V
	// would violate the bound decl and return nothing).
	facts := engine.QueryFacts("tool_version", "mytool")
	if len(facts) == 0 {
		t.Fatalf("no tool_version fact written for mytool (write failed the /string decl?)")
	}
	var got []int
	found2 := false
	for _, f := range facts {
		if len(f.Args) >= 2 {
			v := versionFromBinding(f.Args[1])
			got = append(got, v)
			if v == 2 {
				found2 = true
			}
		}
	}
	if !found2 {
		t.Errorf("expected a tool_version of 2 after two hot-reloads; got versions %v", got)
	}
}
