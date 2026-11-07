package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
// derived: that the code changed since the pair was confirmed while the section
// did not (spec_drift), or the reverse (spec_stale_code), that a link
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
	stale    bool   // the section changed, the code did not follow
	driftAt  int64  // unix seconds of the first scan that saw the changed side, 0 when unknown
	commit   string // the latest commit to the changed file since the link was confirmed, "" when git does not say
	commitAt int64
	// specRef and codeRef name the two ends whichever way the block is read, so
	// the line can say how to confirm the link.
	specRef, codeRef string
	dangling         string // why the link is dangling, "" when it is not
	subject          string // what the line is about, in the target's terms
}

// specCommit is the latest commit to a drifted element's file since its link
// was confirmed (spec_drift_commit).
type specCommit struct {
	hash string
	at   int64
}

type specView struct {
	elements  map[string]core.Fact // spec_element by ref
	sections  map[string]core.Fact // spec_section by ref
	links     []core.Fact
	drift     map[string]bool
	stale     map[string]bool
	driftAt   map[string]int64 // first scan that saw the code differ
	staleAt   map[string]int64 // first scan that saw the section differ
	commits   map[string]specCommit
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
		b.WriteString(" DRIFT: the code changed since this link was confirmed; the section did not follow")
		b.WriteString(specChangeDetail(e))
	}
	if e.stale {
		b.WriteString(" STALE CODE: the section changed since this link was confirmed; the code did not follow")
		b.WriteString(specChangeDetail(e))
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

// specChangeDetail says what changed and how to settle it: when the first scan
// saw the side differ, the commit git attributes to it when git is loaded and
// the change is committed, and the call that confirms the link after a review.
func specChangeDetail(e specEntry) string {
	var parts []string
	if e.driftAt > 0 {
		parts = append(parts, "first seen "+specTime(e.driftAt))
	}
	if e.commit != "" {
		hash := e.commit
		if len(hash) > 8 {
			hash = hash[:8]
		}
		parts = append(parts, fmt.Sprintf("latest commit to the file since: %s %s", hash, specTime(e.commitAt)))
	} else if e.drift {
		parts = append(parts, "no commit since; the change is uncommitted or git history is not loaded")
	}
	detail := ""
	if len(parts) > 0 {
		detail = " (" + strings.Join(parts, "; ") + ")"
	}
	return detail + fmt.Sprintf(". Read both; edit the side that is wrong, or `confirm_spec_link spec_ref=%q code_ref=%q` if they still agree", e.specRef, e.codeRef)
}

func specTime(unix int64) string { return time.Unix(unix, 0).UTC().Format("2006-01-02 15:04 UTC") }

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
		drift: map[string]bool{}, stale: map[string]bool{}, driftAt: map[string]int64{}, staleAt: map[string]int64{},
		commits: map[string]specCommit{}, dangling: map[string]bool{},
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
	stale, _ := h.kernel.Query("spec_stale_code")
	for _, f := range stale {
		if len(f.Args) >= 2 {
			v.stale[specStr(f.Args[0])+"\x00"+specStr(f.Args[1])] = true
		}
	}
	driftSince, _ := h.kernel.Query("spec_drift_since")
	for _, f := range driftSince {
		if len(f.Args) >= 3 {
			v.driftAt[specStr(f.Args[0])+"\x00"+specStr(f.Args[1])] = specNum(f.Args[2])
		}
	}
	staleSince, _ := h.kernel.Query("spec_stale_since")
	for _, f := range staleSince {
		if len(f.Args) >= 3 {
			v.staleAt[specStr(f.Args[0])+"\x00"+specStr(f.Args[1])] = specNum(f.Args[2])
		}
	}
	commits, _ := h.kernel.Query("spec_drift_commit")
	for _, f := range commits {
		if len(f.Args) >= 4 {
			key := specStr(f.Args[0]) + "\x00" + specStr(f.Args[1])
			// Several commits can share the latest second: the first by hash, so the line is stable.
			if c, ok := v.commits[key]; !ok || specStr(f.Args[2]) < c.hash {
				v.commits[key] = specCommit{hash: specStr(f.Args[2]), at: specNum(f.Args[3])}
			}
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
	e := specEntry{section: spec, code: code, specRef: spec, codeRef: code, relation: strings.TrimPrefix(specStr(l.Args[2]), "/"), drift: v.drift[key], stale: v.stale[key]}
	if e.drift {
		e.driftAt = v.driftAt[key]
		if c, ok := v.commits[key]; ok {
			e.commit, e.commitAt = c.hash, c.at
		}
	}
	if e.stale {
		e.driftAt = v.staleAt[key]
	}
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
