package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

func TestPersistFastSnapshotToDB_PreservesGlobalFacts(t *testing.T) {
	dbPath := t.TempDir() + "/world.db"
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	defer db.Close()

	filePath := t.TempDir() + "/main.go"
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	facts := []core.Fact{
		{
			Predicate: "file_topology",
			Args: []any{
				filePath,
				"hash123",
				core.MangleAtom("/go"),
				int64(10),
				core.MangleAtom("/file"),
			},
		},
		{
			Predicate: "project_language",
			Args:      []any{core.MangleAtom("/go")},
		},
		{
			Predicate: "entry_point",
			Args:      []any{"main.main"},
		},
	}

	if err := PersistFastSnapshotToDB(db, facts); err != nil {
		t.Fatalf("PersistFastSnapshotToDB failed: %v", err)
	}

	loaded, err := db.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatalf("LoadAllWorldFacts failed: %v", err)
	}

	seen := map[string]bool{}
	for _, fact := range loaded {
		seen[fact.Predicate] = true
	}

	if !seen["file_topology"] {
		t.Fatal("expected file_topology fact in cached snapshot")
	}
	if !seen["project_language"] {
		t.Fatal("expected project_language fact in cached snapshot")
	}
	if !seen["entry_point"] {
		t.Fatal("expected entry_point fact in cached snapshot")
	}
}

func TestContentHashAccepted(t *testing.T) {
	sum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if !contentHashAccepted(sum) {
		t.Fatal("lowercase SHA-256 digest was rejected")
	}
	if contentHashAccepted("hash123") {
		t.Fatal("short token was accepted")
	}
	if contentHashAccepted(sum[:63] + "g") {
		t.Fatal("non-hex digest was accepted")
	}
	upper := "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
	if contentHashAccepted(upper) {
		t.Fatal("uppercase digest was accepted")
	}
}

func TestPersistFastSnapshot_WritesContentFingerprint(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	rel := "main.go"
	path := filepath.Join(root, rel)
	info := writeAt(t, path, []byte("package main\n\nfunc main() {}\n"), time.Unix(1_700_000_000, 0))
	sum, err := calculateHash(path)
	if err != nil {
		t.Fatal(err)
	}
	db := newScanDB(t)
	facts := []core.Fact{{
		Predicate: "file_topology",
		Args: []any{
			rel,
			sum,
			core.MangleAtom("/go"),
			info.ModTime().UnixNano(),
			core.MangleAtom("/false"),
		},
	}}
	if err := PersistFastSnapshotToDBInRoot(db, root, facts); err != nil {
		t.Fatal(err)
	}

	_, fp, err := db.LoadWorldFactsForFile(rel, "fast")
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := parseContentFingerprint(fp)
	if !ok {
		t.Fatalf("fast fingerprint %q is not a content identity", fp)
	}
	if stored.hash != sum {
		t.Fatalf("fingerprint hash = %s, want %s", stored.hash, sum)
	}
	if stored.size != info.Size() || stored.mtime != info.ModTime().UnixNano() {
		t.Fatalf("fingerprint stamp size %d mtime %d, file size %d mtime %d", stored.size, stored.mtime, info.Size(), info.ModTime().UnixNano())
	}
	cur := stampFromInfo(path, info, nil)
	if !stored.trusts(cur, time.Now().UnixNano()) {
		t.Fatalf("fresh content fingerprint does not trust the file (fp %s)", fp)
	}
}

func TestPersistFastSnapshot_NonDigestHashIsNotContentIdentity(t *testing.T) {
	root := canonicalTempRoot(t)
	rel := "main.go"
	path := filepath.Join(root, rel)
	info := writeAt(t, path, []byte("package main\n\nfunc main() {}\n"), time.Unix(1_700_000_000, 0))
	db := newScanDB(t)
	facts := []core.Fact{{
		Predicate: "file_topology",
		Args: []any{
			rel,
			"hash123",
			core.MangleAtom("/go"),
			info.ModTime().UnixNano(),
			core.MangleAtom("/false"),
		},
	}}
	if err := PersistFastSnapshotToDBInRoot(db, root, facts); err != nil {
		t.Fatal(err)
	}
	_, fp, err := db.LoadWorldFactsForFile(rel, "fast")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parseContentFingerprint(fp); ok {
		t.Fatalf("non-digest hash stored as content fingerprint %q", fp)
	}
	legacy := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	if fp == legacy {
		t.Fatalf("stored the size:mtime key %q", fp)
	}
	loaded, err := db.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) == 0 {
		t.Fatal("facts were dropped with the unusable hash")
	}
}

