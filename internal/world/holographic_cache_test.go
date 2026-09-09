package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTempPkg(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// TestPackageParseCache_HitsOnSecondCall pins the reason the cache exists:
// PromptSection runs once per LLM turn with a file target and used to re-parse
// up to 100 sibling files each time to produce text that is byte-identical
// until a file changes. Measured on internal/core before the cache: 61 ms and
// 12 MB of garbage per turn.
func TestPackageParseCache_HitsOnSecondCall(t *testing.T) {
	dir := t.TempDir()
	writeTempPkg(t, dir, map[string]string{
		"a.go": "package p\n\n// A does a.\nfunc A() error { return nil }\n",
		"b.go": "package p\n\ntype T struct{ X int }\n",
	})
	target := filepath.Join(dir, "a.go")

	h := NewHolographicProvider(nil, dir)
	first := h.PromptSection(context.Background(), target)
	if first == "" {
		t.Fatal("first PromptSection returned nothing")
	}
	hits, misses := h.CacheStats()
	if misses != 1 || hits != 0 {
		t.Fatalf("after first call: hits=%d misses=%d, want 0/1", hits, misses)
	}

	second := h.PromptSection(context.Background(), target)
	hits, misses = h.CacheStats()
	if hits != 1 || misses != 1 {
		t.Fatalf("after second call: hits=%d misses=%d, want 1/1", hits, misses)
	}
	if first != second {
		t.Fatalf("cached render differs from the fresh one:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestPackageParseCache_InvalidatesOnFileChange is the correctness half. A cache
// that keeps answering after the package changed would feed the model a
// description of code that no longer exists.
func TestPackageParseCache_InvalidatesOnFileChange(t *testing.T) {
	dir := t.TempDir()
	writeTempPkg(t, dir, map[string]string{
		"a.go": "package p\n\nfunc Original() error { return nil }\n",
	})
	target := filepath.Join(dir, "a.go")

	h := NewHolographicProvider(nil, dir)
	first := h.PromptSection(context.Background(), target)
	if !strings.Contains(first, "Original") {
		t.Fatalf("first render missing Original:\n%s", first)
	}

	// Distinct mtime: the fingerprint is (name, size, mtime) and the rename is
	// same-length, so without the sleep the two states could be
	// indistinguishable on a coarse-grained clock.
	time.Sleep(10 * time.Millisecond)
	writeTempPkg(t, dir, map[string]string{
		"a.go": "package p\n\nfunc Replaced() error { return nil }\n",
	})

	second := h.PromptSection(context.Background(), target)
	if strings.Contains(second, "Original") {
		t.Fatalf("stale cache served a symbol that no longer exists:\n%s", second)
	}
	if !strings.Contains(second, "Replaced") {
		t.Fatalf("second render missing Replaced:\n%s", second)
	}
}

// TestPackageParseCache_InvalidatesOnNewFile covers the other change shape: a
// sibling appearing changes the package's surface without touching any existing
// file, so the directory fingerprint must move even though every byte that was
// already there is unchanged.
//
// It asserts on the miss counter and the sibling roster rather than on the
// rendered section: PromptSection prefers symbols defined in the target file
// and only falls back to package-wide ones when the target defines none, so a
// new sibling is correctly invisible in the render here.
func TestPackageParseCache_InvalidatesOnNewFile(t *testing.T) {
	dir := t.TempDir()
	writeTempPkg(t, dir, map[string]string{"a.go": "package p\n\nfunc A() {}\n"})
	target := filepath.Join(dir, "a.go")

	h := NewHolographicProvider(nil, dir)
	if _, err := h.GetContext(target); err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if _, misses := h.CacheStats(); misses != 1 {
		t.Fatalf("first call misses = %d, want 1", misses)
	}
	if _, err := h.GetContext(target); err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if hits, _ := h.CacheStats(); hits != 1 {
		t.Fatalf("second call did not hit the cache")
	}

	time.Sleep(10 * time.Millisecond)
	writeTempPkg(t, dir, map[string]string{"b.go": "package p\n\nfunc Sibling() {}\n"})

	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if _, misses := h.CacheStats(); misses != 2 {
		t.Fatalf("a new sibling did not invalidate the cache (misses = %d, want 2)", misses)
	}
	if len(hc.PackageSiblings) != 1 || !strings.HasSuffix(hc.PackageSiblings[0], "b.go") {
		t.Fatalf("sibling roster did not pick up the new file: %v", hc.PackageSiblings)
	}
	var found bool
	for _, sig := range hc.PackageSignatures {
		if sig.Name == "Sibling" {
			found = true
		}
	}
	if !found {
		t.Fatalf("re-parse did not pick up the new sibling's signatures: %+v", hc.PackageSignatures)
	}
}

// TestPackageParseCache_DeepCopiesBothWays pins the lesson internal/diff learned
// the hard way: a cache that hands a caller its own memory turns any downstream
// append into silent corruption of every later reader.
func TestPackageParseCache_DeepCopiesBothWays(t *testing.T) {
	src := &packageParse{
		goFiles:    []string{"a.go"},
		allGoFiles: []string{"a.go"},
		signatures: []SymbolSignature{{Name: "A"}},
		types:      []TypeDefinition{{Name: "T", Kind: "struct", Fields: []string{"X int"}}},
		constants:  []ConstDefinition{{Name: "C"}},
		imports:    map[string][]string{"a.go": {"context"}},
		pkgName:    map[string]string{"a.go": "p"},
	}

	c := newPackageParseCache()
	c.put("/dir", "fp", src)

	// Mutating the source after put must not reach the cache.
	src.signatures[0].Name = "MUTATED"
	src.types[0].Fields[0] = "MUTATED"
	src.imports["a.go"][0] = "MUTATED"

	got, ok := c.get("/dir", "fp")
	if !ok {
		t.Fatal("expected a cache hit")
	}
	if got.signatures[0].Name != "A" {
		t.Errorf("signature aliased the caller's slice: %q", got.signatures[0].Name)
	}
	if got.types[0].Fields[0] != "X int" {
		t.Errorf("nested Fields slice aliased the caller's memory: %q", got.types[0].Fields[0])
	}
	if got.imports["a.go"][0] != "context" {
		t.Errorf("imports map aliased the caller's slice: %q", got.imports["a.go"][0])
	}

	// Mutating what get returned must not reach the cache either.
	got.signatures[0].Name = "ALSO_MUTATED"
	again, _ := c.get("/dir", "fp")
	if again.signatures[0].Name != "A" {
		t.Errorf("get handed out shared memory: %q", again.signatures[0].Name)
	}
}

// TestPackageParseCache_EvictsLeastRecentlyUsed keeps retention bounded to the
// parse of maxCachedPackages directories rather than of the whole workspace.
func TestPackageParseCache_EvictsLeastRecentlyUsed(t *testing.T) {
	c := newPackageParseCache()
	mk := func() *packageParse {
		return &packageParse{imports: map[string][]string{}, pkgName: map[string]string{}}
	}
	for i := 0; i < maxCachedPackages; i++ {
		c.put(dirName(i), "fp", mk())
	}
	// Touch the oldest so it is no longer the eviction candidate.
	if _, ok := c.get(dirName(0), "fp"); !ok {
		t.Fatal("expected dir 0 to still be cached")
	}
	c.put(dirName(maxCachedPackages), "fp", mk())

	if _, ok := c.get(dirName(0), "fp"); !ok {
		t.Error("recently used entry was evicted")
	}
	if _, ok := c.get(dirName(1), "fp"); ok {
		t.Error("least recently used entry survived eviction")
	}
	if c.order.Len() > maxCachedPackages {
		t.Errorf("cache grew past its bound: %d", c.order.Len())
	}
}

func dirName(i int) string { return "/dir/" + string(rune('a'+i/26)) + string(rune('a'+i%26)) }

// TestPackageParseCache_StaleFingerprintDropsEntry proves a fingerprint miss
// removes the entry rather than leaving a wrong answer behind it.
func TestPackageParseCache_StaleFingerprintDropsEntry(t *testing.T) {
	c := newPackageParseCache()
	c.put("/dir", "old", &packageParse{imports: map[string][]string{}, pkgName: map[string]string{}})

	if _, ok := c.get("/dir", "new"); ok {
		t.Fatal("a changed fingerprint must miss")
	}
	if _, ok := c.entries["/dir"]; ok {
		t.Fatal("stale entry was kept after a fingerprint miss")
	}
}

// TestDirectoryFingerprint_IgnoresTestFiles matches what parsePackage parses:
// test files are excluded from signature extraction, so a change to one must
// not invalidate the package parse and pay for a full re-parse.
func TestDirectoryFingerprint_IgnoresTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempPkg(t, dir, map[string]string{"a.go": "package p\n\nfunc A() {}\n"})

	before, _, err := directoryFingerprint(dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	writeTempPkg(t, dir, map[string]string{"a_test.go": "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"})
	after, _, err := directoryFingerprint(dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if before != after {
		t.Error("adding a test file changed the package-parse fingerprint")
	}
}

// TestPackageParseCache_NilSafe covers the zero-value provider the tests build
// directly and the nil cache receiver.
func TestPackageParseCache_NilSafe(t *testing.T) {
	var c *packageParseCache
	if _, ok := c.get("/dir", "fp"); ok {
		t.Error("nil cache reported a hit")
	}
	c.put("/dir", "fp", &packageParse{})
	if h, m := c.stats(); h != 0 || m != 0 {
		t.Errorf("nil cache stats = %d/%d, want 0/0", h, m)
	}

	h := &HolographicProvider{}
	if hits, misses := h.CacheStats(); hits != 0 || misses != 0 {
		t.Errorf("fresh provider stats = %d/%d, want 0/0", hits, misses)
	}
	var nilProvider *HolographicProvider
	if hits, misses := nilProvider.CacheStats(); hits != 0 || misses != 0 {
		t.Errorf("nil provider stats = %d/%d, want 0/0", hits, misses)
	}
}

// BenchmarkHolographicPromptSection measures the per-turn cost the cache
// removes. Run with -benchtime=Nx against a large package to compare.
func BenchmarkHolographicPromptSection(b *testing.B) {
	dir := b.TempDir()
	for i := 0; i < 60; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i/26))+string(rune('a'+i%26))+".go")
		src := "package p\n\n// Doc.\nfunc F" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + "() error { return nil }\n"
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	target := filepath.Join(dir, "faa.go")
	h := NewHolographicProvider(nil, dir)
	ctx := context.Background()
	h.PromptSection(ctx, target) // warm

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.PromptSection(ctx, target)
	}
}
