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

	store, err := NewStore(nerdDir)
	if err != nil {
		logging.CampaignWarn("northstar store unavailable (%v); campaigns touching protected surfaces will be refused by the risk gate", err)
		fmt.Println("   ⚠ Northstar observer unavailable — campaigns on protected paths will be refused")
		return nil
	}

	guardian := NewGuardian(store, DefaultGuardianConfig())
	if llmClient != nil {
		guardian.SetLLMClient(llmClient)
	}
	if kern != nil {
		guardian.SetParentKernel(kern)
	}
	if err := guardian.Initialize(); err != nil {
		logging.CampaignWarn("northstar guardian failed to initialize (%v); campaigns touching protected surfaces will be refused by the risk gate", err)
		fmt.Println("   ⚠ Northstar observer failed to initialize — campaigns on protected paths will be refused")
		return nil
	}

	fmt.Println("   ✓ Northstar observer initialized")
	return NewCampaignObserver(guardian)
}
