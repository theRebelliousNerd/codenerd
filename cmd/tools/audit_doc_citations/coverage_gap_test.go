package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/docdecl"
)

func TestFindModuleRootEmptyStartReportsNoGoMod(t *testing.T) {
	_, err := findModuleRoot("")
	if err == nil {
		t.Fatalf("findModuleRoot(\"\") = nil error, want no go.mod error")
	}
	if !strings.Contains(err.Error(), "no go.mod") {
		t.Fatalf("findModuleRoot(\"\") error = %q, want it to mention no go.mod", err.Error())
	}
}

func TestFindModuleRootStatErrorBranch(t *testing.T) {
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	// blocker is a regular file, so anything beneath it cannot be statted
	// normally: Stat on blocker/subdir/go.mod fails with ENOTDIR, which
	// os.IsNotExist reports as false, exercising the stat-error branch.
	start := filepath.Join(blocker, "subdir")
	_, err := findModuleRoot(start)
	if err == nil {
		t.Fatalf("findModuleRoot(%q) = nil error, want stat error", start)
	}
	if !strings.Contains(err.Error(), "stat ") {
		t.Fatalf("findModuleRoot(%q) error = %q, want stat prefix", start, err.Error())
	}
}

func TestLoadPkgInitializesNilPkgsMap(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte("package p\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatalf("write p.go: %v", err)
	}
	c := &checker{root: root}
	if c.pkgs != nil {
		t.Fatalf("precondition: pkgs map should be nil, got %#v", c.pkgs)
	}
	p, err := c.loadPkg("internal/p")
	if err != nil {
		t.Fatalf("loadPkg: %v", err)
	}
	if p == nil {
		t.Fatalf("loadPkg returned nil package")
	}
	if c.pkgs == nil {
		t.Fatalf("loadPkg did not initialize pkgs map")
	}
	cached, ok := c.pkgs["internal/p"]
	if !ok {
		t.Fatalf("pkgs map missing entry for internal/p after loadPkg")
	}
	if cached != p {
		t.Fatalf("pkgs map entry does not match returned package")
	}
	p2, err := c.loadPkg("internal/p")
	if err != nil {
		t.Fatalf("second loadPkg: %v", err)
	}
	if p2 != p {
		t.Fatalf("second loadPkg did not return cached package")
	}
}

func TestReportAddCoversNilAndNonNilByKind(t *testing.T) {
	var nilMap Report
	nilMap.add(Finding{Doc: "d.md", Line: 1, Citation: "internal/p", Kind: kindMissingPath, Reason: "r"})
	if nilMap.ByKind == nil {
		t.Fatalf("add with nil ByKind left map nil")
	}
	if nilMap.ByKind[kindMissingPath] != 1 || nilMap.Broken != 1 {
		t.Fatalf("add with nil ByKind = %+v, want 1 missing-path and Broken=1", nilMap.ByKind)
	}
	seeded := newReport()
	seeded.add(Finding{Doc: "d.md", Line: 2, Citation: "internal/p", Kind: kindMissingPath, Reason: "r"})
	if seeded.ByKind[kindMissingPath] != 1 || seeded.Broken != 1 {
		t.Fatalf("add with initialized ByKind = %+v Broken=%d, want 1/1", seeded.ByKind, seeded.Broken)
	}
	if len(seeded.ByKind) != 4 {
		t.Fatalf("add with initialized ByKind dropped keys: %#v", seeded.ByKind)
	}
}

