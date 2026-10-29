package core

import "testing"

// Documentation evidence is host-measured (Docs/audits/documentation-evidence-contract.md):
// a model cannot witness that a document's citation resolves, that a
// requirement is realized, or that a reviewer accepted a relation. The
// policy is an allowlist, so these stay out until someone names them; this
// pins that nobody has, through the actual envelope filter.
func TestModelObservationPolicyRefusesDocumentEvidence(t *testing.T) {
	updates := []string{
		`doc_reference("ref1", "Docs/a.md", 3, /planned_seam, "internal/x.go").`,
		`doc_resolution("ref1", /declared, "internal/x.Run").`,
		`doc_scan_manifest("gen1", /complete, /workspace).`,
		`unit_realizes("internal/x", "spec1").`,
		`relation_disposition("claim1", /accepted, "me").`,
		`acceptance_receipt("spec1", /passed).`,
	}
	facts, blocked := FilterMangleUpdates(nil, updates, ModelObservationPolicy())
	if len(facts) != 0 {
		t.Fatalf("model envelope admitted document evidence: %+v", facts)
	}
	if len(blocked) != len(updates) {
		t.Fatalf("blocked %d of %d: %+v", len(blocked), len(updates), blocked)
	}
}
