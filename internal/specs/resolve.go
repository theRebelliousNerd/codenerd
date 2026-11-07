package specs

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/world/codemodel"
)

// Measurement is what the code model says about an element a requirement
// names, beyond what the index row carries. It is measured only for those
// elements: a call graph and a test search are not free.
type Measurement struct {
	HasTest bool
	// Calls are the refs of the elements it calls directly.
	Calls []string
}

// Input is everything the resolver reads.
type Input struct {
	Docs  []*DocScan
	Codes []*CodeScan
	Code  *CodeIndex
	// Specs are the parsed machine-checkable spec files.
	Specs []*SpecFile
	// Modified is a file's last-modified time in unix seconds. The clock is the
	// caller's: the kernel compares two measurements, it never reads a clock.
	Modified func(file string) int64
	// Measure answers for a requirement target. Nil measures nothing.
	Measure func(e Element) Measurement
}

// Result is the pairing of one snapshot of specs and code.
type Result struct {
	Links []Link
	// Sections are the sections some link has an end in.
	Sections []Section
	// SectionModified is each listed section's document time.
	SectionModified map[string]int64
	// Elements are the elements the kernel is told about: link targets, the
	// exported elements of every package a link reaches (the universe
	// unspecced_element is derived over), and the targets of requirements.
	Elements        []Element
	ElementModified map[string]int64
	// Measured holds the measurement of each requirement target, by ref.
	Measured     map[string]Measurement
	Requirements []Requirement
	Requires     []Requires
	Calls        []RequiresCalls
	// Problems are citations that were read and refused.
	Problems []Problem
}

// Resolve pairs the scans against the code index.
func Resolve(in Input) Result {
	r := &resolver{in: in, docs: map[string]*DocScan{}, seen: map[Link]bool{}, problems: map[Problem]bool{}}
	for _, d := range in.Docs {
		r.docs[d.Path] = d
	}
	sort.Slice(in.Docs, func(i, j int) bool { return in.Docs[i].Path < in.Docs[j].Path })
	for _, d := range in.Docs {
		for _, c := range d.Cites {
			r.docCite(d, c)
		}
	}
	sort.Slice(in.Codes, func(i, j int) bool { return in.Codes[i].Path < in.Codes[j].Path })
	for _, cs := range in.Codes {
		for _, c := range cs.Cites {
			r.codeCite(cs, c)
		}
	}
	return r.result()
}

type resolver struct {
	in       Input
	docs     map[string]*DocScan
	links    []Link
	seen     map[Link]bool
	problems map[Problem]bool
	plist    []Problem
}

func (r *resolver) link(l Link) {
	if !r.seen[l] {
		r.seen[l] = true
		r.links = append(r.links, l)
	}
}

func (r *resolver) problem(p Problem) {
	if !r.problems[p] {
		r.problems[p] = true
		r.plist = append(r.plist, p)
	}
}

// docCite resolves a citation read from a section to the elements it names.
func (r *resolver) docCite(d *DocScan, c RawCite) {
	spec := d.Path + ":" + c.Section
	mk := func(code string) Link {
		return Link{SpecRef: spec, CodeRef: code, Relation: c.Relation, Source: c.Source}
	}
	ix := r.in.Code
	stale := func(code string) { r.link(mk(code)) }
	if c.Path != "" {
		if c.Symbol == "" {
			switch h, ok := ix.FileHeader(c.Path); {
			case ok:
				r.link(mk(h.Ref))
			case !ix.HasFile(c.Path):
				stale(c.Path + ":" + codemodel.HeaderKey)
			}
			return
		}
		if !ix.HasFile(c.Path) {
			if c.Explicit || !strings.Contains(c.Symbol, ".") {
				stale(c.Path + ":" + c.Symbol)
			}
			return
		}
		switch found := ix.inFile(c.Path, c.Symbol); len(found) {
		case 1:
			r.link(mk(found[0].Ref))
		case 0:
			if c.Explicit || !strings.Contains(c.Symbol, ".") {
				stale(c.Path + ":" + c.Symbol)
			}
		default:
			r.problem(Problem{Where: spec, Text: c.Raw, Reason: ambiguity(c.Symbol, found)})
		}
		return
	}
	switch found := ix.global(c.Symbol); len(found) {
	case 1:
		r.link(mk(found[0].Ref))
	case 0:
		// A bare name that resolves to nothing may not be a citation at all;
		// an annotation is deliberate, and is stale.
		if c.Explicit {
			stale(c.Symbol)
		}
	default:
		r.problem(Problem{Where: spec, Text: c.Raw, Reason: ambiguity(c.Symbol, found)})
	}
}

