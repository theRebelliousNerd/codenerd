package campaign

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/gates"
	"codenerd/internal/types"
)

// recursePolicy is the loop's side of policy/recurse.mg: it asserts what the
// gates measured and reads back what the kernel decided. It decides nothing
// itself.
type recursePolicy struct {
	k core.Kernel
}

// Ratchet verdicts (recurse_ratchet/2).
const (
	ratchetKeep   = "/keep"
	ratchetRevert = "/revert"
	ratchetRefuse = "/refuse"
)

// Attempt outcomes (recurse_attempt/5).
const (
	outcomeKept       = "/kept"
	outcomeReverted   = "/reverted"
	outcomeRefused    = "/refused"
	outcomeUnverified = "/unverified"
)

// Gate verdicts as the ratchet compares them.
const (
	verdictPass       = "/pass"
	verdictFail       = "/fail"
	verdictUnverified = "/unverified"
)

func kindAtom(k gates.Kind) string { return "/" + string(k) }

// beginVisit starts a node's visit: nothing has been attempted in it yet.
func (p recursePolicy) beginVisit() error {
	return p.k.Retract("recurse_visit_attempted")
}

// attempted marks a finding as attempted in the current visit.
func (p recursePolicy) attempted(id string) error {
	return p.k.Assert(core.Fact{Predicate: "recurse_visit_attempted", Args: []interface{}{id}})
}

// visit replaces the visit and the open findings with a fresh measurement.
// regressions are the IDs that were not open when the pass began.
func (p recursePolicy) visit(node string, open []gates.Finding, regressions map[string]bool) error {
	if err := p.k.RemoveFactsByPredicateSet(map[string]struct{}{
		"recurse_visit": {}, "recurse_finding": {}, "recurse_regression": {},
	}); err != nil {
		return fmt.Errorf("recurse: clear visit: %w", err)
	}
	facts := []core.Fact{{Predicate: "recurse_visit", Args: []interface{}{node}}}
	for _, f := range open {
		facts = append(facts, core.Fact{Predicate: "recurse_finding", Args: []interface{}{
			f.ID, f.Node, f.Gate, kindAtom(f.Kind), f.Target, f.Signature,
		}})
		if regressions[f.ID] {
			facts = append(facts, core.Fact{Predicate: "recurse_regression", Args: []interface{}{f.ID}})
		}
	}
	if err := p.k.AssertBatch(facts); err != nil {
		return fmt.Errorf("recurse: assert visit: %w", err)
	}
	return nil
}

// next returns the finding the kernel picks for the visit, or "" when it
// derives none. Several at the best rank resolve to the lowest ID, so the pick
// is reproducible.
func (p recursePolicy) next() (string, error) {
	ids, err := p.derivedColumn("recurse_next", 0)
	if err != nil || len(ids) == 0 {
		return "", err
	}
	return ids[0], nil
}

// attempt records one finished attempt. A kept attempt also marks its node as
// changed, which lifts stalls on the node's other findings.
func (p recursePolicy) attempt(findingID, node string, cycle int, outcome, signature string) error {
	facts := []core.Fact{{Predicate: "recurse_attempt", Args: []interface{}{findingID, node, cycle, outcome, signature}}}
	if outcome == outcomeKept {
		facts = append(facts, core.Fact{Predicate: "recurse_node_kept", Args: []interface{}{node, cycle}})
	}
	return p.k.AssertBatch(facts)
}

// ratchetGate is one gate's before and after for an attempt.
type ratchetGate struct {
	Gate                    string
	Before, After           string
	BeforeCount, AfterCount int
}

// ratchetInput is what re-measuring an attempt found.
type ratchetInput struct {
	Cycle     int
	Gates     []ratchetGate
	TargetID  string
	TargetOK  bool // the target finding is no longer open
	Changed   bool // the attempt changed the tree at all
	Forbidden []string
}

// ratchet asserts the re-measurement and returns the kernel's verdict. Exactly
// one verdict is derived for a cycle; anything else is a policy fault and is
// returned as an error, never guessed at.
func (p recursePolicy) ratchet(in ratchetInput) (string, error) {
	changed := "/no"
	if in.Changed {
		changed = "/yes"
	}
	target := "/open"
	if in.TargetOK {
		target = "/resolved"
	}
	facts := []core.Fact{
		{Predicate: "recurse_ratchet_changed", Args: []interface{}{in.Cycle, changed}},
		{Predicate: "recurse_ratchet_target", Args: []interface{}{in.Cycle, in.TargetID, target}},
	}
	for _, g := range in.Gates {
		facts = append(facts, core.Fact{Predicate: "recurse_ratchet_gate", Args: []interface{}{
			in.Cycle, g.Gate, g.Before, g.After, g.BeforeCount, g.AfterCount,
		}})
	}
	for _, path := range in.Forbidden {
		facts = append(facts, core.Fact{Predicate: "recurse_ratchet_forbidden", Args: []interface{}{in.Cycle, path}})
	}
	if err := p.k.AssertBatch(facts); err != nil {
		return "", fmt.Errorf("recurse: assert ratchet: %w", err)
	}
	rows, err := p.k.Query("recurse_ratchet")
	if err != nil {
		return "", fmt.Errorf("recurse: query ratchet: %w", err)
	}
	var verdicts []string
	for _, f := range rows {
		if len(f.Args) == 2 && argInt(f.Args[0]) == in.Cycle {
			verdicts = append(verdicts, types.ExtractString(f.Args[1]))
		}
	}
	if len(verdicts) != 1 {
		return "", fmt.Errorf("recurse: the kernel derived %d ratchet verdicts for cycle %d (%v); the policy must decide one", len(verdicts), in.Cycle, verdicts)
	}
	return verdicts[0], nil
}

// worseGates returns the gates the kernel judged worse after cycle's attempt.
func (p recursePolicy) worseGates(cycle int) ([]string, error) {
	rows, err := p.k.Query("recurse_gate_worse")
	if err != nil {
		return nil, fmt.Errorf("recurse: query recurse_gate_worse: %w", err)
	}
	var out []string
	for _, f := range rows {
		if len(f.Args) == 2 && argInt(f.Args[0]) == cycle {
			out = append(out, types.ExtractString(f.Args[1]))
		}
	}
	sort.Strings(out)
	return out, nil
}

// ratchetKinds returns the workspace-wide gate kinds every attempt is
// re-measured against.
func (p recursePolicy) ratchetKinds() (map[gates.Kind]bool, error) {
	names, err := p.derivedColumn("recurse_ratchet_kind", 0)
	if err != nil {
		return nil, err
	}
	out := map[gates.Kind]bool{}
	for _, n := range names {
		out[gates.Kind(strings.TrimPrefix(n, "/"))] = true
	}
	return out, nil
}

// stalled and refused list the findings the loop has stopped attempting.
func (p recursePolicy) stalled() ([]string, error) { return p.derivedColumn("finding_stalled", 0) }
func (p recursePolicy) refused() ([]string, error) { return p.derivedColumn("finding_refused", 0) }

// derivedColumn returns the distinct values of one argument of a derived predicate,
// sorted.
func (p recursePolicy) derivedColumn(predicate string, i int) ([]string, error) {
	rows, err := p.k.Query(predicate)
	if err != nil {
		return nil, fmt.Errorf("recurse: query %s: %w", predicate, err)
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range rows {
		if len(f.Args) <= i {
			continue
		}
		v := types.ExtractString(f.Args[i])
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, nil
}

func argInt(a interface{}) int {
	switch v := a.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return -1
}
