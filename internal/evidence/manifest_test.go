package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The manifest digest is byte-for-byte the digest Snapshot has always
// produced, so witnesses recorded before the manifest still compare equal.
func TestManifestDigestIsTheSnapshotLineDigest(t *testing.T) {
	root, _ := fixture(t)
	m, err := SnapshotManifest(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, name := range []string{"add.go", "add_test.go", "go.mod"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(h, "%q:%o:%x\n", name, info.Mode(), sum[:])
	}
	if got, want := m.Digest(), hex.EncodeToString(h.Sum(nil)); got != want {
		t.Fatalf("digest %s, want the line digest %s", got, want)
	}
	digest, err := Snapshot(t.Context(), root)
	if err != nil || digest != m.Digest() {
		t.Fatalf("Snapshot %s (%v) != manifest digest %s", digest, err, m.Digest())
	}
	if m.Len() != 3 {
		t.Fatalf("manifest certifies %d files, want 3", m.Len())
	}
}

func TestChangedPathsNamesAddedRemovedEditedAndModeChanges(t *testing.T) {
	root, _ := fixture(t)
	before, err := SnapshotManifest(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := ChangedPaths(before, before); len(got) != 0 {
		t.Fatalf("unchanged manifest reports %v", got)
	}
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "add_test.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "go.mod"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".nerd", "agents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "agents", "coder.md"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := SnapshotManifest(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".nerd/agents/coder.md", "add.go", "add_test.go", "go.mod"}
	if got := ChangedPaths(before, after); !reflect.DeepEqual(got, want) {
		t.Fatalf("changed %v, want %v", got, want)
	}
}
