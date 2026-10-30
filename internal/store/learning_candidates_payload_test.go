package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// A database staged before proposals were stored whole is rebuilt on open:
// its rows keep their ids, counts and statuses, their fact is "" (unknown),
// and two proposals of one mapping that differ in qualifier become two
// candidates rather than one counted twice (audit
// manual-learning-evidence-conservation).
func TestLearningCandidates_LegacyTableMigratesAndProposalsStayDistinct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE learning_candidates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phrase TEXT NOT NULL, verb TEXT NOT NULL, target TEXT, reason TEXT,
		count INTEGER DEFAULT 1, status TEXT DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(phrase, verb, target, reason));
		INSERT INTO learning_candidates (id, phrase, verb, target, reason, count, status) VALUES (7, 'add a feature', '/create', 'feature', '/critic_manual', 3, 'rejected');`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	s, err := NewLocalStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cands, err := s.ListLearningCandidates("", 10)
	if err != nil || len(cands) != 1 {
		t.Fatalf("after migration: %+v, %v", cands, err)
	}
	if c := cands[0]; c.ID != 7 || c.Count != 3 || c.Status != "rejected" || c.Fact != "" {
		t.Fatalf("legacy row = %+v, want id 7, count 3, rejected, fact unknown", c)
	}

	a := `learned_exemplar("add a feature", /create, "feature", "include a test", 97).`
	b := `learned_exemplar("add a feature", /create, "feature", "", 90).`
	for _, f := range []string{a, b, a} {
		if _, err := s.RecordLearningCandidate("add a feature", "/create", "feature", "/critic_manual", f); err != nil {
			t.Fatal(err)
		}
	}
	cands, err = s.ListLearningCandidates("pending", 10)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range cands {
		counts[c.Fact] = c.Count
	}
	if len(cands) != 2 || counts[a] != 2 || counts[b] != 1 {
		t.Fatalf("pending candidates = %+v, want the two proposals apart, the repeated one counted twice", cands)
	}
	if _, err := s.RecordLearningCandidate("x", "/fix", "", "/r", ""); err == nil {
		t.Fatal("a candidate with no proposed fact was staged")
	}
}
