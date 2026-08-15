package northstar

import (
	"fmt"
	"path/filepath"

	"codenerd/internal/logging"
)

// BuildCampaignObserver constructs the vision-guardian observer the campaign
// risk gate requires for protected surfaces.
//
// Why this exists: risk_scoring.go refuses to start any campaign whose targets
// touch a protected root unless the orchestrator has a Northstar observer. The
// setter for that field had ZERO callers repo-wide, so every such campaign was
// permanently blocked -- reproduced live with `nerd campaign start` against
// internal/core.
//
// Returns nil, loudly, when the guardian cannot be built. A nil observer keeps
// the gate closed, which is the safe direction: an inert observer would satisfy
// the gate while checking nothing, and a campaign that appears to run under a
// guardian that is not actually guarding is worse than one that refuses to
// start.
func BuildCampaignObserver(cwd string, llmClient LLMClient, kern KernelClient) *CampaignObserver {
	nerdDir := filepath.Join(cwd, ".nerd")

	// Shared guardian: a campaign that opened its own store cached its own copy
	// of GuardianState, so its periodic-check counter diverged from the chat
	// session's the moment either recorded a check.
	guardian, err := AcquireGuardian(nerdDir, DefaultGuardianConfig())
	if err != nil {
		logging.CampaignWarn("northstar store unavailable (%v); campaigns touching protected surfaces will be refused by the risk gate", err)
		fmt.Println("   ⚠ Northstar observer unavailable — campaigns on protected paths will be refused")
		return nil
	}

	if llmClient != nil {
		guardian.SetLLMClient(llmClient)
	}
	if kern != nil {
		guardian.SetParentKernel(kern)
	}
	// A guardian without a querier is exactly the inert observer this
	// function's doc comment warns about for module northstars -- it would
	// satisfy the risk gate while checking alignment against the project
	// vision alone and silently ignoring every module's declared purpose.
	if q, ok := kern.(FactQuerier); ok {
		guardian.SetQuerier(q)
	}
	if err := guardian.Initialize(); err != nil {
		_ = ReleaseGuardian(guardian)
		logging.CampaignWarn("northstar guardian failed to initialize (%v); campaigns touching protected surfaces will be refused by the risk gate", err)
		fmt.Println("   ⚠ Northstar observer failed to initialize — campaigns on protected paths will be refused")
		return nil
	}

	fmt.Println("   ✓ Northstar observer initialized")
	return NewCampaignObserver(guardian)
}
