package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAutopoiesisPage_TabNavigation(t *testing.T) {
	m := NewAutopoiesisPageModel()

	// Initial state
	if m.activeTab != TabPatterns {
		t.Errorf("Expected initial tab to be TabPatterns, got %d", m.activeTab)
	}

	// Navigate forward with Tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activeTab != TabLearnings {
		t.Errorf("Expected tab to be TabLearnings after Tab, got %d", m.activeTab)
	}

	// Wrap around with Tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activeTab != TabPatterns {
		t.Errorf("Expected tab to wrap to TabPatterns, got %d", m.activeTab)
	}
}

func TestAutopoiesisPage_ArrowKeyTabNavigation(t *testing.T) {
	m := NewAutopoiesisPageModel()

	// Navigate with Right arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}, Alt: false})
	keyMsg := tea.KeyMsg{}
	keyMsg.Type = tea.KeyRight
	m, _ = m.Update(keyMsg)
	if m.activeTab != TabLearnings {
		t.Errorf("Expected tab to be TabLearnings after Right arrow, got %d", m.activeTab)
	}

	// Navigate with Left arrow
	keyMsg.Type = tea.KeyLeft
	m, _ = m.Update(keyMsg)
	if m.activeTab != TabPatterns {
		t.Errorf("Expected tab to be TabPatterns after Left arrow, got %d", m.activeTab)
	}
}

func TestAutopoiesisPage_ShiftTabNavigation(t *testing.T) {
	m := NewAutopoiesisPageModel()
	m.activeTab = TabLearnings

	// Navigate backward with Shift+Tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.activeTab != TabPatterns {
		t.Errorf("Expected tab to be TabPatterns after Shift+Tab, got %d", m.activeTab)
	}
}

func TestSplitPane_FocusToggle(t *testing.T) {
	styles := DefaultStyles()
	split := NewSplitPaneView(styles, 100, 50, 0.6)

	// Initial state
	if split.FocusRight {
		t.Error("Expected initial focus to be on left pane")
	}

	// Toggle focus
	split.ToggleFocus()
	if !split.FocusRight {
		t.Error("Expected focus to be on right pane after toggle")
	}

	// Toggle back
	split.ToggleFocus()
	if split.FocusRight {
		t.Error("Expected focus to be back on left pane")
	}
}

func TestSplitPane_KeyboardFocusSwitch(t *testing.T) {
	styles := DefaultStyles()
	split := NewSplitPaneView(styles, 100, 50, 0.6)
	split.SetMode(ModeSplitPane)

	// Test Ctrl+L focus switch
	handled := split.HandleKey("ctrl+l")
	if !handled {
		t.Error("Expected ctrl+l to be handled")
	}
	if !split.FocusRight {
		t.Error("Expected focus to switch to right pane")
	}

	// Test Ctrl+Tab focus switch
	handled = split.HandleKey("ctrl+tab")
	if !handled {
		t.Error("Expected ctrl+tab to be handled")
	}
	if split.FocusRight {
		t.Error("Expected focus to switch back to left pane")
	}
}

func TestLogicPane_CircularNavigation(t *testing.T) {
	styles := DefaultStyles()
	pane := NewLogicPane(styles, 100, 50)

	// Create test trace with multiple nodes
	trace := &DerivationTrace{
		Query:       "test(X)",
		TotalFacts:  3,
		DerivedTime: time.Second,
		RootNodes: []*DerivationNode{
			{Predicate: "node1", Args: []string{"a"}, Source: "edb", Expanded: true},
			{Predicate: "node2", Args: []string{"b"}, Source: "idb", Expanded: true},
			{Predicate: "node3", Args: []string{"c"}, Source: "edb", Expanded: true},
		},
	}

	pane.SetTrace(trace)

	// Verify initial state
	if pane.SelectedNode != 0 {
		t.Errorf("Expected initial selected node to be 0, got %d", pane.SelectedNode)
	}

	// Navigate forward
	pane.SelectNext()
	if pane.SelectedNode != 1 {
		t.Errorf("Expected selected node to be 1, got %d", pane.SelectedNode)
	}

	pane.SelectNext()
	if pane.SelectedNode != 2 {
		t.Errorf("Expected selected node to be 2, got %d", pane.SelectedNode)
	}

	// Wrap around to beginning
	pane.SelectNext()
	if pane.SelectedNode != 0 {
		t.Errorf("Expected selected node to wrap to 0, got %d", pane.SelectedNode)
	}
}

