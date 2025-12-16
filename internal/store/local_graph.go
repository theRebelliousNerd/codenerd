package store

import (
	"codenerd/internal/logging"
	"encoding/json"
	"fmt"
)

// =============================================================================
// KNOWLEDGE GRAPH (Shard C)
// =============================================================================

// KnowledgeLink represents a graph edge.
type KnowledgeLink struct {
	EntityA  string
	Relation string
	EntityB  string
	Weight   float64
	Metadata map[string]interface{}
}

// StoreLink stores a knowledge graph edge.
func (s *LocalStore) StoreLink(entityA, relation, entityB string, weight float64, metadata map[string]interface{}) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreLink")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing graph link: %s -[%s]-> %s (weight=%.2f)", entityA, relation, entityB, weight)

	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO knowledge_graph (entity_a, relation, entity_b, weight, metadata)
		 VALUES (?, ?, ?, ?, ?)`,
		entityA, relation, entityB, weight, string(metaJSON),
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store graph link: %v", err)
		return err
	}

	logging.StoreDebug("Graph link stored successfully")
	return nil
}

// QueryLinks retrieves links for an entity.
func (s *LocalStore) QueryLinks(entity string, direction string) ([]KnowledgeLink, error) {
	timer := logging.StartTimer(logging.CategoryStore, "QueryLinks")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Querying graph links for entity=%q direction=%s", entity, direction)

	var query string
	switch direction {
	case "outgoing":
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_a = ?"
	case "incoming":
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_b = ?"
	default: // both
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_a = ? OR entity_b = ?"
	}

	var args []interface{}
	if direction == "both" {
		args = []interface{}{entity, entity}
	} else {
		args = []interface{}{entity}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Graph query failed for entity=%q: %v", entity, err)
		return nil, err
	}
	defer rows.Close()

	var links []KnowledgeLink
	for rows.Next() {
		var link KnowledgeLink
		var metaJSON string
		if err := rows.Scan(&link.EntityA, &link.Relation, &link.EntityB, &link.Weight, &metaJSON); err != nil {
			continue
		}
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &link.Metadata)
		}
		links = append(links, link)
	}

	logging.StoreDebug("Graph query returned %d links", len(links))
	return links, nil
}

// TraversePath finds a path between two entities using BFS.
func (s *LocalStore) TraversePath(from, to string, maxDepth int) ([]KnowledgeLink, error) {
	timer := logging.StartTimer(logging.CategoryStore, "TraversePath")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 5
	}

	logging.StoreDebug("Graph traversal: %s -> %s (maxDepth=%d)", from, to, maxDepth)

	// BFS traversal
	type pathNode struct {
		entity string
		path   []KnowledgeLink
	}

	visited := make(map[string]bool)
	queue := []pathNode{{entity: from, path: nil}}

	for len(queue) > 0 && len(queue[0].path) < maxDepth {
		current := queue[0]
		queue = queue[1:]

		if visited[current.entity] {
			continue
		}
		visited[current.entity] = true

		if current.entity == to {
			logging.StoreDebug("Path found with %d hops, visited %d nodes", len(current.path), len(visited))
			return current.path, nil
		}

		links, err := s.QueryLinks(current.entity, "outgoing")
		if err != nil {
			continue
		}

		for _, link := range links {
			if !visited[link.EntityB] {
				newPath := make([]KnowledgeLink, len(current.path)+1)
				copy(newPath, current.path)
				newPath[len(current.path)] = link
				queue = append(queue, pathNode{entity: link.EntityB, path: newPath})
			}
		}
	}

	logging.StoreDebug("No path found from %s to %s (visited %d nodes)", from, to, len(visited))
	return nil, fmt.Errorf("no path found from %s to %s", from, to)
}

// HydrateKnowledgeGraph loads all knowledge graph entries and converts them to
// knowledge_link facts for injection into the Mangle kernel.
// This method should be called during kernel initialization or when the knowledge
// graph is updated to ensure facts are available to Mangle rules.
func (s *LocalStore) HydrateKnowledgeGraph(assertFunc func(predicate string, args []interface{}) error) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "HydrateKnowledgeGraph")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.Store("Hydrating knowledge graph into Mangle kernel")

	// Query all knowledge graph entries
	rows, err := s.db.Query(
		`SELECT entity_a, relation, entity_b, weight FROM knowledge_graph ORDER BY weight DESC`,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query knowledge graph for hydration: %v", err)
		return 0, fmt.Errorf("failed to query knowledge graph: %w", err)
	}
	defer rows.Close()

	count := 0
	skipped := 0
	for rows.Next() {
		var entityA, relation, entityB string
		var weight float64
		if err := rows.Scan(&entityA, &relation, &entityB, &weight); err != nil {
			skipped++
			continue // Skip malformed entries
		}

		// Convert to Mangle fact: knowledge_link(entity_a, relation, entity_b)
		if err := assertFunc("knowledge_link", []interface{}{entityA, relation, entityB}); err == nil {
			count++
		} else {
			skipped++
		}
	}

	logging.Store("Knowledge graph hydration complete: asserted=%d, skipped=%d", count, skipped)
	return count, nil
}
