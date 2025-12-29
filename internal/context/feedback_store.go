package context

import (
	"database/sql"
	"fmt"
	"math"
	"sync"
	"time"

	"codenerd/internal/logging"
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

	// Cache for frequently queried predicates (predicate -> usefulness score)
	cache     map[string]float64
	cacheMu   sync.RWMutex
	cacheTime time.Time

	// Configuration
	minSamples   int           // Minimum samples before score affects activation
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
	Predicate      string
	HelpfulCount   int
	NoiseCount     int
	TotalMentions  int
	WeightedScore  float64 // -1.0 (always noise) to +1.0 (always helpful)
	LastUpdated    time.Time
}

// NewContextFeedbackStore creates a new feedback store.
// dbPath should be ".nerd/context_feedback.db"
func NewContextFeedbackStore(dbPath string) (*ContextFeedbackStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open feedback database: %w", err)
	}

	store := &ContextFeedbackStore{
		db:            db,
		cache:         make(map[string]float64),
		minSamples:    10,              // Conservative: 10 samples before affecting scoring
		decayHalfLife: 7 * 24 * time.Hour, // 7-day half-life for decay
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
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
	helpfulFacts []string,
	noiseFacts []string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert main feedback record
	result, err := tx.Exec(`
		INSERT INTO context_feedback (turn_id, timestamp, manifest_hash, overall_usefulness, intent_verb)
		VALUES (?, ?, ?, ?, ?)
	`, turnID, time.Now().Format(time.RFC3339), manifestHash, overallUsefulness, intentVerb)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build query based on whether intent filter is specified
	var query string
	var args []interface{}

	if intentVerb != "" {
		query = `
			SELECT pf.rating, cf.timestamp
			FROM predicate_feedback pf
			JOIN context_feedback cf ON pf.feedback_id = cf.id
			WHERE pf.predicate = ? AND cf.intent_verb = ?
			ORDER BY cf.timestamp DESC
			LIMIT 100
		`
		args = []interface{}{predicate, intentVerb}
	} else {
		query = `
			SELECT pf.rating, cf.timestamp
			FROM predicate_feedback pf
			JOIN context_feedback cf ON pf.feedback_id = cf.id
			WHERE pf.predicate = ?
			ORDER BY cf.timestamp DESC
			LIMIT 100
		`
		args = []interface{}{predicate}
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
			continue
		}

		ts, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
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
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

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
			continue
		}
		pf.WeightedScore = s.computePredicateScore(pf.Predicate, "")
		results = append(results, pf)
	}

	return results, nil
}

// GetTopNoisePredicates returns predicates with lowest usefulness scores.
func (s *ContextFeedbackStore) GetTopNoisePredicates(limit int) ([]PredicateFeedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
			continue
		}
		pf.WeightedScore = s.computePredicateScore(pf.Predicate, "")
		results = append(results, pf)
	}

	return results, nil
}

// GetOverallStats returns aggregate statistics about context feedback.
func (s *ContextFeedbackStore) GetOverallStats() (totalFeedback int, avgUsefulness float64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`
		SELECT COUNT(*), AVG(overall_usefulness)
		FROM context_feedback
	`)
	err = row.Scan(&totalFeedback, &avgUsefulness)
	return
}

// Close closes the database connection.
func (s *ContextFeedbackStore) Close() error {
	return s.db.Close()
}
