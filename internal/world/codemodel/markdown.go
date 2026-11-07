package codemodel

import (
	"fmt"
	"regexp"
	"strings"
)

// Markdown is modelled by section, not by line. Docs were edited with line
// tools (edit_lines, apply_edits, insert_lines) because the element verbs had
// no model of them, and a line range drifts the moment anything above it
// moves. A heading is the address a reader already uses, so it is the
// address here.
//
// Elements:
//
//   - header: everything above the first heading (front matter, the lead
//     paragraph). Present only when something is there.
//   - section: a heading and everything up to the next heading of the same or
//     a higher level, its nested sections included. ATX (`## Install`) and
//     setext (`Install` over `-----`) headings both open one.
//   - code_block: a fenced code block, owned by the section it sits in.
//   - table: a pipe table, owned by the section it sits in.
//
// Keys. A section is keyed by its heading path, "Install > Linux"; the same
// heading under the same parent a second time is "Install > Linux#2", in file
// order (the spelling Go uses for a second init). A child of a duplicate
// carries the duplicate's key as its prefix ("Install#2 > Linux"). A code
// block or table is "<section key> > code#N" or "<section key> > table#N",
// N counting that kind within the one section; before the first heading the
// section key is "header". The ref the tools print is "docs/guide.md:" plus
// the key. A key does not name a position: an edit in the body of a section
// leaves every key alone, only a heading rename, an added or removed
// heading, or a new duplicate renumbers keys.
//
// What is ignored: `#` lines inside fenced code blocks, front matter and
// HTML comments are not headings. Indented code and HTML blocks are not
// modelled; a `#` line indented four or more spaces is not a heading.

const (
	// KindSection is a Markdown heading with its body and nested sections.
	KindSection Kind = "section"
	// KindCodeBlock is a fenced code block in a Markdown file.
	KindCodeBlock Kind = "code_block"
	// KindTable is a pipe table in a Markdown file.
	KindTable Kind = "table"
)

// MarkdownHeaderKey is the key of the text above a file's first heading. It is
// the header every other model has.
const MarkdownHeaderKey = HeaderKey

// IsMarkdownPath reports a file CodeDOM models as Markdown sections.
func IsMarkdownPath(path string) bool {
	switch strings.ToLower(pathExt(path)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

func pathExt(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/' && path[i] != '\\'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

var (
	mdATX        = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$`)
	mdSetext     = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	mdFenceOpen  = regexp.MustCompile("^([ \t]*)(`{3,}|~{3,})(.*)$")
	mdTableDelim = regexp.MustCompile(`^[ \t]*\|?[ \t]*:?-+:?[ \t]*(\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$`)
	mdListMarker = regexp.MustCompile(`^[ \t]*([-*+]|\d{1,9}[.)])([ \t]|$)`)
	mdOrdinal    = regexp.MustCompile(`#\d+$`)
)

type mdLine struct{ start, end int } // end excludes the newline

type mdHeading struct {
	level              int
	text               string
	startLine, endLine int // 0-based lines of the heading construct
}

type mdBlock struct {
	kind               Kind
	info               string
	startLine, endLine int
}

type mdScan struct {
	src      string
	lines    []mdLine
	frontEnd int // last line of front matter, -1 when there is none
	headings []mdHeading
	blocks   []mdBlock
}

func (s *mdScan) text(i int) string { return s.src[s.lines[i].start:s.lines[i].end] }

func (s *mdScan) blank(i int) bool { return strings.TrimSpace(s.text(i)) == "" }

func mdSplitLines(src string) []mdLine {
	var lines []mdLine
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lines = append(lines, mdLine{start, i})
			start = i + 1
		}
	}
	if start < len(src) {
		lines = append(lines, mdLine{start, len(src)})
	}
	return lines
}

