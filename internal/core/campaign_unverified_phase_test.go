package core

import (
	"slices"
	"testing"

	"codenerd/internal/types"
)

// External audit N03 (2026-09-19): a phase whose checkpoint never passed within
// its attempts closed /completed, so the phases built on it became eligible.
// It closes /unverified now, and the shipped corpus must read that as
// incomplete: its hard dependent is not eligible, and with nothing else to run
// the campaign is blocked on the unverified phase by name -- not on the generic
// "no eligible phases" -- which is the reason the orchestrator records when it
// fails the campaign.
func TestCampaignPolicy_AnUnverifiedPhaseBlocksItsDependentsByName(t *testing.T) {
	for _, tc := range []struct {
		status       string
		wantEligible bool
		wantBlocked  []string
	}{
		{status: "/unverified", wantEligible: false, wantBlocked: []string{"/phase_unverified"}},
		{status: "/completed", wantEligible: true},
	} {
		t.Run(tc.status, func(t *testing.T) {
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("the shipped corpus must load: %v", err)
			}
			for _, f := range []types.Fact{
				{Predicate: "campaign", Args: []any{"c1", types.MangleAtom("/feature"), "campaign", "src", types.MangleAtom("/active")}},
				{Predicate: "campaign_phase", Args: []any{"p1", "c1", "build", 1, types.MangleAtom(tc.status), "ctx"}},
				{Predicate: "campaign_phase", Args: []any{"p2", "c1", "ship", 2, types.MangleAtom("/pending"), "ctx"}},
				{Predicate: "phase_dependency", Args: []any{"p2", "p1", types.MangleAtom("/hard")}},
			} {
				if aerr := k.Assert(f); aerr != nil {
					t.Fatalf("assert %s%v: %v", f.Predicate, f.Args, aerr)
				}
			}

			eligible, err := k.Query("phase_eligible")
			if err != nil {
				t.Fatalf("query phase_eligible: %v", err)
			}
			gotEligible := slices.ContainsFunc(eligible, func(f types.Fact) bool { return types.ExtractString(f.Args[0]) == "p2" })
			if gotEligible != tc.wantEligible {
				t.Fatalf("p2 eligible = %t with p1 %s, want %t (phase_eligible = %v)", gotEligible, tc.status, tc.wantEligible, eligible)
			}

			blocked, err := k.Query("campaign_blocked")
			if err != nil {
				t.Fatalf("query campaign_blocked: %v", err)
			}
			var reasons []string
			for _, f := range blocked {
				reasons = append(reasons, types.ExtractString(f.Args[1]))
			}
			slices.Sort(reasons)
			if !slices.Equal(reasons, tc.wantBlocked) {
				t.Fatalf("campaign_blocked reasons = %v with p1 %s, want %v", reasons, tc.status, tc.wantBlocked)
			}
		})
	}
}
