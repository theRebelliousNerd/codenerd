package workspace

import (
	"path"
	"runtime"
	"strings"
)

// pattern is one compiled world.ignore_patterns entry.
type pattern struct {
	neg        bool
	dirOnly    bool
	unanchored bool
	base       string
	segments   []segment
}

type segment struct {
	star bool
	glob string
	lit  string
}

type patternSet struct {
	items []pattern
	fold  bool
}

func compilePatterns(raw []string) patternSet {
	ps := patternSet{fold: runtime.GOOS == "windows"}
	for _, r := range raw {
		if p, ok := compilePattern(r); ok {
			ps.items = append(ps.items, p)
		}
	}
	return ps
}

func compilePattern(raw string) (pattern, bool) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "\\", "/")
	if s == "" || s == "!" {
		return pattern{}, false
	}
	neg := false
	if strings.HasPrefix(s, "!") {
		neg = true
		s = strings.TrimSpace(s[1:])
		if s == "" {
			return pattern{}, false
		}
	}
	dirOnly := false
	for strings.HasSuffix(s, "/") {
		dirOnly = true
		s = strings.TrimSuffix(s, "/")
	}
	if s == "" || strings.Contains(s, "//") || !bracketsBalanced(s) {
		return pattern{}, false
	}
	p := pattern{neg: neg, dirOnly: dirOnly}
	if !strings.Contains(s, "/") {
		p.unanchored = true
		p.base = s
		return p, true
	}
	s = strings.TrimPrefix(s, "/")
	if s == "" {
		return pattern{}, false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" {
			return pattern{}, false
		}
		seg := segment{}
		switch {
		case part == "**":
			seg.star = true
		case strings.ContainsAny(part, "*?[]"):
			seg.glob = part
		default:
			seg.lit = part
		}
		p.segments = append(p.segments, seg)
	}
	if len(p.segments) == 0 {
		return pattern{}, false
	}
	return p, true
}

func bracketsBalanced(s string) bool {
	open := false
	for _, r := range s {
		switch r {
		case '[':
			if !open {
				open = true
			}
		case ']':
			if open {
				open = false
			}
		}
	}
	return !open
}

// excluded reports whether the last matching user pattern drops rel.
// isDir is false for a file. Ancestors are tested as directories.
func (ps patternSet) excluded(rel string, isDir bool) bool {
	last := ps.lastMatch(rel, isDir)
	return last >= 0 && !ps.items[last].neg
}

// blocksDir is false when the directory must be opened: either it is not
// excluded, or a later negation might re-include something inside it.
func (ps patternSet) blocksDir(rel string) bool {
	last := ps.lastMatch(rel, true)
	if last < 0 || ps.items[last].neg {
		return false
	}
	for i := last + 1; i < len(ps.items); i++ {
		if ps.items[i].neg && ps.items[i].mayReincludeInside(rel, ps.fold) {
			return false
		}
	}
	return true
}

func (ps patternSet) lastMatch(rel string, isDir bool) int {
	rel, ok := cleanRel(rel)
	if !ok || rel == "" {
		return -1
	}
	last := -1
	for i := range ps.items {
		if ps.items[i].matches(rel, isDir, ps.fold) {
			last = i
		}
	}
	return last
}

func (p pattern) matches(rel string, isDir, fold bool) bool {
	if p.unanchored {
		if (!p.dirOnly || isDir) && matchName(p.base, path.Base(rel), fold) {
			return true
		}
		for _, a := range ancestorPaths(rel) {
			if matchName(p.base, path.Base(a), fold) {
				return true
			}
		}
		return false
	}
	if matchSegs(p.segments, strings.Split(rel, "/"), p.dirOnly, isDir, fold) {
		return true
	}
	for _, a := range ancestorPaths(rel) {
		if matchSegs(p.segments, strings.Split(a, "/"), p.dirOnly, true, fold) {
			return true
		}
	}
	return false
}

// mayReincludeInside is true when this negation could be the last match for
// some path under dir. Unknown shapes return true so the directory is opened.
func (p pattern) mayReincludeInside(dir string, fold bool) bool {
	if !p.neg {
		return false
	}
	if p.unanchored {
		return true
	}
	var lits []string
	for _, seg := range p.segments {
		if seg.star || seg.glob != "" {
			break
		}
		lits = append(lits, seg.lit)
	}
	prefix := strings.Join(lits, "/")
	allLit := len(lits) == len(p.segments)
	var ok bool
	dir, ok = cleanRel(dir)
	if !ok {
		// Not a path we can reason about. Opening the directory is safe.
		return true
	}
	if allLit {
		return strictAncestor(dir, prefix, fold)
	}
	if prefix == "" {
		return true
	}
	np, nd := normPath(prefix, fold), normPath(dir, fold)
	if nd == np || strictAncestor(dir, prefix, fold) || strictAncestor(prefix, dir, fold) {
		return true
	}
	return false
}

func matchSegs(segs []segment, parts []string, dirOnly, isDir, fold bool) bool {
	return matchAt(segs, 0, parts, 0, dirOnly, isDir, fold)
}

func matchAt(segs []segment, si int, parts []string, pi int, dirOnly, isDir, fold bool) bool {
	for si < len(segs) {
		seg := segs[si]
		if seg.star {
			if si == len(segs)-1 {
				if dirOnly && !isDir {
					return false
				}
				return true
			}
			for k := pi; k <= len(parts); k++ {
				if matchAt(segs, si+1, parts, k, dirOnly, isDir, fold) {
					return true
				}
			}
			return false
		}
		if pi >= len(parts) || !seg.match(parts[pi], fold) {
			return false
		}
		si++
		pi++
	}
	if pi != len(parts) {
		return false
	}
	if dirOnly && !isDir {
		return false
	}
	return true
}

func (s segment) match(part string, fold bool) bool {
	part = normPath(part, fold)
	switch {
	case s.glob != "":
		ok, err := path.Match(normPath(s.glob, fold), part)
		return err == nil && ok
	default:
		return part == normPath(s.lit, fold)
	}
}

func matchName(pat, name string, fold bool) bool {
	name = normPath(name, fold)
	pat = normPath(pat, fold)
	if strings.ContainsAny(pat, "*?[]") {
		ok, err := path.Match(pat, name)
		return err == nil && ok
	}
	return pat == name
}

func ancestorPaths(rel string) []string {
	var out []string
	for {
		i := strings.LastIndex(rel, "/")
		if i <= 0 {
			break
		}
		rel = rel[:i]
		out = append(out, rel)
	}
	return out
}

func strictAncestor(dir, full string, fold bool) bool {
	var okD, okF bool
	dir, okD = cleanRel(dir)
	full, okF = cleanRel(full)
	if !okD || !okF || full == "" || dir == full {
		return false
	}
	if dir == "" {
		return true
	}
	return strings.HasPrefix(normPath(full, fold), normPath(dir, fold)+"/")
}

func normPath(s string, fold bool) string {
	if fold {
		return strings.ToLower(s)
	}
	return s
}