func TestFindModuleRootSuccessTakesErrNilBranch(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/tmp\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	sub := filepath.Join(tmp, "sub", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	got, err := findModuleRoot(sub)
	if err != nil {
		t.Fatalf("findModuleRoot(sub) error = %v, want nil", err)
	}
	if got != tmp {
		t.Fatalf("findModuleRoot(sub) = %q, want %q", got, tmp)
	}
}

func TestFindModuleRootMissingGoModTakesIsNotExistBranch(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	_, err := findModuleRoot(sub)
	if err == nil {
		t.Fatalf("findModuleRoot in empty tree = nil error, want no go.mod error")
	}
	if !strings.Contains(err.Error(), "no go.mod") {
		t.Fatalf("findModuleRoot in empty tree error = %q, want no go.mod", err.Error())
	}
	if strings.Contains(err.Error(), "stat ") {
		t.Fatalf("findModuleRoot in empty tree error = %q, must not take stat-error branch", err.Error())
	}
}

func TestLoadPkgSkipsDirsNonGoAndTestFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "p")
	if err := os.MkdirAll(filepath.Join(dir, "subpkg"), 0o755); err != nil {
		t.Fatalf("mkdir subpkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte("package p\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatalf("write p.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("package p\nfunc FromTxt() {}\n"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("package p\nfunc OnlyInTest() {}\n"), 0o644); err != nil {
		t.Fatalf("write p_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subpkg", "q.go"), []byte("package subpkg\nfunc OnlyInSubdir() {}\n"), 0o644); err != nil {
		t.Fatalf("write subpkg/q.go: %v", err)
	}
	c := &checker{root: root, pkgs: map[string]*docdecl.Package{}}
	_ = c
	p, err := (&checker{root: root}).loadPkg("internal/p")
	if err != nil {
		t.Fatalf("loadPkg: %v", err)
	}
	if !p.Has("Hello") && !p.Has("p.Hello") {
		t.Fatalf("loadPkg skipped real p.go: Hello not found")
	}
	if p.Has("OnlyInTest") || p.Has("p.OnlyInTest") {
		t.Fatalf("loadPkg graded _test.go symbol OnlyInTest")
	}
	if p.Has("FromTxt") || p.Has("p.FromTxt") {
		t.Fatalf("loadPkg graded non-Go file symbol FromTxt")
	}
	if p.Has("OnlyInSubdir") || p.Has("p.OnlyInSubdir") || p.Has("subpkg.OnlyInSubdir") {
		t.Fatalf("loadPkg descended into subdirectory")
	}
}

func TestLoadPkgWithInitializedMapSkipsNilInit(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		d := filepath.Join(root, "internal", name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
		if err := os.WriteFile(filepath.Join(d, name+".go"), []byte("package "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s.go: %v", name, err)
		}
	}
	c := &checker{root: root, pkgs: map[string]*docdecl.Package{}}
	before := c.pkgs
	pa, err := c.loadPkg("internal/a")
	if err != nil {
		t.Fatalf("loadPkg a: %v", err)
	}
	if pa == nil {
		t.Fatalf("loadPkg a returned nil")
	}
	pb, err := c.loadPkg("internal/b")
	if err != nil {
		t.Fatalf("loadPkg b: %v", err)
	}
	if pb == nil {
		t.Fatalf("loadPkg b returned nil")
	}
	if len(c.pkgs) != 2 {
		t.Fatalf("pkgs has %d entries, want 2", len(c.pkgs))
	}
	_ = before
}

func TestPredicateName_PinBranches(t *testing.T) {
	if got := predicateName("ghost/1"); got != "ghost" {
		t.Errorf("predicateName(ghost/1) = %q, want ghost", got)
	}
	if got := predicateName("pkg.Func"); got != "Func" {
		t.Errorf("predicateName(pkg.Func) = %q, want Func", got)
	}
	if got := predicateName("a.b.c"); got != "c" {
		t.Errorf("predicateName(a.b.c) = %q, want c", got)
	}
	if got := predicateName("plain"); got != "plain" {
		t.Errorf("predicateName(plain) = %q, want plain", got)
	}
	if got := predicateName("trailing."); got != "trailing." {
		t.Errorf("predicateName(trailing.) = %q, want trailing.", got)
	}
}

func TestAppendUnique_PinDedup(t *testing.T) {
	if got := appendUnique(nil, "a"); len(got) != 1 || got[0] != "a" {
		t.Fatalf("appendUnique(nil,a) = %v, want [a]", got)
	}
	if got := appendUnique([]string{"a"}, "a"); len(got) != 1 {
		t.Errorf("appendUnique dedup = %v, want [a]", got)
	}
	if got := appendUnique([]string{"a"}, "b"); len(got) != 2 || got[1] != "b" {
		t.Errorf("appendUnique append = %v, want [a b]", got)
	}
}

func TestSkipStdlibDir_PinShapes(t *testing.T) {
	for _, n := range []string{"internal", "vendor", "testdata", "cmd", ".git", ".hidden"} {
		if !skipStdlibDir(n) {
			t.Errorf("skipStdlibDir(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"pkg", "sync", "filepath"} {
		if skipStdlibDir(n) {
			t.Errorf("skipStdlibDir(%q) = true, want false", n)
		}
	}
}

func TestLineReason_PinExtraBranches(t *testing.T) {
	if got := lineReason([][2]int{{0, 5}}, 8); !strings.Contains(got, "outside the file") {
		t.Errorf("zero-start range: %q", got)
	}
	if got := lineReason([][2]int{{9, 10}}, 8); !strings.Contains(got, "line 9 is past end of file") || !strings.Contains(got, "line 10 is past end of file") {
		t.Errorf("both-past-end range should name both lines, got %q", got)
	}
	if got := lineReason(nil, 8); got != "" {
		t.Errorf("empty spans should be empty, got %q", got)
	}
}

func TestForeignStdlib_PinNilAndNonSplit(t *testing.T) {
	c := &checker{}
	if c.foreignStdlib(nil, "pkg.Func") {
		t.Errorf("nil package should never be stdlib")
	}
	pk := docdecl.NewPackage("internal/pkg")
	if !strings.Contains(pk.DirBase, "pkg") {
		t.Fatalf("fixture DirBase = %q, want pkg", pk.DirBase)
	}
	if c.foreignStdlib(pk, "noseparator") {
		t.Errorf("unsplittable symbol should not be stdlib")
	}
	if c.foreignStdlib(pk, "pkg.Anything") {
		t.Errorf("qualifier matching the cited package dir should stay a repo citation")
	}
	pk.PkgNames["other"] = true
	if c.foreignStdlib(pk, "other.Func") {
		t.Errorf("qualifier declared by the cited package should stay a repo citation")
	}
}

func TestStdlibHas_PinEmptyAndMissing(t *testing.T) {
	c := &checker{}
	if c.stdlibHas("", "Map") {
		t.Errorf("empty pkg should be false")
	}
	if c.stdlibHas("sync", "") {
		t.Errorf("empty rest should be false")
	}
	if c.stdlibHas("no-such-pkg-xyz", "Foo") {
		t.Errorf("unknown qualifier should not be stdlib")
	}
	if !c.stdlibHas("sync", "Map") {
		t.Fatalf("sync.Map should be stdlib")
	}
	if c.stdCache["sync"] == nil {
		t.Fatalf("second lookup should hit the cache for sync")
	}
	if !c.stdlibHas("sync", "Map") {
		t.Errorf("cached sync.Map should still be stdlib")
	}
}

func TestLoadPkg_PinReadDirError(t *testing.T) {
	root := t.TempDir()
	c := &checker{root: root, pkgs: map[string]*docdecl.Package{}}
	if _, err := c.loadPkg("internal/does-not-exist"); err == nil {
		t.Fatalf("loadPkg on missing dir should fail")
	} else if !strings.Contains(err.Error(), "read package") {
		t.Fatalf("loadPkg error = %q, want read package", err.Error())
	}
}

func TestLineCount_PinCacheAndMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "a\nb\nc\n"
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &checker{root: root, lines: map[string]int{}}
	want := len(docdecl.SplitLines(content))
	got, err := c.lineCount("internal/p/f.go")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("lineCount = %d, want %d", got, want)
	}
	got2, err := c.lineCount("internal/p/f.go")
	if err != nil {
		t.Fatal(err)
	}
	if got2 != want {
		t.Fatalf("cached lineCount = %d, want %d", got2, want)
	}
	if _, err := c.lineCount("internal/missing/f.go"); err == nil {
		t.Errorf("lineCount on missing file should fail")
	}
}

func TestJudgePath_PinEscapeAndNotAFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &checker{root: root, preds: map[string]bool{}, dirs: map[string][]string{}, pkgs: map[string]*docdecl.Package{}, lines: map[string]int{}}
	cites, findings, err := c.judgePath("docs/d.md", 1, docdecl.PathHit{Rel: "../outside.go", Raw: "../outside.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cites != 1 || len(findings) != 1 || findings[0].Kind != kindMissingPath {
		t.Fatalf("escape = cites %d findings %+v, want one missing-path", cites, findings)
	}
	if !strings.Contains(findings[0].Reason, "escapes") {
		t.Errorf("escape reason = %q, want escapes", findings[0].Reason)
	}
	cites, findings, err = c.judgePath("docs/d.md", 2, docdecl.PathHit{Rel: "internal/pkg", Raw: "internal/pkg"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cites != 1 || len(findings) != 1 || findings[0].Kind != kindMissingPath {
		t.Fatalf("directory citation = cites %d findings %+v, want one missing-path", cites, findings)
	}
	if !strings.Contains(findings[0].Reason, "not a file") {
		t.Errorf("directory reason = %q, want not a file", findings[0].Reason)
	}
}

func TestIndexPackageDirs_PinMissingTopsAndFindsPkg(t *testing.T) {
	empty := t.TempDir()
	dirs, err := indexPackageDirs(empty)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 0 {
		t.Fatalf("empty root dirs = %v, want empty", dirs)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "internal"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, err = indexPackageDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 0 {
		t.Fatalf("file top dirs = %v, want empty", dirs)
	}
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, "internal", "mypkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root2, "internal", "mypkg", "x.go"), []byte("package mypkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, err = indexPackageDirs(root2)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range dirs["mypkg"] {
		if d == "internal/mypkg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("mypkg dirs = %v, want internal/mypkg", dirs)
	}
}

func TestParsePackageDir_PinEmptyAndMissing(t *testing.T) {
	if _, err := parsePackageDir(filepath.Join(t.TempDir(), "no-such-dir")); err == nil {
		t.Errorf("parsePackageDir on missing dir should fail")
	}
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := parsePackageDir(empty)
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("parsePackageDir with no Go files = %+v, want nil", p)
	}
}

func TestLoadStdlib_PinMissing(t *testing.T) {
	if p := loadStdlib("no-such-pkg-xyz"); p != nil {
		t.Errorf("loadStdlib(unknown) = %+v, want nil", p)
	}
}
