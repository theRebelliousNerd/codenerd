// limits_cleanup_test.go pins the LIMITS CLEANUP behavior changes in package
// core: syntax diagnostics pass whole, and healed-file markers stay commented
// even for multi-line errors.
package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// checkSyntax must hand the self-healing loop the parser's error verbatim:
// the old first-line / 100-rune cuts truncated error locations on long
// clauses. Parser-agnostic: whatever parseUnit reports must survive intact.
func TestCheckSyntax_ErrorReturnedVerbatim(t *testing.T) {
	inputs := []string{
		`broken_predicate(arg1, arg2`,
		`rule(X) :- `,
		`fact("a", "b", `,
		"12345 ++++ !!!!",
		strings.Repeat("x", 300),
		`bad_rule(X) :- undefined(`,
	}
	erroring := 0
	for _, in := range inputs {
		_, wantErr := parseUnit(strings.NewReader(in))
		if wantErr == nil {
			continue
		}
		erroring++
		gotErr := checkSyntax(in)
		if gotErr == nil {
			t.Errorf("checkSyntax(%q) = nil, parser reports %q", in, wantErr)
			continue
		}
		if gotErr.Error() != wantErr.Error() {
			t.Errorf("checkSyntax cut the error:\nparser: %q\ncheck:  %q", wantErr.Error(), gotErr.Error())
		}
	}
	if erroring == 0 {
		t.Fatal("no input produced a parse error; the test pins nothing")
	}
}

// Parser errors on long clauses are multi-line and hundreds of chars (an
// unterminated string reports 464 chars over two lines): the old first-line /
// 100-rune cuts destroyed almost all of that.
func TestCheckSyntax_LongMultilineErrorKeepsFullDetail(t *testing.T) {
	in := `fact("` + strings.Repeat("y", 300)
	err := checkSyntax(in)
	if err == nil {
		t.Fatal("checkSyntax of an unterminated string = nil, want a parse error")
	}
	msg := err.Error()
	if len(msg) <= 100 {
		t.Errorf("error len = %d, want the full multi-hundred-char diagnostic", len(msg))
	}
	if !strings.Contains(msg, "\n") {
		t.Errorf("error lost its line breaks: %q", msg)
	}
	if strings.HasSuffix(strings.TrimSpace(msg), "...") {
		t.Errorf("error still carries a truncation ellipsis: %q", msg)
	}
}

// End to end: healing a statement whose parse error spans lines must leave a
// file with no uncommented error text.
func TestValidateLearnedRulesContent_HealKeepsFileCommented(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl foo(Name).")
	k.Evaluate()
	// heal=true persists the healed text to the path given; a bare file name
	// would land in the package directory the test runs in.
	learnedPath := filepath.Join(t.TempDir(), "learned.mg")
	res := k.validateLearnedRulesContent(`fact("`+strings.Repeat("y", 300)+"\n", learnedPath, true)
	if res.stats.InvalidRules == 0 {
		t.Fatalf("stats = %+v, want the malformed statement counted invalid", res.stats)
	}
	for _, line := range strings.Split(res.healedText, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			t.Errorf("healed text leaked an uncommented line: %q (full: %q)", line, res.healedText)
		}
	}
	if !strings.Contains(res.healedText, "# SELF-HEALED: ") {
		t.Errorf("healed text has no marker: %q", res.healedText)
	}
}

// Mutually recursive rules resolve premises in a cycle; the trace must
// terminate and mark the repeat instead of recursing to the depth guard and
// rendering a leaf that reads as complete.
func TestTraceQuery_CyclicPremisesTerminateWithMarkedRepeat(t *testing.T) {
	const schema = `
Decl base_link(Name) bound [/string].
Decl link_a(Name) bound [/string].
Decl link_b(Name) bound [/string].
link_b(X) :- base_link(X).
link_a(X) :- link_b(X).
link_b(X) :- link_a(X).
`
	k := newTracedKernel(t, schema, []Fact{
		{Predicate: "base_link", Args: []any{"alpha"}},
	})
	trace, err := k.TraceQuery(context.Background(), "link_a")
	if err != nil {
		t.Fatalf("TraceQuery: %v", err)
	}
	if len(trace.RootNodes) == 0 {
		t.Fatal("no root nodes")
	}
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
}

// A derivation chain deeper than maxTraceDepth still renders every level it
// shows, and the capped node says the chain continues.
func TestTraceQuery_DeepChainMarksDepthGuardHonestly(t *testing.T) {
	const levels = 12
	var sb strings.Builder
	sb.WriteString("Decl base_item(Name) bound [/string].\n")
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&sb, "Decl level%d(Name) bound [/string].\n", i)
	}
	sb.WriteString("level1(X) :- base_item(X).\n")
	for i := 2; i <= levels; i++ {
		fmt.Fprintf(&sb, "level%d(X) :- level%d(X).\n", i, i-1)
	}
	k := newTracedKernel(t, sb.String(), []Fact{
		{Predicate: "base_item", Args: []any{"alpha"}},
	})
	trace, err := k.TraceQuery(context.Background(), fmt.Sprintf("level%d", levels))
	if err != nil {
		t.Fatalf("TraceQuery: %v", err)
	}
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
	if maxDepth != maxTraceDepth {
		t.Errorf("max depth = %d, want the guard at %d", maxDepth, maxTraceDepth)
	}
	if marked == 0 {
		t.Error("no node carries the depth mark; the capped chain renders as complete")
	}
}

// A healed marker may embed a multi-line error; every line must stay inside
// a comment so the healed file keeps parsing.
func TestSyntaxHealMarker_MultilineErrorStaysCommented(t *testing.T) {
	got := healMarkerLine("syntax error: ", "first line\nsecond line\nthird")
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "#") {
			t.Errorf("marker leaked an uncommented line: %q (full: %q)", line, got)
		}
	}
	if !strings.Contains(got, "second line") {
		t.Errorf("marker dropped error detail: %q", got)
	}
}
