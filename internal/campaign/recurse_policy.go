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
//
// It also bounds what a forever run leaves in the kernel. A finding stalls
// when its last two attempts ended the same way with no kept change to its
// node since; so per finding only the previous and the latest attempt can
// matter, and a kept change retires every attempt on its node. Nothing else
// is kept, however many passes the loop runs.
type recursePolicy struct {
	k core.Kernel

	pairs  map[string]*attemptPair    // finding -> its previous and latest attempt
	byNode map[string]map[string]bool // node -> the findings attempted on it
	kept   map[string]core.Fact       // node -> its latest recurse_node_kept
}

type attemptPair struct {
	previous, latest *core.Fact
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
func (p *recursePolicy) beginVisit() error {
	return p.k.Retract("recurse_visit_attempted")
}

// attempted marks a finding as attempted in the current visit.
func (p *recursePolicy) attempted(id string) error {
	return p.k.Assert(core.Fact{Predicate: "recurse_visit_attempted", Args: []interface{}{id}})
}

// visit replaces the visit and the open findings with a fresh measurement.
// regressions are the IDs that were not open when the pass began.
func (p *recursePolicy) visit(node string, open []gates.Finding, regressions map[string]bool) error {
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
func (p *recursePolicy) next() (string, error) {
	ids, err := p.derivedColumn("recurse_next", 0)
	if err != nil || len(ids) == 0 {
		return "", err
	}
	return ids[0], nil
}

// attempt records one finished attempt on a finding. A kept attempt also
// marks its node as changed, which lifts stalls on the node's other findings.
func (p *recursePolicy) attempt(findingID, node string, cycle int, outcome, signature string) error {
	if outcome == outcomeKept {
		return p.nodeKept(node, cycle)
	}
	f := core.Fact{Predicate: "recurse_attempt", Args: []interface{}{findingID, node, cycle, outcome, signature}}
	if outcome == outcomeRefused {
		// A refusal is the owner's to lift; no kept change retires it.
		return p.k.AssertBatch([]core.Fact{f})
	}
	if p.pairs == nil {
		p.pairs, p.byNode = map[string]*attemptPair{}, map[string]map[string]bool{}
	}
	key := findingID
	pair := p.pairs[key]
	var retract []core.Fact
	if pair == nil {
		pair = &attemptPair{}
		p.pairs[key] = pair
		if p.byNode[node] == nil {
			p.byNode[node] = map[string]bool{}
		}
		p.byNode[node][key] = true
	}
	if pair.previous != nil {
		retract = append(retract, *pair.previous)
	}
	pair.previous, pair.latest = pair.latest, &f
	if len(retract) > 0 {
		if err := p.k.RetractExactFactsBatch(retract); err != nil {
			return fmt.Errorf("recurse: retire attempt: %w", err)
		}
	}
	return p.k.AssertBatch([]core.Fact{f})
}

// nodeKept records a kept change to node, retiring the node's earlier
// attempts: none of them can stall a finding any more.
func (p *recursePolicy) nodeKept(node string, cycle int) error {
	if p.kept == nil {
		p.kept = map[string]core.Fact{}
	}
	var retract []core.Fact
	if prev, ok := p.kept[node]; ok {
		retract = append(retract, prev)
	}
	for key := range p.byNode[node] {
		if pair := p.pairs[key]; pair != nil {
			for _, f := range []*core.Fact{pair.previous, pair.latest} {
				if f != nil {
					retract = append(retract, *f)
				}
			}
			delete(p.pairs, key)
		}
	}
	delete(p.byNode, node)
	if len(retract) > 0 {
		if err := p.k.RetractExactFactsBatch(retract); err != nil {
			return fmt.Errorf("recurse: retire attempts: %w", err)
		}
	}
	f := core.Fact{Predicate: "recurse_node_kept", Args: []interface{}{node, cycle}}
	p.kept[node] = f
	return p.k.AssertBatch([]core.Fact{f})
}

// improveAngle returns the angle the kernel aims node's improvement step at
// this pass, given the metrics measurable there, or "" when none applies.
func (p *recursePolicy) improveAngle(node string, pass int, measurable map[string]int) (string, error) {
	if err := p.k.RemoveFactsByPredicateSet(map[string]struct{}{
		"recurse_current_pass": {}, "recurse_node_measures": {},
	}); err != nil {
		return "", fmt.Errorf("recurse: clear improvement facts: %w", err)
	}
	facts := []core.Fact{{Predicate: "recurse_current_pass", Args: []interface{}{pass}}}
	for _, m := range sortedMetricNames(measurable) {
		facts = append(facts, core.Fact{Predicate: "recurse_node_measures", Args: []interface{}{node, "/" + m}})
	}
	if err := p.k.AssertBatch(facts); err != nil {
		return "", fmt.Errorf("recurse: assert improvement facts: %w", err)
	}
	rows, err := p.k.Query("recurse_improve_angle")
	if err != nil {
		return "", fmt.Errorf("recurse: query recurse_improve_angle: %w", err)
	}
	var angles []string
	for _, f := range rows {
		if len(f.Args) > 1 && types.ExtractString(f.Args[0]) == node {
			angles = append(angles, strings.TrimPrefix(types.ExtractString(f.Args[1]), "/"))
		}
	}
	sort.Strings(angles)
	if len(angles) == 0 {
		return "", nil
	}
	return angles[0], nil
}

func sortedMetricNames(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ratchetGate is one gate's before and after for an attempt.
type ratchetGate struct {
	Gate                    string
	Before, After           string
	BeforeCount, AfterCount int
}

// ratchetInput is what re-measuring an attempt found.
type ratchetInput struct {
	Cycle int
	Gates []ratchetGate
	// TargetID is a fix's finding; "" for an improvement.
	TargetID string
	TargetOK bool // the target finding is no longer open
	// Improve is an improvement's angle; "" for a fix.
	Improve string
	// Before and After are the metrics measured around the attempt.
	Before, After map[string]int
	Changed       bool // the attempt changed the tree at all
	Forbidden     []string
	// Tested: a test gate gave a verdict on the attempt's result.
	Tested bool
}

// ratchet asserts the re-measurement and returns the kernel's verdict. Exactly
// one verdict is derived for a cycle; anything else is a policy fault and is
// returned as an error, never guessed at.
func (p *recursePolicy) ratchet(in ratchetInput) (string, error) {
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
	}
	if in.TargetID != "" {
		facts = append(facts, core.Fact{Predicate: "recurse_ratchet_target", Args: []interface{}{in.Cycle, in.TargetID, target}})
	}
	if in.Improve != "" {
		facts = append(facts, core.Fact{Predicate: "recurse_improve", Args: []interface{}{in.Cycle, "/" + in.Improve}})
	}
	if in.Tested {
		facts = append(facts, core.Fact{Predicate: "recurse_ratchet_tested", Args: []interface{}{in.Cycle}})
	}
	for _, m := range sortedMetricNames(in.Before) {
		if after, ok := in.After[m]; ok {
			facts = append(facts, core.Fact{Predicate: "recurse_metric", Args: []interface{}{in.Cycle, "/" + m, in.Before[m], after}})
		}
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

// ratchetInputs are the predicates one cycle's judgment is asserted as.
var ratchetInputs = map[string]struct{}{
	"recurse_ratchet_changed": {}, "recurse_ratchet_target": {}, "recurse_ratchet_gate": {},
	"recurse_ratchet_forbidden": {}, "recurse_improve": {}, "recurse_metric": {},
	"recurse_ratchet_tested": {},
}

// endCycle retires a judged cycle's inputs. Its verdict is in the journal and
// its outcome in recurse_attempt; left in the kernel, every cycle's gate
// verdicts and metrics would accumulate for as long as the loop runs.
func (p *recursePolicy) endCycle() error {
	if err := p.k.RemoveFactsByPredicateSet(ratchetInputs); err != nil {
		return fmt.Errorf("recurse: retire cycle facts: %w", err)
	}
	return nil
}

// worseGates returns the gates the kernel judged worse after cycle's attempt.
func (p *recursePolicy) worseGates(cycle int) ([]string, error) {
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
func (p *recursePolicy) ratchetKinds() (map[gates.Kind]bool, error) {
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
func (p *recursePolicy) stalled() ([]string, error) { return p.derivedColumn("finding_stalled", 0) }
func (p *recursePolicy) refused() ([]string, error) { return p.derivedColumn("finding_refused", 0) }

// derivedColumn returns the distinct values of one argument of a derived predicate,
// sorted.
func (p *recursePolicy) derivedColumn(predicate string, i int) ([]string, error) {
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
