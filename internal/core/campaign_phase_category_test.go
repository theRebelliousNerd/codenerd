package core

import "testing"

// TestPhaseCategory_RealPhaseIDShape_DerivesPrecedence reproduces the live
// campaign a19dd99f (2026-09-04): every phase carried a canonical category
// yet the kernel raised validation_error(_, /topology, "missing_category")
// for all four. The earlier fixture used phase IDs without a leading slash;
// the decomposer emits "/phase_<id>_<n>", which Fact.ToAtom promotes to a
// name constant while campaign_phase/phase_category declare that column
// /string. This test asserts the facts exactly as campaign.Phase.ToFacts
// emits them (Go strings) and checks the derivation chain end to end.
func TestPhaseCategory_RealPhaseIDShape_DerivesPrecedence(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	facts := []Fact{
		{Predicate: "campaign_phase", Args: []any{"/phase_abc_0", "/campaign_abc", "Reconnaissance", int64(1), "/pending", "default"}},
		{Predicate: "phase_category", Args: []any{"/phase_abc_0", "/scaffold"}},
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	for _, q := range []string{"campaign_phase", "phase_category", "phase_precedence", "has_phase_category"} {
		rows, qerr := k.Query(q)
		if qerr != nil {
			t.Fatalf("Query(%s): %v", q, qerr)
		}
		t.Logf("%s -> %d rows: %v", q, len(rows), rows)
	}
	precedence, _ := k.Query("phase_precedence")
	if len(precedence) == 0 {
		t.Errorf("phase_precedence did not derive for a canonical category with a real phase ID")
	}
	errs, _ := k.Query("validation_error")
	for _, e := range errs {
		if len(e.Args) >= 3 && toString(e.Args[2]) == "missing_category" {
			t.Errorf("bogus missing_category for %v", e.Args[0])
		}
	}
}
