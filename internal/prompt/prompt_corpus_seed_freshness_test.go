package prompt

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codenerd/internal/core/defaults"

	_ "github.com/mattn/go-sqlite3"
)

// TestPromptCorpusSeedMatchesEmbeddedYAML pins the shipped first-boot seed
// internal/core/defaults/prompt_corpus.db to the embedded YAML corpus: every
// YAML atom is in the seed with identical content, and the seed holds no
// removed atom. It had drifted to 878 atoms against 917 (2026-09-28), nine of
// them deleted atoms, and nothing noticed. Reconcile with
// `go run ./cmd/tools/prompt_builder -reconcile-embedded` to retain embeddings
// whose input is unchanged and invalidate changed ones. A vectorless rebuild
// degrades first-boot vector search to lexical. The predicate corpus
// equivalent is `go run ./cmd/tools/predicate_corpus_builder -check`.
func TestPromptCorpusSeedMatchesEmbeddedYAML(t *testing.T) {
	if !defaults.PromptCorpusAvailable() {
		t.Skip("no embedded prompt corpus seed")
	}
	data, err := defaults.PromptCorpusDB.ReadFile("prompt_corpus.db")
	if err != nil {
		t.Fatalf("read embedded seed: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "prompt_corpus.db")
	if err := os.WriteFile(dbPath, data, 0o644); err != nil {
		t.Fatalf("materialize seed: %v", err)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open seed: %v", err)
	}
	defer db.Close()

	seed := make(map[string]string)
	rows, err := db.Query(`SELECT atom_id, content FROM prompt_atoms`)
	if err != nil {
		t.Fatalf("read seed atoms: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			t.Fatalf("scan seed atom: %v", err)
		}
		seed[id] = content
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate seed atoms: %v", err)
	}

	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	for _, mismatch := range promptSeedMismatches(seed, corpus.All()) {
		t.Error(mismatch)
	}
}

// promptSeedMismatches reports every parity failure with the credential-free,
// embedding-preserving command that repairs the shipped seed.
func promptSeedMismatches(seed map[string]string, atoms []*PromptAtom) []string {
	const regenerate = "go run ./cmd/tools/prompt_builder -reconcile-embedded"
	seen := make(map[string]bool, len(atoms))
	var mismatches []string
	for _, a := range atoms {
		seen[a.ID] = true
		got, ok := seed[a.ID]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("seed lacks YAML atom %s; run `%s`", a.ID, regenerate))
		} else if got != a.Content {
			mismatches = append(mismatches, fmt.Sprintf("seed content for %s differs from YAML; run `%s`", a.ID, regenerate))
		}
	}
	for id := range seed {
		if !seen[id] {
			mismatches = append(mismatches, fmt.Sprintf("seed holds removed atom %s; run `%s`", id, regenerate))
		}
	}
	sort.Strings(mismatches)
	return mismatches
}

func TestPromptSeedMismatches_NamesRegeneratingCommand(t *testing.T) {
	atom := NewPromptAtom("fixture/current", CategoryMethodology, "canonical content")
	for _, tc := range []struct {
		name string
		seed map[string]string
		want string
	}{
		{name: "current", seed: map[string]string{atom.ID: atom.Content}},
		{name: "missing", seed: map[string]string{}, want: "seed lacks YAML atom fixture/current"},
		{name: "changed", seed: map[string]string{atom.ID: "old content"}, want: "seed content for fixture/current differs from YAML"},
		{name: "removed", seed: map[string]string{atom.ID: atom.Content, "fixture/removed": "old content"}, want: "seed holds removed atom fixture/removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := promptSeedMismatches(tc.seed, []*PromptAtom{atom})
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("current seed reported stale: %v", got)
				}
				return
			}
			want := tc.want + "; run `go run ./cmd/tools/prompt_builder -reconcile-embedded`"
			if len(got) != 1 || got[0] != want {
				t.Fatalf("stale seed diagnostics = %v, want [%s]", got, want)
			}
		})
	}
}
