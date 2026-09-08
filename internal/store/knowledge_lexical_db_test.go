package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestKnowledgeLexicalDB_RankingBoundsAndProvenance(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for i := 0; i < 40; i++ {
		if err := s.StoreKnowledgeAtom(fmt.Sprintf("noise/%d", i), "quartz partial match", 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.StoreKnowledgeAtom("guide/Z01", "quartz zephyr bounded indexing hypothesis", 0.4); err != nil {
		t.Fatal(err)
	}
	hits, err := SearchKnowledgeAtomsLexicalDB(t.Context(), s.db, "quartz zephyr", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Concept != "guide/Z01" || hits[0].Confidence != 0.4 {
		t.Fatalf("relevant low-confidence source lost before ranking: %+v", hits)
	}
	hits, err = s.SearchKnowledgeAtomsLexical(t.Context(), "quartz", 1000)
	if err != nil || len(hits) != 10 {
		t.Fatalf("bounded query: %d hits, %v", len(hits), err)
	}
	if err := s.db.PingContext(t.Context()); err != nil {
		t.Fatalf("borrowed handle closed: %v", err)
	}
}

func TestKnowledgeLexicalDB_LiteralWildcardsAndMissingKnowledge(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.StoreKnowledgeAtom("ordinary", "unrelated data", 1); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"unknownword", "xx%", "xx_", "x' UNION SELECT 'zz"} {
		hits, err := SearchKnowledgeAtomsLexicalDB(t.Context(), s.db, query, 5)
		if err != nil || len(hits) != 0 {
			t.Fatalf("%q leaked unrelated knowledge: %+v, %v", query, hits, err)
		}
	}
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if hits, err := SearchKnowledgeAtomsLexicalDB(t.Context(), db, "missing", 5); err != nil || len(hits) != 0 {
		t.Fatalf("missing table fabricated knowledge: %+v, %v", hits, err)
	}
}

func TestKnowledgeLexicalDB_CancelsWhileWaitingForBorrowedConnection(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "blocked.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	hits, err := SearchKnowledgeAtomsLexicalDB(ctx, db, "quartz", 5)
	if !errors.Is(err, context.DeadlineExceeded) || len(hits) != 0 {
		t.Fatalf("blocked query did not honor deadline: %+v, %v", hits, err)
	}
}
