package ui

import (
	"testing"
	"time"
)

// Helper function to create a test LogicPane with sample data
func createTestLogicPaneWithData() LogicPane {
	styles := DefaultStyles()
	pane := NewLogicPane(styles, 100, 50)

	// Create test trace with various nodes
	trace := &DerivationTrace{
		Query:       "test_query(X)",
		TotalFacts:  10,
		DerivedTime: 50 * time.Millisecond,
		RootNodes: []*DerivationNode{
			{
				Predicate: "user_intent",
				Args:      []string{"/code", "/fix", "bug.go", "fix bug"},
				Source:    "edb",
				Rule:      "",
				Expanded:  true,
				Children: []*DerivationNode{
					{
						Predicate: "next_action",
						Args:      []string{"/execute_intent"},
						Source:    "idb",
						Rule:      "intent_to_action",
						Expanded:  true,
					},
				},
			},
			{
				Predicate: "file_content",
				Args:      []string{"test.go", "package main"},
				Source:    "edb",
				Rule:      "",
				Expanded:  true,
			},
			{
				Predicate: "permitted",
				Args:      []string{"/read_file"},
				Source:    "idb",
				Rule:      "safe_action",
				Expanded:  true,
			},
		},
	}

	pane.SetTrace(trace)
	return pane
}

func TestApplyFilters_NoFilters(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Should have all nodes (4 total: 3 root + 1 child)
	if len(pane.Nodes) != 4 {
		t.Errorf("Expected 4 nodes without filters, got %d", len(pane.Nodes))
	}

	if len(pane.AllNodes) != 4 {
		t.Errorf("Expected 4 AllNodes, got %d", len(pane.AllNodes))
	}
}

func TestApplyFilters_SearchQuery(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Search for "intent" - should match user_intent and next_action
	pane.SetSearchQuery("intent")

	if len(pane.Nodes) != 2 {
		t.Errorf("Expected 2 nodes matching 'intent', got %d", len(pane.Nodes))
	}

	// Verify the matched nodes
	for _, node := range pane.Nodes {
		if node.Predicate != "user_intent" && node.Predicate != "next_action" {
			t.Errorf("Unexpected node in results: %s", node.Predicate)
		}
	}
}

func TestApplyFilters_SearchQuery_CaseInsensitive(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Search with different case
	pane.SetSearchQuery("USER")

	if len(pane.Nodes) != 1 {
		t.Errorf("Expected 1 node matching 'USER' (case-insensitive), got %d", len(pane.Nodes))
	}

	if pane.Nodes[0].Predicate != "user_intent" {
		t.Errorf("Expected user_intent, got %s", pane.Nodes[0].Predicate)
	}
}

func TestApplyFilters_SearchQuery_MatchesArgs(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Search for something in args
	pane.SetSearchQuery("bug.go")

	if len(pane.Nodes) != 1 {
		t.Errorf("Expected 1 node with 'bug.go' in args, got %d", len(pane.Nodes))
	}

	if pane.Nodes[0].Predicate != "user_intent" {
		t.Errorf("Expected user_intent, got %s", pane.Nodes[0].Predicate)
	}
}

func TestApplyFilters_SearchQuery_MatchesRule(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Search for rule name
	pane.SetSearchQuery("intent_to_action")

	if len(pane.Nodes) != 1 {
		t.Errorf("Expected 1 node with 'intent_to_action' rule, got %d", len(pane.Nodes))
	}

	if pane.Nodes[0].Predicate != "next_action" {
		t.Errorf("Expected next_action, got %s", pane.Nodes[0].Predicate)
	}
}

func TestApplyFilters_SearchQuery_NoMatches(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Search for something that doesn't exist
	pane.SetSearchQuery("nonexistent_predicate")

	if len(pane.Nodes) != 0 {
		t.Errorf("Expected 0 nodes for nonexistent search, got %d", len(pane.Nodes))
	}
}

func TestApplyFilters_FilterSource_EDB(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Filter for EDB only
	pane.SetFilterSource("edb")

	if len(pane.Nodes) != 2 {
		t.Errorf("Expected 2 EDB nodes, got %d", len(pane.Nodes))
	}

	// Verify all are EDB
	for _, node := range pane.Nodes {
		if node.Source != "edb" {
			t.Errorf("Expected EDB source, got %s for %s", node.Source, node.Predicate)
		}
	}
}

