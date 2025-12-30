package core

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/mangle"
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
	// 1. Check if it's explicitly marked as EDB
	// Query is_edb_predicate(fact.Predicate)
	// Since this is called frequently, in a real system we would cache this.
	// For now, we rely on the Kernel's query speed.
	edbResults, _ := k.Query(fmt.Sprintf("is_edb_predicate(\"%s\")", fact.Predicate))
	if len(edbResults) > 0 {
		return mangle.SourceEDB, ""
	}

	// 2. Check if it has a known rule source (IDB)
	// Query rule_metadata(fact.Predicate, RuleName)
	ruleResults, _ := k.Query(fmt.Sprintf("rule_metadata(\"%s\", RuleName)", fact.Predicate))
	if len(ruleResults) > 0 {
		if len(ruleResults[0].Args) > 1 {
			if ruleName, ok := ruleResults[0].Args[1].(string); ok {
				return mangle.SourceIDB, ruleName
			}
		}
	}

	// Default to EDB if unknown (safe fallback)
	return mangle.SourceEDB, ""
}

// findPremises attempts to find the facts that supported this derivation.
// Since we don't have a true retro-justification engine, we use heuristic matching
// based on the known rule structures.
func (k *RealKernel) findPremises(ctx context.Context, fact mangle.Fact, ruleName string) []mangle.Fact {
	var premises []mangle.Fact

	switch ruleName {
	case "transitive_impact":
		// impacted(X) :- dependency_link(X, Y, _), modified(Y).
		// We look for dependency_link facts where Arg0 matches our Arg0
		if len(fact.Args) > 0 {
			deps, _ := k.Query("dependency_link")
			for _, d := range deps {
				// Filter to facts where first arg matches
				if len(d.Args) > 0 && fmt.Sprintf("%v", d.Args[0]) == fmt.Sprintf("%v", fact.Args[0]) {
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
				if len(s.Args) > 0 && fmt.Sprintf("%v", s.Args[0]) == fmt.Sprintf("%v", fact.Args[0]) {
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
				if len(f.Args) > 0 && fmt.Sprintf("%v", f.Args[0]) == fmt.Sprintf("%v", fact.Args[0]) {
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
