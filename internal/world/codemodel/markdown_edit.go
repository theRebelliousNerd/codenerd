package codemodel

import (
	"fmt"
	"regexp"
	"strings"
)

var mdATXMarks = regexp.MustCompile(`^( {0,3})#{1,6}`)

// MarkdownHeadingLevels are the levels of the headings in src, in order. A
// heading inside a fenced code block, front matter or an HTML comment is not
// one.
func MarkdownHeadingLevels(src string) []int {
	s := scanMarkdown(Normalize(src))
	levels := make([]int, len(s.headings))
	for i, h := range s.headings {
		levels[i] = h.level
	}
	return levels
}

// RelevelMarkdown shifts every heading of src so the shallowest one is at
// level to, keeping the relative depth of the rest: a section written for the
// top of a file becomes a child when it is inserted inside another section.
// shift is how many levels it moved (0 leaves src as written). A setext
// heading that moves is written as an ATX heading, because setext has only
// two levels. It refuses a shift that would put a heading outside 1-6.
func RelevelMarkdown(src string, to int) (out string, shift int, err error) {
	src = Normalize(src)
	s := scanMarkdown(src)
	if len(s.headings) == 0 {
		return src, 0, nil
	}
	shallowest, deepest := 7, 0
	for _, h := range s.headings {
		shallowest = min(shallowest, h.level)
		deepest = max(deepest, h.level)
	}
	shift = to - shallowest
	if shift == 0 {
		return src, 0, nil
	}
	if deepest+shift > 6 || to < 1 {
		return "", 0, fmt.Errorf("the source's headings run from level %d to %d; placing the shallowest at level %d would put the deepest at level %d, and Markdown has levels 1 to 6",
			shallowest, deepest, to, deepest+shift)
	}
	rewrite := make(map[int]string) // first line of a heading -> its replacement lines
	skip := make(map[int]bool)      // the other lines of a setext heading
	for _, h := range s.headings {
		marks := strings.Repeat("#", h.level+shift)
		if h.startLine == h.endLine {
			line := s.text(h.startLine)
			rewrite[h.startLine] = mdATXMarks.ReplaceAllString(line, "${1}"+marks)
			continue
		}
		rewrite[h.startLine] = marks + " " + h.text
		for l := h.startLine + 1; l <= h.endLine; l++ {
			skip[l] = true
		}
	}
	var sb strings.Builder
	for i := range s.lines {
		if skip[i] {
			continue
		}
		if r, ok := rewrite[i]; ok {
			sb.WriteString(r)
		} else {
			sb.WriteString(s.text(i))
		}
		if s.lines[i].end < len(src) {
			sb.WriteByte('\n')
		}
	}
	out = sb.String()
	if !strings.HasSuffix(src, "\n") {
		out = strings.TrimSuffix(out, "\n")
	}
	return out, shift, nil
}

// StartsWithMarkdownHeading reports that the first non-blank line of src
// opens a heading, ATX or setext.
func StartsWithMarkdownHeading(src string) bool {
	s := scanMarkdown(Normalize(src))
	if len(s.headings) == 0 {
		return false
	}
	first := firstNonBlank(s, 0, len(s.lines)-1)
	return first >= 0 && s.headings[0].startLine == first
}
