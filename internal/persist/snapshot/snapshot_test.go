package snapshot

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"codenerd/internal/persist/factsnap"
	"codenerd/internal/types"
)

// TODO: Missing test for Resolve handling a reference that is solely a codec extension.
// TODO: Summarize test with facts containing empty Predicate strings.
// TODO: Export and SanitizeName with an extremely long filename (e.g., 300+ characters).
// TODO: Missing tests for DefaultName when the prefix is a string formatted exactly like the timestamp suffix.
// TODO: List must be explicitly tested with multiple snapshots having the exact same ModTime to ensure fallback sorting.
// TODO: Missing test for List when .nerd/snapshots is a file.
// TODO: Resolve with identical bare names but different codec extensions to verify deterministic resolution order.

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

func TestDir_WhenVariousRoots_ShouldReturnCanonicalPath(t *testing.T) {
	cases := []struct {
		root     string
		expected string
	}{
		{"", filepath.Join(".", ".nerd", "snapshots")},
		{".", filepath.Join(".", ".nerd", "snapshots")},
		{"/custom/root", filepath.Join("/custom/root", ".nerd", "snapshots")},
		{"myworkspace", filepath.Join("myworkspace", ".nerd", "snapshots")},
	}

	for _, tc := range cases {
		name := strings.ReplaceAll(tc.root, "/", "_")
		if name == "" {
			name = "empty"
		} else if name == "." {
			name = "dot"
		}
		t.Run(name, func(t *testing.T) {
			actual := Dir(tc.root)
			if actual != tc.expected {
				t.Fatalf("Dir(%q) = %q, want %q", tc.root, actual, tc.expected)
			}
		})
	}
}

func TestResolve_WhenReferenceIsOnlyCodecSuffix_ShouldFailWithDirectoryHint(t *testing.T) {
	root := t.TempDir()
	for _, ref := range []string{factsnap.ExtGzip, factsnap.ExtZstd, factsnap.ExtJSON} {
		_, err := Resolve(root, ref)
		if err == nil {
			t.Fatalf("Resolve(%q) should fail, it is only a codec suffix", ref)
		}
		if !strings.Contains(err.Error(), Dir(root)) {
			t.Fatalf("Resolve(%q) error should name snapshot dir, got: %v", ref, err)
		}
	}
}

func TestSummarize_WhenEmptyPredicate_ShouldCountAndSort(t *testing.T) {
	in := []types.Fact{
		{Predicate: "", Args: []any{int64(1)}},
		{Predicate: "a", Args: []any{int64(1)}},
		{Predicate: "", Args: []any{int64(2)}},
	}
	got := Summarize(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(got), got)
	}
	if got[0].Predicate != "" || got[0].Count != 2 {
		t.Fatalf("first row = %+v, want empty predicate with count 2", got[0])
	}
	if got[1].Predicate != "a" || got[1].Count != 1 {
		t.Fatalf("second row = %+v, want a/1", got[1])
	}
	if out := Summarize(nil); len(out) != 0 {
		t.Fatalf("Summarize(nil) = %+v, want empty", out)
	}
}

func TestSanitizeName_WhenExtremelyLongName_ShouldAccept(t *testing.T) {
	long := strings.Repeat("a", 300)
	got, err := SanitizeName(long)
	if err != nil {
		t.Fatalf("SanitizeName(300 chars): %v", err)
	}
	if got != long {
		t.Fatalf("long name round-trip = %q, want %q", got, long)
	}
}

func TestExport_WhenExtremelyLongName_ShouldNotEscape(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("b", 300)
	path, err := Export(root, long, facts(1), factsnap.CodecGzip)
	if err != nil {
		// Filesystem name limit (typically 255 bytes) makes failure acceptable;
		// the invariant is containment, not success.
		if _, statErr := os.Stat(filepath.Join(filepath.Dir(root), long+factsnap.ExtGzip)); statErr == nil {
			t.Fatal("long-name export wrote outside the snapshot directory")
		}
		return
	}
	if !strings.HasPrefix(path, Dir(root)) {
		t.Fatalf("export path %q escapes snapshot dir %q", path, Dir(root))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("exported long-name snapshot missing: %v", err)
	}
}

func TestDefaultName_WhenPrefixLooksLikeTimestamp_ShouldStillPrefix(t *testing.T) {
	prefix := "20260815-140501"
	got := DefaultName(prefix)
	want := regexp.MustCompile(`^20260815-140501-\d{8}-\d{6}$`)
	if !want.MatchString(got) {
		t.Fatalf("DefaultName(%q) = %q, want prefix plus fresh timestamp", prefix, got)
	}
	if got == prefix {
		t.Fatalf("DefaultName returned the prefix unchanged: %q", got)
	}
}

