package orient

import (
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseMarkdown splits YAML frontmatter the way project docs do, except a
// missing or unclosed fence is the whole file: dropping the body because a
// delimiter was absent would hide the document. Unknown keys stay on
// Frontmatter. The body is the normalized text after the fence, not a prefix.
func parseMarkdown(rel, tool, kind, text string, tracked bool) Source {
	fmText, body, has := splitFront(text)
	fm := map[string]any{}
	if has && strings.TrimSpace(fmText) != "" {
		if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil || fm == nil {
			fm = map[string]any{"unparsed_frontmatter": fmText}
		}
	}
	src := Source{
		ID:            rel,
		Tool:          tool,
		Kind:          kind,
		Path:          rel,
		Tracked:       tracked,
		Body:          body,
		Digest:        digest(body),
		Frontmatter:   fm,
		Description:   stringField(fm, "description"),
		Tags:          tagsOf(fm),
		DeclaredTools: toolsOf(fm),
	}
	src.Name = sourceName(rel, kind, fm)
	src.Prompt = promptOf(kind, fm, body)
	applyScope(&src, rel)
	return src
}

func splitFront(text string) (front, body string, ok bool) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return "", text, false
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] != "---" {
			continue
		}
		return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
	}
	return "", text, false
}

func sourceName(rel, kind string, fm map[string]any) string {
	if name := stringField(fm, "name"); name != "" {
		return name
	}
	if slug := stringField(fm, "slug"); slug != "" {
		return slug
	}
	if path.Base(rel) == "SKILL.md" {
		return path.Base(path.Dir(rel))
	}
	if kind == "memory" {
		parent := path.Base(path.Dir(rel))
		if !isContainerDir(parent) {
			return parent
		}
	}
	return fileStem(rel)
}

func isContainerDir(name string) bool {
	switch name {
	case "agent-memory", "memory", "skills", "rules", "commands", "agents", "references",
		".claude", ".codex", ".agents", ".gemini", ".grok", ".cursor", ".jules", ".github", ".nerd", ".":
		return true
	default:
		return false
	}
}

func fileStem(rel string) string {
	base := path.Base(rel)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		return base
	}
	return stem
}

func stringField(fm map[string]any, key string) string {
	return strings.TrimSpace(rawField(fm, key))
}

// rawField keeps the stored string. Names and descriptions are trimmed;
// a prompt is not, because the trailing newline of a TOML multiline string
// is part of the value the file wrote.
func rawField(fm map[string]any, key string) string {
	if fm == nil {
		return ""
	}
	s, ok := fm[key].(string)
	if !ok {
		return ""
	}
	return s
}

func tagsOf(fm map[string]any) []string {
	if fm == nil {
		return nil
	}
	var out []string
	for _, key := range []string{"tags", "topics", "groups"} {
		out = append(out, stringList(fm[key])...)
	}
	return out
}

func toolsOf(fm map[string]any) []string {
	if fm == nil {
		return nil
	}
	var out []string
	for _, key := range []string{"tools", "allowed-tools", "allowed_tools"} {
		out = append(out, stringList(fm[key])...)
	}
	return out
}

// stringList accepts a comma-separated scalar or a list of scalars. It does
// not split a description: callers pass the tool and tag fields only.
func stringList(v any) []string {
	switch t := v.(type) {
	case string:
		return splitComma(t)
	case []any:
		var out []string
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, splitComma(s)...)
		}
		return out
	default:
		return nil
	}
}

func splitComma(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func promptOf(kind string, fm map[string]any, body string) string {
	for _, key := range []string{"developer_instructions", "prompt", "customInstructions", "roleDefinition"} {
		s := rawField(fm, key)
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	if kind == "subagent" || kind == "mode" {
		return body
	}
	return ""
}

func applyScope(src *Source, rel string) {
	fileDir := fileScope(rel)
	if src.Kind == "rule" || src.Kind == "instructions" {
		if globs := globList(src.Frontmatter); len(globs) > 0 {
			var scopes []string
			for _, g := range globs {
				scopes = append(scopes, globScope(g, fileDir))
			}
			src.Scopes = sortedUnique(scopes)
			if len(src.Scopes) > 0 {
				src.ScopeDir = src.Scopes[0]
			}
			return
		}
	}
	switch src.Kind {
	case "instructions", "rule", "command":
		src.ScopeDir = fileDir
	}
}

func globList(fm map[string]any) []string {
	if fm == nil {
		return nil
	}
	return append(stringList(fm["globs"]), stringList(fm["glob"])...)
}

func globScope(glob, fileDir string) string {
	glob = path.Clean(strings.TrimSpace(filepathToSlash(glob)))
	if glob == "." {
		return fileDir
	}
	i := strings.IndexAny(glob, "*?[")
	if i < 0 {
		return fileDir
	}
	prefix := strings.TrimSuffix(glob[:i], "/")
	if prefix == "" || prefix == "." {
		return fileDir
	}
	return prefix
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

func fileScope(rel string) string {
	// Absolute home paths are not repository subtrees.
	if strings.HasPrefix(rel, "/") || (len(rel) >= 2 && rel[1] == ':') {
		return ""
	}
	d := path.Dir(rel)
	if d == "." || d == "" {
		return "."
	}
	return d
}

func finalize(src *Source) {
	addTopic(src, src.Description)
	for _, tag := range src.Tags {
		addTopic(src, tag)
	}
	addNameTag(src)
}
