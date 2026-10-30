package campaign

import (
	"fmt"
	"slices"

	"codenerd/internal/tools"
)

// acceptanceRuntime belongs to one measured invocation. It is neither
// serialized nor re-resolved from a possibly changed nerd.md on resume.
type acceptanceRuntime struct {
	workspace string
	gateID    string
	command   []string
	okExits   []int
	env       map[string]string
}

// acceptanceEnvironmentLocked requires o.mu for reading. Restored recurse
// campaigns (including legacy snapshots) cannot invent an empty overlay.
func (o *Orchestrator) acceptanceEnvironmentLocked() ([]string, error) {
	if o.campaign == nil || o.campaign.Acceptance == nil {
		return nil, nil
	}
	a := o.campaign.Acceptance
	if a.runtime == nil {
		if a.RuntimeContract || (o.campaign.Type == CampaignTypeRecurse && len(a.Command) > 0) {
			return nil, fmt.Errorf("recurse acceptance runtime contract unavailable; start a fresh measured recurse invocation")
		}
		return nil, nil
	}
	if a.GateID != a.runtime.gateID || !slices.Equal(a.Command, a.runtime.command) || !slices.Equal(a.OKExitCodes, a.runtime.okExits) {
		return nil, fmt.Errorf("recurse acceptance declaration differs from its measured invocation")
	}
	declared, err := tools.CanonicalWorkspaceRoot(a.runtime.workspace)
	if err != nil {
		return nil, fmt.Errorf("recurse acceptance workspace unavailable")
	}
	current, err := tools.CanonicalWorkspaceRoot(o.workspace)
	if err != nil || current != declared {
		return nil, fmt.Errorf("recurse acceptance workspace differs from its measured invocation")
	}
	return tools.CheckEnvironment(a.runtime.env), nil
}