// scanMarkdown finds a document's headings, fenced code blocks and tables.
func scanMarkdown(src string) *mdScan {
	s := &mdScan{src: src, lines: mdSplitLines(src), frontEnd: -1}
	n := len(s.lines)
	i := 0
	if n > 0 {
		if open := strings.TrimRight(s.text(0), " \t"); open == "---" || open == "+++" {
			for j := 1; j < n; j++ {
				l := strings.TrimRight(s.text(j), " \t")
				if l == open || (open == "---" && l == "...") {
					s.frontEnd = j
					i = j + 1
					break
				}
			}
		}
	}
	var para []int
	inComment := false
	for i < n {
		line := s.text(i)
		if inComment {
			if strings.Contains(line, "-->") {
				inComment = false
			}
			i++
			continue
		}
		if strings.TrimSpace(line) == "" {
			para = nil
			i++
			continue
		}
		if m := mdFenceOpen.FindStringSubmatch(line); m != nil && !(m[2][0] == '`' && strings.Contains(m[3], "`")) {
			end := s.fenceEnd(i, m[2])
			s.blocks = append(s.blocks, mdBlock{kind: KindCodeBlock, info: strings.TrimSpace(m[3]), startLine: i, endLine: end})
			para = nil
			i = end + 1
			continue
		}
		if rest, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), "<!--"); ok {
			if !strings.Contains(rest, "-->") {
				inComment = true
			}
			para = nil
			i++
			continue
		}
		if m := mdATX.FindStringSubmatch(line); m != nil {
			s.headings = append(s.headings, mdHeading{level: len(m[1]), text: strings.TrimSpace(m[2]), startLine: i, endLine: i})
			para = nil
			i++
			continue
		}
		if m := mdSetext.FindStringSubmatch(line); m != nil && len(para) > 0 {
			level := 1
			if m[1][0] == '-' {
				level = 2
			}
			parts := make([]string, len(para))
			for k, p := range para {
				parts[k] = strings.TrimSpace(s.text(p))
			}
			s.headings = append(s.headings, mdHeading{level: level, text: strings.Join(parts, " "), startLine: para[0], endLine: i})
			para = nil
			i++
			continue
		}
		if i+1 < n && s.tableStart(i) {
			end := i + 1
			for end+1 < n && !s.blank(end+1) && mdATX.FindStringSubmatch(s.text(end+1)) == nil && mdFenceOpen.FindStringSubmatch(s.text(end+1)) == nil {
				end++
			}
			s.blocks = append(s.blocks, mdBlock{kind: KindTable, startLine: i, endLine: end})
			para = nil
			i = end + 1
			continue
		}
		if mdParagraphLine(line) {
			para = append(para, i)
		} else {
			para = nil
		}
		i++
	}
	return s
}

// fenceEnd is the line of the fence that closes the one opened at line open,
// or the last line when it is never closed (as CommonMark does).
func (s *mdScan) fenceEnd(open int, fence string) int {
	ch, width := fence[0], len(fence)
	for j := open + 1; j < len(s.lines); j++ {
		l := strings.TrimSpace(s.text(j))
		if len(l) >= width && strings.Trim(l, string(ch)) == "" {
			return j
		}
	}
	return len(s.lines) - 1
}

// tableStart reports a pipe table whose header row is line i: the next line
// is a delimiter row with as many columns.
func (s *mdScan) tableStart(i int) bool {
	head, delim := s.text(i), s.text(i+1)
	if !strings.Contains(head, "|") || !strings.Contains(delim, "|") || !mdTableDelim.MatchString(delim) {
		return false
	}
	return mdCells(head) == mdCells(delim)
}

func mdCells(row string) int {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	return strings.Count(row, "|") + 1
}

// mdParagraphLine reports a line that can be the text of a setext heading. A
// list item, a quote, a table row, an HTML line or an indented line cannot,
// so a `---` under one is a rule, not an underline.
func mdParagraphLine(line string) bool {
	if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
		return false
	}
	switch line[0] {
	case '>', '<', '|':
		return false
	}
	return !mdListMarker.MatchString(line)
}

