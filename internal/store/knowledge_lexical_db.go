package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
)

// SearchKnowledgeAtomsLexicalDB is a bounded, read-only lexical search over a
// knowledge_atoms table reachable through a shared *sql.DB handle.
//
// The JIT expert bridge owns nothing here: it queries the compiler's shared
// shard handle with QueryContext and never opens a second store or closes a
// DB it does not own. Ranking reuses lexicalScore. Missing tables read as
// missing knowledge (nil, nil). Cancellation aborts via QueryContext.
func SearchKnowledgeAtomsLexicalDB(ctx context.Context, db *sql.DB, query string, limit int) ([]KnowledgeAtom, error) {
	if db == nil {
		return nil, nil
	}
	limit = normalizeLexicalLimit(limit)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keywords := knowledgeLexicalKeywords(query, 8)
	if len(keywords) == 0 {
		return nil, nil
	}
	querySQL, args := buildKnowledgeLexicalQuery(keywords, lexicalFetchLimit(limit))
	rows, err := db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, lexicalQueryError(err)
	}
	defer rows.Close()
	hits, err := scanKnowledgeLexicalRows(rows, keywords)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trimKnowledgeLexicalHits(hits, limit), nil
}

// SearchKnowledgeAtomsLexical runs the same search on a LocalStore handle.
func (s *LocalStore) SearchKnowledgeAtomsLexical(ctx context.Context, query string, limit int) ([]KnowledgeAtom, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SearchKnowledgeAtomsLexicalDB(ctx, s.db, query, limit)
}

type scoredKnowledgeHit struct {
	atom  KnowledgeAtom
	score float64
}

func normalizeLexicalLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 10 {
		return 10
	}
	return limit
}

func lexicalFetchLimit(limit int) int {
	fetch := limit * 3
	if fetch > 30 {
		return 30
	}
	return fetch
}

func lexicalQueryError(err error) error {
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return nil
	}
	return err
}

func buildKnowledgeLexicalQuery(keywords []string, fetchLimit int) (string, []any) {
	conditions := make([]string, 0, len(keywords))
	scores := make([]string, 0, len(keywords))
	args := make([]any, 0, len(keywords)*4+1)
	escape := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
	for _, kw := range keywords {
		condition := `(LOWER(concept) LIKE ? ESCAPE '\' OR LOWER(content) LIKE ? ESCAPE '\')`
		conditions = append(conditions, condition)
		scores = append(scores, "CASE WHEN "+condition+" THEN 1 ELSE 0 END")
		pattern := "%" + escape.Replace(kw) + "%"
		args = append(args, pattern, pattern)
	}
	// Rank relevance before limiting candidates. Otherwise many high-confidence
	// one-word matches can hide the only row matching the whole query.
	args = append(args, args...)
	querySQL := "SELECT concept, content, confidence FROM knowledge_atoms WHERE " +
		strings.Join(conditions, " OR ") +
		" ORDER BY (" + strings.Join(scores, " + ") + ") DESC, confidence DESC, concept, content LIMIT ?"
	return querySQL, append(args, fetchLimit)
}

func scanKnowledgeLexicalRows(rows *sql.Rows, keywords []string) ([]scoredKnowledgeHit, error) {
	var hits []scoredKnowledgeHit
	for rows.Next() {
		var concept, content string
		var confidence sql.NullFloat64
		if err := rows.Scan(&concept, &content, &confidence); err != nil {
			return nil, err
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		atom := KnowledgeAtom{Concept: concept, Content: content, Confidence: confidence.Float64}
		hits = append(hits, scoredKnowledgeHit{atom: atom, score: lexicalScore(concept+" "+content, keywords)})
	}
	return hits, nil
}

func trimKnowledgeLexicalHits(hits []scoredKnowledgeHit, limit int) []KnowledgeAtom {
	if len(hits) == 0 {
		return nil
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].atom.Confidence > hits[j].atom.Confidence
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]KnowledgeAtom, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.atom)
	}
	return out
}

func knowledgeLexicalKeywords(query string, max int) []string {
	if max <= 0 {
		max = 8
	}
	words := strings.Fields(strings.ToLower(query))
	seen := make(map[string]struct{}, len(words))
	out := make([]string, 0, max)
	for _, w := range words {
		w = strings.Trim(w, ".,:;()[]{}<>\"'`!?-_")
		if len(w) < 2 || len(out) >= max {
			continue
		}
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	return out
}
