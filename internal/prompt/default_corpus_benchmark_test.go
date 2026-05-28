package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func BenchmarkHydrateAtomContextTags(b *testing.B) {
	// Setup DB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE prompt_atoms (
			atom_id TEXT PRIMARY KEY
		);
		CREATE TABLE atom_context_tags (
			atom_id TEXT,
			dimension TEXT,
			tag TEXT,
			PRIMARY KEY (atom_id, dimension, tag)
		);
	`)
	if err != nil {
		b.Fatal(err)
	}

	// Insert test data
	numAtoms := 1000
	atoms := make([]*PromptAtom, numAtoms)
	for i := range numAtoms {
		id := fmt.Sprintf("atom_%d", i)
		_, err := db.Exec("INSERT INTO prompt_atoms (atom_id) VALUES (?)", id)
		if err != nil {
			b.Fatal(err)
		}

		atoms[i] = &PromptAtom{
			ID:               id,
			OperationalModes: []string{"mode1", "mode2"},
			CampaignPhases:   []string{"phase1", "phase2"},
			BuildLayers:      []string{"layer1", "layer2"},
			IntentVerbs:      []string{"intent1", "intent2"},
		}
	}

	// Seed tags data
	err = HydrateAtomContextTags(context.Background(), db, atoms)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// We need to truncate the tags table before each run
		b.StopTimer()
		_, err := db.Exec("DELETE FROM atom_context_tags")
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		err = HydrateAtomContextTags(context.Background(), db, atoms)
		if err != nil {
			b.Fatal(err)
		}
	}
}
