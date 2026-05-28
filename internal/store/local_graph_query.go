package store

import (
	"fmt"

	"codenerd/internal/logging"
)

// LocalStoreGraphAdapter wraps LocalStore to implement types.GraphQuery.
// This bridges the knowledge_graph table to the VirtualStore's Mangle-World
// query_graph virtual predicate, enabling Mangle rules to query the
// knowledge graph via query_graph(Type, Params, Result).
type LocalStoreGraphAdapter struct {
	store *LocalStore
}

// NewLocalStoreGraphAdapter creates a GraphQuery adapter backed by LocalStore.
func NewLocalStoreGraphAdapter(store *LocalStore) *LocalStoreGraphAdapter {
	if store == nil {
		return nil
	}
	return &LocalStoreGraphAdapter{store: store}
}

// QueryGraph performs a query against the knowledge graph.
// Supported query types:
//   - "links": params["arg"] = entity name → returns outgoing links as []string
//   - "incoming": params["arg"] = entity name → returns incoming links as []string
//   - "path": params["arg"] = "from->to" → returns path existence as bool
//   - "relations": params["arg"] = entity name → returns all connected entities (both directions)
func (a *LocalStoreGraphAdapter) QueryGraph(queryType string, params map[string]any) (any, error) {
	entity, _ := params["arg"].(string)
	if entity == "" {
		return nil, fmt.Errorf("query_graph: missing or empty 'arg' parameter")
	}

	logging.StoreDebug("GraphQuery: type=%s entity=%s", queryType, entity)

	switch queryType {
	case "links", "outgoing":
		links, err := a.store.QueryLinks(entity, "outgoing")
		if err != nil {
			return nil, fmt.Errorf("query_graph links failed: %w", err)
		}
		result := make([]string, 0, len(links))
		for _, l := range links {
			result = append(result, l.EntityB)
		}
		return result, nil

	case "incoming":
		links, err := a.store.QueryLinks(entity, "incoming")
		if err != nil {
			return nil, fmt.Errorf("query_graph incoming failed: %w", err)
		}
		result := make([]string, 0, len(links))
		for _, l := range links {
			result = append(result, l.EntityA)
		}
		return result, nil

	case "relations", "both":
		links, err := a.store.QueryLinks(entity, "both")
		if err != nil {
			return nil, fmt.Errorf("query_graph relations failed: %w", err)
		}
		result := make([]string, 0, len(links))
		for _, l := range links {
			if l.EntityA == entity {
				result = append(result, l.EntityB)
			} else {
				result = append(result, l.EntityA)
			}
		}
		return result, nil

	case "path":
		// Expect "from->to" format in entity string
		parts := splitPathArg(entity)
		if len(parts) != 2 {
			return nil, fmt.Errorf("query_graph path: expected 'from->to' format, got %q", entity)
		}
		path, err := a.store.TraversePath(parts[0], parts[1], 5)
		if err != nil {
			return false, nil // No path found is not an error
		}
		return len(path) > 0, nil

	default:
		return nil, fmt.Errorf("query_graph: unsupported query type %q", queryType)
	}
}

// splitPathArg splits "from->to" into ["from", "to"].
func splitPathArg(s string) []string {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '-' && s[i+1] == '>' {
			return []string{s[:i], s[i+2:]}
		}
	}
	return nil
}