func TestPersistFastSnapshot_SameSizeRewriteIsNotTrusted(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	rel := "sib.go"
	path := filepath.Join(root, rel)
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
	db := newScanDB(t)
	if err := PersistFastSnapshotToDBInRoot(db, root, []core.Fact{{
		Predicate: "file_topology",
		Args:      []any{rel, sum, core.MangleAtom("/go"), info.ModTime().UnixNano(), core.MangleAtom("/false")},
	}}); err != nil {
		t.Fatal(err)
	}
	_, fp, err := db.LoadWorldFactsForFile(rel, "fast")
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := parseContentFingerprint(fp)
	if !ok {
		t.Fatalf("fingerprint %q", fp)
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
	cur := stampFromInfo(path, after, nil)
	if stored.trusts(cur, time.Now().UnixNano()) {
		t.Fatal("content fingerprint trusted a same-size rewrite with a restored mtime")
	}
	hash, changed, err := resolveContent(path, cur, stored, time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	if !changed || hash == sum || hash == "" {
		t.Fatalf("rewrite resolve = hash %q changed %v, want a new hash and a reparse", hash, changed)
	}
}

func TestOldFormatFastRowIsReparsedNotTrusted(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "sib.go")
	live := []byte("package p\n\nfunc Target() int { return 1 }\n")
	info := writeAt(t, path, live, time.Unix(1_700_000_000, 0))
	canonical := canonicalScanPath(root, path)
	// The key PersistFastSnapshot used to write: size and mtime. It still
	// matches this file, and the stored symbol does not.
	legacy := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	if _, ok := parseContentFingerprint(legacy); ok {
		t.Fatalf("legacy key %q parsed as a content identity", legacy)
	}
	cur := stampFromInfo(path, info, nil)
	stored, _ := parseContentFingerprint(legacy)
	hash, changed, err := resolveContent(path, cur, stored, time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	if !changed || hash == "" {
		t.Fatalf("legacy key resolve = hash %q changed %v, want a reparse", hash, changed)
	}

	db := newScanDB(t)
	if err := db.UpsertWorldFile(store.WorldFileMeta{
		Path:        canonical,
		Lang:        "go",
		Size:        info.Size(),
		ModTime:     info.ModTime().UnixNano(),
		Hash:        "stale",
		Fingerprint: legacy,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceWorldFactsForFile(canonical, "fast", legacy, []store.WorldFactInput{
		{Predicate: "file_topology", Args: []any{canonical, "stale", "/go", info.ModTime().UnixNano(), "/false"}},
		{Predicate: "symbol_graph", Args: []any{"func:Stale", "/function", "/public", canonical, "func Stale()"}},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := NewScanner().ScanWorkspaceIncremental(context.Background(), root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Unchanged {
		t.Fatal("old-format row was trusted; the scan skipped the reparse")
	}
	facts := loadFastFacts(t, db, root, path)
	if factArgsContain(facts, "symbol_graph", "func:Stale") {
		t.Fatal("stale symbol survived the reparse")
	}
	if !factArgsContain(facts, "symbol_graph", "func:Target") {
		t.Fatal("reparse did not store func:Target")
	}
	_, fp, err := db.LoadWorldFactsForFile(canonical, "fast")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := parseContentFingerprint(fp)
	if !ok || got.hash != hash {
		t.Fatalf("reparse stored fingerprint %q, want content hash %s", fp, hash)
	}
}

func TestIncrementalScan_UnchangedAfterPersist(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "sib.go")
	writeAt(t, path, []byte("package p\n\nfunc Target() int { return 1 }\n"), time.Unix(1_700_000_000, 0))
	db := newScanDB(t)
	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	canonical := canonicalScanPath(root, path)
	_, fp, err := db.LoadWorldFactsForFile(canonical, "fast")
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := parseContentFingerprint(fp)
	if !ok {
		t.Fatalf("persisted fast fingerprint %q is not a content identity", fp)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.trusts(stampFromInfo(path, info, nil), time.Now().UnixNano()) {
		t.Fatalf("persisted fingerprint does not trust the file (%s)", fp)
	}

	quiet, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !quiet.Unchanged {
		t.Fatalf("quiet scan after persist was a change: changed %v new %v deleted %v full %v", quiet.ChangedFiles, quiet.NewFiles, quiet.DeletedFiles, quiet.Full)
	}
	_, fp2, err := db.LoadWorldFactsForFile(canonical, "fast")
	if err != nil {
		t.Fatal(err)
	}
	if fp2 != fp {
		t.Fatalf("quiet scan rewrote fingerprint\n%s\n%s", fp, fp2)
	}
	if !factArgsContain(loadFastFacts(t, db, root, path), "symbol_graph", "func:Target") {
		t.Fatal("quiet scan dropped func:Target")
	}
}
