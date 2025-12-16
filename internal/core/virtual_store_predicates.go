package core

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// VIRTUAL PREDICATES - Knowledge Query Handlers
// =============================================================================
// These methods implement virtual predicates for the Mangle kernel,
// enabling logic rules to query the knowledge.db (LocalStore).
// Used during OODA Observe phase to hydrate learned facts into the kernel.

// QueryLearned queries cold_storage for learned facts by predicate name.
// Implements: query_learned(Predicate, Args) Bound
func (v *VirtualStore) QueryLearned(predicate string) ([]Fact, error) {
	logging.VirtualStoreDebug("QueryLearned: predicate=%s", predicate)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("QueryLearned: no knowledge database configured")
		return nil, fmt.Errorf("no knowledge database configured")
	}

	storedFacts, err := db.LoadFacts(predicate)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryLearned failed: %v", err)
		return nil, fmt.Errorf("failed to query learned facts: %w", err)
	}

	facts := make([]Fact, 0, len(storedFacts))
	for _, sf := range storedFacts {
		facts = append(facts, Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
		})
	}

	logging.VirtualStoreDebug("QueryLearned: found %d facts for predicate %s", len(facts), predicate)
	return facts, nil
}

// QueryAllLearned queries all facts from cold_storage.
// Returns facts grouped by fact_type (preference, constraint, fact).
func (v *VirtualStore) QueryAllLearned(factType string) ([]Fact, error) {
	logging.VirtualStoreDebug("QueryAllLearned: factType=%s", factType)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	storedFacts, err := db.LoadAllFacts(factType)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryAllLearned failed: %v", err)
		return nil, fmt.Errorf("failed to query all learned facts: %w", err)
	}

	facts := make([]Fact, 0, len(storedFacts))
	for _, sf := range storedFacts {
		facts = append(facts, Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
		})
	}

	logging.VirtualStoreDebug("QueryAllLearned: found %d facts of type %s", len(facts), factType)
	return facts, nil
}

// PersistFactsToKnowledge stores a batch of facts into knowledge.db cold_storage.
// This is used to mirror on-disk/AST projections into the learning store so
// HydrateLearnings can re-assert them for Mangle logic.
func (v *VirtualStore) PersistFactsToKnowledge(facts []Fact, factType string, priority int) error {
	logging.VirtualStoreDebug("PersistFactsToKnowledge: %d facts, type=%s, priority=%d", len(facts), factType, priority)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.VirtualStoreDebug("PersistFactsToKnowledge: no database, skipping")
		return nil
	}
	if factType == "" {
		factType = "fact"
	}
	if priority <= 0 {
		priority = 5
	}

	for _, f := range facts {
		if err := db.StoreFact(f.Predicate, f.Args, factType, priority); err != nil {
			logging.Get(logging.CategoryVirtualStore).Error("Failed to persist fact %s: %v", f.Predicate, err)
			return fmt.Errorf("persist fact %s: %w", f.Predicate, err)
		}
	}

	logging.VirtualStoreDebug("PersistFactsToKnowledge: persisted %d facts", len(facts))
	return nil
}

// PersistLink stores a relationship into the knowledge graph table.
func (v *VirtualStore) PersistLink(entityA, relation, entityB string, weight float64, meta map[string]interface{}) error {
	logging.VirtualStoreDebug("PersistLink: %s -[%s]-> %s (weight=%.2f)", entityA, relation, entityB, weight)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil
	}
	if weight <= 0 {
		weight = 1.0
	}

	if err := db.StoreLink(entityA, relation, entityB, weight, meta); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to persist link: %v", err)
		return err
	}

	return nil
}

// QueryKnowledgeGraph queries the knowledge graph for entity relationships.
// Implements: query_knowledge_graph(EntityA, Relation, EntityB) Bound
func (v *VirtualStore) QueryKnowledgeGraph(entity, direction string) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	links, err := db.QueryLinks(entity, direction)
	if err != nil {
		return nil, fmt.Errorf("failed to query knowledge graph: %w", err)
	}

	facts := make([]Fact, 0, len(links))
	for _, link := range links {
		facts = append(facts, Fact{
			Predicate: "knowledge_link",
			Args:      []interface{}{link.EntityA, link.Relation, link.EntityB},
		})
	}
	return facts, nil
}

// QueryActivations queries the activation log for recent activation scores.
// Implements: query_activations(FactID, Score) Bound
func (v *VirtualStore) QueryActivations(limit int, minScore float64) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	activations, err := db.GetRecentActivations(limit, minScore)
	if err != nil {
		return nil, fmt.Errorf("failed to query activations: %w", err)
	}

	facts := make([]Fact, 0, len(activations))
	for factID, score := range activations {
		facts = append(facts, Fact{
			Predicate: "activation",
			Args:      []interface{}{factID, score},
		})
	}
	return facts, nil
}

