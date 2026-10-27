package init

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// maxAgentAtomContentChars caps one generated agent prompt atom. An atom is
// the unit the JIT compiler selects and embeds whole: the pre-cap files held
// one methodology and one domain atom of 10-23k characters each, which the
// embedding model refused outright (embeddinggemma's window is ~2048 tokens)
// and which selection could only take or leave entire. 2000 characters
// (~500 tokens) keeps an atom to one embeddable idea; the shipped corpus'
// median atom is ~1150 characters.
const maxAgentAtomContentChars = 2000

// agentPromptAtom is one prompts.yaml entry as buildPromptsYAML renders it.
type agentPromptAtom struct {
	ID          string
	Category    string
	Subcategory string
	Priority    int
	Mandatory   bool
	DependsOn   []string
	Description string
	Concise     string
	Min         string
	Content     string
}

// mdSection is one heading-delimited piece of markdown. Title is the heading
// text ("" for a preamble before the first heading); Body keeps the heading
// line so a piece still reads as its section.
type mdSection struct {
	Title string
	Body  string
}

// splitMarkdownSections cuts markdown into atoms at heading boundaries,
// then packs any piece still over maxChars at paragraph, line, and finally
// word boundaries. Every returned body is non-empty and at most maxChars
// runes. A heading with no body of its own rides into the next section
// instead of becoming a stub atom.
func splitMarkdownSections(content string, maxChars int) []mdSection {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if content == "" {
		return nil
	}
	// A body already within the cap is one atom, headings and all: splitting
	// a 900-character identity into per-heading fragments would leave the
	// mandatory base atom holding only its first line.
	if utf8.RuneCountInString(content) <= maxChars {
		return []mdSection{{Body: content}}
	}
	var sections []mdSection
	var cur strings.Builder
	curTitle := ""
	flush := func() {
		body := strings.TrimSpace(cur.String())
		cur.Reset()
		if body == "" {
			return
		}
		sections = append(sections, mdSection{Title: curTitle, Body: body})
	}
	for _, line := range strings.Split(content, "\n") {
		if title, ok := markdownHeading(line); ok {
			flush()
			curTitle = title
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
	}
	flush()

	// A section that is only its heading line belongs with what follows.
	var merged []mdSection
	for i, s := range sections {
		if isHeadingOnly(s) && i+1 < len(sections) {
			sections[i+1].Body = s.Body + "\n\n" + sections[i+1].Body
			continue
		}
		merged = append(merged, s)
	}
	if len(merged) == 0 {
		merged = sections
	}

	var out []mdSection
	for _, s := range merged {
		for _, piece := range packToChars(s.Body, maxChars) {
			out = append(out, mdSection{Title: s.Title, Body: piece})
		}
	}
	return out
}

func isHeadingOnly(s mdSection) bool {
	lines := strings.Split(strings.TrimSpace(s.Body), "\n")
	return len(lines) == 1
}

// markdownHeading reports whether line is an ATX heading of level 1-3 and
// returns its text.
func markdownHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 3 || level >= len(trimmed) || trimmed[level] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[level+1:]), true
}

// packToChars splits body into pieces of at most maxChars runes, packing
// whole paragraphs first, then lines, then words.
func packToChars(body string, maxChars int) []string {
	if utf8.RuneCountInString(body) <= maxChars {
		return []string{body}
	}
	var pieces []string
	var cur strings.Builder
	flush := func() {
		if p := strings.TrimSpace(cur.String()); p != "" {
			pieces = append(pieces, p)
		}
		cur.Reset()
	}
	add := func(unit, sep string) {
		if cur.Len() > 0 && utf8.RuneCountInString(cur.String())+utf8.RuneCountInString(sep)+utf8.RuneCountInString(unit) > maxChars {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString(sep)
		}
		cur.WriteString(unit)
	}
	for _, para := range strings.Split(body, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if utf8.RuneCountInString(para) <= maxChars {
			add(para, "\n\n")
			continue
		}
		for _, line := range strings.Split(para, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if utf8.RuneCountInString(line) <= maxChars {
				add(line, "\n")
				continue
			}
			for _, piece := range splitLongLine(line, maxChars) {
				add(piece, "\n")
			}
		}
	}
	flush()
	return pieces
}

// splitLongLine cuts one over-long line at the last space inside each
// maxChars window, falling back to a hard rune cut.
func splitLongLine(line string, maxChars int) []string {
	runes := []rune(line)
	var out []string
	for len(runes) > maxChars {
		cut := maxChars
		for i := maxChars; i > maxChars/2; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
		out = append(out, strings.TrimSpace(string(runes[:cut])))
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

// atomSlug turns a section title into an id path segment.
func atomSlug(title string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "-")
	}
	if slug == "" {
		return "section"
	}
	return slug
}

// uniqueSlug returns slug, or slug-2, slug-3, ... until unused in seen.
func uniqueSlug(slug string, seen map[string]bool) string {
	candidate := slug
	for n := 2; seen[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", slug, n)
	}
	seen[candidate] = true
	return candidate
}

// sectionTitle names a piece for its id and description: the heading text,
// or the first words of a preamble piece.
func sectionTitle(s mdSection) string {
	if s.Title != "" {
		return s.Title
	}
	first := strings.TrimSpace(strings.SplitN(s.Body, "\n", 2)[0])
	first = strings.TrimLeft(first, "#*- ")
	words := strings.Fields(first)
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Join(words, " ")
}

