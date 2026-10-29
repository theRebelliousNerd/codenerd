package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDeleteKnowledgeAtoms(t *testing.T) {
	tests := []struct {
		name        string
		del         func(s *LocalStore) (int64, error)
		wantDeleted int64
		wantLeft    []string
	}{
		{
			name: "exact concepts",
			del: func(s *LocalStore) (int64, error) {
				return s.DeleteKnowledgeAtomsByConcept("strategic/vision", "strategic/pattern")
			},
			wantDeleted: 3,
			wantLeft:    []string{"strategic/source/README.md", "strategic/visionary", "doc/a.md/x"},
		},
		{
			name:        "literal prefix",
			del:         func(s *LocalStore) (int64, error) { return s.DeleteKnowledgeAtomsByPrefix("strategic/source/") },
			wantDeleted: 1,
			wantLeft:    []string{"strategic/vision", "strategic/vision", "strategic/pattern", "strategic/visionary", "doc/a.md/x"},
		},
		{
			name:        "underscore in a prefix is not a wildcard",
			del:         func(s *LocalStore) (int64, error) { return s.DeleteKnowledgeAtomsByPrefix("doc_") },
			wantDeleted: 0,
			wantLeft:    []string{"strategic/vision", "strategic/vision", "strategic/pattern", "strategic/source/README.md", "strategic/visionary", "doc/a.md/x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
			if err != nil {
				t.Fatalf("NewLocalStore: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			s.SetEmbeddingEngine(&MockEmbeddingEngine{})
			ctx := context.Background()
			for _, atom := range []struct{ concept, content string }{
				{"strategic/vision", "old vision"},
				{"strategic/vision", "old vision"},
				{"strategic/pattern", "a pattern"},
				{"strategic/source/README.md", "readme body"},
				{"strategic/visionary", "unrelated concept sharing a prefix"},
				{"doc/a.md/x", "doc atom"},
			} {
				if err := s.StoreKnowledgeAtomWithEmbedding(ctx, atom.concept, atom.content, 1, "test"); err != nil {
					t.Fatalf("store %s: %v", atom.concept, err)
				}
			}

			deleted, err := tt.del(s)
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if deleted != tt.wantDeleted {
				t.Fatalf("deleted = %d, want %d", deleted, tt.wantDeleted)
			}
			atoms, err := s.GetAllKnowledgeAtoms()
			if err != nil {
				t.Fatalf("GetAllKnowledgeAtoms: %v", err)
			}
			left := map[string]int{}
			for _, atom := range atoms {
				left[atom.Concept]++
			}
			want := map[string]int{}
			for _, concept := range tt.wantLeft {
				want[concept]++
			}
			if len(left) != len(want) {
				t.Fatalf("left = %v, want %v", left, want)
			}
			for concept, n := range want {
				if left[concept] != n {
					t.Fatalf("left = %v, want %v", left, want)
				}
			}

			var vectors int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM vectors WHERE json_extract(metadata, '$.content_type') = 'knowledge_atom'`).Scan(&vectors); err != nil {
				t.Fatalf("count vectors: %v", err)
			}
			if vectors != len(tt.wantLeft) {
				t.Fatalf("knowledge-atom vectors = %d, want %d (one per remaining atom)", vectors, len(tt.wantLeft))
			}
		})
	}
}

func TestDeleteKnowledgeAtomsByPrefix_RefusesEmptyPrefix(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.DeleteKnowledgeAtomsByPrefix(""); err == nil {
		t.Fatal("an empty prefix would empty the table; want an error")
	}
}