func TestList_WhenSameModTime_ShouldFallbackToNameOrder(t *testing.T) {
	root := t.TempDir()
	if _, err := Export(root, "beta", facts(2), factsnap.CodecGzip); err != nil {
		t.Fatalf("Export beta: %v", err)
	}
	if _, err := Export(root, "alpha", facts(2), factsnap.CodecGzip); err != nil {
		t.Fatalf("Export alpha: %v", err)
	}
	fixed := time.Date(2026, 8, 15, 14, 5, 1, 0, time.UTC)
	for _, name := range []string{"alpha", "beta"} {
		p := filepath.Join(Dir(root), name+factsnap.ExtGzip)
		if err := os.Chtimes(p, fixed, fixed); err != nil {
			t.Fatalf("Chtimes %s: %v", name, err)
		}
		if sc := p + factsnap.ExtSHA256; true {
			_ = os.Chtimes(sc, fixed, fixed)
		}
	}
	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "alpha" || entries[1].Name != "beta" {
		t.Fatalf("same-ModTime entries should sort by name, got %q then %q", entries[0].Name, entries[1].Name)
	}
}

func TestList_WhenSnapshotsPathIsFile_ShouldReturnError(t *testing.T) {
	root := t.TempDir()
	dir := Dir(root)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(root); err == nil {
		t.Fatal("expected List to fail when snapshots path is a regular file")
	} else if !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("error should mention snapshot, got: %v", err)
	}
}

func TestResolve_WhenBareNameHasBothCodecs_ShouldPreferGzip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	gz := filepath.Join(Dir(root), "dupe"+factsnap.ExtGzip)
	zst := filepath.Join(Dir(root), "dupe"+factsnap.ExtZstd)
	for _, p := range []string{gz, zst} {
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Resolve(root, "dupe")
	if err != nil {
		t.Fatalf("Resolve dupe: %v", err)
	}
	if got != gz {
		t.Fatalf("Resolve dupe = %q, want deterministic gzip %q", got, gz)
	}
}

