package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/store"
)

// Pushed shard experts.
//
// CompilationContext.Experts names the shard agents the kernel derived a
// compile needs (expert_needed, policy/shard_experts.mg, read by
// internal/experts). Their knowledge reaches the compile whoever is speaking:
// a /coder turn on a Bubble Tea file carries the Bubble Tea expert's guidance
// without the model having to think of consulting it.
//
// A pushed expert lends its knowledge, never its voice. Its identity atoms
// ("You are BubbleTeaExpert ...") and the knowledge rows that describe the
// agent itself are left out: the compile's own persona speaks, and a second
// identity in one prompt replaces the agent rather than informing it (the
// identity-leak class jit_compiler.mg's regime dimensions exist to stop). Its
// methodology and domain atoms enter the candidate pool, where the vector
// channel admits the ones relevant to the task like any other flesh; its
// knowledge rows are retrieved for the task and enter at full score, exactly
// as the speaking expert's own rows do.

// expertSource is one pushed expert's registered knowledge DB.
type expertSource struct {
	key string
	db  *sql.DB
}

// selfDescriptionConcepts are knowledge rows that describe the agent itself
// rather than its subject: the baseline seed init writes into every agent KB
// (internal/init/agents_knowledge_seed.go baselineAgentAtoms). They belong to
// the agent's own compiles only.
var selfDescriptionConcepts = map[string]bool{
	"agent_identity": true,
	"agent_mission":  true,
}

// expertSources resolves cc.Experts to registered DBs, skipping the compile's
// own speaker (already the shard source) and any agent with no registered DB,
// in name order so the source vector is stable.
func (c *JITPromptCompiler) expertSources(cc *CompilationContext, speakerKey string) []expertSource {
	if cc == nil || len(cc.Experts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(cc.Experts))
	seen := map[string]bool{}
	for _, name := range cc.Experts {
		key := shardDBKey(name)
		if key == "" || key == speakerKey || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	c.shardMu.RLock()
	defer c.shardMu.RUnlock()
	out := make([]expertSource, 0, len(keys))
	for _, key := range keys {
		if db := c.shardDBs[key]; db != nil {
			out = append(out, expertSource{key: key, db: db})
		}
	}
	return out
}

// expertSourceName is a pushed expert's source name in the source vector and
// the catalog: the same name its DB carries when it is the speaker, since the
// DB and its atoms are the same.
func expertSourceName(key string) string { return "expert:" + key }

// lendable reports whether a pushed expert's atom may enter another persona's
// compile: everything but its identity.
func lendable(atom *PromptAtom) bool {
	return atom != nil && atom.Category != CategoryIdentity
}

// collectExpertAtoms loads each pushed expert's lendable atoms.
func (c *JITPromptCompiler) collectExpertAtoms(ctx context.Context, src compileSources, vector sourceVector, pending *pendingCatalogs, degraded *degradation) []*PromptAtom {
	var out []*PromptAtom
	for _, expert := range src.experts {
		name := expertSourceName(expert.key)
		atoms, err := c.loadCatalogAtoms(ctx, name, expert.db, vector.parts[name], pending)
		if err != nil {
			degraded.add(name)
			continue
		}
		for _, atom := range atoms {
			if lendable(atom) {
				out = append(out, atom)
			}
		}
	}
	return out
}

// collectPushedExpertKnowledge retrieves each pushed expert's knowledge rows
// for the task, without the rows that describe the agent itself. The atoms
// carry no shard tag: they serve the compile's own speaker.
func collectPushedExpertKnowledge(ctx context.Context, experts []expertSource, query string, degraded *degradation) map[string][]store.KnowledgeAtom {
	if len(experts) == 0 || strings.TrimSpace(query) == "" {
		return nil
	}
	out := make(map[string][]store.KnowledgeAtom, len(experts))
	for _, expert := range experts {
		if ctx.Err() != nil {
			degraded.add(expertSourceName(expert.key))
			return out
		}
		atoms, err := store.SearchKnowledgeAtomsLexicalDB(ctx, expert.db, query, knowledgeRowsPerSource)
		if err != nil {
			degraded.add(expertSourceName(expert.key))
			continue
		}
		kept := atoms[:0]
		for _, atom := range atoms {
			if !selfDescriptionConcepts[atom.Concept] {
				kept = append(kept, atom)
			}
		}
		if len(kept) > 0 {
			out[expert.key] = kept
		}
	}
	return out
}

// ShardTechnologies returns the technologies a registered agent's knowledge
// base claims: the distinct languages: and frameworks: tags on its atoms,
// lower-cased and without the leading slash, sorted. An agent with no
// registered DB is an error; a DB with no such tags claims nothing.
func (c *JITPromptCompiler) ShardTechnologies(ctx context.Context, shardID string) ([]string, error) {
	db, ok := c.LookupShardDB(shardID)
	if !ok || db == nil {
		return nil, fmt.Errorf("no knowledge DB registered for %q", shardID)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT tag FROM atom_context_tags WHERE dimension IN ('lang', 'framework') AND is_exclusion = 0`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("read technologies of %q: %w", shardID, err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("read technologies of %q: %w", shardID, err)
		}
		tech := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(tag), "/"))
		if tech != "" && !seen[tech] {
			seen[tech] = true
			out = append(out, tech)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read technologies of %q: %w", shardID, err)
	}
	sort.Strings(out)
	return out, nil
}