// ParseMarkdown builds the element model of a Markdown file. A Markdown file
// always parses: every text is a document.
func ParseMarkdown(path, src string) *File {
	src = Normalize(src)
	f := &File{Path: path, Language: LangMarkdown, Source: src, Parsed: true}
	s := scanMarkdown(src)
	n := len(s.lines)
	if n == 0 {
		return f
	}

	// End line of each section: up to the next heading of the same or a
	// higher level, minus the blank lines that separate them.
	sectionEnd := make([]int, len(s.headings))
	for k, h := range s.headings {
		end := n - 1
		for j := k + 1; j < len(s.headings); j++ {
			if s.headings[j].level <= h.level {
				end = s.headings[j].startLine - 1
				break
			}
		}
		for end > h.endLine && s.blank(end) {
			end--
		}
		sectionEnd[k] = end
	}

	var elements []Element
	add := func(e Element, startLine, endLine int) {
		e.Start = s.lines[startLine].start
		e.End = s.lines[endLine].end
		e.UnitStart, e.UnitEnd = e.Start, e.End
		e.StartLine = startLine + 1
		e.EndLine = endLine + 1
		e.Exported = true
		e.Revision = Revision(src[e.Start:e.End])
		elements = append(elements, e)
	}

	firstHeading := n
	if len(s.headings) > 0 {
		firstHeading = s.headings[0].startLine
	}
	if first := firstNonBlank(s, 0, firstHeading-1); first >= 0 {
		last := firstHeading - 1
		for last > first && s.blank(last) {
			last--
		}
		add(Element{Key: MarkdownHeaderKey, Name: MarkdownHeaderKey, Kind: KindHeader,
			Signature: strings.TrimSpace(s.text(first)), DeclLine: first + 1}, first, last)
	}

	// items are the headings and blocks in file order.
	type item struct {
		heading int // index into s.headings, or -1
		block   int
	}
	var items []item
	hi, bi := 0, 0
	for hi < len(s.headings) || bi < len(s.blocks) {
		if bi >= len(s.blocks) || (hi < len(s.headings) && s.headings[hi].startLine < s.blocks[bi].startLine) {
			items = append(items, item{hi, -1})
			hi++
		} else {
			items = append(items, item{-1, bi})
			bi++
		}
	}

	type frame struct {
		level int
		key   string
		path  []string
	}
	var stack []frame
	seen := make(map[string]int)
	counts := make(map[string]int)
	unique := func(base string) string {
		seen[base]++
		if n := seen[base]; n > 1 {
			return fmt.Sprintf("%s#%d", base, n)
		}
		return base
	}
	for _, it := range items {
		if it.heading >= 0 {
			h := s.headings[it.heading]
			for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
				stack = stack[:len(stack)-1]
			}
			name := h.text
			if name == "" {
				name = "(untitled)"
			}
			base, path := name, []string{name}
			if len(stack) > 0 {
				base = stack[len(stack)-1].key + " > " + name
				path = append(append([]string(nil), stack[len(stack)-1].path...), name)
			}
			key := unique(base)
			stack = append(stack, frame{level: h.level, key: key, path: path})
			e := Element{Key: key, Name: name, Names: []string{name}, Kind: KindSection, Level: h.level,
				Signature: strings.TrimSpace(s.text(h.startLine)), DeclLine: h.startLine + 1,
				Doc: markdownLead(s, h.endLine+1, sectionEnd[it.heading]), path: path}
			e.headEnd = s.lines[h.endLine].end
			add(e, h.startLine, sectionEnd[it.heading])
			continue
		}
		b := s.blocks[it.block]
		owner := MarkdownHeaderKey
		if len(stack) > 0 {
			owner = stack[len(stack)-1].key
		}
		word := "code"
		if b.kind == KindTable {
			word = "table"
		}
		counts[owner+"|"+word]++
		key := unique(fmt.Sprintf("%s > %s#%d", owner, word, counts[owner+"|"+word]))
		e := Element{Key: key, Kind: b.kind, Signature: strings.TrimSpace(s.text(b.startLine)), DeclLine: b.startLine + 1}
		if b.kind == KindCodeBlock {
			if lang, _, _ := strings.Cut(b.info, " "); lang != "" {
				e.Name = lang
				e.Names = []string{lang}
			}
		}
		add(e, b.startLine, b.endLine)
	}
	f.Elements = elements
	return f
}

func firstNonBlank(s *mdScan, lo, hi int) int {
	for i := lo; i <= hi && i < len(s.lines); i++ {
		if !s.blank(i) {
			return i
		}
	}
	return -1
}

