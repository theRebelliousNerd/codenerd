package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/specs"
	"codenerd/internal/world/codemodel"
)

// The specs block of the holographic context.
//
// A coder looking at code is told which section of which document specifies
// it, with a bounded excerpt of what the section says; looking at a Markdown
// section, which code elements it names. Beside each, what the kernel has
// derived: that the code changed after the section (spec_drift), that a link
// names something that is gone (dangling_spec_link), that a stated requirement
// is not met (requirement_violated), and which exported declarations of the
// package no section reaches (unspecced_element, spec_coverage). Go reads the
// facts and the documents; the judgments are the kernel's.
//
// Bounded like every counted block. How many entries are listed, and how many
// runes of each excerpt, are the working policy's decisions when decide is set
// (the /specs and /spec_excerpt dimensions of holographic_render.mg). With no
// policy every entry renders and an excerpt is the section's whole prose: there
// is no budget to cut to. What is cut is named: the entries not listed, and the
// characters of an excerpt not shown, each with the call that reads the rest.

// specEntry is one line of the block: a link seen from the target.
type specEntry struct {
	section  string // spec ref
	code     string // code ref
	relation string
	drift    bool
	dangling string // why the link is dangling, "" when it is not
	subject  string // what the line is about, in the target's terms
}

type specView struct {
	elements  map[string]core.Fact // spec_element by ref
	sections  map[string]core.Fact // spec_section by ref
	links     []core.Fact
	drift     map[string]bool
	dangling  map[string]bool
	violated  []core.Fact
	reqText   map[string]string
	unspecced map[string]bool
	coverage  map[string][2]int64
}

// specBlock renders the block for the file at rel (workspace-relative), or ""
// when the file is in no spec and nothing is derived about it. target is the
// name the render's other blocks give the policy.
func (h *HolographicProvider) specBlock(ctx context.Context, target, rel string, budgetBytes int, decide func(ctx context.Context, dimension, target string, total, avgBytes, budgetBytes int) (int, error)) string {
	if h == nil || h.kernel == nil || rel == "" {
		return ""
	}
	v := h.loadSpecView()
	if v == nil {
		return ""
	}
	var entries []specEntry
	var extra []string // lines that are not links: unspecced elements, violations
	markdown := codemodel.IsMarkdownPath(rel)
	if markdown {
		entries, extra = v.fromSection(rel)
	} else {
		entries, extra = v.fromCode(rel)
	}
	if len(entries) == 0 && len(extra) == 0 {
		return ""
	}

	// Each entry's excerpt is resolved before the counts are measured, so the
	// policy measures the lines the prompt will carry.
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = specEntryLine(e, "", 0)
	}
	shown := entries
	chars, cut := 0, false
	if decide != nil {
		kept := resolveRenderedLines(ctx, holoSpecs, target, lines, budgetBytes, decide)
		shown = entries[:len(kept)]
		if len(shown) > 0 && !markdown {
			got, err := decide(ctx, holoSpecExcerpt, target, len(shown), 1, budgetBytes)
			if err != nil {
				logging.WorldDebug("holographic spec excerpt: policy failed (%v); excerpts withheld", err)
				shown = nil
			} else {
				cut, chars = true, got
			}
		}
	}

	var b strings.Builder
	if markdown {
		fmt.Fprintf(&b, "### Code the sections of %s name (%d links)\n\n", filepath.Base(rel), len(entries))
	} else {
		fmt.Fprintf(&b, "### Specs for %s (%d links)\n\n", filepath.Base(rel), len(entries))
	}
	docs := map[string]string{} // file -> source, read once
	excerpted := map[string]bool{}
	for _, e := range shown {
		excerpt, rest := "", 0
		// A section that specifies several elements of the file says what it says once.
		if !markdown && e.dangling == "" && !excerpted[e.section] {
			excerpted[e.section] = true
			excerpt, rest = h.specExcerpt(docs, e.section, chars, cut)
		}
		b.WriteString(specEntryLine(e, excerpt, rest))
	}
	if rest := len(entries) - len(shown); rest > 0 {
		if markdown {
			fmt.Fprintf(&b, "- … %d more links not listed; `get_elements path=%s` lists the sections\n", rest, rel)
		} else {
			fmt.Fprintf(&b, "- … %d more links not listed; `get_element` on a section ref (`path.md:Heading > Path`) reads it whole\n", rest)
		}
	}
	for _, line := range extra {
		b.WriteString(line)
	}
	b.WriteString("\n")
	return b.String()
}

