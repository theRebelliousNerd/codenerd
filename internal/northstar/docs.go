package northstar

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// INGESTED DOCUMENTS + EMBEDDING RELEVANCE
// =============================================================================
//
// The ingested_docs table has existed since the first schema and had no Go API:
// nothing wrote it, nothing read it, and calculateRelevance carried the comment
// "in production, this would use embeddings". The TODO said "implement or
// remove". Implementing it is the right call -- relevance is what decides
// whether an expensive LLM alignment check is worth running, and bag-of-words
// overlap against three vision sentences is a very thin signal for that.

// IngestedDoc is a project document Northstar has read and can score work
// against (a design doc, an RFC, the north-star corpus itself).
type IngestedDoc struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Summary    string    `json:"summary"`
	Relevance  float64   `json:"relevance"`
	IngestedAt time.Time `json:"ingested_at"`
	Embedding  []float32 `json:"-"`
}

// Embedder produces vectors for text. Injected rather than imported so
// northstar stays independent of any particular embedding backend; when no
// Embedder is set the keyword path is used and nothing breaks.
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// IngestDoc stores (or replaces) a document. The ID is derived from the path so
// re-ingesting the same file updates it in place rather than accumulating
// duplicates that would each vote in the relevance average.
func (s *Store) IngestDoc(doc *IngestedDoc) error {
	if doc == nil {
		return fmt.Errorf("doc is nil")
	}
	if strings.TrimSpace(doc.Path) == "" {
		return fmt.Errorf("doc path is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if doc.ID == "" {
		sum := sha256.Sum256([]byte(doc.Path))
		doc.ID = "doc-" + hex.EncodeToString(sum[:])[:16]
	}
	if doc.IngestedAt.IsZero() {
		doc.IngestedAt = time.Now()
	}

	_, err := s.db.Exec(`
		INSERT INTO ingested_docs (id, path, title, content, summary, relevance, ingested_at, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path = excluded.path,
			title = excluded.title,
			content = excluded.content,
			summary = excluded.summary,
			relevance = excluded.relevance,
			ingested_at = excluded.ingested_at,
			embedding = excluded.embedding
	`, doc.ID, doc.Path, doc.Title, doc.Content, doc.Summary, doc.Relevance,
		doc.IngestedAt, encodeEmbedding(doc.Embedding))
	if err != nil {
		return fmt.Errorf("ingest doc %s: %w", doc.Path, err)
	}
	return nil
}

// ListIngestedDocs returns docs ordered by stored relevance, highest first.
func (s *Store) ListIngestedDocs(limit int) ([]IngestedDoc, error) {
	if limit <= 0 {
		return []IngestedDoc{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, path, title, content, summary, relevance, ingested_at, embedding
		FROM ingested_docs
		ORDER BY relevance DESC, ingested_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []IngestedDoc
	for rows.Next() {
		var doc IngestedDoc
		var title, summary sql.NullString
		var embedding []byte
		if err := rows.Scan(&doc.ID, &doc.Path, &title, &doc.Content, &summary,
			&doc.Relevance, &doc.IngestedAt, &embedding); err != nil {
			return nil, fmt.Errorf("scan ingested doc: %w", err)
		}
		doc.Title = title.String
		doc.Summary = summary.String
		doc.Embedding = decodeEmbedding(embedding)
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ingested docs: %w", err)
	}
	return docs, nil
}

// DeleteIngestedDoc removes a document by path.
func (s *Store) DeleteIngestedDoc(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM ingested_docs WHERE path = ?`, path)
	return err
}

// encodeEmbedding stores a float32 vector as little-endian IEEE-754 in the
// BLOB column the schema already declared. A nil vector stores NULL, which is
// how "ingested but not embedded" is represented.
func encodeEmbedding(vec []float32) any {
	if len(vec) == 0 {
		return nil
	}
	buf := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeEmbedding(buf []byte) []float32 {
	if len(buf) < 4 {
		return nil
	}
	vec := make([]float32, len(buf)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return vec
}

// CosineSimilarity returns the cosine of the angle between two vectors, or 0
// when either is empty or degenerate.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// SetEmbedder installs the vector backend used by the embedding relevance path.
func (g *Guardian) SetEmbedder(e Embedder) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.embedder = e
}

// IngestDocument reads text into the Northstar corpus, scoring and (when an
// Embedder is installed) vectorising it against the current vision.
func (g *Guardian) IngestDocument(path, title, content string) (*IngestedDoc, error) {
	if g.store == nil {
		return nil, fmt.Errorf("guardian has no store")
	}
	doc := &IngestedDoc{
		Path:       path,
		Title:      title,
		Content:    content,
		Relevance:  g.calculateRelevance(content),
		IngestedAt: time.Now(),
	}

	g.mu.RLock()
	embedder := g.embedder
	g.mu.RUnlock()
	if embedder != nil {
		if vec, err := embedder.Embed(content); err == nil {
			doc.Embedding = vec
		}
	}

	if err := g.store.IngestDoc(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// DocumentRelevance scores text against the ingested corpus.
//
// With an Embedder installed this is cosine similarity against the best-matching
// embedded document -- the "embedding relevance path" the TODO asked for. With
// no Embedder it degrades to term overlap against document titles and content,
// which is still strictly more signal than the vision-only bag of words, and
// returns (0, false) when the corpus is empty so callers can fall through to
// their existing default rather than treating "no corpus" as "not relevant".
func (g *Guardian) DocumentRelevance(text string) (float64, bool) {
	if g.store == nil || strings.TrimSpace(text) == "" {
		return 0, false
	}
	docs, err := g.store.ListIngestedDocs(maxRelevanceDocs)
	if err != nil || len(docs) == 0 {
		return 0, false
	}

	g.mu.RLock()
	embedder := g.embedder
	g.mu.RUnlock()

	if embedder != nil {
		if queryVec, err := embedder.Embed(text); err == nil && len(queryVec) > 0 {
			best := 0.0
			matched := false
			for _, doc := range docs {
				if len(doc.Embedding) == 0 {
					continue
				}
				if sim := CosineSimilarity(queryVec, doc.Embedding); sim > best {
					best = sim
					matched = true
				}
			}
			if matched {
				return clamp01(best), true
			}
		}
	}

	best := 0.0
	for _, doc := range docs {
		if score := termOverlap(text, doc.Title+" "+doc.Summary+" "+doc.Content); score > best {
			best = score
		}
	}
	return clamp01(best), true
}

const maxRelevanceDocs = 64

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// termOverlap is the fraction of distinct significant terms in source that also
// appear in text.
func termOverlap(text, source string) float64 {
	textLower := strings.ToLower(text)
	seen := make(map[string]struct{})
	matches := 0
	for _, word := range strings.Fields(strings.ToLower(source)) {
		word = strings.Trim(word, ".,:;!?()[]{}\"'`")
		if len(word) <= 3 {
			continue
		}
		if _, dup := seen[word]; dup {
			continue
		}
		seen[word] = struct{}{}
		if strings.Contains(textLower, word) {
			matches++
		}
	}
	if len(seen) == 0 {
		return 0
	}
	return float64(matches) / float64(len(seen))
}

// =============================================================================
// METRICS
// =============================================================================

// AlignmentMetrics is the guardian's operational rollup: how much checking has
// happened, how often it refused, and how aligned the work has been.
type AlignmentMetrics struct {
	TotalChecks      int                     `json:"total_checks"`
	ChecksByResult   map[AlignmentResult]int `json:"checks_by_result"`
	BlockedRate      float64                 `json:"blocked_rate"`
	FailedRate       float64                 `json:"failed_rate"`
	MeanScore        float64                 `json:"mean_score"`
	OverallAlignment float64                 `json:"overall_alignment"`
	ActiveDrift      int                     `json:"active_drift"`
	ResolvedDrift    int                     `json:"resolved_drift"`
	FirstCheck       time.Time               `json:"first_check,omitempty"`
	LastCheck        time.Time               `json:"last_check,omitempty"`
	IngestedDocs     int                     `json:"ingested_docs"`
}

// GetMetrics computes the alignment rollup directly in SQL.
//
// Deliberately not derived from the running Guardian's cached state: the whole
// point of the metric is to describe what the database recorded, including
// checks written by other processes (a campaign run, a CLI invocation).
func (s *Store) GetMetrics() (*AlignmentMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := &AlignmentMetrics{ChecksByResult: map[AlignmentResult]int{}}

	rows, err := s.db.Query(`SELECT result, COUNT(*), AVG(score) FROM alignment_checks GROUP BY result`)
	if err != nil {
		return nil, fmt.Errorf("aggregate alignment checks: %w", err)
	}
	defer rows.Close()

	weightedScore := 0.0
	for rows.Next() {
		var result string
		var count int
		var avgScore sql.NullFloat64
		if err := rows.Scan(&result, &count, &avgScore); err != nil {
			return nil, fmt.Errorf("scan alignment metrics: %w", err)
		}
		m.ChecksByResult[AlignmentResult(result)] = count
		m.TotalChecks += count
		weightedScore += avgScore.Float64 * float64(count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alignment metrics: %w", err)
	}

	if m.TotalChecks > 0 {
		m.MeanScore = weightedScore / float64(m.TotalChecks)
		m.BlockedRate = float64(m.ChecksByResult[AlignmentBlocked]) / float64(m.TotalChecks)
		m.FailedRate = float64(m.ChecksByResult[AlignmentFailed]) / float64(m.TotalChecks)

		// ORDER BY ... LIMIT 1 rather than MIN()/MAX(): go-sqlite3 only applies
		// the DATETIME column affinity to plain column reads, so an aggregate
		// over the same column comes back as a string and fails to scan into a
		// time.Time.
		var first, last sql.NullTime
		if err := s.db.QueryRow(`SELECT timestamp FROM alignment_checks ORDER BY timestamp ASC LIMIT 1`).Scan(&first); err != nil {
			return nil, fmt.Errorf("alignment check window (first): %w", err)
		}
		if err := s.db.QueryRow(`SELECT timestamp FROM alignment_checks ORDER BY timestamp DESC LIMIT 1`).Scan(&last); err != nil {
			return nil, fmt.Errorf("alignment check window (last): %w", err)
		}
		if first.Valid {
			m.FirstCheck = first.Time
		}
		if last.Valid {
			m.LastCheck = last.Time
		}
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM drift_events WHERE resolved = 0`).Scan(&m.ActiveDrift); err != nil {
		return nil, fmt.Errorf("count active drift: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM drift_events WHERE resolved = 1`).Scan(&m.ResolvedDrift); err != nil {
		return nil, fmt.Errorf("count resolved drift: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ingested_docs`).Scan(&m.IngestedDocs); err != nil {
		return nil, fmt.Errorf("count ingested docs: %w", err)
	}
	if err := s.db.QueryRow(`SELECT overall_alignment FROM guardian_state WHERE id = 1`).Scan(&m.OverallAlignment); err != nil {
		return nil, fmt.Errorf("read overall alignment: %w", err)
	}

	return m, nil
}

// GetDriftHistory returns drift events newest first, resolved ones included.
func (s *Store) GetDriftHistory(limit int) ([]DriftEvent, error) {
	if limit <= 0 {
		return []DriftEvent{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, timestamp, severity, category, description, evidence_json,
			related_check, resolved, resolved_at, resolution
		FROM drift_events
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DriftEvent
	for rows.Next() {
		var event DriftEvent
		var evidenceJSON, relatedCheck, resolution sql.NullString
		var resolvedAt sql.NullTime
		var resolved int
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.Severity, &event.Category,
			&event.Description, &evidenceJSON, &relatedCheck, &resolved, &resolvedAt, &resolution); err != nil {
			return nil, fmt.Errorf("scan drift event: %w", err)
		}
		event.RelatedCheck = relatedCheck.String
		event.Resolved = resolved != 0
		event.Resolution = resolution.String
		if resolvedAt.Valid {
			t := resolvedAt.Time
			event.ResolvedAt = &t
		}
		if err := unmarshalJSONField("drift evidence", evidenceJSON, &event.Evidence); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drift history: %w", err)
	}
	return events, nil
}

// SortedResults returns the result buckets in a stable order for display.
func (m *AlignmentMetrics) SortedResults() []AlignmentResult {
	out := make([]AlignmentResult, 0, len(m.ChecksByResult))
	for r := range m.ChecksByResult {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}
