package context

import (
	"database/sql"
	"fmt"
	"math"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"
)

// =============================================================================
// CONTEXT FEEDBACK STORE - LLM-Driven Context Learning
// =============================================================================
// This implements the third feedback loop in codeNERD's learning architecture:
// 1. Tool Learning - learns from tool execution failures
// 2. Prompt Evolution - learns from task verdict failures
// 3. Context Learning (THIS) - learns which facts are useful per task type

// ContextFeedbackStore persists and queries LLM feedback on context usefulness.
// It tracks per-predicate usefulness ratings across intent types, enabling
// the ActivationEngine to boost or penalize predicates based on historical feedback.
type ContextFeedbackStore struct {
	db *sql.DB
	mu sync.RWMutex

	// Cache for frequently queried predicates (predicate -> usefulness score).
	// Entries are invalidated on every StoreFeedback, which is the only write
	// path, so no TTL is needed: the cache cannot outlive the data it mirrors.
	cache   map[string]float64
	cacheMu sync.RWMutex

	// Configuration
	minSamples    int           // Minimum samples before score affects activation
	decayHalfLife time.Duration // Time for feedback weight to halve
}

// StoredFeedback represents a single feedback entry from one LLM turn.
type StoredFeedback struct {
	ID                int64
	TurnID            int
	Timestamp         time.Time
	ManifestHash      string  // Correlate to PromptManifest
	OverallUsefulness float64 // 0.0-1.0
	IntentVerb        string  // e.g., "/fix", "/test", "/review"
	TaskSucceeded     bool    // Ground truth from execution result
}

// PredicateFeedback represents aggregated feedback for a predicate.
type PredicateFeedback struct {
	Predicate     string
	HelpfulCount  int
	NoiseCount    int
	TotalMentions int
	WeightedScore float64 // -1.0 (always noise) to +1.0 (always helpful)
	LastUpdated   time.Time
}

// NewContextFeedbackStore creates a new feedback store.
// dbPath should be ".nerd/context_feedback.db"
func NewContextFeedbackStore(dbPath string) (*ContextFeedbackStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open feedback database: %w", err)
	}
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

	s := &ContextFeedbackStore{
		db:            db,
		cache:         make(map[string]float64),
		minSamples:    10,                 // Conservative: 10 samples before affecting scoring
		decayHalfLife: 7 * 24 * time.Hour, // 7-day half-life for decay
	}

	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// initSchema creates the database tables if they don't exist.
