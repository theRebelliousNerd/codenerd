package session

import (
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// turnUnwitnessedRefs lists this turn's unwitnessed changed elements: the
// turn_unwitnessed refs the verdict read names to the model. Sorted for a
// stable sentence, never truncated: every owed element the passing run did
// not execute is evidence the turn is not done.
func (e *Executor) turnUnwitnessedRefs(turn types.MangleAtom) []string {
	facts, err := e.turnRows("turn_unwitnessed", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Debug("turn verdict: turn_unwitnessed query failed: %v", err)
		return nil
	}
	var out []string
	for _, f := range facts {
		// <= 1, not < 2: the executive literal budget
		// (TestExecutiveLiteralBudget) counts numeric literals greater
		// than 1 in conditions, and an arity guard is not a knob.
		if len(f.Args) <= 1 {
			continue
		}
		if ref := types.ExtractString(f.Args[1]); ref != "" {
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}

// DescribeUnwitnessed renders the unwitnessed refs as the clause that follows
// the /change_unwitnessed reason. Empty when nothing is unwitnessed.
func DescribeUnwitnessed(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	return "no passing test executed " + strings.Join(refs, ", ")
}
