package store

import (
	"codenerd/internal/logging"
	"database/sql"
	"time"
)

// =============================================================================
// SESSION MANAGEMENT (History, Activation, Compressed State)
// =============================================================================

// LogActivation records activation scores for facts.
func (s *LocalStore) LogActivation(factID string, score float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Logging activation: fact_id=%s score=%.4f", factID, score)

	_, err := s.db.Exec(
		"INSERT INTO activation_log (fact_id, activation_score) VALUES (?, ?)",
		factID, score,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to log activation for %s: %v", factID, err)
		return err
	}

	logging.StoreDebug("Activation logged successfully: fact_id=%s", factID)
	return nil
}

// GetRecentActivations retrieves recent activation scores.
func (s *LocalStore) GetRecentActivations(limit int, minScore float64) (map[string]float64, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetRecentActivations")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	logging.StoreDebug("Retrieving recent activations: limit=%d minScore=%.4f", limit, minScore)

	rows, err := s.db.Query(
		`SELECT fact_id, MAX(activation_score) as max_score
		 FROM activation_log
		 WHERE timestamp > datetime('now', '-1 hour')
		 GROUP BY fact_id
		 HAVING max_score >= ?
		 ORDER BY max_score DESC
		 LIMIT ?`,
		minScore, limit,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query recent activations: %v", err)
		return nil, err
	}
	defer rows.Close()

	activations := make(map[string]float64)
	for rows.Next() {
		var factID string
		var score float64
		if err := rows.Scan(&factID, &score); err != nil {
			continue
		}
		activations[factID] = score
	}

	logging.StoreDebug("Retrieved %d recent activations (minScore=%.4f)", len(activations), minScore)
	return activations, nil
}

// StoreSessionTurn records a conversation turn.
// Uses INSERT OR IGNORE for idempotent syncing (duplicate turns are silently skipped).
func (s *LocalStore) StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing session turn: session=%s turn=%d input_len=%d response_len=%d",
		sessionID, turnNumber, len(userInput), len(response))

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO session_history (session_id, turn_number, user_input, intent_json, response, atoms_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, userInput, intentJSON, response, atomsJSON,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store session turn: session=%s turn=%d: %v", sessionID, turnNumber, err)
		return err
	}

	logging.StoreDebug("Session turn stored: session=%s turn=%d", sessionID, turnNumber)
	return nil
}

// GetSessionHistory retrieves session history.
func (s *LocalStore) GetSessionHistory(sessionID string, limit int) ([]map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetSessionHistory")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	logging.StoreDebug("Retrieving session history: session=%s limit=%d", sessionID, limit)

	rows, err := s.db.Query(
		`SELECT turn_number, user_input, intent_json, response, atoms_json, created_at
		 FROM session_history
		 WHERE session_id = ?
		 ORDER BY turn_number DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query session history for %s: %v", sessionID, err)
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var turnNumber int
		var userInput, intentJSON, response, atomsJSON string
		var createdAt time.Time
		if err := rows.Scan(&turnNumber, &userInput, &intentJSON, &response, &atomsJSON, &createdAt); err != nil {
			continue
		}
		history = append(history, map[string]interface{}{
			"turn_number": turnNumber,
			"user_input":  userInput,
			"intent":      intentJSON,
			"response":    response,
			"atoms":       atomsJSON,
			"timestamp":   createdAt,
		})
	}

	logging.StoreDebug("Retrieved %d session history turns for session=%s", len(history), sessionID)
	return history, nil
}

// StoreCompressedState persists the semantic compression state for a session turn.
// Uses INSERT OR REPLACE so the latest state for a turn is idempotent.
func (s *LocalStore) StoreCompressedState(sessionID string, turnNumber int, stateJSON string, ratio float64) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreCompressedState")
	defer timer.Stop()

	if sessionID == "" || stateJSON == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO compressed_states (session_id, turn_number, state_json, compression_ratio)
		 VALUES (?, ?, ?, ?)`,
		sessionID, turnNumber, stateJSON, ratio,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to store compressed state: session=%s turn=%d: %v", sessionID, turnNumber, err)
		return err
	}

	logging.StoreDebug("Stored compressed state: session=%s turn=%d ratio=%.1f", sessionID, turnNumber, ratio)
	return nil
}

// LoadLatestCompressedState loads the most recent compressed state for a session.
// Returns empty string if no state exists.
func (s *LocalStore) LoadLatestCompressedState(sessionID string) (string, int, float64, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadLatestCompressedState")
	defer timer.Stop()

	if sessionID == "" {
		return "", 0, 1.0, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var stateJSON string
	var turnNumber int
	var ratio float64
	err := s.db.QueryRow(
		`SELECT state_json, turn_number, compression_ratio
		 FROM compressed_states
		 WHERE session_id = ?
		 ORDER BY turn_number DESC
		 LIMIT 1`,
		sessionID,
	).Scan(&stateJSON, &turnNumber, &ratio)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, 1.0, nil
		}
		return "", 0, 1.0, err
	}

	return stateJSON, turnNumber, ratio, nil
}
