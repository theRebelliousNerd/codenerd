//go:build windows

package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWindowsDirRevsAgreeWithStat locks the FileIdExtdDirectoryInfo layout.
// A shifted ChangeTime would let a same-size rewrite look unchanged.
func TestWindowsDirRevsAgreeWithStat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkDirRevs(t, dir)
	gens, ok := dirContentGens(dir)
	if !ok {
		t.Fatal("dirContentGens")
	}
	note := filepath.Join(dir, "note.txt")
	noteInfo, err := os.Stat(note)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := gens["note.txt"]
	if !ok {
		t.Fatal("note.txt missing from directory query")
	}
	gen, genOK, isClock := fileContentGen(note, noteInfo)
	if !genOK || !isClock || g.gen != gen || g.size != noteInfo.Size() || g.mtime != noteInfo.ModTime().UnixNano() {
		t.Fatalf("note.txt directory gen %d handle %d ok %v size %d/%d mtime %d/%d",
			g.gen, gen, genOK, g.size, noteInfo.Size(), g.mtime, noteInfo.ModTime().UnixNano())
	}
	if _, isDir := gens["sub.go"]; isDir {
		t.Fatal("directory sub.go included in dirContentGens")
	}
	// Package directory and a larger sibling. go test's working directory
	// is the package under test.
	checkDirRevs(t, ".")
	checkDirRevs(t, filepath.Join("..", "core"))
}

func checkDirRevs(t *testing.T, dir string) {
	t.Helper()
	revs, err := windowsDirRevs(dir)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		want = append(want, e.Name())
	}
	if len(revs) != len(want) {
		t.Fatalf("%s: %d revs, ReadDir has %d .go files", dir, len(revs), len(want))
	}
	for i, name := range want {
		if revs[i].name != name {
			t.Fatalf("%s: rev[%d]=%q, ReadDir %q", dir, i, revs[i].name, name)
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if revs[i].size != info.Size() || revs[i].mtime != info.ModTime().UnixNano() || !revs[i].genOK {
			t.Fatalf("%s %s: rev size %d mtime %d ok %v; stat size %d mtime %d",
				dir, name, revs[i].size, revs[i].mtime, revs[i].genOK, info.Size(), info.ModTime().UnixNano())
		}
		gen, ok, isClock := fileContentGen(path, info)
		if !ok || !isClock || revs[i].gen != gen {
			t.Fatalf("%s %s: directory gen %d, handle gen %d ok %v", dir, name, revs[i].gen, gen, ok)
		}
	}
	viaGo, err := goFileRevs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !revsEqual(revs, viaGo) {
		t.Fatalf("%s: goFileRevs disagreed with windowsDirRevs", dir)
	}
}
