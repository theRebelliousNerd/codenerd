package specs

import (
	"regexp"
	"strings"

	"codenerd/internal/docdecl"
	"codenerd/internal/world/codemodel"
)

// RawCite is a citation read from a section, before it is resolved against
// the code. Reading it needs only the document, so a scan is cached per
// document and resolved again whenever the code changes.
type RawCite struct {
	// Section is the key of the section holding the citation.
	Section string
	// Path is the workspace path the citation names, "" for a bare symbol.
	Path string
	// Symbol is the symbol it names, "" for a file-level citation.
	Symbol string
	// Relation and Source say which edge the citation makes.
	Relation Relation
	Source   Source
	// Raw is the citation as written.
	Raw string
	// Explicit marks a citation the author addressed: a path alone, a
	// "path#Symbol" or "path:Symbol" spelling, or an annotation. A backticked
	// symbol that only sits next to a path is not explicit: it may be a
	// standard-library selector or plain prose, so it is stale only when it is
	// a bare name.
	Explicit bool
}

// DocScan is what one Markdown document says about code.
type DocScan struct {
	// Path is the document's workspace path.
	Path string
	// Model is the document's section model, with its source released: it is
	// held to resolve a heading path, not to read text.
	Model    *codemodel.File
	Sections []Section
	Cites    []RawCite
}

// ScanDoc reads one Markdown document: its sections, and the code each cites.
// The lexer fixes which roots a path may start at.
func ScanDoc(path, src string, lex *docdecl.Lexer) *DocScan {
	model := codemodel.ParseMarkdown(path, src)
	scan := &DocScan{Path: path}
	var sections []*codemodel.Element
	for i := range model.Elements {
		if e := &model.Elements[i]; e.Kind == codemodel.KindSection {
			sections = append(sections, e)
			scan.Sections = append(scan.Sections, Section{
				Ref: path + ":" + e.Key, File: path, Key: e.Key, Heading: e.Name,
				Level: e.Level, Revision: e.Revision,
			})
		}
	}
	text := model.Source
	for i, e := range sections {
		// What the section says: its own text after the heading, code blocks
		// included (an example is part of a spec), nested sections excluded.
		scan.Sections[i].Fingerprint = codemodel.Fingerprint(codemodel.LangMarkdown, dropHeading(text[e.Start:ownEnd(sections, i)]))
		for _, span := range ownSpans(model, sections, i) {
			scan.scanSpan(e.Key, text[span[0]:span[1]], lex)
		}
	}
	model.Source = ""
	scan.Model = model
	return scan
}

// ownEnd is where section i's own text stops: at the next heading of any
// level, or at the end of the section.
func ownEnd(sections []*codemodel.Element, i int) int {
	end := sections[i].End
	if i+1 < len(sections) && sections[i+1].Start < end {
		end = sections[i+1].Start
	}
	return end
}

// ownSpans are the byte ranges of section i's own text: from its heading to
// the next heading of any level, with fenced code blocks left out. The nested
// sections are not part of it; each owns its text.
func ownSpans(model *codemodel.File, sections []*codemodel.Element, i int) [][2]int {
	e := sections[i]
	end := ownEnd(sections, i)
	spans := [][2]int{{e.Start, end}}
	for k := range model.Elements {
		b := &model.Elements[k]
		if b.Kind != codemodel.KindCodeBlock || b.Start < e.Start || b.End > end {
			continue
		}
		var next [][2]int
		for _, sp := range spans {
			if b.End <= sp[0] || b.Start >= sp[1] {
				next = append(next, sp)
				continue
			}
			if b.Start > sp[0] {
				next = append(next, [2]int{sp[0], b.Start})
			}
			if b.End < sp[1] {
				next = append(next, [2]int{b.End, sp[1]})
			}
		}
		spans = next
	}
	return spans
}

