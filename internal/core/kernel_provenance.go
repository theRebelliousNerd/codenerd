// Package core - provenance / "why did this fact get derived?" API.
//
// Backed by mangle-go's DerivationRecorder, which captures every rule
// firing, let-transform emission and do-transform (aggregation) emission as
// an evaluation runs. Off by default: EnableProvenance lets Explain answer.
//
// Evaluation itself never records. Explain records, on demand, the slice of
// the program upstream of the goal's predicate over the current EDB, and keeps
// that recording until the next write or program change
// (kernel_proof_slice.go). Recording during every evaluate() made each write a
// full recorded fixpoint, seconds and hundreds of megabytes on a world-sized
// EDB. ExplanationEnvelope (kernel_explanation.go) is the bounded,
// epoch-stamped form a consumer reads.
package core

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/provenance"
)

// EnableProvenance lets Explain answer. Nothing is recorded until a goal is
// explained; then the goal's slice of the current evaluation is recorded.
func (k *RealKernel) EnableProvenance() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.provenanceOn {
		k.provenanceOn = true
		logging.Kernel("Provenance ENABLED (proofs are recorded per goal on demand)")
	}
}

// DisableProvenance turns explanations off and releases recorded slices.
func (k *RealKernel) DisableProvenance() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.provenanceOn {
		k.provenanceOn = false
		k.proofs = proofSliceCache{}
		logging.Kernel("Provenance DISABLED")
	}
}

// IsProvenanceEnabled reports whether Explain may answer.
func (k *RealKernel) IsProvenanceEnabled() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.provenanceOn
}

// ExplainOptions controls the depth and breadth of Explain output.
// MaxProofs and MaxDepth zero values fall back to the provenance package
// defaults. MaxNodes, MaxEdges and MaxBytes bound what ExplainEnvelope
// extracts (config.ExplainConfig carries the configured values); zero means
// unbounded, which only Explain's raw proofs use.
type ExplainOptions struct {
	MaxProofs int
	MaxDepth  int
	MaxNodes  int
	MaxEdges  int
	MaxBytes  int
}

// Explain returns proof trees for the given goal fact. The goal must be
// a ground (variable-free) atom string parseable by the Mangle parser,
// e.g. "next_action(/generate_tool)" or "permitted(/edit, "main.go")".
//
// Returns ErrProvenanceDisabled if EnableProvenance() was not called.
// Returns ErrNoProof if the goal is not derived in the current evaluation.
func (k *RealKernel) Explain(goalStr string, opts ExplainOptions) ([]*provenance.ProofNode, error) {
	// The proof describes the current evaluation, so bring it up to date.
	if err := k.ensureEvaluated(); err != nil {
		return nil, fmt.Errorf("Explain: evaluation failed: %w", err)
	}

	// Exclusive: recording a slice fills the kernel's slice cache.
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.explainLocked(goalStr, opts)
}

// explainLocked builds the proofs of goalStr from the recorded slice of the
// current evaluation, recording it first if needed. Caller holds k.mu
// exclusively.
func (k *RealKernel) explainLocked(goalStr string, opts ExplainOptions) ([]*provenance.ProofNode, error) {
	if !k.provenanceOn {
		return nil, ErrProvenanceDisabled
	}
	if k.store == nil {
		return nil, fmt.Errorf("Explain: kernel store not initialized")
	}
	if err := k.readableLocked(); err != nil {
		return nil, err
	}

	// Parse the goal. The provenance API wants an ast.Atom, not a Fact —
	// the Mangle parser handles the round-trip for us via parse.Unit.
	parsed, err := parseUnit(strings.NewReader(goalStr + "."))
	if err != nil {
		return nil, fmt.Errorf("Explain: failed to parse goal %q: %w", goalStr, err)
	}
	if len(parsed.Clauses) == 0 {
		return nil, fmt.Errorf("Explain: no clauses parsed from goal %q", goalStr)
	}
	goal := parsed.Clauses[0].Head

	slice, err := k.proofSliceLocked(goal)
	if err != nil {
		return nil, err
	}
	proofs, err := provenance.BuildFromRecording(slice.recorder, slice.store, goal, provenance.Options{
		MaxProofs: opts.MaxProofs,
		MaxDepth:  opts.MaxDepth,
	})
	if err != nil {
		return nil, err
	}
	logging.Kernel("Explain: goal=%s produced %d proof(s)", goalStr, len(proofs))
	return proofs, nil
}

// ErrProvenanceDisabled is returned by Explain when provenance is off.
var ErrProvenanceDisabled = fmt.Errorf("provenance recording is disabled; call EnableProvenance() first")
