package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"codenerd/internal/prompt"
)

// Rows and vectors a reconcile removes leave free pages; the seed is embedded
// in every binary, so reconcile compacts them away instead of shipping them.
func TestReconcileSeed_CompactsFreedPages(test *testing.T) {
	databasePath, database := newSeedFixture(test)
	kept := prompt.NewPromptAtom("fixture/kept", prompt.CategoryMethodology, "kept body")
	insertSeedFixtureAtom(test, database, kept, "embedded", nil, nil)
	vector := make([]byte, 32<<10)
	for i := 0; i < 40; i++ {
		dropped := prompt.NewPromptAtom(fmt.Sprintf("fixture/dropped-%02d", i), prompt.CategoryMethodology, "dropped body")
		insertSeedFixtureAtom(test, database, dropped, "embedded", vector, "RETRIEVAL_DOCUMENT")
	}
	if err := database.Close(); err != nil {
		test.Fatal(err)
	}
	before, err := os.Stat(databasePath)
	if err != nil {
		test.Fatal(err)
	}
	counts, err := reconcilePromptSeed(context.Background(), databasePath, []*prompt.PromptAtom{kept})
	if err != nil || counts.Deleted != 40 {
		test.Fatalf("reconcile = %+v (%v), want 40 deleted", counts, err)
	}
	reopened := openSeedFixture(test, databasePath)
	var free int
	if err := reopened.QueryRow("PRAGMA freelist_count").Scan(&free); err != nil {
		test.Fatal(err)
	}
	if free != 0 {
		test.Fatalf("reconciled seed keeps %d free pages", free)
	}
	if err := reopened.Close(); err != nil {
		test.Fatal(err)
	}
	after, err := os.Stat(databasePath)
	if err != nil {
		test.Fatal(err)
	}
	if after.Size()*4 > before.Size() {
		test.Fatalf("seed %d -> %d bytes: freed vectors were not compacted", before.Size(), after.Size())
	}
}
