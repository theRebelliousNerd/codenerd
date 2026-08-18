package snapshot

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/persist/factsnap"
	"codenerd/internal/types"
)

var (
	defaultNamePattern = regexp.MustCompile(`^snapshot-\d{8}-\d{6}$`)
	kernelNamePattern  = regexp.MustCompile(`^kernel-\d{8}-\d{6}$`)
)

func facts(n int) []types.Fact {
	out := make([]types.Fact, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, types.Fact{
			Predicate: "code_defines",
			Args: []any{
				filepath.ToSlash(filepath.Join("internal", "pkg", "f.go")),
				"sym",
				types.MangleAtom("/func"),
				int64(i),
			},
		})
	}
	return out
}

func TestExport_WhenNamedSnapshot_ShouldLandUnderNerdSnapshots(t *testing.T) {
	root := t.TempDir()
	path, err := Export(root, "kernel-test", facts(25), factsnap.CodecGzip)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	want := filepath.Join(root, ".nerd", "snapshots", "kernel-test"+factsnap.ExtGzip)
	if path != want {
		t.Fatalf("export path = %s, want %s", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
	if !factsnap.HasSidecar(path) {
		t.Fatalf("expected integrity sidecar next to %s", path)
	}
}

func TestImport_WhenReferencedByBareName_ShouldResolveExtension(t *testing.T) {
	root := t.TempDir()
	want := facts(40)
	if _, err := Export(root, "kernel-bare", want, factsnap.CodecZstd); err != nil {
		t.Fatalf("Export: %v", err)
	}

	got, path, err := Import(root, "kernel-bare")
	if err != nil {
		t.Fatalf("Import by bare name: %v", err)
	}
	if !strings.HasSuffix(path, factsnap.ExtZstd) {
		t.Fatalf("resolved to %s, expected a .sc.zst file", path)
	}
	if len(got) != len(want) {
		t.Fatalf("imported %d facts, exported %d", len(got), len(want))
	}
}

func TestImport_WhenReferenceUnknown_ShouldFailWithDirectoryHint(t *testing.T) {
	root := t.TempDir()
	_, _, err := Import(root, "nope")
	if err == nil {
		t.Fatal("expected an error for a missing snapshot")
	}
	if !strings.Contains(err.Error(), Dir(root)) {
		t.Fatalf("error should name the snapshot dir, got: %v", err)
	}
}

func TestSanitizeName_WhenNameEscapesDirectory_ShouldReject(t *testing.T) {
	bad := []string{"", "  ", "../evil", "sub/dir", `win\dir`, ".hidden", "..", "bad name", "semi;colon"}
	for _, name := range bad {
		if got, err := SanitizeName(name); err == nil {
			t.Fatalf("SanitizeName(%q) accepted it as %q", name, got)
		}
	}
	// A name that already carries the codec suffix must not double it up.
	got, err := SanitizeName("kernel" + factsnap.ExtGzip)
	if err != nil {
		t.Fatalf("SanitizeName: %v", err)
	}
	if got != "kernel" {
		t.Fatalf("SanitizeName stripped to %q, want %q", got, "kernel")
	}
}

func TestExport_WhenNameEscapesDirectory_ShouldNotWriteOutsideSnapshots(t *testing.T) {
	root := t.TempDir()
	if _, err := Export(root, "../../escape", facts(1), factsnap.CodecGzip); err == nil {
		t.Fatal("expected traversal name to be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape"+factsnap.ExtGzip)); err == nil {
		t.Fatal("a file was written outside the snapshot directory")
	}
}

func TestList_WhenDirectoryMissing_ShouldReturnEmptyNotError(t *testing.T) {
	entries, err := List(t.TempDir())
	if err != nil {
		t.Fatalf("List on fresh workspace: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestList_WhenSnapshotsExist_ShouldSkipSidecarsAndReportCodec(t *testing.T) {
	root := t.TempDir()
	if _, err := Export(root, "one", facts(10), factsnap.CodecGzip); err != nil {
		t.Fatalf("Export one: %v", err)
	}
	if _, err := Export(root, "two", facts(10), factsnap.CodecZstd); err != nil {
		t.Fatalf("Export two: %v", err)
	}
	// A leftover temp file from a killed export must not show up as a snapshot.
	if err := os.WriteFile(filepath.Join(Dir(root), ".one.sc.gz.tmp123"), []byte("junk"), 0o644); err != nil {
		t.Fatalf("seed temp: %v", err)
	}

	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name+"/"+e.Codec)
		}
		t.Fatalf("expected 2 snapshots, got %d: %v", len(entries), names)
	}
	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	if byName["one"].Codec != "gzip" || byName["two"].Codec != "zstd" {
		t.Fatalf("codec detection wrong: %+v", entries)
	}
	for _, e := range entries {
		if !e.Verifiable {
			t.Fatalf("%s should have an integrity sidecar", e.Name)
		}
		if e.Bytes <= 0 {
			t.Fatalf("%s reported %d bytes", e.Name, e.Bytes)
		}
	}
}

func TestSummarize_WhenMixedPredicates_ShouldOrderByCountThenName(t *testing.T) {
	in := []types.Fact{
		{Predicate: "b", Args: []any{1}},
		{Predicate: "a", Args: []any{1}},
		{Predicate: "c", Args: []any{1}},
		{Predicate: "c", Args: []any{2}},
	}
	got := Summarize(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	if got[0].Predicate != "c" || got[0].Count != 2 {
		t.Fatalf("first row = %+v, want c/2", got[0])
	}
	if got[1].Predicate != "a" || got[2].Predicate != "b" {
		t.Fatalf("ties should sort by name, got %+v", got)
	}
}

func TestCodecFor_WhenAliasGiven_ShouldMapOrReject(t *testing.T) {
	for _, name := range []string{"", "gzip", "gz", "auto"} {
		if c, err := CodecFor(name); err != nil || c != factsnap.CodecGzip {
			t.Fatalf("CodecFor(%q) = %v, %v", name, c, err)
		}
	}
	for _, name := range []string{"zstd", "ZST"} {
		if c, err := CodecFor(name); err != nil || c != factsnap.CodecZstd {
			t.Fatalf("CodecFor(%q) = %v, %v", name, c, err)
		}
	}
	if _, err := CodecFor("brotli"); err == nil {
		t.Fatal("expected unknown codec to be rejected")
	}
}

func TestDefaultName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		regex  *regexp.Regexp
	}{
		{
			name:   "empty prefix defaults to snapshot",
			prefix: "",
			regex:  defaultNamePattern,
		},
		{
			name:   "custom prefix",
			prefix: "kernel",
			regex:  kernelNamePattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultName(tt.prefix)
			if !tt.regex.MatchString(got) {
				t.Errorf("DefaultName(%q) = %v, want matching %v", tt.prefix, got, tt.regex)
			}
		})
	}
}

