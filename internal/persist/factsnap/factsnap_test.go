package factsnap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"codenerd/internal/types"
)

// sampleFacts generates n facts spanning several predicates and arities so
// the test exercises the column-oriented path.
func sampleFacts(n int) []types.Fact {
	facts := make([]types.Fact, 0, n)
	for i := range n {
		switch i % 4 {
		case 0:
			facts = append(facts, types.Fact{
				Predicate: "code_defines",
				Args: []any{
					fmt.Sprintf("/internal/pkg_%d/file_%d.go", i%17, i),
					fmt.Sprintf("symbol_%d", i),
					types.MangleAtom("/func"),
					int64(10 + i%200),
					int64(40 + i%200),
				},
			})
		case 1:
			facts = append(facts, types.Fact{
				Predicate: "code_calls",
				Args: []any{
					fmt.Sprintf("caller_%d", i),
					fmt.Sprintf("callee_%d", (i*7)%500),
				},
			})
		case 2:
			facts = append(facts, types.Fact{
				Predicate: "projected_fact",
				Args: []any{
					fmt.Sprintf("dream:write_file:%d", i),
					types.MangleAtom("/modified"),
					fmt.Sprintf("/workspace/foo/bar_%d.go", i),
				},
			})
		case 3:
			facts = append(facts, types.Fact{
				Predicate: "campaign_task",
				Args: []any{
					fmt.Sprintf("/task_%d", i),
					fmt.Sprintf("/phase_%d", i%5),
					fmt.Sprintf("Do thing %d for the campaign", i),
					"/pending",
					"/code",
				},
			})
		}
	}
	return facts
}

// sortFacts gives a deterministic order so DeepEqual works regardless of the
// underlying store's iteration order.
func sortFacts(in []types.Fact) []types.Fact {
	out := make([]types.Fact, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Predicate != out[j].Predicate {
			return out[i].Predicate < out[j].Predicate
		}
		return fmt.Sprintf("%v", out[i].Args) < fmt.Sprintf("%v", out[j].Args)
	})
	return out
}

// equalishFacts compares two fact slices ignoring numeric int64 vs int slop.
func equalishFacts(a, b []types.Fact) bool {
	a = sortFacts(a)
	b = sortFacts(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Predicate != b[i].Predicate {
			return false
		}
		if len(a[i].Args) != len(b[i].Args) {
			return false
		}
		for j := range a[i].Args {
			if !reflect.DeepEqual(normalizeArg(a[i].Args[j]), normalizeArg(b[i].Args[j])) {
				return false
			}
		}
	}
	return true
}

// normalizeArg collapses representational ambiguity for comparison:
//   - int  -> int64                       (Go-side numeric defaults)
//   - string starting with "/" -> MangleAtom   (Mangle re-reads names as MangleAtom)
//
// types.Fact.ToAtom() automatically treats a "/foo" string as a name constant
// so the AST never carries the distinction; on read we always materialise
// names as MangleAtom for symmetry.
func normalizeArg(v any) any {
	switch x := v.(type) {
	case int:
		return int64(x)
	case string:
		if len(x) > 0 && x[0] == '/' {
			return types.MangleAtom(x)
		}
		return x
	default:
		return v
	}
}

func TestRoundTripGzip(t *testing.T) {
	facts := sampleFacts(1000)
	dir := t.TempDir()
	path := filepath.Join(dir, "snap")

	if err := Write(path, facts); err != nil {
		t.Fatalf("Write: %v", err)
	}

	canonical := CanonicalPath(path, CodecGzip)
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("expected snapshot at %s: %v", canonical, err)
	}

	got, err := Read(canonical)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(got) != len(facts) {
		t.Fatalf("fact count mismatch: wrote %d read %d", len(facts), len(got))
	}
	if !equalishFacts(facts, got) {
		a := sortFacts(facts)
		b := sortFacts(got)
		for i := 0; i < len(a) && i < len(b) && i < 5; i++ {
			t.Logf("want[%d] = {pred=%s args=%#v}", i, a[i].Predicate, a[i].Args)
			t.Logf("got [%d] = {pred=%s args=%#v}", i, b[i].Predicate, b[i].Args)
		}
		t.Fatalf("round trip diverged")
	}
}

func TestRoundTripZstd(t *testing.T) {
	facts := sampleFacts(1000)
	dir := t.TempDir()
	path := filepath.Join(dir, "snap")

	if err := WriteCodec(path, facts, CodecZstd); err != nil {
		t.Fatalf("WriteCodec(zstd): %v", err)
	}
	canonical := CanonicalPath(path, CodecZstd)
	got, err := Read(canonical)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !equalishFacts(facts, got) {
		t.Fatalf("zstd round trip diverged")
	}
}

