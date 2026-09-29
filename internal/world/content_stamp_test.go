package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/tools"
)

func useStableContentStamp(t *testing.T) {
	t.Helper()
	prev := symbolStampQuantum
	symbolStampQuantum = -time.Hour
	t.Cleanup(func() { symbolStampQuantum = prev })
}

func TestContentStamp_AgeAndGeneration(t *testing.T) {
	prev := symbolStampQuantum
	symbolStampQuantum = 2 * time.Second
	t.Cleanup(func() { symbolStampQuantum = prev })

	now := time.Now().UnixNano()
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	stored := contentStamp{
		size: 4, mtime: now - int64(time.Hour), gen: now - int64(time.Hour),
		genOK: true, genIsClock: true, hash: hash,
	}
	cur := stored
	cur.hash = ""
	if !stored.trusts(cur, now) {
		t.Fatal("stable stamp was not trusted")
	}
	young := stored
	young.mtime = now
	young.gen = now
	curYoung := young
	curYoung.hash = ""
	if young.trusts(curYoung, now) {
		t.Fatal("timestamp inside the tick was trusted")
	}
	moved := cur
	moved.gen++
	if stored.trusts(moved, now) {
		t.Fatal("generation change was trusted")
	}
	got, ok := parseContentFingerprint(formatContentFingerprint(stored, hash))
	if !ok || got != stored {
		t.Fatalf("fingerprint round trip = %+v ok %v", got, ok)
	}
	if _, ok := parseContentFingerprint("42:100"); ok {
		t.Fatal("size:mtime fingerprint parsed as a content identity")
	}
}

func TestFileCache_Get_SameSizeRestoredMtime(t *testing.T) {
	useStableContentStamp(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	other := []byte("package p\n\nfunc Other() int { return 22 }\n")
	if len(body) != len(other) {
		t.Fatalf("fixture length %d != %d", len(body), len(other))
	}
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))
	sum, err := calculateHash(path)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewFileCache(root)
	cache.Update(path, info, sum)
	if !cache.Entries[path].GenOK {
		t.Fatal("Update stored no content generation")
	}
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, other, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("metadata moved: size %d->%d mtime %s->%s", info.Size(), after.Size(), info.ModTime(), after.ModTime())
	}
	loaded := NewFileCache(root)
	if _, ok := loaded.Get(path, after); ok {
		t.Fatal("Get returned the stored hash after a same-size rewrite with a restored mtime")
	}
}

func TestFileCache_Get_IdenticalTouch(t *testing.T) {
	useStableContentStamp(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))
	sum, err := calculateHash(path)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewFileCache(root)
	cache.Update(path, info, sum)
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}

	touched := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, touched, touched); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.ModTime().Equal(info.ModTime()) {
		t.Fatal("mtime did not move")
	}
	got, ok := NewFileCache(root).Get(path, after)
	if !ok || got != sum {
		t.Fatalf("Get after an identical-content touch = %q %v, want the stored hash", got, ok)
	}
}

func TestFileCache_Get_LegacyStampFollowsContent(t *testing.T) {
	useStableContentStamp(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	other := []byte("package p\n\nfunc Other() int { return 22 }\n")
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))
	sum, err := calculateHash(path)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewFileCache(root)
	cache.Entries[path] = CacheEntry{Hash: sum, ModTime: info.ModTime().UnixNano(), Size: info.Size()}
	cache.Dirty = true
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, other, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded := NewFileCache(root)
	if loaded.Entries[path].GenOK {
		t.Fatal("reloading a legacy entry set GenOK")
	}
	if _, ok := loaded.Get(path, after); ok {
		t.Fatal("legacy size+mtime entry returned its hash after the bytes changed")
	}

	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	moved := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, moved, moved); err != nil {
		t.Fatal(err)
	}
	touched, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	fresh := NewFileCache(root)
	got, ok := fresh.Get(path, touched)
	if !ok || got != sum {
		t.Fatalf("legacy entry after an identical touch = %q %v, want the stored hash", got, ok)
	}
}

