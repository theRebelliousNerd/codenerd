package core

import (
	"testing"
)

// TestVirtualStore_DreamRouterSingleton ensures that wiring a DreamRouter
// onto a VirtualStore is idempotent and that subsequent retrievals return
// the same instance. The bug we are guarding against here is the
// duplicate-construction pattern that was observed in dream.log at boot:
//
//	08:18:22 Creating DreamRouter
//	08:18:23 Creating DreamRouter
//
// Two boot-time code paths each instantiated DreamRouter, leaving the
// VirtualStore wired to one router while the UI model held another. Only
// the model-side router was connected to the Ouroboros queue, so half of
// the "DreamRouter connected" log lines never fired in production.
//
// This test pins the singleton invariant: the router accessed via
// VirtualStore.GetDreamer().GetDreamRouter() is the same pointer the
// factory wired in. Any caller that needs the router after boot must
// retrieve it from the dreamer, not construct a new one.
func TestVirtualStore_DreamRouterSingleton(t *testing.T) {
	t.Parallel()

	k := setupMockKernel(t)
	vs := NewVirtualStore(nil)
	vs.SetKernel(k)

	// Mimic the factory wiring path.
	router := NewDreamRouter(k, nil, nil)
	vs.SetDreamRouter(router)

	planMgr := NewDreamPlanManager(k)
	vs.SetDreamPlanManager(planMgr)

	dreamer := vs.GetDreamer()
	if dreamer == nil {
		t.Fatal("VirtualStore did not produce a Dreamer after SetKernel")
	}

	gotRouter := dreamer.GetDreamRouter()
	if gotRouter != router {
		t.Errorf("dreamer.GetDreamRouter() = %p, want factory-wired router %p",
			gotRouter, router)
	}

	gotPlanMgr := dreamer.GetDreamPlanManager()
	if gotPlanMgr != planMgr {
		t.Errorf("dreamer.GetDreamPlanManager() = %p, want factory-wired manager %p",
			gotPlanMgr, planMgr)
	}

	// Second retrieval must yield the same instances (no lazy re-creation).
	if dreamer.GetDreamRouter() != gotRouter {
		t.Error("second GetDreamRouter() returned a different instance — singleton invariant broken")
	}
	if dreamer.GetDreamPlanManager() != gotPlanMgr {
		t.Error("second GetDreamPlanManager() returned a different instance — singleton invariant broken")
	}
}

// TestVirtualStore_DreamerSingletonAcrossCalls verifies that the lazy
// VirtualStore.GetDreamer() entry point returns a stable Dreamer across
// calls. If the model_update boot path retrieves the dreamer twice, it
// must observe the same Dreamer instance both times. Previously the model
// constructed its own DreamRouter alongside the factory's, defeating
// this guarantee at the router level even though the underlying Dreamer
// was a singleton.
func TestVirtualStore_DreamerSingletonAcrossCalls(t *testing.T) {
	t.Parallel()

	k := setupMockKernel(t)
	vs := NewVirtualStore(nil)
	vs.SetKernel(k)

	d1 := vs.GetDreamer()
	d2 := vs.GetDreamer()
	if d1 == nil {
		t.Fatal("first GetDreamer() returned nil")
	}
	if d1 != d2 {
		t.Errorf("GetDreamer() not idempotent: %p vs %p", d1, d2)
	}
}
