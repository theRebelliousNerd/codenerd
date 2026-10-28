package tools

import (
	"context"
	"testing"
)

func TestDeclaredCapability_BuiltInsTakeTheReviewedManifest(t *testing.T) {
	grep := &Tool{Name: "grep", Execute: func(context.Context, map[string]any) (string, error) { return "", nil }}
	if c, err := grep.DeclaredCapability(); err != nil || c != CapabilityOverlapping {
		t.Errorf("grep = %q, %v; run_command reaches the same end, so it overlaps", c, err)
	}
	// The manifest wins over a field set on a built-in's name.
	forged := &Tool{Name: "write_file", Capability: CapabilityOverlapping}
	if c, _ := forged.DeclaredCapability(); c != CapabilityUnique {
		t.Errorf("a field overrode the reviewed manifest: %q", c)
	}
}

func TestDeclaredCapability_DynamicToolDeclaresItsOwnOrNone(t *testing.T) {
	dyn := &Tool{Name: "plugin_probe", Capability: CapabilityUnique}
	if c, err := dyn.DeclaredCapability(); err != nil || c != CapabilityUnique {
		t.Errorf("declared capability = %q, %v", c, err)
	}
	none := &Tool{Name: "plugin_probe"}
	if c, err := none.DeclaredCapability(); err != nil || c != "" {
		t.Errorf("an undeclared dynamic tool = %q, %v; it declares none, which is not an error", c, err)
	}
	bad := &Tool{Name: "plugin_probe", Capability: "essential"}
	if _, err := bad.DeclaredCapability(); err == nil {
		t.Error("a capability that is neither unique nor overlapping was accepted")
	}
}

func TestBuiltinCapability_ValuesAreDeclaredClasses(t *testing.T) {
	for _, name := range []string{"read_file", "grep", "bash", "mcp_call", "find_symbol"} {
		if c := BuiltinCapability(name); !c.Valid() {
			t.Errorf("%s has capability %q", name, c)
		}
	}
	if BuiltinCapability("not_a_tool") != "" {
		t.Error("an unknown name has a capability")
	}
}
