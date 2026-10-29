package session

import (
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
)

// neededExperts asks the kernel which shard agents a compile aimed at target
// needs (expert_needed, policy/shard_experts.mg), and returns their keys for
// CompilationContext.Experts. Every compile boundary asks -- the turn and each
// planned step -- so `nerd fix`, `nerd run`, chat turns, delegated subagents
// and campaign tasks all carry the expert knowledge their files call for, by
// the same derivation. The executor does not choose an expert; a kernel that
// cannot answer leaves the compile without one and says so in the log.
func (e *Executor) neededExperts(verb, target string) []string {
	if e.kernel == nil || strings.TrimSpace(target) == "" {
		return nil
	}
	sel, err := experts.Select(e.kernel, experts.Task{
		Verb:      verb,
		Targets:   []string{target},
		Workspace: e.workspaceForVerification(),
	})
	if err != nil {
		logging.Get(logging.CategorySession).Warn("expert selection for %s %s failed: %v", verb, target, err)
		return nil
	}
	agents := sel.Agents()
	if len(agents) > 0 {
		logging.Session("Shard experts needed for %s %s: %v", verb, target, agents)
	}
	return agents
}
