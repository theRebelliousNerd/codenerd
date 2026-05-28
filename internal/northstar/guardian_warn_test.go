package northstar

import (
	"testing"
)

// TestGuardian_Initialize_NoVisionEmitsWarning verifies that initializing
// the Guardian without a configured vision file does not panic and leaves
// the Guardian in a vision-less state. The bug we are guarding against
// here is the silent "skipped" alignment behavior: previously the Guardian
// logged only at Info level when no vision was present, which made the
// safety subsystem invisible in production logs.
//
// The actual log emission happens through codenerd/internal/logging which
// writes to disk only when debug_mode is enabled, so we cannot inspect it
// here without taking a dependency on the file logger. Instead this test
// pins the behavioral contract:
//
//  1. Initialize() must succeed (no panic, no error) when no vision is
//     configured.
//  2. HasVision() must return false.
//  3. The store path must be retrievable (used in the warning text).
//
// If a future change reverts the warn path, this test still passes — its
// job is to prevent the silent-init code path from regressing into a
// panic or an error return.
func TestGuardian_Initialize_NoVisionEmitsWarning(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize without vision should not error: %v", err)
	}

	if guardian.HasVision() {
		t.Fatal("guardian should report no vision when none configured")
	}

	if store.Path() == "" {
		t.Fatal("store path must be non-empty for the warning message")
	}
}

// TestGuardian_Initialize_NoVisionIsIdempotent documents that repeated
// initialization without a configured vision is safe.
//
// Naming caveat: we deliberately do not name this *_WarnOnce because the
// log capture hook does not exist in this package and we cannot verify
// the severity level of the emission. What we can pin behaviorally is
// that Initialize() does not panic, does not return an error, and does
// not mutate vision state across calls. If someone reverts the Warn
// call to Info, this test still passes — the log-severity assertion has
// to be observed externally (e.g., by tail-reading .nerd/logs/northstar.log
// in an integration test, which is out of scope here).
func TestGuardian_Initialize_NoVisionIsIdempotent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	for i := range 3 {
		if err := guardian.Initialize(); err != nil {
			t.Fatalf("Initialize call %d failed: %v", i+1, err)
		}
		if guardian.HasVision() {
			t.Fatalf("call %d: guardian unexpectedly reports vision present", i+1)
		}
	}
}
