package store

import (
	"fmt"

	"codenerd/internal/logging"
)

// The user's saved long-term memory is the cold_storage rows of this type.
// It is the type HydrateLearnings loads into learned_preference, so a row of
// it is a preference the next turn actually sees; a row of any other type the
// remember path used to write ("learned") is stored and never hydrated.
const (
	UserPreferenceFactType = "preference"
	// UserPreferencePredicate is the key a natural-language "remember"
	// instruction stores under; the value is the thing to remember.
	UserPreferencePredicate = "preference"
	// legacyRememberedFactType is what the chat remember path wrote before
	// the two producers agreed on UserPreferenceFactType.
	legacyRememberedFactType = "learned"
)

// UserPreferences returns every saved long-term preference, in the store's
// own order. It reads exactly what hydration reads.
func (s *LocalStore) UserPreferences() ([]StoredFact, error) {
	return s.LoadAllFacts(UserPreferenceFactType)
}

// RememberUserPreference saves value under key as a long-term preference, or
// reaffirms it when it is already saved. Reaffirmation is a renewal: it
// resets the access clock archival reads (last_accessed), and when the same
// preference was already archived the archived copy goes in the same
// transaction, so the active row is its one record.
//
// The generic upsert (StoreFact) it replaces on this path touched only
// updated_at, and hydration deliberately counts no access, so an old, rarely
// read preference the person had just reaffirmed was archived at the next
// maintenance tick and fresh hydration lost it; an archived one reaffirmed
// came back as a second copy forget could not see (audit
// user-memory-archival-lifecycle). Other cold facts keep StoreFact's
// semantics.
func (s *LocalStore) RememberUserPreference(key, value string) error {
	argsJSON, err := encodeFactArgs([]any{value})
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO cold_storage (predicate, args, fact_type, priority, updated_at, last_accessed)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(predicate, args) DO UPDATE SET
		 fact_type = excluded.fact_type,
		 priority = excluded.priority,
		 updated_at = CURRENT_TIMESTAMP,
		 last_accessed = CURRENT_TIMESTAMP`,
		key, string(argsJSON), UserPreferenceFactType, userPreferencePriority,
	); err != nil {
		return fmt.Errorf("remember preference %s: %w", key, err)
	}
	if _, err := tx.Exec(`DELETE FROM archived_facts WHERE predicate = ? AND args = ?`, key, string(argsJSON)); err != nil {
		return fmt.Errorf("remember preference %s: retire the archived copy: %w", key, err)
	}
	return tx.Commit()
}

// userPreferencePriority is the cold-storage priority a remembered
// preference is saved at.
const userPreferencePriority = 10

// DeleteStoredFact removes one cold-storage row by its identity and reports
// how many rows went (0 when it was already absent). A row is addressed by
// id, never by predicate: the generic "preference" key holds many values.
func (s *LocalStore) DeleteStoredFact(id int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM cold_storage WHERE id = ?", id)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete cold storage row %d: %v", id, err)
		return 0, fmt.Errorf("delete cold storage row %d: %w", id, err)
	}
	return result.RowsAffected()
}

// adoptRememberedPreferences gives the rows the old chat remember path wrote
// the type hydration loads.
//
// That path stored (predicate "preference", type "learned"), the shared
// Piggyback promotion stored type "preference", and HydrateLearnings loads
// "preference", so a remembered preference sat in the store and never reached
// a turn. The migration is keyed by the predicate as well as the type, never
// by the type alone: "learned" is also the default type of every model-sent
// kernel update (persistence.go), and those are not user instructions. The
// one writer that ever stored the predicate "preference" under "learned" was
// the remember path, because a model update whose predicate names a
// preference is typed "preference" by that same code. (predicate, args) is
// unique, so the update cannot collide with an existing row.
func (s *LocalStore) adoptRememberedPreferences() error {
	result, err := s.db.Exec(
		`UPDATE cold_storage SET fact_type = ? WHERE predicate = ? AND fact_type = ?`,
		UserPreferenceFactType, UserPreferencePredicate, legacyRememberedFactType)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err == nil && n > 0 {
		logging.Store("Adopted %d remembered preferences stored as type %q so they hydrate as %q", n, legacyRememberedFactType, UserPreferenceFactType)
	}
	return nil
}
