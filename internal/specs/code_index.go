package specs

import (
	"regexp"
	"strings"
)

// CodeIndex is the lookup the resolver asks: every element of the workspace by
// every spelling a citation may use, and the set of files that exist.
type CodeIndex struct {
	elems  []Element
	byRef  map[string]int
	byFile map[string][]int
	// byName maps a spelling (Key, ID, Ref, bare Name) to the elements it names.
	byName map[string][]int
	files  map[string]bool
}

// NewCodeIndex indexes elements. files are the workspace paths that exist, any
// kind: a citation of a file that is not in the set is stale.
func NewCodeIndex(elems []Element, files []string) *CodeIndex {
	ix := &CodeIndex{
		elems:  elems,
		byRef:  make(map[string]int, len(elems)),
		byFile: make(map[string][]int),
		byName: make(map[string][]int),
		files:  make(map[string]bool, len(files)),
	}
	for _, f := range files {
		ix.files[f] = true
	}
	for i := range elems {
		e := &elems[i]
		ix.files[e.File] = true
		ix.byRef[e.Ref] = i
		ix.byFile[e.File] = append(ix.byFile[e.File], i)
		if searchable(e) {
			seen := map[string]bool{}
			for _, spelling := range []string{e.Key, e.ID, e.Ref} {
				if spelling != "" && !seen[spelling] {
					seen[spelling] = true
					ix.byName[spelling] = append(ix.byName[spelling], i)
				}
			}
			// A bare name finds functions and types; a method is found by its
			// receiver-qualified spelling, because `Evaluate` alone names every
			// method of that name in the workspace.
			if e.Receiver == "" && e.Name != "" && e.Name != e.Key && !seen[e.Name] {
				ix.byName[e.Name] = append(ix.byName[e.Name], i)
			}
		}
	}
	return ix
}

// searchable elements are the ones a bare citation may name: a header or a
// broken region is an address inside a file, not something prose cites.
func searchable(e *Element) bool {
	switch e.Kind {
	case "header", "syntax_error":
		return false
	}
	return !strings.HasPrefix(e.Key, "syntax_error#")
}

// Elements returns every indexed element.
func (ix *CodeIndex) Elements() []Element { return ix.elems }

// Element returns the element with ref.
func (ix *CodeIndex) Element(ref string) (Element, bool) {
	i, ok := ix.byRef[ref]
	if !ok {
		return Element{}, false
	}
	return ix.elems[i], true
}

// HasFile reports that path exists in the workspace.
func (ix *CodeIndex) HasFile(path string) bool { return ix.files[path] }

// FileHeader is the file-level address of path: its header element.
func (ix *CodeIndex) FileHeader(path string) (Element, bool) {
	for _, i := range ix.byFile[path] {
		if ix.elems[i].Kind == "header" {
			return ix.elems[i], true
		}
	}
	return Element{}, false
}

var (
	recvQualified = regexp.MustCompile(`^\(\*?([A-Za-z_][A-Za-z0-9_]*)\)\.([A-Za-z_][A-Za-z0-9_]*)$`)
	predArity     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*/[0-9]+$`)
)

// canonicalSymbol spells a citation as the element model keys it: Go's
// "(*T).M" is "T.M", and a Mangle predicate with its arity is its decl key.
func canonicalSymbol(sym string) string {
	if m := recvQualified.FindStringSubmatch(sym); m != nil {
		return m[1] + "." + m[2]
	}
	return sym
}

// inFile finds the elements of path that sym names. A key is exact; a bare
// name finds functions, types and the like, and falls back to methods only
// when nothing else carries it.
func (ix *CodeIndex) inFile(path, sym string) []Element {
	sym = canonicalSymbol(sym)
	var exact, named, methods []Element
	for _, i := range ix.byFile[path] {
		e := ix.elems[i]
		if !searchable(&e) {
			continue
		}
		switch {
		case e.Key == sym || (predArity.MatchString(sym) && e.Key == "decl:"+sym):
			exact = append(exact, e)
		case e.Receiver == "" && e.Name == sym:
			named = append(named, e)
		case e.Receiver != "" && e.Name == sym:
			methods = append(methods, e)
		}
	}
	switch {
	case len(exact) > 0:
		return exact
	case len(named) > 0:
		return named
	}
	return methods
}

// global finds the elements a bare symbol names anywhere in the workspace. A
// Mangle predicate with its arity finds the predicate's Decl.
func (ix *CodeIndex) global(sym string) []Element {
	sym = canonicalSymbol(sym)
	var idx []int
	if predArity.MatchString(sym) {
		idx = ix.byName["decl:"+sym]
	} else {
		idx = ix.byName[sym]
	}
	out := make([]Element, 0, len(idx))
	for _, i := range idx {
		out = append(out, ix.elems[i])
	}
	return out
}
