package specs

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Name        string      `yaml:"name"`
	Title       string      `yaml:"title"`
	Summary     string      `yaml:"summary"`
	Description string      `yaml:"description"`
	ReadWhen    string      `yaml:"read_when"`
	DocType     string      `yaml:"doc_type"`
	Subsystem   string      `yaml:"subsystem"`
	Tags        []string    `yaml:"tags"`
	Source      string      `yaml:"source"`
	Binding     []Binding   `yaml:"binding"`
	Bindings    []Binding   `yaml:"bindings"`
	Invariants  []Invariant `yaml:"invariants"`
}

var invariantPrefixes = []string{"codenerd:invariant", "browsernerd:invariant"}
var invariantEnds = []string{"codenerd:end", "browsernerd:end"}

// Parse parses YAML-frontmatter Markdown plus optional invariant directives.
func Parse(path string, content []byte) (Spec, error) {
	fm, body, bodyOffset, err := splitFrontmatter(content)
	if err != nil {
		return Spec{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(fm.Invariants) > hardMaxInvariantsPerSpec {
		return Spec{}, fmt.Errorf("%s: invariant count exceeds %d", path, hardMaxInvariantsPerSpec)
	}
	bindings := append([]Binding(nil), fm.Binding...)
	bindings = append(bindings, fm.Bindings...)
	doc := Spec{
		Name: boundedRaw(strings.TrimSpace(fm.Name), 256), Title: boundedRaw(stripMarkdownTitle(fm.Title), 256),
		Path: path, Source: boundedRaw(strings.TrimSpace(fm.Source), 1024),
		Summary:   boundedRaw(firstNonEmpty(fm.Summary, fm.Description), 4096),
		ReadWhen:  boundedRaw(strings.TrimSpace(fm.ReadWhen), 2048),
		DocType:   boundedRaw(strings.TrimSpace(fm.DocType), 128),
		Subsystem: boundedRaw(strings.TrimSpace(fm.Subsystem), 128),
		Tags:      normalizeTags(fm.Tags), Bindings: normalizeBindings(bindings), Body: body,
	}
	if doc.Name == "" {
		doc.Name = firstNonEmpty(doc.Title, firstMarkdownHeading(body), strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	if doc.Title == "" {
		doc.Title = firstNonEmpty(firstMarkdownHeading(body), doc.Name)
	}
	if doc.Summary == "" {
		doc.Summary = firstParagraph(body)
	}
	for _, inv := range fm.Invariants {
		inv.Name = boundedRaw(strings.TrimSpace(inv.Name), 256)
		inv.Query = boundedRaw(strings.TrimSpace(inv.Query), 513)
		inv.Expect = normalizeExpect(inv.Expect)
		inv.File = doc.Source
		doc.Invariants = append(doc.Invariants, inv)
	}
	inline, err := parseInlineInvariants(body, bodyOffset, doc.Source)
	if err != nil {
		return Spec{}, fmt.Errorf("%s: %w", path, err)
	}
	doc.Invariants = append(doc.Invariants, inline...)
	if len(doc.Invariants) > hardMaxInvariantsPerSpec {
		return Spec{}, fmt.Errorf("%s: invariant count exceeds %d", path, hardMaxInvariantsPerSpec)
	}
	return doc, nil
}

func splitFrontmatter(content []byte) (frontmatter, string, int, error) {
	text := string(content)
	var fm frontmatter
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64<<10), hardMaxFileBytes)
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fm, "", 0, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, text, 1, nil
	}
	closeIndex := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closeIndex = index
			break
		}
	}
	if closeIndex < 0 {
		return fm, "", 0, fmt.Errorf("unterminated frontmatter block")
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:closeIndex], "\n")), &document); err != nil {
		return fm, "", 0, fmt.Errorf("parse frontmatter: %w", err)
	}
	count := 0
	if err := validateYAMLTree(&document, 0, &count); err != nil {
		return fm, "", 0, fmt.Errorf("parse frontmatter: %w", err)
	}
	if err := document.Decode(&fm); err != nil {
		return fm, "", 0, fmt.Errorf("decode frontmatter: %w", err)
	}
	return fm, strings.Join(lines[closeIndex+1:], "\n"), closeIndex + 2, nil
}

