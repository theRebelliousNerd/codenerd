package prompt

import (
	"fmt"
	"sort"
)

// =============================================================================
// SECTIONED ATOMS AND DEPENDENCY CLOSURE
// =============================================================================
//
// One instruction is often authored as a head atom plus ordered sections
// (section_of / section_order): the head states the rule, the sections carry
// its protocol, examples and edge cases. Sections stay optional — a tight
// budget keeps the head and sheds sections from the tail — but they are never
// selected on their own and never left behind by their head:
//
//   - expandSections: a selected head brings its sections, ordered, each
//     scored just below the head so fitting admits them head-first and
//     shedding removes the tail first. A section selected on its own (a
//     vector hit) brings its head, or is dropped when the head is not
//     eligible here. Sections inherit the head's admission: they carry the
//     head's selectors, so a veto that binds the head binds them too.
//   - closeDependencies: after fitting and after post-render shedding, any
//     optional atom whose prerequisite is gone is removed, transitively. A
//     mandatory atom that would lose a prerequisite is an infeasible compile,
//     reported, never a silently incomplete prompt
//     (Docs/audits/jit-dependency-closed-budgeting.md).

// sectionScoreStep is how far below its head (as a fraction of the head's
// score) each successive section ranks. Small enough to stay above every
// other atom the head outranks; large enough to order the sections.
const sectionScoreStep = 1e-4

// expandSections completes every selected head with its sections and every
// selected section with its head.
func expandSections(selected []*ScoredAtom, candidates []*PromptAtom, cc *CompilationContext) []*ScoredAtom {
	byID := make(map[string]*PromptAtom, len(candidates))
	sections := make(map[string][]*PromptAtom)
	for _, a := range candidates {
		if a == nil {
			continue
		}
		byID[a.ID] = a
		if a.SectionOf != "" {
			sections[a.SectionOf] = append(sections[a.SectionOf], a)
		}
	}
	if len(sections) == 0 {
		return selected
	}
	for head := range sections {
		list := sections[head]
		sort.SliceStable(list, func(i, j int) bool { return list[i].SectionOrder < list[j].SectionOrder })
	}

	chosen := make(map[string]*ScoredAtom, len(selected))
	out := make([]*ScoredAtom, 0, len(selected))
	for _, s := range selected {
		if s == nil || s.Atom == nil || chosen[s.Atom.ID] != nil {
			continue
		}
		chosen[s.Atom.ID] = s
		out = append(out, s)
	}

	eligible := func(a *PromptAtom) bool { return a != nil && (cc == nil || a.MatchesContext(cc)) }

	// A section selected on its own brings its head, or goes.
	kept := out[:0]
	var heads []*ScoredAtom
	for _, s := range out {
		head := s.Atom.SectionOf
		if head == "" || chosen[head] != nil {
			kept = append(kept, s)
			continue
		}
		if h := byID[head]; eligible(h) {
			hs := &ScoredAtom{Atom: h, LogicScore: s.LogicScore, VectorScore: s.VectorScore,
				Combined: s.Combined, Source: s.Source, SelectionReason: "head of selected section " + s.Atom.ID}
			chosen[head] = hs
			heads = append(heads, hs)
			kept = append(kept, s)
			continue
		}
		delete(chosen, s.Atom.ID)
	}
	out = append(kept, heads...)

	// Every selected head brings its sections, ranked just below it.
	n := len(out)
	for i := 0; i < n; i++ {
		head := out[i]
		for _, sec := range sections[head.Atom.ID] {
			if chosen[sec.ID] != nil || !eligible(sec) {
				continue
			}
			score := head.Combined * (1 - sectionScoreStep*float64(sec.SectionOrder))
			ss := &ScoredAtom{Atom: sec, LogicScore: head.LogicScore, VectorScore: head.VectorScore,
				Combined: score, Source: head.Source, SelectionReason: "section of " + head.Atom.ID}
			chosen[sec.ID] = ss
			out = append(out, ss)
		}
	}
	return out
}

// errInfeasibleClosure reports a mandatory atom whose prerequisite did not
// survive fitting: the compile cannot honour its own required set.
type errInfeasibleClosure struct {
	Atom, Missing string
}

func (e errInfeasibleClosure) Error() string {
	return fmt.Sprintf("mandatory atom %q lost its prerequisite %q to the budget; the required prompt does not fit", e.Atom, e.Missing)
}

// closeDependencies removes every optional atom whose prerequisite is not in
// the set, transitively, in one worklist pass over reverse edges. It returns
// the surviving atoms in their original order and the IDs removed.
func closeDependencies(atoms []*OrderedAtom) ([]*OrderedAtom, []string, error) {
	present := make(map[string]*OrderedAtom, len(atoms))
	for _, oa := range atoms {
		if oa != nil && oa.Atom != nil {
			present[oa.Atom.ID] = oa
		}
	}
	dependents := make(map[string][]string)
	var queue []string
	for id, oa := range present {
		for _, dep := range oa.Atom.DependsOn {
			dependents[dep] = append(dependents[dep], id)
			if present[dep] == nil {
				queue = append(queue, id)
			}
		}
	}
	removed := make(map[string]bool)
	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		oa := present[id]
		if oa == nil || removed[id] {
			continue
		}
		if oa.Atom.IsMandatory {
			for _, dep := range oa.Atom.DependsOn {
				if present[dep] == nil || removed[dep] {
					return nil, nil, errInfeasibleClosure{Atom: id, Missing: dep}
				}
			}
		}
		removed[id] = true
		order = append(order, id)
		queue = append(queue, dependents[id]...)
	}
	if len(removed) == 0 {
		return atoms, nil, nil
	}
	kept := make([]*OrderedAtom, 0, len(atoms)-len(removed))
	for _, oa := range atoms {
		if oa != nil && oa.Atom != nil && removed[oa.Atom.ID] {
			continue
		}
		kept = append(kept, oa)
	}
	return kept, order, nil
}
