package catalog

import (
	"slices"
	"testing"

	"codenerd/internal/tools"
)

// A tool joins the catalog only with an answer to "what does this do that
// nothing else does?": the reviewed capability manifest names it. Without the
// declaration its value class could only ever read unknown.
func TestBuiltinCapabilityCoversCatalog(t *testing.T) {
	names, err := Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !tools.BuiltinCapability(name).Valid() {
			t.Errorf("built-in tool %q has no reviewed capability: add it to BuiltinCapability in internal/tools/capability.go", name)
		}
	}
}

// A manifest entry for a tool the catalog no longer registers is a stale
// declaration: it would hand a future tool of that name a capability nobody
// reviewed.
func TestBuiltinCapabilityNamesNoUnregisteredTool(t *testing.T) {
	names, err := Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range tools.BuiltinCapabilityNames() {
		if !slices.Contains(names, name) {
			t.Errorf("BuiltinCapability declares %q, which the catalog does not register", name)
		}
	}
}
