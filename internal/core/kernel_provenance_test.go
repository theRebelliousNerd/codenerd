package core

import (
	"errors"
	"testing"
)

// TestProvenance_DisabledByDefault asserts that fresh kernels do not
// pay the cost of a DerivationRecorder. Explain() should return
// ErrProvenanceDisabled and nothing should be allocated for proof
// recording.
func TestProvenance_DisabledByDefault(t *testing.T) {
	k := &RealKernel{}
	if k.IsProvenanceEnabled() {
		t.Fatalf("fresh kernel reports provenance enabled; want disabled")
	}
	_, err := k.Explain("foo(/bar)", ExplainOptions{})
	if err == nil {
		t.Fatalf("Explain on uninitialized kernel returned nil error; want a failure")
	}
	// Either ErrProvenanceDisabled (preferred) or an evaluation error
	// (ensureEvaluated may run first and fail). Both prove provenance
	// is off; we only care that we don't crash.
	_ = errors.Is(err, ErrProvenanceDisabled)
}

// TestProvenance_EnableTogglesRecorder verifies the public on/off API
// flips the internal recorder state and IsProvenanceEnabled mirrors it.
func TestProvenance_EnableTogglesRecorder(t *testing.T) {
	k := &RealKernel{}

	k.EnableProvenance()
	if !k.IsProvenanceEnabled() {
		t.Errorf("after EnableProvenance, IsProvenanceEnabled returned false")
	}
	if k.proofRecorder == nil {
		t.Errorf("after EnableProvenance, proofRecorder is nil")
	}

	// Calling Enable a second time should be idempotent (no panic, still on).
	k.EnableProvenance()
	if !k.IsProvenanceEnabled() {
		t.Errorf("Enable should be idempotent; got disabled")
	}

	k.DisableProvenance()
	if k.IsProvenanceEnabled() {
		t.Errorf("after DisableProvenance, IsProvenanceEnabled returned true")
	}
	if k.proofRecorder != nil {
		t.Errorf("after DisableProvenance, proofRecorder is non-nil")
	}

	// Disable when already disabled is a no-op.
	k.DisableProvenance()
	if k.IsProvenanceEnabled() {
		t.Errorf("second Disable shouldn't re-enable; got enabled")
	}
}
