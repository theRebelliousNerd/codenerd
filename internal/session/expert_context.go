package session

import (
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
)

// selectExperts asks the kernel which shard agents a turn aimed at target
// needs (expert_needed, policy/shard_experts.mg) and which of them it must
// consult before the turn (expert_spawn). Every compile boundary asks -- the
// turn and each planned step -- so `nerd fix`, `nerd run`, chat turns,
// delegated subagents and campaign tasks all carry the expert knowledge their
// files call for, by the same derivation. The executor does not choose an
// expert; a kernel that cannot answer leaves the turn without one and says so
// in the log.
func (e *Executor) selectExperts(verb, target string) experts.Selection {
	if e.kernel == nil || strings.TrimSpace(target) == "" {
		return experts.Selection{}
	}
	sel, err := experts.Select(e.kernel, experts.Task{
		Verb:      verb,
		Targets:   []string{target},
		Workspace: e.workspaceForVerification(),
		Speaker:   turnSpeakerAgent(verb),
	})
	if err != nil {
		logging.Get(logging.CategorySession).Warn("expert selection for %s %s failed: %v", verb, target, err)
		return experts.Selection{}
	}
	if agents := sel.Agents(); len(agents) > 0 {
		logging.Session("Shard experts needed for %s %s: %v", verb, target, agents)
	}
	return sel
}

// neededExperts is selectExperts' agent keys, for CompilationContext.Experts.
func (e *Executor) neededExperts(verb, target string) []string {
	return e.selectExperts(verb, target).Agents()
}

// turnSpeakerAgent is the shard agent a verb runs the turn as -- the same
// test buildCompilationContext applies to choose the persona: a verb the
// taxonomy does not know that addresses a user agent ("/consult/goexpert",
// "/goexpert") -- or "" for a built-in persona's verb.
func turnSpeakerAgent(verb string) string {
	if perception.GetShardTypeForVerb(verb) != "" {
		return ""
	}
	return UserAgentFromIntentVerb(verb)
}
