package prompt_evolution

// The evolution trigger. A failure group buys a generated atom only when its
// failures are the agent's mistakes and share one cause a single instruction
// could prevent. Until 2026-10-28 any MinFailuresForEvolution failures with the
// same regex problem type bought one, so a timeout, a wrong file and a
// provider outage could teach one atom.
//
// Code facts first: a failure whose verdict says no mistake of the agent's
// (CategoryNoAgentMistake, jev_verdict.go) is not evidence for an atom, and
// the group must still reach MinFailuresForEvolution without it. Then Jev's
// shared-cause observation (screenCohort), under the
// atom_checks switch:
//
//	shares_one_cause          decision
//	yes                       generate
//	no                        not warranted: no generation; the evidence
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
