package main

import "testing"

// The decomposer gave DecomposeRequest.UserHints to the planner, but no front
// end supplied them: read and never written. nerd campaign start --hint
// supplies them.
func TestCampaignHintFlag_ReachesTheDecomposer(t *testing.T) {
	if campaignStartCmd.Flags().Lookup("hint") == nil {
		t.Fatal("nerd campaign start has no --hint flag")
	}
	req := decomposeRequest("add rate limiting", nil, "feature", 1000, []string{"keep the public API stable"})
	if len(req.UserHints) != 1 || req.UserHints[0] != "keep the public API stable" {
		t.Fatalf("UserHints = %v", req.UserHints)
	}
}
