package chat

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	nerdsystem "codenerd/internal/system"
)

// newCortexModel is a test model on the production kernel: the domain shards
// the factory boots (nerdsystem.NewDomainCortex), not a single-store
// RealKernel.
func newCortexModel(t *testing.T) (Model, *core.CortexKernel) {
	t.Helper()
	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	m := NewTestModel()
	m.kernel = ck
	return m, ck
}

// The session asks the Cortex; the catch-all shard's kernel is only for the
// consumers that need a RealKernel. Until 2026-09-23 the session ran on the
// catch-all (cortex.RealKernel): chat's facts reached one shard of seven, and
// the tactile router it re-registered read permitted_action -- owned by the
// policy shard -- where no such row ever lands.
func TestSessionKernels_TheSessionAsksTheCortex(t *testing.T) {
	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	primary := ck.GetPrimaryRealKernel()
	kernel, gotPrimary, err := sessionKernels(&nerdsystem.Cortex{Kernel: ck, RealKernel: primary})
	if err != nil {
		t.Fatal(err)
	}
	if kernel != chatKernel(ck) {
		t.Errorf("the session kernel is %T, want the Cortex", kernel)
	}
	if gotPrimary != primary {
		t.Error("the RealKernel consumers did not get the catch-all shard's kernel")
	}
	if _, _, err := sessionKernels(&nerdsystem.Cortex{Kernel: ck}); err == nil {
		t.Error("a Cortex without its catch-all kernel was accepted")
	}
}

// A chat turn routes on the production kernel, and the facts it asserts
// reach the shards whose rules join them: the turn's intent is in shards
// other than the catch-all.
func TestDecideRoute_OnTheProductionKernel(t *testing.T) {
	m, ck := newCortexModel(t)
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "internal/session/executor.go", Confidence: 0.95}
	assertRouteIntent(t, m, intent)

	route := m.decideRoute("review internal/session/executor.go and fix any issues", intent, "coder")
	if route.Kind != RouteMultiStep || len(route.Steps) != 2 {
		t.Fatalf("route = %s with %d steps, want multi_step with 2", route.Kind, len(route.Steps))
	}
	holders := 0
	for _, domain := range ck.ShardDomains() {
		shard, ok := ck.GetShard(domain)
		if !ok || domain == "cortex" {
			continue
		}
		if rows, err := shard.Kernel().Query("user_intent"); err == nil && len(rows) > 0 {
			holders++
		}
	}
	if holders == 0 {
		t.Error("the turn's user_intent reached no shard but the catch-all")
	}

	if err := m.kernel.Retract("user_intent"); err != nil {
		t.Fatal(err)
	}
	fix := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.93}
	assertRouteIntent(t, m, fix)
	if route := m.decideRoute("fix the typo in README.md", fix, "coder"); route.Kind != RouteDelegate || route.Shard != "coder" || !route.Verify {
		t.Errorf("route = %s/%q verify=%v, want delegate/coder, verified: route_verifies must derive on the sharded kernel", route.Kind, route.Shard, route.Verify)
	}
}
