package mangle

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func proofLimitEngine(t *testing.T, schema, factPred string, factArgs ...any) *Engine {
	t.Helper()
	engine, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	if err := engine.AddFact(factPred, factArgs...); err != nil {
		t.Fatalf("AddFact: %v", err)
	}
	return engine
}

func traceProof(t *testing.T, engine *Engine, query string) *DerivationTrace {
	t.Helper()
	tracer := NewProofTreeTracer(engine)
	tracer.IndexRules()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	trace, err := tracer.TraceQuery(ctx, query)
	if err != nil {
		t.Fatalf("TraceQuery(%s): %v", query, err)
	}
	if len(trace.RootNodes) == 0 {
		t.Fatal("no root nodes")
	}
	return trace
}

// Mutually recursive rules resolve premises in a cycle. findPremises skips
// only a body predicate equal to the rule's own name, so link_a → link_b →
// link_a is a real cycle. The trace must stop and mark the repeat instead of
// rendering it as a finished leaf.
func TestProofTree_CyclicPremisesTerminateWithMarkedRepeat(t *testing.T) {
	schema := `
	Decl base_link(Name) descr [mode("-")].
	Decl link_a(Name) descr [mode("-")].
	Decl link_b(Name) descr [mode("-")].

	link_b(X) :- base_link(X).
	link_a(X) :- link_b(X).
	link_b(X) :- link_a(X).
	`
	trace := traceProof(t, proofLimitEngine(t, schema, "base_link", "alpha"), "link_a(X)")

	maxDepth := 0
	marked := 0
	for _, n := range trace.AllNodes {
		if n.Depth > maxDepth {
			maxDepth = n.Depth
		}
		if strings.Contains(n.RuleName, "cycle: expanded above") {
			marked++
			if len(n.Children) != 0 {
				t.Errorf("cycle-marked node %v has %d children, want an unexpanded leaf", n.Fact, len(n.Children))
			}
		}
	}
	if marked == 0 {
		t.Error("no node carries the cycle mark; the repeat renders as an ordinary leaf")
	}
	if maxDepth > 3 {
		t.Errorf("max depth = %d, want the cycle to stop the walk within 3 levels", maxDepth)
	}
	if ascii := trace.RenderASCII(); !strings.Contains(ascii, "cycle: expanded above") {
		t.Errorf("ASCII render hid the cycle:\n%s", ascii)
	}
}

// A derivation chain deeper than maxProofDepth still renders every level it
// shows, and the capped node says the chain continues.
func TestProofTree_DeepChainMarksDepthGuardHonestly(t *testing.T) {
	const levels = 12
	var sb strings.Builder
	sb.WriteString("Decl base_item(Name) descr [mode(\"-\")].\n")
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&sb, "Decl level%d(Name) descr [mode(\"-\")].\n", i)
	}
	sb.WriteString("level1(X) :- base_item(X).\n")
	for i := 2; i <= levels; i++ {
		fmt.Fprintf(&sb, "level%d(X) :- level%d(X).\n", i, i-1)
	}
	trace := traceProof(t, proofLimitEngine(t, sb.String(), "base_item", "alpha"), fmt.Sprintf("level%d(X)", levels))

	maxDepth := 0
	marked := 0
	for _, n := range trace.AllNodes {
		if n.Depth > maxDepth {
			maxDepth = n.Depth
		}
		if strings.Contains(n.RuleName, "chain continues past depth limit") {
			marked++
			if len(n.Children) != 0 {
				t.Errorf("depth-marked node %v has %d children, want an unexpanded leaf", n.Fact, len(n.Children))
			}
		}
	}
	if maxDepth != maxProofDepth {
		t.Errorf("max depth = %d, want the guard at %d", maxDepth, maxProofDepth)
	}
	if marked == 0 {
		t.Error("no node carries the depth mark; the capped chain renders as complete")
	}
	if ascii := trace.RenderASCII(); !strings.Contains(ascii, "chain continues past depth limit") {
		t.Errorf("ASCII render hid the depth mark:\n%s", ascii)
	}
}