// atomizeContent turns one markdown body into capped atoms under baseID.
// The first piece keeps baseID (and the concise/min variants, which describe
// the whole), so existing depends_on references to baseID keep resolving;
// later pieces get baseID/<slug> ids.
func atomizeContent(baseID, category, subcategory string, priority int, mandatory bool, dependsOn []string, description, concise, min, content string) []agentPromptAtom {
	pieces := splitMarkdownSections(content, maxAgentAtomContentChars)
	if len(pieces) == 0 {
		pieces = []mdSection{{Body: strings.TrimSpace(content)}}
	}
	seen := map[string]bool{}
	atoms := make([]agentPromptAtom, 0, len(pieces))
	for i, piece := range pieces {
		atom := agentPromptAtom{
			Category:    category,
			Subcategory: subcategory,
			Priority:    priority,
			Mandatory:   mandatory,
			DependsOn:   dependsOn,
			Description: description,
			Content:     piece.Body,
		}
		if i == 0 {
			atom.ID = baseID
			atom.Concise = concise
			atom.Min = min
		} else {
			atom.ID = baseID + "/" + uniqueSlug(atomSlug(sectionTitle(piece)), seen)
		}
		if len(pieces) > 1 {
			if title := sectionTitle(piece); title != "" {
				atom.Description = description + " — " + title
			}
		}
		atoms = append(atoms, atom)
	}
	return atoms
}

// renderAgentPromptAtom renders one atom in the prompts.yaml house style.
func renderAgentPromptAtom(atom agentPromptAtom) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- id: %s\n", strconv.Quote(atom.ID))
	fmt.Fprintf(&b, "  category: %s\n", strconv.Quote(atom.Category))
	fmt.Fprintf(&b, "  subcategory: %s\n", strconv.Quote(atom.Subcategory))
	fmt.Fprintf(&b, "  priority: %d\n", atom.Priority)
	fmt.Fprintf(&b, "  is_mandatory: %t\n", atom.Mandatory)
	if len(atom.DependsOn) > 0 {
		quoted := make([]string, len(atom.DependsOn))
		for i, dep := range atom.DependsOn {
			quoted[i] = strconv.Quote(dep)
		}
		fmt.Fprintf(&b, "  depends_on: [%s]\n", strings.Join(quoted, ", "))
	}
	fmt.Fprintf(&b, "  description: %s\n", strconv.Quote(atom.Description))
	if strings.TrimSpace(atom.Concise) != "" {
		b.WriteString("  content_concise: |\n")
		b.WriteString(indentForYAMLBlock(strings.TrimSpace(atom.Concise), "    "))
		b.WriteString("\n")
	}
	if strings.TrimSpace(atom.Min) != "" {
		b.WriteString("  content_min: |\n")
		b.WriteString(indentForYAMLBlock(strings.TrimSpace(atom.Min), "    "))
		b.WriteString("\n")
	}
	b.WriteString("  content: |\n")
	b.WriteString(indentForYAMLBlock(strings.TrimSpace(atom.Content), "    "))
	b.WriteString("\n")
	return b.String()
}

// resplitPromptsYAML re-atomizes an existing prompts.yaml whose atoms exceed
// the cap: each oversize atom keeps every field, and its content is cut into
// capped sub-atoms (the first keeps the id). Returns changed=false when no
// atom needed splitting. Node-level editing preserves fields this package
// does not know about (intent_verbs, operational_modes, hand edits).
func resplitPromptsYAML(data []byte) ([]byte, bool, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false, fmt.Errorf("yaml parse failed: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.SequenceNode {
		return nil, false, fmt.Errorf("prompts.yaml is not a list of atoms")
	}
	seq := doc.Content[0]
	usedIDs := map[string]bool{}
	for _, item := range seq.Content {
		if id := mappingScalar(item, "id"); id != "" {
			usedIDs[id] = true
		}
	}
	changed := false
	var out []*yaml.Node
	for _, item := range seq.Content {
		content := mappingScalar(item, "content")
		id := mappingScalar(item, "id")
		if id == "" || utf8.RuneCountInString(content) <= maxAgentAtomContentChars {
			out = append(out, item)
			continue
		}
		pieces := splitMarkdownSections(content, maxAgentAtomContentChars)
		if len(pieces) <= 1 {
			out = append(out, item)
			continue
		}
		changed = true
		seen := map[string]bool{}
		description := mappingScalar(item, "description")
		for i, piece := range pieces {
			cp, err := deepCopyNode(item)
			if err != nil {
				return nil, false, err
			}
			setMappingScalar(cp, "content", piece.Body, yaml.LiteralStyle)
			if i > 0 {
				newID := id + "/" + uniqueSlug(atomSlug(sectionTitle(piece)), seen)
				for usedIDs[newID] {
					newID = id + "/" + uniqueSlug(atomSlug(sectionTitle(piece))+"-x", seen)
				}
				usedIDs[newID] = true
				setMappingScalar(cp, "id", newID, 0)
				removeMappingKey(cp, "content_concise")
				removeMappingKey(cp, "content_min")
				if title := sectionTitle(piece); description != "" && title != "" {
					setMappingScalar(cp, "description", description+" — "+title, 0)
				}
			}
			out = append(out, cp)
		}
	}
	if !changed {
		return nil, false, nil
	}
	seq.Content = out
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, false, fmt.Errorf("re-render prompts.yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

func mappingScalar(node *yaml.Node, key string) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1].Value
		}
	}
	return ""
}

func setMappingScalar(node *yaml.Node, key, value string, style yaml.Style) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Value = value
			if style != 0 {
				node.Content[i+1].Style = style
			}
			return
		}
	}
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: style})
}

func removeMappingKey(node *yaml.Node, key string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

func deepCopyNode(node *yaml.Node) (*yaml.Node, error) {
	data, err := yaml.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("copy prompt atom: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("copy prompt atom: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("copy prompt atom: empty document")
	}
	return doc.Content[0], nil
}
