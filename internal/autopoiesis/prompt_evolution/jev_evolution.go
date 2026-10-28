package prompt_evolution

import (
	"context"

	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
)

// The evolution trigger. A failure group buys a generated atom only when its
// failures are the agent's mistakes and share one cause a single instruction
// could prevent. Until 2026-10-28 any MinFailuresForEvolution failures with the
// same regex problem type bought one, so a timeout, a wrong file and a
// provider outage could teach one atom.
//
// Code facts first: a failure whose verdict says no mistake of the agent's
// (CategoryNoAgentMistake, jev_verdict.go) is not evidence for an atom, and
// the group must still reach MinFailuresForEvolution without it. Then Jev's
// shared-cause observation (learnjudge.JudgeSharedCause), under the
// atom_checks switch:
//
//	shares_one_cause          decision
//	yes                       generate
//	no                        not warranted: nothing is bought, the evidence
//	                          stays unconsumed so later failures can join it
//	unsettled                 not warranted (Jev's word is not enough to buy
//	                          a generation)
//	no answer (off, switched  the count rule decides, as before, and the log
//	off, failed, timed out)   says so

// agentMistakes drops failures that are no mistake of the agent's.
func agentMistakes(failures []*ExecutionRecord) []*ExecutionRecord {
	out := failures[:0:0]
	for _, f := range failures {
		if f != nil && f.Verdict != nil && f.Verdict.Category == CategoryNoAgentMistake {
			continue
		}
		out = append(out, f)
	}
	return out
}

// groupWarrantsEvolution applies the table to one group that met the count.
func (pe *PromptEvolver) groupWarrantsEvolution(ctx context.Context, group failureGroup, failures []*ExecutionRecord) bool {
	u := pe.jevFor()
	log := logging.Get(logging.CategoryAutopoiesis)
	if u == nil {
		return true
	}
	if !u.uses.AtomChecks {
		log.Info("Evolution trigger for %s: atom_checks is off; the failure count decides", group.key)
		return true
	}
	in := make([]learnjudge.GroupFailure, 0, len(failures))
	for _, f := range failures {
		if f == nil || f.Verdict == nil {
			continue
		}
		in = append(in, learnjudge.GroupFailure{Request: f.TaskRequest, Explanation: f.Verdict.Explanation, Category: string(f.Verdict.Category)})
	}
	p, err := learnjudge.JudgeSharedCause(ctx, u.asker, in)
	if err != nil {
		log.Warn("Evolution trigger for %s: no Jev answer, the failure count decides: %v", group.key, err)
		return true
	}
	switch u.read(p) {
	case readYes:
		logging.Autopoiesis("Evolution trigger for %s: %d failures share one cause (Jev %.2f); generating", group.key, len(in), p)
		return true
	case readNo:
		logging.Autopoiesis("Evolution trigger for %s: %d failures do not share one cause (Jev %.2f); not generating", group.key, len(in), p)
	default:
		logging.Autopoiesis("Evolution trigger for %s: shared cause unsettled (Jev %.2f); not generating", group.key, p)
	}
	return false
}