func (s *ContextFeedbackStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS context_feedback (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		turn_id INTEGER NOT NULL,
		timestamp TEXT NOT NULL,
		manifest_hash TEXT,
		overall_usefulness REAL NOT NULL,
		intent_verb TEXT NOT NULL,
		task_succeeded INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS predicate_feedback (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		feedback_id INTEGER NOT NULL REFERENCES context_feedback(id),
		predicate TEXT NOT NULL,
		rating TEXT NOT NULL CHECK(rating IN ('helpful', 'noise'))
	);

	CREATE INDEX IF NOT EXISTS idx_predicate_feedback_predicate
		ON predicate_feedback(predicate);
	CREATE INDEX IF NOT EXISTS idx_context_feedback_intent
		ON context_feedback(intent_verb);
	CREATE INDEX IF NOT EXISTS idx_context_feedback_timestamp
		ON context_feedback(timestamp);
	`

	_, err := s.db.Exec(schema)
	return err
}

// StoreFeedback persists feedback from a single LLM turn.
func (s *ContextFeedbackStore) StoreFeedback(
	turnID int,
	manifestHash string,
	overallUsefulness float64,
	intentVerb string,
	taskSucceeded bool,
	helpfulFacts []string,
	noiseFacts []string,
) error {
	// The usefulness feeds AVG stats and learned scores, so bound it at the
	// door: a producer on a 0-100 scale (or a NaN) would otherwise poison the
	// loop silently. Clamp rather than reject — the write path is async
	// fire-and-forget, so a rejection would vanish into a log while the rest
	// of a good record was lost — but say so loudly when it happens.
	if math.IsNaN(overallUsefulness) {
		overallUsefulness = 0
	}
	clamped := math.Min(1, math.Max(0, overallUsefulness))
	if clamped != overallUsefulness {
		logging.Get(logging.CategoryContext).Warn(
			"StoreFeedback: overall usefulness %.4f outside [0,1]; clamping", overallUsefulness)
		overallUsefulness = clamped
	}
	// Empty predicate names carry no signal and would aggregate into a phantom
	// "" row in the helpful/noise tables.
	helpfulFacts = dropEmptyPredicates(helpfulFacts)
	noiseFacts = dropEmptyPredicates(noiseFacts)

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert main feedback record
	taskSucceededInt := 0
	if taskSucceeded {
		taskSucceededInt = 1
	}

	result, err := tx.Exec(`
		INSERT INTO context_feedback (turn_id, timestamp, manifest_hash, overall_usefulness, intent_verb, task_succeeded)
		VALUES (?, ?, ?, ?, ?, ?)
	`, turnID, time.Now().Format(time.RFC3339), manifestHash, overallUsefulness, intentVerb, taskSucceededInt)
	if err != nil {
		return fmt.Errorf("failed to insert feedback: %w", err)
	}

	feedbackID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get feedback ID: %w", err)
	}

	// Insert helpful predicate ratings
	for _, predicate := range helpfulFacts {
		_, err := tx.Exec(`
			INSERT INTO predicate_feedback (feedback_id, predicate, rating)
			VALUES (?, ?, 'helpful')
		`, feedbackID, predicate)
		if err != nil {
			return fmt.Errorf("failed to insert helpful predicate: %w", err)
		}
	}

	// Insert noise predicate ratings
	for _, predicate := range noiseFacts {
		_, err := tx.Exec(`
			INSERT INTO predicate_feedback (feedback_id, predicate, rating)
			VALUES (?, ?, 'noise')
		`, feedbackID, predicate)
		if err != nil {
			return fmt.Errorf("failed to insert noise predicate: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Invalidate cache
	s.cacheMu.Lock()
	s.cache = make(map[string]float64)
	s.cacheMu.Unlock()

	logging.ContextDebug("Stored context feedback: turn=%d, usefulness=%.2f, helpful=%d, noise=%d",
		turnID, overallUsefulness, len(helpfulFacts), len(noiseFacts))

	return nil
}

// GetPredicateUsefulness returns a usefulness score for a predicate.
// Returns 0.0 if not enough samples, otherwise -1.0 (noise) to +1.0 (helpful).
func (s *ContextFeedbackStore) GetPredicateUsefulness(predicate string) float64 {
	// Check cache first
	s.cacheMu.RLock()
	if score, ok := s.cache[predicate]; ok {
		s.cacheMu.RUnlock()
		return score
	}
	s.cacheMu.RUnlock()

	score := s.computePredicateScore(predicate, "")

	// Update cache
	s.cacheMu.Lock()
	s.cache[predicate] = score
	s.cacheMu.Unlock()

	return score
}

// GetPredicateUsefulnessForIntent returns usefulness score for a predicate
// in the context of a specific intent verb (e.g., "/fix", "/test").
func (s *ContextFeedbackStore) GetPredicateUsefulnessForIntent(predicate, intentVerb string) float64 {
	cacheKey := predicate + ":" + intentVerb

	s.cacheMu.RLock()
	if score, ok := s.cache[cacheKey]; ok {
		s.cacheMu.RUnlock()
		return score
	}
	s.cacheMu.RUnlock()

	score := s.computePredicateScore(predicate, intentVerb)

	s.cacheMu.Lock()
	s.cache[cacheKey] = score
	s.cacheMu.Unlock()

	return score
}

// computePredicateScore calculates the weighted usefulness score.
// Applies time-based decay and requires minimum samples.
func (s *ContextFeedbackStore) computePredicateScore(predicate, intentVerb string) float64 {
	// Build query based on whether intent filter is specified
	var query string
	var args []any

	if intentVerb != "" {
		query = `
			SELECT pf.rating, cf.timestamp
			FROM predicate_feedback pf
			JOIN context_feedback cf ON pf.feedback_id = cf.id
			WHERE pf.predicate = ? AND cf.intent_verb = ?
			ORDER BY cf.timestamp DESC
			LIMIT 100
		`
		args = []any{predicate, intentVerb}
	} else {
		query = `
			SELECT pf.rating, cf.timestamp
			FROM predicate_feedback pf
			JOIN context_feedback cf ON pf.feedback_id = cf.id
			WHERE pf.predicate = ?
			ORDER BY cf.timestamp DESC
			LIMIT 100
		`
		args = []any{predicate}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logging.ContextDebug("Failed to query predicate feedback: %v", err)
		return 0.0
	}
	defer rows.Close()

	now := time.Now()
	var weightedSum float64
	var totalWeight float64
	var count int

	for rows.Next() {
		var rating string
		var timestamp string
		if err := rows.Scan(&rating, &timestamp); err != nil {
			logging.ContextDebug("Skipping malformed predicate feedback row: %v", err)
			continue
		}

		ts, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			logging.ContextDebug("Skipping predicate feedback with bad timestamp %q: %v", timestamp, err)
			continue
		}

		// Calculate decay weight based on age
		age := now.Sub(ts)
		weight := math.Pow(0.5, float64(age)/float64(s.decayHalfLife))

		// Convert rating to score
		var score float64
		if rating == "helpful" {
			score = 1.0
		} else { // noise
			score = -1.0
		}

		weightedSum += score * weight
		totalWeight += weight
		count++
	}
	// A mid-iteration database failure must not present a partial sample as a
	// learned score: without enough trustworthy evidence the answer is neutral.
	if err := rows.Err(); err != nil {
		logging.ContextDebug("Predicate feedback query failed mid-iteration: %v", err)
		return 0.0
	}

	// Require minimum samples before affecting scoring
	if count < s.minSamples {
		return 0.0
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedSum / totalWeight
}

// GetPredicateFeedback returns detailed feedback statistics for a predicate.
func (s *ContextFeedbackStore) GetPredicateFeedback(predicate string) (*PredicateFeedback, error) {
	row := s.db.QueryRow(`
		SELECT
			pf.predicate,
			SUM(CASE WHEN pf.rating = 'helpful' THEN 1 ELSE 0 END) as helpful_count,
			SUM(CASE WHEN pf.rating = 'noise' THEN 1 ELSE 0 END) as noise_count,
			COUNT(*) as total_mentions,
			MAX(cf.timestamp) as last_updated
		FROM predicate_feedback pf
		JOIN context_feedback cf ON pf.feedback_id = cf.id
		WHERE pf.predicate = ?
		GROUP BY pf.predicate
	`, predicate)

	var pf PredicateFeedback
	var lastUpdated string
	err := row.Scan(&pf.Predicate, &pf.HelpfulCount, &pf.NoiseCount, &pf.TotalMentions, &lastUpdated)
	if err == sql.ErrNoRows {
		return nil, nil // No feedback for this predicate yet
	}
	if err != nil {
		return nil, err
	}

	pf.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdated)
	pf.WeightedScore = s.computePredicateScore(predicate, "")

	return &pf, nil
}

// GetTopHelpfulPredicates returns predicates with highest usefulness scores.
func (s *ContextFeedbackStore) GetTopHelpfulPredicates(limit int) ([]PredicateFeedback, error) {
	rows, err := s.db.Query(`
		SELECT
			pf.predicate,
			SUM(CASE WHEN pf.rating = 'helpful' THEN 1 ELSE 0 END) as helpful_count,
			SUM(CASE WHEN pf.rating = 'noise' THEN 1 ELSE 0 END) as noise_count,
			COUNT(*) as total_mentions
		FROM predicate_feedback pf
		GROUP BY pf.predicate
		HAVING COUNT(*) >= ?
		ORDER BY (helpful_count - noise_count) * 1.0 / COUNT(*) DESC
		LIMIT ?
	`, s.minSamples, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PredicateFeedback
	for rows.Next() {
		var pf PredicateFeedback
		if err := rows.Scan(&pf.Predicate, &pf.HelpfulCount, &pf.NoiseCount, &pf.TotalMentions); err != nil {
			logging.ContextDebug("Skipping malformed top-predicate row: %v", err)
			continue
		}
		pf.WeightedScore = s.computePredicateScore(pf.Predicate, "")
		results = append(results, pf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// GetTopNoisePredicates returns predicates with lowest usefulness scores.
func (s *ContextFeedbackStore) GetTopNoisePredicates(limit int) ([]PredicateFeedback, error) {
	rows, err := s.db.Query(`
		SELECT
			pf.predicate,
			SUM(CASE WHEN pf.rating = 'helpful' THEN 1 ELSE 0 END) as helpful_count,
			SUM(CASE WHEN pf.rating = 'noise' THEN 1 ELSE 0 END) as noise_count,
			COUNT(*) as total_mentions
		FROM predicate_feedback pf
		GROUP BY pf.predicate
		HAVING COUNT(*) >= ?
		ORDER BY (helpful_count - noise_count) * 1.0 / COUNT(*) ASC
		LIMIT ?
	`, s.minSamples, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PredicateFeedback
	for rows.Next() {
		var pf PredicateFeedback
		if err := rows.Scan(&pf.Predicate, &pf.HelpfulCount, &pf.NoiseCount, &pf.TotalMentions); err != nil {
			logging.ContextDebug("Skipping malformed top-predicate row: %v", err)
			continue
		}
		pf.WeightedScore = s.computePredicateScore(pf.Predicate, "")
		results = append(results, pf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// GetOverallStats returns aggregate statistics about context feedback.
func (s *ContextFeedbackStore) GetOverallStats() (totalFeedback int, avgUsefulness float64, err error) {
	// COALESCE because AVG over an empty table is NULL, which fails to scan
	// into float64 — a fresh workspace reported an error instead of "no data
	// yet", so any caller that surfaced stats gave up on the first run.
	row := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(AVG(overall_usefulness), 0)
		FROM context_feedback
	`)
	err = row.Scan(&totalFeedback, &avgUsefulness)
	return
}

