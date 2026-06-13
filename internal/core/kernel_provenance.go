// Package core - provenance / "why did this fact get derived?" API.
//
// Backed by the Codeberg mangle-go fork's DerivationRecorder mechanism
// (commit 4dcaa582), which captures every rule firing, let-transform
// emission, and do-transform (aggregation) emission as evaluation runs.
// Off by default — must be turned on with EnableProvenance() before the
// next evaluate(), since the recorder is wired during eval option
// assembly (see kernel_eval.go).
//
// The recorder buffer is reset at the start of every evaluate() so its
// memory footprint stays bounded; only the most recent fixpoint pass is
// inspectable. Long-running sessions can therefore leave provenance on
// without unbounded growth.
package core

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/provenance"
)

// EnableProvenance turns on derivation recording. The next evaluate()
// will install a fresh MemoryRecorder; subsequent evaluates will reset
// it. To take effect immediately on the next Query/QueryAll, also call
// MarkPolicyDirty() or assert a fact.
func (k *RealKernel) EnableProvenance() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.proofRecorder == nil {
		k.proofRecorder = provenance.NewMemoryRecorder()
		logging.Kernel("Provenance recording ENABLED (next evaluate will populate)")
	}
}

// DisableProvenance turns off derivation recording and releases the
// current recorder buffer.
func (k *RealKernel) DisableProvenance() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.proofRecorder != nil {
		k.proofRecorder = nil
		logging.Kernel("Provenance recording DISABLED")
	}
}

// IsProvenanceEnabled reports whether the recorder is currently active.
func (k *RealKernel) IsProvenanceEnabled() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.proofRecorder != nil
}

// ExplainOptions controls the depth and breadth of Explain output.
// Zero values fall back to the provenance package defaults (3 proofs,
// depth 32) which suit most "why was X derived?" inspections.
type ExplainOptions struct {
	MaxProofs int
	MaxDepth  int
}

// Explain returns proof trees for the given goal fact. The goal must be
// a ground (variable-free) atom string parseable by the Mangle parser,
// e.g. "next_action(/generate_tool)" or "permitted(/edit, "main.go")".
//
// Returns ErrProvenanceDisabled if EnableProvenance() was not called
// before the most recent evaluate(). Returns ErrNoProof if the recorder
// has no events for the goal (either it was never derived, or it was
// derived before the recorder was installed).
func (k *RealKernel) Explain(goalStr string, opts ExplainOptions) ([]*provenance.ProofNode, error) {
	// First ensure evaluation is current (the recorder only sees the most
	// recent pass anyway, but stale dirty facts would be confusing).
	if err := k.ensureEvaluated(); err != nil {
		return nil, fmt.Errorf("Explain: evaluation failed: %w", err)
	}

	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.proofRecorder == nil {
		return nil, ErrProvenanceDisabled
	}
	if k.store == nil {
		return nil, fmt.Errorf("Explain: kernel store not initialized")
	}

	// Parse the goal. The provenance API wants an ast.Atom, not a Fact —
	// the Mangle parser handles the round-trip for us via parse.Unit.
	programStr := goalStr + "."
	parsed, err := parseUnit(strings.NewReader(programStr))
	if err != nil {
		return nil, fmt.Errorf("Explain: failed to parse goal %q: %w", goalStr, err)
	}
	if len(parsed.Clauses) == 0 {
		return nil, fmt.Errorf("Explain: no clauses parsed from goal %q", goalStr)
	}
	goal := parsed.Clauses[0].Head

	provOpts := provenance.Options{
		MaxProofs: opts.MaxProofs,
		MaxDepth:  opts.MaxDepth,
	}

	proofs, err := provenance.BuildFromRecording(k.proofRecorder, k.store, goal, provOpts)
	if err != nil {
		return nil, err
	}
	logging.Kernel("Explain: goal=%s produced %d proof(s)", goalStr, len(proofs))
	return proofs, nil
}

// ErrProvenanceDisabled is returned by Explain when no recorder is
// installed. Call EnableProvenance() and re-trigger evaluation first.
var ErrProvenanceDisabled = fmt.Errorf("provenance recording is disabled; call EnableProvenance() first")
