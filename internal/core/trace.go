package core

import (
	"context"
	"fmt"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"

	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// TraceQuery executes a query and returns a derivation trace.
// detailed proof tree showing how facts were derived.
func (k *RealKernel) TraceQuery(ctx context.Context, query string) (*mangle.DerivationTrace, error) {
	start := time.Now()

	// 1. Execute the query using the existing Kernel mechanism
	facts, err := k.Query(query)
	if err != nil {
		return nil, err
	}

	// 2. Build the trace structure
	trace := &mangle.DerivationTrace{
		Query:     query,
		RootNodes: make([]*mangle.DerivationNode, 0),
		AllNodes:  make([]*mangle.DerivationNode, 0),
		Timestamp: start,
	}

	// 3. For each result fact, build its derivation tree
	for _, fact := range facts {
		mangleFact := mangle.Fact{
			Predicate: fact.Predicate,
			Args:      fact.Args,
			Timestamp: time.Now(), // approximate
		}

		node := k.buildDerivationNode(ctx, mangleFact, "", 0)
		trace.RootNodes = append(trace.RootNodes, node)
		trace.AllNodes = append(trace.AllNodes, k.flattenTree(node)...)
	}

	trace.Duration = time.Since(start)

	return trace, nil
}

// buildDerivationNode recursively builds the proof tree for a fact.
func (k *RealKernel) buildDerivationNode(ctx context.Context, fact mangle.Fact, parentID string, depth int) *mangle.DerivationNode {
	// Generate a simple unique ID
	nodeID := fmt.Sprintf("node_%d_%d", time.Now().UnixNano(), depth)

	node := &mangle.DerivationNode{
		ID:        nodeID,
		ParentID:  parentID,
		Fact:      fact,
		Depth:     depth,
		Timestamp: time.Now(),
		Children:  make([]*mangle.DerivationNode, 0),
	}

	// Identify source and rule
	source, ruleName := k.classifyFact(fact)
	node.Source = source
	node.RuleName = ruleName

	// Recursively find premises if it is a derived fact
	if source == mangle.SourceIDB && depth < 10 { // Depth limit to prevent cycles
		premises := k.findPremises(ctx, fact, ruleName)
		for _, premise := range premises {
			child := k.buildDerivationNode(ctx, premise, nodeID, depth+1)
			node.Children = append(node.Children, child)
		}
	}

	return node
}

// classifyFact determines if a fact is EDB (base) or IDB (derived) and which rule produced it.
// Replaces hardcoded maps with Mangle lookups (LOGOS Refactor).
func (k *RealKernel) classifyFact(fact mangle.Fact) (mangle.DerivationSource, string) {
	// 1. Ask the analyzed program. A predicate that appears as a rule head IS
	//    derived — this is the ground truth and it cannot drift.
	//
	//    This check used to be absent, and classification rested entirely on the
	//    rule_metadata/is_edb_predicate facts consulted below. Those are a
	//    hand-maintained list in the .mg corpus covering a few dozen predicates,
	//    so every predicate added since it was written fell through to the EDB
	//    default. buildDerivationNode only recurses into premises when a fact is
	//    IDB, so `nerd why` printed a bare one-line "[EDB]" for genuinely derived
	//    facts and never showed a derivation chain — observed live on
	//    project_write_protected, which is derived at policy/projectdoc.mg:10.
	//    A glass box you cannot see through is worse than no glass box, because
	//    it still reads as an answer.
	if info := k.GetProgramInfo(); info != nil {
		for sym := range info.IdbPredicates {
			if sym.Symbol == fact.Predicate {
				// rule_metadata still supplies the friendly rule name when the
				// corpus happens to carry one; the classification no longer
				// depends on it.
				return mangle.SourceIDB, k.ruleNameFor(fact.Predicate)
			}
		}
		for sym := range info.EdbPredicates {
			if sym.Symbol == fact.Predicate {
				return mangle.SourceEDB, ""
			}
		}
	}

	// 2. Fall back to the curated metadata for predicates the analyzer does not
	//    know about (e.g. facts asserted for a predicate declared elsewhere).
	edbResults, _ := k.Query(fmt.Sprintf("is_edb_predicate(\"%s\")", fact.Predicate))
	if len(edbResults) > 0 {
		return mangle.SourceEDB, ""
	}
	if name := k.ruleNameFor(fact.Predicate); name != "" {
		return mangle.SourceIDB, name
	}

	// Default to EDB if unknown (safe fallback)
	return mangle.SourceEDB, ""
}

// ruleNameFor returns the descriptive rule name the corpus records for a
// derived predicate, or "" when it records none. Absence is not evidence the
// predicate is a base fact — it usually just means nobody added the metadata.
func (k *RealKernel) ruleNameFor(predicate string) string {
	ruleResults, _ := k.Query(fmt.Sprintf("rule_metadata(\"%s\", RuleName)", predicate))
	if len(ruleResults) > 0 && len(ruleResults[0].Args) > 1 {
		if ruleName, ok := ruleResults[0].Args[1].(string); ok {
			return ruleName
		}
	}
	return ""
}

// findPremises attempts to find the facts that supported this derivation.
// Since we don't have a true retro-justification engine, we use heuristic matching
// based on the known rule structures.
func (k *RealKernel) findPremises(ctx context.Context, fact mangle.Fact, ruleName string) []mangle.Fact {
	if err := ctx.Err(); err != nil {
		return nil
	}
	var premises []mangle.Fact

	switch ruleName {
	case "transitive_impact":
		// impacted(X) :- dependency_link(X, Y, _), modified(Y).
		// We look for dependency_link facts where Arg0 matches our Arg0
		if len(fact.Args) > 0 {
			deps, _ := k.Query("dependency_link")
			for _, d := range deps {
				// Filter to facts where first arg matches
				if len(d.Args) > 0 && types.ExtractString(d.Args[0]) == types.ExtractString(fact.Args[0]) {
					premises = append(premises, convertCoreFactToMangle(d))
				}
			}
		}

	case "permission_gate":
		// permitted(Action) :- safe_action(Action).
		if len(fact.Args) > 0 {
			safes, _ := k.Query("safe_action")
			for _, s := range safes {
				// Filter to facts where arg matches
				if len(s.Args) > 0 && types.ExtractString(s.Args[0]) == types.ExtractString(fact.Args[0]) {
					premises = append(premises, convertCoreFactToMangle(s))
				}
			}
		}

	case "focus_threshold":
		// clarification_needed(Ref) :- focus_resolution(Ref, ..., Score), Score < ...
		if len(fact.Args) > 0 {
			focus, _ := k.Query("focus_resolution")
			for _, f := range focus {
				// Filter to facts where first arg matches
				if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == types.ExtractString(fact.Args[0]) {
					premises = append(premises, convertCoreFactToMangle(f))
				}
			}
		}

	case "strategy_selector":
		// next_action depends on user_intent
		intents, _ := k.Query("user_intent")
		for _, i := range intents {
			premises = append(premises, convertCoreFactToMangle(i))
		}

		// Add more heuristics as needed
	}

	// Generic fallback: derive the premises from the ACTUAL rule bodies.
	//
	// The switch above covers four hand-written rule names out of the hundreds
	// in the corpus, so every other derived fact rendered as a leaf and
	// `nerd why` showed no chain at all — the glass box was three special cases
	// wide. Reading the body predicates out of the analyzed program covers every
	// rule, including ones added after this function was written.
	if len(premises) == 0 {
		premises = k.premisesFromProgram(fact)
	}

	return premises
}

// premisesFromProgram finds supporting facts by reading the body of every rule
// whose head matches this fact's predicate, then querying those body
// predicates. Mirrors mangle.ProofTreeTracer.findPremises, which already worked
// this way — RealKernel simply never got the same treatment.
//
// Argument matching is a first-argument heuristic, as it is in the tracer: full
// unification would need the variable bindings the evaluator discarded. A
// zero-arity head (project_write_protected() and friends) has nothing to match
// on, so all facts of each body predicate are shown — which is the correct
// answer for a rule that fires on mere existence.
func (k *RealKernel) premisesFromProgram(fact mangle.Fact) []mangle.Fact {
	info := k.GetProgramInfo()
	if info == nil {
		return nil
	}

	bodyPreds := make(map[string]bool)
	for _, rule := range info.Rules {
		if rule.Head.Predicate.Symbol != fact.Predicate {
			continue
		}
		for _, premise := range rule.Premises {
			atom, ok := premise.(ast.Atom)
			if !ok {
				continue // negations, comparisons and transforms carry no facts
			}
			sym := atom.Predicate.Symbol
			if sym == fact.Predicate {
				continue // skip self-reference so recursive rules cannot loop
			}
			bodyPreds[sym] = true
		}
	}

	var premises []mangle.Fact
	for pred := range bodyPreds {
		facts, err := k.Query(pred)
		if err != nil {
			continue
		}
		for _, bf := range facts {
			// With no args on the head there is nothing to correlate against,
			// so every supporting fact is relevant.
			if len(fact.Args) == 0 {
				premises = append(premises, convertCoreFactToMangle(bf))
				continue
			}
			if len(bf.Args) >= 1 && types.ExtractString(bf.Args[0]) == types.ExtractString(fact.Args[0]) {
				premises = append(premises, convertCoreFactToMangle(bf))
			}
		}
	}
	return premises
}

func (k *RealKernel) flattenTree(node *mangle.DerivationNode) []*mangle.DerivationNode {
	nodes := []*mangle.DerivationNode{node}
	for _, child := range node.Children {
		nodes = append(nodes, k.flattenTree(child)...)
	}
	return nodes
}

func convertCoreFactToMangle(f Fact) mangle.Fact {
	return mangle.Fact{
		Predicate: f.Predicate,
		Args:      f.Args,
		Timestamp: time.Now(),
	}
}
