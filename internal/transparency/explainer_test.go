package transparency

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

func TestExplainerExplainTrace(t *testing.T) {
	explainer := NewExplainer()
	trace := buildTestTrace()

	text := explainer.ExplainTrace(trace)
	if !strings.Contains(text, "Explanation") {
		t.Fatalf("expected explanation header")
	}
	if !strings.Contains(text, "Query") {
		t.Fatalf("expected query section")
	}
	if !strings.Contains(text, "next_action") {
		t.Fatalf("expected predicate in explanation")
	}
}

func TestFormatOperationSummary(t *testing.T) {
	summary := &OperationSummary{
		Operation:     "Test Run",
		Duration:      "1s",
		FilesAffected: []string{"main.go"},
		Outcome:       "success",
		NextSteps:     []string{"commit"},
	}
	text := FormatOperationSummary(summary)
	if !strings.Contains(text, "Test Run Complete") {
		t.Fatalf("expected summary header")
	}
	if !strings.Contains(text, "main.go") {
		t.Fatalf("expected file list in summary")
	}
}

func buildTestTrace() *mangle.DerivationTrace {
	child := &mangle.DerivationNode{
		Fact: mangle.Fact{
			Predicate: "permitted",
			Args:      []any{"/test"},
		},
		Source: mangle.SourceEDB,
	}
	root := &mangle.DerivationNode{
		Fact: mangle.Fact{
			Predicate: "next_action",
			Args:      []any{"/test"},
		},
		Source:   mangle.SourceIDB,
		RuleName: "strategy_selector",
		Children: []*mangle.DerivationNode{child},
	}
	return &mangle.DerivationTrace{
		Query:     "next_action(/test)",
		RootNodes: []*mangle.DerivationNode{root},
		AllNodes:  []*mangle.DerivationNode{root, child},
		Duration:  time.Second,
	}
}

func TestExplainerExplainTrace_WhenNilOrEmpty_ShouldReturnNoDerivation(t *testing.T) {
	explainer := NewExplainer()
	if got := explainer.ExplainTrace(nil); got != "No derivation found for this query." {
		t.Fatalf("expected no-derivation for nil trace, got %q", got)
	}
	if got := explainer.ExplainTrace(&mangle.DerivationTrace{}); got != "No derivation found for this query." {
		t.Fatalf("expected no-derivation for empty trace, got %q", got)
	}
}

func TestFormatOperationSummary_WhenNil_ShouldReturnEmpty(t *testing.T) {
	if got := FormatOperationSummary(nil); got != "" {
		t.Fatalf("expected empty string for nil summary, got %q", got)
	}
}

func TestOperationSummary_StatusLine_ShouldFormatFields(t *testing.T) {
	plain := &OperationSummary{Operation: "Scan"}
	if got := plain.StatusLine(); got != "Scan" {
		t.Fatalf("expected bare operation, got %q", got)
	}
	stamped := &OperationSummary{
		Operation:   "Scan",
		Outcome:     "success",
		Duration:    "1s",
		CompletedAt: time.Date(2026, 11, 8, 10, 11, 12, 0, time.UTC),
	}
	got := stamped.StatusLine()
	if !strings.Contains(got, "[10:11:12]") || !strings.Contains(got, "Scan: success") || !strings.Contains(got, "(1s)") {
		t.Fatalf("expected timestamp, outcome and duration in %q", got)
	}
}

func TestExplainRule_WhenUnknownOrEmpty_ShouldFallback(t *testing.T) {
	if got := explainRule("no_such_rule_xyz"); got != "rule 'no_such_rule_xyz'" {
		t.Fatalf("expected generic rule fallback, got %q", got)
	}
	if got := explainRule(""); got != "" {
		t.Fatalf("expected empty for empty rule, got %q", got)
	}
	if got := explainRule("permitted"); got != "safety permission gate" {
		t.Fatalf("expected glossary hit for permitted, got %q", got)
	}
}

func TestFormatFactForHuman_ShouldHandleShapes(t *testing.T) {
	if got := formatFactForHuman(mangle.Fact{Predicate: "foo"}); got != "foo" {
		t.Fatalf("expected bare predicate, got %q", got)
	}
	if got := formatFactForHuman(mangle.Fact{Predicate: "p", Args: []any{"/a"}}); got != "p(/a)" {
		t.Fatalf("expected atom arg unquoted, got %q", got)
	}
	if got := formatArg("hi"); got != `"hi"` {
		t.Fatalf("expected quoted string, got %q", got)
	}
	if got := formatArg("/a"); got != "/a" {
		t.Fatalf("expected atom passthrough, got %q", got)
	}
	if got := formatArg(42); got != "42" {
		t.Fatalf("expected int formatting, got %q", got)
	}
}
