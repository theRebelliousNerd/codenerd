package prompt

import (
	"context"
	"time"

	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
)

// recordKnowledgeConsumption writes what a compile consumed of each shard
// agent's knowledge into that agent's KB: the rows its prompt carries
// (knowledge_usage), and, for an expert it consulted whose rows none of the
// prompt carries, the task's question as a miss (knowledge_misses). The
// background refresh decides over both (policy/knowledge_refresh.mg): used
// agents are refreshed first, unused acquired rows are pruned, and a miss is
// a gap to research. A cache hit records too: that turn used the rows as
// much as the one that compiled them.
func (c *JITPromptCompiler) recordKnowledgeConsumption(ctx context.Context, cc *CompilationContext, result *CompilationResult) {
	if result == nil || len(result.ExpertsConsulted) == 0 {
		return
	}
	used := map[string][]string{}
	for _, atom := range result.IncludedAtoms {
		if atom != nil && atom.KnowledgeExpert != "" && atom.KnowledgeConcept != "" {
			used[atom.KnowledgeExpert] = append(used[atom.KnowledgeExpert], atom.KnowledgeConcept)
		}
	}
	query := ""
	if cc != nil {
		query = cc.SemanticQuery
	}
	now := time.Now()
	for _, key := range result.ExpertsConsulted {
		db, ok := c.LookupShardDB(key)
		if !ok || db == nil {
			continue
		}
		var err error
		if concepts := used[key]; len(concepts) > 0 {
			err = knowledge.RecordRetrieval(ctx, db, concepts, now)
		} else {
			err = knowledge.RecordMiss(ctx, db, query, now)
		}
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("knowledge consumption of %s not recorded: %v", key, err)
		}
	}
}