func TestList_WhenMixedEntryKinds_ShouldSkipNonSnapshots(t *testing.T) {
	root := t.TempDir()
	if _, err := Export(root, "keep-gz", facts(3), factsnap.CodecGzip); err != nil {
		t.Fatalf("Export keep-gz: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(Dir(root), "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	junk := []string{".hidden" + factsnap.ExtGzip, "notes.txt", "stale" + factsnap.ExtGzip + ".tmp1"}
	for _, name := range junk {
		if err := os.WriteFile(filepath.Join(Dir(root), name), []byte("junk"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(Dir(root), "legacy"+factsnap.ExtJSON), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(root), "noside"+factsnap.ExtGzip), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name+"/"+e.Codec)
		}
		t.Fatalf("expected 3 snapshots, got %d: %v", len(entries), names)
	}
	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	if byName["keep-gz"].Codec != "gzip" {
		t.Fatalf("keep-gz codec = %q, want gzip", byName["keep-gz"].Codec)
	}
	if !byName["keep-gz"].Verifiable {
		t.Fatal("keep-gz should be verifiable via its sidecar")
	}
	if byName["legacy"].Codec != "json" {
		t.Fatalf("legacy codec = %q, want json", byName["legacy"].Codec)
	}
	if byName["legacy"].Verifiable {
		t.Fatal("legacy without a sidecar must not report Verifiable")
	}
	if byName["noside"].Codec != "gzip" {
		t.Fatalf("noside codec = %q, want gzip", byName["noside"].Codec)
	}
	if byName["noside"].Verifiable {
		t.Fatal("noside without a sidecar must not report Verifiable")
	}
	for _, bad := range []string{"subdir", ".hidden", "notes", "stale"} {
		if _, ok := byName[bad]; ok {
			t.Fatalf("List surfaced non-snapshot %q", bad)
		}
	}
}

func TestList_WhenModTimesDiffer_ShouldOrderNewestFirst(t *testing.T) {
	root := t.TempDir()
	if _, err := Export(root, "older", facts(2), factsnap.CodecGzip); err != nil {
		t.Fatalf("Export older: %v", err)
	}
	if _, err := Export(root, "newer", facts(2), factsnap.CodecGzip); err != nil {
		t.Fatalf("Export newer: %v", err)
	}
	oldTime := time.Date(2026, 8, 15, 14, 5, 1, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	if err := os.Chtimes(filepath.Join(Dir(root), "older"+factsnap.ExtGzip), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(Dir(root), "newer"+factsnap.ExtGzip), newTime, newTime); err != nil {
		t.Fatal(err)
	}
	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "newer" || entries[1].Name != "older" {
		t.Fatalf("newest-first order broken, got %q then %q", entries[0].Name, entries[1].Name)
	}
}

func TestSanitizeName_WhenPaddedOrSuffixed_ShouldNormalize(t *testing.T) {
	got, err := SanitizeName("  kernel  ")
	if err != nil {
		t.Fatalf("SanitizeName padded: %v", err)
	}
	if got != "kernel" {
		t.Fatalf("SanitizeName padded = %q, want %q", got, "kernel")
	}
	got, err = SanitizeName("kernel" + factsnap.ExtZstd)
	if err != nil {
		t.Fatalf("SanitizeName zstd suffix: %v", err)
	}
	if got != "kernel" {
		t.Fatalf("SanitizeName zstd suffix = %q, want %q", got, "kernel")
	}
	got, err = SanitizeName("my.snap_01-2")
	if err != nil {
		t.Fatalf("SanitizeName dotted: %v", err)
	}
	if got != "my.snap_01-2" {
		t.Fatalf("SanitizeName dotted = %q, want it unchanged", got)
	}
	if _, err := SanitizeName("."); err == nil {
		t.Fatal("SanitizeName(\".\") should be rejected")
	}
}

func TestExport_WhenNameHasCodecSuffix_ShouldNotDoubleExtension(t *testing.T) {
	root := t.TempDir()
	path, err := Export(root, "dupname"+factsnap.ExtGzip, facts(2), factsnap.CodecGzip)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	want := filepath.Join(Dir(root), "dupname"+factsnap.ExtGzip)
	if path != want {
		t.Fatalf("export path = %q, want %q (no doubled suffix)", path, want)
	}
	path, err = Export(root, "dupz"+factsnap.ExtZstd, facts(2), factsnap.CodecZstd)
	if err != nil {
		t.Fatalf("Export zstd: %v", err)
	}
	want = filepath.Join(Dir(root), "dupz"+factsnap.ExtZstd)
	if path != want {
		t.Fatalf("export zstd path = %q, want %q", path, want)
	}
	if _, err := Export(root, "   ", facts(1), factsnap.CodecGzip); err == nil {
		t.Fatal("expected empty export name to be rejected")
	}
}

func TestResolve_WhenReferenceHasSpacesOrExtension_ShouldResolve(t *testing.T) {
	root := t.TempDir()
	path, err := Export(root, "spaced", facts(2), factsnap.CodecGzip)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	got, err := Resolve(root, "  spaced  ")
	if err != nil {
		t.Fatalf("Resolve padded: %v", err)
	}
	if got != path {
		t.Fatalf("Resolve padded = %q, want %q", got, path)
	}
	got, err = Resolve(root, "spaced"+factsnap.ExtGzip)
	if err != nil {
		t.Fatalf("Resolve with extension: %v", err)
	}
	if got != path {
		t.Fatalf("Resolve with extension = %q, want %q", got, path)
	}
}

func TestResolve_WhenReferenceIsDirectory_ShouldFail(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(Dir(root), "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(root, "subdir"); err == nil {
		t.Fatal("expected a directory bare name to fail resolution")
	}
	if _, err := Resolve(root, sub); err == nil {
		t.Fatal("expected an explicit directory path to fail resolution")
	}
}

func TestCodecFor_WhenCaseAndSpacesVary_ShouldNormalize(t *testing.T) {
	for _, name := range []string{"  GZIP  ", " Gz ", "GZ"} {
		if c, err := CodecFor(name); err != nil || c != factsnap.CodecGzip {
			t.Fatalf("CodecFor(%q) = %v, %v; want gzip", name, c, err)
		}
	}
	for _, name := range []string{"  ZSTD  ", " zst ", "ZStd"} {
		if c, err := CodecFor(name); err != nil || c != factsnap.CodecZstd {
			t.Fatalf("CodecFor(%q) = %v, %v; want zstd", name, c, err)
		}
	}
}

func TestImport_WhenGivenExplicitPath_ShouldImport(t *testing.T) {
	root := t.TempDir()
	want := facts(5)
	path, err := Export(root, "explicit-import", want, factsnap.CodecGzip)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	got, resolved, err := Import(root, path)
	if err != nil {
		t.Fatalf("Import explicit path: %v", err)
	}
	if resolved != path {
		t.Fatalf("Import resolved %q, want %q", resolved, path)
	}
	if len(got) != len(want) {
		t.Fatalf("imported %d facts, exported %d", len(got), len(want))
	}
}