func parseInlineInvariants(body string, bodyOffset int, defaultSource string) ([]Invariant, error) {
	lines := strings.Split(body, "\n")
	result := make([]Invariant, 0)
	for index := 0; index < len(lines); {
		attrs, ok := matchAnyDirective(lines[index], invariantPrefixes)
		if !ok {
			index++
			continue
		}
		openLine := bodyOffset + index
		inv := Invariant{
			Name: boundedRaw(strings.TrimSpace(attrs["name"]), 256), Expect: normalizeExpect(attrs["expect"]),
			File: boundedRaw(defaultSource, 1024), From: positiveInt(attrs["from"]), To: positiveInt(attrs["to"]), Inline: true,
		}
		if value := strings.TrimSpace(attrs["in"]); value != "" {
			inv.File = boundedRaw(value, 1024)
		}
		var prose, query []string
		inQuery, closed := false, false
		end := index + 1
		for ; end < len(lines); end++ {
			if _, matches := matchAnyDirective(lines[end], invariantEnds); matches {
				closed = true
				break
			}
			trimmed := strings.TrimSpace(lines[end])
			if strings.HasPrefix(trimmed, "```") {
				if !inQuery && strings.HasPrefix(trimmed, "```query") {
					inQuery = true
				} else if inQuery {
					inQuery = false
				}
				continue
			}
			if inQuery {
				query = append(query, lines[end])
			} else {
				prose = append(prose, lines[end])
			}
		}
		if !closed {
			return nil, fmt.Errorf("invariant %q opened at line %d has no end directive", inv.Name, openLine)
		}
		if inv.Name == "" {
			inv.Name = fmt.Sprintf("invariant_line_%d", openLine)
		}
		inv.Query = boundedRaw(strings.TrimSpace(strings.Join(query, "\n")), 513)
		inv.Prose = boundedRaw(strings.TrimSpace(strings.Join(prose, "\n")), hardMaxExcerptBytes)
		result = append(result, inv)
		if len(result) > hardMaxInvariantsPerSpec {
			return nil, fmt.Errorf("invariant count exceeds %d", hardMaxInvariantsPerSpec)
		}
		index = end + 1
	}
	return result, nil
}

func matchAnyDirective(line string, prefixes []string) (map[string]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "<!--") || !strings.HasSuffix(trimmed, "-->") {
		return nil, false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
	for _, prefix := range prefixes {
		if inner != prefix && !strings.HasPrefix(inner, prefix+" ") {
			continue
		}
		attrs := make(map[string]string)
		for _, token := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(inner, prefix))) {
			key, value := splitAttribute(token)
			if key != "" {
				attrs[key] = value
			}
		}
		return attrs, true
	}
	return nil, false
}

func splitAttribute(token string) (string, string) {
	for _, separator := range []string{"=", ":"} {
		if index := strings.Index(token, separator); index >= 0 {
			return token[:index], token[index+1:]
		}
	}
	return token, ""
}

func normalizeBindings(bindings []Binding) []Binding {
	if len(bindings) > 20 {
		bindings = bindings[:20]
	}
	result := make([]Binding, 0, len(bindings))
	for _, binding := range bindings {
		binding.Kind = strings.ToLower(strings.TrimSpace(binding.Kind))
		binding.Target = boundedRaw(strings.TrimSpace(binding.Target), 256)
		if binding.Target == "" || binding.Kind != "component" && binding.Kind != "route" && binding.Kind != "selector" {
			continue
		}
		result = append(result, binding)
	}
	return result
}

func normalizeTags(tags []string) []string {
	if len(tags) > 20 {
		tags = tags[:20]
	}
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag = boundedRaw(strings.TrimSpace(tag), 80); tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

func validateYAMLTree(node *yaml.Node, depth int, count *int) error {
	if node == nil {
		return nil
	}
	(*count)++
	if *count > 10000 || depth > 100 {
		return fmt.Errorf("frontmatter structure exceeds parser limits")
	}
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are not allowed")
	}
	for _, child := range node.Content {
		if err := validateYAMLTree(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func boundedRaw(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}

func normalizeExpect(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "present"
	}
	return boundedRaw(value, 32)
}

func positiveInt(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number < 1 {
		return 0
	}
	return number
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func stripMarkdownTitle(value string) string { return strings.Trim(strings.TrimSpace(value), "*_") }

func firstMarkdownHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "# ") {
			return stripMarkdownTitle(strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")))
		}
	}
	return ""
}

func firstParagraph(body string) string {
	lines := make([]string, 0)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "```") {
			continue
		}
		lines = append(lines, trimmed)
		if len(strings.Join(lines, " ")) >= 320 {
			break
		}
	}
	return strings.Join(lines, " ")
}

func sameFile(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	left = filepath.ToSlash(filepath.Clean(left))
	right = filepath.ToSlash(filepath.Clean(right))
	return strings.EqualFold(left, right) || strings.EqualFold(filepath.Base(left), filepath.Base(right))
}