func TestIncrementalScan_SameSizeRewriteRestoredMtime(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "sib.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	other := []byte("package p\n\nfunc Other() int { return 22 }\n")
	if len(body) != len(other) {
		t.Fatalf("fixture length %d != %d", len(body), len(other))
	}
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))

	db := newScanDB(t)
	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	quiet, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !quiet.Unchanged {
		t.Fatalf("quiet scan was a change: changed %v new %v deleted %v", quiet.ChangedFiles, quiet.NewFiles, quiet.DeletedFiles)
	}
	key, entry := cacheEntryNamed(t, root, "sib.go")
	requireTrusted(t, key, entry)

	if err := os.WriteFile(path, other, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("metadata moved: size %d->%d mtime %s->%s", info.Size(), after.Size(), info.ModTime(), after.ModTime())
	}

	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if delta.Unchanged || delta.Full {
		t.Fatalf("rewrite scan unchanged=%v full=%v", delta.Unchanged, delta.Full)
	}
	if !pathListed(delta.ChangedFiles, path) {
		t.Fatalf("changed files = %v, want %s", delta.ChangedFiles, path)
	}
	sum, err := calculateHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if !factArgsContain(delta.NewFacts, "file_topology", sum) {
		t.Fatal("file_topology did not take the new hash")
	}
	if !factArgsContain(delta.NewFacts, "symbol_graph", "func:Other") {
		t.Fatal("symbol_graph did not gain func:Other")
	}
	if factArgsContain(delta.NewFacts, "symbol_graph", "func:Target") {
		t.Fatal("symbol_graph still defines func:Target")
	}
	stored := loadFastFacts(t, db, root, path)
	if !factArgsContain(stored, "symbol_graph", "func:Other") || factArgsContain(stored, "symbol_graph", "func:Target") {
		t.Fatal("stored fast rows did not follow the rewrite")
	}
}

func TestIncrementalScan_TouchIdenticalContentIsNotAChange(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "sib.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))

	db := newScanDB(t)
	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	key, entry := cacheEntryNamed(t, root, "sib.go")
	requireTrusted(t, key, entry)

	touched := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, touched, touched); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.ModTime().Equal(info.ModTime()) {
		t.Fatal("mtime did not move")
	}
	res, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Unchanged {
		t.Fatalf("identical touch was a change: changed %v new %v deleted %v", res.ChangedFiles, res.NewFiles, res.DeletedFiles)
	}
	stored := loadFastFacts(t, db, root, path)
	if !factArgsContain(stored, "symbol_graph", "func:Target") {
		t.Fatal("stored rows lost func:Target after an identical touch")
	}
}

