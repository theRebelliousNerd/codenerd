package snapshot_test

// The integration the persist corpus asked for: a domain object becomes facts,
// the facts become a snapshot on disk, the snapshot becomes facts again, and a
// second, independently booted kernel ends up holding the same EDB. Unit tests
// in factsnap prove the codec; this proves the whole export/import loop the
// `nerd snapshot` command drives.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/persist/factsnap"
	"codenerd/internal/persist/snapshot"
	"codenerd/internal/types"
)

// codeSymbol is a stand-in domain object with a ToFacts() projection, which is
// the shape world/campaign objects already have.
type codeSymbol struct {
	File  string
	Name  string
	Kind  string
	Start int64
	End   int64
}

func (c codeSymbol) ToFacts() []types.Fact {
	return []types.Fact{{
		Predicate: "code_defines",
		Args:      []any{c.File, c.Name, types.MangleAtom(c.Kind), c.Start, c.End},
	}}
}

func domainFacts(n int) []types.Fact {
	var out []types.Fact
	for i := 0; i < n; i++ {
		out = append(out, codeSymbol{
			File:  fmt.Sprintf("internal/pkg%d/file%d.go", i%5, i),
			Name:  fmt.Sprintf("Symbol%d", i),
			Kind:  "/func",
			Start: int64(10 + i),
			End:   int64(40 + i),
		}.ToFacts()...)
	}
	return out
}

func factKeys(facts []core.Fact) []string {
	keys := make([]string, 0, len(facts))
	for _, f := range facts {
		parts := make([]string, 0, len(f.Args))
		for _, a := range f.Args {
			// core.baseTermToValue hands back plain strings for name
			// constants while factsnap hands back types.MangleAtom; both
			// re-encode to the same Mangle constant, so compare on the
			// rendered value rather than the Go type. This normalisation is
			// the divergence documented on factsnap.baseTermToValue.
			parts = append(parts, fmt.Sprintf("%v", a))
		}
		keys = append(keys, f.Predicate+"("+strings.Join(parts, ",")+")")
	}
	sort.Strings(keys)
	return keys
}

func TestSnapshotRoundTrip_WhenReloadedIntoFreshKernel_ShouldRestoreSameFacts(t *testing.T) {
	root := t.TempDir()

	source, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Skipf("kernel unavailable in this environment: %v", err)
	}
	want := domainFacts(120)
	if err := source.LoadFacts(want); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}

	// Export the kernel's view, not the input slice: this is what
	// `nerd snapshot export` does, so the test covers the kernel's own
	// atom -> Fact conversion as well as the codec's.
	exported, err := source.Query("code_defines")
	if err != nil {
		t.Fatalf("query source kernel: %v", err)
	}
	if len(exported) != len(want) {
		t.Fatalf("source kernel holds %d code_defines facts, loaded %d", len(exported), len(want))
	}

	path, err := snapshot.Export(root, "kernel-roundtrip", exported, factsnap.CodecZstd)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if filepath.Dir(path) != snapshot.Dir(root) {
		t.Fatalf("snapshot landed at %s, expected under %s", path, snapshot.Dir(root))
	}

	imported, resolved, err := snapshot.Import(root, "kernel-roundtrip")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if resolved != path {
		t.Fatalf("Import resolved %s, exported %s", resolved, path)
	}
	if len(imported) != len(exported) {
		t.Fatalf("imported %d facts, exported %d", len(imported), len(exported))
	}

	// Fresh workspace, fresh kernel: nothing carries over in memory.
	target, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("second kernel: %v", err)
	}
	if err := target.LoadFacts(imported); err != nil {
		t.Fatalf("LoadFacts(imported): %v", err)
	}
	restored, err := target.Query("code_defines")
	if err != nil {
		t.Fatalf("query target kernel: %v", err)
	}

	got, expect := factKeys(restored), factKeys(exported)
	if len(got) != len(expect) {
		t.Fatalf("restored %d facts, expected %d", len(got), len(expect))
	}
	for i := range expect {
		if got[i] != expect[i] {
			t.Fatalf("fact %d diverged after the round trip:\n  before: %s\n  after:  %s", i, expect[i], got[i])
		}
	}
}

func TestSnapshotRoundTrip_WhenSnapshotCorrupted_ShouldRefuseToImport(t *testing.T) {
	root := t.TempDir()
	path, err := snapshot.Export(root, "corrupt-me", domainFacts(50), factsnap.CodecGzip)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	data := mustRead(t, path)
	mustWrite(t, path, data[:len(data)-16])

	// Import must not hand a caller a plausible-looking short fact set that
	// would then be asserted into a kernel as truth.
	if _, _, err := snapshot.Import(root, "corrupt-me"); err == nil {
		t.Fatal("expected a corrupted snapshot to fail import")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
