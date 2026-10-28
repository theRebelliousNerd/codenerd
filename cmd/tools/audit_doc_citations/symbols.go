package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/docdecl"
)

func (c *checker) loadPkg(dir string) (*docdecl.Package, error) {
	if p, ok := c.pkgs[dir]; ok {
		return p, nil
	}
	abs := absJoin(c.root, dir)
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read package %s: %w", dir, err)
	}
	p := docdecl.NewPackage(dir)
	fset := token.NewFileSet()
	for _, ent := range ents {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".go") {
			continue
		}
		p.AddFile(fset, filepath.Join(abs, ent.Name()), dir+"/"+ent.Name(), nil)
	}
	c.pkgs[dir] = p
	return p, nil
}

func predicateName(sym string) string {
	if i := strings.IndexByte(sym, '/'); i > 0 {
		return sym[:i]
	}
	if i := strings.LastIndex(sym, "."); i >= 0 && i < len(sym)-1 {
		return sym[i+1:]
	}
	return sym
}

// indexPackageDirs maps a directory base name (the `pkg` in `pkg.Func`) to
// the slash-relative directories under internal/ and cmd/ that contain Go
// files. Package clause `main` is recorded later, when the directory is
// parsed; the index itself is the directory name docs use.
func indexPackageDirs(root string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, top := range []string{"internal", "cmd"} {
		base := filepath.Clean(filepath.Join(root, top))
		st, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !st.IsDir() {
			continue
		}
		err = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if filepath.Clean(path) != base && docdecl.SkipCorpusDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			out[slashBase(rel)] = appendUnique(out[slashBase(rel)], rel)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, dirs := range out {
		sort.Strings(dirs)
	}
	return out, nil
}

func appendUnique(ss []string, s string) []string {
	for _, e := range ss {
		if e == s {
			return ss
		}
	}
	return append(ss, s)
}

// stdlibHas reports whether pkg.rest is declared in the standard library.
// rest may be a selector chain (`TokenCounter.charsPerToken`); only an exact
// declaration matches, so a repo type that the library does not have is still
// graded against the repo package.
//
// The lookup is the directory base name under GOROOT/src, which is the
// qualifier docs write: `filepath.Join` lives in path/filepath, `json.Marshal`
// in encoding/json. A missing GOROOT leaves the name as a repo citation
// rather than failing the run.
func (c *checker) stdlibHas(pkg, rest string) bool {
	if pkg == "" || rest == "" {
		return false
	}
	p, ok := c.stdCache[pkg]
	if !ok {
		p = loadStdlib(pkg)
		if c.stdCache == nil {
			c.stdCache = map[string]*docdecl.Package{}
		}
		c.stdCache[pkg] = p
	}
	if p == nil {
		return false
	}
	return p.Has(rest) || p.Has(pkg+"."+rest)
}

func loadStdlib(pkg string) *docdecl.Package {
	dir, ok := stdlibDirs()[pkg]
	if !ok {
		return nil
	}
	p, err := parsePackageDir(dir)
	if err != nil || p == nil || !p.PkgNames[pkg] {
		return nil
	}
	return p
}

var (
	stdlibIndexOnce sync.Once
	stdlibIndex     map[string]string
)

func stdlibDirs() map[string]string {
	stdlibIndexOnce.Do(func() {
		stdlibIndex = indexStdlib()
	})
	return stdlibIndex
}

// indexStdlib maps a package qualifier to the GOROOT directory that declares
// it. internal, vendor, testdata, and src/cmd are skipped: src/cmd is the Go
// tool (package main), and a nested internal copy of a name must not beat
// src/sync or src/go/types. When two directories share a base name, the
// shorter path wins, so path/filepath is `filepath` and go/types is `types`.
func indexStdlib() map[string]string {
	out := map[string]string{}
	score := map[string]int{}
	root := runtime.GOROOT()
	if root == "" {
		return out
	}
	src := filepath.Join(root, "src")
	_ = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if path != src && skipStdlibDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.EqualFold(filepath.Ext(name), ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		base := filepath.Base(dir)
		rel, err := filepath.Rel(src, dir)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		sc := strings.Count(rel, "/")
		prev, seen := score[base]
		if seen && (sc > prev || (sc == prev && dir >= out[base])) {
			return nil
		}
		score[base] = sc
		out[base] = dir
		return nil
	})
	return out
}

func skipStdlibDir(name string) bool {
	return name == "internal" || name == "vendor" || name == "testdata" || name == "cmd" || strings.HasPrefix(name, ".")
}

// parsePackageDir collects declarations in one directory. _test.go is left
// out so a test-only name is not a standard-library declaration. A file that
// does not parse contributes nothing; the files that do still count.
func parsePackageDir(abs string) (*docdecl.Package, error) {
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	p := docdecl.NewPackage(filepath.ToSlash(abs))
	fset := token.NewFileSet()
	any := false
	for _, ent := range ents {
		name := ent.Name()
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(name), ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		any = true
		p.AddFile(fset, filepath.Join(abs, name), name, nil)
	}
	if !any {
		return nil, nil
	}
	return p, nil
}
