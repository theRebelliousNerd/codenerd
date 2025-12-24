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

func TestExplainerExplainFact(t *testing.T) {
	explainer := NewExplainer()
	trace := buildTestTrace()

	text := explainer.ExplainFact(trace, "next_action")
	if !strings.Contains(text, "Why `next_action` holds") {
		t.Fatalf("expected why header")
	}
}

func TestExplainerExplainDecision(t *testing.T) {
	explainer := NewExplainer()
	trace := buildTestTrace()

	text := explainer.ExplainDecision("run tests", trace)
	if !strings.Contains(text, "Decision:") {
		t.Fatalf("expected decision header")
	}
}

func TestQuickExplain(t *testing.T) {
	text := QuickExplain("next_action", []interface{}{"/test"})
	if !strings.Contains(text, "Next action will be") {
		t.Fatalf("expected quick explain output")
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
			Args:      []interface{}{"/test"},
		},
		Source: mangle.SourceEDB,
	}
	root := &mangle.DerivationNode{
		Fact: mangle.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/test"},
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
