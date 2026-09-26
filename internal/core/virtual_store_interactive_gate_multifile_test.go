package core

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// gateStore is a VirtualStore whose Dreamer is built before a test starts
// measuring, so the guard below sees only the gate.
func gateStore(t *testing.T) *VirtualStore {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vs := NewVirtualStore(nil)
	vs.SetKernel(k)
	if vs.getDreamer() == nil {
		t.Fatal("no Dreamer for a store with a kernel")
	}
	return vs
}

// boundedPreflight runs the gate under a heap guard. The recursion these
// tests pin grew the heap quadratically -- faster than a context deadline
// could stop it -- and starved the whole machine, so a regression must end
// the test binary here at +2 GiB rather than take every process down.
func boundedPreflight(vs *VirtualStore, id, tool string, args map[string]any) error {
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	done := make(chan error, 1)
	go func() { done <- vs.PreflightDestructiveToolCall(context.Background(), id, tool, args) }()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-tick.C:
			var now runtime.MemStats
			runtime.ReadMemStats(&now)
			if now.HeapAlloc > base.HeapAlloc+2<<30 {
				panic(fmt.Sprintf("%s preflight grew the heap past 2 GiB: the multi-file gate is recursing", tool))
			}
		}
	}
}

// A multi-file call is gated once per file and returns. Each file used to
// re-enter the gate with the call's "paths" still in its payload, so it was
// multi-file again and recursed without end; the quadratic action IDs took a
// two-path call to 200+ GiB committed (2026-09-26). Not parallel: the heap
// guard must see only this call.
func TestPreflight_AMultiFileCallGatesEachFileOnce(t *testing.T) {
	vs := gateStore(t)
	args := map[string]any{"paths": []any{"docs/a.md", "docs/b.md"}, "old": "x", "new": "y"}
	if err := boundedPreflight(vs, "repoint-1", "repoint", args); err != nil {
		t.Fatalf("a two-file repoint of ordinary files was not approved: %v", err)
	}
}

// Every file of a multi-file call meets the Dreamer: one protected file
// blocks the whole call.
func TestPreflight_AMultiFileCallIsBlockedByOneProtectedFile(t *testing.T) {
	vs := gateStore(t)
	err := boundedPreflight(vs, "repoint-2", "repoint", map[string]any{"paths": []any{"docs/a.md", ".git/config"}})
	if err == nil {
		t.Fatal("a repoint that rewrites .git/config passed the gate")
	}
	if !strings.Contains(err.Error(), "dreamer safety gate") {
		t.Fatalf("want the Dreamer's block, got: %v", err)
	}
}
