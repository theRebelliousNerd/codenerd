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

	const wantCount = 915
	// Includes tool-agnostic editing discipline alongside change evidence, and
	// the working-context methodology atom (methodology/working_context).
	// 920 at 8ebd7616, minus the 6 envelope-restating atoms deleted by
	// f73362ff (piggyback, reasoning_trace, output_protocol,
	// self_correction, tool_steering); plus
	// language/mangle/engine_truths_pinned (2026-09-18), served when the kernel
	// derives /authoring_mangle (policy/jit_needs.mg); minus
	// language/mangle/docs/builtins_complete/aggregators (2026-09-18), a second
	// reducer reference teaching fn:CountDistinct and fn:CollectToMap, which the
	// pinned engine does not have; plus capability/structure_queries
	// (2026-09-21), which teaches the five structural query tools over the
	// world model's structure index and says raw search opens only after them;
	// minus campaign/taxonomist/{output_protocol,reasoning_trace} (ca21c7e5,
	// the planner is not told to answer in a Piggyback envelope); plus
	// eval/delegation_judge/{implementation,review} (6cf5b177, the delegation
	// judge's prompt compiled from atoms). The count held; the order did not.
	const wantDigest = "73445b874f3a90a3efadc6b21f52403a62fe0defd9b2193bcfceebd4f6c4bf1b"
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
