package ui

import (
	"testing"
)

// BenchmarkComputeHash benchmarks the hash computation with mixed inputs
func BenchmarkComputeHash(b *testing.B) {
	// Setup typical inputs for LogicPane cache key
	traceVersion := 1
	width := 100
	height := 50
	showActivation := true
	selectedNode := 123
	scrollOffset := 10
	searchQuery := "some query"
	filterSource := "idb"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeHash(traceVersion, width, height, showActivation, selectedNode, scrollOffset, searchQuery, filterSource)
	}
}

// BenchmarkComputeHashIntegersOnly benchmarks the hash computation with only integers (worst case for allocation)
func BenchmarkComputeHashIntegersOnly(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeHash(1, 2, 3, 4, 5, 6, 7, 8)
	}
}

func BenchmarkRenderCacheCall(b *testing.B) {
    rc := NewRenderCache(100)
    cr := NewCachedRender(rc)

	traceVersion := 1
	width := 100
	height := 50
	showActivation := true
	selectedNode := 123
	scrollOffset := 10
	searchQuery := "some query"
	filterSource := "idb"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // simulate the call in LogicPane.renderContent
		cacheKey := []interface{}{
			traceVersion,
			width,
			height,
			showActivation,
			selectedNode,
			scrollOffset,
			searchQuery,
			filterSource,
		}
        cr.Render(cacheKey, func() string { return "content" })
    }
}

func BenchmarkRenderTree(b *testing.B) {
	styles := NewStyles(LightTheme())
	// Create a logic pane
	lp := NewLogicPane(styles, 100, 50)

	// Populate with dummy nodes
	nodeCount := 1000
	nodes := make([]*DerivationNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = &DerivationNode{
			Predicate: "test_predicate",
			Args: []string{"arg1", "arg2", "arg3"},
			Source: "idb",
			Rule: "some_rule",
			Depth: i % 10,
			Expanded: i % 2 == 0,
			Activation: 0.5,
            Children: []*DerivationNode{},
		}
	}
	lp.Nodes = nodes

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lp.renderTree()
	}
}