func TestLogicPane_CircularNavigationBackward(t *testing.T) {
	styles := DefaultStyles()
	pane := NewLogicPane(styles, 100, 50)

	trace := &DerivationTrace{
		Query:       "test(X)",
		TotalFacts:  3,
		DerivedTime: time.Second,
		RootNodes: []*DerivationNode{
			{Predicate: "node1", Args: []string{"a"}, Source: "edb", Expanded: true},
			{Predicate: "node2", Args: []string{"b"}, Source: "idb", Expanded: true},
			{Predicate: "node3", Args: []string{"c"}, Source: "edb", Expanded: true},
		},
	}

	pane.SetTrace(trace)

	// Navigate backward from first node (should wrap to last)
	pane.SelectPrev()
	if pane.SelectedNode != 2 {
		t.Errorf("Expected selected node to wrap to 2, got %d", pane.SelectedNode)
	}

	// Navigate backward again
	pane.SelectPrev()
	if pane.SelectedNode != 1 {
		t.Errorf("Expected selected node to be 1, got %d", pane.SelectedNode)
	}
}

func TestSplitPane_RightPaneNavigation(t *testing.T) {
	styles := DefaultStyles()
	split := NewSplitPaneView(styles, 100, 50, 0.6)
	split.SetMode(ModeSplitPane)
	split.FocusRight = true

	// Create test trace
	trace := &DerivationTrace{
		Query:       "test(X)",
		TotalFacts:  2,
		DerivedTime: time.Second,
		RootNodes: []*DerivationNode{
			{Predicate: "node1", Args: []string{"a"}, Source: "edb", Expanded: true},
			{Predicate: "node2", Args: []string{"b"}, Source: "idb", Expanded: true},
		},
	}
	split.RightPane.SetTrace(trace)

	// Test navigation when focused
	handled := split.HandleKey("down")
	if !handled {
		t.Error("Expected down key to be handled when right pane focused")
	}

	handled = split.HandleKey("up")
	if !handled {
		t.Error("Expected up key to be handled when right pane focused")
	}

	// Test toggle expand
	handled = split.HandleKey("enter")
	if !handled {
		t.Error("Expected enter key to be handled when right pane focused")
	}

	// Test toggle activation
	initialActivation := split.RightPane.ShowActivation
	handled = split.HandleKey("a")
	if !handled {
		t.Error("Expected 'a' key to be handled when right pane focused")
	}
	if split.RightPane.ShowActivation == initialActivation {
		t.Error("Expected activation display to toggle")
	}
}

func TestSplitPane_NavigationWhenNotFocused(t *testing.T) {
	styles := DefaultStyles()
	split := NewSplitPaneView(styles, 100, 50, 0.6)
	split.SetMode(ModeSplitPane)
	split.FocusRight = false // Left pane focused

	// Navigation keys should not be handled when right pane not focused
	handled := split.HandleKey("down")
	if handled {
		t.Error("Expected down key to not be handled when left pane focused")
	}

	handled = split.HandleKey("up")
	if handled {
		t.Error("Expected up key to not be handled when left pane focused")
	}
}

func TestSplitPane_NavigationInWrongMode(t *testing.T) {
	styles := DefaultStyles()
	split := NewSplitPaneView(styles, 100, 50, 0.6)
	split.SetMode(ModeSinglePane) // Not split mode

	// Keys should not be handled in non-split mode
	handled := split.HandleKey("ctrl+l")
	if handled {
		t.Error("Expected keys to not be handled in single pane mode")
	}
}

func BenchmarkLogicPane_CircularNavigation(b *testing.B) {
	styles := DefaultStyles()
	pane := NewLogicPane(styles, 100, 50)

	// Create large trace
	nodes := make([]*DerivationNode, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = &DerivationNode{
			Predicate: "node",
			Args:      []string{"arg"},
			Source:    "edb",
			Expanded:  true,
		}
	}

	trace := &DerivationTrace{
		Query:       "test(X)",
		TotalFacts:  100,
		DerivedTime: time.Second,
		RootNodes:   nodes,
	}
	pane.SetTrace(trace)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.SelectNext()
	}
}
