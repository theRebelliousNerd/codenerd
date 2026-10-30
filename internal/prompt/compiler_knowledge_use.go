package prompt

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
)

// KnowledgeLookup is what one knowledge search of a compile returned: the
// rows it found, or that it failed (an error or the deadline).
type KnowledgeLookup struct {
	Found  int
	Failed bool
}

// knowledgeLookups collects a compile's knowledge searches; the expert
// searches run concurrently with each other's callers. A nil ledger records
// nothing.
type knowledgeLookups struct {
	mu   sync.Mutex
	byKB map[string]KnowledgeLookup
}

func (l *knowledgeLookups) record(key string, found int, err error) {
	if l == nil || key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byKB == nil {
		l.byKB = map[string]KnowledgeLookup{}
	}
	l.byKB[key] = KnowledgeLookup{Found: found, Failed: err != nil}
}

func (l *knowledgeLookups) snapshot() map[string]KnowledgeLookup {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return maps.Clone(l.byKB)
}

// recordKnowledgeConsumption writes what a compile consumed of each shard
// agent's knowledge into that agent's KB, from what each search returned:
//   - rows the prompt carries: their use (knowledge_usage), and the task's
//     question recovered (a miss recorded for it before no longer stands);
//   - a search that succeeded and found nothing: the question as a miss
//     (knowledge_misses), the gap the refresh researches;
//   - a search that found rows the prompt does not carry (selection or the
//     budget left them out), or that failed: nothing. Neither says the KB
//     lacks the knowledge.
//
// Until 2026-10-30 every consulted KB whose rows the final prompt lacked
// recorded a miss, so a row the budget squeezed out read as a corpus gap, and
// a later compile that delivered it left the miss standing for the refresh
// to research. A cache hit records too: that turn used the rows as much as
// the one that compiled them.
func (c *JITPromptCompiler) recordKnowledgeConsumption(ctx context.Context, cc *CompilationContext, result *CompilationResult) {
	if result == nil || len(result.KnowledgeLookups) == 0 {
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
	for _, key := range slices.Sorted(maps.Keys(result.KnowledgeLookups)) {
		lookup := result.KnowledgeLookups[key]
		src := c.snapshotSources(&CompilationContext{ShardID: key})
		db := src.shardDB
		if db == nil {
			src.release()
			continue
		}
		var err error
		switch concepts := used[key]; {
		case len(concepts) > 0:
			err = knowledge.RecordRetrieval(ctx, db, concepts, now)
			if err == nil {
				err = knowledge.RecordRecovery(ctx, db, query, now)
			}
		case lookup.Failed:
			logging.Get(logging.CategoryJIT).Debug("knowledge search of %s failed; no miss is recorded for it", key)
		case lookup.Found > 0:
			logging.Get(logging.CategoryJIT).Debug("knowledge search of %s found %d row(s) the prompt left out; not a miss", key, lookup.Found)
		default:
			err = knowledge.RecordMiss(ctx, db, query, now)
		}
		src.release()
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("knowledge consumption of %s not recorded: %v", key, err)
		}
	}
}
