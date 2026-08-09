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

	const wantCount = 902
	const wantDigest = "1a699dd491bb78a227c51a18ea18505db20c0af9db4b90759e600b3cdd64f155"
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