func TestIncrementalScan_SameSizeDeclarationRewritesSiblingRows(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	funcSrc := "package p\n\nfunc T(x int) int { return x }\n"
	typeSrc := "package p\n\ntype T int\n"
	if len(typeSrc) < len(funcSrc) {
		gap := len(funcSrc) - len(typeSrc)
		typeSrc = typeSrc[:len(typeSrc)-1] + " //" + strings.Repeat(".", gap-3) + "\n"
	}
	if len(typeSrc) != len(funcSrc) {
		t.Fatalf("declaration fixtures len %d and %d", len(funcSrc), len(typeSrc))
	}
	past := time.Unix(1_700_000_000, 0)
	aPath := filepath.Join(root, "a.go")
	bPath := filepath.Join(root, "b.go")
	writeAt(t, aPath, []byte(funcSrc), past)
	writeAt(t, bPath, []byte("package p\n\nfunc Use(x int) int { return T(x) }\n"), past)

	db := newScanDB(t)
	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureDeepFactsInRoot(ctx, root, []string{aPath, bPath}, db, 1); err != nil {
		t.Fatal(err)
	}
	if !loadDeepCalls(t, db, root, "b.go")[[2]string{"fn:p.Use", "fn:p.T"}] {
		t.Fatal("fixture: b.go did not record the fn: call")
	}
	key, entry := cacheEntryNamed(t, root, "a.go")
	requireTrusted(t, key, entry)

	before, err := os.Stat(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, []byte(typeSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(aPath, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("metadata moved: size %d->%d mtime %s->%s", before.Size(), after.Size(), before.ModTime(), after.ModTime())
	}

	delta := rescan(t, scanner, root, db)
	if !pathListed(delta.ChangedFiles, aPath) {
		t.Fatalf("changed files = %v, want %s", delta.ChangedFiles, aPath)
	}
	if pathListed(delta.ChangedFiles, bPath) {
		t.Fatal("b.go's bytes did not change; it was listed as a changed file")
	}
	if !hasFact(delta.RetractFacts, "code_calls", "fn:p.Use", "fn:p.T") {
		t.Fatal("b.go kept code_calls(fn:p.Use, fn:p.T) after a.go declared type T")
	}
	if !hasFact(delta.NewFacts, "code_calls", "p.Use", "p.T") {
		t.Fatal("refreshed b.go dropped the bare code_calls(p.Use, p.T) row")
	}
	got := loadDeepCalls(t, db, root, "b.go")
	if got[[2]string{"fn:p.Use", "fn:p.T"}] || !got[[2]string{"p.Use", "p.T"}] {
		t.Fatalf("stored b.go calls = %v", callList(got))
	}
}

func TestDeepFacts_SameSizeRestoredMtime(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "sib.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	other := []byte("package p\n\nfunc Other() int { return 22 }\n")
	if len(body) != len(other) {
		t.Fatalf("fixture length %d != %d", len(body), len(other))
	}
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))
	db := newScanDB(t)
	ctx := context.Background()

	first, err := EnsureDeepFactsInRoot(ctx, root, []string{path}, db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.FilesParsed != 1 {
		t.Fatalf("first parse count = %d", first.FilesParsed)
	}
	second, err := EnsureDeepFactsInRoot(ctx, root, []string{path}, db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if second.FilesParsed != 0 {
		t.Fatalf("unchanged file parsed %d times", second.FilesParsed)
	}
	requireTrustedDeep(t, db, root, path)

	if err := os.WriteFile(path, other, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("metadata moved: size %d->%d mtime %s->%s", info.Size(), after.Size(), info.ModTime(), after.ModTime())
	}
	third, err := EnsureDeepFactsInRoot(ctx, root, []string{path}, db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if third.FilesParsed != 1 {
		t.Fatalf("same-size rewrite parsed %d files, want 1", third.FilesParsed)
	}
	if !factArgsContain(third.NewFacts, "", "Other") || factArgsContain(third.NewFacts, "", "Target") {
		t.Fatal("deep facts did not follow the rewrite")
	}
}

func TestDeepFacts_IdenticalTouchReuses(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "sib.go")
	body := []byte("package p\n\nfunc Target() int { return 1 }\n")
	info := writeAt(t, path, body, time.Unix(1_700_000_000, 0))
	db := newScanDB(t)
	ctx := context.Background()
	if _, err := EnsureDeepFactsInRoot(ctx, root, []string{path}, db, 1); err != nil {
		t.Fatal(err)
	}
	requireTrustedDeep(t, db, root, path)

	touched := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, touched, touched); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.ModTime().Equal(info.ModTime()) {
		t.Fatal("mtime did not move")
	}
	res, err := EnsureDeepFactsInRoot(ctx, root, []string{path}, db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesParsed != 0 {
		t.Fatalf("identical touch parsed %d files", res.FilesParsed)
	}
	if !factArgsContain(res.NewFacts, "", "Target") {
		t.Fatal("reused deep facts lost Target")
	}
}

func writeAt(t *testing.T, path string, body []byte, when time.Time) os.FileInfo {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func canonicalTempRoot(t *testing.T) string {
	t.Helper()
	root, err := tools.CanonicalWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func newScanDB(t *testing.T) *store.LocalStore {
	t.Helper()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func cacheEntryNamed(t *testing.T, root, base string) (string, CacheEntry) {
	t.Helper()
	cache := NewFileCache(root)
	for key, entry := range cache.Entries {
		if filepath.Base(key) == base {
			return key, entry
		}
	}
	t.Fatalf("no cache entry named %s (%d entries)", base, len(cache.Entries))
	return "", CacheEntry{}
}

func requireTrusted(t *testing.T, path string, entry CacheEntry) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	cur := stampFromInfo(path, info, nil)
	if !stampFromEntry(entry).trusts(cur, time.Now().UnixNano()) {
		t.Fatalf("stamp for %s is not trusted (stored genOK=%v gen=%d mtime=%d; cur genOK=%v gen=%d mtime=%d)",
			path, entry.GenOK, entry.Gen, entry.ModTime, cur.genOK, cur.gen, cur.mtime)
	}
}

func requireTrustedDeep(t *testing.T, db *store.LocalStore, root, abs string) {
	t.Helper()
	rel := canonicalScanPath(root, abs)
	_, fp, err := db.LoadWorldFactsForFile(rel, "deep")
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := parseContentFingerprint(fp)
	if !ok {
		t.Fatalf("deep fingerprint %q is not content-addressed", fp)
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	cur := stampFromInfo(abs, info, nil)
	if !stored.trusts(cur, time.Now().UnixNano()) {
		t.Fatalf("deep stamp for %s is not trusted (fp %s; cur genOK=%v gen=%d mtime=%d)", abs, fp, cur.genOK, cur.gen, cur.mtime)
	}
}

func pathListed(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func factArgsContain(facts []core.Fact, pred, needle string) bool {
	for _, f := range facts {
		if pred != "" && f.Predicate != pred {
			continue
		}
		for _, arg := range f.Args {
			if s, ok := arg.(string); ok && s == needle {
				return true
			}
			if s, ok := arg.(string); ok && pred == "" && strings.Contains(s, needle) {
				return true
			}
		}
	}
	return false
}

func loadFastFacts(t *testing.T, db *store.LocalStore, root, abs string) []core.Fact {
	t.Helper()
	rel := canonicalScanPath(root, abs)
	inputs, _, err := db.LoadWorldFactsForFile(rel, "fast")
	if err != nil {
		t.Fatal(err)
	}
	facts := make([]core.Fact, len(inputs))
	for i, in := range inputs {
		facts[i] = core.Fact{Predicate: in.Predicate, Args: in.Args}
	}
	return facts
}
