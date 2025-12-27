package world

// GraphQuery defines the interface for querying the World Model graph.
// This interface allows Mangle policies (via Virtual Predicates) to access
// the dependency graph, AST, and file topology without direct coupling.
type GraphQuery interface {
	// QueryGraph performs a query against the world graph.
	// queryType: e.g., "dependencies", "symbols", "callers"
	// params: query-specific parameters
	// Returns: structured result (e.g., []string, []Symbol, etc.)
	QueryGraph(queryType string, params map[string]interface{}) (interface{}, error)
}
