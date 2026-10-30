package store

import (
	"path/filepath"
	"testing"
)

// Saying "remember X" again for a preference already saved renewed nothing
// archival reads: the upsert touched updated_at, archival selects on
// last_accessed and access_count, and hydration deliberately counts no
// access. An old, rarely read preference was archived the next maintenance
// tick after the person reaffirmed it, and fresh hydration lost it (audit
// user-memory-archival-lifecycle).
func TestRememberUserPreference_ReaffirmationRenewsAgainstArchival(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RememberUserPreference(UserPreferencePredicate, "tabs for indentation"); err != nil {
		t.Fatal(err)
	}
	// 120 days without a counted read.
	if _, err := s.db.Exec(`UPDATE cold_storage SET last_accessed = datetime('now', '-120 days'), access_count = 0`); err != nil {
		t.Fatal(err)
	}
	if err := s.RememberUserPreference(UserPreferencePredicate, "tabs for indentation"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ArchiveOldFacts(90, 5); err != nil || n != 0 {
		t.Fatalf("ArchiveOldFacts = %d, %v: a just-reaffirmed preference was archived", n, err)
	}
	prefs, err := s.UserPreferences()
	if err != nil || len(prefs) != 1 {
		t.Fatalf("preferences = %v, %v; want the reaffirmed one", prefs, err)
	}
}

// Reaffirming a preference that maintenance already archived makes the
// active row the one copy: the archived duplicate goes in the same
// transaction, so read, list and forget see one record.
func TestRememberUserPreference_ReaffirmingAnArchivedPreferenceLeavesOneCopy(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RememberUserPreference(UserPreferencePredicate, "tabs for indentation"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE cold_storage SET last_accessed = datetime('now', '-120 days'), access_count = 0`); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ArchiveOldFacts(90, 5); err != nil || n != 1 {
		t.Fatalf("precondition: ArchiveOldFacts = %d, %v; want the stale preference archived", n, err)
	}
	if err := s.RememberUserPreference(UserPreferencePredicate, "tabs for indentation"); err != nil {
		t.Fatal(err)
	}
	prefs, err := s.UserPreferences()
	if err != nil || len(prefs) != 1 {
		t.Fatalf("preferences = %v, %v; want the reaffirmed one active", prefs, err)
	}
	archived, err := s.GetArchivedFacts(UserPreferencePredicate)
	if err != nil || len(archived) != 0 {
		t.Fatalf("archived copies = %v, %v; want none beside the active row", archived, err)
	}
}