// dropEmptyPredicates removes blank predicate names from a rating list.
func dropEmptyPredicates(preds []string) []string {
	out := preds[:0]
	for _, p := range preds {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Close closes the database connection.
func (s *ContextFeedbackStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// MinSamples reports how many observations a predicate needs before its
// learned usefulness is allowed to move activation scores. Operators need this
// to read the helpful/noise tables correctly: a predicate below the floor is
// not "neutral", it is "not yet trusted".
func (s *ContextFeedbackStore) MinSamples() int {
	return s.minSamples
}

// FeedbackStats is a snapshot of the context-learning loop, shaped for
// glass-box display and for the `nerd context-stats` operator command.
type FeedbackStats struct {
	// Available is false when no feedback store is wired; every other field is
	// then zero. Callers must not read "0 feedback entries" as "learning ran
	// and found nothing".
	Available bool
	// Err carries the first query failure, if any. Partial stats are still returned.
	Err error

	TotalFeedback int
	AvgUsefulness float64
	MinSamples    int

	Helpful []PredicateFeedback
	Noise   []PredicateFeedback
}

// CollectFeedbackStats gathers a FeedbackStats snapshot from a store, tolerating
// a nil store so callers do not need to branch.
func CollectFeedbackStats(s *ContextFeedbackStore, topN int) FeedbackStats {
	if s == nil {
		return FeedbackStats{}
	}
	if topN <= 0 {
		topN = 10
	}

	stats := FeedbackStats{Available: true, MinSamples: s.minSamples}

	total, avg, err := s.GetOverallStats()
	if err != nil {
		stats.Err = err
	} else {
		stats.TotalFeedback = total
		stats.AvgUsefulness = avg
	}

	if helpful, err := s.GetTopHelpfulPredicates(topN); err != nil {
		if stats.Err == nil {
			stats.Err = err
		}
	} else {
		stats.Helpful = helpful
	}

	if noise, err := s.GetTopNoisePredicates(topN); err != nil {
		if stats.Err == nil {
			stats.Err = err
		}
	} else {
		stats.Noise = noise
	}

	return stats
}
