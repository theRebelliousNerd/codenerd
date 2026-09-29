package codemodel

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// RenameSite is one identifier span that resolves to a declaration. Replacing
// every site's text with the new name, and no other bytes, is the rename.
type RenameSite struct {
	Path       string
	Start, End int
	Line       int
}

// RenameSites lists the declaration's name and every reference that resolves
// to it, in path order. A same-named local is absent: its bind id is not the
// declaration's. files are the other files of the workspace; decl is included
// whether or not it also appears in files.
func RenameSites(decl *File, key string, files []*File) ([]RenameSite, error) {
	if decl == nil {
		return nil, fmt.Errorf("rename needs the file that declares the name")
	}
	e := decl.Element(key)
	if e == nil {
		return nil, fmt.Errorf("%s has no element %q", decl.Path, key)
	}
	if e.Name == "" || e.NameStart <= 0 || e.NameEnd <= e.NameStart {
		return nil, fmt.Errorf("%s has no identifier to rename", e.Key)
	}
	want := renameWant(decl, e)
	if len(want) == 0 {
		return nil, fmt.Errorf("%s has no binding to rename", e.Key)
	}
	seen := map[string]bool{}
	var sites []RenameSite
	add := func(f *File) {
		if f == nil {
			return
		}
		for _, b := range f.binds {
			if !want[b.id] || b.end <= b.start {
				continue
			}
			if b.start < 0 || b.end > len(f.Source) || f.Source[b.start:b.end] != e.Name {
				continue
			}
			k := f.Path + fmt.Sprintf("#%d", b.start)
			if seen[k] {
				continue
			}
			seen[k] = true
			sites = append(sites, RenameSite{Path: f.Path, Start: b.start, End: b.end, Line: f.LineOf(b.start)})
		}
		for _, u := range f.uses {
			if !want[u.Bind] || u.End <= u.Start {
				continue
			}
			if u.Start < 0 || u.End > len(f.Source) || f.Source[u.Start:u.End] != e.Name {
				continue
			}
			k := f.Path + fmt.Sprintf("#%d", u.Start)
			if seen[k] {
				continue
			}
			seen[k] = true
			sites = append(sites, RenameSite{Path: f.Path, Start: u.Start, End: u.End, Line: u.Line})
		}
	}
	add(decl)
	for _, f := range files {
		if f == nil || f.Path == decl.Path {
			continue
		}
		add(f)
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].Path != sites[j].Path {
			return sites[i].Path < sites[j].Path
		}
		return sites[i].Start < sites[j].Start
	})
	return sites, nil
}

// renameWant is the declaration's own bind plus, for a name other files can
// import, the id those imports carry. A default export is not that id: its
// importers bind a local alias, and renaming the function must not rename
// `import App from "./mod"`.
func renameWant(decl *File, e *Element) map[string]bool {
	want := map[string]bool{}
	if e.bind != "" {
		want[e.bind] = true
	}
	exportName := e.Name
	switch {
	case decl.Language == LangPython && e.Receiver == "" && exportName != "":
		want["i:"+decl.Path+"#"+exportName] = true
	case e.namedExport && !e.defaultExport && exportName != "":
		want["i:"+decl.Path+"#"+exportName] = true
	}
	return want
}

// ValidRename reports whether newName can replace an identifier.
func ValidRename(newName string) bool {
	if newName == "" {
		return false
	}
	for i, r := range newName {
		if r == '_' || unicode.IsLetter(r) {
			continue
		}
		if i > 0 && unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

// ApplyRename splices newName into this file's sites and re-parses. Elements
// that do not overlap a replaced span must come out byte-identical, and the
// file must still parse.
func ApplyRename(f *File, newName string, sites []RenameSite) (*Outcome, error) {
	if f == nil {
		return nil, fmt.Errorf("rename needs a file")
	}
	if !ValidRename(newName) {
		return nil, fmt.Errorf("to %q is not an identifier", newName)
	}
	var mine []RenameSite
	for _, site := range sites {
		if site.Path != "" && site.Path != f.Path {
			continue
		}
		mine = append(mine, site)
	}
	if len(mine) == 0 {
		return &Outcome{File: f}, nil
	}
	sort.Slice(mine, func(i, j int) bool { return mine[i].Start > mine[j].Start })
	src := f.Source
	for _, site := range mine {
		if site.Start < 0 || site.End > len(src) || site.Start > site.End {
			return nil, fmt.Errorf("rename span %d-%d is outside %s", site.Start, site.End, f.Path)
		}
		src = src[:site.Start] + newName + src[site.End:]
	}
	target := map[string]bool{}
	lo, hi := mine[len(mine)-1].Start, mine[0].End
	for _, site := range mine {
		if site.Start < lo {
			lo = site.Start
		}
		if site.End > hi {
			hi = site.End
		}
		for _, k := range affectedKeys(f, site.Start, site.End) {
			target[k] = true
		}
	}
	nf, _ := Parse(f.Path, src)
	span := lineSpan{lo: strings.Count(f.Source[:lo], "\n") + 1}
	span.hi = strings.Count(f.Source[:hi], "\n") + 1
	if err := parseVerdict(f, nf, span); err != nil {
		return nil, err
	}
	out := &Outcome{File: nf}
	oldByKey := make(map[string]*Element, len(f.Elements))
	for i := range f.Elements {
		oldByKey[f.Elements[i].Key] = &f.Elements[i]
	}
	newByKey := make(map[string]*Element, len(nf.Elements))
	for i := range nf.Elements {
		newByKey[nf.Elements[i].Key] = &nf.Elements[i]
	}
	var violations []string
	for i := range f.Elements {
		oe := &f.Elements[i]
		if target[oe.Key] {
			if newByKey[oe.Key] == nil {
				out.Removed = append(out.Removed, oe.Key)
			}
			continue
		}
		if oe.Kind == KindSyntaxError {
			continue
		}
		ne := newByKey[oe.Key]
		switch {
		case ne == nil:
			violations = append(violations, fmt.Sprintf("%s (lines %d-%d) would disappear", oe.Key, oe.StartLine, oe.EndLine))
		case ne.Revision != oe.Revision && squash(nf.Text(ne)) != squash(f.Text(oe)):
			violations = append(violations, fmt.Sprintf("%s (lines %d-%d) would change", oe.Key, oe.StartLine, oe.EndLine))
		}
	}
	if len(violations) > 0 {
		return nil, fmt.Errorf("refusing the edit: it changes elements it does not target -- %s", strings.Join(violations, "; "))
	}
	for i := range nf.Elements {
		ne := &nf.Elements[i]
		oe := oldByKey[ne.Key]
		if oe != nil && !target[ne.Key] {
			continue
		}
		if oe != nil && oe.Revision == ne.Revision && ne.Kind != KindHeader {
			continue
		}
		if ne.Kind == KindHeader && !target[HeaderKey] {
			continue
		}
		out.Touched = append(out.Touched, ne)
	}
	return out, nil
}