// codeCite resolves a "spec:" line of a doc comment to the section it names.
func (r *resolver) codeCite(cs *CodeScan, c CodeCite) {
	var elem *Element
	for _, i := range r.in.Code.byFile[cs.Path] {
		if e := &r.in.Code.elems[i]; e.Key == c.Element {
			elem = e
			break
		}
	}
	if elem == nil {
		return
	}
	if c.Heading == "" {
		r.problem(Problem{Where: elem.Ref, Text: c.Raw, Reason: "names a document, not a section: add the heading path after the document"})
		return
	}
	mk := func(spec string) Link {
		return Link{SpecRef: spec, CodeRef: elem.Ref, Relation: RelSpecifies, Source: SrcCodeComment}
	}
	doc, ok := r.docs[c.Doc]
	if !ok {
		r.link(mk(c.Doc + ":" + c.Heading))
		return
	}
	var sections []*codemodel.Element
	for _, e := range doc.Model.Lookup(c.Heading) {
		if e.Kind == codemodel.KindSection {
			sections = append(sections, e)
		}
	}
	switch len(sections) {
	case 0:
		r.link(mk(c.Doc + ":" + c.Heading))
	case 1:
		r.link(mk(c.Doc + ":" + sections[0].Key))
	default:
		keys := make([]string, len(sections))
		for i, s := range sections {
			keys[i] = s.Key
		}
		r.problem(Problem{Where: elem.Ref, Text: c.Raw,
			Reason: fmt.Sprintf("%q names %d sections of %s (%s); spell the full heading path", c.Heading, len(sections), c.Doc, strings.Join(keys, "; "))})
	}
}

func ambiguity(sym string, found []Element) string {
	refs := make([]string, len(found))
	for i, e := range found {
		refs[i] = e.Ref
	}
	sort.Strings(refs)
	return fmt.Sprintf("%q names %d elements, so it is a search, not an address: %s", sym, len(refs), strings.Join(refs, "; "))
}

func (r *resolver) result() Result {
	in := r.in
	out := Result{
		Links:           r.links,
		Problems:        r.plist,
		SectionModified: map[string]int64{},
		ElementModified: map[string]int64{},
		Measured:        map[string]Measurement{},
	}
	modified := func(file string) int64 {
		if in.Modified == nil {
			return 0
		}
		return in.Modified(file)
	}
	// Sections with a link end.
	linked := map[string]bool{}
	for _, l := range out.Links {
		linked[l.SpecRef] = true
	}
	for _, d := range in.Docs {
		for _, s := range d.Sections {
			if linked[s.Ref] {
				out.Sections = append(out.Sections, s)
				out.SectionModified[s.Ref] = modified(d.Path)
			}
		}
	}
	// Elements: targets, the exported universe of the packages they sit in,
	// and the targets of requirements.
	want := map[string]bool{}
	pkgs := map[string]bool{}
	for _, l := range out.Links {
		if e, ok := in.Code.Element(l.CodeRef); ok {
			want[e.Ref] = true
			pkgs[e.Dir] = true
		}
	}
	for _, sf := range in.Specs {
		out.Requirements = append(out.Requirements, sf.Requirements...)
		out.Requires = append(out.Requires, sf.Requires...)
		out.Calls = append(out.Calls, sf.Calls...)
	}
	targets := map[string]bool{}
	for _, q := range out.Requires {
		targets[q.CodeRef] = true
	}
	for _, q := range out.Calls {
		targets[q.CodeRef] = true
	}
	for ref := range targets {
		if e, ok := in.Code.Element(ref); ok {
			want[e.Ref] = true
			if in.Measure != nil {
				out.Measured[ref] = in.Measure(e)
			}
		}
	}
	for _, e := range in.Code.Elements() {
		if want[e.Ref] || (pkgs[e.Dir] && inUniverse(e)) {
			out.Elements = append(out.Elements, e)
			out.ElementModified[e.Ref] = modified(e.File)
		}
	}
	sort.Slice(out.Elements, func(i, j int) bool { return out.Elements[i].Ref < out.Elements[j].Ref })
	return out
}

// inUniverse is what a package is expected to have specified: its exported
// callable and type declarations outside tests.
func inUniverse(e Element) bool {
	if !e.Exported || e.Test {
		return false
	}
	switch e.Kind {
	case "function", "method", "struct", "interface", "type", "class":
		return true
	}
	return false
}
