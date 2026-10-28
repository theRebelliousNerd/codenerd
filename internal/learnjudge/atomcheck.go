package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Atom check: before an evolved prompt atom is promoted into the agent's
// instructions, whether it only describes one past task and whether it goes
// against an atom already promoted. The decision is prompt_evolution's
// (jev_promotion.go).

// AtomText is an atom as the check reads it.
type AtomText struct {
	ID, Content string
}

// Question names; contradicts_N is one per existing atom.
const OneTaskOnly = "describes_one_task"

func contradictsName(i int) string { return fmt.Sprintf("contradicts_%d", i) }

// AtomCheck holds Jev's answers. Contradicts holds, per existing atom in
// order, the probability that following the candidate goes against it.
type AtomCheck struct {
	OneTaskOnly float64
	Contradicts []float64
	Model       string
}

const (
	atomTextLimit      = 6000
	atomMaxComparisons = 40
)

// AtomCheckRequest builds the state and the questions. Existing atoms beyond
// atomMaxComparisons are left out, and the state says how many.
func AtomCheckRequest(candidate AtomText, existing []AtomText) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{"agent": "codeNERD, a coding agent; an atom is one piece of its standing instructions"}
	putBounded(state, "candidate", strings.TrimSpace(candidate.Content), atomTextLimit)
	if len(existing) > atomMaxComparisons {
		state["existing_atoms_omitted"] = len(existing) - atomMaxComparisons
		existing = existing[:atomMaxComparisons]
	}
	qs := map[string]typesafe.Question{
		OneTaskOnly: typesafe.Noul{
			Instructions: "Is `candidate` about one specific past task rather than guidance for many tasks?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The instruction only makes sense for one task: it names one file, function, bug or request and says what to do about that one case.",
				False: "The instruction states a practice the agent can follow on many different tasks, even if it gives one example.",
			},
		},
	}
	for i, a := range existing {
		text, omitted := bounded(strings.TrimSpace(a.Content), atomTextLimit)
		atom := map[string]any{"text": text}
		if omitted > 0 {
			atom["text_omitted_bytes"] = omitted
		}
		qs[contradictsName(i)] = typesafe.Noul{
			Instructions: map[string]any{
				"question": "Would following `candidate` mean going against `atom`?",
				"atom":     atom,
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "The two instructions tell the agent to do opposite or incompatible things in the same situation.",
				False: "The two can both be followed: they agree, cover different situations, or are unrelated.",
			},
		}
	}
	return state, qs
}

// CheckAtom asks the questions. A failed call is an error, never a "no".
func CheckAtom(ctx context.Context, a Asker, candidate AtomText, existing []AtomText) (*AtomCheck, error) {
	if a == nil {
		return nil, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(candidate.Content) == "" {
		return nil, fmt.Errorf("learnjudge: atom %s has no content", candidate.ID)
	}
	state, qs := AtomCheckRequest(candidate, existing)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpAtomCheck), state, qs)
	if err != nil {
		return nil, err
	}
	c := &AtomCheck{Model: resp.Model}
	if c.OneTaskOnly, err = resp.Noul(OneTaskOnly); err != nil {
		return nil, err
	}
	for i := range existing {
		if i >= atomMaxComparisons {
			break
		}
		p, err := resp.Noul(contradictsName(i))
		if err != nil {
			return nil, err
		}
		c.Contradicts = append(c.Contradicts, p)
	}
	return c, nil
}