// markdownLead is the first line of text under a heading, for the listing: it
// says what the section is about without opening it. A fence or a child
// heading is not text.
func markdownLead(s *mdScan, lo, hi int) string {
	i := firstNonBlank(s, lo, hi)
	if i < 0 || i > hi {
		return ""
	}
	line := strings.TrimSpace(s.text(i))
	if mdFenceOpen.MatchString(line) || strings.HasPrefix(line, "<!--") || mdATX.MatchString(line) {
		return ""
	}
	return line
}

// markdownKeyCanon is a key with its ordinals removed, so a section renumbered
// by a duplicate heading above it is still recognised as itself.
func markdownKeyCanon(key string) string {
	parts := strings.Split(key, " > ")
	for i, p := range parts {
		parts[i] = mdOrdinal.ReplaceAllString(p, "")
	}
	return strings.Join(parts, " > ")
}

// normalizeMarkdownQuery spells a path query as keys are spelled: segments
// trimmed and joined by " > ".
func normalizeMarkdownQuery(q string) string {
	parts := strings.Split(q, ">")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, " > ")
}

// lookupMarkdown finds the elements a query names: the key, the key with
// spacing normalised, a bare heading text, or the tail of the heading path
// ("Linux" or "Install > Linux" under "Docs > Install > Linux"). Case is
// ignored only when nothing matches exactly.
func (f *File) lookupMarkdown(query string) []*Element {
	norm := normalizeMarkdownQuery(query)
	segs := strings.Split(norm, " > ")
	find := func(eq func(a, b string) bool) []*Element {
		for i := range f.Elements {
			if e := &f.Elements[i]; eq(e.Key, norm) {
				return []*Element{e}
			}
		}
		var out []*Element
		for i := range f.Elements {
			e := &f.Elements[i]
			if e.Kind != KindSection || len(e.path) < len(segs) {
				continue
			}
			tail := e.path[len(e.path)-len(segs):]
			ok := true
			for k := range segs {
				if !eq(tail[k], segs[k]) {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, e)
			}
		}
		return out
	}
	if out := find(func(a, b string) bool { return a == b }); len(out) > 0 {
		return out
	}
	return find(strings.EqualFold)
}

// MarkdownSections lists the keys of a Markdown file's sections with their
// line spans, in file order: what a stale ref is told is available.
func (f *File) MarkdownSections() []string {
	var out []string
	for i := range f.Elements {
		e := &f.Elements[i]
		if e.Kind == KindSection {
			out = append(out, fmt.Sprintf("%s (lines %d-%d)", e.Key, e.StartLine, e.EndLine))
		}
	}
	return out
}

// markdownTargets are the elements a Markdown change may reshape beyond the
// ones the caller named: every element whose span the change touches (the
// enclosing sections of an edit, the anchor and parents of an insertion), and
// the whole subtree of a section whose heading the change touches, because a
// renamed or replaced heading renames its descendants' keys.
func markdownTargets(f *File, start, end int) []string {
	set := map[string]bool{}
	for i := range f.Elements {
		e := &f.Elements[i]
		var hit bool
		if start == end {
			hit = start >= e.Start && start <= e.End
		} else {
			hit = start < e.End && end > e.Start
		}
		if hit {
			set[e.Key] = true
		}
	}
	for i := range f.Elements {
		t := &f.Elements[i]
		if t.Kind != KindSection {
			continue
		}
		var head bool
		if start == end {
			head = start > t.Start && start < t.headEnd
		} else {
			head = start < t.headEnd && end > t.Start
		}
		if !head {
			continue
		}
		for j := range f.Elements {
			if d := &f.Elements[j]; j != i && d.Start >= t.Start && d.End <= t.End {
				set[d.Key] = true
			}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return keys
}

// markdownRenumbered reports whether the new file still has an element with
// this one's exact bytes under a key that differs only in ordinals: a
// duplicate heading renumbered by an insertion or deletion above it.
func markdownRenumbered(nf *File, oe *Element) bool {
	canon := markdownKeyCanon(oe.Key)
	for i := range nf.Elements {
		if ne := &nf.Elements[i]; ne.Revision == oe.Revision && markdownKeyCanon(ne.Key) == canon {
			return true
		}
	}
	return false
}
