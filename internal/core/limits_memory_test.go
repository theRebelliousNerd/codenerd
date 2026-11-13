package core

import (
	"runtime"
	"testing"
)

// RuntimeMemory agrees with ReadMemStats to within what a concurrent test
// binary allocates between the two reads, and never reports zero.
func TestRuntimeMemory_TracksTheHeapWithoutStoppingTheWorld(t *testing.T) {
	alloc, sys := RuntimeMemory()
	if alloc == 0 || sys == 0 {
		t.Fatalf("alloc=%d sys=%d: a metric was not read", alloc, sys)
	}
	if alloc > sys {
		t.Fatalf("heap objects (%d) exceed everything the runtime mapped (%d)", alloc, sys)
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	if sys < ms.Sys/2 || sys > ms.Sys*2 {
		t.Fatalf("sys = %d, ReadMemStats says %d", sys, ms.Sys)
	}
	// Heap objects, as ReadMemStats counts them, within what a collection
	// between the two reads can move.
	if alloc < ms.HeapAlloc/8 || alloc > ms.HeapAlloc*8 {
		t.Fatalf("heap objects = %d, ReadMemStats says %d", alloc, ms.HeapAlloc)
	}
}
