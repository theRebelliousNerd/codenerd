package store

import (
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// Embedding-space identity for the generic vectors table.
//
// A vector is only comparable with a query embedded by the same transformation.
// Equal length proves nothing: two models of the same dimension are different
// coordinate systems, and cosine similarity between them is a number with no
// meaning. So every vectors row is stamped with the identity of the engine that
// produced it (vectors.embedding_model, the engine's Name), and every read path
// -- sqlite-vec and brute force -- compares only rows stamped with the identity
// of the engine that embedded the query. The stamp is bound to the engine that
// produced the bytes, captured before the provider call: an engine swapped while
// the call is in flight never lends its identity to the old engine's vector.
//
// Rows with no stamp (written before the column existed) and rows stamped by
// another engine stay stored, and stay reachable by keyword recall; they do not
// rank in semantic recall until `nerd embedding reembed` produces them in the
// current space. Stamping them with today's configuration would be a guess, so
// nothing does.

// ErrNoEmbeddingIdentity reports an engine whose Name is empty: its vectors
// could not be told apart from another engine's, so none is stored.
var ErrNoEmbeddingIdentity = errors.New("embedding engine reports no identity; its vectors cannot be attributed to an embedding space")

// embeddingSpace is the identity a vector produced by engine is stamped with.
func embeddingSpace(engine embedding.EmbeddingEngine) string {
	if engine == nil {
		return ""
	}
	return strings.TrimSpace(engine.Name())
}

// requireEmbeddingSpace is embeddingSpace for write paths: an empty identity is
// an error, never a silently unattributable row.
func requireEmbeddingSpace(engine embedding.EmbeddingEngine) (string, error) {
	space := embeddingSpace(engine)
	if space == "" {
		return "", ErrNoEmbeddingIdentity
	}
	return space, nil
}

// semanticQuery is a query vector with the space it was embedded in. Only rows
// stamped with that space are comparable with it.
type semanticQuery struct {
	vec   []float32
	space string
}

// newSemanticQuery binds a query vector to the identity of the engine that
// embedded it.
func newSemanticQuery(engine embedding.EmbeddingEngine, vec []float32) (semanticQuery, error) {
	space, err := requireEmbeddingSpace(engine)
	if err != nil {
		return semanticQuery{}, err
	}
	return semanticQuery{vec: vec, space: space}, nil
}

// currentSpaceLocked is the identity of the attached engine ("" when none).
// Call holding s.mu (read or write).
func (s *LocalStore) currentSpaceLocked() string {
	return embeddingSpace(s.embeddingEngine)
}

// vectorEligibleClause is the SQL predicate for a vectors row that may be
// compared against a query embedded in one space. It takes the space as its
// single bound argument. Every semantic read path, the vec_index builders and
// the index-drift measure share it.
const vectorEligibleClause = "embedding IS NOT NULL AND embedding_model = ?"

// vecEligibleRowidFilter restricts a vec_index scan to rowids whose canonical
// vectors row is eligible. The index is only ever built from eligible rows, so
// this is the read-side half of the invariant: a stale row left in the index by
// a failed clear, a crash or an older build still cannot be ranked.
const vecEligibleRowidFilter = "rowid IN (SELECT id FROM vectors WHERE " + vectorEligibleClause + ")"

// VectorCoverage says how much of the vectors table the attached engine can
// serve semantically, and why the rest cannot.
type VectorCoverage struct {
	// Space is the identity coverage was measured against.
	Space string
	// Total is every row with text to embed. A row with no text has no
	// meaning to rank and is counted in Blank instead, so a migration can
	// reach a state in which nothing is pending.
	Total int64
	// Blank rows hold no text.
	Blank int64
	// Eligible rows carry an embedding stamped with Space.
	Eligible int64
	// Unembedded rows have no embedding at all (keyword-only).
	Unembedded int64
	// Unattributed rows have an embedding and no stamp: written before the
	// stamp existed, or by a writer that did not record one.
	Unattributed int64
	// Foreign rows were embedded in another space.
	Foreign int64
}

// Unqualified is the number of embedded rows that cannot rank because their
// space is unknown or different.
func (c VectorCoverage) Unqualified() int64 { return c.Unattributed + c.Foreign }

// Pending is every embeddable row a re-embed still has to bring into Space.
func (c VectorCoverage) Pending() int64 { return c.Total - c.Eligible }

// VectorCoverage measures the vectors table against the attached engine's
// space. It errors when no engine is attached: coverage is relative to one.
func (s *LocalStore) VectorCoverage() (VectorCoverage, error) {
	if s == nil {
		return VectorCoverage{}, errors.New("nil store")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	space := s.currentSpaceLocked()
	if space == "" {
		return VectorCoverage{}, errors.New("no embedding engine attached: coverage is relative to one")
	}
	return s.vectorCoverageLocked(space)
}

func (s *LocalStore) vectorCoverageLocked(space string) (VectorCoverage, error) {
	cov := VectorCoverage{Space: space}
	err := s.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN TRIM(content) != '' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN TRIM(content) = '' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN TRIM(content) != '' AND embedding IS NULL THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN TRIM(content) != '' AND embedding IS NOT NULL AND embedding_model = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN TRIM(content) != '' AND embedding IS NOT NULL AND (embedding_model IS NULL OR embedding_model = '') THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN TRIM(content) != '' AND embedding IS NOT NULL AND embedding_model IS NOT NULL AND embedding_model != '' AND embedding_model != ? THEN 1 ELSE 0 END), 0)
		FROM vectors`, space, space).Scan(&cov.Total, &cov.Blank, &cov.Unembedded, &cov.Eligible, &cov.Unattributed, &cov.Foreign)
	if err != nil {
		return VectorCoverage{}, fmt.Errorf("measure vector coverage: %w", err)
	}
	return cov, nil
}

// noteExcludedLocked tells the log, once per embedding space, that rows are
// left out of semantic ranking for want of a compatible space. Storage applies
// the compatibility gate; whether thin semantic coverage warrants a keyword
// fallback or a caveat is the caller's decision, so the exclusion is disclosed
// rather than papered over. The count is a table scan, which is why it runs
// once per space and not per recall; GetVectorStats and VectorCoverage report
// it on demand. Call holding s.mu (read or write).
func (s *LocalStore) noteExcludedLocked(space string) {
	s.noticeMu.Lock()
	seen := s.noticedSpace == space
	s.noticedSpace = space
	s.noticeMu.Unlock()
	if seen {
		return
	}
	cov, err := s.vectorCoverageLocked(space)
	if err != nil || cov.Unqualified() == 0 {
		return
	}
	logging.Get(logging.CategoryStore).Warn(
		"semantic recall in space %q ranks %d of %d vectors: %d are unattributed (written before the embedding space was recorded) and %d belong to another space; they stay stored and keyword-recallable, and `nerd embedding reembed` brings them into this space",
		cov.Space, cov.Eligible, cov.Total, cov.Unattributed, cov.Foreign)
}
