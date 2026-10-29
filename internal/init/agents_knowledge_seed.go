package init

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// baselineAgentAtoms builds offline fallback knowledge for an agent from its
// own definition. It needs no LLM and no network, so an agent created with
// --skip-research or on an offline day still gets a searchable knowledge base
// instead of zero atoms.
func baselineAgentAtoms(agent RecommendedAgent) []initKnowledgeAtom {
	name := strings.TrimSpace(agent.Name)
	if name == "" {
		name = "specialist"
	}
	desc := strings.TrimSpace(agent.Description)
	if desc == "" {
		desc = "specialist agent in the codeNERD ecosystem"
	}
	topics := append([]string(nil), agent.Topics...)
	seen := map[string]bool{}
	clean := topics[:0]
	for _, t := range topics {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, t)
	}
	topics = clean
	if len(topics) == 0 {
		topics = []string{"general software engineering"}
	}
	topicsStr := strings.Join(topics, ", ")

	atoms := []initKnowledgeAtom{
		{
			Concept:    "agent_identity",
			Content:    fmt.Sprintf("You are %s, %s. Research topics: %s.", name, desc, topicsStr),
			Confidence: 0.9,
		},
		{
			Concept:    "agent_mission",
			Content:    fmt.Sprintf("%s exists to carry out %s tasks in this workspace. When spawned, read the task, apply %s expertise, and return grounded results.", name, strings.ToLower(name), topicsStr),
			Confidence: 0.85,
		},
		{
			Concept:    "methodology/overview",
			Content:    fmt.Sprintf("%s methodology: decompose the task, locate the relevant code and docs, apply %s best practices, verify with the project's build and tests.", name, topicsStr),
			Confidence: 0.8,
		},
	}
	for i, topic := range topics {
		atoms = append(atoms, initKnowledgeAtom{
			Concept:    fmt.Sprintf("domain/%s", strings.ToLower(strings.ReplaceAll(strings.TrimSpace(topic), " ", "_"))),
			Content:    fmt.Sprintf("%s domain knowledge (%d of %d): %s. Apply %s conventions, check the repository's own docs and code for project-specific rules, and cite what you used.", name, i+1, len(topics), topic, topic),
			Confidence: 0.75,
		})
	}
	return atoms
}

// seedBaselineKnowledge stores baseline atoms for agents whose KB is empty.
// It returns how many atoms were added.
func seedBaselineKnowledge(ctx context.Context, db *store.LocalStore, agent RecommendedAgent, existingHashes map[string]bool) int {
	if existingHashes == nil {
		existingHashes = make(map[string]bool)
	}
	added := 0
	for _, atom := range baselineAgentAtoms(agent) {
		ok, err := appendKnowledgeAtom(ctx, db, atom.Concept, atom.Content, atom.Confidence, "init/baseline", existingHashes)
		if err != nil {
			logging.Boot("Warning: failed to store baseline atom for %s: %v", agent.Name, err)
			continue
		}
		if ok {
			added++
		}
	}
	return added
}

// backfillKnowledgeVectors ensures every knowledge_atoms row has a matching
// vectors row with content_type=knowledge_atom so semantic search can find it.
// This repairs KBs written through StoreKnowledgeAtom (no embedding), which is
// how bubbleteaexpert ended with 73 atoms and 0 vectors.
func backfillKnowledgeVectors(ctx context.Context, db *store.LocalStore) (int, error) {
	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		return 0, err
	}
	if len(atoms) == 0 {
		return 0, nil
	}
	embedded := listEmbeddedKnowledgeKeys(db)
	added := 0
	for _, atom := range atoms {
		if ctx.Err() != nil {
			return added, ctx.Err()
		}
		key := atom.Concept + "\x00" + atom.Content
		if embedded[key] {
			continue
		}
		if err := db.StoreVectorWithEmbedding(ctx, atom.Content, map[string]any{
			"content_type": "knowledge_atom",
			"concept":      atom.Concept,
			"confidence":   atom.Confidence,
		}); err != nil {
			logging.Boot("Warning: backfill vector for %s failed: %v", atom.Concept, err)
			continue
		}
		embedded[key] = true
		added++
	}
	return added, nil
}

// listEmbeddedKnowledgeKeys returns the set of knowledge atoms that already
// have a vectors row. It reads the vectors table directly and filters in Go
// so it works regardless of the metadata JSON shape.
func listEmbeddedKnowledgeKeys(db *store.LocalStore) map[string]bool {
	out := make(map[string]bool)
	conn := db.GetDB()
	if conn == nil {
		return out
	}
	rows, err := conn.Query(`SELECT content, metadata FROM vectors`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var content string
		var meta sql.NullString
		if err := rows.Scan(&content, &meta); err != nil {
			continue
		}
		if !meta.Valid || meta.String == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(meta.String), &m); err != nil {
			continue
		}
		ctype, _ := m["content_type"].(string)
		if ctype != "knowledge_atom" {
			continue
		}
		concept, _ := m["concept"].(string)
		if concept == "" {
			continue
		}
		out[concept+"\x00"+content] = true
	}
	return out
}
