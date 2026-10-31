package session

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// A coverage profile names each block's file by import path and file name
// ("collide/a.go"); the turn names what it wrote by workspace path ("a.go").
// The two were joined by a suffix match, and "collide/extra.go" ends with
// "a.go": an executed block of extra.go was handed to the changed element in
// a.go, which then counted as executed and could meet its witness though no
// test ran it (audit coverage-source-identity-witness).
//
// coverSourceResolver maps a profile path to the one written path it is.
// A written file's profile path is its module's path joined to the file's
// path within the module (the nearest go.mod above it), so the match is
// exact. A written file no go.mod resolves (no module, an unreadable one)
// falls back to a whole-component suffix: the profile path is the written
// path or ends with "/" + it, and the most specific written path wins. The
// resolved source rides on the block (UncoveredBlock.Source), so every later
// join -- changed lines, element spans, the closure's partition -- compares
// identities instead of matching names again.
type coverSourceResolver struct {
	exact    map[string]string // profile path -> written path
	fallback []string          // written paths no module resolved
}

func newCoverSourceResolver(workspace string, written []string) coverSourceResolver {
	r := coverSourceResolver{exact: map[string]string{}}
	modules := map[string]string{} // module dir -> module path ("" unreadable)
	for _, w := range written {
		n := NormalizeCoverPath(strings.TrimSpace(w))
		if n == "" {
			continue
		}
		if workspace != "" {
			abs := filepath.FromSlash(n)
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(workspace, abs)
			}
			if modDir, modPath := moduleOf(filepath.Dir(abs), modules); modPath != "" {
				if rel, err := filepath.Rel(modDir, abs); err == nil && !strings.HasPrefix(rel, "..") {
					r.exact[modPath+"/"+filepath.ToSlash(rel)] = n
					continue
				}
			}
		}
		r.fallback = append(r.fallback, n)
	}
	return r
}

// source is the written path profile names, or "" when it names none.
func (r coverSourceResolver) source(profile string) string {
	p := NormalizeCoverPath(profile)
	if s, ok := r.exact[p]; ok {
		return s
	}
	best := ""
	for _, w := range r.fallback {
		if (p == w || strings.HasSuffix(p, "/"+w)) && len(w) > len(best) {
			best = w
		}
	}
	return best
}

// moduleOf finds the nearest go.mod at or above dir and returns its directory
// and module path; "" when there is none or it names no module.
func moduleOf(dir string, cache map[string]string) (string, string) {
	for d := dir; ; d = filepath.Dir(d) {
		if path, ok := cache[d]; ok {
			return d, path
		}
		if f, err := os.Open(filepath.Join(d, "go.mod")); err == nil {
			path := modulePath(f)
			f.Close()
			cache[d] = path
			return d, path
		}
		if filepath.Dir(d) == d {
			return "", ""
		}
	}
}

// modulePath reads a complete go.mod with the same lax parser retrieval uses:
// quoted and parenthesized module directives are identities, while unrelated
// future directives are tolerated. Missing, unreadable or malformed manifests
// supply no exact identity; the resolver keeps its component-bound fallback.
func modulePath(f *os.File) string {
	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	parsed, err := modfile.ParseLax(f.Name(), data, nil)
	if err != nil || parsed.Module == nil {
		return ""
	}
	return parsed.Module.Mod.Path
}

// blockIn reports whether block b is in the written file: by the source the
// profile resolved it to, or, for a block built without one, by a whole
// trailing path component of its profile path.
func blockIn(b UncoveredBlock, written string) bool {
	w := NormalizeCoverPath(written)
	if w == "" {
		return false
	}
	if b.Source != "" {
		return b.Source == w
	}
	p := NormalizeCoverPath(b.File)
	return p == w || strings.HasSuffix(p, "/"+w)
}