var (
	htmlComment    = regexp.MustCompile(`<!--(.*?)-->`)
	implementsNote = regexp.MustCompile(`(?i)^\s*implements:\s*(.+?)\s*$`)
	// A selector after a path: ":Symbol", ":Recv.Method", ":decl:pred/2".
	colonSelector = regexp.MustCompile(`^:((?:decl|rule|fact|query):[A-Za-z_][A-Za-z0-9_]*(?:/[0-9]+)?|[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
)

// scanSpan reads the citations in one stretch of a section's own text.
func (d *DocScan) scanSpan(section, text string, lex *docdecl.Lexer) {
	for _, line := range docdecl.SplitLines(text) {
		line = d.takeAnnotations(section, line)
		// A path has a slash and a symbol a backtick; most prose has neither,
		// and the lexer's expressions are the cost of a scan.
		if !strings.ContainsAny(line, "/`") {
			continue
		}
		var paths []docdecl.PathHit
		if strings.Contains(line, "/") {
			paths = lex.Paths(line)
		}
		syms := docdecl.Symbols(line)
		bound, unbound := docdecl.Bind(line, paths, syms)
		for i, p := range paths {
			selectors := d.pathSelectors(line, p, bound[i])
			if len(selectors) == 0 {
				d.cite(section, p.Rel, "", p.Raw, true)
				continue
			}
			for _, sel := range selectors {
				d.cite(section, p.Rel, sel.sym, p.Raw, sel.explicit)
			}
		}
		for _, s := range unbound {
			if docdecl.CodeShaped(s.Sym) {
				d.cite(section, "", s.Sym, "`"+s.Sym+"`", false)
			}
		}
	}
}

// pathSelectors are the symbols a path citation names: the `#Symbol` or
// `:Symbol` written on it, and the backticked symbols bound to it.
func (d *DocScan) pathSelectors(line string, p docdecl.PathHit, bound []docdecl.SymHit) []selector {
	var out []selector
	seen := map[string]bool{}
	add := func(s string, explicit bool) {
		if s != "" && !seen[s] && !docdecl.FileShapedSymbol(s) {
			seen[s] = true
			out = append(out, selector{s, explicit})
		}
	}
	add(p.Hash, true)
	if p.Hash == "" && len(p.Lines) == 0 {
		if m := colonSelector.FindStringSubmatch(line[p.End:]); m != nil {
			add(m[1], true)
		}
	}
	for _, s := range bound {
		add(s.Sym, false)
	}
	return out
}

type selector struct {
	sym      string
	explicit bool
}

// takeAnnotations reads and removes the HTML comments of a line. An
// "implements:" comment is an explicit link; any other comment is not prose
// and must not be read as citations.
func (d *DocScan) takeAnnotations(section, line string) string {
	if !strings.Contains(line, "<!--") {
		return line
	}
	return htmlComment.ReplaceAllStringFunc(line, func(c string) string {
		inner := htmlComment.FindStringSubmatch(c)[1]
		if m := implementsNote.FindStringSubmatch(inner); m != nil {
			for _, target := range strings.Split(m[1], ",") {
				target = strings.TrimSpace(target)
				if target == "" {
					continue
				}
				path, sym := splitTarget(target)
				d.Cites = append(d.Cites, RawCite{Section: section, Path: path, Symbol: sym,
					Relation: RelImplements, Source: SrcAnnotation, Raw: target, Explicit: true})
			}
		}
		return strings.Repeat(" ", len(c))
	})
}

// splitTarget reads an annotation target: "path", "path:Symbol",
// "path#Symbol", or a bare symbol.
func splitTarget(target string) (path, sym string) {
	target = strings.Trim(target, "`")
	if i := strings.IndexAny(target, "#"); i > 0 && strings.Contains(target[:i], ".") {
		return target[:i], strings.TrimSpace(target[i+1:])
	}
	if p, key, ok := splitCodePath(target); ok {
		return p, key
	}
	if strings.Contains(target, "/") && strings.Contains(target, ".") && !predArity.MatchString(target) {
		return target, ""
	}
	return "", target
}

// splitCodePath splits "path/to/file.ext:Key", where the path ends in a code
// extension. The earliest extension wins: a Mangle key may itself read "a:b".
func splitCodePath(s string) (path, key string, ok bool) {
	best := -1
	for _, ext := range codeExts {
		if i := strings.Index(s, "."+ext+":"); i > 0 && (best < 0 || i < best) {
			best = i + len(ext) + 1
		}
	}
	if best < 0 {
		return "", "", false
	}
	return s[:best], s[best+1:], true
}

// codeExts are the extensions of files whose elements a section may cite.
var codeExts = []string{"go", "mg", "py", "ts", "tsx", "js", "jsx"}

// CodeLexer is the path lexer for citing code: paths that start at one of the
// workspace's top-level directories and end in a code file.
func CodeLexer(roots []string) *docdecl.Lexer {
	return docdecl.NewLexer(roots, codeExts)
}

func (d *DocScan) cite(section, path, sym, raw string, explicit bool) {
	d.Cites = append(d.Cites, RawCite{Section: section, Path: path, Symbol: sym,
		Relation: RelCites, Source: SrcDocCitation, Raw: raw, Explicit: explicit})
}
