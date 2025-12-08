package mangle

import (
	"testing"
)

func TestInspectDecl(t *testing.T) {
	cfg := DefaultConfig()
	baseEngine, _ := NewEngine(cfg, nil)
	// Decl p(String).
	baseEngine.LoadSchemaString("Decl p(String).")

	if baseEngine.programInfo == nil {
		t.Fatal("programInfo nil")
	}

	for sym, d := range baseEngine.programInfo.Decls {
		t.Logf("Sym: %v", sym)
		t.Logf("Decl: %+v", d)
		t.Logf("Modes Value: %+v", d.Modes())
		// Cannot print type of return value if empty, but can print definition via %T on result variable?
		modes := d.Modes()
		t.Logf("Modes Type: %T", modes)
	}
}
