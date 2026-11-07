package specs

import (
	"regexp"
	"strings"

	"codenerd/internal/world/codemodel"
)

var setextUnderline = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

// SectionText returns the prose a section owns, without its heading and
// without the sections nested in it, whitespace folded to single spaces: what
// a reader of the section would be told before the next heading. key is the
// section's key in src. ok is false when src has no such section.
func SectionText(path, src, key string) (text string, ok bool) {
	model := codemodel.ParseMarkdown(path, src)
	var sections []*codemodel.Element
	index := -1
	for i := range model.Elements {
		e := &model.Elements[i]
		if e.Kind != codemodel.KindSection {
			continue
		}
		if e.Key == key {
			index = len(sections)
		}
		sections = append(sections, e)
	}
	if index < 0 {
		return "", false
	}
	var parts []string
	for _, span := range ownSpans(model, sections, index) {
		parts = append(parts, model.Source[span[0]:span[1]])
	}
	lines := strings.Split(strings.Join(parts, "\n"), "\n")
	// Drop the heading construct: an ATX heading is one line, a setext
	// heading is its text and an underline.
	if len(lines) >= 2 && setextUnderline.MatchString(lines[1]) {
		lines = lines[2:]
	} else if len(lines) >= 1 {
		lines = lines[1:]
	}
	return strings.Join(strings.Fields(strings.Join(lines, " ")), " "), true
}
