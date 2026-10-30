package store

import (
	"fmt"
	"math"

	"encoding/json"

	"codenerd/internal/logging"
)

// =============================================================================
// THE WORLD-SCAN PROJECTION OF THE KNOWLEDGE GRAPH
// =============================================================================
// Every scanner projects its dependency_link and symbol_graph facts into
// knowledge_graph as depends_on and defined_in edges
// (VirtualStore.ReplaceLinkFacts). Those edges are derived: when a refresh
// supersedes a file's facts, the edges its old facts supported must go too,
// or a removed import keeps answering graph queries and the session's
// dependency context as if it were current (audit
// scanner-graph-source-retirement). Upsert alone never retires anything.
//
// A row belongs to the projection when its metadata says so: projection
// "world_facts", which every projected edge now carries, or, for a row
// written before that marker, one of the source labels only the projection
// ever wrote. Memory-operation and ingest edges carry neither and are never
// retired by a scan.

// WorldLinkProjection marks a knowledge_graph row as derived from world facts.
const WorldLinkProjection = "world_facts"

// legacyWorldLinkSources are the metadata source labels the world projection
// wrote before WorldLinkProjection existed (scan, deep scan, partial file and
// directory scans, the world-model shard). No other writer uses them.
var legacyWorldLinkSources = []any{"scan", "scan-deep", "scan-path", "scan-dir", "world_model"}

// ReplaceWorldLinks retires the projection's edges in retire and publishes
// the edges in publish, in one transaction. An edge in retire is deleted only
// when the stored row belongs to the projection (see above); a caller passes
// in retire the edges its superseded facts supported and that its current
// facts no longer do.
// The caller must own observation-to-publication ordering: this mutex orders
// transaction arrival and cannot detect a delayed delta from an older scan.
func (s *LocalStore) ReplaceWorldLinks(retire, publish []KnowledgeLink) error {
	if len(retire) == 0 && len(publish) == 0 {
		return nil
	}
	for _, l := range publish {
		if l.EntityA == "" || l.Relation == "" || l.EntityB == "" {
			return fmt.Errorf("invalid knowledge graph link: entityA/relation/entityB must be non-empty")
		}
		if math.IsNaN(l.Weight) || math.IsInf(l.Weight, 0) {
			return fmt.Errorf("invalid knowledge graph link weight: %v", l.Weight)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if len(retire) > 0 {
		del, err := tx.Prepare(`DELETE FROM knowledge_graph
			WHERE entity_a = ? AND relation = ? AND entity_b = ?
			  AND (json_extract(metadata, '$.projection') = ?
			       OR json_extract(metadata, '$.source') IN (?, ?, ?, ?, ?))`)
		if err != nil {
			return err
		}
		defer del.Close()
		var retired int64
		for _, l := range retire {
			args := append([]any{l.EntityA, l.Relation, l.EntityB, WorldLinkProjection}, legacyWorldLinkSources...)
			res, err := del.Exec(args...)
			if err != nil {
				return fmt.Errorf("retire %s -[%s]-> %s: %w", l.EntityA, l.Relation, l.EntityB, err)
			}
			if n, err := res.RowsAffected(); err == nil {
				retired += n
			}
		}
		logging.StoreDebug("ReplaceWorldLinks: retired %d of %d superseded edges", retired, len(retire))
	}

	if len(publish) > 0 {
		ins, err := tx.Prepare(`INSERT OR REPLACE INTO knowledge_graph (entity_a, relation, entity_b, weight, metadata)
			VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer ins.Close()
		for _, l := range publish {
			meta := make(map[string]any, len(l.Metadata)+1)
			for k, v := range l.Metadata {
				meta[k] = v
			}
			meta["projection"] = WorldLinkProjection
			metaJSON, err := json.Marshal(meta)
			if err != nil {
				return fmt.Errorf("marshal knowledge graph metadata: %w", err)
			}
			if _, err := ins.Exec(l.EntityA, l.Relation, l.EntityB, l.Weight, string(metaJSON)); err != nil {
				return fmt.Errorf("publish %s -[%s]-> %s: %w", l.EntityA, l.Relation, l.EntityB, err)
			}
		}
	}
	return tx.Commit()
}