func TestApplyFilters_FilterSource_IDB(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Filter for IDB only
	pane.SetFilterSource("idb")

	if len(pane.Nodes) != 2 {
		t.Errorf("Expected 2 IDB nodes, got %d", len(pane.Nodes))
	}

	// Verify all are IDB
	for _, node := range pane.Nodes {
		if node.Source != "idb" {
			t.Errorf("Expected IDB source, got %s for %s", node.Source, node.Predicate)
		}
	}
}

func TestApplyFilters_FilterSource_CaseInsensitive(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Test uppercase input is normalized
	pane.SetFilterSource("EDB")

	if len(pane.Nodes) != 2 {
		t.Errorf("Expected 2 EDB nodes with uppercase input, got %d", len(pane.Nodes))
	}
}

func TestApplyFilters_FilterSource_InvalidIgnored(t *testing.T) {
	pane := createTestLogicPaneWithData()

	originalCount := len(pane.Nodes)

	// Try invalid source
	pane.SetFilterSource("invalid")

	// Should not change nodes
	if len(pane.Nodes) != originalCount {
		t.Errorf("Invalid filter should not change nodes, expected %d, got %d", originalCount, len(pane.Nodes))
	}

	// Filter source should not be set
	if pane.FilterSource != "" {
		t.Errorf("Invalid filter should not be set, got %s", pane.FilterSource)
	}
}

func TestApplyFilters_Combined(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Combine search and source filter
	pane.SetSearchQuery("intent")
	pane.SetFilterSource("idb")

	// Should only match next_action (IDB + contains "intent")
	if len(pane.Nodes) != 1 {
		t.Errorf("Expected 1 node with combined filters, got %d", len(pane.Nodes))
	}

	if pane.Nodes[0].Predicate != "next_action" {
		t.Errorf("Expected next_action, got %s", pane.Nodes[0].Predicate)
	}
}

func TestClearFilters(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Apply filters
	pane.SetSearchQuery("intent")
	pane.SetFilterSource("idb")

	if len(pane.Nodes) != 1 {
		t.Errorf("Expected 1 node with filters, got %d", len(pane.Nodes))
	}

	// Clear filters
	pane.ClearFilters()

	// Should have all nodes again
	if len(pane.Nodes) != 4 {
		t.Errorf("Expected 4 nodes after clearing filters, got %d", len(pane.Nodes))
	}

	if pane.SearchQuery != "" {
		t.Errorf("Expected empty SearchQuery after clear, got %s", pane.SearchQuery)
	}

	if pane.FilterSource != "" {
		t.Errorf("Expected empty FilterSource after clear, got %s", pane.FilterSource)
	}
}

func TestHasActiveFilters(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// No filters initially
	if pane.HasActiveFilters() {
		t.Error("Expected no active filters initially")
	}

	// With search query
	pane.SetSearchQuery("test")
	if !pane.HasActiveFilters() {
		t.Error("Expected active filters with search query")
	}

	// Clear and try source filter
	pane.ClearFilters()
	if pane.HasActiveFilters() {
		t.Error("Expected no active filters after clear")
	}

	pane.SetFilterSource("edb")
	if !pane.HasActiveFilters() {
		t.Error("Expected active filters with source filter")
	}

	// Both filters
	pane.SetSearchQuery("test")
	if !pane.HasActiveFilters() {
		t.Error("Expected active filters with both filters")
	}
}

func TestGetFilterStatus(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// No filters
	status := pane.GetFilterStatus()
	if status != "" {
		t.Errorf("Expected empty status with no filters, got %s", status)
	}

	// With search query
	pane.SetSearchQuery("intent")
	status = pane.GetFilterStatus()
	if status == "" {
		t.Error("Expected non-empty status with search query")
	}
	if !contains(status, "Search: \"intent\"") {
		t.Errorf("Expected search query in status, got %s", status)
	}
	if !contains(status, "2/4") {
		t.Errorf("Expected node count in status, got %s", status)
	}

	// With source filter
	pane.ClearFilters()
	pane.SetFilterSource("edb")
	status = pane.GetFilterStatus()
	if !contains(status, "Source: EDB") {
		t.Errorf("Expected source filter in status, got %s", status)
	}

	// Both filters
	pane.SetSearchQuery("user")
	status = pane.GetFilterStatus()
	if !contains(status, "Search:") || !contains(status, "Source:") {
		t.Errorf("Expected both filters in status, got %s", status)
	}
}

