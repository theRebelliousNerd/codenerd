package chat

import (
	"fmt"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// systemResultBaseline is the set of routing_result/execution_result rows that
// already stood when a turn began its wait for system-action outcomes.
//
// The turn used to remember a COUNT of each predicate and later treated
// rows[count:] as "new". That is positional, and the rows are not an
// append-only log: chat clears both predicates after perception
// (clearStaleKernelFacts), so a previous turn's one row became baseline 1 and
// the new turn's first result was sliced away -- and a result the kernel pruned
// or re-ordered shifted every index after it. Membership is by row identity
// instead: a row is the turn's own when it was not standing in the baseline.
// The baseline is taken after the clear, so the clear cannot hide a result.
type systemResultBaseline struct {
	seen map[string]struct{}
}

// factIdentity is a row's identity: the predicate and every argument. An
// action id is unique per dispatch, so two rows for different actions never
// collide, and a repeated emission of the same outcome is the same row.
func factIdentity(f core.Fact) string {
	return fmt.Sprintf("%s%v", f.Predicate, f.Args)
}

// snapshotSystemResults records the rows standing now.
func (m Model) snapshotSystemResults() systemResultBaseline {
	base := systemResultBaseline{seen: map[string]struct{}{}}
	if m.kernel == nil {
		return base
	}
	for _, predicate := range []string{"routing_result", "execution_result"} {
		facts, err := m.kernel.Query(predicate)
		if err != nil {
			continue
		}
		for _, f := range facts {
			base.seen[factIdentity(f)] = struct{}{}
		}
	}
	return base
}

// newSystemResults returns the rows of predicate that were not in the
// baseline, in query order.
func (m Model) newSystemResults(predicate string, base systemResultBaseline) []core.Fact {
	if m.kernel == nil {
		return nil
	}
	facts, err := m.kernel.Query(predicate)
	if err != nil {
		return nil
	}
	var fresh []core.Fact
	for _, f := range facts {
		if _, stood := base.seen[factIdentity(f)]; stood {
			continue
		}
		fresh = append(fresh, f)
	}
	return fresh
}

// unroutedActions names the actions whose routing failed and which produced no
// execution row: the action was never dispatched, so nothing happened, and the
// turn must say so rather than answer as if a read or a run had occurred. A
// routing failure is a typed outcome (no handler, rate limit, error), not the
// absence of a result. routing_result is (ActionID, Status, Details, Time).
func unroutedActions(routing, exec []core.Fact) []string {
	executed := map[string]struct{}{}
	for _, f := range exec {
		if len(f.Args) > 0 {
			executed[types.ExtractString(f.Args[0])] = struct{}{}
		}
	}
	var notes []string
	for _, f := range routing {
		if types.ArgString(f, 1) != "/failure" {
			continue
		}
		id := types.ExtractString(f.Args[0])
		if _, ran := executed[id]; ran {
			continue
		}
		notes = append(notes, fmt.Sprintf("System action %s was not dispatched (%v); no result was produced for it.", id, types.ArgString(f, 2)))
	}
	return notes
}