func TestLegacyJSONFallback(t *testing.T) {
	facts := sampleFacts(20)
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "legacy.json")

	data, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Read(jsonPath)
	if err != nil {
		t.Fatalf("Read legacy: %v", err)
	}
	if len(got) != len(facts) {
		t.Fatalf("legacy round trip count mismatch: %d vs %d", len(got), len(facts))
	}
}

func TestCanonicalPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		codec    Codec
		expected string
	}{
		{
			name:     "gzip exact match",
			path:     "snapshot" + ExtGzip,
			codec:    CodecGzip,
			expected: "snapshot" + ExtGzip,
		},
		{
			name:     "gzip append missing",
			path:     "snapshot",
			codec:    CodecGzip,
			expected: "snapshot" + ExtGzip,
		},
		{
			name:     "gzip replace .json",
			path:     "snapshot" + ExtJSON,
			codec:    CodecGzip,
			expected: "snapshot" + ExtGzip,
		},
		{
			name:     "zstd exact match",
			path:     "snapshot" + ExtZstd,
			codec:    CodecZstd,
			expected: "snapshot" + ExtZstd,
		},
		{
			name:     "zstd append missing",
			path:     "snapshot",
			codec:    CodecZstd,
			expected: "snapshot" + ExtZstd,
		},
		{
			name:     "zstd replace .json",
			path:     "snapshot" + ExtJSON,
			codec:    CodecZstd,
			expected: "snapshot" + ExtZstd,
		},
		{
			name:     "default codec leaves path untouched",
			path:     "snapshot.foo",
			codec:    CodecAuto,
			expected: "snapshot.foo",
		},
		{
			name:     "unknown codec leaves path untouched",
			path:     "snapshot.foo",
			codec:    Codec(999),
			expected: "snapshot.foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanonicalPath(tt.path, tt.codec)
			if got != tt.expected {
				t.Errorf("CanonicalPath(%q, %v) = %q; want %q", tt.path, tt.codec, got, tt.expected)
			}
		})
	}
}

// TestSizeComparison writes the same 1000-fact corpus as JSON, SimpleColumn+gzip,
// and SimpleColumn+zstd, then prints all three sizes. This is informational so
// the maintainer can see the compression win at a glance.
func TestSizeComparison(t *testing.T) {
	facts := sampleFacts(1000)
	dir := t.TempDir()

	// JSON baseline.
	jsonData, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	jsonPath := filepath.Join(dir, "facts.json")
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		t.Fatalf("json write: %v", err)
	}

	gzPath := filepath.Join(dir, "facts.sc.gz")
	if err := WriteCodec(gzPath, facts, CodecGzip); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	zstPath := filepath.Join(dir, "facts.sc.zst")
	if err := WriteCodec(zstPath, facts, CodecZstd); err != nil {
		t.Fatalf("zstd write: %v", err)
	}

	jsonSize := mustSize(t, jsonPath)
	gzSize := mustSize(t, gzPath)
	zstSize := mustSize(t, zstPath)

	t.Logf("1000-fact corpus sizes:")
	t.Logf("  json             : %7d bytes", jsonSize)
	t.Logf("  simplecolumn.gz  : %7d bytes  (%.1f%% of json)", gzSize, 100*float64(gzSize)/float64(jsonSize))
	t.Logf("  simplecolumn.zst : %7d bytes  (%.1f%% of json)", zstSize, 100*float64(zstSize)/float64(jsonSize))

	if gzSize >= jsonSize {
		t.Errorf("expected gzip-wrapped simplecolumn smaller than json: gz=%d json=%d", gzSize, jsonSize)
	}
}

func mustSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Size()
}

func TestHasSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap")
	sidecarPath := path + ExtSHA256

	if HasSidecar(path) {
		t.Errorf("expected HasSidecar(path) to be false when no sidecar exists")
	}

	if err := os.Mkdir(sidecarPath, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if HasSidecar(path) {
		t.Errorf("expected HasSidecar(path) to be false when sidecar is a directory")
	}

	if err := os.Remove(sidecarPath); err != nil {
		t.Fatalf("failed to remove directory: %v", err)
	}

	if err := os.WriteFile(sidecarPath, []byte("fake sha256 data"), 0o644); err != nil {
		t.Fatalf("failed to write sidecar file: %v", err)
	}
	if !HasSidecar(path) {
		t.Errorf("expected HasSidecar(path) to be true when sidecar is a file")
	}
}