// RecallSimilar performs semantic search on the vectors table.
// Implements: recall_similar(Query, TopK, Results) Bound
func (v *VirtualStore) RecallSimilar(query string, topK int) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	entries, err := db.VectorRecall(query, topK)
	if err != nil {
		return nil, fmt.Errorf("failed semantic recall: %w", err)
	}

	facts := make([]Fact, 0, len(entries))
	for i, entry := range entries {
		facts = append(facts, Fact{
			Predicate: "similar_content",
			Args:      []interface{}{i, entry.Content},
		})
	}
	return facts, nil
}

// QuerySession queries session history for conversation turns.
// Implements: query_session(SessionID, TurnNumber, UserInput) Bound
func (v *VirtualStore) QuerySession(sessionID string, limit int) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	history, err := db.GetSessionHistory(sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	facts := make([]Fact, 0, len(history))
	for _, turn := range history {
		turnNum, _ := turn["turn_number"].(int64)
		userInput, _ := turn["user_input"].(string)
		response, _ := turn["response"].(string)
		facts = append(facts, Fact{
			Predicate: "session_turn",
			Args:      []interface{}{sessionID, turnNum, userInput, response},
		})
	}
	return facts, nil
}

// HasLearned checks if any facts with the given predicate exist in cold_storage.
// Implements: has_learned(Predicate) Bound
func (v *VirtualStore) HasLearned(predicate string) (bool, error) {
	facts, err := v.QueryLearned(predicate)
	if err != nil {
		return false, err
	}
	return len(facts) > 0, nil
}

// QueryTraces queries reasoning_traces for shard execution history.
// Implements: query_traces(ShardType, Limit, TraceID, Success, DurationMs) Bound
func (v *VirtualStore) QueryTraces(shardType string, limit int) ([]Fact, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "QueryTraces")
	defer timer.Stop()

	logging.VirtualStoreDebug("QueryTraces: shardType=%s limit=%d", shardType, limit)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("QueryTraces: no knowledge database configured")
		return nil, fmt.Errorf("no knowledge database configured")
	}

	if limit <= 0 {
		limit = 50
	}

	traces, err := db.GetShardTraces(shardType, limit)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryTraces failed: %v", err)
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}

	facts := make([]Fact, 0, len(traces))
	for _, trace := range traces {
		facts = append(facts, Fact{
			Predicate: "reasoning_trace",
			Args: []interface{}{
				shardType,
				trace.ID,
				trace.Success,
				trace.DurationMs,
			},
		})
	}

	logging.VirtualStoreDebug("QueryTraces: found %d traces for shardType=%s", len(facts), shardType)
	return facts, nil
}

// QueryTraceStats retrieves aggregate statistics for a shard type.
// Implements: query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration) Bound
func (v *VirtualStore) QueryTraceStats(shardType string) ([]Fact, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "QueryTraceStats")
	defer timer.Stop()

	logging.VirtualStoreDebug("QueryTraceStats: shardType=%s", shardType)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("QueryTraceStats: no knowledge database configured")
		return nil, fmt.Errorf("no knowledge database configured")
	}

	stats, err := db.GetTraceStats()
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryTraceStats failed: %v", err)
		return nil, fmt.Errorf("failed to query trace stats: %w", err)
	}

	// Extract stats for the requested shard type
	successRateByType, _ := stats["success_rate_by_type"].(map[string]float64)
	byShardType, _ := stats["by_shard_type"].(map[string]int64)

	totalCount := int64(0)
	successRate := 0.0
	avgDuration := 0.0

	if byShardType != nil {
		if count, ok := byShardType[shardType]; ok {
			totalCount = count
		}
	}
	if successRateByType != nil {
		if rate, ok := successRateByType[shardType]; ok {
			successRate = rate
		}
	}
	if avgDur, ok := stats["avg_duration_ms"].(float64); ok {
		avgDuration = avgDur
	}

	// Calculate success and fail counts from rate
	successCount := int64(float64(totalCount) * successRate)
	failCount := totalCount - successCount

	facts := []Fact{
		{
			Predicate: "trace_stats",
			Args: []interface{}{
				shardType,
				successCount,
				failCount,
				avgDuration,
			},
		},
	}

	logging.VirtualStoreDebug("QueryTraceStats: shardType=%s total=%d success=%d fail=%d avgDur=%.2f",
		shardType, totalCount, successCount, failCount, avgDuration)
	return facts, nil
}

// toAtomOrString converts string to MangleAtom if it starts with /.
func toAtomOrString(v interface{}) interface{} {
	if s, ok := v.(string); ok && strings.HasPrefix(s, "/") {
		return MangleAtom(s)
	}
	return v
}

