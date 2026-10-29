package prompt_evolution

import (
	"context"
	"fmt"
	"sort"

	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
)

// The promotion check (atom_checks): before auto-promotion moves an evolved
// atom into the agent's instructions, Jev reads whether the atom only
// describes one past task and whether following it goes against an atom
// already promoted for the same shard (learnjudge.CheckAtom). The usage
// threshold (ShouldPromote) still has to be met; this can only hold an atom
// back.
//
//	describes_one_task  contradicts any promoted atom   decision
//	yes                 any                             held: stays pending for the operator
//	any                 yes                             held
//	unsettled           not yes                         held (a change to the agent's own
//	not yes             unsettled on one                     instructions waits for a clear reading)
//	no                  no on every one                 promoted
//	no answer (Jev off, atom_checks off, failed)        promoted on the threshold, as before; logged
//
// Promotion at record time waits for the cycle while the check is on, so no
// promotion skips it. An operator's PromoteAtom is the operator's decision and
// is not checked.

// promotionCheckOn reports whether auto-promotion must pass the Jev check.
func (pe *PromptEvolver) promotionCheckOn() bool {
	u := pe.jevFor()
	return u != nil && u.uses.AtomChecks
}

// promotionHolds checks every auto-promotion candidate and returns the ones
// held back, with why. No lock is held across a Jev call.
func (pe *PromptEvolver) promotionHolds(ctx context.Context) map[string]string {
	u := pe.jevFor()
	if u == nil {
		return nil
	}
	log := logging.Get(logging.CategoryAutopoiesis)
	if !u.uses.AtomChecks {
		log.Info("Promotion check: atom_checks is off; the usage threshold decides")
		return nil
	}
	type cand struct {
		id, shard string
		text      learnjudge.AtomText
	}
	var cands []cand
	promoted := map[string][]learnjudge.AtomText{}
	pe.mu.RLock()
	for id, ga := range pe.evolvedAtoms {
		if ga == nil || ga.Atom == nil {
			continue
		}
		text := learnjudge.AtomText{ID: id, Content: ga.Atom.Content}
		if !ga.PromotedAt.IsZero() {
			promoted[ga.ShardType] = append(promoted[ga.ShardType], text)
			continue
		}
		if ga.ShouldPromote(pe.config.ConfidenceThreshold) {
			cands = append(cands, cand{id: id, shard: ga.ShardType, text: text})
		}
	}
	pe.mu.RUnlock()
	sort.Slice(cands, func(i, j int) bool { return cands[i].id < cands[j].id })

	held := map[string]string{}
	for _, c := range cands {
		existing := promoted[c.shard]
		sort.Slice(existing, func(i, j int) bool { return existing[i].ID < existing[j].ID })
		check, err := learnjudge.CheckAtom(ctx, u.asker, c.text, existing)
		if err != nil {
			log.Warn("Promotion check for %s: no Jev answer, the usage threshold decides: %v", c.id, err)
			continue
		}
		if why := promotionHoldReason(check, existing, u); why != "" {
			held[c.id] = why
			logging.Autopoiesis("Promotion check: %s held for the operator: %s", c.id, why)
		}
	}
	return held
}

// promotionHoldReason applies the table; "" promotes.
func promotionHoldReason(c *learnjudge.AtomCheck, existing []learnjudge.AtomText, u *jevUse) string {
	switch u.read(c.OneTaskOnly) {
	case readYes:
		return "it describes one past task, not guidance for many (Jev " + pct(c.OneTaskOnly) + ")"
	case readUnsettled:
		return "whether it describes one past task is unsettled (Jev " + pct(c.OneTaskOnly) + ")"
	}
	for i, p := range c.Contradicts {
		if i >= len(existing) {
			break
		}
		switch u.read(p) {
		case readYes:
			return "it goes against promoted atom " + existing[i].ID + " (Jev " + pct(p) + ")"
		case readUnsettled:
			return "whether it goes against promoted atom " + existing[i].ID + " is unsettled (Jev " + pct(p) + ")"
		}
	}
	return ""
}

func pct(p float64) string { return fmt.Sprintf("%.2f", p) }
