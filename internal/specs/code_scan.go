package specs

import (
	"regexp"
	"strings"

	"codenerd/internal/world/codemodel"
)

// CodeCite is a "spec:" line read from an element's doc comment.
type CodeCite struct {
	// Element is the key of the element the comment documents.
	Element string
	// Doc is the document path the line names.
	Doc string
	// Heading is the heading path after it, "" when the line names a document
	// and no section.
	Heading string
	// Raw is the line as written.
	Raw string
}

// CodeScan is what one code file's doc comments say about specs.
type CodeScan struct {
	Path  string
	Cites []CodeCite
}

// specLine finds "spec: <doc>.md" in a comment line. The heading path is what
// follows the document, after "#", ":" or a space.
var specLine = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])spec:\s*(\S+?\.(?:md|markdown))(.*)$`)

// MayCiteSpecs is the cheap prefilter: a file with no "spec:" in it has no
// citation to read, and need not be parsed.
func MayCiteSpecs(src string) bool { return strings.Contains(src, "spec:") }

// ScanCode reads the "spec:" lines in the doc comments of one code file. The
// comment of an element is the text between the start of its span (where the
// element model puts its leading comment) and its declaration line. A file the
// element model does not parse yields no citations.
func ScanCode(path, src string) *CodeScan {
	scan := &CodeScan{Path: path}
	if !MayCiteSpecs(src) {
		return scan
	}
	model, ok := codemodel.Parse(path, src)
	if !ok || model.Language == codemodel.LangMarkdown {
		return scan
	}
	lineStart := lineStarts(model.Source)
	for i := range model.Elements {
		e := &model.Elements[i]
		if e.DeclLine < 1 || e.DeclLine > len(lineStart) {
			continue
		}
		declStart := lineStart[e.DeclLine-1]
		if e.Start >= declStart {
			continue
		}
		for _, line := range strings.Split(model.Source[e.Start:declStart], "\n") {
			m := specLine.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			heading := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(m[2]), "#: \t"))
			heading = strings.TrimRight(strings.TrimSuffix(heading, "*/"), " \t")
			scan.Cites = append(scan.Cites, CodeCite{Element: e.Key, Doc: m[1], Heading: heading, Raw: strings.TrimSpace(line)})
		}
	}
	return scan
}

func lineStarts(src string) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}
