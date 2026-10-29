package system

import (
	"codenerd/internal/core"
)

// RefusedUserAgentTool is one .nerd/agents.json tool entry the host does not
// know: the agent that named it and the name it declared. Boot refuses the
// whole agent (no config atom, no user_agent_declared_tool facts) and prints
// this pair on stderr; registerUserAgentConfigAtoms in factory.go is the
// caller that does.
type RefusedUserAgentTool struct {
	Agent string
	Tool  string
}

// RefusedUserAgentTools runs boot's own user-agent tool test over defs and
// returns every declared tool it refuses, in declaration order.
//
// It is the exported face of userAgentDeclaredTools, sharing that function
// rather than repeating it, so `nerd config check` and boot can never
// disagree about what "registered" means. The kernel must carry the same
// tool facts boot has when it registers agents (static tools from
// available_tools.json plus compiled tools from disk, asserted before
// initFinalExecutors): a bare kernel refuses every tool that lives outside
// a persona envelope, which boot would have accepted.
func RefusedUserAgentTools(kernel core.Kernel, defs []UserAgentDefinition) ([]RefusedUserAgentTool, error) {
	var out []RefusedUserAgentTool
	for _, def := range defs {
		_, refusals, err := userAgentDeclaredTools(kernel, def.Name, def.Tools)
		if err != nil {
			return nil, err
		}
		for _, r := range refusals {
			out = append(out, RefusedUserAgentTool{Agent: r.agent, Tool: r.tool})
		}
	}
	return out, nil
}