func TestResolve_EmptyReference(t *testing.T) {
	_, err := Resolve("irrelevant", "   ")
	if err == nil {
		t.Fatal("expected error for empty reference")
	}
}

func TestResolve_ExplicitPath(t *testing.T) {
	root := t.TempDir()

	explicitFile := filepath.Join(root, "explicit_file"+factsnap.ExtGzip)
	if err := os.WriteFile(explicitFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(root, explicitFile)
	if err != nil {
		t.Fatalf("Resolve exact explicit path failed: %v", err)
	}
	if got != explicitFile {
		t.Fatalf("Resolve explicit path mismatch: got %q, want %q", got, explicitFile)
	}

	bareExplicit := filepath.Join(root, "explicit_file")
	gotBare, err := Resolve(root, bareExplicit)
	if err != nil {
		t.Fatalf("Resolve bare explicit path failed: %v", err)
	}
	if gotBare != explicitFile {
		t.Fatalf("Resolve bare explicit path mismatch: got %q, want %q", gotBare, explicitFile)
	}
}

func TestResolve_BareName(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}

	snapFile := filepath.Join(Dir(root), "snap1"+factsnap.ExtZstd)
	if err := os.WriteFile(snapFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(root, "snap1")
	if err != nil {
		t.Fatalf("Resolve bare name failed: %v", err)
	}
	if got != snapFile {
		t.Fatalf("Resolve bare name mismatch: got %q, want %q", got, snapFile)
	}

	exactFile := filepath.Join(Dir(root), "snap3.custom")
	if err := os.WriteFile(exactFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotExact, err := Resolve(root, "snap3.custom")
	if err != nil {
		t.Fatalf("Resolve exact name failed: %v", err)
	}
	if gotExact != exactFile {
		t.Fatalf("Resolve exact name mismatch: got %q, want %q", gotExact, exactFile)
	}
}

func TestResolve_NoMatch(t *testing.T) {
	root := t.TempDir()
	_, err := Resolve(root, "non_existent_file")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}
