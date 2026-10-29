package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// countingEmbedder counts the texts it embeds.
type countingEmbedder struct {
	name  string
	calls int
}

func (c *countingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	c.calls++
	return []float32{float32(len(text)), 1}, nil
}
func (c *countingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i], _ = c.Embed(ctx, text)
	}
	return out, nil
}
func (c *countingEmbedder) Dimensions() int { return 2 }
func (c *countingEmbedder) Name() string    { return c.name }

func storedEmbedding(t *testing.T, db *sql.DB, id string) []byte {
	t.Helper()
	var blob []byte
	if err := db.QueryRow(`SELECT embedding FROM prompt_atoms WHERE atom_id = ?`, id).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	return blob
}

// Re-storing an atom whose embedded text, model and task are unchanged keeps
// its stored vector and embeds nothing; a changed description, or a different
// model, embeds again. Every boot re-syncs every prompts.yaml through here.
func TestStoreAtom_ReusesTheEmbeddingOfAnUnchangedAtom(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "atoms.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	eng := &countingEmbedder{name: "model-a"}
	loader := NewAtomLoader(eng)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	atom := &PromptAtom{ID: "context/reuse", Category: CategoryContext, Content: "body", Description: "what it is", TokenCount: 1}
	for i := 0; i < 3; i++ {
		if err := loader.StoreAtom(ctx, db, atom); err != nil {
			t.Fatal(err)
		}
	}
	if eng.calls != 1 {
		t.Fatalf("three stores of an unchanged atom embedded %d times, want 1", eng.calls)
	}
	if len(storedEmbedding(t, db, atom.ID)) == 0 {
		t.Fatal("the reused embedding was not stored")
	}

	atom.Description = "what it is now"
	if err := loader.StoreAtom(ctx, db, atom); err != nil {
		t.Fatal(err)
	}
	if eng.calls != 2 {
		t.Fatalf("a changed description embedded %d times in all, want 2", eng.calls)
	}

	other := &countingEmbedder{name: "model-b"}
	if err := NewAtomLoader(other).StoreAtom(ctx, db, atom); err != nil {
		t.Fatal(err)
	}
	if other.calls != 1 {
		t.Fatalf("a different model embedded %d times, want 1", other.calls)
	}
}