func specEntryLine(e specEntry, excerpt string, rest int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- %s: `%s` (%s)", e.subject, e.section, e.relation)
	if e.dangling != "" {
		fmt.Fprintf(&b, " DANGLING: %s", e.dangling)
	}
	if e.drift {
		b.WriteString(" DRIFT: the code changed after this section was last edited")
	}
	b.WriteString("\n")
	if excerpt != "" {
		fmt.Fprintf(&b, "  > %s", excerpt)
		if rest > 0 {
			fmt.Fprintf(&b, "… (%d more characters; `get_element` on the section returns it whole)", rest)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// specExcerpt is the prose of the section a spec ref names. cut says a policy
// bounded it to chars runes; without one the whole prose is the excerpt.
func (h *HolographicProvider) specExcerpt(docs map[string]string, ref string, chars int, cut bool) (string, int) {
	file, key, ok := splitSpecRef(ref)
	if !ok {
		return "", 0
	}
	src, loaded := docs[file]
	if !loaded {
		data, err := os.ReadFile(filepath.Join(h.workDir, filepath.FromSlash(file)))
		if err == nil {
			src = string(data)
		}
		docs[file] = src
	}
	text, ok := specs.SectionText(file, src, key)
	if !ok || text == "" {
		return "", 0
	}
	if !cut {
		return text, 0
	}
	if chars < 0 {
		chars = 0
	}
	if n := utf8.RuneCountInString(text); n > chars {
		return string([]rune(text)[:chars]), n - chars
	}
	return text, 0
}

// splitSpecRef reads "Docs/x.md:Install > Linux": the document, then the key.
func splitSpecRef(ref string) (file, key string, ok bool) {
	i := strings.Index(ref, ".md:")
	if i < 0 {
		return "", "", false
	}
	return ref[:i+3], ref[i+4:], true
}

// loadSpecView reads the spec facts, or nil when the kernel holds none. A
// predicate that cannot be read is an empty one: the block degrades to what
// the other predicates say, and the context is never withheld for it.
func (h *HolographicProvider) loadSpecView() *specView {
	links, _ := h.kernel.Query("spec_link")
	if len(links) == 0 {
		return nil
	}
	v := &specView{
		elements: map[string]core.Fact{}, sections: map[string]core.Fact{}, links: links,
		drift: map[string]bool{}, dangling: map[string]bool{},
		reqText: map[string]string{}, unspecced: map[string]bool{}, coverage: map[string][2]int64{},
	}
	elements, _ := h.kernel.Query("spec_element")
	for _, f := range elements {
		if len(f.Args) >= 6 {
			v.elements[specStr(f.Args[0])] = f
		}
	}
	sections, _ := h.kernel.Query("spec_section")
	for _, f := range sections {
		if len(f.Args) >= 4 {
			v.sections[specStr(f.Args[0])] = f
		}
	}
	drift, _ := h.kernel.Query("spec_drift")
	for _, f := range drift {
		if len(f.Args) >= 2 {
			v.drift[specStr(f.Args[0])+"\x00"+specStr(f.Args[1])] = true
		}
	}
	dangling, _ := h.kernel.Query("dangling_spec_link")
	for _, f := range dangling {
		if len(f.Args) >= 2 {
			v.dangling[specStr(f.Args[0])+"\x00"+specStr(f.Args[1])] = true
		}
	}
	v.violated, _ = h.kernel.Query("requirement_violated")
	requirements, _ := h.kernel.Query("spec_requirement")
	for _, f := range requirements {
		if len(f.Args) >= 2 {
			v.reqText[specStr(f.Args[0])] = specStr(f.Args[1])
		}
	}
	unspecced, _ := h.kernel.Query("unspecced_element")
	for _, f := range unspecced {
		if len(f.Args) >= 1 {
			v.unspecced[specStr(f.Args[0])] = true
		}
	}
	coverage, _ := h.kernel.Query("spec_coverage")
	for _, f := range coverage {
		if len(f.Args) >= 3 {
			v.coverage[specStr(f.Args[0])] = [2]int64{specNum(f.Args[1]), specNum(f.Args[2])}
		}
	}
	return v
}

func (v *specView) entry(l core.Fact) specEntry {
	spec, code := specStr(l.Args[0]), specStr(l.Args[1])
	key := spec + "\x00" + code
	e := specEntry{section: spec, code: code, relation: strings.TrimPrefix(specStr(l.Args[2]), "/"), drift: v.drift[key]}
	if v.dangling[key] {
		if _, ok := v.sections[spec]; !ok {
			e.dangling = "the section no longer exists"
		} else {
			e.dangling = "the cited element no longer exists"
		}
	}
	return e
}

// fromCode lists the links that end in the elements of the code file rel, the
// stale citations of it, the requirements it violates and what no section reaches.
func (v *specView) fromCode(rel string) ([]specEntry, []string) {
	inFile := map[string]core.Fact{}
	for ref, f := range v.elements {
		if specStr(f.Args[2]) == rel {
			inFile[ref] = f
		}
	}
	var entries []specEntry
	for _, l := range v.links {
		if len(l.Args) < 4 {
			continue
		}
		code := specStr(l.Args[1])
		_, here := inFile[code]
		// A citation of an element that is gone names the file it was in.
		stale := strings.HasPrefix(code, rel+":") && !v.known(code)
		if !here && !stale {
			continue
		}
		e := v.entry(l)
		e.subject = "`" + code + "`"
		entries = append(entries, e)
	}
	sortEntries(entries)
	var extra []string
	refs := make([]string, 0, len(inFile))
	for ref := range inFile {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	var unspecced []string
	for _, ref := range refs {
		if v.unspecced[ref] {
			unspecced = append(unspecced, "`"+ref+"`")
		}
	}
	if len(unspecced) > 0 {
		extra = append(extra, fmt.Sprintf("- No section specifies: %s\n", strings.Join(unspecced, ", ")))
	}
	if dir := packageDirOf(inFile); dir != "" {
		if c, ok := v.coverage[dir]; ok {
			extra = append(extra, fmt.Sprintf("- Specs reach %d of %d exported declarations in `%s`\n", c[0], c[1], dir))
		}
	}
	extra = append(extra, v.violations(inFile)...)
	return entries, extra
}

// fromSection lists the links the sections of the Markdown file rel have.
func (v *specView) fromSection(rel string) ([]specEntry, []string) {
	mine := map[string]bool{}
	for ref, f := range v.sections {
		if specStr(f.Args[1]) == rel {
			mine[ref] = true
		}
	}
	var entries []specEntry
	linked := map[string]core.Fact{}
	for _, l := range v.links {
		if len(l.Args) < 4 {
			continue
		}
		spec := specStr(l.Args[0])
		// A section that does not exist is told by the code that cites it; but a
		// stale citation names the document, so it is listed here too.
		if !mine[spec] && !(strings.HasPrefix(spec, rel+":") && !v.sectionKnown(spec)) {
			continue
		}
		e := v.entry(l)
		code := specStr(l.Args[1])
		e.subject = "`" + spec + "`"
		e.section = code // the line names the code the section links
		entries = append(entries, e)
		if f, ok := v.elements[code]; ok {
			linked[code] = f
		}
	}
	sortEntries(entries)
	return entries, v.violations(linked)
}

func (v *specView) known(code string) bool { _, ok := v.elements[code]; return ok }

func (v *specView) sectionKnown(spec string) bool { _, ok := v.sections[spec]; return ok }

// violations are the requirement_violated rows of the given elements. A
// requirement on an element that is gone is told by that element's file.
func (v *specView) violations(elements map[string]core.Fact) []string {
	var out []string
	for _, f := range v.violated {
		if len(f.Args) < 3 {
			continue
		}
		code := specStr(f.Args[1])
		if _, ok := elements[code]; !ok {
			continue
		}
		id := specStr(f.Args[0])
		line := fmt.Sprintf("- REQUIREMENT %s violated: `%s` lacks %s", strings.TrimPrefix(id, "/"), code, strings.TrimPrefix(specStr(f.Args[2]), "/"))
		if text := v.reqText[id]; text != "" {
			line += fmt.Sprintf(" (%s)", text)
		}
		out = append(out, line+"\n")
	}
	sort.Strings(out)
	return out
}

func packageDirOf(inFile map[string]core.Fact) string {
	dirs := map[string]bool{}
	for _, f := range inFile {
		dirs[specStr(f.Args[1])] = true
	}
	if len(dirs) != 1 {
		return ""
	}
	for d := range dirs {
		return d
	}
	return ""
}

func sortEntries(entries []specEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].subject != entries[j].subject {
			return entries[i].subject < entries[j].subject
		}
		return entries[i].section < entries[j].section
	})
}

// specStr reads a fact argument as text; a name constant keeps its slash.
func specStr(arg any) string {
	if s, ok := arg.(string); ok {
		return s
	}
	return fmt.Sprint(arg)
}

func specNum(arg any) int64 {
	switch n := arg.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}
