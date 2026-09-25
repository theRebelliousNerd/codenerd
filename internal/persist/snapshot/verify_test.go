package snapshot

import (
	"errors"
	"os"
	"testing"

	"codenerd/internal/persist/factsnap"
	"codenerd/internal/types"
)

// An operator can check a copied snapshot against its sidecar without
// importing it; a flipped byte is an integrity error, and a snapshot with no
// sidecar is reported as unverifiable rather than as good.
func TestVerify_ShouldCheckTheSidecarAndRefuseToVouchWithoutOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	facts := []types.Fact{{Predicate: "p", Args: []any{"a", int64(1)}}}

	path, err := Export(root, "good", facts, factsnap.CodecGzip)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if got, err := Verify(root, "good"); err != nil || got != path {
		t.Fatalf("Verify(good) = %q, %v; want %q, nil", got, err, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(root, "good"); !errors.Is(err, factsnap.ErrIntegrity) {
		t.Fatalf("Verify after a flipped byte = %v, want ErrIntegrity", err)
	}

	if _, err := factsnap.WritePath(Dir(root)+"/bare", facts, factsnap.Options{NoSidecar: true}); err != nil {
		t.Fatalf("WritePath: %v", err)
	}
	if _, err := Verify(root, "bare"); !errors.Is(err, ErrNoSidecar) {
		t.Fatalf("Verify(no sidecar) = %v, want ErrNoSidecar", err)
	}
}