func TestApplyFilters_SelectionReset(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Select last node
	pane.SelectedNode = len(pane.Nodes) - 1

	// Apply filter that reduces nodes
	pane.SetSearchQuery("intent")

	// Selection should be reset if out of bounds
	if pane.SelectedNode >= len(pane.Nodes) {
		t.Errorf("Selection not reset, got %d with %d nodes", pane.SelectedNode, len(pane.Nodes))
	}
}

func TestApplyFilters_NilAllNodes(t *testing.T) {
	styles := DefaultStyles()
	pane := NewLogicPane(styles, 100, 50)

	// Don't set any trace, so AllNodes is nil
	pane.applyFilters()

	// Should handle gracefully
	if pane.Nodes != nil {
		t.Error("Expected Nodes to be nil when AllNodes is nil")
	}
}

func TestApplyFilters_EmptySearch(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Empty search should show all nodes
	pane.SetSearchQuery("")

	if len(pane.Nodes) != 4 {
		t.Errorf("Expected 4 nodes with empty search, got %d", len(pane.Nodes))
	}
}

func TestApplyFilters_EmptySource(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Empty source should show all nodes
	pane.SetFilterSource("")

	if len(pane.Nodes) != 4 {
		t.Errorf("Expected 4 nodes with empty source, got %d", len(pane.Nodes))
	}
}

func TestRenderTree_NoMatchesWithFilters(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Apply filter with no matches
	pane.SetSearchQuery("nonexistent")

	// Render tree
	content := pane.renderTree()

	// Should show "No nodes match" message
	if !contains(content, "No nodes match") {
		t.Errorf("Expected 'No nodes match' message, got: %s", content)
	}
}

func TestRenderTree_NoMatchesWithoutFilters(t *testing.T) {
	styles := DefaultStyles()
	pane := NewLogicPane(styles, 100, 50)

	// Create empty trace
	trace := &DerivationTrace{
		Query:       "empty_query()",
		TotalFacts:  0,
		DerivedTime: 0,
		RootNodes:   []*DerivationNode{},
	}
	pane.SetTrace(trace)

	// Render tree
	content := pane.renderTree()

	// Should be empty (no special message without filters)
	if content != "" {
		t.Errorf("Expected empty content without filters and no nodes, got: %s", content)
	}
}

func TestSetSearchQuery_CacheInvalidation(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Get initial content
	content1 := pane.renderContent()

	// Change search query
	pane.SetSearchQuery("intent")

	// Content should be different
	content2 := pane.renderContent()

	if content1 == content2 {
		t.Error("Expected different content after search query change")
	}
}

func TestSetFilterSource_CacheInvalidation(t *testing.T) {
	pane := createTestLogicPaneWithData()

	// Get initial content
	content1 := pane.renderContent()

	// Change filter source
	pane.SetFilterSource("edb")

	// Content should be different
	content2 := pane.renderContent()

	if content1 == content2 {
		t.Error("Expected different content after source filter change")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark filter performance
func BenchmarkApplyFilters_NoFilters(b *testing.B) {
	pane := createTestLogicPaneWithData()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pane.applyFilters()
	}
}

func BenchmarkApplyFilters_SearchOnly(b *testing.B) {
	pane := createTestLogicPaneWithData()
	pane.SearchQuery = "intent"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pane.applyFilters()
	}
}

func BenchmarkApplyFilters_SourceOnly(b *testing.B) {
	pane := createTestLogicPaneWithData()
	pane.FilterSource = "edb"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pane.applyFilters()
	}
}

func BenchmarkApplyFilters_Combined(b *testing.B) {
	pane := createTestLogicPaneWithData()
	pane.SearchQuery = "intent"
	pane.FilterSource = "idb"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pane.applyFilters()
	}
}
