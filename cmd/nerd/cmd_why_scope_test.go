package main

import (
	"context"
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// `nerd why 'safe_action(/read_file)'` printed the pattern, then cut it at
// the first parenthesis and traced bare safe_action: every safe action came
// back as a root, /write_file among them (audit
// why-query-scope-preservation). The accepted pattern is now the query the
// trace runs, checked against a direct typed Query on the same kernel.
func TestTraceWhy_PreservesTheRequestedPattern(t *testing.T) {
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	trace, traced, err := traceWhy(ctx, kern, "safe_action(/read_file)")
	if err != nil {
		t.Fatal(err)
	}
	if traced != "safe_action(/read_file)" {
		t.Fatalf("traced %q, want the requested pattern", traced)
	}
	want, err := kern.Query("safe_action(/read_file)")
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 1 {
		t.Fatalf("precondition: the stock constitution has safe_action(/read_file) once, got %v", want)
	}
	if len(trace.RootNodes) != len(want) {
		var roots []string
		for _, n := range trace.RootNodes {
			roots = append(roots, fmt.Sprint(n.Fact))
		}
		t.Fatalf("roots = %v, want only %v", roots, want)
	}
	if got := fmt.Sprint(trace.RootNodes[0].Fact.Args); got != "[/read_file]" {
		t.Fatalf("root = %s, want /read_file", got)
	}
}

// A goal with no match has no roots; it does not widen to the relation.
func TestTraceWhy_UnmatchedPatternHasNoRoots(t *testing.T) {
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	absent := "safe_action(/not_a_shipped_action)"
	if rows, err := kern.Query(absent); err != nil || len(rows) != 0 {
		t.Fatalf("precondition: %s must be absent, got %v, %v", absent, rows, err)
	}
	trace, _, err := traceWhy(context.Background(), kern, absent)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.RootNodes) != 0 {
		t.Fatalf("an absent goal traced %d roots: the pattern was widened", len(trace.RootNodes))
	}
}

// A bare predicate still means the whole relation, every arity.
func TestTraceWhy_BarePredicateTracesTheRelation(t *testing.T) {
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	trace, traced, err := traceWhy(context.Background(), kern, "safe_action")
	if err != nil {
		t.Fatal(err)
	}
	want, err := kern.Query("safe_action")
	if err != nil {
		t.Fatal(err)
	}
	if traced != "safe_action" || len(trace.RootNodes) != len(want) || len(want) < 2 {
		t.Fatalf("bare safe_action traced %q with %d roots, want the %d rows of the relation", traced, len(trace.RootNodes), len(want))
	}
}

// A pattern that does not parse is refused, not silently traced as its
// predicate.
func TestTraceWhy_MalformedPatternIsRefused(t *testing.T) {
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := traceWhy(context.Background(), kern, "safe_action(/read_file"); err == nil {
		t.Fatal("a malformed pattern was accepted")
	}
}
