package session

import (
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// turnCompilationContext is the compile the task turn uses. Campaign-role
// prompts already copied CampaignPhase in the assembler; this path did not,
// so a phase-gated atom never reached the model doing the work.
func TestBuildCompilationContext_CopiesActiveCampaignPhase(t *testing.T) {
	e := &Executor{}
	active := types.WithSessionContext(t.Context(), &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  "  /recurse_fix  ",
		DreamMode:      true,
	})
	cc := turnCC(e, active, perception.Intent{Verb: "/fix"})
	if cc.CampaignPhase != "/recurse_fix" {
		t.Fatalf("request phase = %q, want /recurse_fix", cc.CampaignPhase)
	}
	if cc.OperationalMode != "/dream" {
		t.Fatalf("phase copy cleared dream mode: %q", cc.OperationalMode)
	}

	inactive := types.WithSessionContext(t.Context(), &types.SessionContext{
		CampaignPhase: "/recurse_fix",
	})
	if got := turnCC(e, inactive, perception.Intent{Verb: "/fix"}).CampaignPhase; got != "" {
		t.Fatalf("inactive campaign phase = %q", got)
	}
	blank := types.WithSessionContext(t.Context(), &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  "   ",
	})
	if got := turnCC(e, blank, perception.Intent{Verb: "/fix"}).CampaignPhase; got != "" {
		t.Fatalf("blank phase = %q", got)
	}

	named := types.WithSessionContext(t.Context(), &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  "Discovery",
	})
	if got := turnCC(e, named, perception.Intent{Verb: "/fix"}).CampaignPhase; got != "Discovery" {
		t.Fatalf("display-name phase = %q", got)
	}

	fallback := &Executor{sessionContext: &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  "/recurse_improve",
	}}
	if got := turnCC(fallback, t.Context(), perception.Intent{Verb: "/fix"}).CampaignPhase; got != "/recurse_improve" {
		t.Fatalf("executor fallback phase = %q", got)
	}

	// The request context wins over the executor's stored one.
	both := &Executor{sessionContext: &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  "/recurse_improve",
	}}
	req := types.WithSessionContext(t.Context(), &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  "/recurse_fix",
	})
	if got := turnCC(both, req, perception.Intent{Verb: "/fix"}).CampaignPhase; got != "/recurse_fix" {
		t.Fatalf("request phase lost to the executor's stored phase: %q", got)
	}
}
