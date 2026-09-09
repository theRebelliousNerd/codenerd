//go:build integration

package e2e_test

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/core"
)

// This file replaces virtualstore_dreamer_cache_integration_test.go and
// VirtualStoreInteractiveGate_DreamerCacheCollision_integration_test.go.
//
// Those two suites existed to expose contract violations in the Dreamer's
// safety-verdict cache — key collisions, false-positive denials, staleness
// after kernel change. They did their job: commit 8e9507d removed the cache
// outright, and dreamer.go now states the reason plainly:
//
//	Safety verdicts are deliberately not reused. The kernel has no revision
//	contract covering facts, policy and external predicates; a TTL or request
//	hash alone cannot make an authorization cache sound.
//
// The tests were not replaced, so they kept calling Dreamer.InvalidateCache and
// the whole 39-file e2e package stopped compiling. Nothing noticed, because the
// package is behind `//go:build integration` and neither `go test ./...` nor CI
// used the tag. A suite that cannot compile looks exactly like a suite that
// passes.
//
// What follows tests the property the removal bought, which is stronger than
// anything the old cache tests could assert: an identical request re-simulated
// after the kernel changes gets the NEW verdict, with no invalidation call and
// nothing to forget to invalidate.

// simulateDelete runs one delete_file simulation through the Dreamer.
func simulateDelete(t *testing.T, dreamer *core.Dreamer, path string) core.DreamResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return dreamer.SimulateAction(ctx, core.ActionRequest{
		Type:   core.ActionDeleteFile,
		Target: path,
	})
}

// TestE2E_DreamerVerdict_IsNeverReusedAcrossKernelChange is the contract test
// for the removed cache.
//
// A delete of an ordinary file is safe. Assert critical_file for that same
// path — the exact kernel change a cache has no revision contract to observe —
// and the identical request must now be refused. A cache keyed on the request
// would answer "safe" from the first run and let the delete through.
func TestE2E_DreamerVerdict_IsNeverReusedAcrossKernelChange(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	vs := core.NewVirtualStore(nil)
	vs.SetKernel(kernel)
	dreamer := vs.GetDreamer()
	if dreamer == nil {
		t.Fatal("VirtualStore did not create a Dreamer")
	}

	const target = "docs/scratch_notes.txt"

	before := simulateDelete(t, dreamer, target)
	if before.Unsafe {
		t.Fatalf("deleting an ordinary file was refused up front: %s", before.Reason)
	}

	// The kernel learns the file is critical. dreamer.mg derives
	// panic_state(Action, "critical_file_missing") from
	// projected_fact(/file_missing) joined against critical_file.
	if err := kernel.Assert(core.Fact{Predicate: "critical_file", Args: []any{target}}); err != nil {
		t.Fatalf("assert critical_file: %v", err)
	}

	after := simulateDelete(t, dreamer, target)
	if !after.Unsafe {
		t.Fatalf("the identical request was still allowed after the kernel marked %s critical; "+
			"a safety verdict was reused across a kernel change", target)
	}
	if before.ActionID == after.ActionID {
		t.Errorf("both simulations share an action id (%s); the second did not run", before.ActionID)
	}
}

// TestE2E_DreamerVerdict_KnownCriticalFileIsRefused pins the base case, so the
// test above cannot pass vacuously by refusing everything.
func TestE2E_DreamerVerdict_KnownCriticalFileIsRefused(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	vs := core.NewVirtualStore(nil)
	vs.SetKernel(kernel)
	dreamer := vs.GetDreamer()
	if dreamer == nil {
		t.Fatal("VirtualStore did not create a Dreamer")
	}

	// go.mod is critical_file in the shipped policy, no assertion needed.
	if got := simulateDelete(t, dreamer, "go.mod"); !got.Unsafe {
		t.Fatalf("deleting go.mod was allowed: %+v", got)
	}
	if got := simulateDelete(t, dreamer, "docs/scratch_notes.txt"); got.Unsafe {
		t.Fatalf("deleting an ordinary file was refused: %s", got.Reason)
	}
}

// TestE2E_DreamerVerdict_RepeatedSimulationsAreIndependent checks the other
// half: with the kernel unchanged, repeated identical requests must each
// produce their own evaluation rather than one memoised answer. Distinct action
// ids are the observable proof that the projection actually ran each time.
func TestE2E_DreamerVerdict_RepeatedSimulationsAreIndependent(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	vs := core.NewVirtualStore(nil)
	vs.SetKernel(kernel)
	dreamer := vs.GetDreamer()
	if dreamer == nil {
		t.Fatal("VirtualStore did not create a Dreamer")
	}

	seen := make(map[string]struct{}, 5)
	for i := 0; i < 5; i++ {
		got := simulateDelete(t, dreamer, "go.mod")
		if !got.Unsafe {
			t.Fatalf("run %d: deleting go.mod was allowed", i)
		}
		if _, dup := seen[got.ActionID]; dup {
			t.Fatalf("run %d reused action id %s; the simulation was memoised", i, got.ActionID)
		}
		seen[got.ActionID] = struct{}{}
	}
}
