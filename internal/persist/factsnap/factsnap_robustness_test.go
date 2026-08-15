package factsnap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/types"
)

// roundTrip writes facts and reads them back from the canonical path.
func roundTrip(t *testing.T, dir, name string, facts []types.Fact, codec Codec) []types.Fact {
	t.Helper()
	path, err := WritePath(filepath.Join(dir, name), facts, Options{Codec: codec})
	if err != nil {
		t.Fatalf("WritePath(%s): %v", name, err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read(%s): %v", path, err)
	}
	return got
}

func TestRead_WhenSuffixStripped_ShouldSniffGzipMagic(t *testing.T) {
	dir := t.TempDir()
	facts := sampleFacts(50)
	path, err := WritePath(filepath.Join(dir, "snap"), facts, Options{Codec: CodecGzip})
	if err != nil {
		t.Fatalf("WritePath: %v", err)
	}

	// Simulate an operator copy that dropped the extension. The sidecar is
	// left behind on purpose so the read exercises only the content sniff.
	bare := filepath.Join(dir, "renamed-no-suffix")
	if err := os.Rename(path, bare); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := Read(bare)
	if err != nil {
		t.Fatalf("Read(bare gzip): %v", err)
	}
	if !equalishFacts(facts, got) {
		t.Fatalf("sniffed gzip read diverged: wrote %d read %d", len(facts), len(got))
	}
}

func TestRead_WhenSuffixStripped_ShouldSniffZstdMagic(t *testing.T) {
	dir := t.TempDir()
	facts := sampleFacts(50)
	path, err := WritePath(filepath.Join(dir, "snap"), facts, Options{Codec: CodecZstd})
	if err != nil {
		t.Fatalf("WritePath: %v", err)
	}
	bare := filepath.Join(dir, "renamed-no-suffix")
	if err := os.Rename(path, bare); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := Read(bare)
	if err != nil {
		t.Fatalf("Read(bare zstd): %v", err)
	}
	if !equalishFacts(facts, got) {
		t.Fatalf("sniffed zstd read diverged")
	}
}

// A file renamed from .sc.zst to .sc.gz used to be a hard decode error even
// though the bytes were perfectly readable; content now wins over the name.
func TestRead_WhenSuffixContradictsContent_ShouldTrustContent(t *testing.T) {
	dir := t.TempDir()
	facts := sampleFacts(30)
	path, err := WritePath(filepath.Join(dir, "snap"), facts, Options{Codec: CodecZstd})
	if err != nil {
		t.Fatalf("WritePath: %v", err)
	}
	lying := filepath.Join(dir, "snap"+ExtGzip)
	if err := os.Rename(path, lying); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := Read(lying)
	if err != nil {
		t.Fatalf("Read(mislabelled): %v", err)
	}
	if !equalishFacts(facts, got) {
		t.Fatalf("mislabelled read diverged")
	}
}

func TestRead_WhenJSONHasNoMagic_ShouldStillUseLegacyPath(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "legacy.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"Predicate":"p","Args":["a"]}]`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	got, err := Read(jsonPath)
	if err != nil {
		t.Fatalf("Read(json): %v", err)
	}
	if len(got) != 1 || got[0].Predicate != "p" {
		t.Fatalf("legacy json decode wrong: %#v", got)
	}
}

func TestWrite_WhenDefaultOptions_ShouldEmitSha256Sidecar(t *testing.T) {
	dir := t.TempDir()
	path, err := WritePath(filepath.Join(dir, "snap"), sampleFacts(10), Options{})
	if err != nil {
		t.Fatalf("WritePath: %v", err)
	}
	if !HasSidecar(path) {
		t.Fatalf("expected sidecar at %s", path+ExtSHA256)
	}
	raw, err := os.ReadFile(path + ExtSHA256)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	// sha256sum(1) format: "<64 hex>  <basename>\n" so `sha256sum -c` works.
	fields := strings.Fields(string(raw))
	if len(fields) != 2 || len(fields[0]) != 64 || fields[1] != filepath.Base(path) {
		t.Fatalf("sidecar not sha256sum-shaped: %q", string(raw))
	}
	if err := Verify(path); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestRead_WhenSnapshotTruncated_ShouldFailIntegrityCheck(t *testing.T) {
	dir := t.TempDir()
	path, err := WritePath(filepath.Join(dir, "snap"), sampleFacts(200), Options{})
	if err != nil {
		t.Fatalf("WritePath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// A partially flushed file is the realistic corruption: gzip may still
	// decode a prefix, so without the sidecar this returns a short, plausible
	// fact set that a caller would happily assert into the kernel.
	if err := os.WriteFile(path, data[:len(data)/2], 0o644); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := Read(path); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("expected ErrIntegrity, got %v", err)
	}
}

func TestWrite_WhenNoSidecarRequested_ShouldRemoveStaleSidecar(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "snap")
	path, err := WritePath(base, sampleFacts(10), Options{})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := WritePath(base, sampleFacts(20), Options{NoSidecar: true}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if HasSidecar(path) {
		// Keeping the old digest would make every subsequent Read fail.
		t.Fatalf("stale sidecar survived a NoSidecar write")
	}
	if _, err := Read(path); err != nil {
		t.Fatalf("Read after NoSidecar rewrite: %v", err)
	}
}

func TestWrite_WhenWritersRaceOnOnePath_ShouldLeaveOneReadableSnapshot(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "contended")

	var wg sync.WaitGroup
	for i := 1; i <= 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if err := Write(base, sampleFacts(n*40)); err != nil {
				t.Errorf("concurrent write %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	// The point of temp+fsync+rename plus the per-path lock: whoever lost the
	// race must not have left a half-written file or a digest describing some
	// other writer's bytes.
	got, err := Read(CanonicalPath(base, CodecGzip))
	if err != nil {
		t.Fatalf("Read after contention: %v", err)
	}
	if len(got)%40 != 0 || len(got) == 0 {
		t.Fatalf("contended snapshot is not any single writer's fact set: %d facts", len(got))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWrite_WhenFactsEmpty_ShouldRoundTripToEmpty(t *testing.T) {
	dir := t.TempDir()
	for _, codec := range []Codec{CodecGzip, CodecZstd} {
		got := roundTrip(t, dir, "empty-"+CodecName(codec), []types.Fact{}, codec)
		if len(got) != 0 {
			t.Fatalf("%s: expected empty read, got %d facts", CodecName(codec), len(got))
		}
	}
	// nil must behave like an empty slice, not panic or write a broken header.
	got := roundTrip(t, dir, "nilfacts", nil, CodecGzip)
	if len(got) != 0 {
		t.Fatalf("nil facts: expected empty read, got %d", len(got))
	}
}

// Booleans are not a Mangle constant type: types.Fact.ToAtom encodes them as
// the /true and /false name constants, so a snapshot round trip returns
// MangleAtom, not bool. That is lossy on the first hop and stable afterwards;
// callers that need a Go bool back must map it themselves.
func TestBool_WhenRoundTripped_ShouldBecomeNameConstantAndThenStayStable(t *testing.T) {
	dir := t.TempDir()
	facts := []types.Fact{{Predicate: "flagged", Args: []any{"target", true, false}}}

	hop1 := roundTrip(t, dir, "bool1", facts, CodecGzip)
	if len(hop1) != 1 || len(hop1[0].Args) != 3 {
		t.Fatalf("unexpected shape: %#v", hop1)
	}
	if got, want := hop1[0].Args[1], types.MangleAtom("/true"); got != want {
		t.Fatalf("true became %#v (%T), want %#v", got, got, want)
	}
	if got, want := hop1[0].Args[2], types.MangleAtom("/false"); got != want {
		t.Fatalf("false became %#v (%T), want %#v", got, got, want)
	}

	hop2 := roundTrip(t, dir, "bool2", hop1, CodecGzip)
	if !equalishFacts(hop1, hop2) {
		t.Fatalf("bool encoding not stable across a second hop: %#v vs %#v", hop1, hop2)
	}
}

// Float multi-hop. Fractional values survive intact; whole-valued floats do
// not, because mangle-go renders Float64(2.0) as the text "2", which the
// SimpleColumn reader parses back as a NumberType. Pinning it here so the
// asymmetry is a known contract rather than a surprise at a call site: a
// caller that must keep float identity for whole values has to carry its own
// tag argument.
func TestFloat_WhenRoundTrippedTwice_ShouldBeStableAfterFirstHop(t *testing.T) {
	dir := t.TempDir()
	facts := []types.Fact{
		{Predicate: "score", Args: []any{"a", 0.1}},
		{Predicate: "score", Args: []any{"b", -1.5}},
		{Predicate: "score", Args: []any{"c", 2.0}},
		{Predicate: "score", Args: []any{"d", 0.0}},
	}

	hop1 := roundTrip(t, dir, "float1", facts, CodecGzip)
	byKey := map[string]any{}
	for _, f := range hop1 {
		byKey[normalizeArg(f.Args[0]).(string)] = f.Args[1]
	}
	if got := byKey["a"]; got != any(0.1) {
		t.Fatalf("0.1 became %#v (%T)", got, got)
	}
	if got := byKey["b"]; got != any(-1.5) {
		t.Fatalf("-1.5 became %#v (%T)", got, got)
	}
	if got := byKey["c"]; got != any(int64(2)) {
		t.Fatalf("whole float 2.0 expected to degrade to int64(2), got %#v (%T)", got, got)
	}
	if got := byKey["d"]; got != any(int64(0)) {
		t.Fatalf("whole float 0.0 expected to degrade to int64(0), got %#v (%T)", got, got)
	}

	hop2 := roundTrip(t, dir, "float2", hop1, CodecGzip)
	if !equalishFacts(hop1, hop2) {
		t.Fatalf("float encoding not stable across a second hop")
	}
	hop3 := roundTrip(t, dir, "float3", hop2, CodecZstd)
	if !equalishFacts(hop2, hop3) {
		t.Fatalf("float encoding not stable across a codec change")
	}
}

// Pins the deliberate divergence from core.baseTermToValue documented on
// factsnap.baseTermToValue: names come back as MangleAtom so deep paths do not
// silently degrade to string constants on the next hop.
func TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom(t *testing.T) {
	dir := t.TempDir()
	facts := []types.Fact{
		{Predicate: "role", Args: []any{types.MangleAtom("/coder")}},
		{Predicate: "role", Args: []any{types.MangleAtom("/a/b/c.go")}},
	}
	hop1 := roundTrip(t, dir, "names1", facts, CodecGzip)
	for _, f := range hop1 {
		if _, ok := f.Args[0].(types.MangleAtom); !ok {
			t.Fatalf("name constant came back as %#v (%T), want types.MangleAtom", f.Args[0], f.Args[0])
		}
	}
	hop2 := roundTrip(t, dir, "names2", hop1, CodecGzip)
	if !equalishFacts(hop1, hop2) {
		t.Fatalf("name constants not stable across a second hop: %#v vs %#v", hop1, hop2)
	}
	for _, f := range hop2 {
		if _, ok := f.Args[0].(types.MangleAtom); !ok {
			t.Fatalf("hop 2 lost the name constant: %#v (%T)", f.Args[0], f.Args[0])
		}
	}
}
