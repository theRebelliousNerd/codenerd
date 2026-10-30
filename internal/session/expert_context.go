package session

import (
	"context"
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
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
		Speaker:   turnUserAgent(verb),
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

// turnCompilationContext is the turn's compile context and the shard experts
// the kernel selected for its target, asked once: the compile carries their
// knowledge, and the consultations the kernel derives run from the same
// selection.
func (e *Executor) turnCompilationContext(ctx context.Context, intent perception.Intent) (*prompt.CompilationContext, experts.Selection) {
	sel := e.selectExperts(intent.Verb, intent.Target)
	return e.buildCompilationContextWith(ctx, intent, sel.Agents()), sel
}

// turnUserAgent is the shard agent a verb runs the turn as: a verb the
// taxonomy does not define that addresses a user agent ("/consult/goexpert",
// "/goexpert"), or "" for every taxonomy verb. buildCompilationContextWith and
// the expert selection both ask it. It used to test only for a shard type,
// so a taxonomy verb whose shard type is /none (/explain, /configure,
// /assault, /stats, /help, /greet, /knowledge) ran as a phantom agent named
// after the verb.
func turnUserAgent(verb string) string {
	if perception.IsTaxonomyVerb(verb) {
		return ""
	}
	return UserAgentFromIntentVerb(verb)
}
