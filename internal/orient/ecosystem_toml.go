package orient

import (
	"fmt"
	"strconv"
	"strings"
)

// parseTOMLSource keeps the whole file as the body. A hand parser is used
// because the module does not depend on a TOML library; a file that does not
// parse is still returned, with the raw text under unparsed_frontmatter.
func parseTOMLSource(rel, tool, kind, text string, tracked bool) Source {
	fm, err := parseTOMLMap(text)
	if err != nil || fm == nil {
		fm = map[string]any{"unparsed_frontmatter": text}
	}
	src := Source{
		ID:            rel,
		Tool:          tool,
		Kind:          kind,
		Path:          rel,
		Tracked:       tracked,
		Body:          text,
		Digest:        digest(text),
		Frontmatter:   fm,
		Description:   stringField(fm, "description"),
		Tags:          tagsOf(fm),
		DeclaredTools: toolsOf(fm),
	}
	src.Name = sourceName(rel, kind, fm)
	src.Prompt = promptOf(kind, fm, text)
	applyScope(&src, rel)
	return src
}

func parseTOMLMap(text string) (map[string]any, error) {
	p := &tomlParser{s: text}
	out := map[string]any{}
	if err := p.parse(out); err != nil {
		return nil, err
	}
	return out, nil
}

type tomlParser struct {
	s     string
	i     int
	table string
}

func (p *tomlParser) parse(out map[string]any) error {
	for {
		p.skipWSAndComments()
		if p.eof() {
			return nil
		}
		if p.peek() == '[' {
			if err := p.tableHeader(); err != nil {
				return err
			}
			continue
		}
		key, err := p.keyPath()
		if err != nil {
			return err
		}
		p.skipSpace()
		if p.eof() || p.peek() != '=' {
			return fmt.Errorf("toml: expected = after %s", key)
		}
		p.i++
		p.skipSpace()
		val, err := p.value()
		if err != nil {
			return err
		}
		full := key
		if p.table != "" {
			full = p.table + "." + key
		}
		out[full] = val
	}
}

func (p *tomlParser) tableHeader() error {
	p.i++
	array := false
	if !p.eof() && p.peek() == '[' {
		array = true
		p.i++
	}
	start := p.i
	for !p.eof() && p.peek() != ']' && p.peek() != '\n' {
		p.i++
	}
	name := strings.TrimSpace(p.s[start:p.i])
	if p.eof() || p.peek() != ']' {
		return fmt.Errorf("toml: unclosed table")
	}
	p.i++
	if array {
		if p.eof() || p.peek() != ']' {
			return fmt.Errorf("toml: unclosed array table")
		}
		p.i++
		name += "[]"
	}
	if name == "" {
		return fmt.Errorf("toml: empty table")
	}
	p.table = name
	return nil
}

func (p *tomlParser) keyPath() (string, error) {
	var parts []string
	for {
		part, err := p.oneKey()
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
		p.skipSpace()
		if p.eof() || p.peek() != '.' {
			break
		}
		p.i++
		p.skipSpace()
	}
	return strings.Join(parts, "."), nil
}

func (p *tomlParser) oneKey() (string, error) {
	p.skipSpace()
	if p.eof() {
		return "", fmt.Errorf("toml: missing key")
	}
	if p.peek() == '"' || p.peek() == '\'' {
		return p.stringValue()
	}
	start := p.i
	for !p.eof() {
		switch p.peek() {
		case '=', '.', ' ', '\t', '\n', '#', ']', '}':
			if p.i == start {
				return "", fmt.Errorf("toml: missing key")
			}
			return p.s[start:p.i], nil
		}
		p.i++
	}
	if p.i == start {
		return "", fmt.Errorf("toml: missing key")
	}
	return p.s[start:p.i], nil
}

func (p *tomlParser) value() (any, error) {
	if p.eof() {
		return nil, fmt.Errorf("toml: missing value")
	}
	switch p.peek() {
	case '"', '\'':
		return p.stringValue()
	case '[':
		return p.arrayValue()
	case '{':
		return p.inlineTable()
	}
	if strings.HasPrefix(p.rest(), "true") && p.boundary(p.i+4) {
		p.i += 4
		return "true", nil
	}
	if strings.HasPrefix(p.rest(), "false") && p.boundary(p.i+5) {
		p.i += 5
		return "false", nil
	}
	return p.scalar()
}

func (p *tomlParser) scalar() (string, error) {
	start := p.i
	for !p.eof() && !p.endsValue(p.peek()) {
		p.i++
	}
	if p.i == start {
		return "", fmt.Errorf("toml: missing value")
	}
	return strings.TrimSpace(p.s[start:p.i]), nil
}

