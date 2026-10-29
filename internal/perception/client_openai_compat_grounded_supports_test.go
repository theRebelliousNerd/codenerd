package perception

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

func TestSupportsGroundedWebSearch_RawCapabilityTruth(t *testing.T) {
	t.Parallel()
	var nilClient *OpenAICompatClient
	if nilClient.SupportsGroundedWebSearch() {
		t.Fatal("nil client must return false")
	}
	// Compile-time assert via interface
	var _ types.GroundedWebSearcher = nilClient
	var _ types.GroundedWebSearcher = (*OpenAICompatClient)(nil)

	meta := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")
	if !meta.SupportsGroundedWebSearch() {
		t.Error("meta client must support grounded search")
	}
	if _, ok := any(meta).(types.GroundedWebSearcher); !ok {
		t.Error("meta client must implement GroundedWebSearcher")
	}

	for _, vendor := range []Provider{ProviderDashScope, ProviderMoonshot} {
		c := newTestCompatClient(t, vendor, "https://example.com/v1")
		if c.SupportsGroundedWebSearch() {
			t.Errorf("vendor %s must return false for SupportsGroundedWebSearch", vendor)
		}
		// Interface still implemented, but GroundedWebSearch must fail closed.
		if _, ok := any(c).(types.GroundedWebSearcher); !ok {
			t.Errorf("vendor %s should still implement GroundedWebSearcher (fail closed)", vendor)
		}
		_, err := c.GroundedWebSearch(context.Background(), "hello")
		if err == nil {
			t.Errorf("vendor %s: expected fail-closed error on GroundedWebSearch", vendor)
		}
	}
}

func TestSupportsGroundedWebSearch_Deterministic(t *testing.T) {
	t.Parallel()
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")
	// Multiple calls must be deterministic.
	for i := 0; i < 5; i++ {
		if !c.SupportsGroundedWebSearch() {
			t.Fatalf("call %d: expected true", i)
		}
	}
	ds := newTestCompatClient(t, ProviderDashScope, "https://example.com")
	for i := 0; i < 5; i++ {
		if ds.SupportsGroundedWebSearch() {
			t.Fatalf("dashscope call %d: expected false", i)
		}
	}
}
