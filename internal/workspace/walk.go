package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Walk calls fn for the root and for every member file and member directory.
// A directory IncludesDir rejects is not passed to fn and is not opened, so
// an ignored tree is never statted beyond the name its parent listed.
// fn returning fs.SkipDir on a directory does not descend. On a file it
// stops the rest of that directory, matching fs.WalkDir.
// rel is slash-separated and relative to the workspace root; the root's rel
// is empty.
func (m *Membership) Walk(ctx context.Context, fn func(rel string, d fs.DirEntry) error) error {
	if m == nil {
		return fs.ErrInvalid
	}
	if fn == nil {
		return fs.ErrInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.ensureFresh()
	fi, err := os.Lstat(m.root)
	if err != nil {
		return err
	}
	rootEntry := statEntry{fi: fi}
	if err := fn("", rootEntry); err != nil {
		if errors.Is(err, fs.SkipDir) {
			return nil
		}
		return err
	}
	return m.walk(ctx, m.root, "", fn)
}

func (m *Membership) walk(ctx context.Context, abs, rel string, fn func(string, fs.DirEntry) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	read := m.readDir
	if read == nil {
		read = os.ReadDir
	}
	entries, err := read(abs)
	if err != nil {
		return err
	}
	var files, dirs []string
	type child struct {
		rel string
		abs string
		d   fs.DirEntry
	}
	kids := make([]child, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if name == "" || name == "." || name == ".." {
			continue
		}
		crel := joinRel(rel, name)
		cabs := filepath.Join(abs, name)
		kids = append(kids, child{rel: crel, abs: cabs, d: e})
		if e.IsDir() {
			if m.needsAsk(crel, true) {
				dirs = append(dirs, crel)
			}
			continue
		}
		if m.needsAsk(crel, false) {
			files = append(files, crel)
		}
	}
	// One check-ignore for the names this directory listed, not one per file
	// inside a tree we are about to skip.
	m.askBatch(files, false)
	m.askBatch(dirs, true)

	for _, k := range kids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if k.d.IsDir() {
			if !m.IncludesDir(k.rel) {
				continue
			}
			if err := fn(k.rel, k.d); err != nil {
				if errors.Is(err, fs.SkipDir) {
					continue
				}
				return err
			}
			if err := m.walk(ctx, k.abs, k.rel, fn); err != nil {
				return err
			}
			continue
		}
		if !m.Includes(k.rel) {
			continue
		}
		if err := fn(k.rel, k.d); err != nil {
			if errors.Is(err, fs.SkipDir) {
				return nil
			}
			return err
		}
	}
	return nil
}

// needsAsk is true when only git can still decide the path. Patterns,
// boundaries, and the snapshot are already answers, and asking git about
// them would cache a "not ignored" that must not override those answers.
func (m *Membership) needsAsk(rel string, asDir bool) bool {
	if !m.git {
		return false
	}
	rel, ok := cleanRel(rel)
	if !ok || rel == "" || alwaysExcluded(rel) {
		return false
	}
	self, under := m.boundaryOf(rel)
	if self || under {
		return false
	}
	if asDir {
		if m.patterns.blocksDir(rel) {
			return false
		}
	} else if m.patterns.excluded(rel, false) {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if asDir {
		if _, ok := m.dirs[rel]; ok {
			return false
		}
	} else if _, ok := m.files[rel]; ok {
		return false
	}
	_, decided := m.decided[decideKey(rel, asDir)]
	return !decided
}

func joinRel(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

type statEntry struct{ fi os.FileInfo }

func (s statEntry) Name() string               { return s.fi.Name() }
func (s statEntry) IsDir() bool                { return s.fi.IsDir() }
func (s statEntry) Type() fs.FileMode          { return s.fi.Mode().Type() }
func (s statEntry) Info() (fs.FileInfo, error) { return s.fi, nil }