func (p *tomlParser) endsValue(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == ',' || c == ']' || c == '}' || c == '#'
}

func (p *tomlParser) stringValue() (string, error) {
	if strings.HasPrefix(p.rest(), `"""`) {
		return p.multiline('"', true)
	}
	if strings.HasPrefix(p.rest(), "'''") {
		return p.multiline('\'', false)
	}
	quote := p.peek()
	if quote != '"' && quote != '\'' {
		return "", fmt.Errorf("toml: expected string")
	}
	p.i++
	var b strings.Builder
	for !p.eof() {
		c := p.peek()
		if c == '\n' {
			return "", fmt.Errorf("toml: newline in string")
		}
		if c == quote {
			p.i++
			return b.String(), nil
		}
		if quote == '"' && c == '\\' {
			esc, err := p.escape()
			if err != nil {
				return "", err
			}
			b.WriteString(esc)
			continue
		}
		b.WriteByte(c)
		p.i++
	}
	return "", fmt.Errorf("toml: unclosed string")
}

func (p *tomlParser) multiline(quote byte, escapes bool) (string, error) {
	p.i += 3
	if !p.eof() && p.peek() == '\n' {
		p.i++
	}
	end := strings.Repeat(string(quote), 3)
	var b strings.Builder
	for !p.eof() {
		if strings.HasPrefix(p.rest(), end) {
			p.i += 3
			return b.String(), nil
		}
		if escapes && p.peek() == '\\' {
			if p.i+1 < len(p.s) && p.s[p.i+1] == '\n' {
				p.i += 2
				for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
					p.i++
				}
				continue
			}
			esc, err := p.escape()
			if err != nil {
				return "", err
			}
			b.WriteString(esc)
			continue
		}
		b.WriteByte(p.peek())
		p.i++
	}
	return "", fmt.Errorf("toml: unclosed multiline string")
}

func (p *tomlParser) escape() (string, error) {
	if p.i+1 >= len(p.s) {
		return "", fmt.Errorf("toml: trailing escape")
	}
	p.i++
	c := p.s[p.i]
	p.i++
	switch c {
	case 'n':
		return "\n", nil
	case 't':
		return "\t", nil
	case 'r':
		return "\r", nil
	case '\\':
		return "\\", nil
	case '"':
		return "\"", nil
	case 'b':
		return "\b", nil
	case 'f':
		return "\f", nil
	case 'u':
		return p.hex(4)
	case 'U':
		return p.hex(8)
	default:
		return "", fmt.Errorf("toml: unknown escape \\%c", c)
	}
}

func (p *tomlParser) hex(n int) (string, error) {
	if p.i+n > len(p.s) {
		return "", fmt.Errorf("toml: short unicode escape")
	}
	h := p.s[p.i : p.i+n]
	p.i += n
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return "", fmt.Errorf("toml: unicode escape %q: %w", h, err)
	}
	return string(rune(v)), nil
}

func (p *tomlParser) arrayValue() (any, error) {
	p.i++
	var items []any
	for {
		p.skipWSAndComments()
		if p.eof() {
			return nil, fmt.Errorf("toml: unclosed array")
		}
		if p.peek() == ']' {
			p.i++
			return items, nil
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
		p.skipWSAndComments()
		if !p.eof() && p.peek() == ',' {
			p.i++
		}
	}
}

func (p *tomlParser) inlineTable() (any, error) {
	p.i++
	out := map[string]any{}
	for {
		p.skipWSAndComments()
		if p.eof() {
			return nil, fmt.Errorf("toml: unclosed inline table")
		}
		if p.peek() == '}' {
			p.i++
			return out, nil
		}
		key, err := p.keyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.eof() || p.peek() != '=' {
			return nil, fmt.Errorf("toml: expected = in inline table")
		}
		p.i++
		p.skipSpace()
		val, err := p.value()
		if err != nil {
			return nil, err
		}
		out[key] = val
		p.skipWSAndComments()
		if !p.eof() && p.peek() == ',' {
			p.i++
		}
	}
}

func (p *tomlParser) skipWSAndComments() {
	for !p.eof() {
		switch p.peek() {
		case ' ', '\t', '\n', '\r':
			p.i++
			continue
		case '#':
			for !p.eof() && p.peek() != '\n' {
				p.i++
			}
			continue
		}
		return
	}
}

func (p *tomlParser) skipSpace() {
	for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
		p.i++
	}
}

func (p *tomlParser) eof() bool { return p.i >= len(p.s) }

func (p *tomlParser) peek() byte { return p.s[p.i] }

func (p *tomlParser) rest() string { return p.s[p.i:] }

func (p *tomlParser) boundary(i int) bool {
	if i >= len(p.s) {
		return true
	}
	return p.endsValue(p.s[i])
}
