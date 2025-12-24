package ui

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

func TestLogicPaneToggleMode(t *testing.T) {
	pane := NewLogicPane(DefaultStyles(), 80, 20)
	if pane.Mode != ModeSinglePane {
		t.Fatalf("expected initial mode single pane")
	}

	pane.ToggleMode()
	if pane.Mode != ModeSplitPane {
		t.Fatalf("expected mode split pane")
	}

	pane.ToggleMode()
	if pane.Mode != ModeFullLogic {
		t.Fatalf("expected mode full logic")
	}

	pane.ToggleMode()
	if pane.Mode != ModeSinglePane {
		t.Fatalf("expected mode single pane again")
	}
}

func TestLogicPaneSetTraceMangle(t *testing.T) {
	pane := NewLogicPane(DefaultStyles(), 80, 20)

	child := &mangle.DerivationNode{
		Fact: mangle.Fact{
			Predicate: "child_pred",
			Args:      []interface{}{1},
		},
		RuleName: "child_rule",
		Source:   mangle.SourceIDB,
		Depth:    1,
	}
	root := &mangle.DerivationNode{
		Fact: mangle.Fact{
			Predicate: "root_pred",
			Args:      []interface{}{"arg"},
		},
		RuleName: "root_rule",
		Source:   mangle.SourceEDB,
		Depth:    0,
		Children: []*mangle.DerivationNode{child},
	}
	trace := &mangle.DerivationTrace{
		Query:     "root_pred(\"arg\")",
		RootNodes: []*mangle.DerivationNode{root},
		AllNodes:  []*mangle.DerivationNode{root, child},
		Duration:  120 * time.Millisecond,
	}

	pane.SetTraceMangle(trace)
	if pane.CurrentTrace == nil || pane.CurrentTrace.Query != trace.Query {
		t.Fatalf("expected trace to be set")
	}
	if len(pane.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(pane.Nodes))
	}
	if pane.Nodes[0].Predicate != "root_pred" || pane.Nodes[1].Predicate != "child_pred" {
		t.Fatalf("unexpected node predicates")
	}

	content := pane.renderContent()
	if !strings.Contains(content, "Query: root_pred") {
		t.Fatalf("expected content to include query")
	}
}

func TestLogicPaneEmptyState(t *testing.T) {
	pane := NewLogicPane(DefaultStyles(), 80, 20)
	pane.SetTrace(nil)
	content := pane.renderContent()
	if !strings.Contains(content, "No derivation trace yet.") {
		t.Fatalf("expected empty state content")
	}
}

func TestSplitPaneViewRenderModes(t *testing.T) {
	view := NewSplitPaneView(DefaultStyles(), 80, 20)
	left := "left content"

	view.SetMode(ModeSinglePane)
	if got := view.Render(left); got != left {
		t.Fatalf("expected single pane to return left content")
	}

	view.SetMode(ModeFullLogic)
	got := view.Render(left)
	if !strings.Contains(got, "No derivation trace yet.") {
		t.Fatalf("expected full logic to render empty state")
	}

	view.RightPane = nil
	view.Mode = ModeSplitPane
	if got = view.Render(left); got != left {
		t.Fatalf("expected split pane with nil right pane to return left content")
	}
}
