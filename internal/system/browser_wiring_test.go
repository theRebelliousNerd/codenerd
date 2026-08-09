package system

import (
	"testing"
	"time"

	"codenerd/internal/mangle"
)

func TestBrowserKernelSinkAssertsIntoLiveKernel(t *testing.T) {
	kernel := &MockSystemKernel{}
	sink := browserKernelSink{kernel: kernel}
	originalArgs := []any{"session-1", "https://example.test", int64(42)}

	if err := sink.AddFacts([]mangle.Fact{{
		Predicate: "navigation_event",
		Args:      originalArgs,
		Timestamp: time.Now(),
	}}); err != nil {
		t.Fatalf("AddFacts() error = %v", err)
	}
	if len(kernel.facts) != 1 {
		t.Fatalf("live kernel fact count = %d, want 1", len(kernel.facts))
	}
	got := kernel.facts[0]
	if got.Predicate != "navigation_event" || len(got.Args) != 3 {
		t.Fatalf("unexpected converted fact: %+v", got)
	}

	originalArgs[0] = "mutated"
	if got.Args[0] != "session-1" {
		t.Fatal("browser kernel sink retained the caller's mutable argument slice")
	}
}
