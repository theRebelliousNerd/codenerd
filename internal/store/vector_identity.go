package store

import (
	"context"
	"database/sql"
	"encoding/json"
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
// Rows with no stamp (written before the column existed) stay stored and
// keyword-recallable, and the first semantic recall in a space re-embeds them
// once with the attached engine (healUnattributedVectors) so they rank after
// that. Rows stamped by another engine stay out until `nerd embedding reembed`
// produces them in the current space. Stamping an unstamped row with today's
// configuration without re-embedding would be a guess, so nothing does: the
// heal writes a fresh vector it just computed, never a claim about the old one.

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

// healUnattributedVectors re-embeds rows written before the embedding space was
// recorded, stamping them with the space they were re-embedded in.
//
// A row with an embedding and no stamp cannot be compared against any query:
// equal length proves nothing across coordinate systems, so the read paths
// exclude it and noteExcludedLocked warns once per space. The old contract left
// it there forever until `nerd embedding reembed` ran. This is the automatic
// half of that: the first semantic recall in a space that still has unattributed
// rows re-embeds exactly those rows with the attached engine, so a later recall
// ranks all of them and the warning stops.
//
// It never guesses: foreign rows (stamped with another space) are untouched,
// blank rows have no meaning to rank, and a row its provider refuses keeps its
// old embedding and its missing stamp. Publication is conditional on the row
// still being unattributed and holding the same content the vector was computed
// from, so a concurrent heal or a concurrent store cannot stamp a vector onto
// the wrong text.
//
// It returns what it healed, the rows its provider refused (they are not
// retried automatically), and the watermark: the highest row id when it
// scanned, so a later check finds legacy rows that arrived after it. skip
// are rows already refused, left out of the scan.
func (s *LocalStore) healUnattributedVectors(ctx context.Context, engine embedding.EmbeddingEngine, space string, skip map[int64]bool) (healed int, refused []int64, watermark int64, err error) {
	if s == nil || s.db == nil || engine == nil || space == "" {
		return 0, nil, 0, nil
	}
	type pendingRow struct {
		id       int64
		content  string
		metadata string
	}
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM vectors`).Scan(&watermark); err != nil {
		return 0, nil, 0, err
	}
	rows, err := s.db.Query(`SELECT id, content, metadata FROM vectors WHERE id <= ? AND TRIM(content) != '' AND embedding IS NOT NULL AND (embedding_model IS NULL OR embedding_model = '')`, watermark)
	if err != nil {
		return 0, nil, watermark, err
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		var meta sql.NullString
		if err := rows.Scan(&r.id, &r.content, &meta); err != nil {
			continue
		}
		if skip[r.id] {
			continue
		}
		if meta.Valid {
			r.metadata = meta.String
		}
		pending = append(pending, r)
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		return 0, nil, watermark, rowsErr
	}
	if len(pending) == 0 {
		return 0, nil, watermark, nil
	}
	const batchSize = 32
	for i := 0; i < len(pending); i += batchSize {
		if err := ctx.Err(); err != nil {
			return healed, refused, watermark, err
		}
		end := i + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]
		texts := make([]string, len(batch))
		taskTypes := make([]string, len(batch))
		uniform := true
		for j, r := range batch {
			texts[j] = r.content
			var meta map[string]any
			if r.metadata != "" {
				_ = json.Unmarshal([]byte(r.metadata), &meta)
			}
			taskTypes[j] = embedding.GetOptimalTaskType(r.content, meta, false)
			if j > 0 && taskTypes[j] != taskTypes[0] {
				uniform = false
			}
		}
		var embeddings [][]float32
		if uniform && taskTypes[0] != "" {
			if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok {
				embeddings, err = batchAware.EmbedBatchWithTask(ctx, texts, taskTypes[0])
			} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
				embeddings = make([][]float32, len(batch))
				for j, txt := range batch {
					vec, embedErr := taskAware.EmbedWithTask(ctx, txt.content, taskTypes[0])
					if embedErr != nil {
						logging.Get(logging.CategoryStore).Warn("heal unattributed vector %d: provider refused: %v", txt.id, embedErr)
						continue
					}
					embeddings[j] = vec
				}
				err = nil
			} else {
				embeddings, err = engine.EmbedBatch(ctx, texts)
			}
		} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
			embeddings = make([][]float32, len(batch))
			for j, txt := range batch {
				vec, embedErr := taskAware.EmbedWithTask(ctx, txt.content, taskTypes[j])
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("heal unattributed vector %d: provider refused: %v", txt.id, embedErr)
					continue
				}
				embeddings[j] = vec
			}
			err = nil
		} else {
			embeddings, err = engine.EmbedBatch(ctx, texts)
		}
		if err != nil {
			embeddings = make([][]float32, len(batch))
			for j, txt := range batch {
				if err := ctx.Err(); err != nil {
					return healed, refused, watermark, err
				}
				var vec []float32
				var embedErr error
				if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
					vec, embedErr = taskAware.EmbedWithTask(ctx, txt.content, taskTypes[j])
				} else {
					vec, embedErr = engine.Embed(ctx, txt.content)
				}
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("heal unattributed vector %d: provider refused: %v", txt.id, embedErr)
					continue
				}
				embeddings[j] = vec
			}
		}
		s.mu.Lock()
		// The batch is stamped with the producing engine's space, captured by the
		// caller before the provider calls; an engine swapped mid-heal does not
		// lend its identity to the old engine's vectors, and its rows never enter
		// the new engine's index.
		indexable := s.vectorExt.Load() && tableExists(s.db, "vec_index") && s.currentSpaceLocked() == space
		for j, txt := range batch {
			if j >= len(embeddings) || len(embeddings[j]) == 0 {
				refused = append(refused, txt.id)
				continue
			}
			embeddingJSON, encErr := encodeEmbedding(embeddings[j])
			if encErr != nil {
				logging.Get(logging.CategoryStore).Warn("heal unattributed vector %d: %v", txt.id, encErr)
				refused = append(refused, txt.id)
				continue
			}
			res, execErr := s.db.Exec(`UPDATE vectors SET embedding = ?, embedding_model = ? WHERE id = ? AND content = ? AND (embedding_model IS NULL OR embedding_model = '')`, embeddingJSON, space, txt.id, txt.content)
			if execErr != nil {
				s.mu.Unlock()
				return healed, refused, watermark, execErr
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				continue
			}
			if indexable {
				vecBlob := encodeFloat32Slice(embeddings[j])
				_, _ = s.db.Exec(`DELETE FROM vec_index WHERE rowid = ?`, txt.id)
				if _, vecErr := s.db.Exec(`INSERT INTO vec_index (rowid, embedding, content, metadata) VALUES (?, ?, ?, ?)`, txt.id, vecBlob, txt.content, txt.metadata); vecErr != nil {
					logging.Get(logging.CategoryStore).Warn("heal vec_index insert failed for rowid=%d: %v (ANN drift)", txt.id, vecErr)
					continue
				}
			}
			healed++
		}
		s.mu.Unlock()
	}
	if healed > 0 {
		logging.Store("healed %d unattributed vectors into space %q", healed, space)
	}
	return healed, refused, watermark, nil
}

// healSpaceState is where the automatic heal stands in one space on this
// store. An attempt is not a completion: the claim used to be taken before
// the heal ran and never released, so a heal its caller canceled, or one
// that found nothing before legacy rows arrived, left every later recall
// skipping rows it never reached (audit legacy-vector-repair-attempt-
// lifecycle).
type healSpaceState struct {
	// running: a heal is in flight. A concurrent recall does not wait for
	// it; it ranks what is eligible now, and the exclusion notice says what
	// is left out.
	running bool
	// complete: a pass finished over every row up to watermark.
	complete  bool
	watermark int64
	// refused are rows the provider refused. They keep their missing stamp
	// and are not retried automatically (`nerd embedding reembed` does).
	refused map[int64]bool
	// failed is a store error that stopped a pass. It is not retried on
	// every recall -- a broken database would rescan on each one -- and the
	// explicit migration recovers.
	failed string
}

// maybeHealUnattributed re-embeds pre-space rows before ranking, then lets
// the caller proceed. The first recall in a space runs the heal; a recall
// after a complete pass checks only for legacy rows above its watermark (one
// MAX(id) when nothing was written), so recall pays almost nothing after the
// heal. A pass its caller canceled stays incomplete and the next recall
// resumes it; a provider-refused row is never retried automatically; a store
// error stops automatic passes in that space. A heal failure never fails the
// recall: the eligible rows still rank and the exclusion warning still
// discloses what was left out.
func (s *LocalStore) maybeHealUnattributed(ctx context.Context, engine embedding.EmbeddingEngine, space string) {
	if s == nil || engine == nil || space == "" {
		return
	}
	s.healMu.Lock()
	if s.healState == nil {
		s.healState = make(map[string]*healSpaceState)
	}
	st := s.healState[space]
	if st == nil {
		st = &healSpaceState{refused: map[int64]bool{}}
		s.healState[space] = st
	}
	if st.running || st.failed != "" {
		s.healMu.Unlock()
		return
	}
	if st.complete {
		watermark := st.watermark
		s.healMu.Unlock()
		newer, top, err := s.unattributedAbove(watermark)
		s.healMu.Lock()
		if err == nil && !newer {
			if top > st.watermark {
				st.watermark = top
			}
			s.healMu.Unlock()
			return
		}
		if st.running {
			s.healMu.Unlock()
			return
		}
	}
	st.running = true
	skip := make(map[int64]bool, len(st.refused))
	for id := range st.refused {
		skip[id] = true
	}
	s.healMu.Unlock()

	_, refused, watermark, err := s.healUnattributedVectors(ctx, engine, space, skip)

	s.healMu.Lock()
	defer s.healMu.Unlock()
	st.running = false
	for _, id := range refused {
		st.refused[id] = true
	}
	switch {
	case err == nil:
		st.complete, st.watermark = true, watermark
	case ctx.Err() != nil:
		st.complete = false
		logging.Get(logging.CategoryStore).Warn("unattributed vector heal in space %q stopped (%v); the next recall resumes it", space, err)
	default:
		st.complete, st.failed = false, err.Error()
		logging.Get(logging.CategoryStore).Warn("unattributed vector heal failed in space %q: %v; automatic heal stops here, `nerd embedding reembed` recovers", space, err)
	}
}

// unattributedAbove reports whether a legacy row (unattributed, embedded,
// with content) has id above watermark, and the highest id there is.
func (s *LocalStore) unattributedAbove(watermark int64) (bool, int64, error) {
	var top int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM vectors`).Scan(&top); err != nil {
		return false, 0, err
	}
	if top <= watermark {
		return false, top, nil
	}
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM vectors WHERE id > ? AND TRIM(content) != '' AND embedding IS NOT NULL AND (embedding_model IS NULL OR embedding_model = '') LIMIT 1`, watermark).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, top, nil
	}
	if err != nil {
		return false, top, err
	}
	return true, top, nil
}
