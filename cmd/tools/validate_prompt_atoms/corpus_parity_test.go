package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"reflect"
	"testing"

	"codenerd/internal/prompt"
)

func TestCheckedInCorpusOrderedParity(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "internal", "prompt", "atoms"))
	if err != nil {
		t.Fatalf("resolve atom root: %v", err)
	}

	issues, stats, err := validateAtomTree(root, validationOptions{CheckRecommendedSelectors: true})
	if err != nil {
		t.Fatalf("validator route: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("validator route reported issues: %+v", issues)
	}

	filesystemRecords, migrations, err := prompt.ParsePromptAtomDirectory(root)
	if err != nil {
		t.Fatalf("filesystem runtime route: %v", err)
	}
	if len(migrations) != 0 {
		t.Fatalf("checked-in corpus requires compatibility migrations: %+v", migrations)
	}
	filesystemIDs := make([]string, 0, len(filesystemRecords))
	for _, record := range filesystemRecords {
		filesystemIDs = append(filesystemIDs, record.Atom.ID)
	}

	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("embedded runtime route: %v", err)
	}
	embeddedAtoms := embedded.All()
	embeddedIDs := make([]string, 0, len(embeddedAtoms))
	for _, atom := range embeddedAtoms {
		embeddedIDs = append(embeddedIDs, atom.ID)
	}

	if !reflect.DeepEqual(stats.AtomIDs, filesystemIDs) {
		t.Fatal("validator and filesystem runtime atom order differ")
	}
	if !reflect.DeepEqual(stats.AtomIDs, embeddedIDs) {
		t.Fatal("validator and embedded runtime atom order differ")
	}

	// The golden follows the atom IDs, not the YAML files: 923 -> 2445 when
	// e5489d53 split every atom over 1500 characters into a head and its
	// sections (section_of), each section being an atom of its own; 2445 ->
	// 2156 when the atomicity pass (2026-10-28) deleted known-by-default
	// atoms and merged duplicates, other lanes adding theirs; 2157 with
	// capability/run_lint (2026-10-29). To
	// refresh it after an intended corpus change, take the count and digest
	// this test prints.
	const wantCount = 2157
	const wantDigest = "07c7b5ffac5d77c8ea89a2db0982e7dcce96ef47056c658d6d9e5733ac2ac70a"
	if len(stats.AtomIDs) != wantCount {
		t.Fatalf("atom count = %d, want golden %d", len(stats.AtomIDs), wantCount)
	}
	if got := orderedIDDigest(stats.AtomIDs); got != wantDigest {
		t.Fatalf("ordered atom ID digest = %s, want golden %s", got, wantDigest)
	}
}

func orderedIDDigest(ids []string) string {
	hash := sha256.New()
	for _, id := range ids {
		hash.Write([]byte(id))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