// HydrateKnowledgeGraph loads knowledge graph entries from LocalStore and hydrates
// the kernel with knowledge_link facts. This can be called independently or as part
// of HydrateLearnings for targeted knowledge graph updates.
func (v *VirtualStore) HydrateKnowledgeGraph(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateKnowledgeGraph")
	defer timer.Stop()

	logging.VirtualStoreDebug("HydrateKnowledgeGraph: starting")

	v.mu.RLock()
	db := v.localDB
	kernel := v.kernel
	v.mu.RUnlock()

	if db == nil {
		logging.VirtualStoreDebug("HydrateKnowledgeGraph: no database, skipping")
		return 0, nil // No database, nothing to hydrate
	}
	if kernel == nil {
		logging.Get(logging.CategoryVirtualStore).Error("HydrateKnowledgeGraph: no kernel configured")
		return 0, fmt.Errorf("no kernel configured")
	}

	// Create assertion function that wraps kernel.Assert
	assertFunc := func(predicate string, args []interface{}) error {
		// Convert args to MangleAtom if needed
		safeArgs := make([]interface{}, len(args))
		for i, arg := range args {
			safeArgs[i] = toAtomOrString(arg)
		}
		return kernel.Assert(Fact{
			Predicate: predicate,
			Args:      safeArgs,
		})
	}

	// Delegate to LocalStore's HydrateKnowledgeGraph
	count, err := db.HydrateKnowledgeGraph(assertFunc)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("HydrateKnowledgeGraph failed: %v", err)
		return 0, fmt.Errorf("failed to hydrate knowledge graph: %w", err)
	}

	logging.VirtualStoreDebug("HydrateKnowledgeGraph: hydrated %d links", count)
	return count, nil
}

// HydrateLearnings loads all learned facts from knowledge.db and asserts them into the kernel.
// This should be called during OODA Observe phase to make learned knowledge available to rules.
func (v *VirtualStore) HydrateLearnings(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateLearnings")
	defer timer.Stop()

	logging.VirtualStore("Hydrating learnings from knowledge.db")

	v.mu.RLock()
	db := v.localDB
	kernel := v.kernel
	v.mu.RUnlock()

	if db == nil {
		logging.VirtualStoreDebug("HydrateLearnings: no database, skipping")
		return 0, nil // No database, nothing to hydrate
	}
	if kernel == nil {
		logging.Get(logging.CategoryVirtualStore).Error("HydrateLearnings: no kernel configured")
		return 0, fmt.Errorf("no kernel configured")
	}

	count := 0

	// Helper to assert with atom conversion
	assertLearned := func(metaPred string, fact Fact) error {
		// Convert args
		safeArgs := make([]interface{}, len(fact.Args))
		for i, arg := range fact.Args {
			safeArgs[i] = toAtomOrString(arg)
		}

		// The predicate itself might be an atom if referenced as data
		predArg := toAtomOrString(fact.Predicate)

		return kernel.Assert(Fact{
			Predicate: metaPred,
			Args:      []interface{}{predArg, safeArgs},
		})
	}

	// 1. Load all preferences (highest priority)
	preferences, err := v.QueryAllLearned("preference")
	if err == nil {
		for _, fact := range preferences {
			if err := assertLearned("learned_preference", fact); err == nil {
				count++
			}
		}
	}

	// 2. Load all user facts
	userFacts, err := v.QueryAllLearned("user_fact")
	if err == nil {
		for _, fact := range userFacts {
			if err := assertLearned("learned_fact", fact); err == nil {
				count++
			}
		}
	}

	// 3. Load all constraints
	constraints, err := v.QueryAllLearned("constraint")
	if err == nil {
		for _, fact := range constraints {
			if err := assertLearned("learned_constraint", fact); err == nil {
				count++
			}
		}
	}

	// 4. Load knowledge graph links (now delegates to dedicated method)
	kgCount, err := v.HydrateKnowledgeGraph(ctx)
	if err == nil {
		count += kgCount
	}

	// 5. Load recent activations (top 50 with score > 0.3)
	activations, err := v.QueryActivations(50, 0.3)
	if err == nil {
		for _, fact := range activations {
			// Activations are direct facts, not meta-facts
			safeArgs := make([]interface{}, len(fact.Args))
			for i, arg := range fact.Args {
				safeArgs[i] = toAtomOrString(arg)
			}
			if err := kernel.Assert(Fact{
				Predicate: fact.Predicate,
				Args:      safeArgs,
			}); err == nil {
				count++
			}
		}
	}

	logging.VirtualStore("HydrateLearnings completed: %d facts hydrated", count)
	return count, nil
}
